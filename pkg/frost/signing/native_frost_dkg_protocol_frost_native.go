//go:build frost_native

package signing

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/ipfs/go-log/v2"
	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

const nativeFROSTDKGMessageTypePrefix = "frost_dkg/native_frost/"

// NativeFROSTDKGRequest contains the local participant context needed to run
// the native FROST DKG protocol over a keep-core broadcast channel.
type NativeFROSTDKGRequest struct {
	MemberIndex            group.MemberIndex
	GroupSize              int
	Threshold              int
	SessionID              string
	IncludedMembersIndexes []group.MemberIndex
	Channel                net.BroadcastChannel
	MembershipValidator    *group.MembershipValidator
}

type nativeFROSTDKGRoundOnePackageMessage struct {
	SenderIDValue         uint32 `json:"senderID"`
	SessionIDValue        string `json:"sessionID"`
	ParticipantIdentifier string `json:"participantIdentifier"`
	PackageData           []byte `json:"packageData"`
}

func (nfdkgropm *nativeFROSTDKGRoundOnePackageMessage) SenderID() group.MemberIndex {
	return group.MemberIndex(nfdkgropm.SenderIDValue)
}

func (nfdkgropm *nativeFROSTDKGRoundOnePackageMessage) SessionID() string {
	return nfdkgropm.SessionIDValue
}

func (nfdkgropm *nativeFROSTDKGRoundOnePackageMessage) Type() string {
	return nativeFROSTDKGMessageTypePrefix + "round_one_package"
}

func (nfdkgropm *nativeFROSTDKGRoundOnePackageMessage) Marshal() ([]byte, error) {
	return json.Marshal(nfdkgropm)
}

func (nfdkgropm *nativeFROSTDKGRoundOnePackageMessage) Unmarshal(data []byte) error {
	if err := json.Unmarshal(data, nfdkgropm); err != nil {
		return err
	}

	if nfdkgropm.SenderID() == 0 {
		return fmt.Errorf("sender ID is zero")
	}
	if nfdkgropm.SessionID() == "" {
		return fmt.Errorf("session ID is empty")
	}
	if nfdkgropm.ParticipantIdentifier == "" {
		return fmt.Errorf("participant identifier is empty")
	}
	if len(nfdkgropm.PackageData) == 0 {
		return fmt.Errorf("round-one package data is empty")
	}

	return nil
}

type nativeFROSTDKGRoundTwoPackageMessage struct {
	SenderIDValue               uint32                           `json:"senderID"`
	SessionIDValue              string                           `json:"sessionID"`
	SenderParticipantIdentifier string                           `json:"senderParticipantIdentifier"`
	Packages                    []*nativeFROSTDKGRoundTwoPackage `json:"packages"`
}

type nativeFROSTDKGRoundTwoPackage struct {
	ReceiverIDValue              uint32 `json:"receiverID"`
	PackageParticipantIdentifier string `json:"packageParticipantIdentifier"`
	PackageData                  []byte `json:"packageData"`
}

func (nfdkgtrpm *nativeFROSTDKGRoundTwoPackageMessage) SenderID() group.MemberIndex {
	return group.MemberIndex(nfdkgtrpm.SenderIDValue)
}

func (nfdkgtrpm *nativeFROSTDKGRoundTwoPackageMessage) SessionID() string {
	return nfdkgtrpm.SessionIDValue
}

func (nfdkgtrpm *nativeFROSTDKGRoundTwoPackageMessage) Type() string {
	return nativeFROSTDKGMessageTypePrefix + "round_two_package"
}

func (nfdkgtrpm *nativeFROSTDKGRoundTwoPackageMessage) Marshal() ([]byte, error) {
	return json.Marshal(nfdkgtrpm)
}

