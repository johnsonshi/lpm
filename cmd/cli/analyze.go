/*
Copyright © 2022 Johnson Shi <Johnson.Shi@microsoft.com>

*/
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/asottile/dockerfile"
	goocispecv1 "github.com/google/go-containerregistry/pkg/v1"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go"
	ocispecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/spf13/cobra"
	"oras.land/oras-go/pkg/content"
	"oras.land/oras-go/pkg/oras"
)

type analyzeCmd struct {
	stdin                    io.Reader
	stdout                   io.Writer
	stderr                   io.Writer
	username                 string
	password                 string
	dockerfile               string
	subjectImageRef          string
	subjectImageManifestFile string
	lpmManifestArtifactRef   string
	output                   string
}

func newAnalyzeCmd(stdin io.Reader, stdout io.Writer, stderr io.Writer, args []string) *cobra.Command {
	analyzeCmd := &analyzeCmd{
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
	}

	cobraCmd := &cobra.Command{
		Use:   "analyze",
		Short: "TODO",
		Example: `lpm analyze \
--username 						username \
--password 						password \
--subject-image-ref 			myregistry.myserver.io/myimage:latest (or myimage@digest) \
--dockerfile 					Dockerfile \
--subject-image-manifest 		subject-image-manifest.json \
[--lpm-manifest-artifact-ref 	myregistry.myserver.io/myimage-lpm:latest (or myimage-lpm@digest)] \
[--output 						lpm-output-copy.json]
`,
		RunE: func(_ *cobra.Command, args []string) error {
			return analyzeCmd.run()
		},
	}

	f := cobraCmd.Flags()

	var usernameLongFlag = "username"
	f.StringVarP(&analyzeCmd.username, usernameLongFlag, "u", "", "username to use for authentication with the registry")
	cobraCmd.MarkFlagRequired(usernameLongFlag)

	// TODO add support for --password-stdin (reading password from stdin) for more secure password input.
	var passwordLongFlag = "password"
	f.StringVarP(&analyzeCmd.password, passwordLongFlag, "p", "", "password to use for authentication with the registry")
	cobraCmd.MarkFlagRequired(passwordLongFlag)

	var dockerfileLongFlag = "dockerfile"
	f.StringVarP(&analyzeCmd.dockerfile, dockerfileLongFlag, "d", "", "subject image's Dockerfile to use for generating layer provenance metadata")
	cobraCmd.MarkFlagRequired(dockerfileLongFlag)

	var subjectImageRefLongFlag = "subject-image-ref"
	f.StringVarP(&analyzeCmd.subjectImageRef, subjectImageRefLongFlag, "s", "", "subject image reference of the layer provenance metadata to be generated")
	cobraCmd.MarkFlagRequired(subjectImageRefLongFlag)

	var subjectImageManifestFileLongFlag = "subject-image-manifest"
	f.StringVarP(&analyzeCmd.subjectImageManifestFile, subjectImageManifestFileLongFlag, "m", "", "subject image manifest to use for generating layer provenance metadata")
	cobraCmd.MarkFlagRequired(subjectImageManifestFileLongFlag)

	var lpmManifestArtifactRefLongFlag = "lpm-manifest-artifact-ref"
	f.StringVarP(&analyzeCmd.lpmManifestArtifactRef, lpmManifestArtifactRefLongFlag, "t", "", "(optional) target artifact ref in which the lpm manifest file will be pushed to")

	f.StringVarP(&analyzeCmd.output, "output", "o", "", "(optional) output file to also write layer provenance metadata (default: stdout)")

	return cobraCmd
}

