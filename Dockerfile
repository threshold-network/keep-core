FROM golang:1.24-alpine3.21 AS build-sources

ENV GOPATH=/go \
	GOBIN=/go/bin \
	APP_NAME=keep-client \
	APP_DIR=/go/src/github.com/keep-network/keep-core \
	TEST_RESULTS_DIR=/mnt/test-results \
	BIN_PATH=/usr/local/bin \
	LD_LIBRARY_PATH=/usr/local/lib/

# TODO: Remove perl once go-ethereum is upgraded to 1.11.
#       See pkg/chain/ethereum/tbtc/gen/Makefile and after_abi_hook for details.
RUN apk update && apk upgrade && apk add --update --no-cache \
	g++ \
	linux-headers \
	protobuf-dev \
	git \
	make \
	nodejs \
	npm \
	bash \
	perl \
	python3 \
	tar \
	jq && \
	rm -rf /var/cache/apk/ && mkdir /var/cache/apk/ && \
	rm -rf /usr/share/man

RUN go install gotest.tools/gotestsum@v1.10.1

RUN mkdir -p $APP_DIR $TEST_RESULTS_DIR

WORKDIR $APP_DIR

# Get dependencies. The third_party directory holds an in-tree module served
# by a directory `replace` in go.mod, so it must be present before the module
# graph can be resolved.
COPY go.mod go.sum $APP_DIR/
COPY ./third_party $APP_DIR/third_party
RUN go mod download

# Copy source code for generation.
COPY ./pkg/beacon/dkg/result/gen $APP_DIR/pkg/beacon/dkg/result/gen
COPY ./pkg/beacon/entry/gen $APP_DIR/pkg/beacon/entry/gen
COPY ./pkg/beacon/gjkr/gen $APP_DIR/pkg/beacon/gjkr/gen
COPY ./pkg/beacon/registry/gen $APP_DIR/pkg/beacon/registry/gen
COPY ./pkg/chain/ethereum/beacon/gen $APP_DIR/pkg/chain/ethereum/beacon/gen
COPY ./pkg/chain/ethereum/common/gen $APP_DIR/pkg/chain/ethereum/common/gen
COPY ./pkg/chain/ethereum/ecdsa/gen $APP_DIR/pkg/chain/ethereum/ecdsa/gen
COPY ./pkg/chain/ethereum/tbtc/gen $APP_DIR/pkg/chain/ethereum/tbtc/gen
COPY ./pkg/chain/ethereum/threshold/gen $APP_DIR/pkg/chain/ethereum/threshold/gen
COPY ./pkg/net/gen $APP_DIR/pkg/net/gen
COPY ./pkg/tbtc/gen $APP_DIR/pkg/tbtc/gen
COPY ./pkg/tecdsa/dkg/gen $APP_DIR/pkg/tecdsa/dkg/gen
COPY ./pkg/tecdsa/signing/gen $APP_DIR/pkg/tecdsa/signing/gen
COPY ./pkg/tecdsa/gen $APP_DIR/pkg/tecdsa/gen
COPY ./pkg/protocol/announcer/gen $APP_DIR/pkg/protocol/announcer/gen
COPY ./pkg/protocol/inactivity/gen $APP_DIR/pkg/protocol/inactivity/gen


# Install code generators.
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.32.0

# Environment is to download published and tagged NPM packages versions.
# Defaults to `development` to mirror the root Makefile's `ifndef environment`
# fallback (the "Build Docker Build Image" CI step never passes this build-arg).
ARG ENVIRONMENT=development

COPY ./Makefile $APP_DIR/Makefile
RUN make get_artifacts environment=$ENVIRONMENT

# TODO(https://github.com/threshold-network/tbtc-v2/pull/1112): remove once
# @keep-network/tbtc-v2 publishes Bridge/WalletProposalValidator/RedemptionWatchtower/
# ReservationRouter (and the rest of the tbtc module's required_contracts) under the
# `development` npm tag. Until then, `get_artifacts` fetches a tbtc-v2 package whose
# Bridge/WalletProposalValidator/RedemptionWatchtower don't yet expose the reservation
# methods this PR binds against, and has no ReservationRouter artifact at all. The
# `client.yml` workflow locally compiles tbtc-v2 PR #1112 (pinned SHA) and drops its
# compiled ABI artifacts for the tbtc module's required_contracts at
# ./ci-shims/tbtc-artifacts/*.json when it runs; this only overrides the tbtc module's
# artifacts, and only for `environment=development` (PR CI) builds - sepolia/mainnet
# builds and the beacon/ecdsa/threshold modules are untouched.
COPY ./ci-shims/tbtc-artifacts /tmp/tbtc-artifacts
RUN if { [ -z "$ENVIRONMENT" ] || [ "$ENVIRONMENT" = "development" ]; } && [ -n "$(ls -A /tmp/tbtc-artifacts 2>/dev/null)" ]; then \
	echo "Using tbtc-v2 module artifacts built from tbtc-v2 PR #1112 (temporary shim)"; \
	cp /tmp/tbtc-artifacts/*.json \
		$APP_DIR/tmp/contracts/development/@keep-network/tbtc-v2/artifacts/; \
fi

# Need this to resolve imports in generated Ethereum commands.
COPY ./config $APP_DIR/config
RUN make generate environment=$ENVIRONMENT

COPY ./ $APP_DIR/

# Update go.sum with any missing dependencies
RUN go mod tidy && go mod download

#
# Build Docker Image
#
FROM build-sources AS build-docker

WORKDIR $APP_DIR

# Client Versioning.
ARG VERSION
ARG REVISION

RUN GOOS=linux make build \
	version=$VERSION \
	revision=$REVISION

FROM alpine:3.21 as runtime-docker

ENV APP_NAME=keep-client \
	APP_DIR=/go/src/github.com/keep-network/keep-core \
	BIN_PATH=/usr/local/bin

# Update Alpine packages to get latest security patches
RUN apk update && apk upgrade && rm -rf /var/cache/apk/*

COPY --from=build-docker $APP_DIR/$APP_NAME $BIN_PATH

# ENTRYPOINT cant handle ENV variables.
ENTRYPOINT ["keep-client"]

# docker caches more when using CMD [] resulting in a faster build.
CMD []

#
# Build Binaries
#
FROM golang:1.24-bullseye AS build-bins

ENV APP_DIR=/go/src/github.com/keep-network/keep-core

WORKDIR $APP_DIR

COPY --from=build-sources $APP_DIR $APP_DIR

ARG ENVIRONMENT

# Client Versioning.
ARG VERSION
ARG REVISION

RUN make release \
	environment=$ENVIRONMENT \
	version=$VERSION \
	revision=$REVISION

FROM scratch as output-bins

ENV APP_DIR=/go/src/github.com/keep-network/keep-core

COPY --from=build-bins $APP_DIR/out/bin .
