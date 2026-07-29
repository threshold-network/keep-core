package ethereum

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"testing"
	"time"

	geth "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	ethereumConfig "github.com/keep-network/keep-common/pkg/chain/ethereum"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

type testFrostPreSignEvidenceReader struct {
	finalized         *types.Header
	finalizedSequence []*types.Header
	finalizedCall     int
	headers           map[uint64]*types.Header
	receipts          []*types.Receipt
	receiptCall       int
}

type testBlockingEthereumRPCLimiter struct {
	acquisitionStarted chan struct{}
	allowAcquisition   chan struct{}
	permitReleased     chan struct{}
}

func (limiter *testBlockingEthereumRPCLimiter) AcquirePermit() error {
	close(limiter.acquisitionStarted)
	<-limiter.allowAcquisition
	return nil
}

func (limiter *testBlockingEthereumRPCLimiter) ReleasePermit() {
	close(limiter.permitReleased)
}

func TestNewFrostPreSignPrimaryEthereumReaderAcceptsWrappedClient(
	t *testing.T,
) {
	server := rpc.NewServer()
	rpcClient := rpc.DialInProc(server)
	defer rpcClient.Close()
	client := ethclient.NewClient(rpcClient)
	wrapped := wrapClientAddons(ethereumConfig.Config{}, client)

	reader, err := newFrostPreSignPrimaryEthereumReader(
		wrapped,
		rpcClient,
		big.NewInt(1),
		0,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if reader == nil {
		t.Fatal("wrapped primary Ethereum reader is nil")
	}
}

func TestFrostPrimaryEthereumRPCLimiterWaitHonorsContext(t *testing.T) {
	for _, test := range []struct {
		name           string
		requestTimeout time.Duration
		cancelRequest  bool
		expectedError  error
	}{
		{
			name:          "caller cancellation",
			cancelRequest: true,
			expectedError: context.Canceled,
		},
		{
			name:           "request timeout",
			requestTimeout: 25 * time.Millisecond,
			expectedError:  context.DeadlineExceeded,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			limiter := &testBlockingEthereumRPCLimiter{
				acquisitionStarted: make(chan struct{}),
				allowAcquisition:   make(chan struct{}),
				permitReleased:     make(chan struct{}),
			}
			reader := &frostPreSignCanonicalHashReader{
				requestTimeout: test.requestTimeout,
				rpcLimiter:     limiter,
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			result := make(chan error, 1)
			go func() {
				_, err := reader.CodeAtHash(
					ctx,
					common.HexToAddress(
						"0x1111111111111111111111111111111111111111",
					),
					common.HexToHash("0x1234"),
				)
				result <- err
			}()

			select {
			case <-limiter.acquisitionStarted:
			case <-time.After(time.Second):
				t.Fatal("rate limiter acquisition did not start")
			}
			if test.cancelRequest {
				cancel()
			}

			select {
			case err := <-result:
				if !errors.Is(err, test.expectedError) {
					t.Fatalf(
						"unexpected rate limiter cancellation error: [%v]",
						err,
					)
				}
			case <-time.After(time.Second):
				t.Fatal("rate limiter wait ignored the request context")
			}

			close(limiter.allowAcquisition)
			select {
			case <-limiter.permitReleased:
			case <-time.After(time.Second):
				t.Fatal("permit acquired after cancellation was not released")
			}
		})
	}
}

func TestFrostPrimaryEthereumRPCSharesConfiguredLimiter(t *testing.T) {
	server := rpc.NewServer()
	rpcClient := rpc.DialInProc(server)
	defer rpcClient.Close()
	client := ethclient.NewClient(rpcClient)
	config := ethereumConfig.Config{ConcurrencyLimit: 1}
	wrapped := wrapClientAddons(config, client)
	limiter, err := sharedEthereumRPCLimiter(config, wrapped)
	if err != nil {
		t.Fatal(err)
	}
	if limiter == nil {
		t.Fatal("shared Ethereum RPC limiter is nil")
	}
	if err := limiter.AcquirePermit(); err != nil {
		t.Fatal(err)
	}
	permitHeld := true
	defer func() {
		if permitHeld {
			limiter.ReleasePermit()
		}
	}()

	result := make(chan error, 1)
	go func() {
		_, err := wrapped.HeaderByNumber(context.Background(), nil)
		result <- err
	}()
	select {
	case err := <-result:
		t.Fatalf(
			"ordinary Ethereum RPC bypassed the limiter held by the raw FROST path: [%v]",
			err,
		)
	case <-time.After(50 * time.Millisecond):
	}

	limiter.ReleasePermit()
	permitHeld = false
	select {
	case <-result:
	case <-time.After(time.Second):
		t.Fatal("ordinary Ethereum RPC did not resume after the shared permit was released")
	}
}

func (reader *testFrostPreSignEvidenceReader) ChainID(
	context.Context,
) (*big.Int, error) {
	return big.NewInt(1), nil
}

func (reader *testFrostPreSignEvidenceReader) HeaderByNumber(
	_ context.Context,
	number *big.Int,
) (*types.Header, error) {
	if reader.finalized == nil || number == nil {
		return nil, fmt.Errorf("header unavailable")
	}
	if number.Sign() < 0 {
		if len(reader.finalizedSequence) == 0 {
			return reader.finalized, nil
		}
		index := reader.finalizedCall
		if index >= len(reader.finalizedSequence) {
			index = len(reader.finalizedSequence) - 1
		}
		reader.finalizedCall++
		return reader.finalizedSequence[index], nil
	}
	if number.IsUint64() && reader.headers != nil {
		if header := reader.headers[number.Uint64()]; header != nil {
			return header, nil
		}
	}
	if reader.finalized.Number != nil &&
		number.Cmp(reader.finalized.Number) == 0 {
		return reader.finalized, nil
	}
	return nil, fmt.Errorf("header unavailable")
}

func (reader *testFrostPreSignEvidenceReader) HeaderByHash(
	_ context.Context,
	hash common.Hash,
) (*types.Header, error) {
	if reader.finalized != nil && reader.finalized.Hash() == hash {
		return reader.finalized, nil
	}
	return nil, fmt.Errorf("header unavailable")
}

func (reader *testFrostPreSignEvidenceReader) TransactionReceipt(
	context.Context,
	common.Hash,
) (*types.Receipt, error) {
	if len(reader.receipts) != 0 {
		index := reader.receiptCall
		if index >= len(reader.receipts) {
			index = len(reader.receipts) - 1
		}
		reader.receiptCall++
		return reader.receipts[index], nil
	}
	return nil, fmt.Errorf("receipt unavailable")
}

func (*testFrostPreSignEvidenceReader) FilterLogs(
	context.Context,
	geth.FilterQuery,
) ([]types.Log, error) {
	return nil, nil
}

func (*testFrostPreSignEvidenceReader) CodeAtHash(
	context.Context,
	common.Address,
	common.Hash,
) ([]byte, error) {
	return nil, fmt.Errorf("code unavailable")
}

func (*testFrostPreSignEvidenceReader) StorageAtHash(
	context.Context,
	common.Address,
	common.Hash,
	common.Hash,
) ([]byte, error) {
	return nil, fmt.Errorf("storage unavailable")
}

func (*testFrostPreSignEvidenceReader) CallContractAtHash(
	context.Context,
	geth.CallMsg,
	common.Hash,
) ([]byte, error) {
	return nil, fmt.Errorf("call unavailable")
}

func TestFrostPreSignCurrentFinalityRequiresEndpointAgreement(t *testing.T) {
	primaryHeader := &types.Header{
		Number: big.NewInt(10),
		Time:   10,
		Extra:  []byte{0x01},
	}
	verifierHeader := &types.Header{
		Number: big.NewInt(10),
		Time:   10,
		Extra:  []byte{0x02},
	}
	chain := &TbtcChain{
		frostPreSignAuthorizationAdapter: &frostPreSignEthereumAdapter{
			reader: &testFrostPreSignEvidenceReader{
				finalized: primaryHeader,
			},
		},
		frostPreSignAuthorizationVerifier: &frostPreSignEthereumAdapter{
			reader: &testFrostPreSignEvidenceReader{
				finalized: verifierHeader,
			},
		},
	}

	if _, err := frostPreSignMatchingCurrentFinalityWithRetry(
		context.Background(),
		chain.frostPreSignAuthorizationAdapter,
		chain.frostPreSignAuthorizationVerifier,
		1,
		0,
	); err == nil {
		t.Fatal("different finalized block hashes were accepted")
	}

	chain.frostPreSignAuthorizationVerifier.reader =
		&testFrostPreSignEvidenceReader{finalized: primaryHeader}
	actual, err := chain.CurrentFrostPreSignFinality(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if actual.BlockNumber != 10 ||
		actual.BlockHash != [32]byte(primaryHeader.Hash()) {
		t.Fatalf("unexpected common finality [%+v]", actual)
	}
}

func TestFrostPreSignCurrentFinalityRetriesTransientEndpointSkew(
	t *testing.T,
) {
	olderHeader := &types.Header{
		Number: big.NewInt(10),
		Time:   10,
		Extra:  []byte{0x01},
	}
	newerHeader := &types.Header{
		Number: big.NewInt(11),
		Time:   11,
		Extra:  []byte{0x02},
	}
	primaryReader := &testFrostPreSignEvidenceReader{
		finalized: newerHeader,
		finalizedSequence: []*types.Header{
			olderHeader,
			newerHeader,
			newerHeader,
			newerHeader,
		},
		headers: map[uint64]*types.Header{
			10: olderHeader,
			11: newerHeader,
		},
	}
	verifierReader := &testFrostPreSignEvidenceReader{
		finalized: newerHeader,
		finalizedSequence: []*types.Header{
			newerHeader,
			newerHeader,
			newerHeader,
			newerHeader,
		},
		headers: map[uint64]*types.Header{
			11: newerHeader,
		},
	}

	actual, err := frostPreSignMatchingCurrentFinalityWithRetry(
		context.Background(),
		&frostPreSignEthereumAdapter{reader: primaryReader},
		&frostPreSignEthereumAdapter{reader: verifierReader},
		2,
		0,
	)
	if err != nil {
		t.Fatalf("temporary finalized-head skew was not retried: [%v]", err)
	}
	if actual.BlockNumber != 11 ||
		actual.BlockHash != [32]byte(newerHeader.Hash()) {
		t.Fatalf("unexpected converged finality [%+v]", actual)
	}
}

func TestFrostPreSignAuthorizationStateRequiresVerifierAgreement(
	t *testing.T,
) {
	canonical := &tbtc.FrostPreSignAuthorizationState{
		ActiveReservationID: [32]byte{0x01},
	}
	forged := *canonical
	forged.ActiveReservationID = [32]byte{0xa5}

	if err := frostPreSignRequireMatchingEvidence(
		"authorization state",
		&forged,
		canonical,
	); err == nil {
		t.Fatal("forged primary reservation agreed with verifier")
	}
	if err := frostPreSignRequireMatchingEvidence(
		"authorization state",
		canonical,
		canonical,
	); err != nil {
		t.Fatalf("matching authorization state rejected: [%v]", err)
	}
}

func TestFrostPreSignAuthorizationReceiptBindsTransactionAndLogs(
	t *testing.T,
) {
	registry := common.HexToAddress(
		"0x1111111111111111111111111111111111111111",
	)
	adapter := &frostPreSignEthereumAdapter{
		profile: tbtc.FrostPreSignActivationProfile{
			RegistryAddress: [20]byte(registry),
		},
	}
	relayHash := [32]byte{0x11}
	blockHash := common.HexToHash("0x22")
	proposal := &tbtc.FrostPreSignAuthorizationProposal{
		Transaction: &tbtc.FrostPreSignTransaction{
			Action:          tbtc.FrostPreSignActionRedemption,
			TransactionHash: bitcoin.Hash{0x33},
		},
		ReservationID:     [32]byte{0x44},
		WalletID:          [32]byte{0x55},
		AuthorizationRoot: [32]byte{0x66},
		SnapshotHash:      [32]byte{0x77},
		ResourceHash:      [32]byte{0x88},
	}
	authorizedEvent :=
		frostPreSignRegistryABI.Events["P2TRPreSigningReservationAuthorized"]
	authorizedData, err := authorizedEvent.Inputs.NonIndexed().Pack(
		proposal.AuthorizationRoot,
		proposal.SnapshotHash,
		proposal.ResourceHash,
		uint8(proposal.Transaction.Action),
	)
	if err != nil {
		t.Fatal(err)
	}
	advancedEvent :=
		frostPreSignRegistryABI.Events["P2TRAuthorizedVariantAdvanced"]
	transactionHash := common.Hash(proposal.Transaction.TransactionHash)
	receipt := &types.Receipt{
		TxHash:           common.Hash(relayHash),
		BlockHash:        blockHash,
		BlockNumber:      big.NewInt(10),
		TransactionIndex: 3,
		Logs: []*types.Log{
			{
				Address: registry,
				Topics: []common.Hash{
					authorizedEvent.ID,
					common.Hash(proposal.ReservationID),
					transactionHash,
					common.Hash(proposal.WalletID),
				},
				Data:        authorizedData,
				BlockHash:   blockHash,
				BlockNumber: 10,
				TxHash:      common.Hash(relayHash),
				TxIndex:     3,
				Index:       7,
			},
			{
				Address: registry,
				Topics: []common.Hash{
					advancedEvent.ID,
					common.Hash(proposal.ReservationID),
					transactionHash,
					common.HexToHash("0x99"),
				},
				BlockHash:   blockHash,
				BlockNumber: 10,
				TxHash:      common.Hash(relayHash),
				TxIndex:     3,
				Index:       8,
			},
		},
	}

	if _, _, err := adapter.validateAuthorizationReceipt(
		receipt,
		relayHash,
		proposal,
	); err != nil {
		t.Fatalf("valid receipt rejected: [%v]", err)
	}

	wrongRelayHash := relayHash
	wrongRelayHash[1] = 0x01
	if _, _, err := adapter.validateAuthorizationReceipt(
		receipt,
		wrongRelayHash,
		proposal,
	); err == nil {
		t.Fatal("receipt for a different relay transaction accepted")
	}

	receipt.Logs[1].TxHash = common.HexToHash("0xaa")
	if _, _, err := adapter.validateAuthorizationReceipt(
		receipt,
		relayHash,
		proposal,
	); err == nil {
		t.Fatal("event from a different transaction accepted")
	}
}

func TestFrostPreSignWaitForFinalityRefreshesReincludedReceipt(
	t *testing.T,
) {
	registry := common.HexToAddress(
		"0x1111111111111111111111111111111111111111",
	)
	relayHash := [32]byte{0x11}
	proposal := &tbtc.FrostPreSignAuthorizationProposal{
		Transaction: &tbtc.FrostPreSignTransaction{
			Action:          tbtc.FrostPreSignActionRedemption,
			TransactionHash: bitcoin.Hash{0x33},
		},
		ReservationID:     [32]byte{0x44},
		WalletID:          [32]byte{0x55},
		AuthorizationRoot: [32]byte{0x66},
		SnapshotHash:      [32]byte{0x77},
		ResourceHash:      [32]byte{0x88},
	}
	orphanedHeader := &types.Header{
		Number: big.NewInt(10),
		Time:   10,
		Extra:  []byte{0x01},
	}
	canonicalHeader := &types.Header{
		Number: big.NewInt(10),
		Time:   10,
		Extra:  []byte{0x02},
	}
	reincludedHeader := &types.Header{
		Number: big.NewInt(11),
		Time:   11,
		Extra:  []byte{0x03},
	}
	orphanedReceipt := testFrostPreSignAuthorizationReceipt(
		t,
		registry,
		relayHash,
		proposal,
		orphanedHeader,
		3,
		7,
		8,
	)
	reincludedReceipt := testFrostPreSignAuthorizationReceipt(
		t,
		registry,
		relayHash,
		proposal,
		reincludedHeader,
		4,
		17,
		18,
	)
	reader := &testFrostPreSignEvidenceReader{
		finalized: reincludedHeader,
		headers: map[uint64]*types.Header{
			10: canonicalHeader,
			11: reincludedHeader,
		},
		receipts: []*types.Receipt{
			orphanedReceipt,
			reincludedReceipt,
		},
	}
	adapter := &frostPreSignEthereumAdapter{
		reader: reader,
		profile: tbtc.FrostPreSignActivationProfile{
			RegistryAddress: [20]byte(registry),
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	finality, err := adapter.waitForFinality(ctx, relayHash, proposal)
	if err != nil {
		t.Fatalf("canonically re-included relay was rejected: [%v]", err)
	}
	if reader.receiptCall < 2 {
		t.Fatal("relay receipt was not refreshed while waiting for finality")
	}
	if finality.BlockNumber != 11 ||
		finality.BlockHash != [32]byte(reincludedHeader.Hash()) ||
		finality.TransactionIndex != 4 ||
		finality.LogIndex != 18 {
		t.Fatalf("finality retained obsolete inclusion: [%+v]", finality)
	}
}

func testFrostPreSignAuthorizationReceipt(
	t *testing.T,
	registry common.Address,
	relayHash [32]byte,
	proposal *tbtc.FrostPreSignAuthorizationProposal,
	header *types.Header,
	transactionIndex uint,
	authorizedLogIndex uint,
	advancedLogIndex uint,
) *types.Receipt {
	t.Helper()
	authorizedEvent :=
		frostPreSignRegistryABI.Events["P2TRPreSigningReservationAuthorized"]
	authorizedData, err := authorizedEvent.Inputs.NonIndexed().Pack(
		proposal.AuthorizationRoot,
		proposal.SnapshotHash,
		proposal.ResourceHash,
		uint8(proposal.Transaction.Action),
	)
	if err != nil {
		t.Fatal(err)
	}
	advancedEvent :=
		frostPreSignRegistryABI.Events["P2TRAuthorizedVariantAdvanced"]
	transactionHash := common.Hash(proposal.Transaction.TransactionHash)
	blockNumber := header.Number.Uint64()
	blockHash := header.Hash()
	return &types.Receipt{
		Status:           types.ReceiptStatusSuccessful,
		TxHash:           common.Hash(relayHash),
		BlockHash:        blockHash,
		BlockNumber:      new(big.Int).Set(header.Number),
		TransactionIndex: transactionIndex,
		Logs: []*types.Log{
			{
				Address: registry,
				Topics: []common.Hash{
					authorizedEvent.ID,
					common.Hash(proposal.ReservationID),
					transactionHash,
					common.Hash(proposal.WalletID),
				},
				Data:        authorizedData,
				BlockHash:   blockHash,
				BlockNumber: blockNumber,
				TxHash:      common.Hash(relayHash),
				TxIndex:     transactionIndex,
				Index:       authorizedLogIndex,
			},
			{
				Address: registry,
				Topics: []common.Hash{
					advancedEvent.ID,
					common.Hash(proposal.ReservationID),
					transactionHash,
					common.HexToHash("0x99"),
				},
				BlockHash:   blockHash,
				BlockNumber: blockNumber,
				TxHash:      common.Hash(relayHash),
				TxIndex:     transactionIndex,
				Index:       advancedLogIndex,
			},
		},
	}
}
