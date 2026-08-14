package signing

import (
	"fmt"
	"strings"
	"testing"
)

func TestDecodeNativeTBTCSignerStateWitnessTip(t *testing.T) {
	storeFingerprint := [32]byte{1}
	previousCommitment := [32]byte{2}
	stateImageDigest := [32]byte{3}
	stateCommitment := ComputeNativeTBTCSignerStateWitnessCommitment(
		storeFingerprint,
		7,
		previousCommitment,
		stateImageDigest,
	)
	payload := nativeTBTCSignerStateWitnessTipTestPayload(
		storeFingerprint,
		7,
		previousCommitment,
		stateImageDigest,
		stateCommitment,
		7,
		stateCommitment,
		[32]byte{},
		"0",
		"0",
		[32]byte{},
		[32]byte{},
		"",
	)

	tip, err := DecodeNativeTBTCSignerStateWitnessTip([]byte(payload))
	if err != nil {
		t.Fatalf("cannot decode valid state-witness tip: %v", err)
	}
	if tip.Generation != 7 || tip.StateCommitment != stateCommitment ||
		tip.WitnessBaseGeneration != 7 ||
		tip.WitnessBaseCommitment != stateCommitment {
		t.Fatal("decoded state-witness tip differs from the wire payload")
	}
}

func TestDecodeNativeTBTCSignerStateWitnessTipRejectsNonCanonicalPayloads(
	t *testing.T,
) {
	storeFingerprint := [32]byte{1}
	previousCommitment := [32]byte{2}
	stateImageDigest := [32]byte{3}
	stateCommitment := ComputeNativeTBTCSignerStateWitnessCommitment(
		storeFingerprint,
		7,
		previousCommitment,
		stateImageDigest,
	)
	valid := func(extra string) string {
		return nativeTBTCSignerStateWitnessTipTestPayload(
			storeFingerprint,
			7,
			previousCommitment,
			stateImageDigest,
			stateCommitment,
			7,
			stateCommitment,
			[32]byte{},
			"0",
			"0",
			[32]byte{},
			[32]byte{},
			extra,
		)
	}

	tests := map[string]string{
		"leading-zero generation": strings.Replace(
			valid(""),
			`"generation":"7"`,
			`"generation":"07"`,
			1,
		),
		"numeric generation": strings.Replace(
			valid(""),
			`"generation":"7"`,
			`"generation":7`,
			1,
		),
		"overflow generation": strings.Replace(
			valid(""),
			`"generation":"7"`,
			`"generation":"18446744073709551616"`,
			1,
		),
		"uppercase bytes32": strings.Replace(
			valid(""),
			nativeTBTCSignerBytes32(storeFingerprint),
			strings.ToUpper(nativeTBTCSignerBytes32(storeFingerprint)),
			1,
		),
		"unknown field": valid(`,"future":true`),
		"trailing JSON": valid("") + `{}`,
		"commitment mismatch": strings.Replace(
			valid(""),
			nativeTBTCSignerBytes32(stateCommitment),
			nativeTBTCSignerBytes32([32]byte{9}),
			1,
		),
	}

	for name, payload := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeNativeTBTCSignerStateWitnessTip(
				[]byte(payload),
			); err == nil {
				t.Fatal("non-canonical state-witness tip was accepted")
			}
		})
	}
}

func TestDecodeNativeTBTCSignerStateWitnessTipRejectsPartialAnchorMetadata(
	t *testing.T,
) {
	storeFingerprint := [32]byte{1}
	previousCommitment := [32]byte{2}
	stateImageDigest := [32]byte{3}
	stateCommitment := ComputeNativeTBTCSignerStateWitnessCommitment(
		storeFingerprint,
		7,
		previousCommitment,
		stateImageDigest,
	)
	payload := nativeTBTCSignerStateWitnessTipTestPayload(
		storeFingerprint,
		7,
		previousCommitment,
		stateImageDigest,
		stateCommitment,
		7,
		stateCommitment,
		[32]byte{4},
		"0",
		"0",
		[32]byte{},
		[32]byte{},
		"",
	)
	if _, err := DecodeNativeTBTCSignerStateWitnessTip(
		[]byte(payload),
	); err == nil {
		t.Fatal("partial state-witness anchor metadata was accepted")
	}
}