func (nfdkgtrpm *nativeFROSTDKGRoundTwoPackageMessage) Unmarshal(data []byte) error {
	if err := json.Unmarshal(data, nfdkgtrpm); err != nil {
		return err
	}

	if nfdkgtrpm.SenderID() == 0 {
		return fmt.Errorf("sender ID is zero")
	}
	if nfdkgtrpm.SessionID() == "" {
		return fmt.Errorf("session ID is empty")
	}
	if nfdkgtrpm.SenderParticipantIdentifier == "" {
		return fmt.Errorf("sender participant identifier is empty")
	}
	if len(nfdkgtrpm.Packages) == 0 {
		return fmt.Errorf("round-two packages are empty")
	}
	for i, pkg := range nfdkgtrpm.Packages {
		if pkg == nil {
			return fmt.Errorf("round-two package [%d] is nil", i)
		}
		if pkg.ReceiverID() == 0 {
			return fmt.Errorf("round-two package [%d] receiver ID is zero", i)
		}
		if pkg.PackageParticipantIdentifier == "" {
			return fmt.Errorf(
				"round-two package [%d] participant identifier is empty",
				i,
			)
		}
		if len(pkg.PackageData) == 0 {
			return fmt.Errorf("round-two package [%d] data is empty", i)
		}
	}

	return nil
}

func (nfdkgtrp *nativeFROSTDKGRoundTwoPackage) ReceiverID() group.MemberIndex {
	return group.MemberIndex(nfdkgtrp.ReceiverIDValue)
}

// RegisterNativeFROSTDKGUnmarshallers registers native FROST DKG protocol
// messages on the given broadcast channel.
func RegisterNativeFROSTDKGUnmarshallers(channel net.BroadcastChannel) {
	channel.SetUnmarshaler(func() net.TaggedUnmarshaler {
		return &nativeFROSTDKGRoundOnePackageMessage{}
	})
	channel.SetUnmarshaler(func() net.TaggedUnmarshaler {
		return &nativeFROSTDKGRoundTwoPackageMessage{}
	})
}

// NativeFROSTParticipantIdentifierForMemberIndex returns the participant
// identifier string expected by the UniFFI FROST SDK for a keep-core 1-based
// group member index.
func NativeFROSTParticipantIdentifierForMemberIndex(
	memberIndex group.MemberIndex,
) (string, error) {
	if memberIndex == 0 {
		return "", fmt.Errorf("member index is zero")
	}
	if memberIndex > group.MaxMemberIndex {
		return "", fmt.Errorf("member index [%v] exceeds maximum", memberIndex)
	}

	// frost-core serializes Identifier::try_from(n) as a 32-byte little-endian
	// scalar hex string wrapped as JSON. DKG group sizes are bounded to uint8
	// indexes in keep-core, so setting the first byte is sufficient.
	var identifier [32]byte
	identifier[0] = byte(memberIndex)

	return strconv.Quote(hex.EncodeToString(identifier[:])), nil
}

