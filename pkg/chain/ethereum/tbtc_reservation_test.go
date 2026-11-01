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
	code := []byte{0x60, 0x80, 0x60, 0x40, 0x52}
	code = append(code, reservationBuildSelector...)
	code = append(code, 0x14, 0x61, 0x01, 0x23, 0x57, 0x00)
	return code
}

// preReservationsCode models the runtime code of a contract that predates the
// reservations build: it dispatches the functions both builds declare, and
// carries no reservations-build selector.
func preReservationsCode(t *testing.T) []byte {
	t.Helper()

	bridgeABI := parseGenABI(t, tbtcabi.BridgeABI)
	depositParameters := methodSelector(t, bridgeABI, "depositParameters")

	code := []byte{0x60, 0x80, 0x60, 0x40, 0x52}
	code = append(code, depositParameters[:]...)
	code = append(code, 0x14, 0x61, 0x01, 0x23, 0x57, 0x00)
	return code
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

// TestReservationBuildSelector pins the hardcoded deployment marker the
// pre-reservations classifier searches the deployed code for against the
// generated Bridge ABI, so an ABI change cannot silently make the classifier
// look for a selector no build carries (which would report every deployment,
// reservations build included, as pre-reservations).
func TestReservationBuildSelector(t *testing.T) {
	bridgeABI := parseGenABI(t, tbtcabi.BridgeABI)

	expected := methodSelector(t, bridgeABI, "getReservationRouter")
	if !bytes.Equal(reservationBuildSelector, expected[:]) {
		t.Fatalf(
			"reservations-build selector drift: classifier looks for 0x%x, the ABI says 0x%x",
			reservationBuildSelector,
			expected,
		)
	}
}

// TestEIP1967ImplementationSlot pins the slot address the classifier reads the
// implementation pointer from, so a typo cannot silently point every
// deployment read at some other word (which reads as zero: the classifier
// would then stop at the proxy, find no selector in proxy code, and report
// every reservations deployment as pre-reservations).
func TestEIP1967ImplementationSlot(t *testing.T) {
	// EIP-1967: bytes32(uint256(keccak256('eip1967.proxy.implementation')) - 1).
	digest := crypto.Keccak256([]byte("eip1967.proxy.implementation"))
	value := new(big.Int).SetBytes(digest)
	value.Sub(value, big.NewInt(1))

	expected := common.BytesToHash(value.Bytes())
	if eip1967ImplementationSlot != expected {
		t.Fatalf(
			"implementation slot drift: classifier reads %s, EIP-1967 specifies %s",
			eip1967ImplementationSlot.Hex(),
			expected.Hex(),
		)
	}
}
