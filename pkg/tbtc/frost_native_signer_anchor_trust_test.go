package tbtc

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/decred/dcrd/dcrec/edwards/v2"
	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
)

func TestFrostNativeSignerAnchorTrustCertificateFrozenVector(t *testing.T) {
	certificate, _ := trustTestBootstrapCertificate(t)
	finalDigest, err :=
		ComputeFrostNativeSignerAnchorTrustFinalDigest(certificate)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := EncodeFrostNativeSignerAnchorTrustCertificate(certificate)
	if err != nil {
		t.Fatal(err)
	}
	encodedDigest := sha256.Sum256(encoded)
	vectors := []struct {
		name     string
		actual   []byte
		expected string
	}{
		{
			"coreDigest",
			certificate.CoreDigest[:],
			"d3b5ea2a8c29dc4f4ef5250fc75efb3fb6bcd87167d523661c4a55e7898fb90d",
		},
		{
			"coreSignature",
			certificate.CoreSignature[:],
			"9a24471862899050b13aa08e83b994a7d247ad4661afca78bfd890d7d49c9c14c5c07cfe01d55dedddbae6eb06304c3cb772b9ce3073c5240d12bab9802b6102",
		},
		{
			"operationID",
			certificate.OperationID[:],
			"49351644f18779575f614ab4b50c14c6454d8bd93d0a287edf41c801826edea0",
		},
		{
			"transitionDigest",
			certificate.TransitionDigest[:],
			"50e2372f4008f0bd47c4e17c517fe5b5dc6062736492a1414ada049878447979",
		},
		{
			"targetAcknowledgementSHA256",
			certificate.TargetAcknowledgementSHA256[:],
			"9605e695983da86a907c2a4b32ae322aa0c242fa0884d9049150007bc463c91b",
		},
		{
			"finalDigest",
			finalDigest[:],
			"0ae35d27aa3b74817a5dd99dcb1355975a74df6d5bcdda14288382eb8067535e",
		},
		{
			"finalSignature",
			certificate.FinalSignature[:],
			"16bc2097a17956f2888a6a370c8ad31e76c0f30ca2fef66d7c2b6ccacccb0cf6f660a10e920d1a5d9fe4d06259ee87f9555f2be330aab19d13fbdceb8bdf6a02",
		},
		{
			"certificateDigest",
			certificate.CertificateDigest[:],
			"059967c2178a72c178e894fe54ac74fccd657aec73f3b2ce48b9e68bae098a0b",
		},
		{
			"canonicalCertificateJSONSHA256",
			encodedDigest[:],
			"ea80af7129eb2a52d3116007d9caab2f5bf9df2edbc7d29811ff275b7f6d0412",
		},
	}
	for _, vector := range vectors {
		actual := hex.EncodeToString(vector.actual)
		if actual != vector.expected {
			t.Errorf(
				"%s vector mismatch: got [%s], want [%s]",
				vector.name,
				actual,
				vector.expected,
			)
		}
	}
}

