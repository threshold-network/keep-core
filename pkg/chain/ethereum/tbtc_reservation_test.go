package ethereum

// This file tests the reservation adapters in tbtc.go that talk to the
// Ethereum node, against a scripted fake client instead of a node or the
// in-memory EVM harness:
//
//   - GetReservationReanchorRequestReceipt: the Mined/Reverted/NotFound
//     mapping from TransactionReceipt results, error propagation for
//     non-not-found RPC failures, and the bounded-lookup requirement.
//   - ReservationVaultFeeDebtSat /
//     ReservationVaultFeeReserveTbtcBaseUnits: that the gauges read the
//     vault's own on-chain values and that the fee reserve is the TBTC
//     balance of the vault address itself, not another account's.

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"testing"
	"time"

	hostchain "github.com/ethereum/go-ethereum"
	hostchainabi "github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/keep-network/keep-common/pkg/chain/ethereum"
	"github.com/keep-network/keep-common/pkg/chain/ethereum/ethutil"
	"github.com/keep-network/keep-core/pkg/chain"
	tbtcabi "github.com/keep-network/keep-core/pkg/chain/ethereum/tbtc/gen/abi"
	tbtccontract "github.com/keep-network/keep-core/pkg/chain/ethereum/tbtc/gen/contract"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// reservationFakeClient is a scripted ethutil.EthereumClient: view calls
// are answered from a per-(address, selector) table, TransactionReceipt is
// answered from canned fields, and every other method is promoted from the
// embedded nil interface and would panic if the code under test reached
// it - keeping the fake honest about the RPC surface actually exercised.
type reservationFakeClient struct {
	ethutil.EthereumClient

	mu sync.Mutex

	// views serves ABI-packed return data keyed by target address and
	// 4-byte method selector.
	views map[common.Address]map[[4]byte]func() []byte

	// balances serves the token's balanceOf(account) results keyed by
	// account; balanceOfTargets records which accounts were queried.
	balances         map[common.Address]*big.Int
	balanceOfTargets []common.Address
	balanceOfSel     [4]byte

	// receiptResult / receiptErr are the canned TransactionReceipt
	// answer; receiptCtx and receiptHash record the lookup's context and
	// the hash it targeted.
	receiptResult *types.Receipt
	receiptErr    error
	receiptCtx    context.Context
	receiptHash   common.Hash
}

func newReservationFakeClient(t *testing.T) *reservationFakeClient {
	t.Helper()

	// balanceOf(address) selector: the first 4 bytes of the keccak256 of
	// the full signature, the same selector the TBTC binding's call data
	// carries.
	balanceOfSel := [4]byte{}
	copy(balanceOfSel[:], crypto.Keccak256([]byte("balanceOf(address)"))[:4])

	return &reservationFakeClient{
		views:        make(map[common.Address]map[[4]byte]func() []byte),
		balances:     make(map[common.Address]*big.Int),
		balanceOfSel: balanceOfSel,
	}
}

// setView registers the ABI-packed return data for one method on one
// contract address, deriving the selector from the method signature.
func (c *reservationFakeClient) setView(
	t *testing.T,
	target common.Address,
	selector [4]byte,
	respond func() []byte,
) {
	t.Helper()
	if c.views[target] == nil {
		c.views[target] = make(map[[4]byte]func() []byte)
	}
	c.views[target][selector] = respond
}

func (c *reservationFakeClient) CallContract(
	ctx context.Context,
	call hostchain.CallMsg,
	blockNumber *big.Int,
) ([]byte, error) {
	if call.To == nil {
		return nil, errors.New("reservationFakeClient: view call without target")
	}
	if len(call.Data) < 4 {
		return nil, errors.New("reservationFakeClient: view call without selector")
	}
	var selector [4]byte
	copy(selector[:], call.Data[:4])

	c.mu.Lock()
	defer c.mu.Unlock()

	// The token's balanceOf is the one response that depends on the call
	// argument: record the queried account and serve its registered
	// balance, so an adapter reading a wrong account gets a wrong value.
	if selector == c.balanceOfSel {
		account := common.BytesToAddress(call.Data[16:36])
		c.balanceOfTargets = append(c.balanceOfTargets, account)
		balance, ok := c.balances[account]
		if !ok {
			return nil, fmt.Errorf(
				"reservationFakeClient: no balance registered for %s",
				account.Hex(),
			)
		}
		packed := make([]byte, 32)
		balance.FillBytes(packed)
		return packed, nil
	}

	targetViews, ok := c.views[*call.To]
	if !ok {
		return nil, fmt.Errorf(
			"reservationFakeClient: no view registered for %s",
			call.To.Hex(),
		)
	}
	respond, ok := targetViews[selector]
	if !ok {
		return nil, fmt.Errorf(
			"reservationFakeClient: no view registered for selector 0x%x on %s",
			selector,
			call.To.Hex(),
		)
	}
	return respond(), nil
}

