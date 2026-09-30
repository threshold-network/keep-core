package ethereum

// This file exercises TbtcChain.ValidateReservationAnchorProposal and
// TbtcChain.ValidateReservationReanchorProposal (tbtc.go) against the
// real, unmodified tbtc-v2 WalletProposalValidator bytecode deployed into
// a shared in-memory go-ethereum EVM (core/vm/runtime over a single
// shared StateDB), backed by a minimal stub Bridge
// (testdata/walletproposalvalidator/StubBridge.sol). No other test in
// this package calls a real validator: the others fake the validator
// response, so only this harness can catch a regression where validation
// runs against a request nonce for which no action was ever authorized.
//
// The test-only in-memory backend satisfies the keep-common
// ethutil.EthereumClient interface that the generated tbtccontract
// constructor requires; it implements exactly the methods that code path
// exercises (CodeAt/CallContract for view calls, plus the block reader
// helpers the wiring touches) and returns a clear "not supported in
// harness" error for the rest.
//
// The block number is the fixed constant below and the block timestamp is
// captured at harness construction from the wall clock, so every seed
// value keeps its margin around wall-clock "now".
//
// Regenerating the vendored artifacts:
//   see testdata/walletproposalvalidator/regenerate.sh

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	hostchain "github.com/ethereum/go-ethereum"
	hostchainabi "github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/core/vm/runtime"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"

	"github.com/keep-network/keep-common/pkg/chain/ethereum"
	"github.com/keep-network/keep-common/pkg/chain/ethereum/ethutil"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	tbtccontract "github.com/keep-network/keep-core/pkg/chain/ethereum/tbtc/gen/contract"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// Compile-time proof that the in-memory backend satisfies the interface the
// generated binding and the keep-common wiring construct against.
var _ ethutil.EthereumClient = (*harnessBackend)(nil)

// Enum values mirrored from tbtc-v2's Wallets.WalletState, Reservation.Action-
// Type, and Reservation.ActionState (see StubBridge.sol's header comment).
const (
	walletStateLive        uint8 = 1
	walletStateMovingFunds uint8 = 2

	actionTypeAcceptance uint8 = 1
	actionTypeReanchor   uint8 = 3

	actionStatePending uint8 = 1
)

// requestTimeoutSafetyMarginSeconds is the margin keep-core assumes the
// validator keeps before an action's timeoutAt: tbtcpg gates proposals on
// reservationRequestTimeoutSafetyMarginSeconds, which must equal tbtc-v2's
// WalletProposalValidatorConstants.REQUEST_TIMEOUT_SAFETY_MARGIN. The
// validator accepts only while block.timestamp < timeoutAt - margin, so
// the boundary tests below fail against the real bytecode if the on-chain
// margin ever moves away from this value.
const requestTimeoutSafetyMarginSeconds uint32 = 2 * 60 * 60

// ----- Stub bridge struct mirrors -----
//
// Field names must capitalize only the first letter of the corresponding
// Solidity field name (see StubBridge.sol): go-ethereum's abi encoder maps
// tuple arguments to struct fields by that exact convention (no `abi:""`
// tags are used here), the same convention the generated tbtcabi bindings
// rely on.

type stubDepositReq struct {
	Depositor   common.Address
	Amount      uint64
	RevealedAt  uint32
	Vault       common.Address
	TreasuryFee uint64
	SweptAt     uint32
	ExtraData   [32]byte
}

type stubWallet struct {
	EcdsaWalletID                          [32]byte
	MainUtxoHash                           [32]byte
	PendingRedemptionsValue                uint64
	CreatedAt                              uint32
	MovingFundsRequestedAt                 uint32
	ClosingStartedAt                       uint32
	PendingMovedFundsSweepRequestsCount    uint32
	State                                  uint8
	MovingFundsTargetWalletsCommitmentHash [32]byte
}

type stubResReq struct {
	Owner                 common.Address
	MintedAmount          uint64
	AcceptedAt            uint32
	WalletPubKeyHash      [20]byte
	AnchorAmount          uint64
	ExpiresAt             uint32
	AnchorTxHash          [32]byte
	AnchorTxOutputIndex   uint32
	State                 uint8
	RequestNonce          uint64
	RetryCredit           bool
	DissolutionEligibleAt uint32
	CumulativeReanchorFee uint64
	ReanchorCooldownUntil uint32
}

type stubAction struct {
	TargetWalletPubKeyHash  [20]byte
	RequestedAt             uint32
	TimeoutAt               uint32
	TxMaxFee                uint64
	ActionType              uint8
	State                   uint8
	FeePaid                 bool
	Redeemer                common.Address
	ActionDataHash          [32]byte
	SourceAnchorUtxoHash    [32]byte
	Amount                  uint64
	UsedRetryCredit         bool
	WatchtowerDefaultDelay  uint32
	WatchtowerLevelOneDelay uint32
	WatchtowerLevelTwoDelay uint32
	RetryCreditSourceNonce  uint64
	IsPartial               bool
	TermSeconds             uint32
	DissolutionDelay        uint32
	MinAmount               uint64
}

