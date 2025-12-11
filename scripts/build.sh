#!/bin/bash
set -eou pipefail

ROOT_DIR="$(realpath "$(dirname $0)/../")"

ENVIRONMENT_DEFAULT=sepolia
read -p "Ethereum Network [$ENVIRONMENT_DEFAULT]: " ENVIRONMENT
ENVIRONMENT=${ENVIRONMENT:-$ENVIRONMENT_DEFAULT}

VERSION_DEFAULT=$(git describe --tags --match "v[0-9]*" HEAD)
read -p "Version [$VERSION_DEFAULT]: " VERSION
VERSION=${VERSION:-$VERSION_DEFAULT}

REVISION_DEFAULT=$(git rev-parse --short HEAD)
read -p "Revision [$REVISION_DEFAULT]: " REVISION
REVISION=${REVISION:-$REVISION_DEFAULT}

BIN_OUTPUT_DIR=${ROOT_DIR}/out/bin/

docker buildx build \
    --platform linux/amd64 \
    --output type=local,dest=${BIN_OUTPUT_DIR} \
    --target=output-bins \
    --build-arg ENVIRONMENT=${ENVIRONMENT} \
    --build-arg VERSION=${VERSION} \
    --build-arg REVISION=${REVISION} \
    .

docker buildx build \
    --platform=linux/amd64 \
    --target runtime-docker \
    --tag thresholdnetwork/keep-client:latest \
    --tag thresholdnetwork/keep-client:${VERSION} \
    --tag thresholdnetwork/keep-client:${ENVIRONMENT} \
    --build-arg ENVIRONMENT=${ENVIRONMENT} \
    --build-arg VERSION=${VERSION} \
    --build-arg REVISION=${REVISION} \
    .