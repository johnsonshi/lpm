/*
Copyright © 2022 Johnson Shi <Johnson.Shi@microsoft.com>

*/
package main

var annotationsForUpstreamOwnership = map[string]string{
	"io.azurecr.lpm.v1.subject.authors": "upstream",
	"io.azurecr.lpm.v1.subject.url":     "upstream",
	"io.azurecr.lpm.v1.subject.source":  "upstream",
	"io.azurecr.lpm.v1.subject.vendor":  "upstream",
}

var annotationsForNonUpstreamOwnership = map[string]string{
	"io.azurecr.lpm.v1.subject.authors": "non-upstream",
	"io.azurecr.lpm.v1.subject.url":     "non-upstream",
	"io.azurecr.lpm.v1.subject.source":  "non-upstream",
	"io.azurecr.lpm.v1.subject.vendor":  "non-upstream",
}

var annotationKeyForSubjectOriginalDockerfileFullCommand = "io.azurecr.lpm.v1.subject.dockerfile.fullcommand"

var annotationKeyForSubjectMediaType = "io.azurecr.lpm.v1.subject.mediaType"
var annotationKeyForSubjectDigest = "io.azurecr.lpm.v1.subject.digest"
var annotationKeyForSubjectSize = "io.azurecr.lpm.v1.subject.size"

var mediaTypeForManifestLpm = "application/io.azurecr.distribution.manifest.v2.lpm.v1+json"
var mediaTypeForConfigLpm = "application/io.azurecr.container.image.v1.lpm.v1+json"
var mediaTypeForLayerLpm = "application/io.azurecr.image.rootfs.diff.tar.gzip.lpm.v1+json"
