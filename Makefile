# environment is used as a tag of the npm packages for contracts artifacts. If
# not overwritten it defaults to `development`.
ifndef environment
override environment = development
endif

# Build with contract packages published to the NPM registry and tagged `development`.
development:
	make all environment=development

# Build with contract packages published to the NPM registry and tagged `sepolia`.
sepolia:
	make all environment=sepolia

# Build with contract packages published to the NPM registry and tagged `mainnet`.
mainnet:
	make all environment=mainnet

# Build with contract packages deployed locally.
local:
	make all environment=local

all: get_artifacts generate build release cmd-help

modules := beacon \
	ecdsa \
	threshold \
	tbtc

# Required by get_npm_package function.
npm_beacon_package := @keep-network/random-beacon
npm_ecdsa_package := @keep-network/ecdsa
npm_threshold_package := @threshold-network/solidity-contracts
npm_tbtc_package := @keep-network/tbtc-v2

# Required by get_local_package function. The paths can be overwritten when calling
# the make command, e.g.:
#   make local local_threshold_path=/other/path/threshold
local_beacon_path := ./solidity/random-beacon
local_ecdsa_path := ./solidity/ecdsa
local_threshold_path := ../../threshold-network/solidity-contracts
local_tbtc_path := ../tbtc-v2/solidity

# Working directory where contracts artifacts should be stored.
contracts_dir := tmp/contracts

# It requires npm of at least 7.x version to support `pack-destination` flag.
define get_npm_package
$(eval npm_package_name := $(npm_$(1)_package))
$(eval npm_package_tag := $(2))
$(eval destination_dir := ${contracts_dir}/${npm_package_tag}/${npm_package_name})
@rm -rf ${destination_dir}
@mkdir -p ${destination_dir}
@npm pack --silent \
	--pack-destination=${destination_dir} \
	$(shell npm view ${npm_package_name}@${npm_package_tag} _id) \
	| xargs -I{} tar -zxf ${destination_dir}/{} -C ${destination_dir} --strip-components 1 package/artifacts
$(info Downloaded NPM package ${npm_package_name}@${npm_package_tag} to ${contracts_dir})
endef

define get_local_package
$(eval module := $(1))
$(eval local_solidity_path := $(local_$(module)_path))
$(eval npm_package_name := $(npm_$(module)_package))
$(eval destination_dir := ${contracts_dir}/local/${npm_package_name})
@[ -d "$(local_solidity_path)" ] || { echo "$(module) path [$(local_solidity_path)] does not exist!"; exit 1; }
@rm -rf ${destination_dir}
@mkdir -p ${destination_dir}
$(info Fetching local package ${module} from path ${local_solidity_path})
rsync -a $(local_solidity_path)/deployments/development/ ${destination_dir}/artifacts
endef

get_artifacts:
ifeq ($(environment), local)
	$(foreach module,$(modules),$(call get_local_package,$(module)))
else
	$(foreach module,$(modules),$(call get_npm_package,$(module),$(environment)))
endif

proto_files := $(shell find ./pkg -name '*.proto')
proto_targets := $(proto_files:.proto=.pb.go)

gen_proto: ${proto_targets}

%.pb.go: %.proto go.mod go.sum
	protoc --go_out=. --go_opt=paths=source_relative $*.proto

generate: gen_proto
	$(info Running Go code generator)
	go generate ./...

# Default parameters for client building. They can be overriten when calling the
# make command.
ifndef version
override version = $(shell git describe --tags --match "v[0-9]*" HEAD)
endif
ifndef revision
override revision = $(shell git rev-parse --short HEAD)
endif

app_name := keep-client

define go_build_cmd
	$(eval out_dir := $(1))
	$(eval file_name := $(2))
	$(eval os := $(3))
	$(eval arch := $(4))

	GOOS=$(os) GOARCH=$(arch) go build \
		-ldflags "-X github.com/keep-network/keep-core/build.Version=$(version) -X github.com/keep-network/keep-core/build.Revision=$(revision) -checklinkname=0" \
		-o $(out_dir)/$(file_name) \
		-a \
		.
endef

define go_build_platform_cmd
	$(eval os := $(firstword $(subst /, ,$(1))))
	$(eval arch := $(lastword $(subst /, ,$(1))))
	$(eval file_name := $(app_name)-$(environment)-$(version)-$(os)-$(arch))

	$(call go_build_cmd,out/bin,$(file_name),$(os),$(arch))
endef

build:
	$(info Building Go code)
	$(call go_build_cmd,.,$(app_name))

platforms := linux/amd64 \
	darwin/amd64

build_multi:
	$(info Building client binaries for multiple platforms)
	$(foreach platform,$(platforms),$(call go_build_platform_cmd,$(platform)))

out/bin/%.tar.gz:
	cd $(@D) && cp $* keep-client && tar -czvf $*.tar.gz keep-client && rm $* keep-client
	cd $(@D) && md5sum $*.tar.gz > $*.md5
	cd $(@D) && sha256sum $*.tar.gz > $*.sha256

binaries := $(addprefix out/bin/$(app_name)-$(environment)-$(version)-, $(subst /,-,$(platforms)))
build_packages = $(addsuffix .tar.gz,$(binaries))

release: build_multi $(build_packages)

cmd-help: build
	@echo '$$ $(app_name) start --help' > docs/resources/client-start-help
	./$(app_name) start --help >> docs/resources/client-start-help

.PHONY: all development sepolia download_artifacts generate gen_proto build cmd-help release build_multi
