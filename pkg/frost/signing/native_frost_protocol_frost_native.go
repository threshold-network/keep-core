//go:build frost_native

package signing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/ipfs/go-log/v2"
	"github.com/keep-network/keep-core/pkg/frost"
	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

const nativeFROSTMessageTypePrefix = "frost_signing/native_frost/"

var (
	// ErrInvalidSigningAttemptPolicy indicates the provided attempt metadata
	// violates coordinator/cohort policy invariants.
	ErrInvalidSigningAttemptPolicy = errors.New("invalid signing attempt policy")
	// ErrConsumedSigningAttemptReplay indicates signer-side replay protection
	// rejected a previously consumed signing attempt payload.
	ErrConsumedSigningAttemptReplay = errors.New("consumed signing attempt replay")
)

type nativeFROSTUniFFIV2SignerMaterial struct {
	KeyPackage       *NativeFROSTKeyPackage       `json:"keyPackage"`
	PublicKeyPackage *NativeFROSTPublicKeyPackage `json:"publicKeyPackage"`
}

func (nufv2sm *nativeFROSTUniFFIV2SignerMaterial) validate() error {
	if nufv2sm == nil {
		return fmt.Errorf("native signer material payload is nil")
	}

	if nufv2sm.KeyPackage == nil {
		return fmt.Errorf("native signer key package is nil")
	}

	if nufv2sm.KeyPackage.Identifier == "" {
		return fmt.Errorf("native signer key package identifier is empty")
	}

	if len(nufv2sm.KeyPackage.Data) == 0 {
		return fmt.Errorf("native signer key package data is empty")
	}

	if nufv2sm.PublicKeyPackage == nil {
		return fmt.Errorf("native signer public key package is nil")
	}

	if nufv2sm.PublicKeyPackage.VerifyingKey == "" {
		return fmt.Errorf("native signer public key package verifying key is empty")
	}

	return nil
}

func decodeNativeFROSTUniFFIV2SignerMaterial(
	signerMaterial *NativeSignerMaterial,
) (*nativeFROSTUniFFIV2SignerMaterial, error) {
	if signerMaterial == nil {
		return nil, fmt.Errorf(
			"%w: signer material is nil",
			ErrNativeCryptographyUnavailable,
		)
	}

	if signerMaterial.Format != NativeSignerMaterialFormatFrostUniFFIV2 {
		return nil, fmt.Errorf(
			"%w: unsupported signer material format: [%s]",
			ErrNativeCryptographyUnavailable,
			signerMaterial.Format,
		)
	}

	if len(signerMaterial.Payload) == 0 {
		return nil, fmt.Errorf(
			"%w: signer material payload is empty",
			ErrNativeCryptographyUnavailable,
		)
	}

	var decoded nativeFROSTUniFFIV2SignerMaterial
	if err := json.Unmarshal(signerMaterial.Payload, &decoded); err != nil {
		return nil, fmt.Errorf(
			"%w: cannot unmarshal native signer material payload: [%v]",
			ErrNativeCryptographyUnavailable,
			err,
		)
	}

	if err := decoded.validate(); err != nil {
		return nil, fmt.Errorf(
			"%w: invalid native signer material payload: [%v]",
			ErrNativeCryptographyUnavailable,
			err,
		)
	}

	return &decoded, nil
}

type nativeFROSTRoundOneCommitmentMessage struct {
	SenderIDValue         uint32 `json:"senderID"`
	SessionIDValue        string `json:"sessionID"`
	ParticipantIdentifier string `json:"participantIdentifier"`
	CommitmentData        []byte `json:"commitmentData"`
}

func (nfr1cm *nativeFROSTRoundOneCommitmentMessage) SenderID() group.MemberIndex {
	return group.MemberIndex(nfr1cm.SenderIDValue)
}

func (nfr1cm *nativeFROSTRoundOneCommitmentMessage) SessionID() string {
	return nfr1cm.SessionIDValue
}

func (nfr1cm *nativeFROSTRoundOneCommitmentMessage) Type() string {
	return nativeFROSTMessageTypePrefix + "round_one_commitment"
}