func (analyzeCmd *analyzeCmd) run() error {
	// Set output writer.
	var out io.Writer
	if analyzeCmd.output == "" {
		out = analyzeCmd.stdout
	} else {
		f, err := os.Create(analyzeCmd.output)
		if err != nil {
			return err
		}
		defer f.Close()
		out = f
	}

	// Parse subject image Dockerfile.
	dockerfileCommands, err := dockerfile.ParseFile(analyzeCmd.dockerfile)
	if err != nil {
		return err
	}

	// Parse subject image manifest file.
	subjectManifestIn, err := os.Open(analyzeCmd.subjectImageManifestFile)
	if err != nil {
		return err
	}
	defer subjectManifestIn.Close()
	subjectManifest, err := goocispecv1.ParseManifest(subjectManifestIn)
	if err != nil {
		return err
	}

	// Modify the subject image manifest to include layer provenance metadata (as OCI annotations) using the Dockerfile.
	subjectManifestWithDockerfileOrigin, err := modifyManifestWithDockerfileOrigin(dockerfileCommands, subjectManifest)
	if err != nil {
		return err
	}

	ctx := context.Background()

	// Create a new ORAS memory store.
	memoryStore := content.NewMemory()

	// Iterate through the subject layer descriptors
	// (containing layer ownership info annotated earlier)
	// and generate reference layer descriptors.
	//
	// These reference layer descriptors will be packaged
	// together as a reference manifest with a reference
	// config.
	//
	// The reference manifest (together with the reference
	// config and reference layer descriptors) will be pushed
	// as an ORAS reference artifact to the registry.
	//
	// As we iterate through the subject layers, we create
	// a corresponding ocispecv1 reference layer descriptor
	// (that refers to the goocispecv1 subject layer)
	// in the memory store.
	referenceLayerDescs := make([]ocispecv1.Descriptor, 0)
	for _, subjectLayerDesc := range subjectManifestWithDockerfileOrigin.Layers {
		// Copy the lpm manifest's annotations, which contains the subject layer's ownership information.
		layerAnnotations := subjectLayerDesc.Annotations

		// Add additional annotations about the subject layer.
		layerAnnotations[annotationKeyForSubjectMediaType] = string(subjectLayerDesc.MediaType)
		layerAnnotations[annotationKeyForSubjectDigest] = subjectLayerDesc.Digest.String()
		layerAnnotations[annotationKeyForSubjectSize] = fmt.Sprint(subjectLayerDesc.Size)

		// Create a reference layer descriptor that refers to the subject layer.
		content := []byte("")
		referenceLayerDesc := ocispecv1.Descriptor{
			MediaType:   mediaTypeForLayerLpm,
			Digest:      digest.FromBytes(content),
			Size:        int64(len(content)),
			Annotations: layerAnnotations,
		}

		// If we are supposed to push the lpm manifest to a registry,
		if analyzeCmd.lpmManifestArtifactRef != "" {
			// Add the reference layer descriptor to the memory store.
			memoryStore.Set(referenceLayerDesc, content)
		}

		// Append the reference layer descriptor in order.
		// Layers are from bottom layer to top layer.
		referenceLayerDescs = append(referenceLayerDescs, referenceLayerDesc)
	}

	// Create manifest annotations for the reference manifest.
	referenceManifestAnnotations := subjectManifestWithDockerfileOrigin.Annotations
	referenceManifestAnnotations[annotationKeyForSubjectMediaType] = string(subjectManifestWithDockerfileOrigin.MediaType)

	// Create config annotations for the reference manifest's config.
	referenceConfigAnnotations := subjectManifestWithDockerfileOrigin.Config.Annotations
	referenceConfigAnnotations[annotationKeyForSubjectMediaType] = string(subjectManifestWithDockerfileOrigin.Config.MediaType)
	referenceConfigAnnotations[annotationKeyForSubjectDigest] = subjectManifestWithDockerfileOrigin.Config.Digest.String()
	referenceConfigAnnotations[annotationKeyForSubjectSize] = fmt.Sprint(subjectManifestWithDockerfileOrigin.Config.Size)

	// Create the reference manifest and reference config.
	referenceManifest, referenceManifestDesc, referenceConfig, referenceConfigDesc, err := content.GenerateManifestAndConfig(referenceManifestAnnotations, referenceConfigAnnotations, referenceLayerDescs...)
	if err != nil {
		return err
	}

	// Set the reference config descriptor's mediaType because content.GenerateConfig() sets the mediaType to "application/vnd.unknown.config.v1+json".
	// See https://github.com/oras-project/oras-go/blob/v1.1.1/pkg/content/manifest.go#L41-L52
	referenceConfigDesc.MediaType = mediaTypeForConfigLpm

	// Write the complete reference manifest (with reference config and reference layers) to output.
	completeManifest := ocispecv1.Manifest{
		Versioned:   ocispecs.Versioned{SchemaVersion: int(subjectManifestWithDockerfileOrigin.SchemaVersion)},
		MediaType:   mediaTypeForManifestLpm,
		Config:      referenceConfigDesc,
		Layers:      referenceLayerDescs,
		Annotations: referenceManifestAnnotations,
	}
	completeManifestJsonString, err := json.MarshalIndent(completeManifest, "", "	")
	if err != nil {
		return err
	}
	out.Write(completeManifestJsonString)

	// Return early if we are not supposed to push the lpm manifest to a registry.
	if analyzeCmd.lpmManifestArtifactRef == "" {
		return nil
	}

	// Add the reference manifest and reference config to the memory store.
	memoryStore.Set(referenceConfigDesc, referenceConfig)
	err = memoryStore.StoreManifest(analyzeCmd.lpmManifestArtifactRef, referenceManifestDesc, referenceManifest)
	if err != nil {
		return err
	}

	// Create a local registry store.
	registry, err := content.NewRegistry(content.RegistryOptions{Username: analyzeCmd.username, Password: analyzeCmd.password})
	if err != nil {
		return err
	}

	fmt.Printf("[*] Pushing to '%s' as an ORAS reference to subject image '%s'...\n", analyzeCmd.lpmManifestArtifactRef, analyzeCmd.subjectImageRef)

	// Push the reference manifest.
	// TODO add an ORAS reference from the reference manifest to the subject image ref.
	desc, err := oras.Copy(ctx, memoryStore, analyzeCmd.lpmManifestArtifactRef, registry, "")
	if err != nil {
		return err
	}
	fmt.Printf("Pushed to '%s' with digest '%s'\n", analyzeCmd.lpmManifestArtifactRef, desc.Digest)

	return nil
}

