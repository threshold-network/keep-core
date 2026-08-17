package signing

import (
	"encoding/json"
	"fmt"
)

const NativeTBTCSignerStateAnchorBootstrapFactsSchema = "tbtc-signer-state-anchor-bootstrap-facts/v1"

// NativeTBTCSignerStateAnchorBootstrapFacts is the only native state exposed
// to the online half of the initial trust ceremony. The checkpoint is required
// to be the exact generation-one genesis of StoreFingerprint.
type NativeTBTCSignerStateAnchorBootstrapFacts struct {
	Schema            string
	StoreFingerprint  [32]byte
	CurrentCheckpoint NativeTBTCSignerStateAnchorCheckpoint
}

type nativeTBTCSignerStateAnchorBootstrapFactsWire struct {
	Schema            string                                    `json:"schema"`
	StoreFingerprint  string                                    `json:"storeFingerprint"`
	CurrentCheckpoint nativeTBTCSignerStateAnchorCheckpointWire `json:"currentCheckpoint"`
}

// DecodeNativeTBTCSignerStateAnchorBootstrapFacts strictly decodes and
// validates the versioned Rust response. In addition to the normal checkpoint
// transcript, bootstrap facts must prove the exact generation-one genesis
// predecessor for the same stable store fingerprint.
func DecodeNativeTBTCSignerStateAnchorBootstrapFacts(
	payload []byte,
) (*NativeTBTCSignerStateAnchorBootstrapFacts, error) {
	wire := &nativeTBTCSignerStateAnchorBootstrapFactsWire{}
	if err := decodeStrictNativeTBTCSignerJSON(
		payload,
		wire,
		"state-anchor bootstrap facts",
	); err != nil {
		return nil, err
	}
	if wire.Schema != NativeTBTCSignerStateAnchorBootstrapFactsSchema {
		return nil, fmt.Errorf(
			"unsupported native signer state-anchor bootstrap-facts schema",
		)
	}
	storeFingerprint, err := decodeNativeTBTCSignerCanonicalBytes32(
		wire.StoreFingerprint,
		false,
	)
	if err != nil {
		return nil, fmt.Errorf("invalid bootstrap store fingerprint: %w", err)
	}
	checkpoint, err := decodeNativeTBTCSignerStateAnchorCheckpoint(
		&wire.CurrentCheckpoint,
	)
	if err != nil {
		return nil, fmt.Errorf("invalid bootstrap checkpoint: %w", err)
	}
	if checkpoint.StoreFingerprint != storeFingerprint ||
		checkpoint.Generation != 1 ||
		checkpoint.PreviousStateCommitment !=
			ComputeNativeTBTCSignerStateWitnessGenesis(storeFingerprint) {
		return nil, fmt.Errorf(
			"native signer state-anchor bootstrap facts are not the exact store genesis",
		)
	}
	return &NativeTBTCSignerStateAnchorBootstrapFacts{
		Schema:            wire.Schema,
		StoreFingerprint:  storeFingerprint,
		CurrentCheckpoint: checkpoint,
	}, nil
}

// EncodeNativeTBTCSignerStateAnchorBootstrapFacts emits the canonical artifact
// bytes consumed by the offline ceremony. It reuses the strict decoder so a
// caller cannot serialize a non-genesis or cross-store checkpoint.
func EncodeNativeTBTCSignerStateAnchorBootstrapFacts(
	facts *NativeTBTCSignerStateAnchorBootstrapFacts,
) ([]byte, error) {
	if facts == nil {
		return nil, fmt.Errorf("native signer state-anchor bootstrap facts are nil")
	}
	wire := nativeTBTCSignerStateAnchorBootstrapFactsWire{
		Schema:           NativeTBTCSignerStateAnchorBootstrapFactsSchema,
		StoreFingerprint: nativeTBTCSignerBytes32(facts.StoreFingerprint),
		CurrentCheckpoint: nativeTBTCSignerStateAnchorCheckpointWire{
			StoreFingerprint: nativeTBTCSignerBytes32(
				facts.CurrentCheckpoint.StoreFingerprint,
			),
			Generation: fmt.Sprint(facts.CurrentCheckpoint.Generation),
			PreviousStateCommitment: nativeTBTCSignerBytes32(
				facts.CurrentCheckpoint.PreviousStateCommitment,
			),
			StateImageDigest: nativeTBTCSignerBytes32(
				facts.CurrentCheckpoint.StateImageDigest,
			),
			StateCommitment: nativeTBTCSignerBytes32(
				facts.CurrentCheckpoint.StateCommitment,
			),
		},
	}
	encoded, err := json.Marshal(wire)
	if err != nil {
		return nil, err
	}
	if _, err := DecodeNativeTBTCSignerStateAnchorBootstrapFacts(encoded); err != nil {
		return nil, err
	}
	return encoded, nil
}
