package ethtest

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	ethabi "github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"

	beaconabi "github.com/keep-network/keep-core/pkg/chain/ethereum/beacon/gen/abi"
	ecdsaabi "github.com/keep-network/keep-core/pkg/chain/ethereum/ecdsa/gen/abi"
	tbtcabi "github.com/keep-network/keep-core/pkg/chain/ethereum/tbtc/gen/abi"
	thresholdabi "github.com/keep-network/keep-core/pkg/chain/ethereum/threshold/gen/abi"
)

// operatorKeyIndex is the key index the fixture hands to the chain handles as
// the node's own operator account.
const operatorKeyIndex = 1

// Call is one read the fixture served, decoded with the contract ABI the
// target address is registered under.
type Call struct {
	Contract string
	To       common.Address
	Method   string
	// Selector is the four leading bytes of the call data, as the caller
	// encoded them. It is kept apart from Method so a trace can assert what
	// went over the wire rather than what the fixture resolved it to.
	Selector []byte
	Args     []interface{}
}

// contract is one ABI-backed target address the fixture answers reads for.
type contract struct {
	name string
	abi  ethabi.ABI
}

// Backend is a deterministic Ethereum JSON-RPC endpoint. Create one with New.
type Backend struct {
	mutex      sync.Mutex
	state      State
	contracts  map[common.Address]*contract
	calls      []Call
	unexpected []string
	faults     map[string]string

	server  *httptest.Server
	keyFile string
}

// New starts a fixture serving the given state and stops it when the test
// finishes.
func New(t *testing.T, state State) *Backend {
	t.Helper()

	state.normalize()

	backend := &Backend{
		state:     state,
		contracts: map[common.Address]*contract{},
		faults:    map[string]string{},
		keyFile:   writeKeyFile(t, t.TempDir()),
	}

	backend.register(t, BridgeAddress, BridgeContract, tbtcabi.BridgeABI)
	backend.register(
		t,
		WalletRegistryAddress,
		WalletRegistryContract,
		ecdsaabi.WalletRegistryABI,
	)
	backend.register(
		t,
		RandomBeaconAddress,
		RandomBeaconContract,
		beaconabi.RandomBeaconABI,
	)
	backend.register(
		t,
		TokenStakingAddress,
		TokenStakingContract,
		thresholdabi.TokenStakingABI,
	)
	// The allowlist is registered with the token staking ABI on purpose: a
	// rolesOf read misrouted to it has to decode far enough to be reported as
	// a routing fault rather than as an unknown address.
	backend.register(
		t,
		AllowlistAddress,
		AllowlistContract,
		thresholdabi.TokenStakingABI,
	)

	backend.server = httptest.NewServer(http.HandlerFunc(backend.serve))
	t.Cleanup(backend.server.Close)

	return backend
}

func (b *Backend) register(
	t *testing.T,
	address common.Address,
	name string,
	rawABI string,
) {
	t.Helper()

	parsed, err := ethabi.JSON(strings.NewReader(rawABI))
	if err != nil {
		t.Fatalf("failed to parse the %s ABI: %v", name, err)
	}

	b.contracts[address] = &contract{name: name, abi: parsed}
}

// URL is the endpoint the fixture listens on.
func (b *Backend) URL() string {
	return b.server.URL
}

// SetEligibleStake seeds the eligible stake the wallet registry reports for
// the given staking provider.
func (b *Backend) SetEligibleStake(
	stakingProvider common.Address,
	amount *big.Int,
) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	b.state.EligibleStakes[stakingProvider] = amount
}

// Fail makes every subsequent read of the given contract method return a chain
// error carrying the given message, until Recover clears it.
func (b *Backend) Fail(contractName, method, message string) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	b.faults[faultKey(contractName, method)] = message
}

// Recover clears the fault set on the given contract method.
func (b *Backend) Recover(contractName, method string) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	delete(b.faults, faultKey(contractName, method))
}

func faultKey(contractName, method string) string {
	return contractName + "." + method
}

