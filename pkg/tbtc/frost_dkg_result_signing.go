package tbtc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"sort"

	"github.com/keep-network/keep-core/pkg/frost/registry"
	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

const frostDKGResultSigningMessageTypePrefix = "frost_dkg/result_signing/"

type frostDKGResultSignatureMessage struct {
	SenderIDValue uint32 `json:"senderID"`
	SessionID     string `json:"sessionID"`
	Digest        []byte `json:"digest"`
	PublicKey     []byte `json:"publicKey"`
	Signature     []byte `json:"signature"`
}

func (fdrsm *frostDKGResultSignatureMessage) SenderID() group.MemberIndex {
	return group.MemberIndex(fdrsm.SenderIDValue)
}

func (fdrsm *frostDKGResultSignatureMessage) Type() string {
	return frostDKGResultSigningMessageTypePrefix + "signature"
}

func (fdrsm *frostDKGResultSignatureMessage) Marshal() ([]byte, error) {
	return json.Marshal(fdrsm)
}

func (fdrsm *frostDKGResultSignatureMessage) Unmarshal(data []byte) error {
	if err := json.Unmarshal(data, fdrsm); err != nil {
		return err
	}

	if fdrsm.SenderID() == 0 {
		return fmt.Errorf("sender ID is zero")
	}
	if fdrsm.SessionID == "" {
		return fmt.Errorf("session ID is empty")
	}
	if len(fdrsm.Digest) != 32 {
		return fmt.Errorf("digest length [%d] is not 32", len(fdrsm.Digest))
	}
	if len(fdrsm.PublicKey) == 0 {
		return fmt.Errorf("public key is empty")
	}
	if len(fdrsm.Signature) == 0 {
		return fmt.Errorf("signature is empty")
	}

	return nil
}

func registerFrostDKGResultSigningUnmarshaller(channel net.BroadcastChannel) {
	channel.SetUnmarshaler(func() net.TaggedUnmarshaler {
		return &frostDKGResultSignatureMessage{}
	})
}