func modifyManifestWithDockerfileOrigin(dockerfileCommands []dockerfile.Command, manifest *goocispecv1.Manifest) (*goocispecv1.Manifest, error) {
	// Set ownership of the image manifest to "non-upstream".
	manifest.Annotations = deepCopyMap(annotationsForNonUpstreamOwnership)
	// Set ownership of the image manifest config to "non-upstream".
	manifest.Config.Annotations = deepCopyMap(annotationsForNonUpstreamOwnership)

	d := len(dockerfileCommands) - 1
	m := len(manifest.Layers) - 1
	for ; d >= 0 && m >= 0; d, m = d-1, m-1 {
		// Stop processing if the Dockerfile command is a "FROM" command.
		// Reason:
		//		The remaining image manifest layers are inherited from the "FROM" command's base image.
		//		Therefore, the ownership of the remaining image manifest layers will be set to "upstream".
		if strings.ToUpper(dockerfileCommands[d].Cmd) == "FROM" {
			break
		}

		// Set ownership of the image manifest layer to "non-upstream".
		manifest.Layers[m].Annotations = deepCopyMap(annotationsForNonUpstreamOwnership)
		manifest.Layers[m].Annotations[annotationKeyForSubjectOriginalDockerfileFullCommand] = dockerfileCommands[d].Original
	}

	for ; m >= 0; m = m - 1 {
		// Set ownership of the remaining image manifest layers to "upstream".
		manifest.Layers[m].Annotations = deepCopyMap(annotationsForUpstreamOwnership)
		manifest.Layers[m].Annotations[annotationKeyForSubjectOriginalDockerfileFullCommand] = dockerfileCommands[d].Original
	}

	return manifest, nil
}

func deepCopyMap(m map[string]string) map[string]string {
	newMap := make(map[string]string)
	for k, v := range m {
		newMap[k] = v
	}
	return newMap
}
