package gen

import (
	_ "embed"
	"strings"
)

//go:generate make

var (
	// NOTE: The _address/RandomBeacon file is an empty placeholder committed to the repository
	// to satisfy this go:embed directive during CI builds (go vet, staticcheck) that don't run
	// go generate. The file gets populated with actual contract addresses during go generate.
	//go:embed _address/RandomBeacon
	randomBeaconAddressFileContent string

	// RandomBeaconAddress is a Random Beacon contract's address read from the NPM package.
	RandomBeaconAddress string = strings.TrimSpace(randomBeaconAddressFileContent)
)
