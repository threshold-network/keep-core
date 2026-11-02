package ethereum

// This file tests the reservation adapters in tbtc.go that talk to the
// Ethereum node, against a scripted fake client instead of a node or the
// in-memory EVM harness:
//
//   - ReservationVaultFeeDebtSat /
//     ReservationVaultFeeReserveTbtcBaseUnits: that the gauges read the
//     vault's own on-chain values and that the fee reserve is the TBTC
//     balance of the vault address itself, not another account's.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"testing"

	hostchain "github.com/ethereum/go-ethereum"
	hostchainabi "github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/keep-network/keep-core/pkg/chain"
	tbtcabi "github.com/keep-network/keep-core/pkg/chain/ethereum/tbtc/gen/abi"
	tbtccontract "github.com/keep-network/keep-core/pkg/chain/ethereum/tbtc/gen/contract"
	"github.com/keep-network/keep-core/pkg/chain/ethereumutil"
	"github.com/keep-network/keep-core/pkg/chain/ethereumutil/ethutil"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// reservationFakeClient is a scripted ethutil.EthereumClient: view calls
// are answered from a per-(address, selector) table, and every other
// method is promoted from the embedded nil interface and would panic if
// the code under test reached it - keeping the fake honest about the RPC
// surface actually exercised.
type reservationFakeClient struct {
	ethutil.EthereumClient

	mu sync.Mutex

	// views serves ABI-packed return data keyed by target address and
	// 4-byte method selector.
	views map[common.Address]map[[4]byte]func() []byte

	// viewErrors serves a failure instead of return data, keyed the same
	// way, so tests can script the exact error shape a view call produces.
	viewErrors map[common.Address]map[[4]byte]error

	// viewCalls records how many times each (address, selector) was called,
	// so tests can assert which probes ran and which did not.
	viewCalls map[common.Address]map[[4]byte]int

	// balances serves the token's balanceOf(account) results keyed by
	// account; balanceOfTargets records which accounts were queried.
	balances         map[common.Address]*big.Int
	balanceOfTargets []common.Address
	balanceOfSel     [4]byte

	// codes serves the runtime code deployed at an address, and codeErrors
	// the failure a code read for that address produces instead.
	codes      map[common.Address][]byte
	codeErrors map[common.Address]error
	codeCalls  map[common.Address]int

	// storage and storageErrors serve per-slot storage words, which the
	// pre-reservations classifier reads to follow EIP-1967 implementation
	// pointers. A slot with no registered word reads as zero.
	storage       map[common.Address]map[common.Hash][]byte
	storageErrors map[common.Address]map[common.Hash]error
}

func newReservationFakeClient(t *testing.T) *reservationFakeClient {
	t.Helper()

	// balanceOf(address) selector: the first 4 bytes of the keccak256 of
	// the full signature, the same selector the TBTC binding's call data
	// carries.
	balanceOfSel := [4]byte{}
	copy(balanceOfSel[:], crypto.Keccak256([]byte("balanceOf(address)"))[:4])

	return &reservationFakeClient{
		views:         make(map[common.Address]map[[4]byte]func() []byte),
		viewErrors:    make(map[common.Address]map[[4]byte]error),
		viewCalls:     make(map[common.Address]map[[4]byte]int),
		balances:      make(map[common.Address]*big.Int),
		balanceOfSel:  balanceOfSel,
		codes:         make(map[common.Address][]byte),
		codeErrors:    make(map[common.Address]error),
		codeCalls:     make(map[common.Address]int),
		storage:       make(map[common.Address]map[common.Hash][]byte),
		storageErrors: make(map[common.Address]map[common.Hash]error),
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

// setViewError registers the failure a call to one method on one contract
// address produces instead of return data.
func (c *reservationFakeClient) setViewError(
	t *testing.T,
	target common.Address,
	selector [4]byte,
	err error,
) {
	t.Helper()
	if c.viewErrors[target] == nil {
		c.viewErrors[target] = make(map[[4]byte]error)
	}
	c.viewErrors[target][selector] = err
}

// viewCallsFor returns how many times a method on a contract address was
// called.
func (c *reservationFakeClient) viewCallsFor(
	target common.Address,
	selector [4]byte,
) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.viewCalls[target][selector]
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

	if c.viewCalls[*call.To] == nil {
		c.viewCalls[*call.To] = make(map[[4]byte]int)
	}
	c.viewCalls[*call.To][selector]++

	if err, ok := c.viewErrors[*call.To][selector]; ok {
		return nil, err
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

// CodeAt serves the runtime code registered for a contract address, which
// is how the pre-reservations classifier reads a deployment's identity.
func (c *reservationFakeClient) CodeAt(
	ctx context.Context,
	contract common.Address,
	blockNumber *big.Int,
) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.codeCalls[contract]++

	if err, ok := c.codeErrors[contract]; ok {
		return nil, err
	}
	return c.codes[contract], nil
}

// StorageAt serves the storage word registered for one slot of a contract,
// which is how the classifier follows a proxy's implementation pointer.
func (c *reservationFakeClient) StorageAt(
	ctx context.Context,
	contract common.Address,
	key common.Hash,
	blockNumber *big.Int,
) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err, ok := c.storageErrors[contract][key]; ok {
		return nil, err
	}
	if word, ok := c.storage[contract][key]; ok {
		return word, nil
	}
	return make([]byte, common.HashLength), nil
}

// setCode registers the runtime code deployed at a contract address.
func (c *reservationFakeClient) setCode(
	target common.Address,
	code []byte,
) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.codes[target] = code
}