func TestFrostNativeSignerAnchorTrustCertificateRoundTripAndValidation(
	t *testing.T,
) {
	certificate, _ := trustTestBootstrapCertificate(t)
	encoded, err := EncodeFrostNativeSignerAnchorTrustCertificate(certificate)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeFrostNativeSignerAnchorTrustCertificate(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(certificate, decoded) {
		t.Fatal("trust certificate round trip changed its exact material")
	}
	expectedAfterCallback := trustTestCloneCertificate(decoded)
	calls := 0
	err = ValidateFrostNativeSignerAnchorTrustCertificate(
		decoded,
		func(
			validated *FrostNativeSignerAnchorTrustCertificate,
			raw []byte,
		) error {
			calls++
			if validated == decoded ||
				!bytes.Equal(raw, decoded.TargetAcknowledgement) {
				t.Fatal("target acknowledgement validator received changed material")
			}
			validated.To.BindingHash[0] ^= 0xff
			validated.TargetAcknowledgement[0] ^= 0xff
			raw[0] ^= 0xff
			return nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("target acknowledgement validator called [%d] times", calls)
	}
	if !reflect.DeepEqual(decoded, expectedAfterCallback) {
		t.Fatal("target acknowledgement callback mutated the validated certificate")
	}
}

func TestFrostNativeSignerAnchorTrustVectorsCarryValidTargetAcknowledgements(
	t *testing.T,
) {
	bootstrap, authority := trustTestBootstrapCertificate(t)
	rotation := trustTestRotationCertificate(t, bootstrap, authority, 0x33)
	for name, certificate := range map[string]*FrostNativeSignerAnchorTrustCertificate{
		"bootstrap": bootstrap,
		"rotation":  rotation,
	} {
		t.Run(name, func(t *testing.T) {
			if err := ValidateFrostNativeSignerAnchorTrustCertificate(
				certificate,
				ValidateFrostNativeSignerAnchorTrustTargetAcknowledgement,
			); err != nil {
				t.Fatalf("valid shared %s vector was rejected: %v", name, err)
			}
		})
	}
}

func TestFrostNativeSignerAnchorTrustSharedValidVectors(t *testing.T) {
	bootstrap, authority := trustTestBootstrapCertificate(t)
	rotation := trustTestRotationCertificate(t, bootstrap, authority, 0x33)
	adoption := trustTestRotationCertificate(t, bootstrap, authority, 0x44)
	adoption.CertificateSequence = 1
	adoption.PreviousCertificateDigest = [32]byte{}
	adoptionResponse := ed25519.NewKeyFromSeed(
		bytes.Repeat([]byte{0x44}, ed25519.SeedSize),
	)
	trustTestFinalizeCertificateWithAcknowledgement(
		t,
		adoption,
		authority,
		adoptionResponse,
		0x44,
	)

	vectors := map[string]struct {
		certificate       *FrostNativeSignerAnchorTrustCertificate
		core              string
		operation         string
		transition        string
		final             string
		certificateDigest string
		jsonSHA256        string
	}{
		"bootstrap": {
			bootstrap,
			"d3b5ea2a8c29dc4f4ef5250fc75efb3fb6bcd87167d523661c4a55e7898fb90d",
			"49351644f18779575f614ab4b50c14c6454d8bd93d0a287edf41c801826edea0",
			"50e2372f4008f0bd47c4e17c517fe5b5dc6062736492a1414ada049878447979",
			"0ae35d27aa3b74817a5dd99dcb1355975a74df6d5bcdda14288382eb8067535e",
			"059967c2178a72c178e894fe54ac74fccd657aec73f3b2ce48b9e68bae098a0b",
			"ea80af7129eb2a52d3116007d9caab2f5bf9df2edbc7d29811ff275b7f6d0412",
		},
		"rotation": {
			rotation,
			"87ce928827c85744e9a01422c83e72ea1cf6f27c2694e4b1d93f519f8a905866",
			"39f50c5787e4decb56b87979062aa7e75f773b1693e584e8872a860da55fec93",
			"1a130d6da974f81cdbfdd923d9e53cf46cd9d81455b3c75e4cad1fe9c15b4385",
			"19f98af5fb9e017470594782c632eeaabf352c681523c626d6f6bb59b43318a2",
			"0d571f120304487645bad248d866f7455ea50bf9b2bc6521de47151b7d671f35",
			"8f834f9b335854218f07fcd2af0fafb311ac046b026f5436c7fe89ff24c9ade8",
		},
		"adoption": {
			adoption,
			"b8028e79579f400c80cd76101473e9a8e02be0644c5c4544a7a582103d5320a4",
			"c4dc774b78b391f173756130abb5412e62958f174783302396df673cfb70c30d",
			"bc3bc28337f870c07f41353d597ea38c96a2dd7f9f2f80536404e1e043296991",
			"aa69c3894ad0118c71189352565122950d2dfa5a20337f48c53c750dddae5969",
			"a8af21e3da145313f4f0357aa5de40a99e01af9faa8ff2cb680f9531ccb271dd",
			"e8b0fe13efbb0c667576411836c2f6ac22df1eb70904cb0d796ae0b3dfc06cdb",
		},
	}
	for name, vector := range vectors {
		t.Run(name, func(t *testing.T) {
			certificate := vector.certificate
			if err := ValidateFrostNativeSignerAnchorTrustCertificate(
				certificate,
				ValidateFrostNativeSignerAnchorTrustTargetAcknowledgement,
			); err != nil {
				t.Fatal(err)
			}
			encoded, err :=
				EncodeFrostNativeSignerAnchorTrustCertificate(certificate)
			if err != nil {
				t.Fatal(err)
			}
			finalDigest, err :=
				ComputeFrostNativeSignerAnchorTrustFinalDigest(certificate)
			if err != nil {
				t.Fatal(err)
			}
			jsonDigest := sha256.Sum256(encoded)
			actual := []string{
				hex.EncodeToString(certificate.CoreDigest[:]),
				hex.EncodeToString(certificate.OperationID[:]),
				hex.EncodeToString(certificate.TransitionDigest[:]),
				hex.EncodeToString(finalDigest[:]),
				hex.EncodeToString(certificate.CertificateDigest[:]),
				hex.EncodeToString(jsonDigest[:]),
			}
			expected := []string{
				vector.core,
				vector.operation,
				vector.transition,
				vector.final,
				vector.certificateDigest,
				vector.jsonSHA256,
			}
			if !reflect.DeepEqual(actual, expected) {
				t.Fatalf(
					"shared %s vector mismatch:\nactual:   %v\nexpected: %v",
					name,
					actual,
					expected,
				)
			}
			t.Logf("canonicalJSON=%s", encoded)
		})
	}
	chainOptions := trustTestChainOptions(rotation)
	chainOptions.ValidateTargetAcknowledgement =
		ValidateFrostNativeSignerAnchorTrustTargetAcknowledgement
	if err := ValidateFrostNativeSignerAnchorTrustCertificateChain(
		[]FrostNativeSignerAnchorTrustCertificate{
			*bootstrap,
			*rotation,
		},
		chainOptions,
	); err != nil {
		t.Fatalf("shared bootstrap/rotation chain is invalid: %v", err)
	}
	adoptionOptions := trustTestChainOptions(adoption)
	adoptionOptions.AllowLegacyAdoption = true
	adoptionOptions.ValidateTargetAcknowledgement =
		ValidateFrostNativeSignerAnchorTrustTargetAcknowledgement
	if err := ValidateFrostNativeSignerAnchorTrustCertificateChain(
		[]FrostNativeSignerAnchorTrustCertificate{*adoption},
		adoptionOptions,
	); err != nil {
		t.Fatalf("shared legacy-adoption chain is invalid: %v", err)
	}
}

func TestFrostNativeSignerAnchorTrustCertificateStrictDecode(t *testing.T) {
	certificate, _ := trustTestBootstrapCertificate(t)
	encoded, err := EncodeFrostNativeSignerAnchorTrustCertificate(certificate)
	if err != nil {
		t.Fatal(err)
	}
	canonical := string(encoded)

	withoutFrom := strings.Replace(canonical, `"from":null,`, "", 1)
	duplicateReference := strings.Replace(
		canonical,
		`"reference":{`,
		`"reference":{"revision":"1","revision":"1",`,
		1,
	)
	caseAlias := strings.Replace(canonical, `"schema":`, `"Schema":`, 1)
	unknown := strings.Replace(canonical, `"kind":`, `"unknown":"x","kind":`, 1)
	nonCanonicalSequence := strings.Replace(
		canonical,
		`"certificateSequence":"1"`,
		`"certificateSequence":"01"`,
		1,
	)
	upperCoreDigest := trustTestUppercaseHexMember(
		t,
		canonical,
		"coreDigest",
	)
	rawBase64 := strings.Replace(
		canonical,
		base64.StdEncoding.EncodeToString(certificate.TargetAcknowledgement),
		strings.TrimRight(
			base64.StdEncoding.EncodeToString(certificate.TargetAcknowledgement),
			"=",
		),
		1,
	)
	coreSignatureBase64 := base64.StdEncoding.EncodeToString(
		certificate.CoreSignature[:],
	)
	rawCoreSignature := strings.Replace(
		canonical,
		coreSignatureBase64,
		strings.TrimRight(coreSignatureBase64, "="),
		1,
	)
	trailing := canonical + `{}`
	nonASCIIKey := strings.Replace(canonical, `"kind":`, `"kınd":`, 1)

	cases := map[string]string{
		"missing from":          withoutFrom,
		"duplicate nested":      duplicateReference,
		"case alias":            caseAlias,
		"unknown":               unknown,
		"noncanonical sequence": nonCanonicalSequence,
		"uppercase hex":         upperCoreDigest,
		"raw base64":            rawBase64,
		"raw signature base64":  rawCoreSignature,
		"trailing object":       trailing,
		"non-ASCII key":         nonASCIIKey,
	}
	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeFrostNativeSignerAnchorTrustCertificate(
				[]byte(payload),
			); err == nil {
				t.Fatal("malformed trust certificate was accepted")
			}
		})
	}
}

func TestFrostNativeSignerAnchorTrustCertificateRejectsParserAndCryptoBypasses(
	t *testing.T,
) {
	valid, _ := trustTestBootstrapCertificate(t)
	tests := map[string]func(*FrostNativeSignerAnchorTrustCertificate){
		"core digest": func(certificate *FrostNativeSignerAnchorTrustCertificate) {
			certificate.CoreDigest[0] ^= 1
		},
		"core signature": func(certificate *FrostNativeSignerAnchorTrustCertificate) {
			certificate.CoreSignature[0] ^= 1
		},
		"operation ID": func(certificate *FrostNativeSignerAnchorTrustCertificate) {
			certificate.OperationID[0] ^= 1
		},
		"transition digest": func(certificate *FrostNativeSignerAnchorTrustCertificate) {
			certificate.TransitionDigest[0] ^= 1
		},
		"acknowledgement hash": func(certificate *FrostNativeSignerAnchorTrustCertificate) {
			certificate.TargetAcknowledgementSHA256[0] ^= 1
		},
		"final signature": func(certificate *FrostNativeSignerAnchorTrustCertificate) {
			certificate.FinalSignature[0] ^= 1
		},
		"certificate digest": func(certificate *FrostNativeSignerAnchorTrustCertificate) {
			certificate.CertificateDigest[0] ^= 1
		},
		"response SPKI": func(certificate *FrostNativeSignerAnchorTrustCertificate) {
			certificate.To.ResponsePublicKeySPKISHA256[0] ^= 1
		},
		"authority SPKI": func(certificate *FrostNativeSignerAnchorTrustCertificate) {
			certificate.To.OfflineAuthoritySPKISHA256[0] ^= 1
		},
		"oversized acknowledgement": func(certificate *FrostNativeSignerAnchorTrustCertificate) {
			certificate.TargetAcknowledgement = bytes.Repeat(
				[]byte{'x'},
				frostNativeSignerAnchorTrustMaximumAcknowledgementBytes+1,
			)
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			certificate := trustTestCloneCertificate(valid)
			mutate(certificate)
			calls := 0
			err := ValidateFrostNativeSignerAnchorTrustCertificate(
				certificate,
				func(
					_ *FrostNativeSignerAnchorTrustCertificate,
					_ []byte,
				) error {
					calls++
					return nil
				},
			)
			if err == nil {
				t.Fatal("tampered trust certificate was accepted")
			}
			if calls != 0 {
				t.Fatal("semantic callback ran before structural/crypto validation")
			}
		})
	}
	if err := ValidateFrostNativeSignerAnchorTrustCertificate(
		valid,
		func(
			_ *FrostNativeSignerAnchorTrustCertificate,
			_ []byte,
		) error {
			return errors.New("rejected")
		},
	); err == nil {
		t.Fatal("target acknowledgement callback rejection was ignored")
	}
	if err := ValidateFrostNativeSignerAnchorTrustCertificate(valid, nil); err == nil {
		t.Fatal("missing target acknowledgement callback was accepted")
	}
}