type stubParameters struct {
	ReservationVault                common.Address
	ReservationMinAmount            uint64
	ReservationTxMaxFee             uint64
	ReservationTermSeconds          uint32
	ReservationDissolutionDelay     uint32
	ReservationMaxTotalAmount       uint64
	ReservationTotalAmount          uint64
	MaxReservationsPerWallet        uint32
	ReservationActionTimeout        uint32
	ReservationRenewalWindowSeconds uint32
}

type stubCaps struct {
	MaxReservationsAmountPerWallet uint64
	ReservationMaxSingleAmount     uint64
	MaxActiveReservations          uint32
}

// contractArtifact is the on-disk JSON shape produced by
// testdata/walletproposalvalidator/regenerate.sh for both the stub bridge
// and the vendored WalletProposalValidator.
type contractArtifact struct {
	ABI      json.RawMessage `json:"abi"`
	Bytecode string          `json:"bytecode"`
}

func loadValidatorTestdataArtifact(t *testing.T, name string) (hostchainabi.ABI, []byte) {
	t.Helper()

	raw, err := os.ReadFile(
		filepath.Join("testdata", "walletproposalvalidator", name),
	)
	if err != nil {
		t.Fatalf("cannot read artifact [%s]: [%v]", name, err)
	}

	var artifact contractArtifact
	if err := json.Unmarshal(raw, &artifact); err != nil {
		t.Fatalf("cannot unmarshal artifact [%s]: [%v]", name, err)
	}

	parsedABI, err := hostchainabi.JSON(strings.NewReader(string(artifact.ABI)))
	if err != nil {
		t.Fatalf("cannot parse ABI of artifact [%s]: [%v]", name, err)
	}

	return parsedABI, common.FromHex(artifact.Bytecode)
}

// harnessBlockNumber is the fixed block number the in-memory EVM executes
// at. The block timestamp is not fixed: newHarnessBackend captures the
// wall-clock time at construction, keeping every seed value's margin
// around wall-clock "now" correct.
const harnessBlockNumber uint64 = 1

// harnessBackend is a test-only, in-memory backend that runs EVM code on
// a single shared state.StateDB (like core/vm/runtime does when
// Config.State is nil, but kept across deploy and seeding calls so the
// state persists) and exposes it through the keep-common
// ethutil.EthereumClient interface.
//
// An in-memory EVM over one shared StateDB is used instead of
// go-ethereum's ethclient/simulated backend because, at the pinned
// go-ethereum v1.13.15, that backend depends on github.com/fjl/memsize,
// whose //go:linkname reference to runtime.stopTheWorld the Go 1.23+
// linker rejects ("invalid reference to runtime.stopTheWorld"): a test
// binary importing it does not link with this module's toolchain.
//
// The chain context is a fixed block number (harnessBlockNumber) and a
// block timestamp captured at construction: the validator compares
// block.timestamp against action timeouts and deposit min-ages, and the
// test seeds use margins around wall-clock "now", so a stale fixed
// timestamp would flip those comparisons.
//
// Methods that the validation wrappers never exercise return a clear
// "not supported in harness" error.
type harnessBackend struct {
	state       *state.StateDB
	chainConfig *params.ChainConfig
	evm         *vm.EVM
	random      common.Hash
	blockNumber uint64
	timestamp   uint64
}

// noOpSubscription is a hostchain.Subscription for the keep-common
// subscriptions this harness can never satisfy: the in-memory backend
// produces no new heads, and with an error channel that closes the
// block counter's resubscription loop would retry every 5 seconds.
// Unsubscribe has no effect; Err returns a channel that never delivers,
// so that loop parks instead of cycling.
type noOpSubscription struct {
	errs chan error
}

func newNoOpSubscription() hostchain.Subscription {
	return &noOpSubscription{errs: make(chan error)}
}

func (*noOpSubscription) Unsubscribe() {}

func (s *noOpSubscription) Err() <-chan error {
	return s.errs
}

func newHarnessBackend() *harnessBackend {
	shared, _ := state.New(
		types.EmptyRootHash,
		state.NewDatabase(rawdb.NewMemoryDatabase()),
		nil,
	)

	// AllDevChainProtocolChanges mirrors the go-ethereum simulated
	// backend's dev chain: chain id 1337, every fork enabled,
	// post-merge. A bare ChainConfig (only ChainID set) would keep
	// pre-Byzantium rules, which breaks the vendored validator: its
	// compiled code uses SHL/SHR (enabled at Petersburg), and
	// pre-Byzantium revert handling drops the reason payload that
	// CallContract decodes.
	chainConfig := params.AllDevChainProtocolChanges
	timestamp := uint64(time.Now().Unix())
	random := common.HexToHash("0x42")

	evm := runtime.NewEnv(&runtime.Config{
		ChainConfig: chainConfig,
		Origin:      common.Address{},
		Coinbase:    common.Address{},
		BlockNumber: new(big.Int).SetUint64(harnessBlockNumber),
		Time:        timestamp,
		Random:      &random,
		BaseFee:     big.NewInt(params.InitialBaseFee),
		GasPrice:    new(big.Int),
		State:       shared,
	})

	return &harnessBackend{
		state:       shared,
		chainConfig: chainConfig,
		evm:         evm,
		random:      random,
		blockNumber: harnessBlockNumber,
		timestamp:   timestamp,
	}
}

