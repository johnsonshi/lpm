#!/bin/bash

set -euao pipefail

IMAGE_NAME=${IMAGE_NAME:-"python-layered-simple"}

ACR_ACCESS_TOKEN_OUTPUT=$(az acr login --name "$REGISTRY_NAME" --expose-token)
ACR_ACCESS_TOKEN_USERNAME="00000000-0000-0000-0000-000000000000"
ACR_ACCESS_TOKEN=$(echo "$ACR_ACCESS_TOKEN_OUTPUT" | jq --raw-output ".accessToken")
ACR_LOGIN_SERVER=$(echo "$ACR_ACCESS_TOKEN_OUTPUT" | jq --raw-output ".loginServer")

make build-cli

mkdir -p ./examples/manifests/lpm-manifests/

./bin/lpm analyze \
    --username 						"${ACR_ACCESS_TOKEN_USERNAME}" \
    --password 						"${ACR_ACCESS_TOKEN}" \
    --subject-image-ref 			"${ACR_LOGIN_SERVER}/${IMAGE_NAME}:latest" \
    --dockerfile 					"./examples/dockerfiles/${IMAGE_NAME}.dockerfile" \
    --subject-image-manifest 		"./examples/manifests/subject-image-manifests/${IMAGE_NAME}.json" \
    --lpm-manifest-artifact-ref 	"${ACR_LOGIN_SERVER}/${IMAGE_NAME}-lpm:latest" \
    --output 						"./examples/manifests/lpm-manifests/${IMAGE_NAME}-lpm.json"
