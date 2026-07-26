package tbtc

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"
)

// FrostNativeSignerAnchorTrustTransitionTarget retains both the exact signed
// Read wrapper passed to Rust and the fully verified service reference it
// represents. Startup uses the latter to validate Rust's transition result
// before it opens the signer store.
type FrostNativeSignerAnchorTrustTransitionTarget struct {
	ExactReadResponse []byte
	Reference         FrostNativeSignerAnchorTrustReference
}

// ValidateFrostNativeSignerAnchorTrustTargetAcknowledgement is the
// certificate-contextual acknowledgement verifier used by production chain
// validation. It is intentionally separate from the generic same-epoch
// verifier: only an offline-authority-signed trust certificate may authorize a
// revision-one event whose predecessor root links the previous service epoch.
func ValidateFrostNativeSignerAnchorTrustTargetAcknowledgement(
	certificate *FrostNativeSignerAnchorTrustCertificate,
	rawAcknowledgement []byte,
) error {
	_, err := verifyFrostNativeSignerAnchorTrustTargetAcknowledgement(
		certificate,
		rawAcknowledgement,
	)
	return err
}

func verifyFrostNativeSignerAnchorTrustTargetAcknowledgement(
	certificate *FrostNativeSignerAnchorTrustCertificate,
	rawAcknowledgement []byte,
) (*FrostNativeSignerCheckpointAcknowledgement, error) {
	if certificate == nil || len(rawAcknowledgement) == 0 {
		return nil, fmt.Errorf(
			"native signer trust target acknowledgement dependencies are incomplete",
		)
	}
	if err := frostNativeSignerAnchorTrustValidateEd25519Point(
		certificate.To.ResponsePublicKey,
	); err != nil {
		return nil, fmt.Errorf(
			"native signer trust target response key is invalid: %w",
			err,
		)
	}
	wire := frostNativeSignerAnchorAcknowledgementWire{}
	if err := decodeStrictFrostNativeSignerAnchorJSON(
		rawAcknowledgement,
		&wire,
	); err != nil {
		return nil, err
	}
	if wire.Schema != FrostNativeSignerCheckpointAcknowledgementSchema ||
		wire.Status != "applied" {
		return nil, fmt.Errorf(
			"native signer trust target acknowledgement schema or status is invalid",
		)
	}
	signingDigest, err := frostNativeSignerAnchorAcknowledgementTranscript(wire)
	if err != nil {
		return nil, err
	}
	signature, err := frostNativeSignerAnchorParseSignature(wire.Signature)
	if err != nil || !ed25519.Verify(
		ed25519.PublicKey(certificate.To.ResponsePublicKey[:]),
		signingDigest,
		signature[:],
	) {
		return nil, fmt.Errorf(
			"native signer trust target acknowledgement signature is invalid",
		)
	}
	acknowledgement, err := frostNativeSignerAnchorAcknowledgementFromWire(wire)
	if err != nil {
		return nil, err
	}
	target := certificate.To.Reference
	if acknowledgement.BindingHash != certificate.To.BindingHash ||
		acknowledgement.RequestDigest == [32]byte{} ||
		acknowledgement.Nonce == [32]byte{} ||
		acknowledgement.ServiceEpoch != target.ServiceEpoch ||
		acknowledgement.Revision != target.Revision ||
		acknowledgement.PreviousEventRoot != target.PreviousEventRoot ||
		acknowledgement.EventRoot != target.EventRoot ||
		acknowledgement.Checkpoint != target.Checkpoint ||
		acknowledgement.OperationID != certificate.OperationID ||
		acknowledgement.TransitionDigest != certificate.TransitionDigest {
		return nil, fmt.Errorf(
			"native signer trust target acknowledgement differs from its certificate",
		)
	}
	if acknowledgement.ServiceEpoch == 0 || acknowledgement.Revision != 1 ||
		acknowledgement.CommittedAtUnixMs == 0 ||
		acknowledgement.ExpiresAtUnixMs <=
			acknowledgement.CommittedAtUnixMs ||
		acknowledgement.ExpiresAtUnixMs-
			acknowledgement.CommittedAtUnixMs >
			uint64(frostNativeSignerAnchorMaximumAcknowledgementLifetime/
				time.Millisecond) {
		return nil, fmt.Errorf(
			"native signer trust target acknowledgement lifetime is invalid",
		)
	}
	if err := validateFrostNativeSignerAnchorCheckpoint(
		acknowledgement.Checkpoint,
		certificate.SignerStoreFingerprint,
	); err != nil {
		return nil, err
	}
	if computeFrostNativeSignerAnchorEventRoot(*acknowledgement) !=
		acknowledgement.EventRoot {
		return nil, fmt.Errorf(
			"native signer trust target acknowledgement event root is invalid",
		)
	}
	acknowledgement.Signature = signature
	copy(acknowledgement.SigningDigest[:], signingDigest)
	acknowledgement.AcknowledgementDigest =
		computeFrostNativeSignerCheckpointAcknowledgementDigest(
			acknowledgement.SigningDigest,
			signature,
			certificate.To.ResponsePublicKeySPKISHA256,
		)
	if acknowledgement.AcknowledgementDigest !=
		target.AcknowledgementDigest {
		return nil, fmt.Errorf(
			"native signer trust target acknowledgement digest is invalid",
		)
	}
	acknowledgement.ExactAcknowledgement =
		append([]byte{}, rawAcknowledgement...)
	return acknowledgement, nil
}

