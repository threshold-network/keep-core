//go:build frost_native && frost_tbtc_signer && cgo

package signing

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"
)

var realCgoTestSignerAnchor struct {
	sync.Mutex
	privateKey ed25519.PrivateKey
	binding    [32]byte
	latest     NativeTBTCSignerStateWitnessTip
}

type realCgoTestSignerAnchorCommitter struct{}

func (committer *realCgoTestSignerAnchorCommitter) VerifyNativeTBTCSignerStateTip(
	_ context.Context,
	local NativeTBTCSignerStateWitnessTip,
) error {
	if local != realCgoTestSignerAnchor.latest {
		return fmt.Errorf("real-cgo test signer tip differs from its test anchor")
	}
	return nil
}

func (committer *realCgoTestSignerAnchorCommitter) CommitNativeTBTCSignerStateTransition(
	_ context.Context,
	_ string,
	expected NativeTBTCSignerStateWitnessTip,
	candidate NativeTBTCSignerStateWitnessTip,
) (*NativeTBTCSignerStateWitnessTip, error) {
	if expected != realCgoTestSignerAnchor.latest {
		return nil, fmt.Errorf(
			"real-cgo test anchor expected tip changed before commit",
		)
	}
	acknowledgement, err := realCgoTestSignerAnchorAcknowledgement(
		candidate,
		expected.AnchorServiceEpoch,
		expected.AnchorRevision+1,
		expected.AnchorEventRoot,
	)
	if err != nil {
		return nil, err
	}
	result, err :=
		AcknowledgeNativeTBTCSignerStateWitnessCheckpoint(acknowledgement)
	if err != nil {
		return nil, err
	}
	if result == nil || !result.Acknowledged {
		return nil, fmt.Errorf("real-cgo test acknowledgement was not installed")
	}
	readback, err := ReadNativeTBTCSignerStateWitnessTip()
	if err != nil {
		return nil, err
	}
	realCgoTestSignerAnchor.latest = *readback
	return readback, nil
}

// setupRealCgoSignerStateAnchor gives the real-cgo suites an actual ABI-4.2
// acknowledgement path. The independent-service half is deliberately an
// in-process test double, but Rust still verifies the frozen signed
// acknowledgement transcript and durably installs it before the central FFI
// barrier releases any native output.
func setupRealCgoSignerStateAnchor(t *testing.T) {
	t.Helper()
	realCgoTestSignerAnchor.Lock()
	defer realCgoTestSignerAnchor.Unlock()

	if realCgoTestSignerAnchor.privateKey == nil {
		seed := sha256.Sum256(
			[]byte("keep-core/real-cgo/native-signer-anchor-test-key/v1"),
		)
		realCgoTestSignerAnchor.privateKey =
			ed25519.NewKeyFromSeed(seed[:])
		realCgoTestSignerAnchor.binding = sha256.Sum256(
			[]byte("keep-core/real-cgo/native-signer-anchor-binding/v1"),
		)
	}
	publicKey := realCgoTestSignerAnchor.privateKey.Public().(ed25519.PublicKey)
	publicKeySPKI, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		t.Fatal(err)
	}
	publicKeySPKIHash := sha256.Sum256(publicKeySPKI)
	t.Setenv(
		"TBTC_SIGNER_STATE_ANCHOR_BINDING_HASH",
		realCgoTestHex32(realCgoTestSignerAnchor.binding),
	)
	t.Setenv(
		"TBTC_SIGNER_STATE_ANCHOR_RESPONSE_PUBLIC_KEY",
		realCgoTestHex32([32]byte(publicKey)),
	)
	t.Setenv(
		"TBTC_SIGNER_STATE_ANCHOR_RESPONSE_PUBLIC_KEY_SPKI_SHA256",
		realCgoTestHex32(publicKeySPKIHash),
	)
	t.Setenv("TBTC_SIGNER_STATE_WITNESS_MAX_RECORDS", "4096")
	t.Setenv("TBTC_SIGNER_STATE_WITNESS_ROTATION_THRESHOLD_RECORDS", "1024")

	tip, err := ReadNativeTBTCSignerStateWitnessTip()
	skipFrostUnavailable(t, "state-witness tip", err)
	if err != nil {
		t.Fatalf("cannot read real-cgo signer state tip: %v", err)
	}
	if tip.AnchorBindingHash == [32]byte{} {
		acknowledgement, err := realCgoTestSignerAnchorAcknowledgement(
			*tip,
			1,
			1,
			[32]byte{},
		)
		if err != nil {
			t.Fatal(err)
		}
		if _, err :=
			AcknowledgeNativeTBTCSignerStateWitnessCheckpoint(
				acknowledgement,
			); err != nil {
			t.Fatalf("cannot install initial real-cgo test anchor: %v", err)
		}
		tip, err = ReadNativeTBTCSignerStateWitnessTip()
		if err != nil {
			t.Fatal(err)
		}
	}
	if tip.AnchorBindingHash != realCgoTestSignerAnchor.binding ||
		tip.AnchorServiceEpoch != 1 || tip.AnchorRevision == 0 {
		t.Fatalf("real-cgo signer has an unexpected persisted test anchor: %+v", tip)
	}
	realCgoTestSignerAnchor.latest = *tip

	barrier := &globalNativeTBTCSignerStateAnchorBarrier
	barrier.mutex.Lock()
	installed := barrier.installed
	if installed && barrier.expectedAnchorBindingHash !=
		realCgoTestSignerAnchor.binding {
		barrier.mutex.Unlock()
		t.Fatal("another test installed a different native signer anchor")
	}
	barrier.mutex.Unlock()
	if installed {
		return
	}
	if err := InstallNativeTBTCSignerStateAnchorBarrier(
		NativeTBTCSignerStateAnchorBarrierConfig{
			InitialTip:                tip,
			ExpectedAnchorBindingHash: realCgoTestSignerAnchor.binding,
			MinimumAnchorServiceEpoch: 1,
			ReadTip:                   ReadNativeTBTCSignerStateWitnessTip,
			Committer:                 &realCgoTestSignerAnchorCommitter{},
			Timeout:                   15 * time.Second,
		},
	); err != nil {
		t.Fatalf("cannot install real-cgo test signer anchor barrier: %v", err)
	}
}

