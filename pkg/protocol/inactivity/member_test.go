package inactivity

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"math/big"
	"reflect"
	"testing"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/chain/local_v1"
	"github.com/keep-network/keep-core/pkg/crypto/secp256k1"
	"github.com/keep-network/keep-core/pkg/operator"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

func TestShouldAcceptMessage(t *testing.T) {
	groupSize := 5
	honestThreshold := 3

	localChain := local_v1.Connect(groupSize, honestThreshold)

	operatorsAddresses := make([]chain.Address, groupSize)
	operatorsPublicKeys := make([][]byte, groupSize)
	for i := range operatorsAddresses {
		_, operatorPublicKey, err := operator.GenerateKeyPair(
			local_v1.DefaultCurve,
		)
		if err != nil {
			t.Fatal(err)
		}

		operatorAddress, err := localChain.Signing().PublicKeyToAddress(
			operatorPublicKey,
		)
		if err != nil {
			t.Fatal(err)
		}

		operatorsAddresses[i] = operatorAddress
		operatorsPublicKeys[i] = operator.MarshalUncompressed(operatorPublicKey)
	}

	tests := map[string]struct {
		senderIndex        group.MemberIndex
		senderPublicKey    []byte
		inactiveMembersIDs []group.MemberIndex
		expectedResult     bool
	}{
		"message from another valid and operating member": {
			senderIndex:        group.MemberIndex(2),
			senderPublicKey:    operatorsPublicKeys[1],
			inactiveMembersIDs: []group.MemberIndex{},
			expectedResult:     true,
		},
		"message from another valid but non-operating member": {
			senderIndex:        group.MemberIndex(2),
			senderPublicKey:    operatorsPublicKeys[1],
			inactiveMembersIDs: []group.MemberIndex{2},
			expectedResult:     false,
		},
		"message from self": {
			senderIndex:        group.MemberIndex(1),
			senderPublicKey:    operatorsPublicKeys[0],
			inactiveMembersIDs: []group.MemberIndex{},
			expectedResult:     false,
		},
		"message from another invalid member": {
			senderIndex:        group.MemberIndex(2),
			senderPublicKey:    operatorsPublicKeys[3],
			inactiveMembersIDs: []group.MemberIndex{},
			expectedResult:     false,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			membershipValdator := group.NewMembershipValidator(
				&testutils.MockLogger{},
				operatorsAddresses,
				localChain.Signing(),
			)

			member := newSigningMember(
				&testutils.MockLogger{},
				group.MemberIndex(1),
				groupSize,
				groupSize-honestThreshold,
				membershipValdator,
				"session_1",
			)

			for _, inactiveMemberID := range test.inactiveMembersIDs {
				member.group.MarkMemberAsInactive(inactiveMemberID)
			}

			result := member.shouldAcceptMessage(test.senderIndex, test.senderPublicKey)

			testutils.AssertBoolsEqual(
				t,
				"result from message validator",
				test.expectedResult,
				result,
			)
		})
	}
}