func (nfr1cm *nativeFROSTRoundOneCommitmentMessage) Marshal() ([]byte, error) {
	return json.Marshal(nfr1cm)
}

func (nfr1cm *nativeFROSTRoundOneCommitmentMessage) Unmarshal(data []byte) error {
	if err := json.Unmarshal(data, nfr1cm); err != nil {
		return err
	}

	if nfr1cm.SenderID() == 0 {
		return fmt.Errorf("sender ID is zero")
	}

	if nfr1cm.SessionID() == "" {
		return fmt.Errorf("session ID is empty")
	}

	if nfr1cm.ParticipantIdentifier == "" {
		return fmt.Errorf("participant identifier is empty")
	}

	if len(nfr1cm.CommitmentData) == 0 {
		return fmt.Errorf("commitment data is empty")
	}

	return nil
}

type nativeFROSTRoundTwoSignatureShareMessage struct {
	SenderIDValue         uint32 `json:"senderID"`
	SessionIDValue        string `json:"sessionID"`
	ParticipantIdentifier string `json:"participantIdentifier"`
	SignatureShareData    []byte `json:"signatureShareData"`
}

func (nfr2ssm *nativeFROSTRoundTwoSignatureShareMessage) SenderID() group.MemberIndex {
	return group.MemberIndex(nfr2ssm.SenderIDValue)
}

func (nfr2ssm *nativeFROSTRoundTwoSignatureShareMessage) SessionID() string {
	return nfr2ssm.SessionIDValue
}

func (nfr2ssm *nativeFROSTRoundTwoSignatureShareMessage) Type() string {
	return nativeFROSTMessageTypePrefix + "round_two_signature_share"
}

func (nfr2ssm *nativeFROSTRoundTwoSignatureShareMessage) Marshal() ([]byte, error) {
	return json.Marshal(nfr2ssm)
}

func (nfr2ssm *nativeFROSTRoundTwoSignatureShareMessage) Unmarshal(data []byte) error {
	if err := json.Unmarshal(data, nfr2ssm); err != nil {
		return err
	}

	if nfr2ssm.SenderID() == 0 {
		return fmt.Errorf("sender ID is zero")
	}

	if nfr2ssm.SessionID() == "" {
		return fmt.Errorf("session ID is empty")
	}

	if nfr2ssm.ParticipantIdentifier == "" {
		return fmt.Errorf("participant identifier is empty")
	}

	if len(nfr2ssm.SignatureShareData) == 0 {
		return fmt.Errorf("signature share data is empty")
	}

	return nil
}

func registerNativeFROSTSigningUnmarshallers(channel net.BroadcastChannel) {
	channel.SetUnmarshaler(func() net.TaggedUnmarshaler {
		return &nativeFROSTRoundOneCommitmentMessage{}
	})
	channel.SetUnmarshaler(func() net.TaggedUnmarshaler {
		return &nativeFROSTRoundTwoSignatureShareMessage{}
	})
}