// ExecuteNativeFROSTDKG executes the three native FROST DKG rounds. The caller
// is responsible for scoping ctx to the on-chain submission timeout.
func ExecuteNativeFROSTDKG(
	ctx context.Context,
	logger log.StandardLogger,
	request *NativeFROSTDKGRequest,
	engine NativeFROSTDKGEngine,
) (*NativeFROSTDKGResult, error) {
	if engine == nil {
		return nil, fmt.Errorf(
			"%w: native FROST DKG engine is unavailable",
			ErrNativeCryptographyUnavailable,
		)
	}

	includedMembersSet, includedMembersIndexes, err :=
		includedMembersFromDKGRequest(request)
	if err != nil {
		return nil, err
	}

	identifiersByMemberIndex, memberIndexesByIdentifier, err :=
		nativeFROSTDKGParticipantIdentifiers(includedMembersIndexes)
	if err != nil {
		return nil, err
	}

	ownIdentifier := identifiersByMemberIndex[request.MemberIndex]
	part1, err := engine.Part1(
		ownIdentifier,
		uint16(len(includedMembersIndexes)),
		uint16(request.Threshold),
	)
	if err != nil {
		return nil, fmt.Errorf("native FROST DKG part one failed: [%w]", err)
	}
	if err := validateNativeFROSTDKGPart1Result(part1); err != nil {
		return nil, err
	}

	roundOneMessage := &nativeFROSTDKGRoundOnePackageMessage{
		SenderIDValue:         uint32(request.MemberIndex),
		SessionIDValue:        request.SessionID,
		ParticipantIdentifier: part1.Package.Identifier,
		PackageData:           append([]byte{}, part1.Package.Data...),
	}
	if err := request.Channel.Send(
		ctx,
		roundOneMessage,
		net.BackoffRetransmissionStrategy,
	); err != nil {
		return nil, fmt.Errorf("cannot send native FROST DKG round-one package: [%w]", err)
	}

	roundOneMessages, err := collectNativeFROSTDKGRoundOnePackageMessages(
		ctx,
		request,
		includedMembersSet,
		includedMembersIndexes,
		identifiersByMemberIndex,
	)
	if err != nil {
		return nil, err
	}

	roundOnePackages := make(
		[]*NativeFROSTDKGRound1Package,
		0,
		len(roundOneMessages),
	)
	for _, memberIndex := range includedMembersIndexes {
		if memberIndex == request.MemberIndex {
			continue
		}

		message := roundOneMessages[memberIndex]
		roundOnePackages = append(roundOnePackages, &NativeFROSTDKGRound1Package{
			Identifier: message.ParticipantIdentifier,
			Data:       append([]byte{}, message.PackageData...),
		})
	}

	part2, err := engine.Part2(part1.SecretPackage, roundOnePackages)
	if err != nil {
		return nil, fmt.Errorf("native FROST DKG part two failed: [%w]", err)
	}
	if err := validateNativeFROSTDKGPart2Result(part2); err != nil {
		return nil, err
	}

	roundTwoPackagesMessage := &nativeFROSTDKGRoundTwoPackageMessage{
		SenderIDValue:               uint32(request.MemberIndex),
		SessionIDValue:              request.SessionID,
		SenderParticipantIdentifier: ownIdentifier,
		Packages: make(
			[]*nativeFROSTDKGRoundTwoPackage,
			0,
			len(part2.Packages),
		),
	}
	for _, pkg := range part2.Packages {
		receiverID, ok := memberIndexesByIdentifier[pkg.Identifier]
		if !ok {
			return nil, fmt.Errorf(
				"native FROST DKG part two produced package for unknown identifier [%s]",
				pkg.Identifier,
			)
		}
		if receiverID == request.MemberIndex {
			return nil, fmt.Errorf(
				"native FROST DKG part two produced package for self",
			)
		}

		roundTwoPackagesMessage.Packages = append(
			roundTwoPackagesMessage.Packages,
			&nativeFROSTDKGRoundTwoPackage{
				ReceiverIDValue:              uint32(receiverID),
				PackageParticipantIdentifier: pkg.Identifier,
				PackageData:                  append([]byte{}, pkg.Data...),
			},
		)
	}

	sort.Slice(
		roundTwoPackagesMessage.Packages,
		func(i, j int) bool {
			return roundTwoPackagesMessage.Packages[i].ReceiverID() <
				roundTwoPackagesMessage.Packages[j].ReceiverID()
		},
	)
	if err := request.Channel.Send(
		ctx,
		roundTwoPackagesMessage,
		net.BackoffRetransmissionStrategy,
	); err != nil {
		return nil, fmt.Errorf("cannot send native FROST DKG round-two packages: [%w]", err)
	}

	roundTwoMessages, err := collectNativeFROSTDKGRoundTwoPackageMessages(
		ctx,
		request,
		includedMembersSet,
		includedMembersIndexes,
		identifiersByMemberIndex,
	)
	if err != nil {
		return nil, err
	}

	roundTwoPackages := make(
		[]*NativeFROSTDKGRound2Package,
		0,
		len(roundTwoMessages),
	)
	for _, memberIndex := range includedMembersIndexes {
		if memberIndex == request.MemberIndex {
			continue
		}

		message := roundTwoMessages[memberIndex]
		roundTwoPackage, err := nativeFROSTDKGRoundTwoPackageForReceiver(
			message,
			request.MemberIndex,
		)
		if err != nil {
			return nil, err
		}

		roundTwoPackages = append(roundTwoPackages, &NativeFROSTDKGRound2Package{
			Identifier:       roundTwoPackage.PackageParticipantIdentifier,
			SenderIdentifier: message.SenderParticipantIdentifier,
			Data:             append([]byte{}, roundTwoPackage.PackageData...),
		})
	}

	result, err := engine.Part3(part2.SecretPackage, roundOnePackages, roundTwoPackages)
	if err != nil {
		return nil, fmt.Errorf("native FROST DKG part three failed: [%w]", err)
	}
	if err := validateNativeFROSTDKGResult(result); err != nil {
		return nil, err
	}

	if logger != nil {
		logger.Debugf(
			"[member:%v] native FROST DKG completed with [%v] participants",
			request.MemberIndex,
			len(includedMembersIndexes),
		)
	}

	return result, nil
}

