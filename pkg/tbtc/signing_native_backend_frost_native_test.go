//go:build frost_native

package tbtc

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/ipfs/go-log/v2"
	"github.com/keep-network/keep-core/pkg/frost"
	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

type countingNativeExecutionFFISigningPrimitive struct {
	signCalls atomic.Int64
}

type deterministicNativeExecutionFFISigningPrimitiveForTBTC struct {
	signCalls atomic.Int64
}

type attemptTrackingNativeExecutionFFISigningPrimitiveForTBTC struct {
	signCalls atomic.Int64
	mutex     sync.Mutex
	records   []attemptTrackingRecordForTBTC
}

type attemptTrackingRecordForTBTC struct {
	attemptNumber       uint
	includedMemberIndex []group.MemberIndex
}

var deterministicNativeFROSTSignatureForTBTC = [frost.SignatureSize]byte{
	0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
	0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10,
	0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18,
	0x19, 0x1A, 0x1B, 0x1C, 0x1D, 0x1E, 0x1F, 0x20,
	0x21, 0x22, 0x23, 0x24, 0x25, 0x26, 0x27, 0x28,
	0x29, 0x2A, 0x2B, 0x2C, 0x2D, 0x2E, 0x2F, 0x30,
	0x31, 0x32, 0x33, 0x34, 0x35, 0x36, 0x37, 0x38,
	0x39, 0x3A, 0x3B, 0x3C, 0x3D, 0x3E, 0x3F, 0x40,
}

func (cnefsp *countingNativeExecutionFFISigningPrimitive) Sign(
	ctx context.Context,
	logger log.StandardLogger,
	request *frostsigning.NativeExecutionFFISigningRequest,
) (*frost.Signature, error) {
	cnefsp.signCalls.Add(1)
	return &frost.Signature{}, nil
}

func (cnefsp *countingNativeExecutionFFISigningPrimitive) RegisterUnmarshallers(
	channel net.BroadcastChannel,
) {
}

func (dnefspf *deterministicNativeExecutionFFISigningPrimitiveForTBTC) Sign(
	ctx context.Context,
	logger log.StandardLogger,
	request *frostsigning.NativeExecutionFFISigningRequest,
) (*frost.Signature, error) {
	dnefspf.signCalls.Add(1)

	if request == nil {
		return nil, fmt.Errorf("request is nil")
	}

	nativeSignerMaterial := request.SignerMaterial
	if nativeSignerMaterial == nil {
		return nil, fmt.Errorf("native signer material is nil")
	}

	if nativeSignerMaterial.Format != frostsigning.NativeSignerMaterialFormatFrostUniFFIV2 {
		return nil, fmt.Errorf(
			"unexpected signer material format: [%s]",
			nativeSignerMaterial.Format,
		)
	}

	signature := &frost.Signature{}
	if err := signature.Unmarshal(deterministicNativeFROSTSignatureForTBTC[:]); err != nil {
		return nil, err
	}

	return signature, nil
}

func (dnefspf *deterministicNativeExecutionFFISigningPrimitiveForTBTC) RegisterUnmarshallers(
	channel net.BroadcastChannel,
) {
}

func (atnefspf *attemptTrackingNativeExecutionFFISigningPrimitiveForTBTC) Sign(
	ctx context.Context,
	logger log.StandardLogger,
	request *frostsigning.NativeExecutionFFISigningRequest,
) (*frost.Signature, error) {
	atnefspf.signCalls.Add(1)

	if request == nil {
		return nil, fmt.Errorf("request is nil")
	}

	if request.Attempt == nil {
		return nil, fmt.Errorf("request attempt is nil")
	}

	atnefspf.mutex.Lock()
	atnefspf.records = append(
		atnefspf.records,
		attemptTrackingRecordForTBTC{
			attemptNumber: request.Attempt.Number,
			includedMemberIndex: append(
				[]group.MemberIndex{},
				request.Attempt.IncludedMembersIndexes...,
			),
		},
	)
	atnefspf.mutex.Unlock()

	// Force retry-loop progression so the next attempt is exercised.
	if request.Attempt.Number == 1 {
		return nil, fmt.Errorf("forced attempt failure")
	}

	signature := &frost.Signature{}
	if err := signature.Unmarshal(deterministicNativeFROSTSignatureForTBTC[:]); err != nil {
		return nil, err
	}

	return signature, nil
}