func executeNativeFROSTSigning(
	ctx context.Context,
	logger log.StandardLogger,
	request *NativeExecutionFFISigningRequest,
	engine NativeFROSTSigningEngine,
	signerMaterial *nativeFROSTUniFFIV2SignerMaterial,
) (*frost.Signature, error) {
	if engine == nil {
		return nil, fmt.Errorf(
			"%w: native FROST signing engine is unavailable",
			ErrNativeCryptographyUnavailable,
		)
	}

	if signerMaterial == nil {
		return nil, fmt.Errorf(
			"%w: native signer material is nil",
			ErrNativeCryptographyUnavailable,
		)
	}

	if err := signerMaterial.validate(); err != nil {
		return nil, fmt.Errorf(
			"%w: invalid native signer material: [%v]",
			ErrNativeCryptographyUnavailable,
			err,
		)
	}

	includedMembersSet, includedMembersIndexes, err := includedMembersFromRequest(request)
	if err != nil {
		return nil, err
	}

	if _, ok := includedMembersSet[request.MemberIndex]; !ok {
		return nil, fmt.Errorf(
			"member [%v] not included in native FROST signing attempt",
			request.MemberIndex,
		)
	}

	messageBytes := request.Message.Bytes()
	if len(messageBytes) == 0 {
		messageBytes = []byte{0}
	}

	ownNonces, ownCommitment, err := engine.GenerateNoncesAndCommitments(
		signerMaterial.KeyPackage,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"native FROST round one generation failed: [%w]",
			err,
		)
	}

	if ownCommitment == nil {
		return nil, fmt.Errorf("native FROST round one returned nil commitment")
	}

	if ownCommitment.Identifier == "" {
		return nil, fmt.Errorf("native FROST round one commitment identifier is empty")
	}

	if len(ownCommitment.Data) == 0 {
		return nil, fmt.Errorf("native FROST round one commitment data is empty")
	}

	if ownNonces == nil {
		return nil, fmt.Errorf("native FROST round one returned nil nonces")
	}

	roundOneMessage := &nativeFROSTRoundOneCommitmentMessage{
		SenderIDValue:         uint32(request.MemberIndex),
		SessionIDValue:        request.SessionID,
		ParticipantIdentifier: ownCommitment.Identifier,
		CommitmentData:        append([]byte{}, ownCommitment.Data...),
	}

	if err := request.Channel.Send(
		ctx,
		roundOneMessage,
		net.BackoffRetransmissionStrategy,
	); err != nil {
		return nil, fmt.Errorf("cannot send native FROST round one message: [%w]", err)
	}

	roundOneMessages, err := collectNativeFROSTRoundOneMessages(
		ctx,
		request,
		includedMembersSet,
		includedMembersIndexes,
	)
	if err != nil {
		return nil, err
	}

	commitmentsBySender := map[group.MemberIndex]*NativeFROSTCommitment{
		request.MemberIndex: ownCommitment,
	}

	for senderID, message := range roundOneMessages {
		commitmentsBySender[senderID] = &NativeFROSTCommitment{
			Identifier: message.ParticipantIdentifier,
			Data:       append([]byte{}, message.CommitmentData...),
		}
	}

	orderedCommitments := make([]*NativeFROSTCommitment, 0, len(includedMembersIndexes))
	for _, memberIndex := range includedMembersIndexes {
		orderedCommitments = append(
			orderedCommitments,
			commitmentsBySender[memberIndex],
		)
	}

	signingPackage, err := engine.NewSigningPackage(
		messageBytes,
		orderedCommitments,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"native FROST signing package creation failed: [%w]",
			err,
		)
	}

	if signingPackage == nil {
		return nil, fmt.Errorf("native FROST signing package is nil")
	}

	ownSignatureShare, err := engine.Sign(
		signingPackage,
		ownNonces,
		signerMaterial.KeyPackage,
	)
	if err != nil {
		return nil, fmt.Errorf("native FROST round two signing failed: [%w]", err)
	}

	if ownSignatureShare == nil {
		return nil, fmt.Errorf("native FROST round two returned nil signature share")
	}

	if ownSignatureShare.Identifier == "" {
		return nil, fmt.Errorf("native FROST signature share identifier is empty")
	}

	if len(ownSignatureShare.Data) == 0 {
		return nil, fmt.Errorf("native FROST signature share data is empty")
	}

	roundTwoMessage := &nativeFROSTRoundTwoSignatureShareMessage{
		SenderIDValue:         uint32(request.MemberIndex),
		SessionIDValue:        request.SessionID,
		ParticipantIdentifier: ownSignatureShare.Identifier,
		SignatureShareData:    append([]byte{}, ownSignatureShare.Data...),
	}

	if err := request.Channel.Send(
		ctx,
		roundTwoMessage,
		net.BackoffRetransmissionStrategy,
	); err != nil {
		return nil, fmt.Errorf("cannot send native FROST round two message: [%w]", err)
	}

	roundTwoMessages, err := collectNativeFROSTRoundTwoMessages(
		ctx,
		request,
		includedMembersSet,
		includedMembersIndexes,
	)
	if err != nil {
		return nil, err
	}

	signatureSharesBySender := map[group.MemberIndex]*NativeFROSTSignatureShare{
		request.MemberIndex: ownSignatureShare,
	}

	for senderID, message := range roundTwoMessages {
		signatureSharesBySender[senderID] = &NativeFROSTSignatureShare{
			Identifier: message.ParticipantIdentifier,
			Data:       append([]byte{}, message.SignatureShareData...),
		}
	}

	orderedSignatureShares := make([]*NativeFROSTSignatureShare, 0, len(includedMembersIndexes))
	for _, memberIndex := range includedMembersIndexes {
		orderedSignatureShares = append(
			orderedSignatureShares,
			signatureSharesBySender[memberIndex],
		)
	}

	signatureBytes, err := engine.Aggregate(
		signingPackage,
		orderedSignatureShares,
		signerMaterial.PublicKeyPackage,
	)
	if err != nil {
		return nil, fmt.Errorf("native FROST aggregation failed: [%w]", err)
	}

	signature := &frost.Signature{}
	if err := signature.Unmarshal(signatureBytes); err != nil {
		return nil, fmt.Errorf(
			"native FROST aggregation returned invalid signature: [%w]",
			err,
		)
	}

	if logger != nil {
		logger.Debugf(
			"[member:%v] native FROST signing completed with [%v] participants",
			request.MemberIndex,
			len(includedMembersIndexes),
		)
	}

	return signature, nil
}