func (c *reservationFakeClient) TransactionReceipt(
	ctx context.Context,
	txHash common.Hash,
) (*types.Receipt, error) {
	c.mu.Lock()
	c.receiptCtx = ctx
	c.receiptHash = txHash
	c.mu.Unlock()
	return c.receiptResult, c.receiptErr
}

// CodeAt answers the contract-code probe go-ethereum issues when a view
// call returns zero bytes; the fake never does, so this is defensive.
func (c *reservationFakeClient) CodeAt(
	ctx context.Context,
	contract common.Address,
	blockNumber *big.Int,
) ([]byte, error) {
	return []byte{0x00}, nil
}

// methodSelector returns the 4-byte selector for a named ABI method,
// computed exactly the way go-ethereum derives it: the first 4 bytes of
// the keccak256 of the method's signature string.
func methodSelector(t *testing.T, abi hostchainabi.ABI, method string) [4]byte {
	t.Helper()
	m, ok := abi.Methods[method]
	if !ok {
		t.Fatalf("method %s not in ABI", method)
	}
	var selector [4]byte
	copy(selector[:], crypto.Keccak256([]byte(m.Sig))[:4])
	return selector
}

// packOutputs returns the ABI-packed return data for a method's outputs.
func packOutputs(t *testing.T, abi hostchainabi.ABI, method string, values ...any) []byte {
	t.Helper()
	m, ok := abi.Methods[method]
	if !ok {
		t.Fatalf("method %s not in ABI", method)
	}
	packed, err := m.Outputs.Pack(values...)
	if err != nil {
		t.Fatalf("cannot pack outputs of %s: [%v]", method, err)
	}
	return packed
}

func parseGenABI(t *testing.T, raw string) hostchainabi.ABI {
	t.Helper()
	abi, err := hostchainabi.JSON(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("cannot parse generated ABI: [%v]", err)
	}
	return abi
}

// newReservationGaugeChain wires a TbtcChain whose reservation gauges run
// against the fake client: a ReservationRouter binding attached at the
// router address plus the baseChain wiring the generated vault and token
// bindings need (the block counter is not on the view paths, so it is
// left nil).
func newReservationGaugeChain(
	t *testing.T,
	client *reservationFakeClient,
	routerAddress common.Address,
) *TbtcChain {
	t.Helper()

	privateKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("cannot generate test key: [%v]", err)
	}
	accountKey := &keystore.Key{
		Address:    crypto.PubkeyToAddress(privateKey.PublicKey),
		PrivateKey: privateKey,
	}
	chainID := big.NewInt(1337)

	nonceManager := ethutil.NewNonceManager(client, accountKey.Address)
	miningWaiter := ethutil.NewMiningWaiter(client, ethereum.Config{})

	router, err := tbtccontract.NewReservationRouter(
		routerAddress,
		chainID,
		accountKey,
		client,
		nonceManager,
		miningWaiter,
		nil,
		&sync.Mutex{},
	)
	if err != nil {
		t.Fatalf("cannot attach the ReservationRouter binding: [%v]", err)
	}

	return &TbtcChain{
		baseChain: &baseChain{
			key:              accountKey,
			client:           client,
			chainID:          chainID,
			nonceManager:     nonceManager,
			miningWaiter:     miningWaiter,
			transactionMutex: &sync.Mutex{},
		},
		reservationRouter: router,
	}
}

