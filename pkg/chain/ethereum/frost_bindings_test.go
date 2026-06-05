package ethereum

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	frostabi "github.com/keep-network/keep-core/pkg/chain/ethereum/frost/gen/abi"
	frostvalidatorabi "github.com/keep-network/keep-core/pkg/chain/ethereum/frost/gen/validatorabi"
)

const frostDkgResultTupleSignature = "(uint256,bytes32,uint8[],bytes,uint256[],uint32[],bytes32)"

func TestFrostGeneratedBindingsUseDeployedDkgResultTupleOrder(t *testing.T) {
	expectedFields := []string{
		"SubmitterMemberIndex",
		"XOnlyOutputKey",
		"MisbehavedMembersIndices",
		"Signatures",
		"SigningMembersIndices",
		"Members",
		"MembersHash",
	}

	assertStructFieldOrder(t, reflect.TypeOf(frostabi.Struct0{}), expectedFields)
	assertStructFieldOrder(t, reflect.TypeOf(frostvalidatorabi.Struct0{}), expectedFields)

	walletRegistryABI, err := frostabi.FrostWalletRegistryMetaData.GetAbi()
	if err != nil {
		t.Fatal(err)
	}

	for _, method := range []string{
		"submitDkgResult",
		"approveDkgResult",
		"challengeDkgResult",
		"isDkgResultValid",
	} {
		expectedSelector := functionSelector(
			method + "(" + frostDkgResultTupleSignature + ")",
		)
		actualSelector := walletRegistryABI.Methods[method].ID
		if !bytes.Equal(actualSelector, expectedSelector) {
			t.Fatalf(
				"unexpected %s selector: got 0x%x, want 0x%x",
				method,
				actualSelector,
				expectedSelector,
			)
		}
	}

	expectedEventID := crypto.Keccak256Hash([]byte(
		"DkgResultSubmitted(bytes32,uint256," +
			frostDkgResultTupleSignature +
			")",
	))
	actualEventID := walletRegistryABI.Events["DkgResultSubmitted"].ID
	if actualEventID != expectedEventID {
		t.Fatalf(
			"unexpected DkgResultSubmitted topic: got 0x%x, want 0x%x",
			actualEventID,
			expectedEventID,
		)
	}

	validatorABI, err := frostvalidatorabi.FrostDkgValidatorMetaData.GetAbi()
	if err != nil {
		t.Fatal(err)
	}

	expectedSelector := functionSelector(
		"resultDigest(" +
			frostDkgResultTupleSignature +
			",uint256,address,address)",
	)
	actualSelector := validatorABI.Methods["resultDigest"].ID
	if !bytes.Equal(actualSelector, expectedSelector) {
		t.Fatalf(
			"unexpected resultDigest selector: got 0x%x, want 0x%x",
			actualSelector,
			expectedSelector,
		)
	}
}

func assertStructFieldOrder(
	t *testing.T,
	structType reflect.Type,
	expectedFields []string,
) {
	t.Helper()

	if structType.NumField() != len(expectedFields) {
		t.Fatalf(
			"unexpected field count for %s: got %d, want %d",
			structType.Name(),
			structType.NumField(),
			len(expectedFields),
		)
	}

	for i, expectedField := range expectedFields {
		if actualField := structType.Field(i).Name; actualField != expectedField {
			t.Fatalf(
				"unexpected field %d for %s: got %s, want %s",
				i,
				structType.Name(),
				actualField,
				expectedField,
			)
		}
	}
}

func functionSelector(signature string) []byte {
	hash := crypto.Keccak256([]byte(signature))
	return hash[:4]
}