func (atnefspf *attemptTrackingNativeExecutionFFISigningPrimitiveForTBTC) RegisterUnmarshallers(
	channel net.BroadcastChannel,
) {
}

func (atnefspf *attemptTrackingNativeExecutionFFISigningPrimitiveForTBTC) uniqueCohortsByAttempt() map[uint][][]group.MemberIndex {
	atnefspf.mutex.Lock()
	defer atnefspf.mutex.Unlock()

	result := make(map[uint][][]group.MemberIndex)
	seen := make(map[uint]map[string]struct{})

	for _, record := range atnefspf.records {
		if seen[record.attemptNumber] == nil {
			seen[record.attemptNumber] = make(map[string]struct{})
		}

		keyParts := make([]string, 0, len(record.includedMemberIndex))
		for _, memberIndex := range record.includedMemberIndex {
			keyParts = append(
				keyParts,
				strconv.FormatUint(uint64(memberIndex), 10),
			)
		}
		cohortKey := strings.Join(keyParts, ",")

		if _, ok := seen[record.attemptNumber][cohortKey]; ok {
			continue
		}

		seen[record.attemptNumber][cohortKey] = struct{}{}
		result[record.attemptNumber] = append(
			result[record.attemptNumber],
			append([]group.MemberIndex{}, record.includedMemberIndex...),
		)
	}

	return result
}

func TestConfigureFrostSigningBackend_FFIStrictConfigured_BuildAdapter(t *testing.T) {
	frostsigning.ResetExecutionBackend()
	frostsigning.UnregisterNativeExecutionAdapter()
	frostsigning.UnregisterNativeExecutionBridge()
	frostsigning.UnregisterNativeExecutionFFIExecutor()
	frostsigning.RegisterNativeExecutionAdapterForBuild()
	t.Cleanup(frostsigning.ResetExecutionBackend)
	t.Cleanup(frostsigning.UnregisterNativeExecutionAdapter)
	t.Cleanup(frostsigning.UnregisterNativeExecutionBridge)
	t.Cleanup(frostsigning.UnregisterNativeExecutionFFIExecutor)

	err := configureFrostSigningBackend(Config{FrostSigningBackend: "ffi"})
	if err != nil {
		t.Fatalf("unexpected strict ffi backend configuration error: [%v]", err)
	}

	if frostsigning.CurrentExecutionBackendName() != frostsigning.NativeExecutionBackendName {
		t.Fatalf(
			"unexpected backend name\nexpected: [%s]\nactual:   [%s]",
			frostsigning.NativeExecutionBackendName,
			frostsigning.CurrentExecutionBackendName(),
		)
	}
}

func TestConfigureFrostSigningBackend_FFIStrictUnavailable_NoBridge(t *testing.T) {
	frostsigning.ResetExecutionBackend()
	frostsigning.UnregisterNativeExecutionAdapter()
	frostsigning.UnregisterNativeExecutionBridge()
	frostsigning.UnregisterNativeExecutionFFIExecutor()
	frostsigning.RegisterNativeExecutionAdapterForBuild()
	// Remove build-registered bridge and executor to exercise strict ffi
	// configuration when no native cryptography path is available.
	frostsigning.UnregisterNativeExecutionBridge()
	frostsigning.UnregisterNativeExecutionFFIExecutor()
	t.Cleanup(frostsigning.ResetExecutionBackend)
	t.Cleanup(frostsigning.UnregisterNativeExecutionAdapter)
	t.Cleanup(frostsigning.UnregisterNativeExecutionBridge)
	t.Cleanup(frostsigning.UnregisterNativeExecutionFFIExecutor)

	err := configureFrostSigningBackend(Config{FrostSigningBackend: "ffi"})
	if err == nil {
		t.Fatal("expected strict ffi backend configuration error")
	}

	if !errors.Is(err, frostsigning.ErrNativeExecutionBackendUnavailable) {
		t.Fatalf(
			"unexpected strict ffi backend error\nexpected: [%v]\nactual:   [%v]",
			frostsigning.ErrNativeExecutionBackendUnavailable,
			err,
		)
	}

	if !errors.Is(err, frostsigning.ErrNativeCryptographyUnavailable) {
		t.Fatalf(
			"unexpected strict ffi native-availability error\nexpected: [%v]\nactual:   [%v]",
			frostsigning.ErrNativeCryptographyUnavailable,
			err,
		)
	}
}

