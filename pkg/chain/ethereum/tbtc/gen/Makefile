npm_package_name=@keep-network/tbtc-v2

# Contracts for which the bindings should be generated.
required_contracts := Bridge MaintainerProxy LightRelay LightRelayMaintainerProxy WalletProposalValidator RedemptionWatchtower ReservationRouter

# There is a bug in the currently used abigen version (v1.10.19) that makes it
# re-declaring structs used by multiple contracts
# https://github.com/ethereum/go-ethereum/issues/24627. This is a problem
# for us because Bridge, WalletProposalValidator and MaintainerProxy contracts all use
# the same structs which are then re-declared in the same package once abigen
# does its work.
# An ultimate solution would be upgrading go-ethereum (thus abigen too) to v1.11 but
# that version contains some breaking changes that make the upgrade non-trivial.
# As a short-term workaround, we use some Makefile shenanigans to slightly rename
# the conflicting structs in the WalletProposalValidator and MaintainerProxy output files.
# We use perl for that purpose as sed is not cross-platform and works a bit
# differently on GNU and BSD.
#
# TODO: Remove once go-ethereum is upgraded to v1.11. See issue:
#       https://github.com/threshold-network/keep-core/issues/3524
define after_abi_hook
	$(eval type := $(1))
	$(if $(filter $(type),WalletProposalValidator),$(call fix_wallet_proposal_validator_collision))
	$(if $(filter $(type),MaintainerProxy),$(call fix_maintainer_proxy_collision))
	$(if $(filter $(type),ReservationRouter),$(call fix_reservation_router_collision))
endef
define fix_wallet_proposal_validator_collision
	@perl -pi -e s,BitcoinTxInfo,BitcoinTxInfo2,g ./abi/WalletProposalValidator.go
	@perl -pi -e s,BitcoinTxUTXO,BitcoinTxUTXO3,g ./abi/WalletProposalValidator.go
endef
define fix_maintainer_proxy_collision
	@perl -pi -e s,BitcoinTxUTXO,BitcoinTxUTXO2,g ./abi/MaintainerProxy.go
	@perl -pi -e s,BitcoinTxProof,BitcoinTxProof2,g ./abi/MaintainerProxy.go
	@perl -pi -e s,BitcoinTxInfo,BitcoinTxInfo3,g ./abi/MaintainerProxy.go
endef
# ReservationRouter introduces its own copies of the BitcoinTx.* structs
# (BitcoinTxInfo, BitcoinTxProof, BitcoinTxUTXO) used by submitReservationProof.
# These names already collide with Bridge's (BitcoinTxInfo/BitcoinTxProof/BitcoinTxUTXO),
# WalletProposalValidator's (BitcoinTxInfo2/BitcoinTxUTXO3), and MaintainerProxy's
# (BitcoinTxInfo3/BitcoinTxProof2/BitcoinTxUTXO2). Renumber ReservationRouter's
# copies to the next free suffix in each family. The router code only ever runs
# via Bridge.fallback delegatecall, so its storage pointer is unused; this is
# a purely textual rename to keep the abi package compiling.
define fix_reservation_router_collision
	@perl -pi -e s,BitcoinTxInfo,BitcoinTxInfo4,g ./abi/ReservationRouter.go
	@perl -pi -e s,BitcoinTxProof,BitcoinTxProof3,g ./abi/ReservationRouter.go
	@perl -pi -e s,BitcoinTxUTXO,BitcoinTxUTXO4,g ./abi/ReservationRouter.go
endef

# See explanation in https://github.com/keep-network/keep-common/issues/117.
define after_contract_hook
	$(eval type := $(1))
	$(if $(filter $(type),WalletProposalValidator),$(call fix_wallet_proposal_validator_contract_collision))
	$(if $(filter $(type),MaintainerProxy),$(call fix_maintainer_proxy_contract_collision))
	$(if $(filter $(type),ReservationRouter),$(call fix_reservation_router_contract_collision))
endef
define fix_wallet_proposal_validator_contract_collision
	@perl -pi -e s,BitcoinTxUTXO,BitcoinTxUTXO3,g ./contract/WalletProposalValidator.go
	@perl -pi -e s,BitcoinTxUTXO,BitcoinTxUTXO3,g ./cmd/WalletProposalValidator.go
endef
define fix_maintainer_proxy_contract_collision
	@perl -pi -e s,BitcoinTxUTXO,BitcoinTxUTXO2,g ./contract/MaintainerProxy.go
	@perl -pi -e s,BitcoinTxProof,BitcoinTxProof2,g ./contract/MaintainerProxy.go
	@perl -pi -e s,BitcoinTxInfo,BitcoinTxInfo3,g ./contract/MaintainerProxy.go
	@perl -pi -e s,BitcoinTxUTXO,BitcoinTxUTXO2,g ./cmd/MaintainerProxy.go
	@perl -pi -e s,BitcoinTxProof,BitcoinTxProof2,g ./cmd/MaintainerProxy.go
	@perl -pi -e s,BitcoinTxInfo,BitcoinTxInfo3,g ./cmd/MaintainerProxy.go