func includedMembersFromRequest(
	request *NativeExecutionFFISigningRequest,
) (map[group.MemberIndex]struct{}, []group.MemberIndex, error) {
	if request == nil {
		return nil, nil, fmt.Errorf("request is nil")
	}

	if request.GroupSize <= 0 {
		return nil, nil, fmt.Errorf("group size must be positive")
	}

	attempt := request.Attempt
	if attempt != nil {
		if attempt.Number == 0 {
			return nil, nil, fmt.Errorf(
				"%w: attempt number is zero",
				ErrInvalidSigningAttemptPolicy,
			)
		}

		if attempt.CoordinatorMemberIndex == 0 {
			return nil, nil, fmt.Errorf(
				"%w: attempt coordinator member index is zero",
				ErrInvalidSigningAttemptPolicy,
			)
		}
	}

	includedMembersSet := make(map[group.MemberIndex]struct{})
	excludedMembersSet := make(map[group.MemberIndex]struct{})

	if attempt != nil {
		for _, memberIndex := range attempt.ExcludedMembersIndexes {
			if memberIndex == 0 {
				continue
			}

			excludedMembersSet[memberIndex] = struct{}{}
		}
	}

	if attempt != nil && len(attempt.IncludedMembersIndexes) > 0 {
		for _, memberIndex := range attempt.IncludedMembersIndexes {
			if memberIndex == 0 {
				return nil, nil, fmt.Errorf(
					"%w: included member index is zero",
					ErrInvalidSigningAttemptPolicy,
				)
			}

			if _, excluded := excludedMembersSet[memberIndex]; excluded {
				return nil, nil, fmt.Errorf(
					"%w: member [%v] is both included and excluded in attempt",
					ErrInvalidSigningAttemptPolicy,
					memberIndex,
				)
			}

			includedMembersSet[memberIndex] = struct{}{}
		}
	} else {
		for i := 1; i <= request.GroupSize; i++ {
			memberIndex := group.MemberIndex(i)
			if _, excluded := excludedMembersSet[memberIndex]; !excluded {
				includedMembersSet[memberIndex] = struct{}{}
			}
		}
	}

	if len(includedMembersSet) == 0 {
		if attempt != nil {
			return nil, nil, fmt.Errorf(
				"%w: included members set is empty",
				ErrInvalidSigningAttemptPolicy,
			)
		}

		return nil, nil, fmt.Errorf("included members set is empty")
	}

	if attempt != nil {
		if _, included := includedMembersSet[attempt.CoordinatorMemberIndex]; !included {
			return nil, nil, fmt.Errorf(
				"%w: attempt coordinator [%v] is not included",
				ErrInvalidSigningAttemptPolicy,
				attempt.CoordinatorMemberIndex,
			)
		}
	}

	includedMembersIndexes := make([]group.MemberIndex, 0, len(includedMembersSet))
	for memberIndex := range includedMembersSet {
		includedMembersIndexes = append(includedMembersIndexes, memberIndex)
	}

	sort.Slice(includedMembersIndexes, func(i, j int) bool {
		return includedMembersIndexes[i] < includedMembersIndexes[j]
	})

	return includedMembersSet, includedMembersIndexes, nil
}

