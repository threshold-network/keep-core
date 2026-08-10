//go:build !frost_native

package tbtc

import (
	"context"
	"fmt"
	"time"

	"github.com/keep-network/keep-core/pkg/chain"
)

func runFrostShareRepairMaintenance(
	_ context.Context,
	authorizationPath string,
	_ time.Duration,
	_ FrostPreSignActivationRuntimeManifest,
	_ *node,
	_ chain.Address,
) (bool, error) {
	if authorizationPath == "" {
		return false, nil
	}
	return true, fmt.Errorf("share-repair maintenance requires the frost_native build")
}