// runCall executes callData at target against the shared state as an EVM
// call from caller. A successful stop returns the return data; a
// contract revert surfaces vm.ErrExecutionReverted with the revert data
// in the return value. State changes of a successful call persist in
// the shared state; a reverted call is atomically undone by the EVM's
// snapshot rollback.
func (b *harnessBackend) runCall(
	caller common.Address,
	target common.Address,
	callData []byte,
) ([]byte, error) {
	ret, _, err := b.evm.Call(
		vm.AccountRef(caller),
		target,
		callData,
		math.MaxUint64,
		uint256.NewInt(0),
	)
	return ret, err
}

// CallContract executes the call against the shared state at the
// harness's block timestamp and the fixed block number. On an EVM
// revert it surfaces go-ethereum's revert error shape: an error whose
// message carries the decoded revert reason string (mirroring
// internal/ethapi's newRevertError), so keep-common's ErrorResolver
// re-invocation and the keep-core wrapper error retain the reason.
func (b *harnessBackend) CallContract(
	ctx context.Context,
	call hostchain.CallMsg,
	blockNumber *big.Int,
) ([]byte, error) {
	var target common.Address
	if call.To != nil {
		target = *call.To
	}
	ret, err := b.runCall(call.From, target, call.Data)
	if err != nil {
		if errors.Is(err, vm.ErrExecutionReverted) {
			if reason, unpackErr := hostchainabi.UnpackRevert(ret); unpackErr == nil {
				return nil, fmt.Errorf("execution reverted: %s", reason)
			}
		}
		return nil, err
	}
	return ret, nil
}

func (b *harnessBackend) CodeAt(
	ctx context.Context,
	contract common.Address,
	blockNumber *big.Int,
) ([]byte, error) {
	return b.state.GetCode(contract), nil
}

func (b *harnessBackend) BalanceAt(
	ctx context.Context,
	account common.Address,
	blockNumber *big.Int,
) (*big.Int, error) {
	return b.state.GetBalance(account).ToBig(), nil
}

func (b *harnessBackend) EstimateGas(
	ctx context.Context,
	call hostchain.CallMsg,
) (uint64, error) {
	return 30_000_000, nil
}

func (b *harnessBackend) SuggestGasPrice(
	ctx context.Context,
) (*big.Int, error) {
	return big.NewInt(params.GWei), nil
}

func (b *harnessBackend) SuggestGasTipCap(ctx context.Context) (*big.Int, error) {
	return big.NewInt(params.GWei / 2), nil
}

func (b *harnessBackend) HeaderByNumber(
	ctx context.Context,
	number *big.Int,
) (*types.Header, error) {
	return &types.Header{
		Number: new(big.Int).SetUint64(b.blockNumber),
		Time:   b.timestamp,
	}, nil
}

func (b *harnessBackend) PendingCodeAt(
	ctx context.Context,
	account common.Address,
) ([]byte, error) {
	return b.state.GetCode(account), nil
}

func (b *harnessBackend) PendingNonceAt(
	ctx context.Context,
	account common.Address,
) (uint64, error) {
	return b.state.GetNonce(account), nil
}

func (b *harnessBackend) FilterLogs(
	ctx context.Context,
	q hostchain.FilterQuery,
) ([]types.Log, error) {
	return nil, nil
}

func (b *harnessBackend) SubscribeFilterLogs(
	ctx context.Context,
	q hostchain.FilterQuery,
	ch chan<- types.Log,
) (hostchain.Subscription, error) {
	return newNoOpSubscription(), nil
}

func (b *harnessBackend) BlockByHash(
	ctx context.Context,
	hash common.Hash,
) (*types.Block, error) {
	return nil, fmt.Errorf("BlockByHash is not supported in harness")
}

func (b *harnessBackend) BlockByNumber(
	ctx context.Context,
	number *big.Int,
) (*types.Block, error) {
	return types.NewBlock(
		&types.Header{Number: new(big.Int).SetUint64(b.blockNumber)},
		nil, nil, nil, nil,
	), nil
}

func (b *harnessBackend) HeaderByHash(
	ctx context.Context,
	hash common.Hash,
) (*types.Header, error) {
	return nil, fmt.Errorf("HeaderByHash is not supported in harness")
}

func (b *harnessBackend) TransactionCount(
	ctx context.Context,
	blockHash common.Hash,
) (uint, error) {
	return 0, nil
}

func (b *harnessBackend) TransactionInBlock(
	ctx context.Context,
	blockHash common.Hash,
	index uint,
) (*types.Transaction, error) {
	return nil, fmt.Errorf("TransactionInBlock is not supported in harness")
}

func (b *harnessBackend) SubscribeNewHead(
	ctx context.Context,
	ch chan<- *types.Header,
) (hostchain.Subscription, error) {
	return newNoOpSubscription(), nil
}

func (b *harnessBackend) TransactionByHash(
	ctx context.Context,
	txHash common.Hash,
) (*types.Transaction, bool, error) {
	return nil, false,
		fmt.Errorf("TransactionByHash is not supported in harness")
}

func (b *harnessBackend) TransactionReceipt(
	ctx context.Context,
	txHash common.Hash,
) (*types.Receipt, error) {
	return nil, fmt.Errorf("TransactionReceipt is not supported in harness")
}

func (b *harnessBackend) SendTransaction(
	ctx context.Context,
	tx *types.Transaction,
) error {
	return fmt.Errorf("SendTransaction is not supported in harness")
}

