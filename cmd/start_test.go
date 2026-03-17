package cmd

import (
	"context"
	"errors"
	"strings"
	"testing"

	commonEthereum "github.com/keep-network/keep-common/pkg/chain/ethereum"
	"github.com/keep-network/keep-core/config"
	"github.com/keep-network/keep-core/pkg/chain"
	chainEthereum "github.com/keep-network/keep-core/pkg/chain/ethereum"
	"github.com/keep-network/keep-core/pkg/firewall"
	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/operator"
	"github.com/spf13/cobra"
)

func TestStartFailsFastWhenEthereumConnectFails(t *testing.T) {
	originalConfig := *clientConfig

	t.Cleanup(func() {
		*clientConfig = originalConfig
	})

	*clientConfig = config.Config{}
	networkInitCalled := false

	deps := defaultStartDeps()
	deps.connectEthereum = func(
		_ context.Context,
		_ commonEthereum.Config,
	) (
		*chainEthereum.BeaconChain,
		*chainEthereum.TbtcChain,
		chain.BlockCounter,
		chain.Signing,
		*operator.PrivateKey,
		error,
	) {
		return nil, nil, nil, nil, nil, errors.New("injected ethereum failure")
	}
	deps.initializeNetwork = func(
		_ context.Context,
		_ []firewall.Application,
		_ *operator.PrivateKey,
		_ chain.BlockCounter,
	) (net.Provider, error) {
		networkInitCalled = true
		return nil, nil
	}

	err := startWithDeps(&cobra.Command{}, deps)
	if err == nil || !strings.Contains(err.Error(), "error connecting to Ethereum node") {
		t.Fatalf("expected ethereum connection failure, got: %v", err)
	}
	if networkInitCalled {
		t.Fatal("expected network initialization not to run after ethereum connection failure")
	}
}

func TestStartFailsFastWhenNetworkInitializationFails(t *testing.T) {
	originalConfig := *clientConfig

	t.Cleanup(func() {
		*clientConfig = originalConfig
	})

	*clientConfig = config.Config{}
	deps := defaultStartDeps()
	deps.connectEthereum = func(
		_ context.Context,
		_ commonEthereum.Config,
	) (
		*chainEthereum.BeaconChain,
		*chainEthereum.TbtcChain,
		chain.BlockCounter,
		chain.Signing,
		*operator.PrivateKey,
		error,
	) {
		return nil, nil, nil, nil, nil, nil
	}

	deps.initializeNetwork = func(
		_ context.Context,
		_ []firewall.Application,
		_ *operator.PrivateKey,
		_ chain.BlockCounter,
	) (net.Provider, error) {
		return nil, errors.New("injected network initialization failure")
	}

	err := startWithDeps(&cobra.Command{}, deps)
	if err == nil || !strings.Contains(err.Error(), "cannot initialize network") {
		t.Fatalf("expected network initialization failure, got: %v", err)
	}
}
