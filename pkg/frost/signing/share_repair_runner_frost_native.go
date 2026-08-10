//go:build frost_native

package signing

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/ipfs/go-log/v2"
	"github.com/keep-network/keep-core/pkg/crypto/ephemeral"
	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

type shareRepairSecretWire struct {
	ContextDigest       string          `json:"context_digest"`
	SenderIdentifier    uint16          `json:"sender_identifier,omitempty"`
	RecipientIdentifier uint16          `json:"recipient_identifier,omitempty"`
	HelperIdentifier    uint16          `json:"helper_identifier,omitempty"`
	DataHex             json.RawMessage `json:"data_hex"`
}

const shareRepairMaximumPendingMessages = 1024

type shareRepairInstalledWire struct {
	Schema                 string `json:"schema"`
	SessionID              string `json:"session_id"`
	KeyGroup               string `json:"key_group"`
	TargetIdentifier       uint16 `json:"target_identifier"`
	RecoveryEpoch          uint64 `json:"recovery_epoch"`
	AuthorizationDigest    string `json:"authorization_digest"`
	ActiveStoreFingerprint string `json:"active_store_fingerprint"`
	Idempotent             bool   `json:"idempotent"`
}

type shareRepairRunner struct {
	member              group.MemberIndex
	authorization       *ShareRepairAuthorization
	authorizationDigest [32]byte
	contextWire         string
	participants        map[group.MemberIndex]struct{}
	helperSet           map[group.MemberIndex]struct{}
	engine              NativeTBTCSignerShareRepairEngine
	bus                 shareRepairBus
	stream              <-chan shareRepairMessage
	ephemeralPrivate    *ephemeral.PrivateKey
	ephemeralPublic     []byte
	pending             []shareRepairMessage
}

type shareRepairRunnerOutcome struct {
	member group.MemberIndex
	result *NativeShareRepairInstallResult
	err    error
}

// RunShareRepair executes the authenticated confidential RTS protocol for this
// node's local helper/target seats. Every participating node invokes it with
// the same authorization and public package. Only the target node returns a
// non-nil install result; helper-only nodes return nil after delivering sigma.
// The target's native Install call returns only after the existing independent
// state-anchor barrier has acknowledged the durable Rust replacement.
func RunShareRepair(
	ctx context.Context,
	logger log.StandardLogger,
	channel net.BroadcastChannel,
	membershipValidator *group.MembershipValidator,
	engine NativeTBTCSignerShareRepairEngine,
	authorization *ShareRepairAuthorization,
	authorityPublicKey ed25519.PublicKey,
	localMemberIndexes []group.MemberIndex,
) (*NativeShareRepairInstallResult, error) {
	if ctx == nil || engine == nil || len(localMemberIndexes) == 0 {
		return nil, fmt.Errorf("share repair dependencies are incomplete")
	}
	validated, err := validateShareRepairAuthorization(
		authorization,
		authorityPublicKey,
		true,
	)
	if err != nil {
		return nil, fmt.Errorf("share repair authorization is invalid: %w", err)
	}
	bus, err := newBroadcastChannelShareRepairBus(
		ctx,
		logger,
		channel,
		membershipValidator,
	)
	if err != nil {
		return nil, err
	}
	return runShareRepairOnBus(
		ctx,
		engine,
		authorization,
		validated.digest,
		localMemberIndexes,
		bus,
	)
}

