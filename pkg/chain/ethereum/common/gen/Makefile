ifndef environment
override environment = development
endif

root_dir := $(realpath ../../../../..)
artifacts_dir := ${root_dir}/tmp/contracts/${environment}/${npm_package_name}/artifacts

$(info Package              ${npm_package_name})
$(info Environment          ${environment})
$(info Artifacts directory  ${artifacts_dir})

# Go bindings generated for the solidity contracts.
contract_files = $(addprefix contract/,$(addsuffix .go,${required_contracts}))

# Files containing addresses extracted from the artifacts.
address_files = $(addprefix _address/,${required_contracts})

all: clean check_artifacts ${address_files} gen_contract_go

clean:
	$(info Cleaning up for ${npm_package_name})
	@rm -rf _address/*
	@rm -rf abi/*
	@rm -rf contract/*
	@mkdir tmp && mv cmd/cmd*.go tmp
	@rm -rf cmd/*
	@mv tmp/* cmd && rm -rf tmp

# Check if artifacts directory exists. If the directory doesn't exists it is
# likely a problem inside a fetched contracts package.
check_artifacts:
	@[ -d "$(artifacts_dir)" ] || { echo "$(artifacts_dir) does not exist!"; exit 1; }

gen_contract_go: ${contract_files}

abi/%.abi: ${artifacts_dir}/%.json
	$(info $* - generating ABI)
	@jq .abi $< > abi/$*.abi

abi/%.go: abi/%.abi
	$(info $* - generating Ethereum bindings)
	@go run -ldflags=-checklinkname=0 github.com/ethereum/go-ethereum/cmd/abigen --abi $< --pkg abi --type $* --out $@
	$(call after_abi_hook,$*)

# Extract address of a contract from an artifact. Artfifacts that are published
# with `development` tag are not accessible on developers' environments, so we
# replace them with zeros.
_address/%: ${artifacts_dir}/%.json
ifeq ($(environment), development)
	$(info $* - skipping address extraction for development package)
	@echo "0x0000000000000000000000000000000000000000" > _address/$*
else
	$(info $* - extracting address)
	@jq -jr .address ${artifacts_dir}/$*.json > _address/$*
endif

contract/%.go cmd/%.go: abi/%.abi abi/%.go _address/% ${artifacts_dir}/%.json
	$(info $* - generating Keep bindings)
	@go run github.com/keep-network/keep-common/tools/generators/ethereum $< contract/$*.go cmd/$*.go
	$(call after_contract_hook,$*)

# Don't remove intermediate files that got generated.
.PRECIOUS: abi/%.abi abi/%.go _address/%
