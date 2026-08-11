//go:build !frost_native

package tbtc

import (
	"fmt"

	"github.com/keep-network/keep-core/pkg/chain"
)

func runFrostShareRepairTransportPreflight(
	authorizationPath string,
	outputPath string,
	_ FrostPreSignActivationRuntimeManifest,
	_ *node,
	_ chain.Address,
) (bool, error) {
	if authorizationPath == "" && outputPath == "" {
		return false, nil
	}
	return true, fmt.Errorf("share-repair transport preflight requires the frost_native build")
}