func TestSignClaim(t *testing.T) {
	signingMember := initializeSigningMember(t)

	walletPublicKeyHex, err := hex.DecodeString(
		"0471e30bca60f6548d7b42582a478ea37ada63b402af7b3ddd57f0c95bb6843175" +
			"aa0d2053a91a050a6797d85c38f2909cb7027f2344a01986aa2f9f8ca7a0c289",
	)
	if err != nil {
		t.Fatal(err)
	}

	walletPublicKey := unmarshalPublicKey(walletPublicKeyHex)

	claim := NewClaimPreimage(
		big.NewInt(3),
		walletPublicKey,
		[]group.MemberIndex{1, 3},
		true,
	)

	publicKey := []byte("publicKey")
	signature := []byte("signature")
	claimHash := ClaimHash{0: 11, 6: 22, 31: 33}
	sessionID := signingMember.sessionID

	claimSigner := newMockClaimSigner(publicKey)
	claimSigner.setSigningOutcome(claim, &signingOutcome{
		signature: signature,
		claimHash: claimHash,
		err:       nil,
	})

	actualSignatureMessage, err := signingMember.signClaim(
		claim,
		claimSigner,
	)
	if err != nil {
		t.Fatal(err)
	}

	expectedSignatureMessage := &claimSignatureMessage{
		senderID:  signingMember.memberIndex,
		claimHash: claimHash,
		signature: signature,
		publicKey: publicKey,
		sessionID: sessionID,
	}

	if !reflect.DeepEqual(
		expectedSignatureMessage,
		actualSignatureMessage,
	) {
		t.Errorf(
			"unexpected signature message \nexpected: %v\nactual:   %v\n",
			expectedSignatureMessage,
			actualSignatureMessage,
		)
	}

	if !bytes.Equal(signature, signingMember.selfInactivityClaimSignature) {
		t.Errorf(
			"unexpected self inactivity claim signature\nexpected: %v\nactual:   %v\n",
			signature,
			signingMember.selfInactivityClaimSignature,
		)
	}

	if claimHash != signingMember.preferredInactivityClaimHash {
		t.Errorf(
			"unexpected preferred inactivity claim hash\nexpected: %v\nactual:   %v\n",
			claimHash,
			signingMember.preferredInactivityClaimHash,
		)
	}
}

func TestSignClaim_ErrorDuringSigning(t *testing.T) {
	signingMember := initializeSigningMember(t)

	walletPublicKeyHex, err := hex.DecodeString(
		"0471e30bca60f6548d7b42582a478ea37ada63b402af7b3ddd57f0c95bb6843175" +
			"aa0d2053a91a050a6797d85c38f2909cb7027f2344a01986aa2f9f8ca7a0c289",
	)
	if err != nil {
		t.Fatal(err)
	}

	walletPublicKey := unmarshalPublicKey(walletPublicKeyHex)

	claim := NewClaimPreimage(
		big.NewInt(3),
		walletPublicKey,
		[]group.MemberIndex{1, 3},
		true,
	)

	claimSigner := newMockClaimSigner([]byte("publicKey"))
	claimSigner.setSigningOutcome(claim, &signingOutcome{
		signature: []byte("signature"),
		claimHash: ClaimHash{0: 11, 6: 22, 31: 33},
		err:       fmt.Errorf("dummy error"),
	})

	_, err = signingMember.signClaim(
		claim,
		claimSigner,
	)

	expectedErr := fmt.Errorf("failed to sign inactivity claim [dummy error]")
	if !reflect.DeepEqual(expectedErr, err) {
		t.Errorf(
			"unexpected error\nexpected: %v\nactual:   %v\n",
			expectedErr,
			err,
		)
	}
}

