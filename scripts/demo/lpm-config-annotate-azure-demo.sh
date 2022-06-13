#!/bin/bash

set -euao pipefail

IMAGE_NAME=${IMAGE_NAME:-"python-layered-simple"}

ACR_ACCESS_TOKEN_OUTPUT=$(az acr login --name "$REGISTRY_NAME" --expose-token)
ACR_ACCESS_TOKEN_USERNAME="00000000-0000-0000-0000-000000000000"
ACR_ACCESS_TOKEN=$(echo "$ACR_ACCESS_TOKEN_OUTPUT" | jq --raw-output ".accessToken")
ACR_LOGIN_SERVER=$(echo "$ACR_ACCESS_TOKEN_OUTPUT" | jq --raw-output ".loginServer")

make build-cli

mkdir -p ./examples/manifests/eol-manifests

./bin/lpm config-annotate \
    --username 						"${ACR_ACCESS_TOKEN_USERNAME}" \
    --password 						"${ACR_ACCESS_TOKEN}" \
    --subject-image-ref 			"${ACR_LOGIN_SERVER}/${IMAGE_NAME}:latest" \
    --manifest-media-type 			"application/io.azurecr.distribution.manifest.v2.eol.v1+json" \
    --config-media-type 			"application/io.azurecr.container.image.v1.eol.v1+json" \
    --annotation 					"io.azurecr.eol.v1.subject.eol.date:2025-01-01" \
    --annotation 					"io.azurecr.eol.v1.subject.eol.reason:end-of-maintenance" \
    --annotation 					"io.azurecr.eol.v1.subject.eol.description:This image will no longer be maintained by the maintainer." \
    --annotation 					"io.azurecr.eol.v1.subject.eol.support.url:https://docs.microsoft.com/en-us/azure/container-registry/container-registry-eol" \
    --lpm-manifest-artifact-ref 	"${ACR_LOGIN_SERVER}/${IMAGE_NAME}-eol:latest" \
    --output 						"./examples/manifests/eol-manifests/${IMAGE_NAME}-eol.json"
