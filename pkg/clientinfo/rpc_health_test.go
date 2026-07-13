package clientinfo

import (
	"context"
	"fmt"
	"testing"
	"time"

	keepclientinfo "github.com/keep-network/keep-common/pkg/clientinfo"
	"github.com/keep-network/keep-core/pkg/bitcoin"
)

// --- fakes ---

type fakeBlockCounter struct {
	currentBlock uint64
	err          error
}

func (f *fakeBlockCounter) CurrentBlock() (uint64, error) {
	return f.currentBlock, f.err
}

func (f *fakeBlockCounter) WaitForBlockHeight(blockNumber uint64) error { return nil }
func (f *fakeBlockCounter) BlockHeightWaiter(blockNumber uint64) (<-chan uint64, error) {
	ch := make(chan uint64)
	close(ch)
	return ch, nil
}
func (f *fakeBlockCounter) WatchBlocks(ctx context.Context) <-chan uint64 {
	ch := make(chan uint64)
	go func() { <-ctx.Done(); close(ch) }()
	return ch
}

type fakeBitcoinChain struct {
	latestHeight uint
	latestErr    error
	headerErr    error
}

func (f *fakeBitcoinChain) GetLatestBlockHeight() (uint, error) {
	return f.latestHeight, f.latestErr
}

func (f *fakeBitcoinChain) GetBlockHeader(blockHeight uint) (*bitcoin.BlockHeader, error) {
	if f.headerErr != nil {
		return nil, f.headerErr
	}
	return &bitcoin.BlockHeader{}, nil
}

func (f *fakeBitcoinChain) GetTransaction(transactionHash bitcoin.Hash) (*bitcoin.Transaction, error) {
	panic("not needed in rpc_health tests")
}
func (f *fakeBitcoinChain) GetTransactionConfirmations(transactionHash bitcoin.Hash) (uint, error) {
	panic("not needed in rpc_health tests")
}
func (f *fakeBitcoinChain) BroadcastTransaction(transaction *bitcoin.Transaction) error {
	panic("not needed in rpc_health tests")
}
func (f *fakeBitcoinChain) GetTransactionMerkleProof(transactionHash bitcoin.Hash, blockHeight uint) (*bitcoin.TransactionMerkleProof, error) {
	panic("not needed in rpc_health tests")
}
func (f *fakeBitcoinChain) GetTransactionsForPublicKeyHash(publicKeyHash [20]byte, limit int) ([]*bitcoin.Transaction, error) {
	panic("not needed in rpc_health tests")
}
func (f *fakeBitcoinChain) GetTxHashesForPublicKeyHash(publicKeyHash [20]byte) ([]bitcoin.Hash, error) {
	panic("not needed in rpc_health tests")
}
func (f *fakeBitcoinChain) GetMempoolForPublicKeyHash(publicKeyHash [20]byte) ([]*bitcoin.Transaction, error) {
	panic("not needed in rpc_health tests")
}
func (f *fakeBitcoinChain) GetUtxosForPublicKeyHash(publicKeyHash [20]byte) ([]*bitcoin.UnspentTransactionOutput, error) {
	panic("not needed in rpc_health tests")
}
func (f *fakeBitcoinChain) GetMempoolUtxosForPublicKeyHash(publicKeyHash [20]byte) ([]*bitcoin.UnspentTransactionOutput, error) {
	panic("not needed in rpc_health tests")
}
func (f *fakeBitcoinChain) EstimateSatPerVByteFee(blocks uint32) (int64, error) {
	panic("not needed in rpc_health tests")
}
func (f *fakeBitcoinChain) GetCoinbaseTxHash(blockHeight uint) (bitcoin.Hash, error) {
	panic("not needed in rpc_health tests")
}

// --- helpers ---

func newTestChecker(eth *fakeBlockCounter, btc *fakeBitcoinChain) *RPCHealthChecker {
	return &RPCHealthChecker{
		ethBlockCounter: eth,
		btcChain:        btc,
		checkInterval:   time.Minute, // not used in direct call tests
	}
}

// --- Ethereum health tests ---