func realCgoTestSignerAnchorAcknowledgement(
	tip NativeTBTCSignerStateWitnessTip,
	serviceEpoch uint64,
	revision uint64,
	previousEventRoot [32]byte,
) ([]byte, error) {
	if serviceEpoch == 0 || revision == 0 ||
		tip.StoreFingerprint == [32]byte{} ||
		tip.StateCommitment == [32]byte{} {
		return nil, fmt.Errorf("real-cgo test acknowledgement input is invalid")
	}
	operationID := sha256.Sum256(append(
		[]byte("real-cgo-test-anchor-operation/v1\x00"),
		tip.StateCommitment[:]...,
	))
	transitionDigest := sha256.Sum256(append(
		[]byte("real-cgo-test-anchor-transition/v1\x00"),
		operationID[:]...,
	))
	requestDigest := sha256.Sum256(append(
		[]byte("real-cgo-test-anchor-request/v1\x00"),
		operationID[:]...,
	))
	nonce := sha256.Sum256(append(
		[]byte("real-cgo-test-anchor-nonce/v1\x00"),
		transitionDigest[:]...,
	))
	committedAt := uint64(time.Now().UnixMilli())
	expiresAt := committedAt + uint64((20*time.Second)/time.Millisecond)
	eventRoot := realCgoTestSignerAnchorEventRoot(
		realCgoTestSignerAnchor.binding,
		serviceEpoch,
		revision,
		previousEventRoot,
		requestDigest,
		nonce,
		tip,
		operationID,
		transitionDigest,
		committedAt,
		expiresAt,
	)
	signingDigest := realCgoTestSignerAnchorSigningDigest(
		realCgoTestSignerAnchor.binding,
		requestDigest,
		nonce,
		serviceEpoch,
		revision,
		previousEventRoot,
		eventRoot,
		tip,
		operationID,
		transitionDigest,
		committedAt,
		expiresAt,
	)
	signature := ed25519.Sign(
		realCgoTestSignerAnchor.privateKey,
		signingDigest[:],
	)
	wire := struct {
		Schema            string `json:"schema"`
		BindingHash       string `json:"bindingHash"`
		RequestDigest     string `json:"requestDigest"`
		Nonce             string `json:"nonce"`
		Status            string `json:"status"`
		ServiceEpoch      string `json:"serviceEpoch"`
		Revision          string `json:"revision"`
		PreviousEventRoot string `json:"previousEventRoot"`
		EventRoot         string `json:"eventRoot"`
		Checkpoint        struct {
			StoreFingerprint        string `json:"storeFingerprint"`
			Generation              string `json:"generation"`
			PreviousStateCommitment string `json:"previousStateCommitment"`
			StateImageDigest        string `json:"stateImageDigest"`
			StateCommitment         string `json:"stateCommitment"`
		} `json:"checkpoint"`
		OperationID       string `json:"operationID"`
		TransitionDigest  string `json:"transitionDigest"`
		CommittedAtUnixMs string `json:"committedAtUnixMs"`
		ExpiresAtUnixMs   string `json:"expiresAtUnixMs"`
		Signature         string `json:"signature"`
	}{
		Schema:            "tbtc-signer-state-witness-checkpoint-ack/v1",
		BindingHash:       realCgoTestHex32(realCgoTestSignerAnchor.binding),
		RequestDigest:     realCgoTestHex32(requestDigest),
		Nonce:             realCgoTestHex32(nonce),
		Status:            "applied",
		ServiceEpoch:      strconv.FormatUint(serviceEpoch, 10),
		Revision:          strconv.FormatUint(revision, 10),
		PreviousEventRoot: realCgoTestHex32(previousEventRoot),
		EventRoot:         realCgoTestHex32(eventRoot),
		OperationID:       realCgoTestHex32(operationID),
		TransitionDigest:  realCgoTestHex32(transitionDigest),
		CommittedAtUnixMs: strconv.FormatUint(committedAt, 10),
		ExpiresAtUnixMs:   strconv.FormatUint(expiresAt, 10),
		Signature:         "0x" + hex.EncodeToString(signature),
	}
	wire.Checkpoint.StoreFingerprint =
		realCgoTestHex32(tip.StoreFingerprint)
	wire.Checkpoint.Generation = strconv.FormatUint(tip.Generation, 10)
	wire.Checkpoint.PreviousStateCommitment =
		realCgoTestHex32(tip.PreviousStateCommitment)
	wire.Checkpoint.StateImageDigest =
		realCgoTestHex32(tip.StateImageDigest)
	wire.Checkpoint.StateCommitment =
		realCgoTestHex32(tip.StateCommitment)
	return json.Marshal(wire)
}

