package signing

import (
	"encoding/json"
	"strings"
	"testing"
)

func testNativeTBTCSignerBootstrapFacts() *NativeTBTCSignerStateAnchorBootstrapFacts {
	store := [32]byte{0x5a}
	genesis := ComputeNativeTBTCSignerStateWitnessGenesis(store)
	image := [32]byte{0x5b}
	commitment := ComputeNativeTBTCSignerStateWitnessCommitment(
		store,
		1,
		genesis,
		image,
	)
	return &NativeTBTCSignerStateAnchorBootstrapFacts{
		Schema:           NativeTBTCSignerStateAnchorBootstrapFactsSchema,
		StoreFingerprint: store,
		CurrentCheckpoint: NativeTBTCSignerStateAnchorCheckpoint{
			StoreFingerprint:        store,
			Generation:              1,
			PreviousStateCommitment: genesis,
			StateImageDigest:        image,
			StateCommitment:         commitment,
		},
	}
}

func testNativeTBTCSignerBootstrapFactsWire(
	t *testing.T,
	facts *NativeTBTCSignerStateAnchorBootstrapFacts,
) nativeTBTCSignerStateAnchorBootstrapFactsWire {
	t.Helper()
	return nativeTBTCSignerStateAnchorBootstrapFactsWire{
		Schema:           facts.Schema,
		StoreFingerprint: nativeTBTCSignerBytes32(facts.StoreFingerprint),
		CurrentCheckpoint: nativeTBTCSignerStateAnchorCheckpointWire{
			StoreFingerprint: nativeTBTCSignerBytes32(
				facts.CurrentCheckpoint.StoreFingerprint,
			),
			Generation: uint64ToCanonicalString(
				facts.CurrentCheckpoint.Generation,
			),
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
}

func TestNativeTBTCSignerStateAnchorBootstrapFactsRoundTrip(t *testing.T) {
	facts := testNativeTBTCSignerBootstrapFacts()
	encoded, err := EncodeNativeTBTCSignerStateAnchorBootstrapFacts(facts)
	if err != nil {
		t.Fatalf("valid bootstrap facts were rejected by the encoder: %v", err)
	}
	decoded, err := DecodeNativeTBTCSignerStateAnchorBootstrapFacts(encoded)
	if err != nil {
		t.Fatalf("canonical bootstrap facts were rejected: %v", err)
	}
	if *decoded != *facts {
		t.Fatalf("bootstrap facts round trip diverged: %+v", decoded)
	}
}

func TestNativeTBTCSignerStateAnchorBootstrapFactsEncoderRejectsNonGenesis(
	t *testing.T,
) {
	if _, err := EncodeNativeTBTCSignerStateAnchorBootstrapFacts(nil); err == nil {
		t.Fatal("nil bootstrap facts were encoded")
	}

	nonGenesis := testNativeTBTCSignerBootstrapFacts()
	nonGenesis.CurrentCheckpoint.Generation = 2
	nonGenesis.CurrentCheckpoint.StateCommitment =
		ComputeNativeTBTCSignerStateWitnessCommitment(
			nonGenesis.CurrentCheckpoint.StoreFingerprint,
			2,
			nonGenesis.CurrentCheckpoint.PreviousStateCommitment,
			nonGenesis.CurrentCheckpoint.StateImageDigest,
		)
	if _, err := EncodeNativeTBTCSignerStateAnchorBootstrapFacts(
		nonGenesis,
	); err == nil {
		t.Fatal("generation-two bootstrap facts were encoded")
	}

	crossStore := testNativeTBTCSignerBootstrapFacts()
	crossStore.StoreFingerprint = [32]byte{0x5c}
	if _, err := EncodeNativeTBTCSignerStateAnchorBootstrapFacts(
		crossStore,
	); err == nil {
		t.Fatal("cross-store bootstrap facts were encoded")
	}
}

func TestNativeTBTCSignerStateAnchorBootstrapFactsStrictDecode(t *testing.T) {
	valid := testNativeTBTCSignerBootstrapFacts()
	canonical, err := EncodeNativeTBTCSignerStateAnchorBootstrapFacts(valid)
	if err != nil {
		t.Fatal(err)
	}
	otherStore := [32]byte{0x6a}
	otherGenesis := ComputeNativeTBTCSignerStateWitnessGenesis(otherStore)
	tests := map[string]struct {
		mutate func(*nativeTBTCSignerStateAnchorBootstrapFactsWire)
	}{
		"wrong schema": {
			mutate: func(wire *nativeTBTCSignerStateAnchorBootstrapFactsWire) {
				wire.Schema = "tbtc-signer-state-anchor-bootstrap-facts/v2"
			},
		},
		"non-canonical store fingerprint": {
			mutate: func(wire *nativeTBTCSignerStateAnchorBootstrapFactsWire) {
				wire.StoreFingerprint = strings.ToUpper(wire.StoreFingerprint)
			},
		},
		"missing store fingerprint": {
			mutate: func(wire *nativeTBTCSignerStateAnchorBootstrapFactsWire) {
				wire.StoreFingerprint = ""
			},
		},
		"zero store fingerprint": {
			mutate: func(wire *nativeTBTCSignerStateAnchorBootstrapFactsWire) {
				wire.StoreFingerprint = nativeTBTCSignerBytes32([32]byte{})
			},
		},
		"checkpoint store-fingerprint mismatch": {
			mutate: func(wire *nativeTBTCSignerStateAnchorBootstrapFactsWire) {
				checkpoint := testNativeTBTCSignerBootstrapFacts().CurrentCheckpoint
				checkpoint.StoreFingerprint = otherStore
				checkpoint.PreviousStateCommitment = otherGenesis
				checkpoint.StateCommitment =
					ComputeNativeTBTCSignerStateWitnessCommitment(
						otherStore,
						1,
						otherGenesis,
						checkpoint.StateImageDigest,
					)
				wire.CurrentCheckpoint.StoreFingerprint =
					nativeTBTCSignerBytes32(checkpoint.StoreFingerprint)
				wire.CurrentCheckpoint.PreviousStateCommitment =
					nativeTBTCSignerBytes32(checkpoint.PreviousStateCommitment)
				wire.CurrentCheckpoint.StateCommitment =
					nativeTBTCSignerBytes32(checkpoint.StateCommitment)
			},
		},
		"generation two": {
			mutate: func(wire *nativeTBTCSignerStateAnchorBootstrapFactsWire) {
				facts := testNativeTBTCSignerBootstrapFacts()
				wire.CurrentCheckpoint.Generation = "2"
				wire.CurrentCheckpoint.StateCommitment =
					nativeTBTCSignerBytes32(
						ComputeNativeTBTCSignerStateWitnessCommitment(
							facts.StoreFingerprint,
							2,
							facts.CurrentCheckpoint.PreviousStateCommitment,
							facts.CurrentCheckpoint.StateImageDigest,
						),
					)
			},
		},
		"non-genesis previous state commitment": {
			mutate: func(wire *nativeTBTCSignerStateAnchorBootstrapFactsWire) {
				facts := testNativeTBTCSignerBootstrapFacts()
				previous := [32]byte{0x6b}
				wire.CurrentCheckpoint.PreviousStateCommitment =
					nativeTBTCSignerBytes32(previous)
				wire.CurrentCheckpoint.StateCommitment =
					nativeTBTCSignerBytes32(
						ComputeNativeTBTCSignerStateWitnessCommitment(
							facts.StoreFingerprint,
							1,
							previous,
							facts.CurrentCheckpoint.StateImageDigest,
						),
					)
			},
		},
		"checkpoint commitment mismatch": {
			mutate: func(wire *nativeTBTCSignerStateAnchorBootstrapFactsWire) {
				wire.CurrentCheckpoint.StateCommitment =
					nativeTBTCSignerBytes32([32]byte{0x6c})
			},
		},
		"non-canonical generation": {
			mutate: func(wire *nativeTBTCSignerStateAnchorBootstrapFactsWire) {
				wire.CurrentCheckpoint.Generation = "01"
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			wire := testNativeTBTCSignerBootstrapFactsWire(t, valid)
			test.mutate(&wire)
			payload, err := json.Marshal(wire)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := DecodeNativeTBTCSignerStateAnchorBootstrapFacts(
				payload,
			); err == nil {
				t.Fatalf("bootstrap facts with %s were accepted", name)
			}
		})
	}

	payloadTests := map[string]string{
		"trailing data": string(canonical) + " {}",
		"unknown member": strings.Replace(
			string(canonical),
			`"schema"`,
			`"unknown":"x","schema"`,
			1,
		),
		"duplicate member": strings.Replace(
			string(canonical),
			`"schema"`,
			`"schema":"x","schema"`,
			1,
		),
		"case-folded duplicate member": strings.Replace(
			string(canonical),
			`"schema"`,
			`"Schema":"x","schema"`,
			1,
		),
		"depth bomb":    strings.Repeat("[", 40) + strings.Repeat("]", 40),
		"empty payload": "",
	}
	for name, payload := range payloadTests {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeNativeTBTCSignerStateAnchorBootstrapFacts(
				[]byte(payload),
			); err == nil {
				t.Fatalf("bootstrap facts payload with %s was accepted", name)
			}
		})
	}
}