func runShareRepairOnBus(
	ctx context.Context,
	engine NativeTBTCSignerShareRepairEngine,
	authorization *ShareRepairAuthorization,
	authorizationDigest [32]byte,
	localMemberIndexes []group.MemberIndex,
	bus shareRepairBus,
) (*NativeShareRepairInstallResult, error) {
	if ctx == nil || engine == nil || authorization == nil ||
		bus == nil || len(localMemberIndexes) == 0 {
		return nil, fmt.Errorf("share repair runner dependencies are incomplete")
	}
	participants := make(map[group.MemberIndex]struct{}, len(authorization.HelperIdentifiers)+1)
	helperSet := make(map[group.MemberIndex]struct{}, len(authorization.HelperIdentifiers))
	for _, helper := range authorization.HelperIdentifiers {
		member := group.MemberIndex(helper)
		participants[member] = struct{}{}
		helperSet[member] = struct{}{}
	}
	participants[group.MemberIndex(authorization.TargetIdentifier)] = struct{}{}

	localSet := make(map[group.MemberIndex]struct{}, len(localMemberIndexes))
	runners := make([]*shareRepairRunner, 0, len(localMemberIndexes))
	started := false
	defer func() {
		if !started {
			for _, runner := range runners {
				runner.ephemeralPrivate.Zero()
			}
		}
	}()
	for _, member := range localMemberIndexes {
		if _, duplicate := localSet[member]; duplicate {
			return nil, fmt.Errorf("duplicate local share-repair seat [%d]", member)
		}
		localSet[member] = struct{}{}
		if _, participating := participants[member]; !participating {
			return nil, fmt.Errorf("local seat [%d] is not in the repair authorization", member)
		}
		keyPair, err := ephemeral.GenerateKeyPair()
		if err != nil {
			return nil, fmt.Errorf("cannot generate recovery ephemeral key for seat [%d]: %w", member, err)
		}
		runners = append(runners, &shareRepairRunner{
			member:              member,
			authorization:       authorization,
			authorizationDigest: authorizationDigest,
			contextWire:         fmt.Sprintf("0x%x", authorizationDigest),
			participants:        participants,
			helperSet:           helperSet,
			engine:              engine,
			bus:                 bus,
			stream:              bus.Subscribe(member),
			ephemeralPrivate:    keyPair.PrivateKey,
			ephemeralPublic:     keyPair.PublicKey.Marshal(),
		})
	}
	bus.Start()
	started = true
	runContext, cancel := context.WithCancel(ctx)
	defer cancel()

	outcomes := make(chan shareRepairRunnerOutcome, len(runners))
	for _, runner := range runners {
		runner := runner
		go func() {
			defer runner.ephemeralPrivate.Zero()
			result, err := runner.run(runContext)
			outcomes <- shareRepairRunnerOutcome{member: runner.member, result: result, err: err}
		}()
	}

	var targetResult *NativeShareRepairInstallResult
	var firstError error
	for range runners {
		outcome := <-outcomes
		if outcome.err != nil && firstError == nil {
			firstError = fmt.Errorf("share repair seat [%d] failed: %w", outcome.member, outcome.err)
			cancel()
		}
		if outcome.result != nil {
			if targetResult != nil {
				firstError = fmt.Errorf("multiple local target results were returned")
			} else {
				targetResult = outcome.result
			}
		}
	}
	if firstError != nil {
		return targetResult, firstError
	}
	if _, targetLocal := localSet[group.MemberIndex(authorization.TargetIdentifier)]; targetLocal &&
		targetResult == nil {
		return nil, fmt.Errorf("local share-repair target returned no install result")
	}
	return targetResult, nil
}

func (runner *shareRepairRunner) run(
	ctx context.Context,
) (*NativeShareRepairInstallResult, error) {
	announcement := shareRepairMessage{
		Type:               shareRepairAnnouncementMessage,
		Sender:             runner.member,
		ContextDigest:      runner.authorizationDigest,
		EphemeralPublicKey: runner.ephemeralPublic,
	}
	runner.bus.Broadcast(announcement)
	stopAnnouncements := make(chan struct{})
	announcementsStopped := make(chan struct{})
	go func() {
		defer close(announcementsStopped)
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-stopAnnouncements:
				return
			case <-ticker.C:
				runner.bus.Broadcast(announcement)
			}
		}
	}()
	defer func() {
		close(stopAnnouncements)
		<-announcementsStopped
	}()

	publicKeys, err := runner.collectAnnouncements(ctx)
	if err != nil {
		return nil, err
	}
	if runner.member == group.MemberIndex(runner.authorization.TargetIdentifier) {
		return runner.runTarget(ctx, publicKeys)
	}
	if _, helper := runner.helperSet[runner.member]; !helper {
		return nil, fmt.Errorf("seat is neither target nor helper")
	}
	return nil, runner.runHelper(ctx, publicKeys)
}

