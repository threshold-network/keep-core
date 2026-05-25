package gen

var (
	// FrostWalletRegistryAddress is zero for development builds. Operators must
	// configure the deployed registry address explicitly until the FROST
	// registry artifact is published with network addresses.
	FrostWalletRegistryAddress = "0x0000000000000000000000000000000000000000"

	// FrostDkgValidatorAddress is zero for development builds. It is optional
	// for runtime challenge checks, which use FrostWalletRegistry.isDkgResultValid,
	// but can be configured for pre-submit resultDigest sanity checks.
	FrostDkgValidatorAddress = "0x0000000000000000000000000000000000000000"
)