func includedMembersFromDKGRequest(
	request *NativeFROSTDKGRequest,
) (map[group.MemberIndex]struct{}, []group.MemberIndex, error) {
	if request == nil {
		return nil, nil, fmt.Errorf("request is nil")
	}
	if request.MemberIndex == 0 {
		return nil, nil, fmt.Errorf("member index is zero")
	}
	if request.GroupSize <= 0 {
		return nil, nil, fmt.Errorf("group size must be positive")
	}
	if request.Threshold <= 0 {
		return nil, nil, fmt.Errorf("threshold must be positive")
	}
	if request.SessionID == "" {
		return nil, nil, fmt.Errorf("session ID is empty")
	}
	if request.Channel == nil {
		return nil, nil, fmt.Errorf("broadcast channel is nil")
	}
	if request.MembershipValidator == nil {
		return nil, nil, fmt.Errorf("membership validator is nil")
	}
	if request.GroupSize > int(group.MaxMemberIndex) {
		return nil, nil, fmt.Errorf("group size [%d] exceeds maximum", request.GroupSize)
	}

	includedMembersSet := make(map[group.MemberIndex]struct{})
	if len(request.IncludedMembersIndexes) > 0 {
		for _, memberIndex := range request.IncludedMembersIndexes {
			if memberIndex == 0 || int(memberIndex) > request.GroupSize {
				return nil, nil, fmt.Errorf(
					"included member index [%v] out of range [1, %d]",
					memberIndex,
					request.GroupSize,
				)
			}

			includedMembersSet[memberIndex] = struct{}{}
		}
	} else {
		for i := 1; i <= request.GroupSize; i++ {
			includedMembersSet[group.MemberIndex(i)] = struct{}{}
		}
	}

	if _, ok := includedMembersSet[request.MemberIndex]; !ok {
		return nil, nil, fmt.Errorf(
			"member [%v] not included in native FROST DKG attempt",
			request.MemberIndex,
		)
	}
	if len(includedMembersSet) < request.Threshold {
		return nil, nil, fmt.Errorf(
			"included members count [%d] is below threshold [%d]",
			len(includedMembersSet),
			request.Threshold,
		)
	}
	if len(includedMembersSet) > int(^uint16(0)) ||
		request.Threshold > int(^uint16(0)) {
		return nil, nil, fmt.Errorf("native FROST DKG parameters exceed uint16")
	}

	includedMembersIndexes := make(
		[]group.MemberIndex,
		0,
		len(includedMembersSet),
	)
	for memberIndex := range includedMembersSet {
		includedMembersIndexes = append(includedMembersIndexes, memberIndex)
	}
	sort.Slice(includedMembersIndexes, func(i, j int) bool {
		return includedMembersIndexes[i] < includedMembersIndexes[j]
	})

	return includedMembersSet, includedMembersIndexes, nil
}

func nativeFROSTDKGParticipantIdentifiers(
	memberIndexes []group.MemberIndex,
) (
	map[group.MemberIndex]string,
	map[string]group.MemberIndex,
	error,
) {
	identifiersByMemberIndex := make(map[group.MemberIndex]string, len(memberIndexes))
	memberIndexesByIdentifier := make(map[string]group.MemberIndex, len(memberIndexes))

	for _, memberIndex := range memberIndexes {
		identifier, err := NativeFROSTParticipantIdentifierForMemberIndex(memberIndex)
		if err != nil {
			return nil, nil, err
		}

		identifiersByMemberIndex[memberIndex] = identifier
		memberIndexesByIdentifier[identifier] = memberIndex
	}

	return identifiersByMemberIndex, memberIndexesByIdentifier, nil
}