// setCodeError registers the failure a code read for one address produces.
func (c *reservationFakeClient) setCodeError(
	target common.Address,
	err error,
) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.codeErrors[target] = err
}

// setImplementation registers the EIP-1967 implementation a proxy points at.
func (c *reservationFakeClient) setImplementation(
	proxy common.Address,
	implementation common.Address,
) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.storage[proxy] == nil {
		c.storage[proxy] = make(map[common.Hash][]byte)
	}
	c.storage[proxy][eip1967ImplementationSlot] = common.BytesToHash(
		implementation.Bytes(),
	).Bytes()
}

// setSlot registers one storage word of a contract, which is how tests place
// EIP-1967 implementation, admin and beacon pointers.
func (c *reservationFakeClient) setSlot(
	target common.Address,
	slot common.Hash,
	value []byte,
) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.storage[target] == nil {
		c.storage[target] = make(map[common.Hash][]byte)
	}
	c.storage[target][slot] = value
}

// setStorageError registers the failure a storage read for one slot of one
// contract produces.
func (c *reservationFakeClient) setStorageError(
	target common.Address,
	key common.Hash,
	err error,
) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.storageErrors[target] == nil {
		c.storageErrors[target] = make(map[common.Hash]error)
	}
	c.storageErrors[target][key] = err
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

