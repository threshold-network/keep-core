//go:build frost_native

package signing

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/keep-network/keep-core/pkg/frost"
	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/net/local"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

var deterministicNativeFROSTSigningPrivateKeyBytesForTest = [32]byte{
	0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
	0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
	0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
	0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
}

type deterministicNativeFROSTSigningEngine struct{}

func (dnfse *deterministicNativeFROSTSigningEngine) GenerateNoncesAndCommitments(
	keyPackage *NativeFROSTKeyPackage,
) (*NativeFROSTNonces, *NativeFROSTCommitment, error) {
	if keyPackage == nil {
		return nil, nil, fmt.Errorf("key package is nil")
	}

	if keyPackage.Identifier == "" {
		return nil, nil, fmt.Errorf("key package identifier is empty")
	}

	nonceSeed := sha256.Sum256(
		append(
			[]byte("nonce:"),
			[]byte(keyPackage.Identifier)...,
		),
	)
	commitmentSeed := sha256.Sum256(
		append(
			[]byte("commitment:"),
			[]byte(keyPackage.Identifier)...,
		),
	)

	return &NativeFROSTNonces{
			Data: nonceSeed[:],
		}, &NativeFROSTCommitment{
			Identifier: keyPackage.Identifier,
			Data:       commitmentSeed[:],
		}, nil
}

func (dnfse *deterministicNativeFROSTSigningEngine) NewSigningPackage(
	message []byte,
	commitments []*NativeFROSTCommitment,
) (*NativeFROSTSigningPackage, error) {
	if len(commitments) == 0 {
		return nil, fmt.Errorf("commitments are empty")
	}

	for _, commitment := range commitments {
		if commitment == nil {
			return nil, fmt.Errorf("commitment is nil")
		}
	}

	return &NativeFROSTSigningPackage{
		Data: append([]byte{}, message...),
	}, nil
}

func (dnfse *deterministicNativeFROSTSigningEngine) Sign(
	signingPackage *NativeFROSTSigningPackage,
	nonces *NativeFROSTNonces,
	keyPackage *NativeFROSTKeyPackage,
) (*NativeFROSTSignatureShare, error) {
	if signingPackage == nil {
		return nil, fmt.Errorf("signing package is nil")
	}

	if nonces == nil {
		return nil, fmt.Errorf("nonces are nil")
	}

	if keyPackage == nil {
		return nil, fmt.Errorf("key package is nil")
	}

	serialized := append([]byte{}, signingPackage.Data...)
	serialized = append(serialized, nonces.Data...)
	serialized = append(serialized, []byte(keyPackage.Identifier)...)
	serialized = append(serialized, keyPackage.Data...)

	shareDigest := sha256.Sum256(serialized)

	return &NativeFROSTSignatureShare{
		Identifier: keyPackage.Identifier,
		Data:       shareDigest[:],
	}, nil
}

func (dnfse *deterministicNativeFROSTSigningEngine) Aggregate(
	signingPackage *NativeFROSTSigningPackage,
	signatureShares []*NativeFROSTSignatureShare,
	publicKeyPackage *NativeFROSTPublicKeyPackage,
) ([]byte, error) {
	if signingPackage == nil {
		return nil, fmt.Errorf("signing package is nil")
	}

	if publicKeyPackage == nil {
		return nil, fmt.Errorf("public key package is nil")
	}

	if len(signatureShares) == 0 {
		return nil, fmt.Errorf("signature shares are empty")
	}

	for _, signatureShare := range signatureShares {
		if signatureShare == nil {
			return nil, fmt.Errorf("signature share is nil")
		}
	}

	privateKey, _ := btcec.PrivKeyFromBytes(
		deterministicNativeFROSTSigningPrivateKeyBytesForTest[:],
	)
	signature, err := schnorr.Sign(privateKey, signingPackage.Data)
	if err != nil {
		return nil, err
	}

	return signature.Serialize(), nil
}