// validatorHarness deploys the stub bridge and the real, vendored
// WalletProposalValidator contract into a shared in-memory EVM, and
// exposes a TbtcChain whose walletProposalValidator binding is wired
// to that deployed validator.
type validatorHarness struct {
	t        *testing.T
	backend  *harnessBackend
	stubABI  hostchainabi.ABI
	stubAddr common.Address
	tc       *TbtcChain
}

// deployContract runs a creation transaction against the shared
// in-memory state and returns the address the new code was stored at.
func deployContract(
	backend *harnessBackend,
	origin common.Address,
	creationCode []byte,
) (common.Address, error) {
	_, address, _, err := runtime.Create(
		creationCode,
		&runtime.Config{
			ChainConfig: backend.chainConfig,
			Origin:      origin,
			Coinbase:    origin,
			BlockNumber: new(big.Int).SetUint64(backend.blockNumber),
			Time:        backend.timestamp,
			Random:      &backend.random,
			BaseFee:     big.NewInt(params.InitialBaseFee),
			GasLimit:    8_000_000,
			State:       backend.state,
		},
	)
	return address, err
}

func newValidatorHarness(t *testing.T) *validatorHarness {
	t.Helper()

	backend := newHarnessBackend()

	// A key is still required for the generated binding's transactor
	// wiring; this harness only runs view calls, so it is never used
	// to sign a transaction.
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("cannot generate private key: [%v]", err)
	}
	address := crypto.PubkeyToAddress(privateKey.PublicKey)

	chainID := big.NewInt(1337)
	accountKey := &keystore.Key{Address: address, PrivateKey: privateKey}

	nonceManager := ethutil.NewNonceManager(backend, address)
	miningWaiter := ethutil.NewMiningWaiter(backend, ethereum.Config{})
	blockCounter, err := ethutil.NewBlockCounter(backend)
	if err != nil {
		t.Fatalf("cannot create block counter: [%v]", err)
	}

	// Deploy the stub bridge. Its constructor takes no arguments, so
	// the creation input is the bare bytecode.
	stubABI, stubBytecode := loadValidatorTestdataArtifact(t, "StubBridge.json")
	stubAddress, err := deployContract(backend, address, stubBytecode)
	if err != nil {
		t.Fatalf("cannot deploy stub bridge: [%v]", err)
	}

	// Deploy the real, vendored validator. Its constructor takes the
	// bridge address, which must be ABI-encoded onto the creation
	// bytecode.
	validatorABI, validatorBytecode := loadValidatorTestdataArtifact(
		t, "WalletProposalValidator.json",
	)
	ctorInput, err := validatorABI.Pack("", stubAddress)
	if err != nil {
		t.Fatalf("cannot pack validator constructor args: [%v]", err)
	}
	validatorAddress, err := deployContract(
		backend,
		address,
		append(validatorBytecode, ctorInput...),
	)
	if err != nil {
		t.Fatalf("cannot deploy wallet proposal validator: [%v]", err)
	}

	wpv, err := tbtccontract.NewWalletProposalValidator(
		validatorAddress,
		chainID,
		accountKey,
		backend,
		nonceManager,
		miningWaiter,
		blockCounter,
		&sync.Mutex{},
	)
	if err != nil {
		t.Fatalf("cannot create WalletProposalValidator binding: [%v]", err)
	}

	return &validatorHarness{
		t:        t,
		backend:  backend,
		stubABI:  stubABI,
		stubAddr: stubAddress,
		tc:       &TbtcChain{walletProposalValidator: wpv},
	}
}

// transact runs a seeding setter on the stub bridge against the shared
// state. The call executes immediately; on success the write persists,
// on a revert the shared state is atomically restored.
func (h *validatorHarness) transact(method string, args ...interface{}) {
	h.t.Helper()
	packed, err := h.stubABI.Pack(method, args...)
	if err != nil {
		h.t.Fatalf("cannot pack stub.%s args: [%v]", method, err)
	}
	if _, err := h.backend.runCall(common.Address{}, h.stubAddr, packed); err != nil {
		h.t.Fatalf("cannot call stub.%s: [%v]", method, err)
	}
}

// commit is a no-op in the in-memory backend: seeding calls already
// write to the shared state as they run. It is kept for the tests'
// readability so they read as "seed, commit, validate".
func (h *validatorHarness) commit() {
}

func (h *validatorHarness) setDeposit(key *big.Int, req stubDepositReq) {
	h.transact("setDeposit", key, req)
}

func (h *validatorHarness) setReservedDeposit(key *big.Int, isReserved bool) {
	h.transact("setReservedDeposit", key, isReserved)
}

func (h *validatorHarness) setWallet(pubKeyHash [20]byte, wallet stubWallet) {
	h.transact("setWallet", pubKeyHash, wallet)
}

func (h *validatorHarness) setReservation(key *big.Int, req stubResReq) {
	h.transact("setReservation", key, req)
}

func (h *validatorHarness) setReservationAction(
	key *big.Int,
	nonce uint64,
	action stubAction,
) {
	h.transact("setReservationAction", key, nonce, action)
}

func (h *validatorHarness) setReservationParameters(p stubParameters) {
	h.transact("setReservationParameters", p)
}

func (h *validatorHarness) setReservationCaps(c stubCaps) {
	h.transact("setReservationCaps", c)
}