func TestFrostNativeSignerAnchorTrustCertificateRejectsInvalidEd25519Points(
	t *testing.T,
) {
	valid, authority := trustTestBootstrapCertificate(t)
	for _, field := range []string{"response", "authority"} {
		t.Run(field, func(t *testing.T) {
			certificate := trustTestCloneCertificate(valid)
			invalid := [ed25519.PublicKeySize]byte{}
			for index := range invalid {
				invalid[index] = 0xff
			}
			switch field {
			case "response":
				certificate.To.ResponsePublicKey = invalid
				certificate.To.ResponsePublicKeySPKISHA256 =
					ComputeFrostNativeSignerAnchorTrustEd25519SPKISHA256(
						invalid,
					)
			case "authority":
				certificate.To.OfflineAuthorityPublicKey = invalid
				certificate.To.OfflineAuthoritySPKISHA256 =
					ComputeFrostNativeSignerAnchorTrustEd25519SPKISHA256(
						invalid,
					)
			}
			trustTestFinalizeCertificate(t, certificate, authority)
			if err := ValidateFrostNativeSignerAnchorTrustCertificate(
				certificate,
				func(
					_ *FrostNativeSignerAnchorTrustCertificate,
					_ []byte,
				) error {
					return nil
				},
			); err == nil || !strings.Contains(err.Error(), "point") {
				t.Fatalf("invalid %s Ed25519 point was accepted: %v", field, err)
			}
		})
	}
}

func TestFrostNativeSignerAnchorTrustEd25519PublicKeyRejectsCanonicalNonPrimeOrderPoints(
	t *testing.T,
) {
	validPrivate := ed25519.NewKeyFromSeed(
		bytes.Repeat([]byte{0x5a}, ed25519.SeedSize),
	)
	validPublic := validPrivate.Public().(ed25519.PublicKey)
	if err := ValidateFrostNativeSignerAnchorTrustEd25519PublicKey(
		validPublic,
	); err != nil {
		t.Fatalf("valid prime-subgroup key was rejected: [%v]", err)
	}

	identity := make([]byte, ed25519.PublicKeySize)
	identity[0] = 1
	orderFourTorsion := make([]byte, ed25519.PublicKeySize)
	mixedOrder, err := hex.DecodeString(
		"9970c93c125fd998ebc1642abe30619e2fd971dbcbeaeb8ccfe919cbfd13b6cf",
	)
	if err != nil {
		t.Fatal(err)
	}
	for name, publicKey := range map[string][]byte{
		"identity":                       identity,
		"order-four torsion":             orderFourTorsion,
		"prime-plus-torsion mixed order": mixedOrder,
	} {
		t.Run(name, func(t *testing.T) {
			point, err := edwards.ParsePubKey(publicKey)
			if err != nil || point == nil ||
				!bytes.Equal(point.Serialize(), publicKey) {
				t.Fatalf(
					"rejection vector is not a canonical Edwards25519 encoding: [%v]",
					err,
				)
			}
			if err := ValidateFrostNativeSignerAnchorTrustEd25519PublicKey(
				publicKey,
			); err == nil ||
				(!strings.Contains(err.Error(), "identity") &&
					!strings.Contains(err.Error(), "subgroup")) {
				t.Fatalf("small-order key was accepted: [%v]", err)
			}
		})
	}

	certificate, _ := trustTestBootstrapCertificate(t)
	copy(certificate.To.ResponsePublicKey[:], identity)
	if _, err := verifyFrostNativeSignerAnchorTrustTargetAcknowledgement(
		certificate,
		[]byte(`{}`),
	); err == nil || !strings.Contains(err.Error(), "response key") {
		t.Fatalf(
			"exported acknowledgement verification reached signature parsing "+
				"with an identity key: [%v]",
			err,
		)
	}
}

func TestFrostNativeSignerAnchorTrustCertificateChain(t *testing.T) {
	bootstrap, authority := trustTestBootstrapCertificate(t)
	rotation := trustTestRotationCertificate(t, bootstrap, authority, 0x33)
	chain := []FrostNativeSignerAnchorTrustCertificate{
		*bootstrap,
		*rotation,
	}
	calls := 0
	options := FrostNativeSignerAnchorTrustChainValidationOptions{
		ExpectedProtocolID:                 bootstrap.ProtocolID,
		ExpectedStreamID:                   bootstrap.StreamID,
		ExpectedSignerStoreFingerprint:     bootstrap.SignerStoreFingerprint,
		ExpectedOfflineAuthorityPublicKey:  bootstrap.To.OfflineAuthorityPublicKey,
		ExpectedOfflineAuthoritySPKISHA256: bootstrap.To.OfflineAuthoritySPKISHA256,
		ExpectedHead:                       trustTestCertificateHead(rotation),
		ValidateTargetAcknowledgement: func(
			_ *FrostNativeSignerAnchorTrustCertificate,
			_ []byte,
		) error {
			calls++
			return nil
		},
	}
	if err := ValidateFrostNativeSignerAnchorTrustCertificateChain(
		chain,
		options,
	); err != nil {
		t.Fatal(err)
	}
	if calls != len(chain) {
		t.Fatalf("validated [%d] acknowledgements, expected [%d]", calls, len(chain))
	}

	tests := map[string]func(
		[]FrostNativeSignerAnchorTrustCertificate,
		*FrostNativeSignerAnchorTrustChainValidationOptions,
	){
		"previous digest": func(
			chain []FrostNativeSignerAnchorTrustCertificate,
			_ *FrostNativeSignerAnchorTrustChainValidationOptions,
		) {
			chain[1].PreviousCertificateDigest[0] ^= 1
		},
		"from endpoint": func(
			chain []FrostNativeSignerAnchorTrustCertificate,
			_ *FrostNativeSignerAnchorTrustChainValidationOptions,
		) {
			chain[1].From.BindingHash[0] ^= 1
		},
		"protocol substitution": func(
			chain []FrostNativeSignerAnchorTrustCertificate,
			_ *FrostNativeSignerAnchorTrustChainValidationOptions,
		) {
			chain[1].ProtocolID[0] ^= 1
		},
		"head digest": func(
			_ []FrostNativeSignerAnchorTrustCertificate,
			options *FrostNativeSignerAnchorTrustChainValidationOptions,
		) {
			options.ExpectedHead.CertificateDigest[0] ^= 1
		},
		"authority pin": func(
			_ []FrostNativeSignerAnchorTrustCertificate,
			options *FrostNativeSignerAnchorTrustChainValidationOptions,
		) {
			options.ExpectedOfflineAuthorityPublicKey[0] ^= 1
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			candidate := trustTestCloneChain(chain)
			candidateOptions := trustTestCloneChainOptions(options)
			candidateOptions.ValidateTargetAcknowledgement = func(
				_ *FrostNativeSignerAnchorTrustCertificate,
				_ []byte,
			) error {
				return nil
			}
			mutate(candidate, &candidateOptions)
			if err := ValidateFrostNativeSignerAnchorTrustCertificateChain(
				candidate,
				candidateOptions,
			); err == nil {
				t.Fatal("forked or incorrectly pinned certificate chain was accepted")
			}
		})
	}
}