func TestGetReservationReanchorRequestReceipt(t *testing.T) {
	var txHash [32]byte
	for i := range txHash {
		txHash[i] = byte(i + 1)
	}
	lookupHash := common.BytesToHash(txHash[:])

	t.Run("successful receipt maps to Mined", func(t *testing.T) {
		client := newReservationFakeClient(t)
		client.receiptResult = &types.Receipt{Status: types.ReceiptStatusSuccessful}
		chain := &TbtcChain{baseChain: &baseChain{client: client}}

		status, err := chain.GetReservationReanchorRequestReceipt(txHash)
		if err != nil {
			t.Fatalf("unexpected error: [%v]", err)
		}
		if status != tbtc.ReservationReanchorRequestReceiptMined {
			t.Fatalf("expected Mined, got %v", status)
		}
	})

	t.Run("failed receipt maps to Reverted", func(t *testing.T) {
		client := newReservationFakeClient(t)
		client.receiptResult = &types.Receipt{Status: types.ReceiptStatusFailed}
		chain := &TbtcChain{baseChain: &baseChain{client: client}}

		status, err := chain.GetReservationReanchorRequestReceipt(txHash)
		if err != nil {
			t.Fatalf("unexpected error: [%v]", err)
		}
		if status != tbtc.ReservationReanchorRequestReceiptReverted {
			t.Fatalf("expected Reverted, got %v", status)
		}
	})

	t.Run("not-found receipt maps to NotFound without error", func(t *testing.T) {
		client := newReservationFakeClient(t)
		client.receiptErr = hostchain.NotFound
		chain := &TbtcChain{baseChain: &baseChain{client: client}}

		status, err := chain.GetReservationReanchorRequestReceipt(txHash)
		if err != nil {
			t.Fatalf("expected no error for a not-found receipt, got [%v]", err)
		}
		if status != tbtc.ReservationReanchorRequestReceiptNotFound {
			t.Fatalf("expected NotFound, got %v", status)
		}
	})

	t.Run("nil receipt without error maps to NotFound", func(t *testing.T) {
		client := newReservationFakeClient(t)
		chain := &TbtcChain{baseChain: &baseChain{client: client}}

		status, err := chain.GetReservationReanchorRequestReceipt(txHash)
		if err != nil {
			t.Fatalf("expected no error, got [%v]", err)
		}
		if status != tbtc.ReservationReanchorRequestReceiptNotFound {
			t.Fatalf("expected NotFound, got %v", status)
		}
	})

	t.Run("transient RPC failure propagates", func(t *testing.T) {
		rpcFailure := errors.New("provider RPC failed")
		client := newReservationFakeClient(t)
		client.receiptErr = rpcFailure
		chain := &TbtcChain{baseChain: &baseChain{client: client}}

		_, err := chain.GetReservationReanchorRequestReceipt(txHash)
		if err == nil {
			t.Fatal("expected the transient RPC failure to propagate, got nil error")
		}
		if !errors.Is(err, rpcFailure) {
			t.Fatalf("expected the wrapped error to match the RPC failure, got [%v]", err)
		}
	})

	t.Run("lookup is bounded by a deadline and uses the given hash", func(t *testing.T) {
		client := newReservationFakeClient(t)
		client.receiptResult = &types.Receipt{Status: types.ReceiptStatusSuccessful}
		chain := &TbtcChain{baseChain: &baseChain{client: client}}

		before := time.Now()
		if _, err := chain.GetReservationReanchorRequestReceipt(txHash); err != nil {
			t.Fatalf("unexpected error: [%v]", err)
		}
		if client.receiptCtx == nil {
			t.Fatal("the receipt lookup context was never recorded")
		}
		deadline, ok := client.receiptCtx.Deadline()
		if !ok {
			t.Fatal("expected the receipt lookup to run under a bounded context")
		}
		if deadline.Before(before) || deadline.After(before.Add(31*time.Second)) {
			t.Fatalf("deadline %v outside the [start, start+30s] window", deadline)
		}
		// The receipt lookup must have targeted the exact hash that was
		// passed in.
		if client.receiptHash != lookupHash {
			t.Fatalf(
				"receipt lookup targeted %s, want %s",
				client.receiptHash.Hex(),
				lookupHash.Hex(),
			)
		}
	})
}