// readFrostNativeSignerAnchorTrustTransitionTarget performs the fresh final
// Read required immediately before the startup-only Rust transition. The
// authenticated trust-floor capability installed by the private constructor
// supplies the exact certificate. Only an already-installed exact replay may
// return a generic, strictly validated descendant under the same final
// binding.
func (client *FrostNativeSignerAnchorClient) readFrostNativeSignerAnchorTrustTransitionTarget(
	ctx context.Context,
	allowCompletedReplayDescendant bool,
) (*FrostNativeSignerAnchorTrustTransitionTarget, error) {
	if client == nil || ctx == nil || client.certifiedTrustFloor == nil {
		return nil, fmt.Errorf(
			"native signer anchor trust-transition Read dependencies are incomplete",
		)
	}
	finalCertificate := client.certifiedTrustFloor
	client.mutex.Lock()
	defer client.mutex.Unlock()
	if client.poisoned != nil {
		return nil, fmt.Errorf(
			"native signer anchor client is poisoned: %w",
			client.poisoned,
		)
	}
	if finalCertificate.To.BindingHash != client.bindingHash ||
		finalCertificate.ProtocolID != client.identity.ProtocolID ||
		finalCertificate.StreamID != client.identity.StreamID ||
		finalCertificate.SignerStoreFingerprint !=
			client.identity.SignerStoreFingerprint ||
		finalCertificate.To.ResponsePublicKeySPKISHA256 !=
			client.identity.OnlineKeyHash {
		return nil, fmt.Errorf(
			"native signer trust certificate differs from the final client identity",
		)
	}

	nonce, err := client.randomBytes32()
	if err != nil {
		return nil, fmt.Errorf(
			"cannot create native signer trust-transition Read nonce: %w",
			err,
		)
	}
	transcript := frostNativeSignerAnchorReadRequestTranscript(
		client.identity,
		nonce,
		client.clientSPKIDER,
	)
	requestDigest := sha256.Sum256(transcript)
	request := frostNativeSignerAnchorReadRequest{
		Schema: FrostNativeSignerAnchorReadRequestSchema,
		Payload: frostNativeSignerAnchorReadRequestPayload{
			Kind:        "read",
			Nonce:       frostNativeSignerAnchorHex32(nonce),
			BindingHash: frostNativeSignerAnchorHex32(client.bindingHash),
			Identity:    frostNativeSignerAnchorIdentityToWire(client.identity),
		},
		ClientPublicKeySPKI: client.clientSPKIBase64,
		Signature: frostNativeSignerAnchorSignatureHex(
			ed25519.Sign(client.clientKey, transcript),
		),
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot encode native signer trust-transition Read: %w",
			err,
		)
	}
	response, _, err := client.post(ctx, client.readEndpoint, payload)
	if err != nil {
		return nil, err
	}
	readResponse := frostNativeSignerAnchorReadResponse{}
	if err := decodeStrictFrostNativeSignerAnchorJSON(
		response,
		&readResponse,
	); err != nil {
		return nil, fmt.Errorf(
			"invalid native signer trust-transition Read response: %w",
			err,
		)
	}
	if readResponse.Schema != FrostNativeSignerAnchorReadResponseSchema ||
		readResponse.Status != "present" ||
		readResponse.Checkpoint == nil {
		return nil, fmt.Errorf(
			"native signer trust-transition Read response is absent or unsupported",
		)
	}
	responseDigest, err :=
		frostNativeSignerAnchorReadResponseTranscript(readResponse)
	if err != nil {
		return nil, err
	}
	responseSignature, err :=
		frostNativeSignerAnchorParseSignature(readResponse.Signature)
	if err != nil || !ed25519.Verify(
		client.onlineKey,
		responseDigest,
		responseSignature[:],
	) {
		return nil, fmt.Errorf(
			"native signer trust-transition Read signature is invalid",
		)
	}
	responseBindingHash, err :=
		frostNativeSignerAnchorParseHex32(readResponse.BindingHash)
	if err != nil || responseBindingHash != client.bindingHash {
		return nil, fmt.Errorf(
			"native signer trust-transition Read binding is invalid",
		)
	}
	responseRequestDigest, err :=
		frostNativeSignerAnchorParseHex32(readResponse.RequestDigest)
	if err != nil || responseRequestDigest != requestDigest {
		return nil, fmt.Errorf(
			"native signer trust-transition Read request digest is invalid",
		)
	}
	responseNonce, err := frostNativeSignerAnchorParseHex32(readResponse.Nonce)
	if err != nil || responseNonce != nonce {
		return nil, fmt.Errorf(
			"native signer trust-transition Read nonce is invalid",
		)
	}
	checkpoint, err :=
		frostNativeSignerAnchorCheckpointFromWire(*readResponse.Checkpoint)
	if err != nil {
		return nil, err
	}
	operationID, err :=
		frostNativeSignerAnchorParseHex32(readResponse.OperationID)
	if err != nil {
		return nil, err
	}
	transitionDigest, err :=
		frostNativeSignerAnchorParseHex32(readResponse.TransitionDigest)
	if err != nil {
		return nil, err
	}

	var acknowledgement *FrostNativeSignerCheckpointAcknowledgement
	if bytes.Equal(
		readResponse.CheckpointAck,
		finalCertificate.TargetAcknowledgement,
	) {
		acknowledgement, err =
			verifyFrostNativeSignerAnchorTrustTargetAcknowledgement(
				finalCertificate,
				readResponse.CheckpointAck,
			)
	} else {
		if !allowCompletedReplayDescendant {
			return nil, fmt.Errorf(
				"new native signer trust transition target is not the exact certified acknowledgement",
			)
		}
		acknowledgement, err = client.verifyAcknowledgement(
			readResponse.CheckpointAck,
			nil,
			nil,
			&checkpoint,
			&operationID,
			false,
			"applied",
			"already-applied",
		)
		if err == nil &&
			(acknowledgement.ServiceEpoch !=
				finalCertificate.To.Reference.ServiceEpoch ||
				acknowledgement.Revision <=
					finalCertificate.To.Reference.Revision) {
			err = fmt.Errorf(
				"native signer trust-transition Read is neither the certified floor nor a descendant",
			)
		}
	}
	if err != nil {
		return nil, fmt.Errorf(
			"invalid native signer trust-transition stored acknowledgement: %w",
			err,
		)
	}
	if acknowledgement.Checkpoint != checkpoint ||
		acknowledgement.OperationID != operationID ||
		acknowledgement.TransitionDigest != transitionDigest {
		return nil, fmt.Errorf(
			"native signer trust-transition Read summary differs from its acknowledgement",
		)
	}
	serviceEpoch, err :=
		frostNativeSignerAnchorParseUint64(readResponse.ServiceEpoch)
	if err != nil {
		return nil, err
	}
	revision, err := frostNativeSignerAnchorParseUint64(readResponse.Revision)
	if err != nil {
		return nil, err
	}
	eventRoot, err := frostNativeSignerAnchorParseHex32(readResponse.EventRoot)
	if err != nil {
		return nil, err
	}
	acknowledgementDigest, err := frostNativeSignerAnchorParseHex32(
		readResponse.CheckpointAckDigest,
	)
	if err != nil ||
		serviceEpoch != acknowledgement.ServiceEpoch ||
		revision != acknowledgement.Revision ||
		eventRoot != acknowledgement.EventRoot ||
		acknowledgementDigest != acknowledgement.AcknowledgementDigest {
		return nil, fmt.Errorf(
			"native signer trust-transition Read summary is inconsistent",
		)
	}
	committedAt, err :=
		frostNativeSignerAnchorParseUint64(readResponse.CommittedAtUnixMs)
	if err != nil {
		return nil, err
	}
	expiresAt, err :=
		frostNativeSignerAnchorParseUint64(readResponse.ExpiresAtUnixMs)
	if err != nil {
		return nil, err
	}
	nowUnixMs := client.now().UnixMilli()
	if nowUnixMs < 0 || committedAt == 0 || expiresAt <= committedAt ||
		expiresAt-committedAt >
			uint64(client.maximumAckLife/time.Millisecond) ||
		committedAt >
			uint64(nowUnixMs)+uint64(client.clockSkew/time.Millisecond) ||
		expiresAt <= uint64(nowUnixMs) {
		return nil, fmt.Errorf(
			"native signer trust-transition Read wrapper is stale or invalid",
		)
	}
	return &FrostNativeSignerAnchorTrustTransitionTarget{
		ExactReadResponse: append([]byte{}, response...),
		Reference: FrostNativeSignerAnchorTrustReference{
			ServiceEpoch:          acknowledgement.ServiceEpoch,
			Revision:              acknowledgement.Revision,
			PreviousEventRoot:     acknowledgement.PreviousEventRoot,
			EventRoot:             acknowledgement.EventRoot,
			AcknowledgementDigest: acknowledgement.AcknowledgementDigest,
			Checkpoint:            acknowledgement.Checkpoint,
		},
	}, nil
}