func (h *validatorHarness) setWalletReservationsCount(
	pubKeyHash [20]byte,
	count uint32,
) {
	h.transact("setWalletReservationsCount", pubKeyHash, count)
}

// depositKeyFor reproduces the on-chain deposit/reservation key derivation:
// uint256(keccak256(abi.encodePacked(fundingTxHash, fundingOutputIndex))).
func depositKeyFor(fundingTxHash bitcoin.Hash, outputIndex uint32) *big.Int {
	indexBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(indexBytes, outputIndex)
	hash := crypto.Keccak256(fundingTxHash[:], indexBytes)
	return new(big.Int).SetBytes(hash)
}

// buildFundingDeposit constructs a tbtc.Deposit alongside a matching
// single-input, single-P2WSH-output Bitcoin funding transaction whose sole
// output locks funds on the deposit's P2(W)SH script. refundableAt is
// encoded into RefundLocktime so that
// BTCUtils.reverseUint32(uint32(refundLocktime)) on-chain evaluates back to
// refundableAt (see WalletProposalValidator.validateReservationAnchorProposal).
func buildFundingDeposit(
	t *testing.T,
	depositor common.Address,
	walletPubKeyHash [20]byte,
	refundPubKeyHash [20]byte,
	refundableAt uint32,
	outputValue int64,
) (*tbtc.Deposit, *bitcoin.Transaction, *big.Int) {
	t.Helper()

	var blindingFactor [8]byte
	copy(blindingFactor[:], []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08})

	var refundLocktime [4]byte
	binary.LittleEndian.PutUint32(refundLocktime[:], refundableAt)

	deposit := &tbtc.Deposit{
		Depositor:           chain.Address(depositor.Hex()),
		BlindingFactor:      blindingFactor,
		WalletPublicKeyHash: walletPubKeyHash,
		RefundPublicKeyHash: refundPubKeyHash,
		RefundLocktime:      refundLocktime,
	}

	depositScript, err := deposit.Script()
	if err != nil {
		t.Fatalf("cannot build deposit script: [%v]", err)
	}
	scriptHash := sha256.Sum256(depositScript)

	p2wshScript := append([]byte{0x00, 0x20}, scriptHash[:]...)

	var prevTxHash bitcoin.Hash
	prevTxHash[0] = 0x01

	fundingTx := &bitcoin.Transaction{
		Version: 2,
		Inputs: []*bitcoin.TransactionInput{
			{
				Outpoint: &bitcoin.TransactionOutpoint{
					TransactionHash: prevTxHash,
					OutputIndex:     0,
				},
				Sequence: 0xffffffff,
			},
		},
		Outputs: []*bitcoin.TransactionOutput{
			{
				Value:           outputValue,
				PublicKeyScript: bitcoin.Script(p2wshScript),
			},
		},
		Locktime: 0,
	}

	depositKey := depositKeyFor(fundingTx.Hash(), 0)

	return deposit, fundingTx, depositKey
}

func depositExtraInfoOf(
	deposit *tbtc.Deposit,
	fundingTx *bitcoin.Transaction,
) struct {
	*tbtc.Deposit
	FundingTx *bitcoin.Transaction
} {
	return struct {
		*tbtc.Deposit
		FundingTx *bitcoin.Transaction
	}{deposit, fundingTx}
}

func mustContainError(t *testing.T, err error, substring string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected an error containing [%s], got nil", substring)
	}
	if !strings.Contains(err.Error(), substring) {
		t.Fatalf("expected error to contain [%s], got: [%v]", substring, err)
	}
}