func TestReservationVaultFeeGaugesReadVaultValues(t *testing.T) {
	routerAddress := common.HexToAddress("0x00000000000000000000000000000000000000a1")
	vaultAddress := common.HexToAddress("0x00000000000000000000000000000000000000a2")
	tokenAddress := common.HexToAddress("0x00000000000000000000000000000000000000a3")
	otherAccount := common.HexToAddress("0x00000000000000000000000000000000000000a4")

	vaultABI := parseGenABI(t, tbtcabi.ReservationVaultABI)

	client := newReservationFakeClient(t)

	// The gauges read distinct vault-side values, so a source/address
	// swap between the two gauges cannot slip through.
	const vaultDebtSat uint64 = 4242
	// The vault's TBTC fee reserve and another account's balance are
	// deliberately different; the reserve must be the vault's own.
	vaultTbtcBalance := new(big.Int).Lsh(big.NewInt(3), 18) // 3 TBTC
	otherTbtcBalance := new(big.Int).Lsh(big.NewInt(7), 18) // 7 TBTC

	// The router's reservationParameters view is deliberately not
	// scripted: the caller passes the vault address it already read, so
	// the gauges must not re-read the reservation parameters.
	client.setView(
		t,
		vaultAddress,
		methodSelector(t, vaultABI, "inKindFeeDebtSat"),
		func() []byte {
			return packOutputs(t, vaultABI, "inKindFeeDebtSat", vaultDebtSat)
		},
	)
	client.setView(
		t,
		vaultAddress,
		methodSelector(t, vaultABI, "tbtcToken"),
		func() []byte {
			return packOutputs(t, vaultABI, "tbtcToken", tokenAddress)
		},
	)
	client.balances[vaultAddress] = vaultTbtcBalance
	client.balances[otherAccount] = otherTbtcBalance

	vaultArg := chain.Address(vaultAddress.Hex())
	chain := newReservationGaugeChain(t, client, routerAddress)

	debt, err := chain.ReservationVaultFeeDebtSat(vaultArg)
	if err != nil {
		t.Fatalf("unexpected error: [%v]", err)
	}
	if debt != vaultDebtSat {
		t.Fatalf("expected the vault's in-kind fee debt %d, got %d", vaultDebtSat, debt)
	}

	reserve, err := chain.ReservationVaultFeeReserveTbtcBaseUnits(vaultArg)
	if err != nil {
		t.Fatalf("unexpected error: [%v]", err)
	}
	if reserve.Cmp(vaultTbtcBalance) != 0 {
		t.Fatalf(
			"expected the vault's TBTC balance %s, got %s",
			vaultTbtcBalance,
			reserve,
		)
	}

	// The reserve must have been read from the VAULT address, not another
	// account: the only balanceOf call must target the vault.
	if len(client.balanceOfTargets) != 1 || client.balanceOfTargets[0] != vaultAddress {
		t.Fatalf(
			"expected a single balanceOf query for the vault address %s, got %v",
			vaultAddress.Hex(),
			client.balanceOfTargets,
		)
	}
}

func TestReservationVaultFeeGaugesSkipUnconfiguredVault(t *testing.T) {
	routerAddress := common.HexToAddress("0x00000000000000000000000000000000000000a1")

	client := newReservationFakeClient(t)

	// Unconfigured vault: the reservation parameters carry the zero
	// address, so the gauges must short-circuit before touching any vault
	// or token view.
	vaultArg := chain.Address(common.Address{}.Hex())
	chain := newReservationGaugeChain(t, client, routerAddress)

	debt, err := chain.ReservationVaultFeeDebtSat(vaultArg)
	if err != nil {
		t.Fatalf("expected the unconfigured-vault skip, got error [%v]", err)
	}
	if debt != 0 {
		t.Fatalf("expected the unconfigured-vault skip sentinel 0, got %d", debt)
	}

	reserve, err := chain.ReservationVaultFeeReserveTbtcBaseUnits(vaultArg)
	if err != nil {
		t.Fatalf("expected the unconfigured-vault skip, got error [%v]", err)
	}
	if reserve.Sign() != 0 {
		t.Fatalf("expected the unconfigured-vault skip sentinel 0, got %s", reserve)
	}

	// No token balance may be read for an unconfigured vault.
	if len(client.balanceOfTargets) != 0 {
		t.Fatalf("expected no balanceOf queries, got %v", client.balanceOfTargets)
	}
}
