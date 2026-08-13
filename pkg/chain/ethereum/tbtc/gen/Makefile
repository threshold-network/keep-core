npm_package_name=@keep-network/tbtc-v2

# Contracts for which the bindings should be generated.
required_contracts := Bridge MaintainerProxy LightRelay LightRelayMaintainerProxy WalletProposalValidator RedemptionWatchtower

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

# See explanation in https://github.com/keep-network/keep-common/issues/117.
define after_contract_hook
	$(eval type := $(1))
	$(if $(filter $(type),WalletProposalValidator),$(call fix_wallet_proposal_validator_contract_collision))
	$(if $(filter $(type),MaintainerProxy),$(call fix_maintainer_proxy_contract_collision))
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

include ../../common/gen/Makefile
