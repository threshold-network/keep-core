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
		viewErrors:   make(map[common.Address]map[[4]byte]error),
		viewCalls:    make(map[common.Address]map[[4]byte]int),
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

// revertingCallError models the error go-ethereum surfaces for an EVM revert
// that carries revert data.
type revertingCallError struct {
	message string
	data    []byte
}

func (e *revertingCallError) Error() string { return e.message }

func (e *revertingCallError) ErrorData() interface{} { return e.data }

// TestReservationsVaultTrustedClassifiesReadFailures pins which reservation
// read failures prove that the Bridge predates the reservations upgrade, and
// which do not. Only a revert without revert data, from a Bridge that still
// answers a getter that exists in both builds, is proof of a pre-reservations
// deployment; everything else must surface as an ordinary error so the sweep
// gate refuses the sweep instead of reading "reservations are not live".
func TestReservationsVaultTrustedClassifiesReadFailures(t *testing.T) {
	bridgeAddress := common.HexToAddress("0x00000000000000000000000000000000000000b1")

	routerABI := parseGenABI(t, tbtcabi.ReservationRouterABI)
	bridgeABI := parseGenABI(t, tbtcabi.BridgeABI)

	parametersSelector := methodSelector(t, routerABI, "reservationParameters")
	depositParametersSelector := methodSelector(t, bridgeABI, "depositParameters")

	transientErr := errors.New("Post \"https://rpc.example\": context deadline exceeded")
	revertWithReasonErr := &revertingCallError{
		message: "execution reverted: reservation parameters are not set",
		data:    []byte{0x08, 0xc3, 0x79, 0xa0},
	}

	tests := map[string]struct {
		readErr            error
		bridgeGetterErr    error
		expectUnavailable  bool
		expectProbeSkipped bool
	}{
		"pre-reservations Bridge: empty revert, pre-upgrade getter answers": {
			readErr:           errors.New("execution reverted"),
			expectUnavailable: true,
		},
		"pre-reservations Bridge: empty revert data payload, pre-upgrade getter answers": {
			readErr:           &revertingCallError{message: "execution reverted", data: []byte{}},
			expectUnavailable: true,
		},
		"transient RPC failure is not proof of absence": {
			readErr:            transientErr,
			expectUnavailable:  false,
			expectProbeSkipped: true,
		},
		"revert carrying revert data is not proof of absence": {
			readErr:            revertWithReasonErr,
			expectUnavailable:  false,
			expectProbeSkipped: true,
		},
		"empty revert from a Bridge that cannot answer its own getters is not proof": {
			readErr:           errors.New("execution reverted"),
			bridgeGetterErr:   errors.New("execution reverted"),
			expectUnavailable: false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			client := newReservationFakeClient(t)
			client.setViewError(t, bridgeAddress, parametersSelector, test.readErr)
			if test.bridgeGetterErr != nil {
				client.setViewError(
					t, bridgeAddress, depositParametersSelector, test.bridgeGetterErr,
				)
			} else {
				client.setView(
					t,
					bridgeAddress,
					depositParametersSelector,
					func() []byte {
						return packOutputs(
							t, bridgeABI, "depositParameters",
							uint64(1000), uint64(20), uint64(500),
							uint32(150),
						)
					},
				)
			}

			chain := newReservationSurfaceChain(t, client, bridgeAddress)

			_, err := chain.ReservationsVaultTrusted()
			if err == nil {
				t.Fatal("expected the read to fail")
			}

			unavailable := errors.Is(err, tbtc.ErrReservationsSurfaceUnavailable)
			if unavailable != test.expectUnavailable {
				t.Fatalf(
					"unexpected pre-reservations classification\nexpected unavailable: %v\nactual:               %v\nerror: %v",
					test.expectUnavailable,
					unavailable,
					err,
				)
			}

			probeCalls := client.viewCallsFor(bridgeAddress, depositParametersSelector)
			if test.expectProbeSkipped && probeCalls != 0 {
				t.Fatalf(
					"expected no pre-upgrade getter probe for a non-revert failure, got %d",
					probeCalls,
				)
			}
			if !test.expectProbeSkipped && probeCalls < 1 {
				t.Fatalf(
					"expected the pre-upgrade getter probe to run, got %d calls",
					probeCalls,
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

// TestReservationParametersSelector pins the hardcoded selector the raw
// reservation-surface probe issues against the generated ReservationRouter
// ABI, so a router ABI change cannot silently make the probe ask for a
// different function (which would revert without data even on an upgraded
// Bridge and be misread as a pre-reservations deployment).
func TestReservationParametersSelector(t *testing.T) {
	routerABI := parseGenABI(t, tbtcabi.ReservationRouterABI)

	expected := methodSelector(t, routerABI, "reservationParameters")
	if reservationParametersSelector != expected {
		t.Fatalf(
			"reservationParameters selector drift: probe issues 0x%x, the ABI says 0x%x",
			reservationParametersSelector,
			expected,
		)
	}
}