// Calls returns every read the fixture served so far, in order.
func (b *Backend) Calls() []Call {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	calls := make([]Call, len(b.calls))
	copy(calls, b.calls)

	return calls
}

// CallsTo returns the reads served for one contract method, in order.
func (b *Backend) CallsTo(contractName, method string) []Call {
	matched := make([]Call, 0)

	for _, call := range b.Calls() {
		if call.Contract == contractName && call.Method == method {
			matched = append(matched, call)
		}
	}

	return matched
}

// CallCount returns how many times one contract method was read.
func (b *Backend) CallCount(contractName, method string) int {
	return len(b.CallsTo(contractName, method))
}

// ResetCalls forgets the reads served so far, so that a following assertion
// counts only the reads a specific action performs.
func (b *Backend) ResetCalls() {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	b.calls = nil
}

// AssertNoUnexpectedCalls fails the test if the fixture was asked for anything
// it could not attribute to a registered contract method - an unknown target
// address, an unknown selector, an unsupported JSON-RPC method, or a read
// routed to the allowlist instead of token staking.
func (b *Backend) AssertNoUnexpectedCalls(t *testing.T) {
	t.Helper()

	b.mutex.Lock()
	defer b.mutex.Unlock()

	for _, unexpected := range b.unexpected {
		t.Errorf("unexpected chain request: %s", unexpected)
	}
}

// UnexpectedCalls returns the requests the fixture refused, in order.
func (b *Backend) UnexpectedCalls() []string {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	unexpected := make([]string, len(b.unexpected))
	copy(unexpected, b.unexpected)

	return unexpected
}