func collectNativeFROSTDKGRoundOnePackageMessages(
	ctx context.Context,
	request *NativeFROSTDKGRequest,
	includedMembersSet map[group.MemberIndex]struct{},
	includedMembersIndexes []group.MemberIndex,
	identifiersByMemberIndex map[group.MemberIndex]string,
) (map[group.MemberIndex]*nativeFROSTDKGRoundOnePackageMessage, error) {
	expectedMessagesCount := len(includedMembersIndexes) - 1
	if expectedMessagesCount <= 0 {
		return map[group.MemberIndex]*nativeFROSTDKGRoundOnePackageMessage{}, nil
	}

	recvCtx, cancelRecvCtx := context.WithCancel(ctx)
	defer cancelRecvCtx()

	messageChan := make(chan *nativeFROSTDKGRoundOnePackageMessage, expectedMessagesCount*4+1)
	request.Channel.Recv(recvCtx, func(message net.Message) {
		payload, ok := message.Payload().(*nativeFROSTDKGRoundOnePackageMessage)
		if !ok {
			return
		}

		if !shouldAcceptNativeFROSTDKGMessage(
			request,
			includedMembersSet,
			payload.SenderID(),
			payload.SessionID(),
			message.SenderPublicKey(),
		) {
			return
		}
		if err := validateNativeFROSTDKGParticipantIdentifier(
			identifiersByMemberIndex,
			payload.SenderID(),
			payload.ParticipantIdentifier,
		); err != nil {
			protocolLogger.Warnf(
				"dropping native FROST DKG round-one package from sender [%d]: [%v]",
				payload.SenderID(),
				err,
			)
			return
		}

		select {
		case messageChan <- payload:
		default:
			protocolLogger.Warnf(
				"dropping native FROST DKG round-one package from sender [%d]; collector buffer full",
				payload.SenderID(),
			)
		}
	})

	receivedMessages := make(map[group.MemberIndex]*nativeFROSTDKGRoundOnePackageMessage)
	for len(receivedMessages) < expectedMessagesCount {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf(
				"native FROST DKG round-one collection interrupted: [%w]",
				ctx.Err(),
			)
		case message := <-messageChan:
			senderID := message.SenderID()
			if existing, ok := receivedMessages[senderID]; ok {
				if !nativeFROSTDKGRoundOnePackageMessagesEqual(existing, message) {
					protocolLogger.Warnf(
						"dropping conflicting native FROST DKG round-one package from sender [%d]",
						senderID,
					)
				}
				continue
			}
			receivedMessages[senderID] = message
		}
	}

	return receivedMessages, nil
}

