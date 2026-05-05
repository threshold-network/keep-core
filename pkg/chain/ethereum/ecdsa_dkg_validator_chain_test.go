package ethereum

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
)

// mockContractCaller implements bind.ContractCaller with configurable responses.
type mockContractCaller struct {
	code        []byte
	codeErr     error
	callResults map[string][]byte // method selector hex -> packed return
	callErr     map[string]error
}

func (m *mockContractCaller) CodeAt(
	_ context.Context,
	_ common.Address,
	_ *big.Int,
) ([]byte, error) {
	return m.code, m.codeErr
}

func (m *mockContractCaller) CallContract(
	_ context.Context,
	call ethereum.CallMsg,
	_ *big.Int,
) ([]byte, error) {
	if call.Data == nil || len(call.Data) < 4 {
		return nil, fmt.Errorf("calldata too short")
	}
	sel := hex.EncodeToString(call.Data[:4])
	if err, ok := m.callErr[sel]; ok && err != nil {
		return nil, err
	}
	if data, ok := m.callResults[sel]; ok {
		return data, nil
	}
	return nil, fmt.Errorf("no mock for selector %s", sel)
}

// packUint256 encodes a uint256 return value as a 32-byte ABI word.
func packUint256(v *big.Int) []byte {
	b := make([]byte, 32)
	vb := v.Bytes()
	copy(b[32-len(vb):], vb)
	return b
}

// selectorOf returns the 4-byte ABI selector hex for a method name.
func selectorOf(method string) string {
	packed, _ := ecdsaDkgValidatorConstantsABI.Pack(method)
	return hex.EncodeToString(packed[:4])
}

var nonZeroAddr = common.HexToAddress("0x0000000000000000000000000000000000000001")

func TestEcdsaWalletGroupParametersFromValidator_ZeroAddress(t *testing.T) {
	_, err := ecdsaWalletGroupParametersFromValidator(
		context.Background(),
		&mockContractCaller{code: []byte{0x60}},
		common.Address{},
	)
	if err == nil {
		t.Fatal("expected error for zero address, got nil")
	}
}

func TestEcdsaWalletGroupParametersFromValidator_NoCode(t *testing.T) {
	_, err := ecdsaWalletGroupParametersFromValidator(
		context.Background(),
		&mockContractCaller{code: []byte{}},
		nonZeroAddr,
	)
	if err == nil {
		t.Fatal("expected error for empty code, got nil")
	}
}

func TestEcdsaWalletGroupParametersFromValidator_QuorumLtHonestThreshold(t *testing.T) {
	// activeThreshold (groupQuorum) < groupThreshold (honestThreshold) must be rejected.
	mc := &mockContractCaller{
		code: []byte{0x60},
		callResults: map[string][]byte{
			selectorOf("groupSize"):       packUint256(big.NewInt(100)),
			selectorOf("activeThreshold"): packUint256(big.NewInt(33)), // quorum < threshold
			selectorOf("groupThreshold"):  packUint256(big.NewInt(51)),
		},
	}
	_, err := ecdsaWalletGroupParametersFromValidator(
		context.Background(),
		mc,
		nonZeroAddr,
	)
	if err == nil {
		t.Fatal("expected error when groupQuorum < honestThreshold, got nil")
	}
}

func TestEcdsaWalletGroupParametersFromValidator_HappyPath(t *testing.T) {
	mc := &mockContractCaller{
		code: []byte{0x60},
		callResults: map[string][]byte{
			selectorOf("groupSize"):       packUint256(big.NewInt(100)),
			selectorOf("activeThreshold"): packUint256(big.NewInt(90)),
			selectorOf("groupThreshold"):  packUint256(big.NewInt(51)),
		},
	}
	params, err := ecdsaWalletGroupParametersFromValidator(
		context.Background(),
		mc,
		nonZeroAddr,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if params.GroupSize != 100 {
		t.Errorf("GroupSize: expected 100, got %d", params.GroupSize)
	}
	if params.GroupQuorum != 90 {
		t.Errorf("GroupQuorum: expected 90, got %d", params.GroupQuorum)
	}
	if params.HonestThreshold != 51 {
		t.Errorf("HonestThreshold: expected 51, got %d", params.HonestThreshold)
	}
}