func (runner *shareRepairRunner) collectAnnouncements(
	ctx context.Context,
) (map[group.MemberIndex]*ephemeral.PublicKey, error) {
	publicKeys := make(map[group.MemberIndex]*ephemeral.PublicKey, len(runner.participants))
	selfPublic, err := ephemeral.UnmarshalPublicKey(runner.ephemeralPublic)
	if err != nil {
		return nil, fmt.Errorf("cannot parse local recovery ephemeral key: %w", err)
	}
	publicKeys[runner.member] = selfPublic
	for len(publicKeys) < len(runner.participants) {
		var message shareRepairMessage
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("collect recovery announcements: %w", ctx.Err())
		case message = <-runner.stream:
		}
		if message.ContextDigest != runner.authorizationDigest {
			continue
		}
		if message.Type != shareRepairAnnouncementMessage {
			if len(runner.pending) >= shareRepairMaximumPendingMessages {
				return nil, fmt.Errorf(
					"share-repair pending-message limit exceeded before announcements completed",
				)
			}
			runner.pending = append(runner.pending, message)
			continue
		}
		if _, expected := runner.participants[message.Sender]; !expected {
			continue
		}
		parsed, err := ephemeral.UnmarshalPublicKey(message.EphemeralPublicKey)
		if err != nil {
			return nil, fmt.Errorf("invalid recovery announcement from [%d]: %w", message.Sender, err)
		}
		if existing, seen := publicKeys[message.Sender]; seen {
			if !bytes.Equal(existing.Marshal(), parsed.Marshal()) {
				return nil, fmt.Errorf("recovery seat [%d] equivocated its ephemeral key", message.Sender)
			}
			continue
		}
		publicKeys[message.Sender] = parsed
	}
	return publicKeys, nil
}

func (runner *shareRepairRunner) nextMessage(
	ctx context.Context,
) (shareRepairMessage, error) {
	if len(runner.pending) > 0 {
		message := runner.pending[0]
		runner.pending[0] = shareRepairMessage{}
		runner.pending = runner.pending[1:]
		return message, nil
	}
	select {
	case <-ctx.Done():
		return shareRepairMessage{}, ctx.Err()
	case message := <-runner.stream:
		return message, nil
	}
}

func encodeShareRepairSealedSecret(
	secret shareRepairSecretWire,
	recipient *ephemeral.PublicKey,
) ([]byte, error) {
	defer zeroBytes(secret.DataHex)
	plaintext, err := json.Marshal(secret)
	if err != nil {
		return nil, err
	}
	defer zeroBytes(plaintext)
	sealed, err := sealRound2Share(plaintext, recipient)
	if err != nil {
		return nil, err
	}
	return json.Marshal(sealed)
}

