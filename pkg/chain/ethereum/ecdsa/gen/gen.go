package gen

import (
	_ "embed"
	"strings"
)

//go:generate make

var (
	// NOTE: The _address/WalletRegistry file is an empty placeholder committed to the repository
	// to satisfy this go:embed directive during CI builds (go vet, staticcheck) that don't run
	// go generate. The file gets populated with actual contract addresses during go generate.
	//go:embed _address/WalletRegistry
	walletRegistryAddressFileContent string

	// WalletRegistryAddress is a WalletRegistry contract's address read from the NPM package.
	WalletRegistryAddress string = strings.TrimSpace(walletRegistryAddressFileContent)
)
