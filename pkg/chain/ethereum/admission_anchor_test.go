package ethereum

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/keep-network/keep-core/internal/ethtest"
	"github.com/keep-network/keep-core/internal/testutils"
)

// anchorBlock identifies one block by what the endpoint reports for it.
type anchorBlock struct {
	number uint64
	hash   string
}

// readAnchorBlock reads a block's own number and hash as the endpoint reports
// them.
//
// The hash is read rather than derived. Deriving it means decoding the block
// into the header type this module pins and hashing that, which only agrees
// with the chain while the pinned type models every field the header carries.
// It does not: a header from a fork later than the pinned release holds fields
// the type drops on the way in, so the derived hash is the hash of a different
// header. Reading the reported hash asks the endpoint which block it served
// instead of assuming this side can recompute it.
//
// A block the endpoint does not have, or one answering for another height, is
// an error rather than a value to compare: both mean the read that follows
// would be taken against unknown state.
func readAnchorBlock(
	ctx context.Context,
	client *rpc.Client,
	number uint64,
) (anchorBlock, error) {
	var payload json.RawMessage

	err := client.CallContext(
		ctx,
		&payload,
		"eth_getBlockByNumber",
		hexutil.EncodeUint64(number),
		false,
	)
	if err != nil {
		return anchorBlock{}, fmt.Errorf(
			"failed to read block [%d]: [%w]",
			number,
			err,
		)
	}

	if len(payload) == 0 || string(payload) == "null" {
		return anchorBlock{}, fmt.Errorf(
			"the endpoint holds no block [%d]",
			number,
		)
	}

	var reported struct {
		Number *hexutil.Uint64 `json:"number"`
		Hash   *common.Hash    `json:"hash"`
	}
	if err := json.Unmarshal(payload, &reported); err != nil {
		return anchorBlock{}, fmt.Errorf(
			"failed to decode block [%d]: [%w]",
			number,
			err,
		)
	}

	if reported.Number == nil {
		return anchorBlock{}, fmt.Errorf(
			"the block served for [%d] reports no number",
			number,
		)
	}
	if reported.Hash == nil {
		return anchorBlock{}, fmt.Errorf(
			"the block served for [%d] reports no hash",
			number,
		)
	}
	if uint64(*reported.Number) != number {
		return anchorBlock{}, fmt.Errorf(
			"asked for block [%d] and was served block [%d]",
			number,
			uint64(*reported.Number),
		)
	}

	return anchorBlock{
		number: uint64(*reported.Number),
		hash:   reported.Hash.Hex(),
	}, nil
}

// anchorFixtureState is a served block carrying a hash of its own and the fork
// fields a current mainnet header holds but the pinned header type does not
// model.
func anchorFixtureState(number uint64, hash string) ethtest.State {
	return ethtest.State{
		BlockNumber: number,
		BlockHash:   common.HexToHash(hash),
		BlockFields: map[string]json.RawMessage{
			"requestsHash": json.RawMessage(
				`"0x6036c41849da9c076ed79654d434017387a88fb833c2856b32e18218b3341c5f"`,
			),
			"blobGasUsed":   json.RawMessage(`"0x60000"`),
			"excessBlobGas": json.RawMessage(`"0x4b40000"`),
			"parentBeaconBlockRoot": json.RawMessage(
				`"0x4a5b1e39b0e37e05dfbde0dcbbd0cbeb9ee2b23f0b3ff4d2ec4be5b7db6e34ee"`,
			),
		},
	}
}

// TestReadAnchorBlock_ReadsTheReportedHash pins the anchoring the mainnet
// assertions rest on: the hash comes back from the endpoint, and a block
// carrying fields the pinned header type has no room for does not disturb it.
//
// The same block is also decoded through that header type here. The hash it
// derives differs from the one the endpoint reports, which is the whole reason
// the reported hash is what gets read.
func TestReadAnchorBlock_ReadsTheReportedHash(t *testing.T) {
	const (
		number = uint64(25937431)
		hash   = "0xcffd80c8013f3a65c29eb2355ce58fe01cc98e7eb5123352e81907adc77fed36"
	)

	backend := ethtest.New(t, anchorFixtureState(number, hash))

	client, err := ethclient.Dial(backend.URL())
	if err != nil {
		t.Fatal(err)
	}

	anchor, err := readAnchorBlock(
		context.Background(),
		client.Client(),
		number,
	)
	if err != nil {
		t.Fatal(err)
	}

	testutils.AssertStringsEqual(t, "anchor hash", hash, anchor.hash)
	testutils.AssertUintsEqual(t, "anchor number", number, anchor.number)

	header, err := client.HeaderByNumber(
		context.Background(),
		new(big.Int).SetUint64(number),
	)
	if err != nil {
		t.Fatal(err)
	}
	if header.Hash().Hex() == anchor.hash {
		t.Error(
			"the pinned header type derived the reported hash, so this test " +
				"no longer covers the case it exists for",
		)
	}

	backend.AssertNoUnexpectedCalls(t)
}

// TestReadAnchorBlock_RejectsUnusableAnswers covers the answers that must not
// be mistaken for an anchored block: a height the endpoint does not have, a
// block reporting no hash of its own, and a block answering for another
// height.
func TestReadAnchorBlock_RejectsUnusableAnswers(t *testing.T) {
	const (
		number = uint64(25937431)
		hash   = "0xcffd80c8013f3a65c29eb2355ce58fe01cc98e7eb5123352e81907adc77fed36"
	)

	var tests = map[string]func(ethtest.State) ethtest.State{
		"no block at that height": func(state ethtest.State) ethtest.State {
			state.BlockMissing = true
			return state
		},
		"block reporting no hash": func(state ethtest.State) ethtest.State {
			state.BlockFields["hash"] = json.RawMessage("null")
			return state
		},
		"block answering for another height": func(
			state ethtest.State,
		) ethtest.State {
			state.BlockFields["number"] = json.RawMessage(
				fmt.Sprintf(`"%s"`, hexutil.EncodeUint64(number-1)),
			)
			return state
		},
	}

	for testName, seed := range tests {
		t.Run(testName, func(t *testing.T) {
			backend := ethtest.New(t, seed(anchorFixtureState(number, hash)))

			client, err := ethclient.Dial(backend.URL())
			if err != nil {
				t.Fatal(err)
			}

			anchor, err := readAnchorBlock(
				context.Background(),
				client.Client(),
				number,
			)
			if err == nil {
				t.Fatalf("the answer was accepted as anchor [%+v]", anchor)
			}
		})
	}
}