func deterministicNativeFROSTSigningVerifyingKeyForTest() string {
	_, publicKey := btcec.PrivKeyFromBytes(
		deterministicNativeFROSTSigningPrivateKeyBytesForTest[:],
	)

	return hex.EncodeToString(schnorr.SerializePubKey(publicKey))
}

type recordingNativeFROSTSigningEngine struct {
	deterministicNativeFROSTSigningEngine
	mutex                     sync.Mutex
	commitmentIDSnapshots     [][]string
	signatureShareIDSnapshots [][]string
}

func (rnfse *recordingNativeFROSTSigningEngine) NewSigningPackage(
	message []byte,
	commitments []*NativeFROSTCommitment,
) (*NativeFROSTSigningPackage, error) {
	commitmentIDs := make([]string, 0, len(commitments))
	for _, commitment := range commitments {
		if commitment == nil {
			commitmentIDs = append(commitmentIDs, "<nil>")
			continue
		}

		commitmentIDs = append(commitmentIDs, commitment.Identifier)
	}

	rnfse.mutex.Lock()
	rnfse.commitmentIDSnapshots = append(
		rnfse.commitmentIDSnapshots,
		append([]string{}, commitmentIDs...),
	)
	rnfse.mutex.Unlock()

	return rnfse.deterministicNativeFROSTSigningEngine.NewSigningPackage(
		message,
		commitments,
	)
}

func (rnfse *recordingNativeFROSTSigningEngine) Aggregate(
	signingPackage *NativeFROSTSigningPackage,
	signatureShares []*NativeFROSTSignatureShare,
	publicKeyPackage *NativeFROSTPublicKeyPackage,
) ([]byte, error) {
	signatureShareIDs := make([]string, 0, len(signatureShares))
	for _, signatureShare := range signatureShares {
		if signatureShare == nil {
			signatureShareIDs = append(signatureShareIDs, "<nil>")
			continue
		}

		signatureShareIDs = append(signatureShareIDs, signatureShare.Identifier)
	}

	rnfse.mutex.Lock()
	rnfse.signatureShareIDSnapshots = append(
		rnfse.signatureShareIDSnapshots,
		append([]string{}, signatureShareIDs...),
	)
	rnfse.mutex.Unlock()

	return rnfse.deterministicNativeFROSTSigningEngine.Aggregate(
		signingPackage,
		signatureShares,
		publicKeyPackage,
	)
}

func (rnfse *recordingNativeFROSTSigningEngine) commitmentIDs() [][]string {
	rnfse.mutex.Lock()
	defer rnfse.mutex.Unlock()

	snapshots := make([][]string, 0, len(rnfse.commitmentIDSnapshots))
	for _, snapshot := range rnfse.commitmentIDSnapshots {
		snapshots = append(snapshots, append([]string{}, snapshot...))
	}

	return snapshots
}

func (rnfse *recordingNativeFROSTSigningEngine) signatureShareIDs() [][]string {
	rnfse.mutex.Lock()
	defer rnfse.mutex.Unlock()

	snapshots := make([][]string, 0, len(rnfse.signatureShareIDSnapshots))
	for _, snapshot := range rnfse.signatureShareIDSnapshots {
		snapshots = append(snapshots, append([]string{}, snapshot...))
	}

	return snapshots
}

func TestBuildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive_Sign_NativeFROSTPath(
	t *testing.T,
) {
	RegisterNativeFROSTSigningEngine(&deterministicNativeFROSTSigningEngine{})
	t.Cleanup(UnregisterNativeFROSTSigningEngine)

	provider := local.Connect()
	channel, err := provider.BroadcastChannelFor("native-frost-signing-protocol-test")
	if err != nil {
		t.Fatalf("failed creating broadcast channel: [%v]", err)
	}

	primitive := &buildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive{}
	primitive.RegisterUnmarshallers(channel)

	participantCount := 3
	includedMembers := []group.MemberIndex{1, 2, 3}

	requests := make([]*NativeExecutionFFISigningRequest, participantCount)
	for i := 0; i < participantCount; i++ {
		memberIndex := group.MemberIndex(i + 1)
		requests[i], err = newNativeFROSTSigningRequestForTest(
			memberIndex,
			includedMembers,
			channel,
			participantCount,
		)
		if err != nil {
			t.Fatalf("failed preparing request for member [%v]: [%v]", memberIndex, err)
		}
	}

	ctx, cancelCtx := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelCtx()

	results := make([]*frostSignatureResultForTest, participantCount)
	wg := sync.WaitGroup{}
	wg.Add(participantCount)

	for i := 0; i < participantCount; i++ {
		go func(index int) {
			defer wg.Done()

			signature, signErr := primitive.Sign(ctx, nil, requests[index])
			results[index] = &frostSignatureResultForTest{
				signature: signature,
				err:       signErr,
			}
		}(i)
	}

	wg.Wait()

	for i, result := range results {
		if result == nil {
			t.Fatalf("missing result for member [%v]", i+1)
		}

		if result.err != nil {
			t.Fatalf(
				"unexpected signing error for member [%v]: [%v]",
				i+1,
				result.err,
			)
		}

		if result.signature == nil {
			t.Fatalf("nil signature for member [%v]", i+1)
		}
	}

	for i := 1; i < participantCount; i++ {
		if !results[0].signature.Equals(results[i].signature) {
			t.Fatalf(
				"signature mismatch\nfirst:  [%v]\nsecond: [%v]",
				results[0].signature,
				results[i].signature,
			)
		}
	}

	assertNativeFROSTSignatureVerifiesBIP340(
		t,
		results[0].signature,
		requests[0],
	)
}

func TestVerifyNativeFROSTBIP340SignatureRejectsInvalidAggregate(
	t *testing.T,
) {
	messageDigest, err := messageDigestFromBigInt(bigOneForTest())
	if err != nil {
		t.Fatalf("unexpected message digest error: [%v]", err)
	}

	err = verifyNativeFROSTBIP340Signature(
		&frost.Signature{},
		messageDigest,
		&NativeFROSTPublicKeyPackage{
			VerifyingKey: deterministicNativeFROSTSigningVerifyingKeyForTest(),
		},
		nil,
	)
	if err == nil {
		t.Fatal("expected invalid BIP-340 aggregate signature to be rejected")
	}
}

func TestBuildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive_Sign_NativeFROSTPath_AttemptVariationUsesCohortSelections(
	t *testing.T,
) {
	engine := &recordingNativeFROSTSigningEngine{}
	RegisterNativeFROSTSigningEngine(engine)
	t.Cleanup(UnregisterNativeFROSTSigningEngine)

	provider := local.Connect()
	channel, err := provider.BroadcastChannelFor("native-frost-signing-protocol-attempt-variation-test")
	if err != nil {
		t.Fatalf("failed creating broadcast channel: [%v]", err)
	}

	primitive := &buildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive{}
	primitive.RegisterUnmarshallers(channel)

	runRound := func(
		sessionID string,
		includedMembers []group.MemberIndex,
		groupSize int,
	) []*frost.Signature {
		requests := make([]*NativeExecutionFFISigningRequest, len(includedMembers))
		for i := 0; i < len(includedMembers); i++ {
			memberIndex := includedMembers[i]

			request, roundErr := newNativeFROSTSigningRequestWithSessionForTest(
				memberIndex,
				includedMembers,
				channel,
				groupSize,
				sessionID,
			)
			if roundErr != nil {
				t.Fatalf(
					"failed preparing request for member [%v] in session [%s]: [%v]",
					memberIndex,
					sessionID,
					roundErr,
				)
			}

			requests[i] = request
		}

		ctx, cancelCtx := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelCtx()

		results := make([]*frostSignatureResultForTest, len(includedMembers))
		var wg sync.WaitGroup
		wg.Add(len(includedMembers))

		for i := 0; i < len(includedMembers); i++ {
			go func(index int) {
				defer wg.Done()

				signature, signErr := primitive.Sign(ctx, nil, requests[index])
				results[index] = &frostSignatureResultForTest{
					signature: signature,
					err:       signErr,
				}
			}(i)
		}

		wg.Wait()

		signatures := make([]*frost.Signature, len(includedMembers))
		for i := 0; i < len(includedMembers); i++ {
			if results[i] == nil {
				t.Fatalf(
					"missing signing result for member [%v] in session [%s]",
					includedMembers[i],
					sessionID,
				)
			}

			if results[i].err != nil {
				t.Fatalf(
					"unexpected signing error for member [%v] in session [%s]: [%v]",
					includedMembers[i],
					sessionID,
					results[i].err,
				)
			}

			if results[i].signature == nil {
				t.Fatalf(
					"nil signature for member [%v] in session [%s]",
					includedMembers[i],
					sessionID,
				)
			}

			signatures[i] = results[i].signature
		}

		return signatures
	}

	assertSignaturesMatch := func(
		sessionID string,
		signatures []*frost.Signature,
	) {
		if len(signatures) == 0 {
			t.Fatalf("no signatures for session [%s]", sessionID)
		}

		for i := 1; i < len(signatures); i++ {
			if !signatures[0].Equals(signatures[i]) {
				t.Fatalf(
					"signature mismatch in session [%s]\nfirst:  [%v]\nsecond: [%v]",
					sessionID,
					signatures[0],
					signatures[i],
				)
			}
		}
	}

	roundOneSignatures := runRound(
		"native-frost-signing-session-attempt-1",
		[]group.MemberIndex{1, 2, 3},
		3,
	)
	assertSignaturesMatch("native-frost-signing-session-attempt-1", roundOneSignatures)

	roundTwoSignatures := runRound(
		"native-frost-signing-session-attempt-2",
		[]group.MemberIndex{1, 3},
		3,
	)
	assertSignaturesMatch("native-frost-signing-session-attempt-2", roundTwoSignatures)

	snapshotHistogram := func(snapshots [][]string) map[string]int {
		histogram := make(map[string]int)
		for _, snapshot := range snapshots {
			histogram[strings.Join(snapshot, ",")]++
		}

		return histogram
	}

	expectedHistogram := map[string]int{
		"member-1,member-2,member-3": 3,
		"member-1,member-3":          2,
	}

	assertHistogram := func(name string, actual map[string]int) {
		if len(actual) != len(expectedHistogram) {
			t.Fatalf(
				"unexpected %s histogram size\nexpected: [%v]\nactual:   [%v]",
				name,
				len(expectedHistogram),
				len(actual),
			)
		}

		for key, expectedCount := range expectedHistogram {
			actualCount, ok := actual[key]
			if !ok {
				t.Fatalf("missing %s histogram key: [%s]", name, key)
			}

			if actualCount != expectedCount {
				t.Fatalf(
					"unexpected %s count for key [%s]\nexpected: [%v]\nactual:   [%v]",
					name,
					key,
					expectedCount,
					actualCount,
				)
			}
		}
	}

	assertHistogram(
		"commitment IDs",
		snapshotHistogram(engine.commitmentIDs()),
	)
	assertHistogram(
		"signature share IDs",
		snapshotHistogram(engine.signatureShareIDs()),
	)
}

func TestBuildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive_Sign_NativeFROSTPathWithoutEngine(
	t *testing.T,
) {
	UnregisterNativeFROSTSigningEngine()
	t.Cleanup(UnregisterNativeFROSTSigningEngine)

	provider := local.Connect()
	channel, err := provider.BroadcastChannelFor("native-frost-signing-protocol-unavailable-test")
	if err != nil {
		t.Fatalf("failed creating broadcast channel: [%v]", err)
	}

	primitive := &buildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive{}
	primitive.RegisterUnmarshallers(channel)

	request, err := newNativeFROSTSigningRequestForTest(
		1,
		[]group.MemberIndex{1},
		channel,
		1,
	)
	if err != nil {
		t.Fatalf("failed creating native request: [%v]", err)
	}

	_, err = primitive.Sign(context.Background(), nil, request)
	if err == nil {
		t.Fatal("expected error")
	}

	if !errors.Is(err, ErrNativeCryptographyUnavailable) {
		t.Fatalf(
			"unexpected error\nexpected: [%v]\nactual:   [%v]",
			ErrNativeCryptographyUnavailable,
			err,
		)
	}
}