func TestFrostNativeSignerAnchorTrustCertificateMissingSuffix(t *testing.T) {
	bootstrap, authority := trustTestBootstrapCertificate(t)
	second := trustTestRotationCertificate(t, bootstrap, authority, 0x34)
	third := trustTestRotationCertificate(t, second, authority, 0x35)
	validSuffix := []FrostNativeSignerAnchorTrustCertificate{*third}
	options := trustTestChainOptions(third)
	options.PriorHead = trustTestCertificateHead(second)
	if err := ValidateFrostNativeSignerAnchorTrustCertificateChain(
		validSuffix,
		options,
	); err != nil {
		t.Fatalf("valid authenticated missing suffix was rejected: %v", err)
	}

	tests := map[string]func(
		[]FrostNativeSignerAnchorTrustCertificate,
		*FrostNativeSignerAnchorTrustChainValidationOptions,
	){
		"gap": func(
			chain []FrostNativeSignerAnchorTrustCertificate,
			options *FrostNativeSignerAnchorTrustChainValidationOptions,
		) {
			chain[0].CertificateSequence++
			trustTestFinalizeCertificate(t, &chain[0], authority)
			options.ExpectedHead = trustTestCertificateHead(&chain[0])
		},
		"fork": func(
			chain []FrostNativeSignerAnchorTrustCertificate,
			options *FrostNativeSignerAnchorTrustChainValidationOptions,
		) {
			chain[0].PreviousCertificateDigest[0] ^= 1
			trustTestFinalizeCertificate(t, &chain[0], authority)
			options.ExpectedHead = trustTestCertificateHead(&chain[0])
		},
		"from mismatch": func(
			chain []FrostNativeSignerAnchorTrustCertificate,
			options *FrostNativeSignerAnchorTrustChainValidationOptions,
		) {
			chain[0].From.BindingHash[0] ^= 1
			trustTestFinalizeCertificate(t, &chain[0], authority)
			options.ExpectedHead = trustTestCertificateHead(&chain[0])
		},
		"missing prior head": func(
			_ []FrostNativeSignerAnchorTrustCertificate,
			options *FrostNativeSignerAnchorTrustChainValidationOptions,
		) {
			options.PriorHead = nil
		},
		"wrong prior endpoint": func(
			_ []FrostNativeSignerAnchorTrustCertificate,
			options *FrostNativeSignerAnchorTrustChainValidationOptions,
		) {
			options.PriorHead.Endpoint.BindingHash[0] ^= 1
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			chain := trustTestCloneChain(validSuffix)
			candidateOptions := trustTestCloneChainOptions(options)
			calls := 0
			candidateOptions.ValidateTargetAcknowledgement = func(
				_ *FrostNativeSignerAnchorTrustCertificate,
				_ []byte,
			) error {
				calls++
				return nil
			}
			mutate(chain, &candidateOptions)
			if err := ValidateFrostNativeSignerAnchorTrustCertificateChain(
				chain,
				candidateOptions,
			); err == nil {
				t.Fatal("invalid missing suffix was accepted")
			}
			if calls != 0 {
				t.Fatal("semantic callback ran before suffix anchoring")
			}
		})
	}
}

func TestFrostNativeSignerAnchorTrustCertificateAllowsPostSigningRotation(
	t *testing.T,
) {
	bootstrap, authority := trustTestBootstrapCertificate(t)
	rotation := trustTestRotationCertificate(t, bootstrap, authority, 0x39)
	fromCheckpoint := trustTestCheckpoint(
		bootstrap.SignerStoreFingerprint,
		bootstrap.To.Reference.Checkpoint.Generation+3,
		trustTestBytes32(0x91),
		trustTestBytes32(0x92),
	)
	trustTestSetRotationFromDescendant(
		t,
		rotation,
		authority,
		7,
		fromCheckpoint,
	)

	fullChain := []FrostNativeSignerAnchorTrustCertificate{
		*bootstrap,
		*rotation,
	}
	if err := ValidateFrostNativeSignerAnchorTrustCertificateChain(
		fullChain,
		trustTestChainOptions(rotation),
	); err != nil {
		t.Fatalf("post-signing rotation in a full chain was rejected: %v", err)
	}

	options := trustTestChainOptions(rotation)
	options.PriorHead = trustTestCertificateHead(bootstrap)
	if err := ValidateFrostNativeSignerAnchorTrustCertificateChain(
		[]FrostNativeSignerAnchorTrustCertificate{*rotation},
		options,
	); err != nil {
		t.Fatalf("post-signing rotation suffix was rejected: %v", err)
	}
}

func TestFrostNativeSignerAnchorTrustReferenceDescendantRules(t *testing.T) {
	bootstrap, _ := trustTestBootstrapCertificate(t)
	floor := bootstrap.To.Reference
	valid := floor
	valid.Revision = 2
	valid.PreviousEventRoot = floor.EventRoot
	valid.EventRoot = trustTestBytes32(0xa1)
	valid.AcknowledgementDigest = trustTestBytes32(0xa2)
	if err := frostNativeSignerAnchorTrustValidateReferenceDescendant(
		floor,
		valid,
	); err != nil {
		t.Fatalf("valid later reference was rejected: %v", err)
	}
	atBound := valid
	atBound.Revision =
		floor.Revision + FrostNativeSignerAnchorMaximumHistoryEvents
	if err := frostNativeSignerAnchorTrustValidateReferenceDescendant(
		floor,
		atBound,
	); err != nil {
		t.Fatalf("last restartable reference was rejected: %v", err)
	}
	beyondBound := atBound
	beyondBound.Revision++
	if err := frostNativeSignerAnchorTrustValidateReferenceDescendant(
		floor,
		beyondBound,
	); err == nil {
		t.Fatal("reference beyond the restartable history bound was accepted")
	}

	tests := map[string]func(*FrostNativeSignerAnchorTrustReference){
		"equal revision fork": func(candidate *FrostNativeSignerAnchorTrustReference) {
			*candidate = floor
			candidate.EventRoot[0] ^= 1
		},
		"wrong epoch": func(candidate *FrostNativeSignerAnchorTrustReference) {
			candidate.ServiceEpoch++
		},
		"missing parent": func(candidate *FrostNativeSignerAnchorTrustReference) {
			candidate.PreviousEventRoot = [32]byte{}
		},
		"generation rollback": func(candidate *FrostNativeSignerAnchorTrustReference) {
			candidate.Checkpoint = trustTestCheckpoint(
				floor.Checkpoint.StoreFingerprint,
				floor.Checkpoint.Generation-1,
				trustTestBytes32(0xa3),
				trustTestBytes32(0xa4),
			)
		},
		"equal generation checkpoint fork": func(candidate *FrostNativeSignerAnchorTrustReference) {
			candidate.Checkpoint = trustTestCheckpoint(
				floor.Checkpoint.StoreFingerprint,
				floor.Checkpoint.Generation,
				trustTestBytes32(0xa5),
				trustTestBytes32(0xa6),
			)
		},
		// Mirrors the Rust descendant validator, which refuses a store
		// fingerprint change across generations. The two trees must admit the
		// same references or one accepts a chain the other refuses on every
		// store open.
		"store fingerprint change": func(candidate *FrostNativeSignerAnchorTrustReference) {
			rehomed := floor.Checkpoint.StoreFingerprint
			rehomed[0] ^= 1
			candidate.Checkpoint = trustTestCheckpoint(
				rehomed,
				floor.Checkpoint.Generation+1,
				trustTestBytes32(0xa7),
				trustTestBytes32(0xa8),
			)
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			candidate := valid
			mutate(&candidate)
			if err := frostNativeSignerAnchorTrustValidateReferenceDescendant(
				floor,
				candidate,
			); err == nil {
				t.Fatal("invalid certified-floor descendant was accepted")
			}
		})
	}
}

func TestFrostNativeSignerAnchorTrustCertificateExactReplay(t *testing.T) {
	bootstrap, _ := trustTestBootstrapCertificate(t)
	head := trustTestCertificateHead(bootstrap)
	options := trustTestChainOptions(bootstrap)
	options.PriorHead = head
	chain := []FrostNativeSignerAnchorTrustCertificate{*bootstrap}
	if err := ValidateFrostNativeSignerAnchorTrustCertificateChain(
		chain,
		options,
	); err != nil {
		t.Fatalf("exact idempotent certificate replay was rejected: %v", err)
	}

	olderHead := *head
	olderHead.CertificateDigest[0] ^= 1
	options.PriorHead = &olderHead
	if err := ValidateFrostNativeSignerAnchorTrustCertificateChain(
		chain,
		options,
	); err == nil {
		t.Fatal("non-exact replay head was accepted")
	}
}