func realCgoTestSignerAnchorSigningDigest(
	binding [32]byte,
	requestDigest [32]byte,
	nonce [32]byte,
	serviceEpoch uint64,
	revision uint64,
	previousEventRoot [32]byte,
	eventRoot [32]byte,
	tip NativeTBTCSignerStateWitnessTip,
	operationID [32]byte,
	transitionDigest [32]byte,
	committedAt uint64,
	expiresAt uint64,
) [32]byte {
	buffer := bytes.NewBuffer(nil)
	buffer.WriteString("tbtc-native-signer-state-anchor-service-response/v1\x00")
	buffer.Write(binding[:])
	buffer.Write(requestDigest[:])
	buffer.Write(nonce[:])
	buffer.WriteByte(1)
	writeRealCgoTestUint64(buffer, serviceEpoch)
	writeRealCgoTestUint64(buffer, revision)
	buffer.Write(previousEventRoot[:])
	buffer.Write(eventRoot[:])
	writeRealCgoTestCheckpoint(buffer, tip)
	buffer.Write(operationID[:])
	buffer.Write(transitionDigest[:])
	writeRealCgoTestUint64(buffer, committedAt)
	writeRealCgoTestUint64(buffer, expiresAt)
	return sha256.Sum256(buffer.Bytes())
}

func realCgoTestSignerAnchorEventRoot(
	binding [32]byte,
	serviceEpoch uint64,
	revision uint64,
	previousEventRoot [32]byte,
	requestDigest [32]byte,
	nonce [32]byte,
	tip NativeTBTCSignerStateWitnessTip,
	operationID [32]byte,
	transitionDigest [32]byte,
	committedAt uint64,
	expiresAt uint64,
) [32]byte {
	buffer := bytes.NewBuffer(nil)
	buffer.WriteString("tbtc-native-signer-state-anchor-event/v1\x00")
	buffer.Write(binding[:])
	writeRealCgoTestUint64(buffer, serviceEpoch)
	writeRealCgoTestUint64(buffer, revision)
	buffer.Write(previousEventRoot[:])
	buffer.Write(requestDigest[:])
	buffer.Write(nonce[:])
	buffer.WriteByte(1)
	writeRealCgoTestCheckpoint(buffer, tip)
	buffer.Write(operationID[:])
	buffer.Write(transitionDigest[:])
	writeRealCgoTestUint64(buffer, committedAt)
	writeRealCgoTestUint64(buffer, expiresAt)
	return sha256.Sum256(buffer.Bytes())
}

func writeRealCgoTestCheckpoint(
	buffer *bytes.Buffer,
	tip NativeTBTCSignerStateWitnessTip,
) {
	buffer.Write(tip.StoreFingerprint[:])
	writeRealCgoTestUint64(buffer, tip.Generation)
	buffer.Write(tip.PreviousStateCommitment[:])
	buffer.Write(tip.StateImageDigest[:])
	buffer.Write(tip.StateCommitment[:])
}

func writeRealCgoTestUint64(buffer *bytes.Buffer, value uint64) {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], value)
	buffer.Write(encoded[:])
}

func realCgoTestHex32(value [32]byte) string {
	return "0x" + hex.EncodeToString(value[:])
}
