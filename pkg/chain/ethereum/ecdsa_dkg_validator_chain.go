package ethereum

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/keep-network/keep-common/pkg/chain/ethereum/ethutil"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// Minimal ABI for EcdsaDkgValidator public constants used by keep-client.
var ecdsaDkgValidatorConstantsABI = mustParseABI(`[
  {"inputs":[],"name":"activeThreshold","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},
  {"inputs":[],"name":"groupSize","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},
  {"inputs":[],"name":"groupThreshold","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"}
]`)

func mustParseABI(raw string) abi.ABI {
	a, err := abi.JSON(strings.NewReader(raw))
	if err != nil {
		panic(fmt.Sprintf("invalid bundled EcdsaDkgValidator ABI: %v", err))
	}
	return a
}

func readEcdsaDkgValidatorUint256(
	ctx context.Context,
	client ethutil.EthereumClient,
	addr common.Address,
	method string,
) (*big.Int, error) {
	calldata, err := ecdsaDkgValidatorConstantsABI.Pack(method)
	if err != nil {
		return nil, fmt.Errorf("pack %s calldata: %w", method, err)
	}
	raw, err := client.CallContract(
		ctx,
		ethereum.CallMsg{To: &addr, Data: calldata},
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("contract call %s: %w", method, err)
	}
	values, err := ecdsaDkgValidatorConstantsABI.Unpack(method, raw)
	if err != nil {
		return nil, fmt.Errorf("unpack %s: %w", method, err)
	}
	if len(values) != 1 {
		return nil, fmt.Errorf("unexpected %s return length [%d]", method, len(values))
	}
	v, ok := values[0].(*big.Int)
	if !ok || v == nil {
		return nil, fmt.Errorf("unexpected %s return type [%T]", method, values[0])
	}
	return v, nil
}

const maxReasonableTBTCGroupSize = 8192

func bigIntPositiveIntLimited(name string, v *big.Int, limit int) (int, error) {
	if v == nil || v.Sign() <= 0 {
		return 0, fmt.Errorf("%s must be positive", name)
	}
	if v.BitLen() > 63 {
		return 0, fmt.Errorf("%s is too large", name)
	}
	i := int(v.Int64())
	if i > limit {
		return 0, fmt.Errorf("%s [%d] exceeds limit [%d]", name, i, limit)
	}
	return i, nil
}

// ecdsaWalletGroupParametersFromValidator queries EcdsaDkgValidator and maps fields
// to keep-client TBTC semantics:
//   - GroupSize      <- groupSize()
//   - GroupQuorum    <- activeThreshold() (minimum “ready” members in DKG announce)
//   - HonestThreshold <- groupThreshold() (threshold signing parameter)
func ecdsaWalletGroupParametersFromValidator(
	ctx context.Context,
	client ethutil.EthereumClient,
	addr common.Address,
) (*tbtc.GroupParameters, error) {
	if addr == (common.Address{}) {
		return nil, fmt.Errorf("EcdsaDkgValidator address is zero")
	}
	gsBig, err := readEcdsaDkgValidatorUint256(ctx, client, addr, "groupSize")
	if err != nil {
		return nil, err
	}
	atBig, err := readEcdsaDkgValidatorUint256(ctx, client, addr, "activeThreshold")
	if err != nil {
		return nil, err
	}
	gtBig, err := readEcdsaDkgValidatorUint256(ctx, client, addr, "groupThreshold")
	if err != nil {
		return nil, err
	}

	groupSize, err := bigIntPositiveIntLimited("groupSize", gsBig, maxReasonableTBTCGroupSize)
	if err != nil {
		return nil, err
	}
	groupQuorum, err := bigIntPositiveIntLimited("activeThreshold", atBig, maxReasonableTBTCGroupSize)
	if err != nil {
		return nil, err
	}
	honestThreshold, err := bigIntPositiveIntLimited(
		"groupThreshold",
		gtBig,
		maxReasonableTBTCGroupSize,
	)
	if err != nil {
		return nil, err
	}

	if groupQuorum > groupSize {
		return nil, fmt.Errorf(
			"activeThreshold/groupQuorum [%d] exceeds groupSize [%d]",
			groupQuorum,
			groupSize,
		)
	}
	if honestThreshold > groupSize {
		return nil, fmt.Errorf(
			"groupThreshold/honestThreshold [%d] exceeds groupSize [%d]",
			honestThreshold,
			groupSize,
		)
	}
	if groupQuorum < honestThreshold {
		return nil, fmt.Errorf(
			"activeThreshold/groupQuorum [%d] is less than groupThreshold/honestThreshold [%d]",
			groupQuorum,
			honestThreshold,
		)
	}

	return &tbtc.GroupParameters{
		GroupSize:       groupSize,
		GroupQuorum:     groupQuorum,
		HonestThreshold: honestThreshold,
	}, nil
}