func TestSigningExecutor_Sign_NativeBackend(t *testing.T) {
	executor := setupSigningExecutor(t)

	frostsigning.ResetExecutionBackend()
	frostsigning.UnregisterNativeExecutionAdapter()
	frostsigning.UnregisterNativeExecutionBridge()
	frostsigning.UnregisterNativeExecutionFFIExecutor()
	frostsigning.RegisterNativeExecutionAdapterForBuild()
	t.Cleanup(frostsigning.ResetExecutionBackend)
	t.Cleanup(frostsigning.UnregisterNativeExecutionAdapter)
	t.Cleanup(frostsigning.UnregisterNativeExecutionBridge)
	t.Cleanup(frostsigning.UnregisterNativeExecutionFFIExecutor)

	err := configureFrostSigningBackend(Config{FrostSigningBackend: "native"})
	if err != nil {
		t.Fatalf("unexpected native backend config error: [%v]", err)
	}

	if frostsigning.CurrentExecutionBackendName() != frostsigning.NativeExecutionBackendName {
		t.Fatalf(
			"unexpected backend name\nexpected: [%s]\nactual:   [%s]",
			frostsigning.NativeExecutionBackendName,
			frostsigning.CurrentExecutionBackendName(),
		)
	}

	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	message := big.NewInt(100)
	startBlock := uint64(0)

	signature, _, endBlock, err := executor.sign(ctx, message, startBlock)
	if err != nil {
		t.Fatalf("unexpected native backend signing error: [%v]", err)
	}

	// Transitional path note:
	// The current native-tag adapter delegates to legacy tECDSA signing.
	// Switch this verification to Schnorr/BIP-340 once native FROST crypto
	// execution is linked.
	walletPublicKey := executor.wallet().publicKey
	if !ecdsa.Verify(
		walletPublicKey,
		message.Bytes(),
		new(big.Int).SetBytes(signature.R[:]),
		new(big.Int).SetBytes(signature.S[:]),
	) {
		t.Fatalf("invalid signature: [%+v]", signature)
	}

	if endBlock <= startBlock {
		t.Fatal("wrong end block")
	}
}

func TestSigningExecutor_Sign_FFIStrictBackend_WithNativeSignerMaterial(
	t *testing.T,
) {
	executor := setupSigningExecutor(t)
	configureSignersWithNativeFROSTUniFFIV2Material(t, executor)

	primitive := &deterministicNativeExecutionFFISigningPrimitiveForTBTC{}

	frostsigning.ResetExecutionBackend()
	frostsigning.UnregisterNativeExecutionAdapter()
	frostsigning.UnregisterNativeExecutionBridge()
	frostsigning.UnregisterNativeExecutionFFIExecutor()
	frostsigning.RegisterNativeExecutionAdapterForBuild()
	err := frostsigning.RegisterNativeExecutionFFISigningPrimitive(primitive)
	if err != nil {
		t.Fatalf("unexpected native FFI primitive registration error: [%v]", err)
	}
	t.Cleanup(frostsigning.ResetExecutionBackend)
	t.Cleanup(frostsigning.UnregisterNativeExecutionAdapter)
	t.Cleanup(frostsigning.UnregisterNativeExecutionBridge)
	t.Cleanup(frostsigning.UnregisterNativeExecutionFFIExecutor)

	err = configureFrostSigningBackend(Config{FrostSigningBackend: "ffi"})
	if err != nil {
		t.Fatalf("unexpected strict ffi backend config error: [%v]", err)
	}

	if frostsigning.CurrentExecutionBackendName() != frostsigning.NativeExecutionBackendName {
		t.Fatalf(
			"unexpected backend name\nexpected: [%s]\nactual:   [%s]",
			frostsigning.NativeExecutionBackendName,
			frostsigning.CurrentExecutionBackendName(),
		)
	}

	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	message := big.NewInt(100)
	startBlock := uint64(0)

	signature, _, endBlock, err := executor.sign(ctx, message, startBlock)
	if err != nil {
		t.Fatalf("unexpected strict ffi signing error: [%v]", err)
	}

	signatureBytes, err := signature.Marshal()
	if err != nil {
		t.Fatalf("cannot marshal signature: [%v]", err)
	}

	if !bytes.Equal(signatureBytes, deterministicNativeFROSTSignatureForTBTC[:]) {
		t.Fatalf(
			"unexpected native FROST signature\nexpected: [%x]\nactual:   [%x]",
			deterministicNativeFROSTSignatureForTBTC[:],
			signatureBytes,
		)
	}

	if primitive.signCalls.Load() == 0 {
		t.Fatal("expected native FFI primitive sign call")
	}

	if endBlock <= startBlock {
		t.Fatal("wrong end block")
	}
}