func TestValidateReservationAnchorProposal(t *testing.T) {
	depositor := common.HexToAddress("0x1111111111111111111111111111111111111a")
	vault := common.HexToAddress("0x00000000000000000000000000000000009999")
	walletPubKeyHash := [20]byte{0xaa, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11, 0x12, 0x13}
	refundPubKeyHash := [20]byte{0xbb, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11, 0x12, 0x13}

	t.Run("valid proposal is accepted", func(t *testing.T) {
		h := newValidatorHarness(t)
		now := uint32(time.Now().Unix())

		deposit, fundingTx, depositKey := buildFundingDeposit(
			t, depositor, walletPubKeyHash, refundPubKeyHash,
			now+uint32((60*24*time.Hour).Seconds()), 1_000_000,
		)

		h.setReservationParameters(stubParameters{ReservationVault: vault})
		h.setWallet(walletPubKeyHash, stubWallet{State: walletStateLive})
		h.setDeposit(depositKey, stubDepositReq{
			Depositor:  depositor,
			Amount:     1_000_000,
			RevealedAt: now - uint32((8 * time.Hour).Seconds()),
			Vault:      vault,
		})
		h.setReservedDeposit(depositKey, true)
		h.setReservationAction(depositKey, 1, stubAction{
			TargetWalletPubKeyHash: walletPubKeyHash,
			RequestedAt:            now - 3600,
			TimeoutAt:              now + uint32((24 * time.Hour).Seconds()),
			TxMaxFee:               20_000,
			ActionType:             actionTypeAcceptance,
			State:                  actionStatePending,
			MinAmount:              100_000,
		})
		h.commit()

		proposal := &tbtc.ReservationAnchorProposal{
			DepositFundingTxHash:      fundingTx.Hash(),
			DepositFundingOutputIndex: 0,
			RequestNonce:              1,
			AnchorTxFee:               big.NewInt(10_000),
		}

		err := h.tc.ValidateReservationAnchorProposal(
			walletPubKeyHash, proposal, depositExtraInfoOf(deposit, fundingTx),
		)
		if err != nil {
			t.Fatalf("unexpected validation error: [%v]", err)
		}
	})

	t.Run("no pending acceptance action at the given nonce", func(t *testing.T) {
		h := newValidatorHarness(t)
		now := uint32(time.Now().Unix())

		// No acceptance action has been authorized for this nonce:
		// reservationActions must return a zeroed struct
		// (actionType == None), and the real validator must reject the
		// proposal as not a pending acceptance action.
		deposit, fundingTx, _ := buildFundingDeposit(
			t, depositor, walletPubKeyHash, refundPubKeyHash,
			now+uint32((60*24*time.Hour).Seconds()), 1_000_000,
		)

		h.setReservationParameters(stubParameters{ReservationVault: vault})
		h.setWallet(walletPubKeyHash, stubWallet{State: walletStateLive})
		// Deliberately no setReservationAction call: reservationActions
		// returns a zeroed struct (actionType == None) for this nonce.
		h.commit()

		proposal := &tbtc.ReservationAnchorProposal{
			DepositFundingTxHash:      fundingTx.Hash(),
			DepositFundingOutputIndex: 0,
			RequestNonce:              1,
			AnchorTxFee:               big.NewInt(10_000),
		}

		err := h.tc.ValidateReservationAnchorProposal(
			walletPubKeyHash, proposal, depositExtraInfoOf(deposit, fundingTx),
		)
		mustContainError(t, err, "Not a pending acceptance action")
	})

	t.Run("amount below the action's snapshotted minimum", func(t *testing.T) {
		h := newValidatorHarness(t)
		now := uint32(time.Now().Unix())

		deposit, fundingTx, depositKey := buildFundingDeposit(
			t, depositor, walletPubKeyHash, refundPubKeyHash,
			now+uint32((60*24*time.Hour).Seconds()), 3_000,
		)

		// The live reservationMinAmount is far below the action's
		// snapshotted minAmount; validation must use the snapshot, not
		// the live parameter, to catch a later governance decrease from
		// reverting an already-broadcast anchor.
		h.setReservationParameters(stubParameters{
			ReservationVault:     vault,
			ReservationMinAmount: 1,
		})
		h.setWallet(walletPubKeyHash, stubWallet{State: walletStateLive})
		h.setDeposit(depositKey, stubDepositReq{
			Depositor:  depositor,
			Amount:     3_000,
			RevealedAt: now - uint32((8 * time.Hour).Seconds()),
			Vault:      vault,
		})
		h.setReservedDeposit(depositKey, true)
		h.setReservationAction(depositKey, 1, stubAction{
			TargetWalletPubKeyHash: walletPubKeyHash,
			RequestedAt:            now - 3600,
			TimeoutAt:              now + uint32((24 * time.Hour).Seconds()),
			TxMaxFee:               2_000,
			ActionType:             actionTypeAcceptance,
			State:                  actionStatePending,
			MinAmount:              5_000, // 3_000 amount < 5_000 minAmount + 1_000 fee
		})
		h.commit()

		proposal := &tbtc.ReservationAnchorProposal{
			DepositFundingTxHash:      fundingTx.Hash(),
			DepositFundingOutputIndex: 0,
			RequestNonce:              1,
			AnchorTxFee:               big.NewInt(1_000),
		}

		err := h.tc.ValidateReservationAnchorProposal(
			walletPubKeyHash, proposal, depositExtraInfoOf(deposit, fundingTx),
		)
		mustContainError(t, err, "Anchor amount below the reservation minimum")
	})

	// anchorSeed holds the inputs the cases below vary; the defaults set
	// by validateAnchor describe a proposal the validator accepts.
	type anchorSeed struct {
		walletState  uint8
		actionTarget [20]byte
		timeoutAt    uint32
		txMaxFee     uint64
		anchorTxFee  int64
	}

	// validateAnchor seeds a valid acceptance, lets adjust change one
	// input, and runs the real validator. now is the harness block
	// timestamp the validator compares against, not a fresh time.Now(),
	// so boundary values are exact.
	validateAnchor := func(
		t *testing.T,
		adjust func(now uint32, s *anchorSeed),
	) error {
		h := newValidatorHarness(t)
		now := uint32(h.backend.timestamp)

		s := anchorSeed{
			walletState:  walletStateLive,
			actionTarget: walletPubKeyHash,
			timeoutAt:    now + uint32((24 * time.Hour).Seconds()),
			txMaxFee:     20_000,
			anchorTxFee:  10_000,
		}
		adjust(now, &s)

		deposit, fundingTx, depositKey := buildFundingDeposit(
			t, depositor, walletPubKeyHash, refundPubKeyHash,
			now+uint32((60*24*time.Hour).Seconds()), 1_000_000,
		)

		h.setReservationParameters(stubParameters{ReservationVault: vault})
		h.setWallet(walletPubKeyHash, stubWallet{State: s.walletState})
		h.setDeposit(depositKey, stubDepositReq{
			Depositor:  depositor,
			Amount:     1_000_000,
			RevealedAt: now - uint32((8 * time.Hour).Seconds()),
			Vault:      vault,
		})
		h.setReservedDeposit(depositKey, true)
		h.setReservationAction(depositKey, 1, stubAction{
			TargetWalletPubKeyHash: s.actionTarget,
			RequestedAt:            now - 3600,
			TimeoutAt:              s.timeoutAt,
			TxMaxFee:               s.txMaxFee,
			ActionType:             actionTypeAcceptance,
			State:                  actionStatePending,
			MinAmount:              100_000,
		})
		h.commit()

		proposal := &tbtc.ReservationAnchorProposal{
			DepositFundingTxHash:      fundingTx.Hash(),
			DepositFundingOutputIndex: 0,
			RequestNonce:              1,
			AnchorTxFee:               big.NewInt(s.anchorTxFee),
		}

		return h.tc.ValidateReservationAnchorProposal(
			walletPubKeyHash, proposal, depositExtraInfoOf(deposit, fundingTx),
		)
	}

	t.Run("action timing out exactly at the safety margin is rejected", func(t *testing.T) {
		// The validator's check is strict (block.timestamp < timeoutAt -
		// margin), so a proposal exactly at the margin is already too
		// late; keep-core must not treat it as signable.
		err := validateAnchor(t, func(now uint32, s *anchorSeed) {
			s.timeoutAt = now + requestTimeoutSafetyMarginSeconds
		})
		mustContainError(t, err, "Acceptance action has timed out")
	})

	t.Run("action timing out one second past the safety margin is accepted", func(t *testing.T) {
		err := validateAnchor(t, func(now uint32, s *anchorSeed) {
			s.timeoutAt = now + requestTimeoutSafetyMarginSeconds + 1
		})
		if err != nil {
			t.Fatalf("unexpected validation error: [%v]", err)
		}
	})

	t.Run("wallet in MovingFunds state is accepted", func(t *testing.T) {
		// Acceptance must keep working while a wallet moves funds, or
		// reserved deposits on a draining wallet could never be anchored.
		err := validateAnchor(t, func(now uint32, s *anchorSeed) {
			s.walletState = walletStateMovingFunds
		})
		if err != nil {
			t.Fatalf("unexpected validation error: [%v]", err)
		}
	})

	t.Run("fee above the action's snapshotted max fee", func(t *testing.T) {
		err := validateAnchor(t, func(now uint32, s *anchorSeed) {
			s.anchorTxFee = int64(s.txMaxFee) + 1
		})
		mustContainError(t, err, "Proposed transaction fee is too high")
	})

	t.Run("wallet does not match the authorized action", func(t *testing.T) {
		// The proposal's wallet is Live and controls the deposit, but the
		// action authorizes another wallet: only the action's target may
		// anchor the reservation.
		err := validateAnchor(t, func(now uint32, s *anchorSeed) {
			s.actionTarget = [20]byte{0xee, 0x01}
		})
		mustContainError(t, err, "Wallet does not match the authorized action")
	})
}

