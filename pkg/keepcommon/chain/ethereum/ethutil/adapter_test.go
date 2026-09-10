package ethutil

import (
	"context"
	"fmt"
	"math/big"
	"reflect"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"

	chainEthereum "github.com/keep-network/keep-common/pkg/chain/ethereum"
)

func TestEthereumAdapter_HeaderByNumber(t *testing.T) {
	client := &mockAdaptedEthereumClient{
		blocks: []*big.Int{
			big.NewInt(0),
			big.NewInt(1),
			big.NewInt(2),
		},
		blocksBaseFee: []*big.Int{
			big.NewInt(10),
			big.NewInt(11),
			big.NewInt(12),
		},
	}

	adapter := &ethereumAdapter{client}

	headerOne, err := adapter.HeaderByNumber(context.Background(), big.NewInt(1))
	if err != nil {
		t.Fatal(err)
	}

	lastHeader, err := adapter.HeaderByNumber(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}

	expectedHeaderOneNumber := big.NewInt(1)
	if expectedHeaderOneNumber.Cmp(headerOne.Number) != 0 {
		t.Errorf(
			"unexpected header number\n"+
				"expected: [%v]\n"+
				"actual:   [%v]",
			expectedHeaderOneNumber,
			headerOne.Number,
		)
	}

	expectedLastHeaderNumber := big.NewInt(2)
	if expectedLastHeaderNumber.Cmp(lastHeader.Number) != 0 {
		t.Errorf(
			"unexpected last header number\n"+
				"expected: [%v]\n"+
				"actual:   [%v]",
			expectedLastHeaderNumber,
			lastHeader.Number,
		)
	}
}

func TestEthereumAdapter_SubscribeNewHead(t *testing.T) {
	ctx, cancelCtx := context.WithTimeout(
		context.Background(),
		10*time.Millisecond,
	)
	defer cancelCtx()

	client := &mockAdaptedEthereumClient{
		blocks: []*big.Int{
			big.NewInt(0),
			big.NewInt(1),
			big.NewInt(2),
		},
	}

	adapter := &ethereumAdapter{client}

	// no more than 3 elements should be put into this
	// channel by SubscribeNewHead
	headerChan := make(chan *chainEthereum.Header, 100)
	_, err := adapter.SubscribeNewHead(ctx, headerChan)
	if err != nil {
		t.Fatal(err)
	}

	<-ctx.Done()

	expectedHeaderChanLen := 3
	headerChanLen := len(headerChan)
	if expectedHeaderChanLen != headerChanLen {
		t.Errorf(
			"unexpected number of blocks\n"+
				"expected: [%v]\n"+
				"actual:   [%v]",
			expectedHeaderChanLen,
			headerChanLen,
		)
	}

	blocks := make([]*big.Int, 0)
	for header := range headerChan {
		blocks = append(blocks, header.Number)

		// headerChan is not closed so we have to break manually
		if len(blocks) == 3 {
			break
		}
	}

	expectedBlocks := []*big.Int{
		big.NewInt(0),
		big.NewInt(1),
		big.NewInt(2),
	}
	if !reflect.DeepEqual(expectedBlocks, blocks) {
		t.Errorf(
			"unexpected blocks\n"+
				"expected: [%v]\n"+
				"actual:   [%v]",
			expectedBlocks,
			blocks,
		)
	}
}

func TestEthereumAdapter_PendingNonceAt(t *testing.T) {
	var address [20]byte
	copy(address[:], []byte{255})

	client := &mockAdaptedEthereumClient{
		nonces: map[common.Address]uint64{
			common.BytesToAddress(address[:]): 100,
		},
	}

	adapter := &ethereumAdapter{client}

	nonce, err := adapter.PendingNonceAt(context.Background(), address)
	if err != nil {
		t.Fatal(err)
	}

	expectedNonce := uint64(100)
	if expectedNonce != nonce {
		t.Errorf(
			"unexpected nonce\n"+
				"expected: [%+v]\n"+
				"actual:   [%+v]",
			expectedNonce,
			nonce,
		)
	}
}

type mockAdaptedEthereumClient struct {
	*mockEthereumClient

	blocks        []*big.Int
	blocksBaseFee []*big.Int
	nonces        map[common.Address]uint64
}

func (maec *mockAdaptedEthereumClient) HeaderByNumber(
	ctx context.Context,
	number *big.Int,
) (*types.Header, error) {
	index := len(maec.blocks) - 1

	if number != nil {
		index = int(number.Int64())
	}

	return &types.Header{
		Number:  maec.blocks[index],
		BaseFee: maec.blocksBaseFee[index],
	}, nil
}

func (maec *mockAdaptedEthereumClient) SubscribeNewHead(
	ctx context.Context,
	ch chan<- *types.Header,
) (ethereum.Subscription, error) {
	go func() {
		for _, block := range maec.blocks {
			ch <- &types.Header{Number: block}
		}
	}()

	return &subscriptionWrapper{
		unsubscribeFn: func() {},
		errChan:       make(chan error),
	}, nil
}

func (maec *mockAdaptedEthereumClient) PendingNonceAt(
	ctx context.Context,
	account common.Address,
) (uint64, error) {
	if nonce, ok := maec.nonces[account]; ok {
		return nonce, nil
	}

	return 0, fmt.Errorf("no nonce for given account")
}