func TestPreflightStrictNativeTBTCSignerJSONAcceptsCanonicalPayloads(
	t *testing.T,
) {
	valid := []string{
		`{}`,
		`[]`,
		`"scalar"`,
		`true`,
		`null`,
		`17`,
		`{"a":1,"b":[true,null,1.5,"x"],"c":{"d":"e"}}`,
		// Exactly at the depth bound: the innermost of 33 nested arrays is
		// scanned at depth 32.
		strings.Repeat("[", 33) + strings.Repeat("]", 33),
	}
	for _, payload := range valid {
		if err := preflightStrictNativeTBTCSignerJSON(
			[]byte(payload),
			0,
		); err != nil {
			t.Fatalf("canonical JSON %q was rejected: %v", payload, err)
		}
	}
}

func TestPreflightStrictNativeTBTCSignerJSONRejections(t *testing.T) {
	tests := map[string]string{
		"invalid JSON":                  `{`,
		"unterminated array":            `[1,`,
		"object trailing data":          `{} {}`,
		"scalar trailing data":          `1 2`,
		"garbage trailing data":         `{}x`,
		"duplicate member":              `{"a":1,"a":2}`,
		"case-folded duplicate member":  `{"a":1,"A":2}`,
		"nested duplicate member":       `{"a":{"b":1,"b":2}}`,
		"empty member name":             `{"":1}`,
		"member name with space":        `{"a b":1}`,
		"member name outside ASCII":     `{"ké":1}`,
		"member name with control char": "{\"a\\u0001b\":1}",
		"depth bomb": strings.Repeat("[", 34) +
			strings.Repeat("]", 34),
		"object depth bomb": strings.Repeat(`{"a":`, 34) + "1" +
			strings.Repeat("}", 34),
	}
	for name, payload := range tests {
		t.Run(name, func(t *testing.T) {
			if err := preflightStrictNativeTBTCSignerJSON(
				[]byte(payload),
				0,
			); err == nil {
				t.Fatalf("JSON with %s passed preflight", name)
			}
		})
	}

	if err := preflightStrictNativeTBTCSignerJSON([]byte(`{}`), 1); err == nil {
		t.Fatal("preflight accepted a non-root starting depth")
	}
}

func TestNativeTBTCSignerStrictJSONDecodePreflightIsWired(t *testing.T) {
	// decodeStrictNativeTBTCSignerJSON fronts every native-signer decoder;
	// case-folded aliases must be rejected before Go's case-insensitive
	// field matching can unify them.
	target := &struct {
		Schema string `json:"schema"`
	}{}
	if err := decodeStrictNativeTBTCSignerJSON(
		[]byte(`{"schema":"a","Schema":"b"}`),
		target,
		"test subject",
	); err == nil {
		t.Fatal("case-folded duplicate members were accepted")
	}
	if err := decodeStrictNativeTBTCSignerJSON(
		[]byte(`{"schema":"a"}`),
		target,
		"test subject",
	); err != nil {
		t.Fatalf("canonical strict JSON was rejected: %v", err)
	}
}