// newReservationSurfaceChain wires a TbtcChain whose reservation-surface reads
// run against the fake client: the ReservationRouter binding and the Bridge
// binding both attached at the Bridge address, exactly as Connect attaches
// them in production (see reservationRouterBinding: router calls reach the
// Bridge's fallback, which delegatecalls into the router).
func newReservationSurfaceChain(
	t *testing.T,
	client *reservationFakeClient,
	bridgeAddress common.Address,
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

	base := &baseChain{
		key:              accountKey,
		client:           client,
		chainID:          chainID,
		nonceManager:     nonceManager,
		miningWaiter:     miningWaiter,
		transactionMutex: &sync.Mutex{},
	}

	router, err := tbtccontract.NewReservationRouter(
		bridgeAddress,
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

	bridge, err := tbtccontract.NewBridge(
		bridgeAddress,
		chainID,
		accountKey,
		client,
		nonceManager,
		miningWaiter,
		nil,
		&sync.Mutex{},
	)
	if err != nil {
		t.Fatalf("cannot attach the Bridge binding: [%v]", err)
	}

	return &TbtcChain{
		baseChain:         base,
		bridge:            bridge,
		bridgeAddress:     bridgeAddress,
		reservationRouter: router,
	}
}

// jsonRPCCallError models the error go-ethereum's JSON-RPC client surfaces
// for an EVM revert: a wrapped rpc.jsonError whose ErrorData() is the hex
// string the node returned, "0x" for an empty payload.
type jsonRPCCallError struct {
	message string
	data    string
}

func (e *jsonRPCCallError) Error() string { return e.message }

func (e *jsonRPCCallError) ErrorData() interface{} { return e.data }

// reservationsBuildCode models the runtime code of a reservations-build
// contract: the dispatcher of a build that declares getReservationRouter().
// The bytes around the selector stand for arbitrary dispatcher code, so the
// classifier cannot be relying on the selector being the whole code.
func reservationsBuildCode() []byte {
	return dispatcherEntryCode(reservationBuildSelector)
}

// dispatcherEntryCode models a solc dispatcher entry: PUSH4 <selector>, EQ,
// PUSH2 <destination>, JUMPI, in front of a STOP. The PUSH4 opcode is what
// makes the selector an executable entry point rather than arbitrary bytes.
func dispatcherEntryCode(selector []byte) []byte {
	code := []byte{0x60, 0x80, 0x60, 0x40, 0x52}
	code = append(code, 0x63) // PUSH4
	code = append(code, selector...)
	code = append(code, 0x14, 0x61, 0x01, 0x23, 0x57, 0x00) // EQ PUSH2 0x0123 JUMPI STOP
	return code
}

// preReservationsCode models the runtime code of a contract that predates the
// reservations build: it dispatches the functions both builds declare, and
// carries no reservations-build selector.
func preReservationsCode(t *testing.T) []byte {
	t.Helper()

	bridgeABI := parseGenABI(t, tbtcabi.BridgeABI)
	depositParameters := methodSelector(t, bridgeABI, "depositParameters")

	return dispatcherEntryCode(depositParameters[:])
}

// pushDataOnlyCode models pre-reservations code that carries the
// reservations-build selector only inside a PUSH immediate: PUSH5
// 0x5157ec0a00, POP, STOP. The bytes are present but no call can reach them as
// a dispatcher entry.
func pushDataOnlyCode(selector []byte) []byte {
	code := []byte{0x60, 0x80, 0x60, 0x40, 0x52}
	code = append(code, 0x64) // PUSH5
	code = append(code, selector...)
	code = append(code, 0x00, 0x50, 0x00) // 0x00 as the fifth operand, POP, STOP
	return code
}

// solidityMetadata models the CBOR metadata solc appends to deployed code: a
// map with the "ipfs" hash and the "solc" version, in the encoding the
// Solidity documentation specifies. Its length is the map's payload length,
// excluding the two-byte footer that the caller writes.
func solidityMetadata(fill func(hash []byte)) []byte {
	ipfsHash := make([]byte, 34)
	if fill != nil {
		fill(ipfsHash)
	}

	metadata := []byte{0xa2, 0x64, 'i', 'p', 'f', 's'}     // map(2), key "ipfs"
	metadata = append(metadata, 0x58, byte(len(ipfsHash))) // bytes(34)
	metadata = append(metadata, ipfsHash...)
	metadata = append(metadata, 0x64, 's', 'o', 'l', 'c') // key "solc"
	metadata = append(metadata, 0x43, 0x00, 0x08, 0x11)   // bytes(3) version
	return metadata
}

// withMetadata appends metadata and the two-byte footer holding its payload
// length, the exclusive convention Solidity uses: a 51-byte map ends in 0x0033.
func withMetadata(body, metadata []byte) []byte {
	code := make([]byte, 0, len(body)+len(metadata)+2)
	code = append(code, body...)
	code = append(code, metadata...)
	return append(code, byte(len(metadata)>>8), byte(len(metadata)))
}

// metadataOnlyCode models pre-reservations code that carries the
// reservations-build selector only inside its metadata: the ipfs hash spells
// PUSH4 <selector>, so a walk that reads the metadata as code would report a
// dispatch entry that no call can reach.
func metadataOnlyCode(selector []byte) []byte {
	body := []byte{0x60, 0x80, 0x60, 0x40, 0x52, 0x00}

	return withMetadata(body, solidityMetadata(func(hash []byte) {
		hash[0] = push4Opcode
		copy(hash[1:], selector)
	}))
}

// push4ConstantCode models pre-reservations code that pushes the
// reservations-build selector as an ordinary constant and discards it: PUSH4
// <selector>, POP, STOP. The opcode is there, the dispatch compare is not.
func push4ConstantCode(selector []byte) []byte {
	code := []byte{0x60, 0x80, 0x60, 0x40, 0x52}
	code = append(code, push4Opcode)
	code = append(code, selector...)
	code = append(code, 0x50, 0x00) // POP, STOP
	return code
}

// markerDispatchWithTrailingData models a runtime whose dispatcher compares the
// calldata selector and whose last two bytes are unrelated trailing data. The
// final word 0x0021 is the code length, so a length check that trusts any
// in-range value would cut the whole dispatcher out of the search window.
func markerDispatchWithTrailingData(selector []byte) []byte {
	code := []byte{0x60, 0x00, 0x35}      // PUSH1 0x00, CALLDATALOAD
	code = append(code, 0x60, 0xe0, 0x1c) // PUSH1 0xe0, SHR
	code = append(code, push4Opcode)
	code = append(code, selector...)
	code = append(code, eqOpcode, 0x60, 0x14, 0x57)   // EQ, PUSH1 0x14, JUMPI
	code = append(code, 0x60, 0x00, 0x60, 0x00, 0xfd) // PUSH1 0, PUSH1 0, REVERT
	code = append(code, 0x5b)                         // JUMPDEST
	code = append(code, 0x60, 0x00, 0x60, 0x00, 0x52) // PUSH1 0, PUSH1 0, MSTORE
	code = append(code, 0x60, 0x20, 0x60, 0x00, 0xf3) // PUSH1 0x20, PUSH1 0, RETURN
	return append(code, 0x00, byte(len(code)+2))      // trailing data = code length
}

// invalidMetadataTailCode models pre-reservations code whose final two bytes
// describe a length that does not hold a metadata map: the region they name
// starts with a dispatcher entry, not with a CBOR map header. The tail is
// therefore not metadata, so the whole code stays in the search window and the
// fake dispatch entry is found - a refusal, not a false pre-reservations
// classification.
func invalidMetadataTailCode(selector []byte) []byte {
	body := []byte{0x60, 0x80, 0x60, 0x40, 0x52}
	fakeTail := dispatcherEntryCode(selector)

	code := append(body, fakeTail...)
	return append(code, byte(len(fakeTail)>>8), byte(len(fakeTail)))
}

// TestReservationsVaultTrustedClassifiesReadFailures pins which reservation
// read failures make the adapter report a pre-reservations deployment, and
// which must surface as an ordinary error so the sweep gate refuses the
// sweep.
//
// The classification is a property of the deployed code, not of the error
// shape: a reservations-build Bridge bubbles a failed delegated reservation
// call as an execution revert with no returndata, so the empty-revert shape
// the fallback produces says nothing about which functions exist. Only a
// deployment whose implementation chain carries no reservations-build selector
// is provably pre-reservations; everything unreadable or unrecognised is an
// error.
func TestReservationsVaultTrustedClassifiesReadFailures(t *testing.T) {
	bridgeAddress := common.HexToAddress("0x00000000000000000000000000000000000000b1")
	implementationAddress := common.HexToAddress("0x00000000000000000000000000000000000000c1")
	deeperAddress := common.HexToAddress("0x00000000000000000000000000000000000000c2")
	deepestAddress := common.HexToAddress("0x00000000000000000000000000000000000000c3")

	routerABI := parseGenABI(t, tbtcabi.ReservationRouterABI)
	parametersSelector := methodSelector(t, routerABI, "reservationParameters")

	transientErr := errors.New("Post \"https://rpc.example\": context deadline exceeded")
	emptyRevertErr := errors.New("execution reverted")

	tests := map[string]struct {
		readErr     error
		deploy      func(t *testing.T, client *reservationFakeClient)
		expectPreM1 bool
	}{
		"reservations deployment: an empty revert is not proof": {
			readErr: emptyRevertErr,
			deploy: func(t *testing.T, client *reservationFakeClient) {
				client.setCode(bridgeAddress, []byte{0x60, 0x80, 0x60, 0x40})
				client.setCode(implementationAddress, reservationsBuildCode())
				client.setImplementation(bridgeAddress, implementationAddress)
			},
		},
		"reservations deployment: a revert with an all-zero payload is not proof": {
			readErr: &jsonRPCCallError{
				message: "execution reverted",
				data:    "0x00",
			},
			deploy: func(t *testing.T, client *reservationFakeClient) {
				client.setCode(bridgeAddress, []byte{0x60, 0x80, 0x60, 0x40})
				client.setCode(implementationAddress, reservationsBuildCode())
				client.setImplementation(bridgeAddress, implementationAddress)
			},
		},
		"reservations deployment: a transient read failure is not proof": {
			readErr: transientErr,
			deploy: func(t *testing.T, client *reservationFakeClient) {
				client.setCode(bridgeAddress, []byte{0x60, 0x80, 0x60, 0x40})
				client.setCode(implementationAddress, reservationsBuildCode())
				client.setImplementation(bridgeAddress, implementationAddress)
			},
		},
		"pre-reservations deployment: empty revert": {
			readErr:     emptyRevertErr,
			expectPreM1: true,
			deploy: func(t *testing.T, client *reservationFakeClient) {
				client.setCode(bridgeAddress, []byte{0x60, 0x80, 0x60, 0x40})
				client.setCode(implementationAddress, preReservationsCode(t))
				client.setImplementation(bridgeAddress, implementationAddress)
			},
		},
		"pre-reservations deployment: a transient read failure still sweeps": {
			readErr:     transientErr,
			expectPreM1: true,
			deploy: func(t *testing.T, client *reservationFakeClient) {
				client.setCode(bridgeAddress, []byte{0x60, 0x80, 0x60, 0x40})
				client.setCode(implementationAddress, preReservationsCode(t))
				client.setImplementation(bridgeAddress, implementationAddress)
			},
		},
		"pre-reservations deployment without a proxy": {
			readErr:     emptyRevertErr,
			expectPreM1: true,
			deploy: func(t *testing.T, client *reservationFakeClient) {
				client.setCode(bridgeAddress, preReservationsCode(t))
			},
		},
		"pre-reservations deployment: the selector appears only inside PUSH data": {
			readErr:     emptyRevertErr,
			expectPreM1: true,
			deploy: func(t *testing.T, client *reservationFakeClient) {
				client.setCode(
					bridgeAddress,
					pushDataOnlyCode(reservationBuildSelector),
				)
			},
		},
		"pre-reservations deployment: the selector appears only in metadata": {
			readErr:     emptyRevertErr,
			expectPreM1: true,
			deploy: func(t *testing.T, client *reservationFakeClient) {
				client.setCode(
					bridgeAddress,
					metadataOnlyCode(reservationBuildSelector),
				)
			},
		},
		"pre-reservations deployment: the selector is only a pushed constant": {
			readErr:     emptyRevertErr,
			expectPreM1: true,
			deploy: func(t *testing.T, client *reservationFakeClient) {
				client.setCode(
					bridgeAddress,
					push4ConstantCode(reservationBuildSelector),
				)
			},
		},
		"reservations deployment: a native dispatcher whose trailing data is not metadata": {
			readErr: emptyRevertErr,
			deploy: func(t *testing.T, client *reservationFakeClient) {
				client.setCode(
					bridgeAddress,
					markerDispatchWithTrailingData(reservationBuildSelector),
				)
			},
		},
		"reservations deployment: a tail that only looks like metadata": {
			readErr: emptyRevertErr,
			deploy: func(t *testing.T, client *reservationFakeClient) {
				client.setCode(
					bridgeAddress,
					invalidMetadataTailCode(reservationBuildSelector),
				)
			},
		},
		"beacon proxy: the implementation resolves through a beacon": {
			readErr: emptyRevertErr,
			deploy: func(t *testing.T, client *reservationFakeClient) {
				client.setCode(bridgeAddress, []byte{0x60, 0x80, 0x60, 0x40})
				client.setSlot(
					bridgeAddress,
					eip1967BeaconSlot,
					common.HexToHash(
						"0x00000000000000000000000000000000000000000000000000000000000000be",
					).Bytes(),
				)
				client.setCode(implementationAddress, reservationsBuildCode())
			},
		},
		"beacon proxy: the beacon slot itself is unreadable": {
			readErr: emptyRevertErr,
			deploy: func(t *testing.T, client *reservationFakeClient) {
				client.setCode(bridgeAddress, []byte{0x60, 0x80, 0x60, 0x40})
				client.setStorageError(
					bridgeAddress, eip1967BeaconSlot, transientErr,
				)
			},
		},
		"beacon proxy: the beacon slot is not a full word": {
			readErr: emptyRevertErr,
			deploy: func(t *testing.T, client *reservationFakeClient) {
				client.setCode(bridgeAddress, []byte{0x60, 0x80, 0x60, 0x40})
				client.setSlot(
					bridgeAddress, eip1967BeaconSlot, []byte{0x01, 0x02},
				)
			},
		},
		"proxy carrying an admin but no implementation": {
			readErr: emptyRevertErr,
			deploy: func(t *testing.T, client *reservationFakeClient) {
				client.setCode(bridgeAddress, []byte{0x60, 0x80, 0x60, 0x40})
				client.setSlot(
					bridgeAddress,
					eip1967AdminSlot,
					common.HexToHash(
						"0x00000000000000000000000000000000000000000000000000000000000000ad",
					).Bytes(),
				)
				client.setCode(implementationAddress, reservationsBuildCode())
			},
		},
		"proxy carrying an admin alongside its implementation": {
			readErr: emptyRevertErr,
			deploy: func(t *testing.T, client *reservationFakeClient) {
				client.setCode(bridgeAddress, []byte{0x60, 0x80, 0x60, 0x40})
				client.setImplementation(bridgeAddress, implementationAddress)
				client.setSlot(
					bridgeAddress,
					eip1967AdminSlot,
					common.HexToHash(
						"0x00000000000000000000000000000000000000000000000000000000000000ad",
					).Bytes(),
				)
				client.setCode(
					implementationAddress,
					preReservationsCode(t),
				)
			},
			expectPreM1: true,
		},
		"unreadable implementation code": {
			readErr: emptyRevertErr,
			deploy: func(t *testing.T, client *reservationFakeClient) {
				client.setCode(bridgeAddress, []byte{0x60, 0x80, 0x60, 0x40})
				client.setImplementation(bridgeAddress, implementationAddress)
				client.setCodeError(implementationAddress, transientErr)
			},
		},
		"unreadable implementation slot": {
			readErr: emptyRevertErr,
			deploy: func(t *testing.T, client *reservationFakeClient) {
				client.setCode(bridgeAddress, []byte{0x60, 0x80, 0x60, 0x40})
				client.setStorageError(
					bridgeAddress, eip1967ImplementationSlot, transientErr,
				)
			},
		},
		"truncated implementation slot": {
			readErr: emptyRevertErr,
			deploy: func(t *testing.T, client *reservationFakeClient) {
				client.setCode(bridgeAddress, []byte{0x60, 0x80, 0x60, 0x40})
				client.mu.Lock()
				client.storage[bridgeAddress] = map[common.Hash][]byte{
					eip1967ImplementationSlot: {0x01, 0x02},
				}
				client.mu.Unlock()
			},
		},
		"no code at the bridge address": {
			readErr: emptyRevertErr,
			deploy:  func(t *testing.T, client *reservationFakeClient) {},
		},
		"implementation chain deeper than the bound": {
			readErr: emptyRevertErr,
			deploy: func(t *testing.T, client *reservationFakeClient) {
				code := []byte{0x60, 0x80, 0x60, 0x40}
				client.setCode(bridgeAddress, code)
				client.setCode(implementationAddress, code)
				client.setCode(deeperAddress, code)
				client.setCode(deepestAddress, code)
				client.setImplementation(bridgeAddress, implementationAddress)
				client.setImplementation(implementationAddress, deeperAddress)
				client.setImplementation(deeperAddress, deepestAddress)
				client.setImplementation(deepestAddress, implementationAddress)
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			client := newReservationFakeClient(t)
			test.deploy(t, client)
			client.setViewError(t, bridgeAddress, parametersSelector, test.readErr)

			chain := newReservationSurfaceChain(t, client, bridgeAddress)

			_, err := chain.ReservationsVaultTrusted()
			if err == nil {
				t.Fatal("expected the read to fail")
			}

			preM1 := errors.Is(err, tbtc.ErrReservationsSurfaceUnavailable)
			if preM1 != test.expectPreM1 {
				t.Fatalf(
					"unexpected pre-reservations classification\nexpected pre-reservations: %v\nactual:                    %v\nerror: %v",
					test.expectPreM1,
					preM1,
					err,
				)
			}
		})
	}
}

// TestReservationsVaultTrustedReadsTrustedVault pins the happy path through
// the same bindings: parameters answered with a configured vault, and the
// Bridge reporting it trusted.
func TestReservationsVaultTrustedReadsTrustedVault(t *testing.T) {
	bridgeAddress := common.HexToAddress("0x00000000000000000000000000000000000000b2")
	vaultAddress := common.HexToAddress("0x00000000000000000000000000000000000000b3")

	routerABI := parseGenABI(t, tbtcabi.ReservationRouterABI)
	bridgeABI := parseGenABI(t, tbtcabi.BridgeABI)

	client := newReservationFakeClient(t)
	client.setView(
		t,
		bridgeAddress,
		methodSelector(t, routerABI, "reservationParameters"),
		func() []byte {
			return reservationParametersOutput(t, routerABI, vaultAddress)
		},
	)
	client.setView(
		t,
		bridgeAddress,
		methodSelector(t, bridgeABI, "isVaultTrusted"),
		func() []byte {
			return packOutputs(t, bridgeABI, "isVaultTrusted", true)
		},
	)

	chain := newReservationSurfaceChain(t, client, bridgeAddress)

	trusted, err := chain.ReservationsVaultTrusted()
	if err != nil {
		t.Fatalf("unexpected error: [%v]", err)
	}
	if !trusted {
		t.Fatal("expected the Bridge to report the reservation vault trusted")
	}
}

// reservationParametersOutput packs the reservation parameters tuple with the
// given reservation vault and zeroed remaining fields.
func reservationParametersOutput(
	t *testing.T,
	routerABI hostchainabi.ABI,
	vault common.Address,
) []byte {
	t.Helper()

	m, ok := routerABI.Methods["reservationParameters"]
	if !ok {
		t.Fatal("reservationParameters not in the router ABI")
	}

	values := make([]any, len(m.Outputs))
	for i, output := range m.Outputs {
		switch output.Type.String() {
		case "address":
			values[i] = vault
		case "uint64":
			values[i] = uint64(0)
		case "uint32":
			values[i] = uint32(0)
		case "uint16":
			values[i] = uint16(0)
		case "uint256":
			values[i] = new(big.Int)
		default:
			t.Fatalf(
				"reservationParameters output %d has unsupported type %s",
				i,
				output.Type.String(),
			)
		}
	}

	packed, err := m.Outputs.Pack(values...)
	if err != nil {
		t.Fatalf("cannot pack reservationParameters outputs: [%v]", err)
	}
	return packed
}

// TestReservationBuildSelector pins the deployment marker the
// pre-reservations classifier searches the deployed code for to the selector
// of getReservationRouter(). A typo here would make the classifier search for
// bytes no build carries, so it would report every deployment - reservations
// build included - as pre-reservations.
//
// The signature is hashed directly rather than read back from the generated
// Bridge ABI: CI's build image regenerates the tbtc bindings from the
// published artifacts plus the vendored fallback fragments, and those
// fragments carry isReservedDeposit but not getReservationRouter, so the ABI
// the CI image compiles against does not contain this entry point.
func TestReservationBuildSelector(t *testing.T) {
	expected := crypto.Keccak256([]byte("getReservationRouter()"))[:4]
	if !bytes.Equal(reservationBuildSelector, expected) {
		t.Fatalf(
			"reservations-build selector drift: classifier looks for 0x%x, "+
				"getReservationRouter() hashes to 0x%x",
			reservationBuildSelector,
			expected,
		)
	}
}

// TestExecutableCodeLength pins the metadata convention: Solidity's footer holds
// the payload length without the footer itself, so a conventional 51-byte map
// ends in 0x0033 and the executable region stops before the map. A tail that
// does not parse as Solidity metadata is not stripped at all: cutting
// executable bytes would let the classifier miss a real dispatch entry, while
// leaving them in the window can only cause a refusal.
func TestExecutableCodeLength(t *testing.T) {
	selector := reservationBuildSelector
	body := []byte{0x60, 0x80, 0x60, 0x40, 0x52, 0x00}
	metadata := solidityMetadata(nil)

	if len(metadata) != 51 {
		t.Fatalf("fixture metadata is %d bytes, want solc's 51", len(metadata))
	}

	conventional := withMetadata(body, metadata)
	if footer := conventional[len(conventional)-2:]; !bytes.Equal(footer, []byte{0x00, 0x33}) {
		t.Fatalf("fixture footer is %x, want the 51-byte exclusive encoding 0033", footer)
	}

	testCases := map[string]struct {
		code       []byte
		executable int
	}{
		"conventional metadata": {
			code:       conventional,
			executable: len(body),
		},
		"metadata without a map header": {
			code: withMetadata(body, func() []byte {
				broken := append([]byte{}, metadata...)
				broken[0] = 0x18 // an unsigned integer, not a map
				return broken
			}()),
			executable: len(body) + len(metadata) + 2,
		},
		"metadata without a solc key": {
			code:       withMetadata(body, []byte{0xa1, 0x64, 'a', 'b', 'c', 'd'}),
			executable: len(body) + 6 + 2,
		},
		"footer beyond the code": {
			code:       append(append([]byte{}, body...), 0xff, 0xff),
			executable: len(body) + 2,
		},
		"zero footer": {
			code:       append(append([]byte{}, body...), 0x00, 0x00),
			executable: len(body) + 2,
		},
		"too short to carry a footer": {
			code:       []byte{0x00},
			executable: 1,
		},
		"trailing data equal to the code length": {
			code:       markerDispatchWithTrailingData(selector),
			executable: 33,
		},
	}

	for name, test := range testCases {
		t.Run(name, func(t *testing.T) {
			if got := executableCodeLength(test.code); got != test.executable {
				t.Fatalf(
					"executable length %d, want %d (code %d bytes)",
					got,
					test.executable,
					len(test.code),
				)
			}
		})
	}
}

// TestHasDispatcherEntry pins the dispatch pattern: the compare is the entry
// point. Every case here is a slice the walk has to classify without help from
// a client or an ABI.
func TestHasDispatcherEntry(t *testing.T) {
	selector := reservationBuildSelector

	// PUSH4 <selector> EQ, then a JUMPI: the dispatcher compare.
	dispatch := []byte{0x60, 0x80, 0x60, 0x40, 0x52, push4Opcode}
	dispatch = append(dispatch, selector...)
	dispatch = append(dispatch, eqOpcode, 0x61, 0x01, 0x23, 0x57, 0x00)

	// The same push with no compare.
	constant := []byte{push4Opcode}
	constant = append(constant, selector...)
	constant = append(constant, 0x50, 0x00)

	// The same push, with the compare outside the executable window.
	truncated := []byte{push4Opcode}
	truncated = append(truncated, selector...)

	// PUSH4 whose immediate is cut short by the window.
	short := []byte{0x60, 0x80, push4Opcode, selector[0], selector[1]}

	// PUSH5 whose operand contains the selector and a following EQ byte.
	preservedByWiderPush := []byte{0x64}
	preservedByWiderPush = append(preservedByWiderPush, selector[:3]...)
	preservedByWiderPush = append(preservedByWiderPush, 0x00, eqOpcode, 0x00)

	testCases := map[string]struct {
		code   []byte
		expect bool
	}{
		"dispatcher compare":                {code: dispatch, expect: true},
		"pushed constant without a compare": {code: constant},
		"compare cut off by the window":     {code: truncated},
		"truncated push immediate":          {code: short},
		"selector inside a wider push":      {code: preservedByWiderPush},
		"empty code":                        {code: nil},
	}

	for name, test := range testCases {
		t.Run(name, func(t *testing.T) {
			if got := hasDispatcherEntry(test.code, selector); got != test.expect {
				t.Fatalf("dispatch %v, want %v for %x", got, test.expect, test.code)
			}
		})
	}
}

// TestEIP1967Slots pins the three proxy slots the classifier reads (EIP-1967:
// bytes32(uint256(keccak256(name)) - 1)), so a typo cannot silently point a
// read at some other word. Every one of them reads as zero when wrong, which
// is the dangerous direction: a wrong implementation slot stops the walk at the
// proxy and reports a reservations deployment as pre-reservations, and a wrong
// beacon slot misses a beacon proxy that has no implementation pointer at all.
func TestEIP1967Slots(t *testing.T) {
	tests := map[string]struct {
		name string
		slot common.Hash
	}{
		"implementation": {
			name: "eip1967.proxy.implementation",
			slot: eip1967ImplementationSlot,
		},
		"admin": {
			name: "eip1967.proxy.admin",
			slot: eip1967AdminSlot,
		},
		"beacon": {
			name: "eip1967.proxy.beacon",
			slot: eip1967BeaconSlot,
		},
	}

	for slotName, test := range tests {
		t.Run(slotName, func(t *testing.T) {
			value := new(big.Int).SetBytes(crypto.Keccak256([]byte(test.name)))
			value.Sub(value, big.NewInt(1))

			expected := common.BytesToHash(value.Bytes())
			if test.slot != expected {
				t.Fatalf(
					"%s slot drift: classifier reads %s, EIP-1967 specifies %s",
					slotName,
					test.slot.Hex(),
					expected.Hex(),
				)
			}
		})
	}
}