func TestRPCHealthChecker_EthereumHealthy(t *testing.T) {
	checker := newTestChecker(&fakeBlockCounter{currentBlock: 12345678}, nil)

	checker.checkEthereumHealth(context.Background())

	healthy, lastCheck, lastSuccess, lastErr, _ := checker.GetEthereumHealthStatus()

	if !healthy {
		t.Errorf("expected healthy, got unhealthy (lastErr: %v)", lastErr)
	}
	if lastCheck.IsZero() {
		t.Error("lastCheck should be set")
	}
	if lastSuccess.IsZero() {
		t.Error("lastSuccess should be set on healthy check")
	}
	if lastErr != nil {
		t.Errorf("expected nil error, got: %v", lastErr)
	}
}

func TestRPCHealthChecker_EthereumUnhealthy_RPCError(t *testing.T) {
	rpcErr := fmt.Errorf("connection refused")
	checker := newTestChecker(&fakeBlockCounter{err: rpcErr}, nil)

	checker.checkEthereumHealth(context.Background())

	healthy, lastCheck, _, lastErr, _ := checker.GetEthereumHealthStatus()

	if healthy {
		t.Error("expected unhealthy after RPC error")
	}
	if lastCheck.IsZero() {
		t.Error("lastCheck should be set even on failure")
	}
	if lastErr == nil {
		t.Error("expected error to be stored")
	}
}

func TestRPCHealthChecker_EthereumUnhealthy_ZeroBlock(t *testing.T) {
	checker := newTestChecker(&fakeBlockCounter{currentBlock: 0}, nil)

	checker.checkEthereumHealth(context.Background())

	healthy, _, _, lastErr, _ := checker.GetEthereumHealthStatus()

	if healthy {
		t.Error("expected unhealthy when block number is 0")
	}
	if lastErr == nil {
		t.Error("expected error for zero block number")
	}
}

func TestRPCHealthChecker_EthereumNilBlockCounter(t *testing.T) {
	// Use a nil interface directly -- not a nil *fakeBlockCounter, which would
	// create a non-nil interface wrapping a nil pointer and bypass the guard.
	checker := &RPCHealthChecker{
		ethBlockCounter: nil, // true nil interface
		checkInterval:   time.Minute,
	}

	// Should not panic when ethBlockCounter is nil.
	checker.checkEthereumHealth(context.Background())

	healthy, lastCheck, _, _, _ := checker.GetEthereumHealthStatus()
	if healthy {
		t.Error("expected not healthy with nil block counter")
	}
	if !lastCheck.IsZero() {
		t.Error("lastCheck should not be set when block counter is nil")
	}
}

func TestRPCHealthChecker_EthereumHealthTransition(t *testing.T) {
	eth := &fakeBlockCounter{currentBlock: 100}
	checker := newTestChecker(eth, nil)

	// First check: healthy.
	checker.checkEthereumHealth(context.Background())
	healthy, _, _, _, _ := checker.GetEthereumHealthStatus()
	if !healthy {
		t.Fatal("expected healthy after first successful check")
	}

	// Inject failure.
	eth.err = fmt.Errorf("node unreachable")
	eth.currentBlock = 0
	checker.checkEthereumHealth(context.Background())
	healthy, _, _, lastErr, _ := checker.GetEthereumHealthStatus()
	if healthy {
		t.Error("expected unhealthy after RPC failure")
	}
	if lastErr == nil {
		t.Error("expected error to be stored")
	}

	// Recover.
	eth.err = nil
	eth.currentBlock = 200
	checker.checkEthereumHealth(context.Background())
	healthy, _, lastSuccess, _, _ := checker.GetEthereumHealthStatus()
	if !healthy {
		t.Error("expected healthy after recovery")
	}
	if lastSuccess.IsZero() {
		t.Error("expected lastSuccess to be updated on recovery")
	}
}

// --- Bitcoin health tests ---

func TestRPCHealthChecker_BitcoinHealthy(t *testing.T) {
	checker := newTestChecker(nil, &fakeBitcoinChain{latestHeight: 800000})

	checker.checkBitcoinHealth(context.Background())

	healthy, lastCheck, lastSuccess, lastErr, _ := checker.GetBitcoinHealthStatus()

	if !healthy {
		t.Errorf("expected healthy, got unhealthy (lastErr: %v)", lastErr)
	}
	if lastCheck.IsZero() {
		t.Error("lastCheck should be set")
	}
	if lastSuccess.IsZero() {
		t.Error("lastSuccess should be set on healthy check")
	}
	if lastErr != nil {
		t.Errorf("expected nil error, got: %v", lastErr)
	}
}