func decodeShareRepairSealedSecret(
	payload []byte,
	privateKey *ephemeral.PrivateKey,
) (*shareRepairSecretWire, error) {
	sealed := &sealedRound2Share{}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(sealed); err != nil {
		return nil, fmt.Errorf("cannot decode sealed recovery envelope: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, fmt.Errorf("sealed recovery envelope has trailing JSON")
	}
	plaintext, err := openRound2Share(sealed, privateKey)
	if err != nil {
		return nil, err
	}
	defer zeroBytes(plaintext)
	secret := &shareRepairSecretWire{}
	decoder = json.NewDecoder(bytes.NewReader(plaintext))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(secret); err != nil {
		zeroBytes(secret.DataHex)
		return nil, fmt.Errorf("cannot decode recovery secret: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		zeroBytes(secret.DataHex)
		return nil, fmt.Errorf("recovery secret has trailing JSON")
	}
	return secret, nil
}

func (runner *shareRepairRunner) runHelper(
	ctx context.Context,
	publicKeys map[group.MemberIndex]*ephemeral.PublicKey,
) error {
	part1, err := runner.engine.ShareRepairPart1(
		runner.authorization,
		uint16(runner.member),
	)
	if err != nil {
		return fmt.Errorf("native repair part1: %w", err)
	}
	if part1 != nil {
		defer func() {
			for _, delta := range part1.Deltas {
				if delta != nil {
					zeroBytes(delta.Data)
				}
			}
		}()
	}
	if part1 == nil || part1.ContextDigest != runner.contextWire ||
		part1.HelperIdentifier != uint16(runner.member) ||
		part1.PublicKeyPackage == nil ||
		len(part1.PublicKeyPackage.VerifyingShares) != int(runner.authorization.ParticipantCount) ||
		part1.PublicKeyPackage.VerifyingKey == "" ||
		len(part1.Deltas) != len(runner.authorization.HelperIdentifiers) {
		return fmt.Errorf("native repair part1 returned the wrong context or delta set")
	}
	publicPackage, err := json.Marshal(part1.PublicKeyPackage)
	if err != nil {
		return fmt.Errorf("encode repair public key package: %w", err)
	}
	if len(publicPackage) > shareRepairMaximumPublicPayload {
		return fmt.Errorf("repair public key package exceeds the transport cap")
	}
	runner.bus.Broadcast(shareRepairMessage{
		Type:          shareRepairPublicPackageMessage,
		Sender:        runner.member,
		ContextDigest: runner.authorizationDigest,
		Payload:       publicPackage,
	})
	for index, recipient := range runner.authorization.HelperIdentifiers {
		delta := part1.Deltas[index]
		if delta == nil || delta.ContextDigest != runner.contextWire ||
			delta.SenderIdentifier != uint16(runner.member) ||
			delta.RecipientIdentifier != recipient || len(delta.Data) != 32 {
			return fmt.Errorf("native repair part1 delta [%d] is invalid", index)
		}
		recipientKey := publicKeys[group.MemberIndex(recipient)]
		if recipientKey == nil {
			return fmt.Errorf("missing recovery ephemeral key for seat [%d]", recipient)
		}
		dataHex, err := encodeShareRepairSecretHexJSON(delta.Data)
		if err != nil {
			return fmt.Errorf("encode repair delta for [%d]: %w", recipient, err)
		}
		sealed, err := encodeShareRepairSealedSecret(
			shareRepairSecretWire{
				ContextDigest:       delta.ContextDigest,
				SenderIdentifier:    delta.SenderIdentifier,
				RecipientIdentifier: delta.RecipientIdentifier,
				DataHex:             dataHex,
			},
			recipientKey,
		)
		if err != nil {
			return fmt.Errorf("seal repair delta for [%d]: %w", recipient, err)
		}
		zeroBytes(delta.Data)
		runner.bus.Broadcast(shareRepairMessage{
			Type:          shareRepairDeltaMessage,
			Sender:        runner.member,
			Recipient:     group.MemberIndex(recipient),
			ContextDigest: runner.authorizationDigest,
			Payload:       sealed,
		})
	}

	deltas := make(map[uint16]*NativeShareRepairDelta, len(runner.helperSet))
	defer func() {
		for _, delta := range deltas {
			zeroBytes(delta.Data)
		}
	}()
	for len(deltas) < len(runner.helperSet) {
		message, err := runner.nextMessage(ctx)
		if err != nil {
			return fmt.Errorf("collect recovery deltas: %w", err)
		}
		if message.Type != shareRepairDeltaMessage ||
			message.ContextDigest != runner.authorizationDigest ||
			message.Recipient != runner.member {
			continue
		}
		if _, expected := runner.helperSet[message.Sender]; !expected {
			continue
		}
		secret, err := decodeShareRepairSealedSecret(message.Payload, runner.ephemeralPrivate)
		if err != nil {
			return fmt.Errorf("open repair delta from [%d]: %w", message.Sender, err)
		}
		data, err := decodeShareRepairSecretHexJSON(secret.DataHex)
		zeroBytes(secret.DataHex)
		if err != nil || len(data) != 32 || secret.ContextDigest != runner.contextWire ||
			secret.SenderIdentifier != uint16(message.Sender) ||
			secret.RecipientIdentifier != uint16(runner.member) ||
			secret.HelperIdentifier != 0 {
			zeroBytes(data)
			return fmt.Errorf("repair delta from [%d] has invalid bindings", message.Sender)
		}
		if existing := deltas[secret.SenderIdentifier]; existing != nil {
			if !bytes.Equal(existing.Data, data) {
				zeroBytes(data)
				return fmt.Errorf("repair helper [%d] equivocated its delta", message.Sender)
			}
			zeroBytes(data)
			continue
		}
		deltas[secret.SenderIdentifier] = &NativeShareRepairDelta{
			ContextDigest:       secret.ContextDigest,
			SenderIdentifier:    secret.SenderIdentifier,
			RecipientIdentifier: secret.RecipientIdentifier,
			Data:                data,
		}
	}
	ordered := make([]*NativeShareRepairDelta, 0, len(deltas))
	for _, sender := range runner.authorization.HelperIdentifiers {
		ordered = append(ordered, deltas[sender])
	}
	part2, err := runner.engine.ShareRepairPart2(
		runner.authorization,
		uint16(runner.member),
		ordered,
	)
	if err != nil {
		return fmt.Errorf("native repair part2: %w", err)
	}
	for _, delta := range deltas {
		zeroBytes(delta.Data)
	}
	if part2 != nil && part2.Sigma != nil {
		defer zeroBytes(part2.Sigma.Data)
	}
	if part2 == nil || part2.ContextDigest != runner.contextWire || part2.Sigma == nil ||
		part2.Sigma.ContextDigest != runner.contextWire ||
		part2.Sigma.HelperIdentifier != uint16(runner.member) || len(part2.Sigma.Data) != 32 {
		return fmt.Errorf("native repair part2 returned an invalid sigma")
	}
	targetKey := publicKeys[group.MemberIndex(runner.authorization.TargetIdentifier)]
	if targetKey == nil {
		return fmt.Errorf("missing recovery ephemeral key for target seat")
	}
	dataHex, err := encodeShareRepairSecretHexJSON(part2.Sigma.Data)
	if err != nil {
		return fmt.Errorf("encode repair sigma: %w", err)
	}
	sealed, err := encodeShareRepairSealedSecret(
		shareRepairSecretWire{
			ContextDigest:    part2.Sigma.ContextDigest,
			HelperIdentifier: part2.Sigma.HelperIdentifier,
			DataHex:          dataHex,
		},
		targetKey,
	)
	if err != nil {
		return fmt.Errorf("seal repair sigma: %w", err)
	}
	zeroBytes(part2.Sigma.Data)
	runner.bus.Broadcast(shareRepairMessage{
		Type:          shareRepairSigmaMessage,
		Sender:        runner.member,
		Recipient:     group.MemberIndex(runner.authorization.TargetIdentifier),
		ContextDigest: runner.authorizationDigest,
		Payload:       sealed,
	})
	return runner.waitForInstalledReceipt(ctx)
}

func (runner *shareRepairRunner) runTarget(
	ctx context.Context,
	_ map[group.MemberIndex]*ephemeral.PublicKey,
) (*NativeShareRepairInstallResult, error) {
	sigmas := make(map[uint16]*NativeShareRepairSigma, len(runner.helperSet))
	publicPackages := make(map[uint16][]byte, len(runner.helperSet))
	var publicKeyPackage *NativeFROSTPublicKeyPackage
	var canonicalPublicKeyPackage []byte
	defer func() {
		for _, sigma := range sigmas {
			zeroBytes(sigma.Data)
		}
	}()
	for len(sigmas) < len(runner.helperSet) || len(publicPackages) < len(runner.helperSet) {
		message, err := runner.nextMessage(ctx)
		if err != nil {
			return nil, fmt.Errorf("collect recovery sigmas: %w", err)
		}
		if message.ContextDigest != runner.authorizationDigest {
			continue
		}
		if _, expected := runner.helperSet[message.Sender]; !expected {
			continue
		}
		if message.Type == shareRepairPublicPackageMessage {
			candidate := &NativeFROSTPublicKeyPackage{}
			decoder := json.NewDecoder(bytes.NewReader(message.Payload))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(candidate); err != nil {
				return nil, fmt.Errorf("decode repair public package from [%d]: %w", message.Sender, err)
			}
			if err := decoder.Decode(&struct{}{}); err != io.EOF {
				return nil, fmt.Errorf("repair public package from [%d] has trailing JSON", message.Sender)
			}
			if len(candidate.VerifyingShares) != int(runner.authorization.ParticipantCount) ||
				candidate.VerifyingKey == "" {
				return nil, fmt.Errorf("repair public package from [%d] has the wrong shape", message.Sender)
			}
			canonical, err := json.Marshal(candidate)
			if err != nil {
				return nil, fmt.Errorf("canonicalize repair public package: %w", err)
			}
			if existing := publicPackages[uint16(message.Sender)]; existing != nil {
				if !bytes.Equal(existing, canonical) {
					return nil, fmt.Errorf("repair helper [%d] equivocated its public package", message.Sender)
				}
				continue
			}
			if canonicalPublicKeyPackage != nil &&
				!bytes.Equal(canonicalPublicKeyPackage, canonical) {
				return nil, fmt.Errorf("repair helpers disagree on the public key package")
			}
			canonicalPublicKeyPackage = append([]byte(nil), canonical...)
			publicPackages[uint16(message.Sender)] = canonical
			publicKeyPackage = candidate
			continue
		}
		if message.Type != shareRepairSigmaMessage || message.Recipient != runner.member {
			continue
		}
		secret, err := decodeShareRepairSealedSecret(message.Payload, runner.ephemeralPrivate)
		if err != nil {
			return nil, fmt.Errorf("open repair sigma from [%d]: %w", message.Sender, err)
		}
		data, err := decodeShareRepairSecretHexJSON(secret.DataHex)
		zeroBytes(secret.DataHex)
		if err != nil || len(data) != 32 || secret.ContextDigest != runner.contextWire ||
			secret.HelperIdentifier != uint16(message.Sender) ||
			secret.SenderIdentifier != 0 || secret.RecipientIdentifier != 0 {
			zeroBytes(data)
			return nil, fmt.Errorf("repair sigma from [%d] has invalid bindings", message.Sender)
		}
		if existing := sigmas[secret.HelperIdentifier]; existing != nil {
			if !bytes.Equal(existing.Data, data) {
				zeroBytes(data)
				return nil, fmt.Errorf("repair helper [%d] equivocated its sigma", message.Sender)
			}
			zeroBytes(data)
			continue
		}
		sigmas[secret.HelperIdentifier] = &NativeShareRepairSigma{
			ContextDigest:    secret.ContextDigest,
			HelperIdentifier: secret.HelperIdentifier,
			Data:             data,
		}
	}
	ordered := make([]*NativeShareRepairSigma, 0, len(sigmas))
	for _, helper := range runner.authorization.HelperIdentifiers {
		ordered = append(ordered, sigmas[helper])
	}
	result, err := runner.engine.InstallRepairedShare(
		runner.authorization,
		publicKeyPackage,
		ordered,
	)
	if err != nil {
		return nil, fmt.Errorf("native repaired-share install: %w", err)
	}
	for _, sigma := range sigmas {
		zeroBytes(sigma.Data)
	}
	if result == nil || result.Schema != ShareRepairInstallResultSchema ||
		result.SessionID != runner.authorization.SessionID ||
		result.KeyGroup != runner.authorization.KeyGroup ||
		result.TargetIdentifier != runner.authorization.TargetIdentifier ||
		result.RecoveryEpoch != runner.authorization.RecoveryEpoch ||
		result.AuthorizationDigest != runner.contextWire ||
		result.ActiveStoreFingerprint != runner.authorization.NewStoreFingerprint {
		return nil, fmt.Errorf("native repaired-share install result does not match authorization")
	}
	if err := recordInstalledShareRepair(result); err != nil {
		return nil, fmt.Errorf("arm repaired-seat activation guard: %w", err)
	}
	receipt, err := json.Marshal(shareRepairInstalledWire{
		Schema:                 result.Schema,
		SessionID:              result.SessionID,
		KeyGroup:               result.KeyGroup,
		TargetIdentifier:       result.TargetIdentifier,
		RecoveryEpoch:          result.RecoveryEpoch,
		AuthorizationDigest:    result.AuthorizationDigest,
		ActiveStoreFingerprint: result.ActiveStoreFingerprint,
		Idempotent:             result.Idempotent,
	})
	if err != nil {
		return nil, fmt.Errorf("encode repaired-share installed receipt: %w", err)
	}
	runner.bus.Broadcast(shareRepairMessage{
		Type:          shareRepairInstalledMessage,
		Sender:        runner.member,
		ContextDigest: runner.authorizationDigest,
		Payload:       receipt,
	})
	return result, nil
}

func (runner *shareRepairRunner) waitForInstalledReceipt(ctx context.Context) error {
	for {
		message, err := runner.nextMessage(ctx)
		if err != nil {
			return fmt.Errorf("wait for repaired-share installed receipt: %w", err)
		}
		if message.Type != shareRepairInstalledMessage ||
			message.ContextDigest != runner.authorizationDigest ||
			message.Sender != group.MemberIndex(runner.authorization.TargetIdentifier) {
			continue
		}
		receipt := &shareRepairInstalledWire{}
		decoder := json.NewDecoder(bytes.NewReader(message.Payload))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(receipt); err != nil {
			return fmt.Errorf("decode repaired-share installed receipt: %w", err)
		}
		if err := decoder.Decode(&struct{}{}); err != io.EOF {
			return fmt.Errorf("repaired-share installed receipt has trailing JSON")
		}
		if receipt.Schema != ShareRepairInstallResultSchema ||
			receipt.SessionID != runner.authorization.SessionID ||
			receipt.KeyGroup != runner.authorization.KeyGroup ||
			receipt.TargetIdentifier != runner.authorization.TargetIdentifier ||
			receipt.RecoveryEpoch != runner.authorization.RecoveryEpoch ||
			receipt.AuthorizationDigest != runner.contextWire ||
			receipt.ActiveStoreFingerprint != runner.authorization.NewStoreFingerprint {
			return fmt.Errorf("repaired-share installed receipt does not match authorization")
		}
		return nil
	}
}