type rpcRequest struct {
	ID     json.RawMessage   `json:"id"`
	Method string            `json:"method"`
	Params []json.RawMessage `json:"params"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	Version string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

func (b *Backend) serve(writer http.ResponseWriter, request *http.Request) {
	var decoded rpcRequest
	if err := json.NewDecoder(request.Body).Decode(&decoded); err != nil {
		b.fail(writer, decoded.ID, fmt.Errorf("malformed request: %v", err))
		return
	}

	result, err := b.answer(decoded)
	if err != nil {
		b.fail(writer, decoded.ID, err)
		return
	}

	encoded, err := json.Marshal(result)
	if err != nil {
		b.fail(
			writer,
			decoded.ID,
			fmt.Errorf("unencodable result: %v", err),
		)
		return
	}

	b.reply(writer, rpcResponse{
		Version: "2.0",
		ID:      decoded.ID,
		Result:  encoded,
	})
}

// fail answers with a JSON-RPC error. A deliberately injected chain fault is
// only reported to the caller; anything else is also recorded as an unexpected
// request, so that a test sees it even when the code under test swallows the
// error it caused.
func (b *Backend) fail(
	writer http.ResponseWriter,
	id json.RawMessage,
	cause error,
) {
	fault := &chainFault{}
	if !errors.As(cause, &fault) {
		b.mutex.Lock()
		b.unexpected = append(b.unexpected, cause.Error())
		b.mutex.Unlock()
	}

	b.reply(writer, rpcResponse{
		Version: "2.0",
		ID:      id,
		Error:   &rpcError{Code: -32000, Message: cause.Error()},
	})
}

func (b *Backend) reply(writer http.ResponseWriter, response rpcResponse) {
	writer.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(writer).Encode(response); err != nil {
		b.mutex.Lock()
		b.unexpected = append(
			b.unexpected,
			fmt.Sprintf("failed to write a response: %v", err),
		)
		b.mutex.Unlock()
	}
}

// chainFault is the error a read seeded with Fail answers with. It stands for
// the chain being unreachable, not for the fixture being asked something it
// did not expect.
type chainFault struct {
	message string
}

func (f *chainFault) Error() string {
	return f.message
}

func (b *Backend) answer(request rpcRequest) (interface{}, error) {
	switch request.Method {
	case "eth_chainId":
		b.mutex.Lock()
		defer b.mutex.Unlock()

		return (*hexutil.Big)(b.state.ChainID), nil
	case "eth_blockNumber":
		b.mutex.Lock()
		defer b.mutex.Unlock()

		return hexutil.Uint64(b.state.BlockNumber), nil
	case "eth_getBlockByNumber":
		return b.block()
	case "eth_getCode":
		// Every registered address is a contract as far as the bindings are
		// concerned; the value only has to be non-empty.
		return hexutil.Bytes{0x60, 0x00}, nil
	case "eth_call":
		return b.call(request.Params)
	default:
		return nil, fmt.Errorf(
			"unsupported JSON-RPC method [%s]",
			request.Method,
		)
	}
}

// block answers eth_getBlockByNumber. The response carries the block's own
// hash, which a node reports and cannot be recomputed from the fields below:
// the pinned header type predates the fork fields a current mainnet header
// holds, so anything that re-derives the hash locally derives a different one.
// The extra fields are served for the same reason - a reader has to tolerate
// fields it does not model.
func (b *Backend) block() (json.RawMessage, error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if b.state.BlockMissing {
		return json.RawMessage("null"), nil
	}

	header := &types.Header{
		Number:     new(big.Int).SetUint64(b.state.BlockNumber),
		Difficulty: big.NewInt(0),
		Extra:      []byte{},
	}

	encoded, err := json.Marshal(header)
	if err != nil {
		return nil, fmt.Errorf("unencodable header: %v", err)
	}

	fields := map[string]json.RawMessage{}
	if err := json.Unmarshal(encoded, &fields); err != nil {
		return nil, fmt.Errorf("undecodable header: %v", err)
	}

	fields["hash"], err = json.Marshal(b.state.BlockHash)
	if err != nil {
		return nil, fmt.Errorf("unencodable block hash: %v", err)
	}

	// Overrides land last so a test can also replace what the fixture would
	// otherwise report for itself.
	for name, value := range b.state.BlockFields {
		fields[name] = value
	}

	return json.Marshal(fields)
}

type callArguments struct {
	To    *common.Address `json:"to"`
	Input hexutil.Bytes   `json:"input"`
}

func (b *Backend) call(params []json.RawMessage) (interface{}, error) {
	if len(params) == 0 {
		return nil, errors.New("eth_call was sent without arguments")
	}

	var arguments callArguments
	if err := json.Unmarshal(params[0], &arguments); err != nil {
		return nil, fmt.Errorf("malformed eth_call arguments: %v", err)
	}

	if arguments.To == nil {
		return nil, errors.New("eth_call was sent without a target address")
	}

	b.mutex.Lock()
	target, known := b.contracts[*arguments.To]
	b.mutex.Unlock()

	if !known {
		return nil, fmt.Errorf(
			"eth_call to unknown address [%v]",
			arguments.To.Hex(),
		)
	}

	if len(arguments.Input) < 4 {
		return nil, fmt.Errorf(
			"eth_call to [%s] carries no method selector",
			target.name,
		)
	}

	method, err := target.abi.MethodById(arguments.Input[:4])
	if err != nil {
		return nil, fmt.Errorf(
			"eth_call to [%s] with selector [%x] matches no method: %v",
			target.name,
			arguments.Input[:4],
			err,
		)
	}

	args, err := method.Inputs.Unpack(arguments.Input[4:])
	if err != nil {
		return nil, fmt.Errorf(
			"eth_call to [%s.%s] carries undecodable arguments: %v",
			target.name,
			method.Name,
			err,
		)
	}

	b.record(Call{
		Contract: target.name,
		To:       *arguments.To,
		Method:   method.Name,
		Selector: append([]byte(nil), arguments.Input[:4]...),
		Args:     args,
	})

	outputs, err := b.read(target.name, method.Name, args)
	if err != nil {
		return nil, err
	}

	packed, err := method.Outputs.Pack(outputs...)
	if err != nil {
		return nil, fmt.Errorf(
			"eth_call to [%s.%s] returned unencodable values: %v",
			target.name,
			method.Name,
			err,
		)
	}

	return hexutil.Bytes(packed), nil
}

func (b *Backend) record(call Call) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	b.calls = append(b.calls, call)
}