func collectNativeFROSTDKGRoundTwoPackageMessages(
	ctx context.Context,
	request *NativeFROSTDKGRequest,
	includedMembersSet map[group.MemberIndex]struct{},
	includedMembersIndexes []group.MemberIndex,
	identifiersByMemberIndex map[group.MemberIndex]string,
) (map[group.MemberIndex]*nativeFROSTDKGRoundTwoPackageMessage, error) {
	expectedMessagesCount := len(includedMembersIndexes) - 1
	if expectedMessagesCount <= 0 {
		return map[group.MemberIndex]*nativeFROSTDKGRoundTwoPackageMessage{}, nil
	}

	recvCtx, cancelRecvCtx := context.WithCancel(ctx)
	defer cancelRecvCtx()

	messageChan := make(chan *nativeFROSTDKGRoundTwoPackageMessage, expectedMessagesCount*4+1)
	request.Channel.Recv(recvCtx, func(message net.Message) {
		payload, ok := message.Payload().(*nativeFROSTDKGRoundTwoPackageMessage)
		if !ok {
			return
		}

		if !shouldAcceptNativeFROSTDKGMessage(
			request,
			includedMembersSet,
			payload.SenderID(),
			payload.SessionID(),
			message.SenderPublicKey(),
		) {
			return
		}
		if err := validateNativeFROSTDKGParticipantIdentifier(
			identifiersByMemberIndex,
			payload.SenderID(),
			payload.SenderParticipantIdentifier,
		); err != nil {
			protocolLogger.Warnf(
				"dropping native FROST DKG round-two package from sender [%d]: [%v]",
				payload.SenderID(),
				err,
			)
			return
		}
		receiverPackage, err := nativeFROSTDKGRoundTwoPackageForReceiver(
			payload,
			request.MemberIndex,
		)
		if err != nil {
			protocolLogger.Warnf(
				"dropping native FROST DKG round-two packages from sender [%d] for receiver [%d]: [%v]",
				payload.SenderID(),
				request.MemberIndex,
				err,
			)
			return
		}
		if err := validateNativeFROSTDKGParticipantIdentifier(
			identifiersByMemberIndex,
			request.MemberIndex,
			receiverPackage.PackageParticipantIdentifier,
		); err != nil {
			protocolLogger.Warnf(
				"dropping native FROST DKG round-two package from sender [%d] for receiver [%d]: [%v]",
				payload.SenderID(),
				request.MemberIndex,
				err,
			)
			return
		}

		select {
		case messageChan <- payload:
		default:
			protocolLogger.Warnf(
				"dropping native FROST DKG round-two package from sender [%d]; collector buffer full",
				payload.SenderID(),
			)
		}
	})

	receivedMessages := make(map[group.MemberIndex]*nativeFROSTDKGRoundTwoPackageMessage)
	for len(receivedMessages) < expectedMessagesCount {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf(
				"native FROST DKG round-two collection interrupted: [%w]",
				ctx.Err(),
			)
		case message := <-messageChan:
			senderID := message.SenderID()
			if existing, ok := receivedMessages[senderID]; ok {
				if !nativeFROSTDKGRoundTwoPackageMessagesEqual(existing, message) {
					protocolLogger.Warnf(
						"dropping conflicting native FROST DKG round-two package from sender [%d]",
						senderID,
					)
				}
				continue
			}
			receivedMessages[senderID] = message
		}
	}

	return receivedMessages, nil
}

func shouldAcceptNativeFROSTDKGMessage(
	request *NativeFROSTDKGRequest,
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
		return false
	}

	return request.MembershipValidator.IsValidMembership(senderID, senderPublicKey)
}

func validateNativeFROSTDKGParticipantIdentifier(
	identifiersByMemberIndex map[group.MemberIndex]string,
	memberIndex group.MemberIndex,
	participantIdentifier string,
) error {
	expectedIdentifier, ok := identifiersByMemberIndex[memberIndex]
	if !ok {
		return fmt.Errorf("no expected participant identifier for member [%d]", memberIndex)
	}
	if participantIdentifier != expectedIdentifier {
		return fmt.Errorf(
			"participant identifier mismatch for member [%d]: expected [%s], got [%s]",
			memberIndex,
			expectedIdentifier,
			participantIdentifier,
		)
	}

	return nil
}

func nativeFROSTDKGRoundTwoPackageForReceiver(
	message *nativeFROSTDKGRoundTwoPackageMessage,
	receiverID group.MemberIndex,
) (*nativeFROSTDKGRoundTwoPackage, error) {
	if message == nil {
		return nil, fmt.Errorf("round-two package message is nil")
	}

	var receiverPackage *nativeFROSTDKGRoundTwoPackage
	for _, pkg := range message.Packages {
		if pkg == nil || pkg.ReceiverID() != receiverID {
			continue
		}

		if receiverPackage != nil {
			return nil, fmt.Errorf(
				"multiple round-two packages for receiver [%d]",
				receiverID,
			)
		}

		receiverPackage = pkg
	}

	if receiverPackage == nil {
		return nil, fmt.Errorf(
			"no round-two package for receiver [%d]",
			receiverID,
		)
	}

	return receiverPackage, nil
}