func TestVerifyInactivityClaimSignatures(t *testing.T) {
	signingMember := initializeSigningMember(t)
	signingMember.preferredInactivityClaimHash = ClaimHash{11: 11}
	signingMember.selfInactivityClaimSignature = []byte("sign 1")

	type messageWithOutcome struct {
		message *claimSignatureMessage
		outcome *verificationOutcome
	}

	tests := map[string]struct {
		messagesWithOutcomes    []messageWithOutcome
		expectedValidSignatures map[group.MemberIndex][]byte
	}{
		"messages from other members with valid signatures for the preferred claim": {
			messagesWithOutcomes: []messageWithOutcome{
				{
					&claimSignatureMessage{
						senderID:  2,
						claimHash: ClaimHash{11: 11},
						signature: []byte("sign 2"),
						publicKey: []byte("pubKey 2"),
						sessionID: "session-1",
					},
					&verificationOutcome{
						isValid: true,
						err:     nil,
					},
				},
				{
					&claimSignatureMessage{
						senderID:  3,
						claimHash: ClaimHash{11: 11},
						signature: []byte("sign 3"),
						publicKey: []byte("pubKey 3"),
						sessionID: "session-1",
					},
					&verificationOutcome{
						isValid: true,
						err:     nil,
					},
				},
			},
			expectedValidSignatures: map[group.MemberIndex][]byte{
				signingMember.memberIndex: signingMember.selfInactivityClaimSignature,
				2:                         []byte("sign 2"),
				3:                         []byte("sign 3"),
			},
		},
		"received a message from other member with signature for claim " +
			"different than preferred": {
			messagesWithOutcomes: []messageWithOutcome{
				{
					&claimSignatureMessage{
						senderID:  2,
						claimHash: ClaimHash{12: 12},
						signature: []byte("sign 2"),
						publicKey: []byte("pubKey 2"),
						sessionID: "session-1",
					},
					&verificationOutcome{
						isValid: true,
						err:     nil,
					},
				},
			},
			expectedValidSignatures: map[group.MemberIndex][]byte{
				signingMember.memberIndex: signingMember.selfInactivityClaimSignature,
			},
		},
		"message from other member that causes an error during signature " +
			"verification": {
			messagesWithOutcomes: []messageWithOutcome{
				{
					&claimSignatureMessage{
						senderID:  2,
						claimHash: ClaimHash{11: 11},
						signature: []byte("sign 2"),
						publicKey: []byte("pubKey 2"),
						sessionID: "session-1",
					},
					&verificationOutcome{
						isValid: false,
						err:     fmt.Errorf("dummy error"),
					},
				},
			},
			expectedValidSignatures: map[group.MemberIndex][]byte{
				signingMember.memberIndex: signingMember.selfInactivityClaimSignature,
			},
		},
		"message from other member with invalid signature": {
			messagesWithOutcomes: []messageWithOutcome{
				{
					&claimSignatureMessage{
						senderID:  2,
						claimHash: ClaimHash{11: 11},
						signature: []byte("bad sign"),
						publicKey: []byte("pubKey 2"),
						sessionID: "session-1",
					},
					&verificationOutcome{
						isValid: false,
						err:     nil,
					},
				},
			},
			expectedValidSignatures: map[group.MemberIndex][]byte{
				signingMember.memberIndex: signingMember.selfInactivityClaimSignature,
			},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			claimSigner := newMockClaimSigner([]byte("publicKey"))

			var messages []*claimSignatureMessage
			for _, messageWithOutcome := range test.messagesWithOutcomes {
				messages = append(messages, messageWithOutcome.message)
				claimSigner.setVerificationOutcome(
					messageWithOutcome.message,
					messageWithOutcome.outcome,
				)
			}

			validSignatures := signingMember.verifyInactivityClaimSignatures(
				messages,
				claimSigner,
			)
			if !reflect.DeepEqual(validSignatures, test.expectedValidSignatures) {
				t.Errorf(
					"unexpected valid signatures\nexpected: %v\nactual:   %v\n",
					test.expectedValidSignatures,
					validSignatures,
				)
			}
		})
	}
}

func TestSubmitClaim(t *testing.T) {
	submittingMember := initializeSubmittingMember(t)

	claim := &ClaimPreimage{}
	signatures := map[group.MemberIndex][]byte{
		11: []byte("signature 11"),
		22: []byte("signature 22"),
		33: []byte("signature 33"),
	}

	claimSubmitter := newMockClaimSubmitter()
	claimSubmitter.setSubmittingOutcome(claim, nil)

	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	err := submittingMember.submitClaim(
		ctx,
		claim,
		signatures,
		claimSubmitter,
	)
	if err != nil {
		t.Fatal(err)
	}
}

func initializeSigningMember(t *testing.T) *signingMember {
	groupSize := 5
	honestThreshold := 3

	localChain := local_v1.Connect(groupSize, honestThreshold)

	operatorsAddresses := make([]chain.Address, groupSize)
	operatorsPublicKeys := make([][]byte, groupSize)
	for i := range operatorsAddresses {
		_, operatorPublicKey, err := operator.GenerateKeyPair(
			local_v1.DefaultCurve,
		)
		if err != nil {
			t.Fatal(err)
		}

		operatorAddress, err := localChain.Signing().PublicKeyToAddress(
			operatorPublicKey,
		)
		if err != nil {
			t.Fatal(err)
		}

		operatorsAddresses[i] = operatorAddress
		operatorsPublicKeys[i] = operator.MarshalUncompressed(operatorPublicKey)
	}

	membershipValidator := group.NewMembershipValidator(
		&testutils.MockLogger{},
		operatorsAddresses,
		localChain.Signing(),
	)

	return newSigningMember(
		&testutils.MockLogger{},
		group.MemberIndex(1),
		groupSize,
		groupSize-honestThreshold,
		membershipValidator,
		"session_1",
	)
}