type frostSignatureResultForTest struct {
	signature *frost.Signature
	err       error
}

func newNativeFROSTSigningRequestForTest(
	memberIndex group.MemberIndex,
	includedMembers []group.MemberIndex,
	channel net.BroadcastChannel,
	groupSize int,
) (*NativeExecutionFFISigningRequest, error) {
	return newNativeFROSTSigningRequestWithSessionForTest(
		memberIndex,
		includedMembers,
		channel,
		groupSize,
		"native-frost-signing-session",
	)
}

func newNativeFROSTSigningRequestWithSessionForTest(
	memberIndex group.MemberIndex,
	includedMembers []group.MemberIndex,
	channel net.BroadcastChannel,
	groupSize int,
	sessionID string,
) (*NativeExecutionFFISigningRequest, error) {
	keyPackage := &NativeFROSTKeyPackage{
		Identifier: fmt.Sprintf("member-%v", memberIndex),
		Data: []byte{
			byte(memberIndex),
			0x01,
		},
	}

	verifyingShares := make(map[string]string)
	for i := 1; i <= groupSize; i++ {
		verifyingShares[fmt.Sprintf("member-%v", i)] = fmt.Sprintf("share-%v", i)
	}

	payload, err := json.Marshal(&nativeFROSTUniFFIV2SignerMaterial{
		KeyPackage: keyPackage,
		PublicKeyPackage: &NativeFROSTPublicKeyPackage{
			VerifyingShares: verifyingShares,
			VerifyingKey:    deterministicNativeFROSTSigningVerifyingKeyForTest(),
		},
	})
	if err != nil {
		return nil, err
	}

	return &NativeExecutionFFISigningRequest{
		Message:            bigOneForTest(),
		SessionID:          sessionID,
		MemberIndex:        memberIndex,
		GroupSize:          groupSize,
		DishonestThreshold: 1,
		Channel:            channel,
		SignerMaterial: &NativeSignerMaterial{
			Format:  NativeSignerMaterialFormatFrostUniFFIV2,
			Payload: payload,
		},
		Attempt: &Attempt{
			Number:                 1,
			CoordinatorMemberIndex: includedMembers[0],
			IncludedMembersIndexes: append([]group.MemberIndex{}, includedMembers...),
		},
	}, nil
}

func bigOneForTest() *big.Int {
	return big.NewInt(1)
}

func assertNativeFROSTSignatureVerifiesBIP340(
	t *testing.T,
	signature *frost.Signature,
	request *NativeExecutionFFISigningRequest,
) {
	t.Helper()

	messageDigest, err := messageDigestFromBigInt(request.Message)
	if err != nil {
		t.Fatalf("unexpected message digest error: [%v]", err)
	}

	publicKeyBytes, err := hex.DecodeString(
		deterministicNativeFROSTSigningVerifyingKeyForTest(),
	)
	if err != nil {
		t.Fatalf("unexpected verifying key decode error: [%v]", err)
	}

	publicKey, err := schnorr.ParsePubKey(publicKeyBytes)
	if err != nil {
		t.Fatalf("unexpected verifying key parse error: [%v]", err)
	}

	signatureBytes := signature.Serialize()
	parsedSignature, err := schnorr.ParseSignature(signatureBytes[:])
	if err != nil {
		t.Fatalf("unexpected signature parse error: [%v]", err)
	}

	if !parsedSignature.Verify(messageDigest[:], publicKey) {
		t.Fatal("expected native FROST aggregate signature to verify as BIP-340")
	}
}