func TestSigningExecutor_Sign_NativeBackend_FallsBackWhenOnlyLegacySignerMaterial(
	t *testing.T,
) {
	executor := setupSigningExecutor(t)

	// Force legacy-only signer material to exercise fallback classification
	// behavior even when frost_native build defaults resolve to native material.
	for _, signer := range executor.signers {
		signer.signerMaterial = signer.privateKeyShare
	}

	primitive := &countingNativeExecutionFFISigningPrimitive{}

	frostsigning.ResetExecutionBackend()
	frostsigning.UnregisterNativeExecutionAdapter()
	frostsigning.UnregisterNativeExecutionBridge()
	frostsigning.UnregisterNativeExecutionFFIExecutor()
	frostsigning.RegisterNativeExecutionAdapterForBuild()
	err := frostsigning.RegisterNativeExecutionFFISigningPrimitive(primitive)
	if err != nil {
		t.Fatalf("unexpected native FFI primitive registration error: [%v]", err)
	}
	t.Cleanup(frostsigning.ResetExecutionBackend)
	t.Cleanup(frostsigning.UnregisterNativeExecutionAdapter)
	t.Cleanup(frostsigning.UnregisterNativeExecutionBridge)
	t.Cleanup(frostsigning.UnregisterNativeExecutionFFIExecutor)

	err = configureFrostSigningBackend(Config{FrostSigningBackend: "native"})
	if err != nil {
		t.Fatalf("unexpected native backend config error: [%v]", err)
	}

	if frostsigning.CurrentExecutionBackendName() != frostsigning.NativeExecutionBackendName {
		t.Fatalf(
			"unexpected backend name\nexpected: [%s]\nactual:   [%s]",
			frostsigning.NativeExecutionBackendName,
			frostsigning.CurrentExecutionBackendName(),
		)
	}

	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	message := big.NewInt(100)
	startBlock := uint64(0)

	signature, _, endBlock, err := executor.sign(ctx, message, startBlock)
	if err != nil {
		t.Fatalf("unexpected native backend signing error: [%v]", err)
	}

	if primitive.signCalls.Load() != 0 {
		t.Fatalf(
			"unexpected native primitive sign calls count\nexpected: [%d]\nactual:   [%d]",
			0,
			primitive.signCalls.Load(),
		)
	}

	walletPublicKey := executor.wallet().publicKey
	if !ecdsa.Verify(
		walletPublicKey,
		message.Bytes(),
		new(big.Int).SetBytes(signature.R[:]),
		new(big.Int).SetBytes(signature.S[:]),
	) {
		t.Fatalf("invalid signature: [%+v]", signature)
	}

	if endBlock <= startBlock {
		t.Fatal("wrong end block")
	}
}