func initializeSubmittingMember(t *testing.T) *submittingMember {
	signingMember := initializeSigningMember(t)
	return signingMember.initializeSubmittingMember()
}

type signingOutcome struct {
	signature []byte
	claimHash ClaimHash
	err       error
}

type verificationOutcome struct {
	isValid bool
	err     error
}

type mockClaimSigner struct {
	publicKey            []byte
	signingOutcomes      map[*ClaimPreimage]*signingOutcome
	verificationOutcomes map[string]*verificationOutcome
}

func newMockClaimSigner(publicKey []byte) *mockClaimSigner {
	return &mockClaimSigner{
		publicKey:            publicKey,
		signingOutcomes:      make(map[*ClaimPreimage]*signingOutcome),
		verificationOutcomes: make(map[string]*verificationOutcome),
	}
}

func (mrs *mockClaimSigner) setSigningOutcome(
	claim *ClaimPreimage,
	outcome *signingOutcome,
) {
	mrs.signingOutcomes[claim] = outcome
}

func (mrs *mockClaimSigner) setVerificationOutcome(
	message *claimSignatureMessage,
	outcome *verificationOutcome,
) {
	key := signatureVerificationKey(
		message.publicKey,
		message.signature,
		message.claimHash,
	)
	mrs.verificationOutcomes[key] = outcome
}

func (mrs *mockClaimSigner) SignClaim(claim *ClaimPreimage) (*SignedClaimHash, error) {
	if outcome, ok := mrs.signingOutcomes[claim]; ok {
		return &SignedClaimHash{
			PublicKey: mrs.publicKey,
			Signature: outcome.signature,
			ClaimHash: outcome.claimHash,
		}, outcome.err
	}

	return nil, fmt.Errorf(
		"could not find singing outcome for the inactivity claim",
	)
}

func (mrs *mockClaimSigner) VerifySignature(signedClaimHash *SignedClaimHash) (bool, error) {
	key := signatureVerificationKey(
		signedClaimHash.PublicKey,
		signedClaimHash.Signature,
		signedClaimHash.ClaimHash,
	)
	if outcome, ok := mrs.verificationOutcomes[key]; ok {
		return outcome.isValid, outcome.err
	}

	return false, fmt.Errorf(
		"could not find signature verification outcome for the signed claim",
	)
}

func signatureVerificationKey(
	publicKey []byte,
	signature []byte,
	claimHash ClaimHash,
) string {
	return fmt.Sprintf("%s-%s-%s", publicKey, signature, claimHash[:])
}

type mockClaimSubmitter struct {
	submittingOutcomes map[*ClaimPreimage]error
}

func newMockClaimSubmitter() *mockClaimSubmitter {
	return &mockClaimSubmitter{
		submittingOutcomes: make(map[*ClaimPreimage]error),
	}
}

func (mrs *mockClaimSubmitter) setSubmittingOutcome(
	claim *ClaimPreimage,
	err error,
) {
	mrs.submittingOutcomes[claim] = err
}

func (mrs *mockClaimSubmitter) SubmitClaim(
	ctx context.Context,
	memberIndex group.MemberIndex,
	claim *ClaimPreimage,
	signatures map[group.MemberIndex][]byte,
) error {
	if err, ok := mrs.submittingOutcomes[claim]; ok {
		return err
	}
	return fmt.Errorf(
		"could not find submitting outcome for the claim",
	)
}

func unmarshalPublicKey(bytes []byte) *ecdsa.PublicKey {
	publicKey, err := secp256k1.Unmarshal(bytes)
	if err != nil {
		panic(err)
	}

	return publicKey
}