func TestAuthenticateFrostNativeSignerAnchorTrustChainMintsDefensiveCapability(
	t *testing.T,
) {
	bootstrap, _ := trustTestBootstrapCertificate(t)
	chain := []FrostNativeSignerAnchorTrustCertificate{*bootstrap}
	capability, err :=
		authenticateFrostNativeSignerAnchorTrustCertificateChain(
			chain,
			trustTestChainOptions(bootstrap),
		)
	if err != nil {
		t.Fatalf("valid trust chain did not mint a capability: %v", err)
	}
	if capability == nil ||
		capability.certificate.CertificateDigest !=
			bootstrap.CertificateDigest {
		t.Fatal("verified trust-floor capability is incomplete")
	}
	expectedAcknowledgementByte := bootstrap.TargetAcknowledgement[0]
	expectedDigest := bootstrap.CertificateDigest
	chain[0].TargetAcknowledgement[0] ^= 1
	chain[0].CertificateDigest[0] ^= 1
	if capability.certificate.TargetAcknowledgement[0] !=
		expectedAcknowledgementByte ||
		capability.certificate.CertificateDigest !=
			expectedDigest {
		t.Fatal("verified trust-floor capability aliases mutable input")
	}

	invalid := *trustTestCloneCertificate(bootstrap)
	invalid.FinalSignature[0] ^= 1
	if _, err := authenticateFrostNativeSignerAnchorTrustCertificateChain(
		[]FrostNativeSignerAnchorTrustCertificate{invalid},
		trustTestChainOptions(bootstrap),
	); err == nil {
		t.Fatal("invalid authority signature minted a trust-floor capability")
	}
}

func TestFrostNativeSignerAnchorTrustChainRequiresEveryPin(t *testing.T) {
	bootstrap, _ := trustTestBootstrapCertificate(t)
	chain := []FrostNativeSignerAnchorTrustCertificate{*bootstrap}
	validOptions := trustTestChainOptions(bootstrap)
	tests := map[string]func(*FrostNativeSignerAnchorTrustChainValidationOptions){
		"protocol": func(options *FrostNativeSignerAnchorTrustChainValidationOptions) {
			options.ExpectedProtocolID = [32]byte{}
		},
		"stream": func(options *FrostNativeSignerAnchorTrustChainValidationOptions) {
			options.ExpectedStreamID = [32]byte{}
		},
		"store": func(options *FrostNativeSignerAnchorTrustChainValidationOptions) {
			options.ExpectedSignerStoreFingerprint = [32]byte{}
		},
		"authority key": func(options *FrostNativeSignerAnchorTrustChainValidationOptions) {
			options.ExpectedOfflineAuthorityPublicKey =
				[ed25519.PublicKeySize]byte{}
		},
		"authority hash": func(options *FrostNativeSignerAnchorTrustChainValidationOptions) {
			options.ExpectedOfflineAuthoritySPKISHA256 = [32]byte{}
		},
		"head": func(options *FrostNativeSignerAnchorTrustChainValidationOptions) {
			options.ExpectedHead = nil
		},
		"head sequence": func(options *FrostNativeSignerAnchorTrustChainValidationOptions) {
			options.ExpectedHead.CertificateSequence = 0
		},
		"head digest": func(options *FrostNativeSignerAnchorTrustChainValidationOptions) {
			options.ExpectedHead.CertificateDigest = [32]byte{}
		},
		"head certified revision": func(options *FrostNativeSignerAnchorTrustChainValidationOptions) {
			options.ExpectedHead.Endpoint.Reference.Revision = 2
		},
		"validator": func(options *FrostNativeSignerAnchorTrustChainValidationOptions) {
			options.ValidateTargetAcknowledgement = nil
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			options := trustTestCloneChainOptions(validOptions)
			mutate(&options)
			if err := ValidateFrostNativeSignerAnchorTrustCertificateChain(
				chain,
				options,
			); err == nil {
				t.Fatal("zero production trust pin was accepted")
			}
		})
	}
}

func TestFrostNativeSignerAnchorTrustLegacyAdoptionIsExplicit(t *testing.T) {
	bootstrap, authority := trustTestBootstrapCertificate(t)
	legacy := trustTestRotationCertificate(t, bootstrap, authority, 0x44)
	legacy.CertificateSequence = 1
	legacy.PreviousCertificateDigest = [32]byte{}
	trustTestFinalizeCertificate(t, legacy, authority)
	chain := []FrostNativeSignerAnchorTrustCertificate{*legacy}
	options := FrostNativeSignerAnchorTrustChainValidationOptions{
		ExpectedProtocolID:                 legacy.ProtocolID,
		ExpectedStreamID:                   legacy.StreamID,
		ExpectedSignerStoreFingerprint:     legacy.SignerStoreFingerprint,
		ExpectedOfflineAuthorityPublicKey:  legacy.To.OfflineAuthorityPublicKey,
		ExpectedOfflineAuthoritySPKISHA256: legacy.To.OfflineAuthoritySPKISHA256,
		ExpectedHead:                       trustTestCertificateHead(legacy),
		ValidateTargetAcknowledgement: func(
			_ *FrostNativeSignerAnchorTrustCertificate,
			_ []byte,
		) error {
			return nil
		},
	}
	if err := ValidateFrostNativeSignerAnchorTrustCertificateChain(
		chain,
		options,
	); err == nil || !strings.Contains(err.Error(), "adoption") {
		t.Fatalf("expected legacy adoption rejection, got [%v]", err)
	}
	options.AllowLegacyAdoption = true
	if err := ValidateFrostNativeSignerAnchorTrustCertificateChain(
		chain,
		options,
	); err != nil {
		t.Fatalf("explicitly authorized legacy adoption was rejected: %v", err)
	}
}

func TestFrostNativeSignerAnchorTrustRotationInvariants(t *testing.T) {
	bootstrap, authority := trustTestBootstrapCertificate(t)
	valid := trustTestRotationCertificate(t, bootstrap, authority, 0x55)
	tests := map[string]func(*FrostNativeSignerAnchorTrustCertificate){
		"manifest sequence skip": func(certificate *FrostNativeSignerAnchorTrustCertificate) {
			certificate.To.ActivationManifestSequence++
		},
		"service epoch skip": func(certificate *FrostNativeSignerAnchorTrustCertificate) {
			certificate.To.Reference.ServiceEpoch++
		},
		"revision": func(certificate *FrostNativeSignerAnchorTrustCertificate) {
			certificate.To.Reference.Revision = 2
		},
		"previous event root": func(certificate *FrostNativeSignerAnchorTrustCertificate) {
			certificate.To.Reference.PreviousEventRoot[0] ^= 1
		},
		"checkpoint advance": func(certificate *FrostNativeSignerAnchorTrustCertificate) {
			certificate.To.Reference.Checkpoint.Generation++
		},
		"authority rotation": func(certificate *FrostNativeSignerAnchorTrustCertificate) {
			certificate.To.OfflineAuthorityPublicKey[0] ^= 1
		},
		"authority response key alias": func(certificate *FrostNativeSignerAnchorTrustCertificate) {
			certificate.To.ResponsePublicKey =
				certificate.To.OfflineAuthorityPublicKey
			certificate.To.ResponsePublicKeySPKISHA256 =
				certificate.To.OfflineAuthoritySPKISHA256
		},
		"maximum records change": func(certificate *FrostNativeSignerAnchorTrustCertificate) {
			certificate.To.WitnessMaximumRecords++
		},
		"rotation threshold change": func(certificate *FrostNativeSignerAnchorTrustCertificate) {
			certificate.To.WitnessRotationThresholdRecords--
		},
		"unchanged binding": func(certificate *FrostNativeSignerAnchorTrustCertificate) {
			certificate.To.BindingHash = certificate.From.BindingHash
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			certificate := trustTestCloneCertificate(valid)
			mutate(certificate)
			trustTestFinalizeCertificate(t, certificate, authority)
			if err := ValidateFrostNativeSignerAnchorTrustCertificate(
				certificate,
				func(
					_ *FrostNativeSignerAnchorTrustCertificate,
					_ []byte,
				) error {
					return nil
				},
			); err == nil {
				t.Fatal("invalid rotation invariant was accepted")
			}
		})
	}
}