func TestSigningExecutor_Sign_FFIStrictBackend_AttemptVariationChangesCohortSelection(
	t *testing.T,
) {
	executor := setupSigningExecutor(t)
	configureSignersWithNativeFROSTUniFFIV2Material(t, executor)

	primitive := &attemptTrackingNativeExecutionFFISigningPrimitiveForTBTC{}

	frostsigning.ResetExecutionBackend()
	frostsigning.UnregisterNativeExecutionAdapter()
	frostsigning.UnregisterNativeExecutionBridge()
	frostsigning.UnregisterNativeExecutionFFIExecutor()
	frostsigning.RegisterNativeExecutionAdapterForBuild()
	err := frostsigning.RegisterNativeExecutionFFISigningPrimitive(primitive)
	if err != nil {
		t.Fatalf("unexpected native FFI primitive registration error: [%v]", err)
	}
	t.Cleanup(frostsigning.ResetExecutionBackend)
	t.Cleanup(frostsigning.UnregisterNativeExecutionAdapter)
	t.Cleanup(frostsigning.UnregisterNativeExecutionBridge)
	t.Cleanup(frostsigning.UnregisterNativeExecutionFFIExecutor)

	err = configureFrostSigningBackend(Config{FrostSigningBackend: "ffi"})
	if err != nil {
		t.Fatalf("unexpected strict ffi backend config error: [%v]", err)
	}

	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	message := big.NewInt(100)
	startBlock := uint64(0)

	signature, _, endBlock, err := executor.sign(ctx, message, startBlock)
	if err != nil {
		t.Fatalf("unexpected strict ffi signing error: [%v]", err)
	}

	signatureBytes, err := signature.Marshal()
	if err != nil {
		t.Fatalf("cannot marshal signature: [%v]", err)
	}

	if !bytes.Equal(signatureBytes, deterministicNativeFROSTSignatureForTBTC[:]) {
		t.Fatalf(
			"unexpected native FROST signature\nexpected: [%x]\nactual:   [%x]",
			deterministicNativeFROSTSignatureForTBTC[:],
			signatureBytes,
		)
	}

	if primitive.signCalls.Load() == 0 {
		t.Fatal("expected native FFI primitive sign call")
	}

	cohortsByAttempt := primitive.uniqueCohortsByAttempt()
	attemptOneCohorts, ok := cohortsByAttempt[1]
	if !ok {
		t.Fatal("expected observed cohort for attempt 1")
	}
	if len(attemptOneCohorts) != 1 {
		t.Fatalf(
			"unexpected unique cohort count for attempt 1\nexpected: [%d]\nactual:   [%d]",
			1,
			len(attemptOneCohorts),
		)
	}

	attemptTwoCohorts, ok := cohortsByAttempt[2]
	if !ok {
		t.Fatal("expected observed cohort for attempt 2")
	}
	if len(attemptTwoCohorts) != 1 {
		t.Fatalf(
			"unexpected unique cohort count for attempt 2\nexpected: [%d]\nactual:   [%d]",
			1,
			len(attemptTwoCohorts),
		)
	}

	attemptOneCohort := attemptOneCohorts[0]
	attemptTwoCohort := attemptTwoCohorts[0]

	expectedCohortSize := executor.groupParameters.HonestThreshold
	if len(attemptOneCohort) != expectedCohortSize {
		t.Fatalf(
			"unexpected cohort size for attempt 1\nexpected: [%d]\nactual:   [%d]",
			expectedCohortSize,
			len(attemptOneCohort),
		)
	}
	if len(attemptTwoCohort) != expectedCohortSize {
		t.Fatalf(
			"unexpected cohort size for attempt 2\nexpected: [%d]\nactual:   [%d]",
			expectedCohortSize,
			len(attemptTwoCohort),
		)
	}

	if reflect.DeepEqual(attemptOneCohort, attemptTwoCohort) {
		t.Fatalf(
			"expected cohort variation across attempts\nattempt 1: [%v]\nattempt 2: [%v]",
			attemptOneCohort,
			attemptTwoCohort,
		)
	}

	if endBlock <= startBlock {
		t.Fatal("wrong end block")
	}
}

func configureSignersWithNativeFROSTUniFFIV2Material(
	t *testing.T,
	executor *signingExecutor,
) {
	t.Helper()

	publicKeyPackage := &frostsigning.NativeFROSTPublicKeyPackage{
		VerifyingShares: map[string]string{
			"1": "share-1",
		},
		VerifyingKey: "group-verifying-key",
	}

	for _, signer := range executor.signers {
		keyPackage := &frostsigning.NativeFROSTKeyPackage{
			Identifier: strconv.FormatUint(uint64(signer.signingGroupMemberIndex), 10),
			Data:       []byte{byte(signer.signingGroupMemberIndex)},
		}

		payload, err := json.Marshal(struct {
			KeyPackage       *frostsigning.NativeFROSTKeyPackage       `json:"keyPackage"`
			PublicKeyPackage *frostsigning.NativeFROSTPublicKeyPackage `json:"publicKeyPackage"`
		}{
			KeyPackage:       keyPackage,
			PublicKeyPackage: publicKeyPackage,
		})
		if err != nil {
			t.Fatalf("cannot marshal native signer material payload: [%v]", err)
		}

		signer.signerMaterial = &frostsigning.NativeSignerMaterial{
			Format:  frostsigning.NativeSignerMaterialFormatFrostUniFFIV2,
			Payload: payload,
		}
	}
}