func nativeFROSTDKGRoundOnePackageMessagesEqual(
	left, right *nativeFROSTDKGRoundOnePackageMessage,
) bool {
	if left == nil || right == nil {
		return left == right
	}

	return left.SenderIDValue == right.SenderIDValue &&
		left.SessionIDValue == right.SessionIDValue &&
		left.ParticipantIdentifier == right.ParticipantIdentifier &&
		bytes.Equal(left.PackageData, right.PackageData)
}

func nativeFROSTDKGRoundTwoPackageMessagesEqual(
	left, right *nativeFROSTDKGRoundTwoPackageMessage,
) bool {
	if left == nil || right == nil {
		return left == right
	}

	return left.SenderIDValue == right.SenderIDValue &&
		left.SessionIDValue == right.SessionIDValue &&
		left.SenderParticipantIdentifier == right.SenderParticipantIdentifier &&
		nativeFROSTDKGRoundTwoPackagesEqual(left.Packages, right.Packages)
}

func nativeFROSTDKGRoundTwoPackagesEqual(
	left, right []*nativeFROSTDKGRoundTwoPackage,
) bool {
	if len(left) != len(right) {
		return false
	}

	for i := range left {
		if left[i] == nil || right[i] == nil {
			if left[i] != right[i] {
				return false
			}
			continue
		}

		if left[i].ReceiverIDValue != right[i].ReceiverIDValue ||
			left[i].PackageParticipantIdentifier !=
				right[i].PackageParticipantIdentifier ||
			!bytes.Equal(left[i].PackageData, right[i].PackageData) {
			return false
		}
	}

	return true
}

func validateNativeFROSTDKGPart1Result(result *NativeFROSTDKGPart1Result) error {
	if result == nil {
		return fmt.Errorf("native FROST DKG part one result is nil")
	}
	if result.SecretPackage == nil {
		return fmt.Errorf("native FROST DKG part one secret package is nil")
	}
	if len(result.SecretPackage.Data) == 0 {
		return fmt.Errorf("native FROST DKG part one secret package data is empty")
	}
	if result.Package == nil {
		return fmt.Errorf("native FROST DKG part one package is nil")
	}
	if result.Package.Identifier == "" {
		return fmt.Errorf("native FROST DKG part one package identifier is empty")
	}
	if len(result.Package.Data) == 0 {
		return fmt.Errorf("native FROST DKG part one package data is empty")
	}

	return nil
}

func validateNativeFROSTDKGPart2Result(result *NativeFROSTDKGPart2Result) error {
	if result == nil {
		return fmt.Errorf("native FROST DKG part two result is nil")
	}
	if result.SecretPackage == nil {
		return fmt.Errorf("native FROST DKG part two secret package is nil")
	}
	if len(result.SecretPackage.Data) == 0 {
		return fmt.Errorf("native FROST DKG part two secret package data is empty")
	}
	if len(result.Packages) == 0 {
		return fmt.Errorf("native FROST DKG part two packages are empty")
	}
	for i, pkg := range result.Packages {
		if pkg == nil {
			return fmt.Errorf("native FROST DKG part two package [%d] is nil", i)
		}
		if pkg.Identifier == "" {
			return fmt.Errorf(
				"native FROST DKG part two package [%d] identifier is empty",
				i,
			)
		}
		if len(pkg.Data) == 0 {
			return fmt.Errorf(
				"native FROST DKG part two package [%d] data is empty",
				i,
			)
		}
	}

	return nil
}

func validateNativeFROSTDKGResult(result *NativeFROSTDKGResult) error {
	if result == nil {
		return fmt.Errorf("native FROST DKG result is nil")
	}

	if result.KeyPackage == nil {
		return fmt.Errorf("native FROST DKG key package is nil")
	}
	if result.KeyPackage.Identifier == "" {
		return fmt.Errorf("native FROST DKG key package identifier is empty")
	}
	if len(result.KeyPackage.Data) == 0 {
		return fmt.Errorf("native FROST DKG key package data is empty")
	}
	if result.PublicKeyPackage == nil {
		return fmt.Errorf("native FROST DKG public key package is nil")
	}
	if result.PublicKeyPackage.VerifyingKey == "" {
		return fmt.Errorf("native FROST DKG public key package verifying key is empty")
	}

	return nil
}