func signAndCollectFrostDKGResultSignatures(
	ctx context.Context,
	node *node,
	frostChain FrostDKGChain,
	channel net.BroadcastChannel,
	membershipValidator *group.MembershipValidator,
	sessionID string,
	seed *big.Int,
	memberIndex group.MemberIndex,
	includedMembersIndexes []group.MemberIndex,
	groupSelectionResult *GroupSelectionResult,
	unsignedResult *registry.Result,
) (*registry.Result, error) {
	if unsignedResult == nil {
		return nil, fmt.Errorf("unsigned FROST DKG result is nil")
	}

	includedMembersSet := make(map[group.MemberIndex]struct{})
	for _, includedMemberIndex := range includedMembersIndexes {
		includedMembersSet[includedMemberIndex] = struct{}{}
	}

	digest, err := frostChain.CalculateFrostDKGResultDigest(seed, unsignedResult)
	if err != nil {
		return nil, fmt.Errorf("cannot calculate FROST DKG result digest: [%w]", err)
	}

	signing := node.chain.Signing()
	ownSignature, err := signing.Sign(digest[:])
	if err != nil {
		return nil, fmt.Errorf("cannot sign FROST DKG result digest: [%w]", err)
	}

	ownMessage := &frostDKGResultSignatureMessage{
		SenderIDValue: uint32(memberIndex),
		SessionID:     sessionID,
		Digest:        append([]byte{}, digest[:]...),
		PublicKey:     append([]byte{}, signing.PublicKey()...),
		Signature:     append([]byte{}, ownSignature...),
	}
	if err := channel.Send(
		ctx,
		ownMessage,
		net.BackoffRetransmissionStrategy,
	); err != nil {
		return nil, fmt.Errorf("cannot broadcast FROST DKG result signature: [%w]", err)
	}

	signatures := map[group.MemberIndex][]byte{
		memberIndex: ownSignature,
	}

	expectedSignaturesCount := node.groupParameters.GroupQuorum
	if expectedSignaturesCount > len(includedMembersIndexes) {
		return nil, fmt.Errorf(
			"FROST DKG included members count [%d] is below quorum [%d]",
			len(includedMembersIndexes),
			expectedSignaturesCount,
		)
	}

	recvCtx, cancelRecvCtx := context.WithCancel(ctx)
	defer cancelRecvCtx()

	messageChan := make(chan *frostDKGResultSignatureMessage, len(includedMembersIndexes)*4+1)
	channel.Recv(recvCtx, func(message net.Message) {
		payload, ok := message.Payload().(*frostDKGResultSignatureMessage)
		if !ok {
			return
		}

		if !shouldAcceptFrostDKGResultSignatureMessage(
			payload,
			message.SenderPublicKey(),
			sessionID,
			memberIndex,
			includedMembersSet,
			membershipValidator,
		) {
			return
		}

		select {
		case messageChan <- payload:
		default:
			logger.Warnf(
				"dropping FROST DKG result signature from member [%d]; collector buffer full",
				payload.SenderID(),
			)
		}
	})

	for len(signatures) < expectedSignaturesCount {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf(
				"FROST DKG result signature collection interrupted: [%w]",
				ctx.Err(),
			)
		case message := <-messageChan:
			senderID := message.SenderID()
			if !bytes.Equal(message.Digest, digest[:]) {
				logger.Warnf(
					"dropping FROST DKG result signature from member [%d]; digest mismatch",
					senderID,
				)
				continue
			}

			valid, err := signing.VerifyWithPublicKey(
				digest[:],
				message.Signature,
				message.PublicKey,
			)
			if err != nil || !valid {
				logger.Warnf(
					"dropping invalid FROST DKG result signature from member [%d]: [%v]",
					senderID,
					err,
				)
				continue
			}

			expectedOperator := groupSelectionResult.OperatorsAddresses[senderID-1]
			actualOperator := signing.PublicKeyBytesToAddress(message.PublicKey)
			if actualOperator != expectedOperator {
				logger.Warnf(
					"dropping FROST DKG result signature from member [%d]; "+
						"operator address [%s] does not match selected operator [%s]",
					senderID,
					actualOperator,
					expectedOperator,
				)
				continue
			}

			if existing, ok := signatures[senderID]; ok {
				if !bytes.Equal(existing, message.Signature) {
					logger.Warnf(
						"dropping conflicting FROST DKG result signature from member [%d]",
						senderID,
					)
				}
				continue
			}

			signatures[senderID] = append([]byte{}, message.Signature...)
		}
	}

	signingMembersIndices := make([]uint64, 0, len(signatures))
	for memberIndex := range signatures {
		signingMembersIndices = append(signingMembersIndices, uint64(memberIndex))
	}
	sort.Slice(signingMembersIndices, func(i, j int) bool {
		return signingMembersIndices[i] < signingMembersIndices[j]
	})

	packedSignatures := make([]byte, 0)
	for _, signingMemberIndex := range signingMembersIndices {
		packedSignatures = append(
			packedSignatures,
			signatures[group.MemberIndex(signingMemberIndex)]...,
		)
	}

	return registry.AssembleResult(
		unsignedResult.SubmitterMemberIndex,
		unsignedResult.XOnlyOutputKey,
		unsignedResult.Members,
		unsignedResult.MisbehavedMembersIndices,
		packedSignatures,
		signingMembersIndices,
	)
}

func shouldAcceptFrostDKGResultSignatureMessage(
	message *frostDKGResultSignatureMessage,
	senderPublicKey []byte,
	sessionID string,
	selfMemberIndex group.MemberIndex,
	includedMembersSet map[group.MemberIndex]struct{},
	membershipValidator *group.MembershipValidator,
) bool {
	if message == nil {
		return false
	}

	senderID := message.SenderID()
	if senderID == 0 || senderID == selfMemberIndex {
		return false
	}
	if message.SessionID != sessionID {
		return false
	}
	if _, included := includedMembersSet[senderID]; !included {
		return false
	}
	if membershipValidator == nil {
		return true
	}

	return membershipValidator.IsValidMembership(senderID, senderPublicKey)
}