func TestRPCHealthChecker_BitcoinUnhealthy_RPCError(t *testing.T) {
	checker := newTestChecker(nil, &fakeBitcoinChain{
		latestErr: fmt.Errorf("electrum unavailable"),
	})

	checker.checkBitcoinHealth(context.Background())

	healthy, _, _, lastErr, _ := checker.GetBitcoinHealthStatus()

	if healthy {
		t.Error("expected unhealthy after RPC error")
	}
	if lastErr == nil {
		t.Error("expected error to be stored")
	}
}

func TestRPCHealthChecker_BitcoinUnhealthy_ZeroHeight(t *testing.T) {
	checker := newTestChecker(nil, &fakeBitcoinChain{latestHeight: 0})

	checker.checkBitcoinHealth(context.Background())

	healthy, _, _, lastErr, _ := checker.GetBitcoinHealthStatus()

	if healthy {
		t.Error("expected unhealthy when block height is 0")
	}
	if lastErr == nil {
		t.Error("expected error for zero block height")
	}
}

func TestRPCHealthChecker_BitcoinUnhealthy_HeaderError(t *testing.T) {
	checker := newTestChecker(nil, &fakeBitcoinChain{
		latestHeight: 800000,
		headerErr:    fmt.Errorf("block header not found"),
	})

	checker.checkBitcoinHealth(context.Background())

	healthy, _, _, lastErr, _ := checker.GetBitcoinHealthStatus()

	if healthy {
		t.Error("expected unhealthy when GetBlockHeader fails")
	}
	if lastErr == nil {
		t.Error("expected error to be stored for header failure")
	}
}

func TestRPCHealthChecker_BitcoinNilChain(t *testing.T) {
	checker := &RPCHealthChecker{
		btcChain:      nil, // true nil interface
		checkInterval: time.Minute,
	}

	// Should not panic when btcChain is nil.
	checker.checkBitcoinHealth(context.Background())

	healthy, lastCheck, _, _, _ := checker.GetBitcoinHealthStatus()
	if healthy {
		t.Error("expected not healthy with nil btc chain")
	}
	if !lastCheck.IsZero() {
		t.Error("lastCheck should not be set when btc chain is nil")
	}
}

func TestRPCHealthChecker_BitcoinHealthTransition(t *testing.T) {
	btc := &fakeBitcoinChain{latestHeight: 500000}
	checker := newTestChecker(nil, btc)

	// First check: healthy.
	checker.checkBitcoinHealth(context.Background())
	healthy, _, _, _, _ := checker.GetBitcoinHealthStatus()
	if !healthy {
		t.Fatal("expected healthy after first successful check")
	}

	// Inject failure.
	btc.latestErr = fmt.Errorf("electrum disconnected")
	checker.checkBitcoinHealth(context.Background())
	healthy, _, _, lastErr, _ := checker.GetBitcoinHealthStatus()
	if healthy {
		t.Error("expected unhealthy after failure")
	}
	if lastErr == nil {
		t.Error("expected error to be stored")
	}

	// Recover.
	btc.latestErr = nil
	checker.checkBitcoinHealth(context.Background())
	healthy, _, lastSuccess, _, _ := checker.GetBitcoinHealthStatus()
	if !healthy {
		t.Error("expected healthy after recovery")
	}
	if lastSuccess.IsZero() {
		t.Error("expected lastSuccess to be updated on recovery")
	}
}

// --- Start idempotency test ---

func TestRPCHealthChecker_StartIdempotent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry := &Registry{keepclientinfo.NewRegistry(), ctx}
	eth := &fakeBlockCounter{currentBlock: 12345}
	btc := &fakeBitcoinChain{latestHeight: 800000}
	checker := NewRPCHealthChecker(registry, eth, btc, time.Hour)

	// Calling Start twice should not panic or duplicate goroutines.
	checker.Start(ctx)
	checker.Start(ctx)

	// Give the initial synchronous checks in start() time to complete.
	time.Sleep(50 * time.Millisecond)

	healthy, _, _, _, _ := checker.GetEthereumHealthStatus()
	if !healthy {
		t.Error("expected healthy after Start")
	}
}
