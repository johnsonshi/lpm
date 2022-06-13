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
	"regexp"

	ocispecs "github.com/opencontainers/image-spec/specs-go"
	ocispecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/spf13/cobra"
	"oras.land/oras-go/pkg/content"
	"oras.land/oras-go/pkg/oras"
)

type configAnnotateCmd struct {
	stdin                  io.Reader
	stdout                 io.Writer
	stderr                 io.Writer
	username               string
	password               string
	subjectImageRef        string
	manifestMediaType      string
	configMediaType        string
	annotationSlice        []string
	lpmManifestArtifactRef string
	output                 string
}

func newConfigAnnotateCmd(stdin io.Reader, stdout io.Writer, stderr io.Writer, args []string) *cobra.Command {
	configAnnotateCmd := &configAnnotateCmd{
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
	}

	cobraCmd := &cobra.Command{
		Use:   "config-annotate",
		Short: "TODO",
		Example: `lpm config-annotate \
--username 						username \
--password 						password \
--subject-image-ref 			myregistry.myserver.io/myimage:latest (or myimage@digest) \
--manifest-media-type 			"application/io.azurecr.distribution.manifest.v2.eol.v1+json" \
--config-media-type 			"application/io.azurecr.container.image.v1.eol.v1+json" \
--annotation 					"io.azurecr.eol.v1.subject.eol.date:2025-01-01" \
--annotation 					"io.azurecr.eol.v1.subject.eol.reason:end-of-maintenance" \
--annotation 					"io.azurecr.eol.v1.subject.eol.description:This image will no longer be maintained by the maintainer." \
--annotation 					"io.azurecr.eol.v1.subject.eol.support.url:https://docs.microsoft.com/en-us/azure/container-registry/container-registry-eol" \
[--lpm-manifest-artifact-ref 	myregistry.myserver.io/myimage-lpm:latest (or myimage-lpm@digest)] \
[--output 						lpm-output-copy.json]
`,
		RunE: func(_ *cobra.Command, args []string) error {
			return configAnnotateCmd.run()
		},
	}

	f := cobraCmd.Flags()

	var usernameLongFlag = "username"
	f.StringVarP(&configAnnotateCmd.username, usernameLongFlag, "u", "", "username to use for authentication with the registry")
	cobraCmd.MarkFlagRequired(usernameLongFlag)

	// TODO add support for --password-stdin (reading password from stdin) for more secure password input.
	var passwordLongFlag = "password"
	f.StringVarP(&configAnnotateCmd.password, passwordLongFlag, "p", "", "password to use for authentication with the registry")
	cobraCmd.MarkFlagRequired(passwordLongFlag)

	var subjectImageRefLongFlag = "subject-image-ref"
	f.StringVarP(&configAnnotateCmd.subjectImageRef, subjectImageRefLongFlag, "s", "", "subject image reference of the config annotation")
	cobraCmd.MarkFlagRequired(subjectImageRefLongFlag)

	var manifestMediaTypeLongFlag = "manifest-media-type"
	f.StringVarP(&configAnnotateCmd.manifestMediaType, manifestMediaTypeLongFlag, "m", "", "manifest media type of the generated manifest")
	cobraCmd.MarkFlagRequired(manifestMediaTypeLongFlag)

	var configMediaTypeLongFlag = "config-media-type"
	f.StringVarP(&configAnnotateCmd.configMediaType, configMediaTypeLongFlag, "c", "", "mediaType of the generated manifest's config")
	cobraCmd.MarkFlagRequired(configMediaTypeLongFlag)

	var annotationSliceLongFlag = "annotation"
	f.StringArrayVarP(&configAnnotateCmd.annotationSlice, annotationSliceLongFlag, "a", []string{}, "annotation to add to the generated manifest's config")
	cobraCmd.MarkFlagRequired(annotationSliceLongFlag)

	var lpmManifestArtifactRefLongFlag = "lpm-manifest-artifact-ref"
	f.StringVarP(&configAnnotateCmd.lpmManifestArtifactRef, lpmManifestArtifactRefLongFlag, "t", "", "(optional) target artifact ref in which the generated manifest file (with config annotations) will be pushed to as an ORAS referrer to the subject image")

	f.StringVarP(&configAnnotateCmd.output, "output", "o", "", "(optional) output file to also write the generated manifest file (with config annotations) (default: stdout)")

	return cobraCmd
}