func TestFrostNativeSignerAnchorTrustTransitionRequest(t *testing.T) {
	bootstrap, authority := trustTestBootstrapCertificate(t)
	rotation := trustTestRotationCertificate(t, bootstrap, authority, 0x66)
	request := &FrostNativeSignerAnchorTrustTransitionRequest{
		CertificateChain: []FrostNativeSignerAnchorTrustCertificate{
			*bootstrap,
			*rotation,
		},
		TargetReadResponse: []byte(`{"schema":"fresh-read","nonce":"n"}`),
	}
	encoded, err := EncodeFrostNativeSignerAnchorTrustTransitionRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeAndValidateFrostNativeSignerAnchorTrustTransitionRequest(
		encoded,
		FrostNativeSignerAnchorTrustChainValidationOptions{
			ExpectedProtocolID:                 rotation.ProtocolID,
			ExpectedStreamID:                   rotation.StreamID,
			ExpectedSignerStoreFingerprint:     rotation.SignerStoreFingerprint,
			ExpectedOfflineAuthorityPublicKey:  rotation.To.OfflineAuthorityPublicKey,
			ExpectedOfflineAuthoritySPKISHA256: rotation.To.OfflineAuthoritySPKISHA256,
			ExpectedHead:                       trustTestCertificateHead(rotation),
			ValidateTargetAcknowledgement: func(
				_ *FrostNativeSignerAnchorTrustCertificate,
				_ []byte,
			) error {
				return nil
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(request, decoded) {
		t.Fatal("trust transition request round trip changed exact bytes")
	}

	emptyWire := frostNativeSignerAnchorTrustTransitionRequestWire{
		Schema:                   FrostNativeSignerAnchorTrustTransitionRequestSchema,
		CertificateChain:         &[]json.RawMessage{},
		TargetReadResponseBase64: base64.StdEncoding.EncodeToString(request.TargetReadResponse),
	}
	emptyJSON, _ := json.Marshal(emptyWire)
	if _, err := DecodeFrostNativeSignerAnchorTrustTransitionRequest(
		emptyJSON,
	); err == nil {
		t.Fatal("empty certificate chain was accepted")
	}

	certificateJSON, _ :=
		EncodeFrostNativeSignerAnchorTrustCertificate(bootstrap)
	tooMany := make(
		[]json.RawMessage,
		FrostNativeSignerAnchorTrustMaximumCertificateChainLength+1,
	)
	for index := range tooMany {
		tooMany[index] = certificateJSON
	}
	tooManyWire := frostNativeSignerAnchorTrustTransitionRequestWire{
		Schema:                   FrostNativeSignerAnchorTrustTransitionRequestSchema,
		CertificateChain:         &tooMany,
		TargetReadResponseBase64: base64.StdEncoding.EncodeToString(request.TargetReadResponse),
	}
	tooManyJSON, _ := json.Marshal(tooManyWire)
	if _, err := DecodeFrostNativeSignerAnchorTrustTransitionRequest(
		tooManyJSON,
	); err == nil {
		t.Fatal("oversized certificate chain was accepted")
	}
}

func TestFrostNativeSignerAnchorTrustWireBoundsAlignWithNativeFFI(
	t *testing.T,
) {
	if frostNativeSignerAnchorTrustMaximumTransitionRequestBytes !=
		frostsigning.NativeTBTCSignerStateAnchorTrustTransitionMaximumRequestBytes {
		t.Fatal("trust-transition request and native FFI bounds differ")
	}

	bootstrap, _ := trustTestBootstrapCertificate(t)
	exactRead := trustTestJSONAtSize(
		t,
		frostNativeSignerAnchorTrustMaximumReadResponseBytes,
	)
	request := &FrostNativeSignerAnchorTrustTransitionRequest{
		CertificateChain: []FrostNativeSignerAnchorTrustCertificate{
			*bootstrap,
		},
		TargetReadResponse: exactRead,
	}
	encoded, err := EncodeFrostNativeSignerAnchorTrustTransitionRequest(request)
	if err != nil {
		t.Fatalf("exact-bound target Read was rejected: %v", err)
	}
	if len(encoded) >
		frostsigning.NativeTBTCSignerStateAnchorTrustTransitionMaximumRequestBytes {
		t.Fatal("accepted request exceeds the native FFI request bound")
	}

	request.TargetReadResponse = trustTestJSONAtSize(
		t,
		frostNativeSignerAnchorTrustMaximumReadResponseBytes+1,
	)
	if _, err := EncodeFrostNativeSignerAnchorTrustTransitionRequest(
		request,
	); err == nil {
		t.Fatal("over-bound target Read was accepted")
	}

	oversizedCertificate := make(
		[]byte,
		frostNativeSignerAnchorTrustMaximumCertificateBytes+1,
	)
	copy(oversizedCertificate, []byte(`{"schema":`))
	if _, err := DecodeFrostNativeSignerAnchorTrustCertificate(
		oversizedCertificate,
	); err == nil {
		t.Fatal("certificate exceeding the durable-record bound was accepted")
	}
}

func TestDecodeFrostNativeSignerAnchorTrustCertificateChain(t *testing.T) {
	bootstrap, authority := trustTestBootstrapCertificate(t)
	rotation := trustTestRotationCertificate(t, bootstrap, authority, 0x67)
	bootstrapJSON, err :=
		EncodeFrostNativeSignerAnchorTrustCertificate(bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	rotationJSON, err :=
		EncodeFrostNativeSignerAnchorTrustCertificate(rotation)
	if err != nil {
		t.Fatal(err)
	}
	chainJSON, err := json.Marshal([]json.RawMessage{
		bootstrapJSON,
		rotationJSON,
	})
	if err != nil {
		t.Fatal(err)
	}
	decoded, err :=
		DecodeFrostNativeSignerAnchorTrustCertificateChain(chainJSON)
	if err != nil {
		t.Fatal(err)
	}
	expected := []FrostNativeSignerAnchorTrustCertificate{
		*bootstrap,
		*rotation,
	}
	if !reflect.DeepEqual(expected, decoded) {
		t.Fatal("secure config certificate chain changed exact material")
	}
	for name, payload := range map[string][]byte{
		"object root": bootstrapJSON,
		"empty":       []byte(`[]`),
		"trailing":    append(append([]byte{}, chainJSON...), []byte(`[]`)...),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeFrostNativeSignerAnchorTrustCertificateChain(
				payload,
			); err == nil {
				t.Fatal("malformed secure config certificate chain was accepted")
			}
		})
	}

	tooMany := make(
		[]json.RawMessage,
		FrostNativeSignerAnchorTrustMaximumCertificateChainLength+1,
	)
	for index := range tooMany {
		tooMany[index] = bootstrapJSON
	}
	tooManyJSON, _ := json.Marshal(tooMany)
	if _, err := DecodeFrostNativeSignerAnchorTrustCertificateChain(
		tooManyJSON,
	); err == nil {
		t.Fatal("oversized secure config certificate chain was accepted")
	}
}

func trustTestJSONAtSize(t *testing.T, size int) []byte {
	t.Helper()
	prefix := []byte(`{"padding":"`)
	suffix := []byte(`"}`)
	if size < len(prefix)+len(suffix) {
		t.Fatal("requested JSON test size is too small")
	}
	result := make([]byte, 0, size)
	result = append(result, prefix...)
	result = append(
		result,
		bytes.Repeat([]byte{'x'}, size-len(prefix)-len(suffix))...,
	)
	result = append(result, suffix...)
	if len(result) != size || !json.Valid(result) {
		t.Fatal("failed to construct exact-size JSON")
	}
	return result
}

func trustTestBootstrapCertificate(
	t *testing.T,
) (*FrostNativeSignerAnchorTrustCertificate, ed25519.PrivateKey) {
	t.Helper()
	authority := ed25519.NewKeyFromSeed(
		bytes.Repeat([]byte{0x11}, ed25519.SeedSize),
	)
	response := ed25519.NewKeyFromSeed(
		bytes.Repeat([]byte{0x22}, ed25519.SeedSize),
	)
	authorityPublic := trustTestRawPublicKey(authority)
	responsePublic := trustTestRawPublicKey(response)
	storeFingerprint := trustTestBytes32(0x03)
	checkpoint := trustTestCheckpoint(
		storeFingerprint,
		7,
		trustTestBytes32(0x31),
		trustTestBytes32(0x32),
	)
	certificate := &FrostNativeSignerAnchorTrustCertificate{
		Kind:                   FrostNativeSignerAnchorTrustCertificateBootstrap,
		CertificateSequence:    1,
		ProtocolID:             trustTestBytes32(0x01),
		StreamID:               trustTestBytes32(0x02),
		SignerStoreFingerprint: storeFingerprint,
		To: FrostNativeSignerAnchorTrustEndpoint{
			ActivationManifestHash:          trustTestBytes32(0x04),
			ActivationManifestSequence:      9,
			BindingHash:                     trustTestBytes32(0x05),
			ResponsePublicKey:               responsePublic,
			ResponsePublicKeySPKISHA256:     ComputeFrostNativeSignerAnchorTrustEd25519SPKISHA256(responsePublic),
			OfflineAuthorityPublicKey:       authorityPublic,
			OfflineAuthoritySPKISHA256:      ComputeFrostNativeSignerAnchorTrustEd25519SPKISHA256(authorityPublic),
			WitnessMaximumRecords:           1000,
			WitnessRotationThresholdRecords: 900,
			Reference: FrostNativeSignerAnchorTrustReference{
				ServiceEpoch:          1,
				Revision:              1,
				EventRoot:             trustTestBytes32(0x41),
				AcknowledgementDigest: trustTestBytes32(0x42),
				Checkpoint:            checkpoint,
			},
		},
		TargetAcknowledgement: []byte(
			`{"schema":"tbtc-signer-state-witness-checkpoint-ack/v1","vector":"bootstrap"}`,
		),
	}
	trustTestFinalizeCertificateWithAcknowledgement(
		t,
		certificate,
		authority,
		response,
		0x51,
	)
	return certificate, authority
}

func trustTestRotationCertificate(
	t *testing.T,
	previous *FrostNativeSignerAnchorTrustCertificate,
	authority ed25519.PrivateKey,
	responseSeed byte,
) *FrostNativeSignerAnchorTrustCertificate {
	t.Helper()
	response := ed25519.NewKeyFromSeed(
		bytes.Repeat([]byte{responseSeed}, ed25519.SeedSize),
	)
	responsePublic := trustTestRawPublicKey(response)
	from := previous.To
	certificate := &FrostNativeSignerAnchorTrustCertificate{
		Kind:                      FrostNativeSignerAnchorTrustCertificateRotation,
		CertificateSequence:       previous.CertificateSequence + 1,
		PreviousCertificateDigest: previous.CertificateDigest,
		ProtocolID:                previous.ProtocolID,
		StreamID:                  previous.StreamID,
		SignerStoreFingerprint:    previous.SignerStoreFingerprint,
		From:                      &from,
		To: FrostNativeSignerAnchorTrustEndpoint{
			ActivationManifestHash:          trustTestBytes32(responseSeed + 1),
			ActivationManifestSequence:      from.ActivationManifestSequence + 1,
			BindingHash:                     trustTestBytes32(responseSeed + 2),
			ResponsePublicKey:               responsePublic,
			ResponsePublicKeySPKISHA256:     ComputeFrostNativeSignerAnchorTrustEd25519SPKISHA256(responsePublic),
			OfflineAuthorityPublicKey:       from.OfflineAuthorityPublicKey,
			OfflineAuthoritySPKISHA256:      from.OfflineAuthoritySPKISHA256,
			WitnessMaximumRecords:           from.WitnessMaximumRecords,
			WitnessRotationThresholdRecords: from.WitnessRotationThresholdRecords,
			Reference: FrostNativeSignerAnchorTrustReference{
				ServiceEpoch:          from.Reference.ServiceEpoch + 1,
				Revision:              1,
				PreviousEventRoot:     from.Reference.EventRoot,
				EventRoot:             trustTestBytes32(responseSeed + 3),
				AcknowledgementDigest: trustTestBytes32(responseSeed + 4),
				Checkpoint:            from.Reference.Checkpoint,
			},
		},
		TargetAcknowledgement: []byte(
			`{"schema":"tbtc-signer-state-witness-checkpoint-ack/v1","vector":"rotation"}`,
		),
	}
	trustTestFinalizeCertificateWithAcknowledgement(
		t,
		certificate,
		authority,
		response,
		responseSeed,
	)
	return certificate
}

func trustTestFinalizeCertificate(
	t *testing.T,
	certificate *FrostNativeSignerAnchorTrustCertificate,
	authority ed25519.PrivateKey,
) {
	t.Helper()
	var err error
	certificate.CoreDigest, err =
		ComputeFrostNativeSignerAnchorTrustCoreDigest(certificate)
	if err != nil {
		t.Fatal(err)
	}
	copy(
		certificate.CoreSignature[:],
		ed25519.Sign(authority, certificate.CoreDigest[:]),
	)
	certificate.OperationID =
		ComputeFrostNativeSignerAnchorTrustOperationID(certificate.CoreDigest)
	certificate.TransitionDigest =
		ComputeFrostNativeSignerAnchorTrustTransitionDigest(
			certificate.CoreDigest,
			certificate.OperationID,
		)
	certificate.TargetAcknowledgementSHA256 =
		sha256.Sum256(certificate.TargetAcknowledgement)
	finalDigest, err :=
		ComputeFrostNativeSignerAnchorTrustFinalDigest(certificate)
	if err != nil {
		t.Fatal(err)
	}
	copy(
		certificate.FinalSignature[:],
		ed25519.Sign(authority, finalDigest[:]),
	)
	certificate.CertificateDigest, err =
		ComputeFrostNativeSignerAnchorTrustCertificateDigest(certificate)
	if err != nil {
		t.Fatal(err)
	}
}

func trustTestFinalizeCertificateWithAcknowledgement(
	t *testing.T,
	certificate *FrostNativeSignerAnchorTrustCertificate,
	authority ed25519.PrivateKey,
	response ed25519.PrivateKey,
	seed byte,
) {
	t.Helper()
	// Establish the core-derived operation identities first. To event-root and
	// acknowledgement fields are deliberately outside the core transcript.
	trustTestFinalizeCertificate(t, certificate, authority)
	acknowledgement := FrostNativeSignerCheckpointAcknowledgement{
		BindingHash:       certificate.To.BindingHash,
		RequestDigest:     trustTestBytes32(seed + 1),
		Nonce:             trustTestBytes32(seed + 2),
		Status:            "applied",
		ServiceEpoch:      certificate.To.Reference.ServiceEpoch,
		Revision:          certificate.To.Reference.Revision,
		PreviousEventRoot: certificate.To.Reference.PreviousEventRoot,
		Checkpoint:        certificate.To.Reference.Checkpoint,
		OperationID:       certificate.OperationID,
		TransitionDigest:  certificate.TransitionDigest,
		CommittedAtUnixMs: 1_700_000_000_000 +
			certificate.CertificateSequence*1_000,
		ExpiresAtUnixMs: 1_700_000_030_000 +
			certificate.CertificateSequence*1_000,
	}
	acknowledgement.EventRoot =
		computeFrostNativeSignerAnchorEventRoot(acknowledgement)
	wire := frostNativeSignerAnchorAcknowledgementWire{
		Schema:            FrostNativeSignerCheckpointAcknowledgementSchema,
		BindingHash:       frostNativeSignerAnchorHex32(acknowledgement.BindingHash),
		RequestDigest:     frostNativeSignerAnchorHex32(acknowledgement.RequestDigest),
		Nonce:             frostNativeSignerAnchorHex32(acknowledgement.Nonce),
		Status:            acknowledgement.Status,
		ServiceEpoch:      fmt.Sprint(acknowledgement.ServiceEpoch),
		Revision:          fmt.Sprint(acknowledgement.Revision),
		PreviousEventRoot: frostNativeSignerAnchorHex32(acknowledgement.PreviousEventRoot),
		EventRoot:         frostNativeSignerAnchorHex32(acknowledgement.EventRoot),
		Checkpoint: frostNativeSignerAnchorCheckpointToWire(
			acknowledgement.Checkpoint,
		),
		OperationID:       frostNativeSignerAnchorHex32(acknowledgement.OperationID),
		TransitionDigest:  frostNativeSignerAnchorHex32(acknowledgement.TransitionDigest),
		CommittedAtUnixMs: fmt.Sprint(acknowledgement.CommittedAtUnixMs),
		ExpiresAtUnixMs:   fmt.Sprint(acknowledgement.ExpiresAtUnixMs),
	}
	signingDigest, err := frostNativeSignerAnchorAcknowledgementTranscript(wire)
	if err != nil {
		t.Fatal(err)
	}
	signature := ed25519.Sign(response, signingDigest)
	wire.Signature = frostNativeSignerAnchorSignatureHex(signature)
	var fixedSigningDigest [32]byte
	copy(fixedSigningDigest[:], signingDigest)
	var fixedSignature [ed25519.SignatureSize]byte
	copy(fixedSignature[:], signature)
	acknowledgement.AcknowledgementDigest =
		computeFrostNativeSignerCheckpointAcknowledgementDigest(
			fixedSigningDigest,
			fixedSignature,
			certificate.To.ResponsePublicKeySPKISHA256,
		)
	raw, err := json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	certificate.To.Reference.EventRoot = acknowledgement.EventRoot
	certificate.To.Reference.AcknowledgementDigest =
		acknowledgement.AcknowledgementDigest
	certificate.TargetAcknowledgement = raw
	trustTestFinalizeCertificate(t, certificate, authority)
}

func trustTestSetRotationFromDescendant(
	t *testing.T,
	certificate *FrostNativeSignerAnchorTrustCertificate,
	authority ed25519.PrivateKey,
	revision uint64,
	checkpoint FrostNativeSignerStateWitnessCheckpoint,
) {
	t.Helper()
	if certificate.From == nil {
		t.Fatal("rotation certificate has no from endpoint")
	}
	from := *certificate.From
	from.Reference.Revision = revision
	from.Reference.PreviousEventRoot = trustTestBytes32(0x93)
	from.Reference.EventRoot = trustTestBytes32(0x94)
	from.Reference.AcknowledgementDigest = trustTestBytes32(0x95)
	from.Reference.Checkpoint = checkpoint
	certificate.From = &from
	certificate.To.Reference.ServiceEpoch = from.Reference.ServiceEpoch + 1
	certificate.To.Reference.Revision = 1
	certificate.To.Reference.PreviousEventRoot = from.Reference.EventRoot
	certificate.To.Reference.Checkpoint = checkpoint
	trustTestFinalizeCertificate(t, certificate, authority)
}

func trustTestCheckpoint(
	storeFingerprint [32]byte,
	generation uint64,
	previous [32]byte,
	image [32]byte,
) FrostNativeSignerStateWitnessCheckpoint {
	return FrostNativeSignerStateWitnessCheckpoint{
		StoreFingerprint:        storeFingerprint,
		Generation:              generation,
		PreviousStateCommitment: previous,
		StateImageDigest:        image,
		StateCommitment: frostsigning.ComputeNativeTBTCSignerStateWitnessCommitment(
			storeFingerprint,
			generation,
			previous,
			image,
		),
	}
}

func trustTestRawPublicKey(
	privateKey ed25519.PrivateKey,
) [ed25519.PublicKeySize]byte {
	var result [ed25519.PublicKeySize]byte
	copy(result[:], privateKey.Public().(ed25519.PublicKey))
	return result
}

func trustTestBytes32(value byte) [32]byte {
	return [32]byte{
		value, value, value, value, value, value, value, value,
		value, value, value, value, value, value, value, value,
		value, value, value, value, value, value, value, value,
		value, value, value, value, value, value, value, value,
	}
}

func trustTestCloneCertificate(
	certificate *FrostNativeSignerAnchorTrustCertificate,
) *FrostNativeSignerAnchorTrustCertificate {
	copy := *certificate
	copy.TargetAcknowledgement = append(
		[]byte{},
		certificate.TargetAcknowledgement...,
	)
	if certificate.From != nil {
		from := *certificate.From
		copy.From = &from
	}
	return &copy
}

func trustTestCloneChain(
	chain []FrostNativeSignerAnchorTrustCertificate,
) []FrostNativeSignerAnchorTrustCertificate {
	result := make([]FrostNativeSignerAnchorTrustCertificate, len(chain))
	for index := range chain {
		result[index] = *trustTestCloneCertificate(&chain[index])
	}
	return result
}

func trustTestCertificateHead(
	certificate *FrostNativeSignerAnchorTrustCertificate,
) *FrostNativeSignerAnchorTrustCertificateHead {
	return &FrostNativeSignerAnchorTrustCertificateHead{
		CertificateSequence:    certificate.CertificateSequence,
		CertificateDigest:      certificate.CertificateDigest,
		ProtocolID:             certificate.ProtocolID,
		StreamID:               certificate.StreamID,
		SignerStoreFingerprint: certificate.SignerStoreFingerprint,
		Endpoint:               certificate.To,
	}
}

func trustTestChainOptions(
	head *FrostNativeSignerAnchorTrustCertificate,
) FrostNativeSignerAnchorTrustChainValidationOptions {
	return FrostNativeSignerAnchorTrustChainValidationOptions{
		ExpectedProtocolID:                 head.ProtocolID,
		ExpectedStreamID:                   head.StreamID,
		ExpectedSignerStoreFingerprint:     head.SignerStoreFingerprint,
		ExpectedOfflineAuthorityPublicKey:  head.To.OfflineAuthorityPublicKey,
		ExpectedOfflineAuthoritySPKISHA256: head.To.OfflineAuthoritySPKISHA256,
		ExpectedHead:                       trustTestCertificateHead(head),
		ValidateTargetAcknowledgement: func(
			_ *FrostNativeSignerAnchorTrustCertificate,
			_ []byte,
		) error {
			return nil
		},
	}
}

func trustTestCloneChainOptions(
	options FrostNativeSignerAnchorTrustChainValidationOptions,
) FrostNativeSignerAnchorTrustChainValidationOptions {
	result := options
	if options.PriorHead != nil {
		prior := *options.PriorHead
		result.PriorHead = &prior
	}
	if options.ExpectedHead != nil {
		expected := *options.ExpectedHead
		result.ExpectedHead = &expected
	}
	return result
}

func trustTestUppercaseHexMember(
	t *testing.T,
	payload string,
	member string,
) string {
	t.Helper()
	prefix := `"` + member + `":"0x`
	start := strings.Index(payload, prefix)
	if start < 0 {
		t.Fatalf("member [%s] not found", member)
	}
	start += len(prefix)
	end := start + 64
	value := payload[start:end]
	for index, character := range value {
		if character >= 'a' && character <= 'f' {
			upper := strings.ToUpper(string(character))
			return payload[:start+index] + upper + payload[start+index+1:]
		}
	}
	t.Fatalf("member [%s] has no hexadecimal letter", member)
	return ""
}
