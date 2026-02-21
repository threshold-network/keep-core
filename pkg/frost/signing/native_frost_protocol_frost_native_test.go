//go:build frost_native

package signing

import (
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/frost"
	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/net/local"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

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

	serialized := append([]byte{}, message...)
	for _, commitment := range commitments {
		if commitment == nil {
			return nil, fmt.Errorf("commitment is nil")
		}

		serialized = append(serialized, []byte(commitment.Identifier)...)
		serialized = append(serialized, commitment.Data...)
	}

	packageDigest := sha256.Sum256(serialized)

	return &NativeFROSTSigningPackage{
		Data: packageDigest[:],
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

	orderedSignatureShares := append([]*NativeFROSTSignatureShare{}, signatureShares...)
	sort.Slice(orderedSignatureShares, func(i, j int) bool {
		return orderedSignatureShares[i].Identifier < orderedSignatureShares[j].Identifier
	})

	serialized := append([]byte{}, signingPackage.Data...)
	for _, signatureShare := range orderedSignatureShares {
		if signatureShare == nil {
			return nil, fmt.Errorf("signature share is nil")
		}

		serialized = append(serialized, []byte(signatureShare.Identifier)...)
		serialized = append(serialized, signatureShare.Data...)
	}

	serialized = append(serialized, []byte(publicKeyPackage.VerifyingKey)...)

	signatureDigest := sha512.Sum512(serialized)

	return signatureDigest[:], nil
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
			VerifyingKey:    "verifying-key",
		},
	})
	if err != nil {
		return nil, err
	}

	return &NativeExecutionFFISigningRequest{
		Message:            bigOneForTest(),
		SessionID:          "native-frost-signing-session",
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
