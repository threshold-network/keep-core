package gen

import (
	_ "embed"
	"strings"
)

//go:generate make

var (
	// NOTE: The _address/* files are empty placeholders committed to the repository
	// to satisfy go:embed directives during CI builds (go vet, staticcheck) that don't run
	// go generate. The files get populated with actual contract addresses during go generate.

	//go:embed _address/Bridge
	bridgeAddressFileContent string

	// BridgeAddress is a Bridge contract's address read from the NPM package.
	BridgeAddress string = strings.TrimSpace(bridgeAddressFileContent)

	//go:embed _address/MaintainerProxy
	maintainerProxyAddressFileContent string

	// MaintainerProxyAddress is a MaintainerProxy contract's address read from
	// the NPM package.
	MaintainerProxyAddress string = strings.TrimSpace(
		maintainerProxyAddressFileContent,
	)

	//go:embed _address/LightRelay
	lightRelayAddressFileContent string

	// LightRelayAddress is a LightRelay contract's address read from the NPM
	// package.
	LightRelayAddress string = strings.TrimSpace(lightRelayAddressFileContent)

	//go:embed _address/LightRelayMaintainerProxy
	lightRelayMaintainerProxyAddressFileContent string

	// LightRelayMaintainerProxyAddress is a LightRelayMaintainerProxy contract's
	// address read from the NPM package.
	LightRelayMaintainerProxyAddress string = strings.TrimSpace(
		lightRelayMaintainerProxyAddressFileContent,
	)

	//go:embed _address/WalletProposalValidator
	walletProposalValidatorAddressFileContent string

	// WalletProposalValidatorAddress is a WalletProposalValidator contract's address read from the NPM package.
	WalletProposalValidatorAddress string = strings.TrimSpace(walletProposalValidatorAddressFileContent)
)
