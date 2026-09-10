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
# round-tripped through the same abigen + keep-common generator
# invocation and diffed byte-identical against abi/ReservationRouter.go,
# contract/ReservationRouter.go, and cmd/ReservationRouter.go). That
# round-trip only proves internal self-consistency with our own
# committed bindings - it is a closed loop that cannot detect the real
# on-chain contract's ABI having diverged from what's committed, which
# is exactly what happened before threshold-network/keep-core#4281 was
# fixed. Run `make verify-vendored-fallback` against a real fetched
# ReservationRouter.json whenever this vendored file is touched or
# needs re-verification; see that target below for why it can't run as
# part of this rule. This fallback only applies when the real artifact
# is missing, and only in the `development` environment, which already
# tolerates placeholder addresses (see the _address/% rule above).
# Non-development builds still hard-fail if the real artifact is ever
# missing there, since a real deployed address must never be
# substituted silently.
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

# verify-vendored-fallback checks ReservationRouter.fallback-artifact.json's ABI
# against a REAL fetched ReservationRouter.json, catching drift between the
# vendored copy and the actual on-chain contract that the round-trip check in
# the comment above (self-consistency against our own committed bindings only)
# cannot catch. It needs the real artifact to exist, which is exactly the case
# the fallback rule above exists to handle the absence of - wiring this into
# `all`/`generate` would make the common case (real artifact missing) fail the
# build. Run it manually, or in a separate CI job with npm/network access,
# whenever ReservationRouter.fallback-artifact.json is touched or needs
# re-verification. Compares the full canonicalized `.abi` array (sorted by
# function name, jq -S) rather than hand-picking fields, so it naturally
# catches missing/renamed functions, added/removed/reordered parameters, and
# type changes alike.
.PHONY: verify-vendored-fallback
verify-vendored-fallback:
	@[ -f "${artifacts_dir}/ReservationRouter.json" ] || { \
		echo "verify-vendored-fallback requires a real ${artifacts_dir}/ReservationRouter.json (fetch @keep-network/tbtc-v2 with npm/network access before running this target)"; \
		exit 1; \
	}
	@vendored=$$(mktemp) && real=$$(mktemp) && \
	jq -S '.abi | sort_by(.name)' ReservationRouter.fallback-artifact.json > "$$vendored" && \
	jq -S '.abi | sort_by(.name)' ${artifacts_dir}/ReservationRouter.json > "$$real" && \
	if diff -u "$$vendored" "$$real"; then \
		echo "ReservationRouter.fallback-artifact.json ABI matches the real artifact"; \
		rm -f "$$vendored" "$$real"; \
	else \
		echo "ReservationRouter.fallback-artifact.json ABI differs from the real artifact - regenerate the vendored fallback (see threshold-network/keep-core#4281)"; \
		rm -f "$$vendored" "$$real"; \
		exit 1; \
	fi

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
# missing, or present under the matching name but with a stale shape
# (e.g. a pre-reservation-feature isReservedDeposit/
# validateReservationAnchorProposal that happens to share a name with
# the reservation-feature method but not its parameter shape) - a
# name-only match is deliberately treated the same as missing, see the
# shape checks below - so a real future npm publish with the correct
# signatures makes this a no-op without needing to be removed first.
# Non-development builds are untouched. Remove this whole block once
# tbtc-v2 publishes the real methods upstream.
.PHONY: patch-artifacts
check_artifacts: patch-artifacts
patch-artifacts:
ifeq ($(environment), development)
	@jq -e '.abi[] | select(.name == "isReservedDeposit") | select((.inputs | length) == 1 and .inputs[0].type == "uint256" and (.outputs | length) == 1 and .outputs[0].type == "bool")' ${artifacts_dir}/Bridge.json >/dev/null 2>&1 || { \
		echo "Bridge - artifact missing reservation methods (or isReservedDeposit has a stale signature), patching in vendored fallback (see threshold-network/keep-core#4281)"; \
		jq --slurpfile extra Bridge.reservation-methods-fallback.json '.abi += $$extra[0]' ${artifacts_dir}/Bridge.json > ${artifacts_dir}/Bridge.json.patched && \
		mv ${artifacts_dir}/Bridge.json.patched ${artifacts_dir}/Bridge.json; \
	}
	@jq -e '.abi[] | select(.name == "validateReservationAnchorProposal") | select(((.inputs[0] // {}).components // []) | any(.name == "requestNonce"))' ${artifacts_dir}/WalletProposalValidator.json >/dev/null 2>&1 || { \
		echo "WalletProposalValidator - artifact missing reservation methods (or validateReservationAnchorProposal has a stale signature missing requestNonce), patching in vendored fallback (see threshold-network/keep-core#4281)"; \
		jq --slurpfile extra WalletProposalValidator.reservation-methods-fallback.json '.abi += $$extra[0]' ${artifacts_dir}/WalletProposalValidator.json > ${artifacts_dir}/WalletProposalValidator.json.patched && \
		mv ${artifacts_dir}/WalletProposalValidator.json.patched ${artifacts_dir}/WalletProposalValidator.json; \
	}
endif