func (configAnnotateCmd *configAnnotateCmd) run() error {
	// Set output writer.
	var out io.Writer
	if configAnnotateCmd.output == "" {
		out = configAnnotateCmd.stdout
	} else {
		f, err := os.Create(configAnnotateCmd.output)
		if err != nil {
			return err
		}
		defer f.Close()
		out = f
	}

	// Process annotations by turning them into a map.
	re := regexp.MustCompile(`:\s*`)
	annotationsMap := make(map[string]string)
	for _, rawAnnotation := range configAnnotateCmd.annotationSlice {
		annotation := re.Split(rawAnnotation, 2)
		if len(annotation) != 2 {
			return fmt.Errorf("invalid annotation: %s", rawAnnotation)
		}
		annotationsMap[annotation[0]] = annotation[1]
		configAnnotateCmd.stderr.Write([]byte(fmt.Sprintf("[*] annotation: '%s: %s'\n", annotation[0], annotation[1])))
	}

	ctx := context.Background()

	// Create a new ORAS memory store.
	memoryStore := content.NewMemory()

	layerDescs := make([]ocispecv1.Descriptor, 0)
	manifest, manifestDesc, config, configDesc, err := content.GenerateManifestAndConfig(nil, nil, layerDescs...)
	if err != nil {
		return err
	}

	// Set the reference config descriptor's mediaType because content.GenerateConfig() sets the mediaType to "application/vnd.unknown.config.v1+json".
	// See https://github.com/oras-project/oras-go/blob/v1.1.1/pkg/content/manifest.go#L41-L52
	configDesc.MediaType = configAnnotateCmd.configMediaType

	// Add the config annotations to the config.
	configDesc.Annotations = annotationsMap

	// Write the complete reference manifest (with reference config and reference layers) to output.
	completeManifest := ocispecv1.Manifest{
		Versioned:   ocispecs.Versioned{SchemaVersion: int(2)},
		MediaType:   configAnnotateCmd.manifestMediaType,
		Config:      configDesc,
		Layers:      layerDescs,
		Annotations: map[string]string{},
	}
	completeManifestJsonString, err := json.MarshalIndent(completeManifest, "", "	")
	if err != nil {
		return err
	}
	out.Write(completeManifestJsonString)

	// Return early if we are not supposed to push the generated manifest to a registry.
	if configAnnotateCmd.lpmManifestArtifactRef == "" {
		return nil
	}

	// Add the reference manifest and reference config to the memory store.
	memoryStore.Set(configDesc, config)
	err = memoryStore.StoreManifest(configAnnotateCmd.lpmManifestArtifactRef, manifestDesc, manifest)
	if err != nil {
		return err
	}

	// Create a local registry store.
	registry, err := content.NewRegistry(content.RegistryOptions{Username: configAnnotateCmd.username, Password: configAnnotateCmd.password})
	if err != nil {
		return err
	}

	fmt.Printf("[*] Pushing to '%s' as an ORAS reference to subject image '%s'...\n", configAnnotateCmd.lpmManifestArtifactRef, configAnnotateCmd.subjectImageRef)

	// Push the reference manifest.
	// TODO add an ORAS reference from the reference manifest to the subject image ref.
	desc, err := oras.Copy(ctx, memoryStore, configAnnotateCmd.lpmManifestArtifactRef, registry, "")
	if err != nil {
		return err
	}
	fmt.Printf("Pushed to '%s' with digest '%s'\n", configAnnotateCmd.lpmManifestArtifactRef, desc.Digest)

	return nil
}