endef
# keep-common's generator emits BitcoinTx* references inside the generated
# contract/cmd binding code paths too (e.g. method wrappers). Apply the same
# renames there so the contract and cmd packages compile.
define fix_reservation_router_contract_collision
	@perl -pi -e s,BitcoinTxInfo,BitcoinTxInfo4,g ./contract/ReservationRouter.go
	@perl -pi -e s,BitcoinTxProof,BitcoinTxProof3,g ./contract/ReservationRouter.go
	@perl -pi -e s,BitcoinTxUTXO,BitcoinTxUTXO4,g ./contract/ReservationRouter.go
	@perl -pi -e s,BitcoinTxInfo,BitcoinTxInfo4,g ./cmd/ReservationRouter.go
	@perl -pi -e s,BitcoinTxProof,BitcoinTxProof3,g ./cmd/ReservationRouter.go
	@perl -pi -e s,BitcoinTxUTXO,BitcoinTxUTXO4,g ./cmd/ReservationRouter.go
endef

include ../../common/gen/Makefile

# @keep-network/tbtc-v2@development on npm does not yet publish
# ReservationRouter.json (threshold-network/keep-core#4281), which makes
# `make generate` fail outright since nothing else can produce that
# prerequisite. Fall back to a vendored copy - its ABI is byte-for-byte
# the ABI already embedded in the committed bindings (re-derived from
# ReservationRouterMetaData.ABI, with the "struct"/"enum"/"contract"
# internalType prefix space abigen's metadata packer strips put back;
# verified by round-tripping through the same abigen + keep-common
# generator invocation and diffing byte-identical against
# abi/ReservationRouter.go, contract/ReservationRouter.go, and
# cmd/ReservationRouter.go) - only when the real artifact is missing,
# and only in the `development` environment, which already tolerates
# placeholder addresses (see the _address/% rule above). Non-development
# builds still hard-fail if the real artifact is ever missing there,
# since a real deployed address must never be substituted silently.
# Remove this rule once tbtc-v2 publishes the real artifact upstream.
${artifacts_dir}/ReservationRouter.json:
ifeq ($(environment), development)
	@[ -f "$@" ] || { \
		echo "ReservationRouter - artifact missing from ${npm_package_name}@${environment}, using vendored fallback (see threshold-network/keep-core#4281)"; \
		cp ReservationRouter.fallback-artifact.json "$@"; \
	}
else
	@[ -f "$@" ] || { echo "$@ does not exist!"; exit 1; }
endif

# @keep-network/tbtc-v2@development on npm publishes Bridge.json and
# WalletProposalValidator.json, but both are stale relative to the
# reservation feature: they're missing isReservedDeposit (Bridge) and
# validateReservationAnchorProposal/validateReservationReanchorProposal
# (WalletProposalValidator), which the committed bindings already call
# (threshold-network/keep-core#4281). Unlike ReservationRouter.json,
# these files exist, so an only-if-missing artifact rule can't apply -
# patch the fetched artifact in place instead, merging in vendored
# fragments (extracted from the committed BridgeMetaData.ABI /
# WalletProposalValidatorMetaData.ABI, internalType prefix space
# restored the same way as ReservationRouter's; verified by
# round-tripping through the same abigen + keep-common generator
# invocation, producing a clean `go build ./...`) before anything reads
# the artifact. Only in `development`; only when the methods are
# actually missing, so a real future npm publish makes this a no-op
# without needing to be removed first. Non-development builds are
# untouched. Remove this whole block once tbtc-v2 publishes the real
# methods upstream.
.PHONY: patch-artifacts
check_artifacts: patch-artifacts
patch-artifacts:
ifeq ($(environment), development)
	@jq -e '.abi[] | select(.name == "isReservedDeposit")' ${artifacts_dir}/Bridge.json >/dev/null 2>&1 || { \
		echo "Bridge - artifact missing reservation methods, patching in vendored fallback (see threshold-network/keep-core#4281)"; \
		jq --slurpfile extra Bridge.reservation-methods-fallback.json '.abi += $$extra[0]' ${artifacts_dir}/Bridge.json > ${artifacts_dir}/Bridge.json.patched && \
		mv ${artifacts_dir}/Bridge.json.patched ${artifacts_dir}/Bridge.json; \
	}
	@jq -e '.abi[] | select(.name == "validateReservationAnchorProposal")' ${artifacts_dir}/WalletProposalValidator.json >/dev/null 2>&1 || { \
		echo "WalletProposalValidator - artifact missing reservation methods, patching in vendored fallback (see threshold-network/keep-core#4281)"; \
		jq --slurpfile extra WalletProposalValidator.reservation-methods-fallback.json '.abi += $$extra[0]' ${artifacts_dir}/WalletProposalValidator.json > ${artifacts_dir}/WalletProposalValidator.json.patched && \
		mv ${artifacts_dir}/WalletProposalValidator.json.patched ${artifacts_dir}/WalletProposalValidator.json; \
	}
endif