func TestValidateReservationReanchorProposal(t *testing.T) {
	sourceWalletPubKeyHash := [20]byte{0xcc, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11, 0x12, 0x13}
	targetWalletPubKeyHash := [20]byte{0xdd, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11, 0x12, 0x13}
	reservationKey := big.NewInt(4242)

	t.Run("valid proposal is accepted", func(t *testing.T) {
		h := newValidatorHarness(t)
		now := uint32(time.Now().Unix())

		h.setReservation(reservationKey, stubResReq{
			WalletPubKeyHash: sourceWalletPubKeyHash,
		})
		h.setReservationAction(reservationKey, 1, stubAction{
			TargetWalletPubKeyHash: targetWalletPubKeyHash,
			RequestedAt:            now - 3600,
			TimeoutAt:              now + uint32((24 * time.Hour).Seconds()),
			TxMaxFee:               10_000,
			ActionType:             actionTypeReanchor,
			State:                  actionStatePending,
		})
		h.setWallet(targetWalletPubKeyHash, stubWallet{State: walletStateLive})
		h.setReservationParameters(stubParameters{MaxReservationsPerWallet: 10})
		h.setWalletReservationsCount(targetWalletPubKeyHash, 1)
		h.commit()

		proposal := &tbtc.ReservationReanchorProposal{
			ReservationKey:            reservationKey,
			RequestNonce:              1,
			TargetWalletPublicKeyHash: targetWalletPubKeyHash,
			ReanchorTxFee:             big.NewInt(5_000),
		}

		err := h.tc.ValidateReservationReanchorProposal(
			sourceWalletPubKeyHash, proposal,
		)
		if err != nil {
			t.Fatalf("unexpected validation error: [%v]", err)
		}
	})

	t.Run("no pending re-anchor action at the given nonce", func(t *testing.T) {
		h := newValidatorHarness(t)

		// No re-anchor action has been authorized for this nonce:
		// reservationActions returns a zeroed struct
		// (actionType == None), and the real validator rejects the
		// proposal as not a pending re-anchor action.
		proposal := &tbtc.ReservationReanchorProposal{
			ReservationKey:            reservationKey,
			RequestNonce:              1,
			TargetWalletPublicKeyHash: targetWalletPubKeyHash,
			ReanchorTxFee:             big.NewInt(5_000),
		}

		err := h.tc.ValidateReservationReanchorProposal(
			sourceWalletPubKeyHash, proposal,
		)
		mustContainError(t, err, "Not a pending re-anchor action")
	})

	t.Run("target wallet does not match the authorized action", func(t *testing.T) {
		h := newValidatorHarness(t)
		now := uint32(time.Now().Unix())

		otherTargetWalletPubKeyHash := [20]byte{0xee, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11, 0x12, 0x13}

		h.setReservation(reservationKey, stubResReq{
			WalletPubKeyHash: sourceWalletPubKeyHash,
		})
		// Action authorizes otherTargetWalletPubKeyHash, but the proposal
		// below names targetWalletPubKeyHash: a dropped/mismapped
		// RequestNonce or target field would let this slip through.
		h.setReservationAction(reservationKey, 1, stubAction{
			TargetWalletPubKeyHash: otherTargetWalletPubKeyHash,
			RequestedAt:            now - 3600,
			TimeoutAt:              now + uint32((24 * time.Hour).Seconds()),
			TxMaxFee:               10_000,
			ActionType:             actionTypeReanchor,
			State:                  actionStatePending,
		})
		h.commit()

		proposal := &tbtc.ReservationReanchorProposal{
			ReservationKey:            reservationKey,
			RequestNonce:              1,
			TargetWalletPublicKeyHash: targetWalletPubKeyHash,
			ReanchorTxFee:             big.NewInt(5_000),
		}

		err := h.tc.ValidateReservationReanchorProposal(
			sourceWalletPubKeyHash, proposal,
		)
		mustContainError(t, err, "Target wallet does not match the authorized action")
	})

	// reanchorSeed holds the inputs the cases below vary; the defaults
	// set by validateReanchor describe a proposal the validator accepts.
	type reanchorSeed struct {
		timeoutAt                uint32
		maxReservationsPerWallet uint32
		targetReservationsCount  uint32
	}

	// validateReanchor seeds a valid re-anchor, lets adjust change one
	// input, and runs the real validator. now is the harness block
	// timestamp, so boundary values are exact.
	validateReanchor := func(
		t *testing.T,
		adjust func(now uint32, s *reanchorSeed),
	) error {
		h := newValidatorHarness(t)
		now := uint32(h.backend.timestamp)

		s := reanchorSeed{
			timeoutAt:                now + uint32((24 * time.Hour).Seconds()),
			maxReservationsPerWallet: 10,
			targetReservationsCount:  1,
		}
		adjust(now, &s)

		h.setReservation(reservationKey, stubResReq{
			WalletPubKeyHash: sourceWalletPubKeyHash,
		})
		h.setReservationAction(reservationKey, 1, stubAction{
			TargetWalletPubKeyHash: targetWalletPubKeyHash,
			RequestedAt:            now - 3600,
			TimeoutAt:              s.timeoutAt,
			TxMaxFee:               10_000,
			ActionType:             actionTypeReanchor,
			State:                  actionStatePending,
		})
		h.setWallet(targetWalletPubKeyHash, stubWallet{State: walletStateLive})
		h.setReservationParameters(stubParameters{
			MaxReservationsPerWallet: s.maxReservationsPerWallet,
		})
		h.setWalletReservationsCount(
			targetWalletPubKeyHash, s.targetReservationsCount,
		)
		h.commit()

		return h.tc.ValidateReservationReanchorProposal(
			sourceWalletPubKeyHash,
			&tbtc.ReservationReanchorProposal{
				ReservationKey:            reservationKey,
				RequestNonce:              1,
				TargetWalletPublicKeyHash: targetWalletPubKeyHash,
				ReanchorTxFee:             big.NewInt(5_000),
			},
		)
	}

	t.Run("action timing out exactly at the safety margin is rejected", func(t *testing.T) {
		// Strict check, as for acceptance: exactly at the margin is too
		// late.
		err := validateReanchor(t, func(now uint32, s *reanchorSeed) {
			s.timeoutAt = now + requestTimeoutSafetyMarginSeconds
		})
		mustContainError(t, err, "Re-anchor action has timed out")
	})

	t.Run("action timing out one second past the safety margin is accepted", func(t *testing.T) {
		err := validateReanchor(t, func(now uint32, s *reanchorSeed) {
			s.timeoutAt = now + requestTimeoutSafetyMarginSeconds + 1
		})
		if err != nil {
			t.Fatalf("unexpected validation error: [%v]", err)
		}
	})

	t.Run("target wallet count at the cap is accepted", func(t *testing.T) {
		// requestReservationReanchor already counted this reservation
		// against the target wallet, so at signing time a count equal to
		// the cap is the normal state of a full wallet, not an overflow.
		err := validateReanchor(t, func(now uint32, s *reanchorSeed) {
			s.targetReservationsCount = s.maxReservationsPerWallet
		})
		if err != nil {
			t.Fatalf("unexpected validation error: [%v]", err)
		}
	})

	t.Run("target wallet count above the cap is rejected", func(t *testing.T) {
		err := validateReanchor(t, func(now uint32, s *reanchorSeed) {
			s.targetReservationsCount = s.maxReservationsPerWallet + 1
		})
		mustContainError(t, err, "Wallet reservations cap exceeded")
	})
}
