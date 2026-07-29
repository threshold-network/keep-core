package ethereum

import (
	"context"
	"fmt"
	"strings"
	"testing"

	geth "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	commonethereum "github.com/keep-network/keep-common/pkg/chain/ethereum"
	"github.com/keep-network/keep-common/pkg/rate"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

func testFrostHistoricalContract(
	t *testing.T,
) frostPreSignManifestContract {
	t.Helper()
	descriptorHash, err := frostPreSignLinkedLibraryInventoryHash(nil)
	if err != nil {
		t.Fatal(err)
	}
	address := strings.ToLower(common.HexToAddress(
		"0x1111111111111111111111111111111111111111",
	).Hex())
	runtimeHash := common.HexToHash("0x010203").Hex()
	startHash := common.HexToHash("0x0a").Hex()
	return frostPreSignManifestContract{
		Address:                     address,
		RuntimeCodeHash:             runtimeHash,
		ProtocolID:                  common.HexToHash("0x20").Hex(),
		DeploymentBlock:             10,
		RelevantEventStartBlock:     10,
		LinkedLibraryDescriptorHash: common.Hash(descriptorHash).Hex(),
		LinkedLibraries:             []frostPreSignManifestLinkedLibrary{},
		Upgradeability: frostPreSignManifestUpgradeability{
			Kind: "immutable",
		},
		HistoricalDeploymentEpochs: []frostPreSignManifestDeploymentEpoch{{
			Start: frostPreSignManifestPoint{
				BlockNumber: 10,
				BlockHash:   startHash,
			},
			Address:                     address,
			RuntimeCodeHash:             runtimeHash,
			LinkedLibraryDescriptorHash: common.Hash(descriptorHash).Hex(),
			LinkedLibraries:             []frostPreSignManifestLinkedLibrary{},
			Upgradeability: frostPreSignManifestUpgradeability{
				Kind: "immutable",
			},
		}},
	}
}

func TestFrostPreSignDeploymentPinFromManifest_HistoricalEpochs(t *testing.T) {
	contract := testFrostHistoricalContract(t)
	pin, err := frostPreSignDeploymentPinFromManifest(
		"bridge",
		"Bridge",
		contract,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(pin.historicalEpochs) != 1 ||
		pin.historicalEpochs[0].start.BlockNumber != 10 ||
		pin.historicalEpochs[0].end != nil ||
		frostPreSignDeploymentDescriptorHash(pin) !=
			frostPreSignDeploymentDescriptorHash(
				pin.historicalEpochs[0].descriptor,
			) {
		t.Fatal("historical deployment epoch was not preserved exactly")
	}
	runtime := frostPreSignRuntimeDeploymentEvidence(
		[]frostPreSignDeploymentPin{pin},
	)
	if len(runtime) != 1 ||
		tbtc.ComputeFrostPreSignDeploymentEvidenceHash(runtime) !=
			frostPreSignDeploymentSetHash([]frostPreSignDeploymentPin{pin}) {
		t.Fatal("runtime historical evidence changes the deployment-set commitment")
	}
}

func TestFrostPreSignDeploymentPinFromManifest_RejectsInvalidEpochRanges(
	t *testing.T,
) {
	tests := map[string]func(*frostPreSignManifestContract){
		"missing": func(contract *frostPreSignManifestContract) {
			contract.HistoricalDeploymentEpochs = nil
		},
		"first start differs from deployment": func(contract *frostPreSignManifestContract) {
			contract.HistoricalDeploymentEpochs[0].Start.BlockNumber++
		},
		"final epoch is closed": func(contract *frostPreSignManifestContract) {
			contract.HistoricalDeploymentEpochs[0].End =
				&frostPreSignManifestPoint{
					BlockNumber: 11,
					BlockHash:   common.HexToHash("0x0b").Hex(),
				}
		},
		"gap": func(contract *frostPreSignManifestContract) {
			first := contract.HistoricalDeploymentEpochs[0]
			first.End = &frostPreSignManifestPoint{
				BlockNumber: 11,
				BlockHash:   common.HexToHash("0x0b").Hex(),
			}
			second := first
			second.Start = frostPreSignManifestPoint{
				BlockNumber: 13,
				BlockHash:   common.HexToHash("0x0d").Hex(),
			}
			second.End = nil
			contract.HistoricalDeploymentEpochs =
				[]frostPreSignManifestDeploymentEpoch{first, second}
		},
		"overlap": func(contract *frostPreSignManifestContract) {
			first := contract.HistoricalDeploymentEpochs[0]
			first.End = &frostPreSignManifestPoint{
				BlockNumber: 12,
				BlockHash:   common.HexToHash("0x0c").Hex(),
			}
			second := first
			second.Start = frostPreSignManifestPoint{
				BlockNumber: 12,
				BlockHash:   common.HexToHash("0x1c").Hex(),
			}
			second.End = nil
			contract.HistoricalDeploymentEpochs =
				[]frostPreSignManifestDeploymentEpoch{first, second}
		},
		"current descriptor mismatch": func(contract *frostPreSignManifestContract) {
			contract.HistoricalDeploymentEpochs[0].RuntimeCodeHash =
				common.HexToHash("0xff").Hex()
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			contract := testFrostHistoricalContract(t)
			mutate(&contract)
			if _, err := frostPreSignDeploymentPinFromManifest(
				"bridge",
				"Bridge",
				contract,
			); err == nil {
				t.Fatal("invalid historical deployment epochs were accepted")
			}
		})
	}
}

type testFrostCanonicalRPCAPI struct {
	points []rpc.BlockNumberOrHash
}

func (api *testFrostCanonicalRPCAPI) GetCode(
	_ context.Context,
	_ common.Address,
	point rpc.BlockNumberOrHash,
) (hexutil.Bytes, error) {
	api.points = append(api.points, point)
	return hexutil.Bytes{0x01}, nil
}

func (api *testFrostCanonicalRPCAPI) GetStorageAt(
	_ context.Context,
	_ common.Address,
	_ common.Hash,
	point rpc.BlockNumberOrHash,
) (hexutil.Bytes, error) {
	api.points = append(api.points, point)
	return make(hexutil.Bytes, 32), nil
}

func (api *testFrostCanonicalRPCAPI) Call(
	_ context.Context,
	_ map[string]interface{},
	point rpc.BlockNumberOrHash,
) (hexutil.Bytes, error) {
	api.points = append(api.points, point)
	return make(hexutil.Bytes, 32), nil
}

type testFrostHeaderByHashReader struct{}

func (*testFrostHeaderByHashReader) HeaderByHash(
	_ context.Context,
	hash common.Hash,
) (*types.Header, error) {
	if hash == (common.Hash{}) {
		return nil, fmt.Errorf("zero hash")
	}
	return &types.Header{}, nil
}

func TestFrostPreSignCanonicalHashReader_RequiresCanonicalEIP1898State(
	t *testing.T,
) {
	server := rpc.NewServer()
	api := &testFrostCanonicalRPCAPI{}
	if err := server.RegisterName("eth", api); err != nil {
		t.Fatal(err)
	}
	client := rpc.DialInProc(server)
	defer client.Close()
	reader := &frostPreSignCanonicalHashReader{
		headerReader: &testFrostHeaderByHashReader{},
		rpcClient:    client,
	}
	blockHash := common.HexToHash("0x1234")
	address := common.HexToAddress(
		"0x1111111111111111111111111111111111111111",
	)
	if _, err := reader.CodeAtHash(
		context.Background(),
		address,
		blockHash,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.StorageAtHash(
		context.Background(),
		address,
		common.Hash{},
		blockHash,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.CallContractAtHash(
		context.Background(),
		geth.CallMsg{To: &address, Data: []byte{0x01}},
		blockHash,
	); err != nil {
		t.Fatal(err)
	}
	if len(api.points) != 3 {
		t.Fatalf("unexpected exact-hash call count [%d]", len(api.points))
	}
	for _, point := range api.points {
		if point.BlockHash == nil || *point.BlockHash != blockHash ||
			!point.RequireCanonical || point.BlockNumber != nil {
			t.Fatalf("state read did not require canonical hash [%+v]", point)
		}
	}
}

func TestFrostPreSignExactHashReader_WrappedProductionClient(t *testing.T) {
	server := rpc.NewServer()
	api := &testFrostCanonicalRPCAPI{}
	if err := server.RegisterName("eth", api); err != nil {
		t.Fatal(err)
	}
	rpcClient := rpc.DialInProc(server)
	defer rpcClient.Close()
	client := ethclient.NewClient(rpcClient)

	config := commonethereum.Config{
		RequestsPerSecondLimit: 10,
		ConcurrencyLimit:       2,
	}
	chain := &baseChain{
		client:    wrapClientAddons(config, client),
		rpcClient: client.Client(),
		rpcLimiter: rate.NewLimiter(&rate.LimiterConfig{
			RequestsPerSecondLimit: config.RequestsPerSecondLimit,
			ConcurrencyLimit:       config.ConcurrencyLimit,
		}),
	}
	adapter := &frostPreSignEthereumAdapter{
		chain: &TbtcChain{baseChain: chain},
	}

	reader, err := adapter.exactHashReader()
	if err != nil {
		t.Fatalf("wrapped production client rejected: %v", err)
	}
	blockHash := common.HexToHash("0x1234")
	address := common.HexToAddress(
		"0x1111111111111111111111111111111111111111",
	)
	if _, err := reader.CodeAtHash(
		context.Background(),
		address,
		blockHash,
	); err != nil {
		t.Fatal(err)
	}
	if len(api.points) != 1 || api.points[0].BlockHash == nil ||
		*api.points[0].BlockHash != blockHash ||
		!api.points[0].RequireCanonical {
		t.Fatalf("unexpected exact-hash RPC point [%+v]", api.points)
	}
}