func TestDecodeNativeTBTCSignerStateWitnessCheckpointAcknowledgementResult(
	t *testing.T,
) {
	storeFingerprint := [32]byte{1}
	stateCommitment := [32]byte{2}
	payload := fmt.Sprintf(
		`{"schema":"%s","acknowledged":true,"idempotent":false,"rotated":true,"storeFingerprint":"%s","generation":"9","stateCommitment":"%s","witnessBaseGeneration":"9","witnessBaseCommitment":"%s","anchorServiceEpoch":"2","anchorServiceRevision":"3","anchorEventRoot":"%s","anchorAcknowledgementDigest":"%s"}`,
		NativeTBTCSignerStateWitnessCheckpointAcknowledgementResultSchema,
		nativeTBTCSignerBytes32(storeFingerprint),
		nativeTBTCSignerBytes32(stateCommitment),
		nativeTBTCSignerBytes32(stateCommitment),
		nativeTBTCSignerBytes32([32]byte{3}),
		nativeTBTCSignerBytes32([32]byte{4}),
	)
	result, err :=
		DecodeNativeTBTCSignerStateWitnessCheckpointAcknowledgementResult(
			[]byte(payload),
		)
	if err != nil {
		t.Fatalf("cannot decode valid checkpoint acknowledgement result: %v", err)
	}
	if !result.Acknowledged || !result.Rotated || result.Idempotent ||
		result.Generation != 9 || result.AnchorServiceEpoch != 2 ||
		result.AnchorServiceRevision != 3 {
		t.Fatal("decoded checkpoint acknowledgement result differs from payload")
	}

	for name, invalid := range map[string]string{
		"missing acknowledged": strings.Replace(
			payload,
			`"acknowledged":true,`,
			"",
			1,
		),
		"numeric generation": strings.Replace(
			payload,
			`"generation":"9"`,
			`"generation":9`,
			1,
		),
		"unacknowledged": strings.Replace(
			payload,
			`"acknowledged":true`,
			`"acknowledged":false`,
			1,
		),
		"unknown field": strings.TrimSuffix(payload, "}") + `,"future":true}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err :=
				DecodeNativeTBTCSignerStateWitnessCheckpointAcknowledgementResult(
					[]byte(invalid),
				); err == nil {
				t.Fatal("invalid checkpoint acknowledgement result was accepted")
			}
		})
	}
}

func TestDecodeNativeTBTCSignerStateWitnessCheckpointRecoveryResult(
	t *testing.T,
) {
	storeFingerprint := [32]byte{1}
	stateCommitment := [32]byte{2}
	payload := fmt.Sprintf(
		`{"schema":"%s","recovered":true,"idempotent":false,"rotated":true,"storeFingerprint":"%s","generation":"9","stateCommitment":"%s","witnessBaseGeneration":"9","witnessBaseCommitment":"%s","anchorServiceEpoch":"2","anchorServiceRevision":"3","anchorEventRoot":"%s","anchorAcknowledgementDigest":"%s"}`,
		NativeTBTCSignerStateWitnessCheckpointRecoveryResultSchema,
		nativeTBTCSignerBytes32(storeFingerprint),
		nativeTBTCSignerBytes32(stateCommitment),
		nativeTBTCSignerBytes32(stateCommitment),
		nativeTBTCSignerBytes32([32]byte{3}),
		nativeTBTCSignerBytes32([32]byte{4}),
	)
	result, err :=
		DecodeNativeTBTCSignerStateWitnessCheckpointRecoveryResult(
			[]byte(payload),
		)
	if err != nil {
		t.Fatalf("cannot decode valid checkpoint recovery result: %v", err)
	}
	if !result.Recovered || !result.Rotated || result.Idempotent ||
		result.Generation != 9 || result.AnchorServiceEpoch != 2 ||
		result.AnchorServiceRevision != 3 {
		t.Fatal("decoded checkpoint recovery result differs from payload")
	}

	for name, invalid := range map[string]string{
		"missing recovered": strings.Replace(
			payload,
			`"recovered":true,`,
			"",
			1,
		),
		"not recovered": strings.Replace(
			payload,
			`"recovered":true`,
			`"recovered":false`,
			1,
		),
		"leading-zero revision": strings.Replace(
			payload,
			`"anchorServiceRevision":"3"`,
			`"anchorServiceRevision":"03"`,
			1,
		),
		"unknown field": strings.TrimSuffix(payload, "}") + `,"future":true}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err :=
				DecodeNativeTBTCSignerStateWitnessCheckpointRecoveryResult(
					[]byte(invalid),
				); err == nil {
				t.Fatal("invalid checkpoint recovery result was accepted")
			}
		})
	}
}

func nativeTBTCSignerStateWitnessTipTestPayload(
	storeFingerprint [32]byte,
	generation uint64,
	previousStateCommitment [32]byte,
	stateImageDigest [32]byte,
	stateCommitment [32]byte,
	witnessBaseGeneration uint64,
	witnessBaseCommitment [32]byte,
	anchorBindingHash [32]byte,
	anchorServiceEpoch string,
	anchorRevision string,
	anchorEventRoot [32]byte,
	anchorAcknowledgementDigest [32]byte,
	extra string,
) string {
	return fmt.Sprintf(
		`{"schema":"%s","storeFingerprint":"%s","generation":"%d","previousStateCommitment":"%s","stateImageDigest":"%s","stateCommitment":"%s","witnessBaseGeneration":"%d","witnessBaseCommitment":"%s","anchorBindingHash":"%s","anchorServiceEpoch":"%s","anchorRevision":"%s","anchorEventRoot":"%s","anchorAcknowledgementDigest":"%s"%s}`,
		NativeTBTCSignerStateWitnessTipSchema,
		nativeTBTCSignerBytes32(storeFingerprint),
		generation,
		nativeTBTCSignerBytes32(previousStateCommitment),
		nativeTBTCSignerBytes32(stateImageDigest),
		nativeTBTCSignerBytes32(stateCommitment),
		witnessBaseGeneration,
		nativeTBTCSignerBytes32(witnessBaseCommitment),
		nativeTBTCSignerBytes32(anchorBindingHash),
		anchorServiceEpoch,
		anchorRevision,
		nativeTBTCSignerBytes32(anchorEventRoot),
		nativeTBTCSignerBytes32(anchorAcknowledgementDigest),
		extra,
	)
}