func collectNativeFROSTRoundOneMessages(
	ctx context.Context,
	request *NativeExecutionFFISigningRequest,
	includedMembersSet map[group.MemberIndex]struct{},
	includedMembersIndexes []group.MemberIndex,
) (map[group.MemberIndex]*nativeFROSTRoundOneCommitmentMessage, error) {
	expectedMessagesCount := len(includedMembersIndexes) - 1
	if expectedMessagesCount <= 0 {
		return map[group.MemberIndex]*nativeFROSTRoundOneCommitmentMessage{}, nil
	}

	recvCtx, cancelRecvCtx := context.WithCancel(ctx)
	defer cancelRecvCtx()

	messageChan := make(chan *nativeFROSTRoundOneCommitmentMessage, expectedMessagesCount*4+1)

	request.Channel.Recv(recvCtx, func(message net.Message) {
		payload, ok := message.Payload().(*nativeFROSTRoundOneCommitmentMessage)
		if !ok {
			return
		}

		if !shouldAcceptNativeFROSTMessage(
			request,
			includedMembersSet,
			payload.SenderID(),
			payload.SessionID(),
			message.SenderPublicKey(),
		) {
			return
		}

		select {
		case messageChan <- payload:
		default:
		}
	})

	receivedMessages := make(map[group.MemberIndex]*nativeFROSTRoundOneCommitmentMessage)

	for len(receivedMessages) < expectedMessagesCount {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf(
				"native FROST round one collection interrupted: [%w]",
				ctx.Err(),
			)

		case message := <-messageChan:
			receivedMessages[message.SenderID()] = message
		}
	}

	return receivedMessages, nil
}

func collectNativeFROSTRoundTwoMessages(
	ctx context.Context,
	request *NativeExecutionFFISigningRequest,
	includedMembersSet map[group.MemberIndex]struct{},
	includedMembersIndexes []group.MemberIndex,
) (map[group.MemberIndex]*nativeFROSTRoundTwoSignatureShareMessage, error) {
	expectedMessagesCount := len(includedMembersIndexes) - 1
	if expectedMessagesCount <= 0 {
		return map[group.MemberIndex]*nativeFROSTRoundTwoSignatureShareMessage{}, nil
	}

	recvCtx, cancelRecvCtx := context.WithCancel(ctx)
	defer cancelRecvCtx()

	messageChan := make(chan *nativeFROSTRoundTwoSignatureShareMessage, expectedMessagesCount*4+1)

	request.Channel.Recv(recvCtx, func(message net.Message) {
		payload, ok := message.Payload().(*nativeFROSTRoundTwoSignatureShareMessage)
		if !ok {
			return
		}

		if !shouldAcceptNativeFROSTMessage(
			request,
			includedMembersSet,
			payload.SenderID(),
			payload.SessionID(),
			message.SenderPublicKey(),
		) {
			return
		}

		select {
		case messageChan <- payload:
		default:
		}
	})

	receivedMessages := make(map[group.MemberIndex]*nativeFROSTRoundTwoSignatureShareMessage)

	for len(receivedMessages) < expectedMessagesCount {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf(
				"native FROST round two collection interrupted: [%w]",
				ctx.Err(),
			)

		case message := <-messageChan:
			receivedMessages[message.SenderID()] = message
		}
	}

	return receivedMessages, nil
}

func shouldAcceptNativeFROSTMessage(
	request *NativeExecutionFFISigningRequest,
	includedMembersSet map[group.MemberIndex]struct{},
	senderID group.MemberIndex,
	sessionID string,
	senderPublicKey []byte,
) bool {
	if senderID == 0 {
		return false
	}

	if senderID == request.MemberIndex {
		return false
	}

	if sessionID != request.SessionID {
		return false
	}

	if _, included := includedMembersSet[senderID]; !included {
		return false
	}

	if request.MembershipValidator == nil {
		return true
	}

	return request.MembershipValidator.IsValidMembership(senderID, senderPublicKey)
}
