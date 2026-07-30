package ethereum

import (
	"context"
	"fmt"
	"math/big"
	"testing"

	geth "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	frostabi "github.com/keep-network/keep-core/pkg/chain/ethereum/frost/gen/abi"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

func TestTbtcChainFrostWalletRegistryAvailable(t *testing.T) {
	chainWithoutFrostRegistry := &TbtcChain{}
	if chainWithoutFrostRegistry.FrostWalletRegistryAvailable() {
		t.Fatal("expected FROST wallet registry to be unavailable")
	}

	chainWithFrostRegistry := &TbtcChain{
		frostWalletRegistry: &frostabi.FrostWalletRegistry{},
	}
	if !chainWithFrostRegistry.FrostWalletRegistryAvailable() {
		t.Fatal("expected FROST wallet registry to be available")
	}
}

type frostDKGRetirementSnapshotTestReader struct {
	*testFrostPreSignEvidenceReader
	registryAddress common.Address
	registered      map[[32]byte]bool
	callHashes      []common.Hash
}

func (reader *frostDKGRetirementSnapshotTestReader) CallContractAtHash(
	_ context.Context,
	message geth.CallMsg,
	blockHash common.Hash,
) ([]byte, error) {
	if message.To == nil || *message.To != reader.registryAddress {
		return nil, fmt.Errorf("unexpected retirement snapshot contract")
	}
	registryABI, err := frostabi.FrostWalletRegistryMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	method, err := registryABI.MethodById(message.Data[:4])
	if err != nil {
		return nil, err
	}
	reader.callHashes = append(reader.callHashes, blockHash)
	switch method.Name {
	case "getWalletCreationState":
		return method.Outputs.Pack(uint8(tbtc.Challenge))
	case "isWalletRegistered":
		values, err := method.Inputs.Unpack(message.Data[4:])
		if err != nil || len(values) != 1 {
			return nil, fmt.Errorf("cannot decode retirement wallet ID")
		}
		walletID, ok := values[0].([32]byte)
		if !ok {
			return nil, fmt.Errorf("retirement wallet ID has an invalid type")
		}
		return method.Outputs.Pack(reader.registered[walletID])
	default:
		return nil, fmt.Errorf("unexpected retirement snapshot method [%s]", method.Name)
	}
}

func TestFrostDKGRetirementSnapshotPinsEveryPredicateToOnePoint(
	t *testing.T,
) {
	header := &types.Header{
		Number: big.NewInt(100),
		Time:   100,
		Extra:  []byte{0xaa},
	}
	registryAddress := common.HexToAddress(
		"0x0000000000000000000000000000000000000011",
	)
	registeredWalletID := [32]byte{2}
	reader := &frostDKGRetirementSnapshotTestReader{
		testFrostPreSignEvidenceReader: &testFrostPreSignEvidenceReader{
			finalized: header,
		},
		registryAddress: registryAddress,
		registered: map[[32]byte]bool{
			registeredWalletID: true,
		},
	}
	chain := &TbtcChain{
		frostWalletRegistryAddr: registryAddress,
		frostPreSignAuthorizationAdapter: &frostPreSignEthereumAdapter{
			reader: reader,
		},
		frostPreSignAuthorizationVerifier: &frostPreSignEthereumAdapter{
			reader: reader,
		},
	}
	point := tbtc.FrostPreSignFinality{
		BlockNumber: header.Number.Uint64(),
		BlockHash:   header.Hash(),
	}
	snapshot, err := chain.FrostDKGRetirementSnapshot(
		context.Background(),
		point,
		[][32]byte{{1}, registeredWalletID},
	)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Point != point || snapshot.State != tbtc.Challenge ||
		snapshot.RegisteredWallets[[32]byte{1}] ||
		!snapshot.RegisteredWallets[registeredWalletID] {
		t.Fatalf("unexpected retirement snapshot: [%+v]", snapshot)
	}
	if len(reader.callHashes) != 3 {
		t.Fatalf("unexpected exact-hash call count: [%d]", len(reader.callHashes))
	}
	for _, callHash := range reader.callHashes {
		if callHash != common.Hash(point.BlockHash) {
			t.Fatalf("retirement predicate used a different block: [%s]", callHash)
		}
	}
}
