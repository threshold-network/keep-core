package tbtc

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/rpc"
)

func testFrostRetainedGroupCompleteIdentity() FrostRetainedGroupHistoryIdentity {
	protocolID := frostRetainedGroupTLSExporterProtocolID()
	exportIdentity := FrostRetainedGroupEndpointIdentity{
		Schema:                    frostRetainedGroupEndpointIdentitySchema,
		Role:                      "retained-history-export",
		TrustDomainID:             "retained-export.example",
		CanonicalEndpoint:         "https://retained-export.example:443/history",
		CanonicalDNSName:          "retained-export.example",
		ResolvedDNSName:           "retained-export-origin.example",
		ResolvedAddressSetHash:    [32]byte{0xa1},
		TLSLeafSPKIHash:           [32]byte{0xa2},
		ServiceIdentity:           "spiffe://retained-export.example/export",
		BackendServiceFingerprint: [32]byte{0xa3},
		OperatorFingerprint:       [32]byte{0xa4},
		AttestationKeyHash:        [32]byte{0xa5},
		TLSExporterProtocolID:     protocolID,
	}
	exportIdentity.EndpointFingerprint =
		computeFrostRetainedGroupEndpointFingerprint(exportIdentity)
	verifierIdentity := FrostRetainedGroupEndpointIdentity{
		Schema:                    frostRetainedGroupEndpointIdentitySchema,
		Role:                      "retained-history-verifier",
		TrustDomainID:             "retained-verifier.example",
		CanonicalEndpoint:         "https://retained-verifier.example:443/rpc",
		CanonicalDNSName:          "retained-verifier.example",
		ResolvedDNSName:           "retained-verifier-origin.example",
		ResolvedAddressSetHash:    [32]byte{0xa6},
		TLSLeafSPKIHash:           [32]byte{0xa7},
		ServiceIdentity:           "spiffe://retained-verifier.example/verifier",
		BackendServiceFingerprint: [32]byte{0xa8},
		OperatorFingerprint:       [32]byte{0xa9},
		AttestationKeyHash:        [32]byte{0xaa},
		TLSExporterProtocolID:     protocolID,
	}
	verifierIdentity.EndpointFingerprint =
		computeFrostRetainedGroupEndpointFingerprint(verifierIdentity)
	identity := FrostRetainedGroupHistoryIdentity{
		Schema:               frostRetainedGroupSourceIdentitySchema,
		TrustDomainID:        "independent-journal-source",
		OperatorFingerprint:  exportIdentity.OperatorFingerprint,
		HistorySignerKeyHash: [32]byte{0xab},
		Export:               exportIdentity,
		Verifier:             verifierIdentity,
	}
	identity.EndpointFingerprint =
		computeFrostRetainedGroupSourceEndpointFingerprint(identity)
	return identity
}

func TestFrostRetainedGroupIdentityFingerprintsFrozen(t *testing.T) {
	identity := testFrostRetainedGroupCompleteIdentity()
	addressHash := frostRetainedGroupResolvedAddressSetHash([]netip.Addr{
		netip.MustParseAddr("2001:db8::1"),
		netip.MustParseAddr("192.0.2.1"),
		netip.MustParseAddr("::ffff:192.0.2.1"),
		netip.MustParseAddr("2001:db8::1"),
	})
	const expectedExport = "416dd89aa039243ca4b03e8b5c15e54e4dd9b5553ceaf230ad5e978bd3e073d3"
	const expectedSource = "d4100a02930b9fd62a78fae2c40316ea8ddd85c2dcffc344a502d5ebdd27d971"
	const expectedAddresses = "9bafc074d0b8ca6b0458ed2d25a48a140f5599854952386087530f1217a90d45"
	if hex.EncodeToString(identity.Export.EndpointFingerprint[:]) != expectedExport ||
		hex.EncodeToString(identity.EndpointFingerprint[:]) != expectedSource ||
		hex.EncodeToString(addressHash[:]) != expectedAddresses {
		t.Fatalf(
			"frozen identity vectors changed: [%x] [%x] [%x]",
			identity.Export.EndpointFingerprint,
			identity.EndpointFingerprint,
			addressHash,
		)
	}
}

func TestFrostRetainedGroupTransportAttestationFrozenVectors(t *testing.T) {
	attestationPrivateKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x01}, 32))
	backendPrivateKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x02}, 32))
	operatorPrivateKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x03}, 32))
	attestationPublicKeyDER, err := x509.MarshalPKIXPublicKey(
		attestationPrivateKey.Public(),
	)
	if err != nil {
		t.Fatal(err)
	}
	backendPublicKeyDER, err := x509.MarshalPKIXPublicKey(
		backendPrivateKey.Public(),
	)
	if err != nil {
		t.Fatal(err)
	}
	operatorPublicKeyDER, err := x509.MarshalPKIXPublicKey(
		operatorPrivateKey.Public(),
	)
	if err != nil {
		t.Fatal(err)
	}
	identity := testFrostRetainedGroupCompleteIdentity().Export
	identity.AttestationKeyHash = sha256.Sum256(attestationPublicKeyDER)
	identity.BackendServiceFingerprint = sha256.Sum256(backendPublicKeyDER)
	identity.OperatorFingerprint = sha256.Sum256(operatorPublicKeyDER)
	identity.EndpointFingerprint =
		computeFrostRetainedGroupEndpointFingerprint(identity)
	challenge := [32]byte{0x11}
	requestDigest := [32]byte{0x12}
	responseDigest := [32]byte{0x13}
	contextHash := frostRetainedGroupTLSExporterContext(
		identity,
		challenge,
		http.MethodPost,
		identity.CanonicalEndpoint,
		requestDigest,
		http.StatusOK,
		responseDigest,
	)
	exporterValueHash := frostRetainedGroupTLSExporterValueHash(
		[]byte("fixed-exported-keying-material"),
	)
	attestation := frostRetainedGroupTransportAttestation{
		Schema:                    frostRetainedGroupTransportAttestationSchema,
		Role:                      identity.Role,
		EndpointFingerprint:       frostActivationHex32(identity.EndpointFingerprint),
		CanonicalEndpoint:         identity.CanonicalEndpoint,
		CanonicalDNSName:          identity.CanonicalDNSName,
		ResolvedDNSName:           identity.ResolvedDNSName,
		ResolvedPeerIP:            "192.0.2.1",
		TLSLeafSPKIHash:           frostActivationHex32(identity.TLSLeafSPKIHash),
		ServiceIdentity:           identity.ServiceIdentity,
		BackendServiceFingerprint: frostActivationHex32(identity.BackendServiceFingerprint),
		OperatorFingerprint:       frostActivationHex32(identity.OperatorFingerprint),
		AttestationKeyHash:        frostActivationHex32(identity.AttestationKeyHash),
		TLSExporterProtocolID:     frostActivationHex32(identity.TLSExporterProtocolID),
		Challenge:                 frostActivationHex32(challenge),
		RequestMethod:             http.MethodPost,
		RequestTarget:             identity.CanonicalEndpoint,
		RequestBodySHA256:         frostActivationHex32(requestDigest),
		ResponseStatus:            http.StatusOK,
		ResponseBodySHA256:        frostActivationHex32(responseDigest),
		IssuedAtUnixMs:            "1700000000000",
		ExpiresAtUnixMs:           "1700000030000",
		TLSExporterContextSHA256:  frostActivationHex32(contextHash),
		TLSExporterValueSHA256:    frostActivationHex32(exporterValueHash),
		BackendSignerPublicKeySPKI: base64.StdEncoding.EncodeToString(
			backendPublicKeyDER,
		),
		BackendSignatureAlgorithm: "ed25519",
		OperatorSignerPublicKeySPKI: base64.StdEncoding.EncodeToString(
			operatorPublicKeyDER,
		),
		OperatorSignatureAlgorithm: "ed25519",
		SignerPublicKeySPKI: base64.StdEncoding.EncodeToString(
			attestationPublicKeyDER,
		),
		SignatureAlgorithm: "ed25519",
	}
	attestationDigest, err := frostRetainedGroupAttestationTranscript(attestation)
	if err != nil {
		t.Fatal(err)
	}
	backendDigest := frostRetainedGroupBackendAttestationDigest(attestationDigest)
	operatorDigest := frostRetainedGroupOperatorAttestationDigest(attestationDigest)
	attestation.BackendSignature = base64.StdEncoding.EncodeToString(
		ed25519.Sign(backendPrivateKey, backendDigest[:]),
	)
	attestation.OperatorSignature = base64.StdEncoding.EncodeToString(
		ed25519.Sign(operatorPrivateKey, operatorDigest[:]),
	)
	attestation.Signature = base64.StdEncoding.EncodeToString(
		ed25519.Sign(attestationPrivateKey, attestationDigest[:]),
	)
	wire, err := json.Marshal(attestation)
	if err != nil {
		t.Fatal(err)
	}
	wireHash := sha256.Sum256(wire)
	const expectedContext = "50587c477f56e9d2597c4cf4d9cf69d69a8ded13b9a074d1c18042eb4aea2e30"
	const expectedExporter = "62e2fd2ccb30b5e2ca49a99f04cbb01a947d8228060ee377dc6896c6c96a08b0"
	const expectedAttestation = "53eb0f08ca2c1761592ec90387c5aba9913668ef68c1e4cf6f20dbbd531e267b"
	const expectedBackend = "7fb92657871cf2dd864eeffaac5d699f1131f81b04507b2092f90987aa4a722c"
	const expectedOperator = "02495d610fde3ad437a22de7e93dae5394d34a0ef4e590f4d613e3fa6197cbc4"
	const expectedTransportSignature = "AV8bIwrHUE6ACkJ9vQWtQTVXGJDR8Nm42nQqCka8NISgXIIPbW2abLhOrGlX/bnYVUUUMbhRfcyYCf+asQ9KBg=="
	const expectedBackendSignature = "0ACdPMwZaraKgWpVRMNnwc6qgLtB90rGDoj18kYCcbd2jJG36VCVf0j5OhHlqP5KoQbzKwJ9BRelXF1pYd/yDA=="
	const expectedOperatorSignature = "lx22S9wJxUSOsgZZlWqP7S5bTdoULktZQ9Ao7KDwdpI8zJDkq3AD76rj737DSthJUt0+6IEdElMyiCnXwdgFBg=="
	const expectedWire = "174da49defe9c6b6c669a8a33177ccf6257df24e1316c650734c8ff35099dcdb"
	if hex.EncodeToString(contextHash[:]) != expectedContext ||
		hex.EncodeToString(exporterValueHash[:]) != expectedExporter ||
		hex.EncodeToString(attestationDigest[:]) != expectedAttestation ||
		hex.EncodeToString(backendDigest[:]) != expectedBackend ||
		hex.EncodeToString(operatorDigest[:]) != expectedOperator ||
		attestation.Signature != expectedTransportSignature ||
		attestation.BackendSignature != expectedBackendSignature ||
		attestation.OperatorSignature != expectedOperatorSignature ||
		hex.EncodeToString(wireHash[:]) != expectedWire {
		t.Fatalf(
			"frozen transport-attestation vectors changed: context=%x exporter=%x attestation=%x backend=%x operator=%x transport=%s backendSignature=%s operatorSignature=%s wire=%x",
			contextHash,
			exporterValueHash,
			attestationDigest,
			backendDigest,
			operatorDigest,
			attestation.Signature,
			attestation.BackendSignature,
			attestation.OperatorSignature,
			wireHash,
		)
	}
}

func TestValidateFrostRetainedGroupHistoryIdentity_CommitsEveryField(
	t *testing.T,
) {
	endpointMutations := map[string]func(*FrostRetainedGroupEndpointIdentity){
		"schema": func(identity *FrostRetainedGroupEndpointIdentity) {
			identity.Schema += "-other"
		},
		"role": func(identity *FrostRetainedGroupEndpointIdentity) {
			if identity.Role == "retained-history-export" {
				identity.Role = "retained-history-verifier"
			} else {
				identity.Role = "retained-history-export"
			}
		},
		"trust domain": func(identity *FrostRetainedGroupEndpointIdentity) {
			identity.TrustDomainID += "-other"
		},
		"canonical endpoint": func(identity *FrostRetainedGroupEndpointIdentity) {
			identity.CanonicalEndpoint = "https://other.example:443/history"
		},
		"canonical DNS name": func(identity *FrostRetainedGroupEndpointIdentity) {
			identity.CanonicalDNSName = "other.example"
		},
		"resolved DNS name": func(identity *FrostRetainedGroupEndpointIdentity) {
			identity.ResolvedDNSName = "other-origin.example"
		},
		"resolved addresses": func(identity *FrostRetainedGroupEndpointIdentity) {
			identity.ResolvedAddressSetHash[31] ^= 0x01
		},
		"TLS leaf": func(identity *FrostRetainedGroupEndpointIdentity) {
			identity.TLSLeafSPKIHash[31] ^= 0x01
		},
		"service identity": func(identity *FrostRetainedGroupEndpointIdentity) {
			identity.ServiceIdentity = "spiffe://retained.example/other"
		},
		"backend identity": func(identity *FrostRetainedGroupEndpointIdentity) {
			identity.BackendServiceFingerprint[31] ^= 0x01
		},
		"operator identity": func(identity *FrostRetainedGroupEndpointIdentity) {
			identity.OperatorFingerprint[31] ^= 0x01
		},
		"attestation key": func(identity *FrostRetainedGroupEndpointIdentity) {
			identity.AttestationKeyHash[31] ^= 0x01
		},
		"TLS exporter protocol": func(identity *FrostRetainedGroupEndpointIdentity) {
			identity.TLSExporterProtocolID[31] ^= 0x01
		},
		"endpoint fingerprint": func(identity *FrostRetainedGroupEndpointIdentity) {
			identity.EndpointFingerprint[31] ^= 0x01
		},
	}
	for name, mutate := range endpointMutations {
		t.Run("export "+name, func(t *testing.T) {
			identity := testFrostRetainedGroupCompleteIdentity()
			mutate(&identity.Export)
			if err := validateFrostRetainedGroupHistoryIdentity(identity); err == nil {
				t.Fatal("mutated export identity was accepted")
			}
		})
		t.Run("verifier "+name, func(t *testing.T) {
			identity := testFrostRetainedGroupCompleteIdentity()
			mutate(&identity.Verifier)
			if err := validateFrostRetainedGroupHistoryIdentity(identity); err == nil {
				t.Fatal("mutated verifier identity was accepted")
			}
		})
	}

	sourceMutations := map[string]func(*FrostRetainedGroupHistoryIdentity){
		"schema": func(identity *FrostRetainedGroupHistoryIdentity) {
			identity.Schema += "-other"
		},
		"trust domain": func(identity *FrostRetainedGroupHistoryIdentity) {
			identity.TrustDomainID += "-other"
		},
		"operator": func(identity *FrostRetainedGroupHistoryIdentity) {
			identity.OperatorFingerprint[31] ^= 0x01
		},
		"history signer": func(identity *FrostRetainedGroupHistoryIdentity) {
			identity.HistorySignerKeyHash[31] ^= 0x01
		},
		"fingerprint": func(identity *FrostRetainedGroupHistoryIdentity) {
			identity.EndpointFingerprint[31] ^= 0x01
		},
	}
	for name, mutate := range sourceMutations {
		t.Run("source "+name, func(t *testing.T) {
			identity := testFrostRetainedGroupCompleteIdentity()
			mutate(&identity)
			if err := validateFrostRetainedGroupHistoryIdentity(identity); err == nil {
				t.Fatal("mutated source identity was accepted")
			}
		})
	}
}

func TestValidateFrostRetainedGroupHistoryIdentity_RejectsRoleReuse(
	t *testing.T,
) {
	testCases := map[string]func(*FrostRetainedGroupHistoryIdentity){
		"within endpoint": func(identity *FrostRetainedGroupHistoryIdentity) {
			identity.Export.BackendServiceFingerprint =
				identity.Export.TLSLeafSPKIHash
			identity.Export.EndpointFingerprint =
				computeFrostRetainedGroupEndpointFingerprint(identity.Export)
			identity.EndpointFingerprint =
				computeFrostRetainedGroupSourceEndpointFingerprint(*identity)
		},
		"cross endpoint hash": func(identity *FrostRetainedGroupHistoryIdentity) {
			identity.Verifier.AttestationKeyHash =
				identity.Export.OperatorFingerprint
			identity.Verifier.EndpointFingerprint =
				computeFrostRetainedGroupEndpointFingerprint(identity.Verifier)
			identity.EndpointFingerprint =
				computeFrostRetainedGroupSourceEndpointFingerprint(*identity)
		},
		"history signer reuse": func(identity *FrostRetainedGroupHistoryIdentity) {
			identity.HistorySignerKeyHash =
				identity.Export.AttestationKeyHash
			identity.EndpointFingerprint =
				computeFrostRetainedGroupSourceEndpointFingerprint(*identity)
		},
		"cross endpoint service": func(identity *FrostRetainedGroupHistoryIdentity) {
			identity.Verifier.ServiceIdentity = identity.Export.ServiceIdentity
			identity.Verifier.EndpointFingerprint =
				computeFrostRetainedGroupEndpointFingerprint(identity.Verifier)
			identity.EndpointFingerprint =
				computeFrostRetainedGroupSourceEndpointFingerprint(*identity)
		},
		"same SPIFFE authority": func(identity *FrostRetainedGroupHistoryIdentity) {
			identity.Verifier.ServiceIdentity =
				"spiffe://retained-export.example/verifier"
			identity.Verifier.EndpointFingerprint =
				computeFrostRetainedGroupEndpointFingerprint(identity.Verifier)
			identity.EndpointFingerprint =
				computeFrostRetainedGroupSourceEndpointFingerprint(*identity)
		},
		"cross endpoint DNS": func(identity *FrostRetainedGroupHistoryIdentity) {
			identity.Verifier.ResolvedDNSName = identity.Export.ResolvedDNSName
			identity.Verifier.EndpointFingerprint =
				computeFrostRetainedGroupEndpointFingerprint(identity.Verifier)
			identity.EndpointFingerprint =
				computeFrostRetainedGroupSourceEndpointFingerprint(*identity)
		},
		"aggregate trust domain": func(identity *FrostRetainedGroupHistoryIdentity) {
			identity.TrustDomainID = identity.Export.TrustDomainID
			identity.EndpointFingerprint =
				computeFrostRetainedGroupSourceEndpointFingerprint(*identity)
		},
	}
	for name, mutate := range testCases {
		t.Run(name, func(t *testing.T) {
			identity := testFrostRetainedGroupCompleteIdentity()
			mutate(&identity)
			if err := validateFrostRetainedGroupHistoryIdentity(identity); err == nil {
				t.Fatal("reused endpoint role identity was accepted")
			}
		})
	}
}

func TestValidateFrostRetainedGroupServiceIdentity_StrictSPIFFECanonicalization(
	t *testing.T,
) {
	for _, valid := range []string{
		"spiffe://retained.example/export",
		"spiffe://retained.example/services/verifier-v1",
		"spiffe://single_label/export",
	} {
		if err := validateFrostRetainedGroupServiceIdentity(valid); err != nil {
			t.Fatalf("canonical SPIFFE identity rejected [%s]: [%v]", valid, err)
		}
	}
	for _, invalid := range []string{
		"spiffe://retained.example",
		"spiffe://retained.example/",
		"spiffe://retained.example:443/export",
		"spiffe://Retained.example/export",
		"spiffe://retained.example/export/",
		"spiffe://retained.example/a//b",
		"spiffe://retained.example/a/../b",
		"spiffe://retained.example/%65xport",
		"spiffe://retained.example/a:b",
		"spiffe://retained.example/a~b",
		"spiffe://user@retained.example/export",
		"spiffe://retained.example/export?query=1",
		"spiffe://retained.example/export#fragment",
	} {
		if err := validateFrostRetainedGroupServiceIdentity(invalid); err == nil {
			t.Fatalf("ambiguous SPIFFE identity accepted [%s]", invalid)
		}
	}
}

func TestValidateFrostRetainedGroupTLSEndpoint_StrictCanonicalization(
	t *testing.T,
) {
	for _, valid := range []string{
		"https://history.example:443/",
		"https://history.example:8443/export",
		"https://127.0.0.1:443/rpc",
		"https://[2001:db8::1]:443/rpc",
	} {
		t.Run("valid "+valid, func(t *testing.T) {
			_, canonical, err := validateFrostRetainedGroupTLSEndpoint(valid)
			if err != nil || canonical != valid {
				t.Fatalf("canonical endpoint rejected: [%s] [%v]", canonical, err)
			}
		})
	}
	for _, invalid := range []string{
		"http://history.example:443/",
		"wss://history.example:443/",
		"https://history.example/",
		"https://history.example:0443/",
		"https://History.example:443/",
		"https://history.example.:443/",
		"https://user@history.example:443/",
		"https://history.example:443/?query=1",
		"https://history.example:443/#fragment",
		"https://history.example:443/%68istory",
		"https://history.example:443/a/../history",
		"https://history.example:443//history",
		"https://history.example:443/history/",
		"https://hé.example:443/",
		"https:\\\\history.example:443\\history",
		"https://history.example:443",
	} {
		t.Run("invalid "+invalid, func(t *testing.T) {
			if _, _, err := validateFrostRetainedGroupTLSEndpoint(invalid); err == nil {
				t.Fatal("ambiguous endpoint was accepted")
			}
		})
	}
}

func TestFrostRetainedGroupResolvedAddressSet_IsCanonicalAndDetectsAliases(
	t *testing.T,
) {
	leftAddresses := []netip.Addr{
		netip.MustParseAddr("2001:db8::1"),
		netip.MustParseAddr("192.0.2.1"),
		netip.MustParseAddr("::ffff:192.0.2.1"),
	}
	rightAddresses := []netip.Addr{
		netip.MustParseAddr("192.0.2.1"),
		netip.MustParseAddr("2001:db8::1"),
	}
	if frostRetainedGroupResolvedAddressSetHash(leftAddresses) !=
		frostRetainedGroupResolvedAddressSetHash(rightAddresses) {
		t.Fatal("address-set hash depends on order, duplicates, or mapped form")
	}
	left := frostRetainedGroupResolvedEndpoint{
		canonicalDNSName: "left.example",
		resolvedDNSName:  "left-origin.example",
		addresses:        []netip.Addr{netip.MustParseAddr("192.0.2.1")},
	}
	right := frostRetainedGroupResolvedEndpoint{
		canonicalDNSName: "right.example",
		resolvedDNSName:  "right-origin.example",
		addresses:        []netip.Addr{netip.MustParseAddr("192.0.2.1")},
	}
	if !frostRetainedGroupEndpointSetsOverlap(left, right) {
		t.Fatal("shared backend IP was not detected")
	}
	right.addresses = []netip.Addr{netip.MustParseAddr("192.0.2.2")}
	right.resolvedDNSName = left.canonicalDNSName
	if !frostRetainedGroupEndpointSetsOverlap(left, right) {
		t.Fatal("CNAME/canonical DNS alias was not detected")
	}
}

type frostRetainedGroupRebindingResolver struct {
	cname     string
	addresses []netip.Addr
}

type testFrostPrimaryEthereumIndependenceVerifier struct {
	verify func(
		context.Context,
		frostRetainedGroupResolvedEndpoint,
		frostRetainedGroupResolvedEndpoint,
	) error
}

func (verifier *testFrostPrimaryEthereumIndependenceVerifier) verifyIndependence(
	ctx context.Context,
	exportEndpoint frostRetainedGroupResolvedEndpoint,
	verifierEndpoint frostRetainedGroupResolvedEndpoint,
) error {
	if verifier == nil {
		return fmt.Errorf("test primary Ethereum verifier is nil")
	}
	if verifier.verify == nil {
		return nil
	}
	return verifier.verify(ctx, exportEndpoint, verifierEndpoint)
}

func (resolver *frostRetainedGroupRebindingResolver) LookupCNAME(
	context.Context,
	string,
) (string, error) {
	return resolver.cname, nil
}

func (resolver *frostRetainedGroupRebindingResolver) LookupNetIP(
	context.Context,
	string,
	string,
) ([]netip.Addr, error) {
	return append([]netip.Addr{}, resolver.addresses...), nil
}

func TestFrostRetainedGroupIndependenceMonitor_RejectsPrimaryDNSRebind(
	t *testing.T,
) {
	primaryURL, err := url.Parse("https://primary.example:443/rpc")
	if err != nil {
		t.Fatal(err)
	}
	exportAddress := netip.MustParseAddr("192.0.2.1")
	verifierAddress := netip.MustParseAddr("192.0.2.2")
	resolver := &frostRetainedGroupRebindingResolver{
		cname:     "primary-origin.example.",
		addresses: []netip.Addr{netip.MustParseAddr("192.0.2.3")},
	}
	primaryEndpoint := frostRetainedGroupResolvedEndpoint{
		canonical:        primaryURL.String(),
		canonicalDNSName: "primary.example",
		resolvedDNSName:  "primary-origin.example",
		addresses:        append([]netip.Addr{}, resolver.addresses...),
		addressSetHash: frostRetainedGroupResolvedAddressSetHash(
			resolver.addresses,
		),
		endpoint: primaryURL,
	}
	monitor := &frostRetainedGroupIndependenceMonitor{
		exportEndpoint: frostRetainedGroupResolvedEndpoint{
			canonicalDNSName: "export.example",
			resolvedDNSName:  "export-origin.example",
			addresses:        []netip.Addr{exportAddress},
			endpoint:         &url.URL{Scheme: "https", Host: "export.example:443", Path: "/"},
		},
		verifierEndpoint: frostRetainedGroupResolvedEndpoint{
			canonicalDNSName: "verifier.example",
			resolvedDNSName:  "verifier-origin.example",
			addresses:        []netip.Addr{verifierAddress},
			endpoint:         &url.URL{Scheme: "https", Host: "verifier.example:443", Path: "/"},
		},
		primaryTransport: &testFrostPrimaryEthereumIndependenceVerifier{
			verify: func(
				ctx context.Context,
				exportEndpoint frostRetainedGroupResolvedEndpoint,
				verifierEndpoint frostRetainedGroupResolvedEndpoint,
			) error {
				currentPrimary, err := resolveFrostRetainedGroupEndpoint(
					ctx,
					primaryEndpoint.endpoint,
					resolver,
				)
				if err != nil {
					return err
				}
				if frostRetainedGroupEndpointSetsOverlap(
					currentPrimary,
					exportEndpoint,
				) || frostRetainedGroupEndpointSetsOverlap(
					currentPrimary,
					verifierEndpoint,
				) {
					return fmt.Errorf(
						"primary Ethereum endpoint now aliases a retained endpoint",
					)
				}
				return nil
			},
		},
	}
	if err := monitor.verify(context.Background()); err != nil {
		t.Fatalf("independent primary endpoint rejected: [%v]", err)
	}
	resolver.addresses = []netip.Addr{exportAddress}
	if err := monitor.verify(context.Background()); err == nil ||
		!strings.Contains(err.Error(), "now aliases") {
		t.Fatalf("primary DNS rebind was accepted: [%v]", err)
	}
}

func performFrostRetainedGroupAttestedTestRequest(
	fixture *frostRetainedGroupHistorySourceFixture,
) error {
	client, ok := fixture.source.httpClient.(*http.Client)
	if !ok {
		return io.ErrUnexpectedEOF
	}
	request, err := http.NewRequest(
		http.MethodPost,
		fixture.server.URL+"/operator-id",
		strings.NewReader("{}"),
	)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if response != nil {
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
	}
	return err
}

func TestFrostRetainedGroupAttestedTransport_RejectsMissingAndMutatedEvidence(
	t *testing.T,
) {
	testCases := map[string]func(*frostRetainedGroupHistoryTestExport){
		"missing": func(export *frostRetainedGroupHistoryTestExport) {
			export.omitTransportAttestation = true
		},
		"duplicate": func(export *frostRetainedGroupHistoryTestExport) {
			export.duplicateTransportAttestation = true
		},
		"role": func(export *frostRetainedGroupHistoryTestExport) {
			export.transportAttestationMutator =
				func(attestation *frostRetainedGroupTransportAttestation) {
					attestation.Role = "retained-history-verifier"
				}
		},
		"endpoint fingerprint": func(export *frostRetainedGroupHistoryTestExport) {
			export.transportAttestationMutator =
				func(attestation *frostRetainedGroupTransportAttestation) {
					attestation.EndpointFingerprint =
						frostActivationHex32([32]byte{0xb0})
				}
		},
		"canonical endpoint": func(export *frostRetainedGroupHistoryTestExport) {
			export.transportAttestationMutator =
				func(attestation *frostRetainedGroupTransportAttestation) {
					attestation.CanonicalEndpoint =
						"https://other.example:443/"
				}
		},
		"canonical DNS": func(export *frostRetainedGroupHistoryTestExport) {
			export.transportAttestationMutator =
				func(attestation *frostRetainedGroupTransportAttestation) {
					attestation.CanonicalDNSName = "other.example"
				}
		},
		"resolved DNS": func(export *frostRetainedGroupHistoryTestExport) {
			export.transportAttestationMutator =
				func(attestation *frostRetainedGroupTransportAttestation) {
					attestation.ResolvedDNSName = "other-origin.example"
				}
		},
		"resolved peer": func(export *frostRetainedGroupHistoryTestExport) {
			export.transportAttestationMutator =
				func(attestation *frostRetainedGroupTransportAttestation) {
					attestation.ResolvedPeerIP = "192.0.2.99"
				}
		},
		"TLS leaf": func(export *frostRetainedGroupHistoryTestExport) {
			export.transportAttestationMutator =
				func(attestation *frostRetainedGroupTransportAttestation) {
					attestation.TLSLeafSPKIHash =
						frostActivationHex32([32]byte{0xb8})
				}
		},
		"service identity": func(export *frostRetainedGroupHistoryTestExport) {
			export.transportAttestationMutator =
				func(attestation *frostRetainedGroupTransportAttestation) {
					attestation.ServiceIdentity =
						"spiffe://retained.test/other"
				}
		},
		"backend": func(export *frostRetainedGroupHistoryTestExport) {
			export.transportAttestationMutator =
				func(attestation *frostRetainedGroupTransportAttestation) {
					attestation.BackendServiceFingerprint =
						frostActivationHex32([32]byte{0xb1})
				}
		},
		"backend signer SPKI": func(export *frostRetainedGroupHistoryTestExport) {
			export.transportAttestationMutator =
				func(attestation *frostRetainedGroupTransportAttestation) {
					attestation.BackendSignerPublicKeySPKI =
						base64.StdEncoding.EncodeToString([]byte("other"))
				}
		},
		"backend signature algorithm": func(export *frostRetainedGroupHistoryTestExport) {
			export.transportAttestationMutator =
				func(attestation *frostRetainedGroupTransportAttestation) {
					attestation.BackendSignatureAlgorithm = "other"
				}
		},
		"operator": func(export *frostRetainedGroupHistoryTestExport) {
			export.transportAttestationMutator =
				func(attestation *frostRetainedGroupTransportAttestation) {
					attestation.OperatorFingerprint =
						frostActivationHex32([32]byte{0xb2})
				}
		},
		"operator signer SPKI": func(export *frostRetainedGroupHistoryTestExport) {
			export.transportAttestationMutator =
				func(attestation *frostRetainedGroupTransportAttestation) {
					attestation.OperatorSignerPublicKeySPKI =
						base64.StdEncoding.EncodeToString([]byte("other"))
				}
		},
		"operator signature algorithm": func(export *frostRetainedGroupHistoryTestExport) {
			export.transportAttestationMutator =
				func(attestation *frostRetainedGroupTransportAttestation) {
					attestation.OperatorSignatureAlgorithm = "other"
				}
		},
		"attestation key": func(export *frostRetainedGroupHistoryTestExport) {
			export.transportAttestationMutator =
				func(attestation *frostRetainedGroupTransportAttestation) {
					attestation.AttestationKeyHash =
						frostActivationHex32([32]byte{0xb9})
				}
		},
		"TLS exporter protocol": func(export *frostRetainedGroupHistoryTestExport) {
			export.transportAttestationMutator =
				func(attestation *frostRetainedGroupTransportAttestation) {
					attestation.TLSExporterProtocolID =
						frostActivationHex32([32]byte{0xba})
				}
		},
		"request digest": func(export *frostRetainedGroupHistoryTestExport) {
			export.transportAttestationMutator =
				func(attestation *frostRetainedGroupTransportAttestation) {
					attestation.RequestBodySHA256 =
						frostActivationHex32([32]byte{0xb3})
				}
		},
		"response digest": func(export *frostRetainedGroupHistoryTestExport) {
			export.transportAttestationMutator =
				func(attestation *frostRetainedGroupTransportAttestation) {
					attestation.ResponseBodySHA256 =
						frostActivationHex32([32]byte{0xb4})
				}
		},
		"status": func(export *frostRetainedGroupHistoryTestExport) {
			export.transportAttestationMutator =
				func(attestation *frostRetainedGroupTransportAttestation) {
					attestation.ResponseStatus++
				}
		},
		"challenge": func(export *frostRetainedGroupHistoryTestExport) {
			export.transportAttestationMutator =
				func(attestation *frostRetainedGroupTransportAttestation) {
					attestation.Challenge = frostActivationHex32([32]byte{0xb5})
				}
		},
		"request target": func(export *frostRetainedGroupHistoryTestExport) {
			export.transportAttestationMutator =
				func(attestation *frostRetainedGroupTransportAttestation) {
					attestation.RequestTarget += "/other"
				}
		},
		"exporter context": func(export *frostRetainedGroupHistoryTestExport) {
			export.transportAttestationMutator =
				func(attestation *frostRetainedGroupTransportAttestation) {
					attestation.TLSExporterContextSHA256 =
						frostActivationHex32([32]byte{0xb6})
				}
		},
		"exporter value": func(export *frostRetainedGroupHistoryTestExport) {
			export.transportAttestationMutator =
				func(attestation *frostRetainedGroupTransportAttestation) {
					attestation.TLSExporterValueSHA256 =
						frostActivationHex32([32]byte{0xb7})
				}
		},
		"stale": func(export *frostRetainedGroupHistoryTestExport) {
			export.transportAttestationMutator =
				func(attestation *frostRetainedGroupTransportAttestation) {
					attestation.IssuedAtUnixMs = "1"
					attestation.ExpiresAtUnixMs = "2"
				}
		},
		"overflow timestamp": func(export *frostRetainedGroupHistoryTestExport) {
			export.transportAttestationMutator =
				func(attestation *frostRetainedGroupTransportAttestation) {
					attestation.IssuedAtUnixMs = "18446744073709551614"
					attestation.ExpiresAtUnixMs = "18446744073709551615"
				}
		},
		"signer SPKI": func(export *frostRetainedGroupHistoryTestExport) {
			export.transportAttestationMutator =
				func(attestation *frostRetainedGroupTransportAttestation) {
					attestation.SignerPublicKeySPKI =
						base64.StdEncoding.EncodeToString([]byte("other"))
				}
		},
		"noncanonical signer SPKI": func(export *frostRetainedGroupHistoryTestExport) {
			export.transportAttestationMutator =
				func(attestation *frostRetainedGroupTransportAttestation) {
					attestation.SignerPublicKeySPKI += "\n"
				}
		},
		"signature algorithm": func(export *frostRetainedGroupHistoryTestExport) {
			export.transportAttestationMutator =
				func(attestation *frostRetainedGroupTransportAttestation) {
					attestation.SignatureAlgorithm = "other"
				}
		},
	}
	for name, configure := range testCases {
		t.Run(name, func(t *testing.T) {
			fixture := newFrostRetainedGroupHistorySourceFixture(t)
			configure(fixture.export)
			if err := performFrostRetainedGroupAttestedTestRequest(fixture); err == nil {
				t.Fatal("unbound transport evidence was accepted")
			}
		})
	}
}

func TestFrostRetainedGroupAttestedTransport_RejectsReplayAcrossTLSConnections(
	t *testing.T,
) {
	fixture := newFrostRetainedGroupHistorySourceFixture(t)
	fixture.export.replayTransportAttestation = true
	client := fixture.source.httpClient.(*http.Client)
	attested := client.Transport.(*frostRetainedGroupAttestedRoundTripper)
	challenge := bytes.Repeat([]byte{0x5a}, 32)
	attested.random = bytes.NewReader(append(challenge, challenge...))
	if err := performFrostRetainedGroupAttestedTestRequest(fixture); err != nil {
		t.Fatalf("initial attested request failed: [%v]", err)
	}
	if err := performFrostRetainedGroupAttestedTestRequest(fixture); err == nil ||
		!strings.Contains(err.Error(), "differently bound") {
		t.Fatalf("cross-connection replay was accepted: [%v]", err)
	}
}

func TestFrostRetainedGroupAttestedTransport_RejectsHostAndPathAmbiguity(
	t *testing.T,
) {
	fixture := newFrostRetainedGroupHistorySourceFixture(t)
	client := fixture.source.httpClient.(*http.Client)
	for name, mutate := range map[string]func(*http.Request){
		"host override": func(request *http.Request) {
			request.Host = "proxy.example:443"
		},
		"encoded path": func(request *http.Request) {
			request.URL.Path = "/operator-id"
			request.URL.RawPath = "/%6fperator-id"
		},
		"query": func(request *http.Request) {
			request.URL.RawQuery = "proxy=1"
		},
	} {
		t.Run(name, func(t *testing.T) {
			request, err := http.NewRequest(
				http.MethodPost,
				fixture.server.URL+"/operator-id",
				strings.NewReader("{}"),
			)
			if err != nil {
				t.Fatal(err)
			}
			mutate(request)
			response, err := client.Do(request)
			if response != nil {
				_ = response.Body.Close()
			}
			if err == nil {
				t.Fatal("ambiguous transport target was accepted")
			}
		})
	}

	serverRequest := &http.Request{
		Host: "proxy.example:443",
		URL:  &url.URL{Path: "/history"},
	}
	if _, err := frostRetainedGroupServerRequestTarget(
		fixture.identity.Export,
		serverRequest,
	); err == nil {
		t.Fatal("server accepted a Host header outside the manifest endpoint")
	}
}

func TestVerifyFrostRetainedGroupTLSConnection_RequiresLeafAndServicePins(
	t *testing.T,
) {
	const serviceIdentity = "spiffe://retained-export.example/export"
	server, leaf, _ := newFrostRetainedGroupHistoryTLSTestServer(
		t,
		http.NotFoundHandler(),
		serviceIdentity,
	)
	_ = server
	identity := testFrostRetainedGroupCompleteIdentity().Export
	identity.TLSLeafSPKIHash = sha256.Sum256(leaf.RawSubjectPublicKeyInfo)
	identity.ServiceIdentity = serviceIdentity
	identity.EndpointFingerprint =
		computeFrostRetainedGroupEndpointFingerprint(identity)
	state := tls.ConnectionState{
		Version:            tls.VersionTLS13,
		HandshakeComplete:  true,
		NegotiatedProtocol: "http/1.1",
		PeerCertificates:   []*x509.Certificate{leaf},
		VerifiedChains:     [][]*x509.Certificate{{leaf}},
	}
	if err := verifyFrostRetainedGroupTLSConnection(state, identity); err != nil {
		t.Fatalf("correct TLS identity rejected: [%v]", err)
	}
	missingALPN := state
	missingALPN.NegotiatedProtocol = ""
	if err := verifyFrostRetainedGroupTLSConnection(
		missingALPN,
		identity,
	); err == nil {
		t.Fatal("missing ALPN was accepted")
	}
	wrongALPN := state
	wrongALPN.NegotiatedProtocol = "h2"
	if err := verifyFrostRetainedGroupTLSConnection(
		wrongALPN,
		identity,
	); err == nil {
		t.Fatal("wrong ALPN was accepted")
	}
	wrongTLSVersion := state
	wrongTLSVersion.Version = tls.VersionTLS12
	if err := verifyFrostRetainedGroupTLSConnection(
		wrongTLSVersion,
		identity,
	); err == nil {
		t.Fatal("wrong TLS version was accepted")
	}
	unverified := state
	unverified.VerifiedChains = nil
	if err := verifyFrostRetainedGroupTLSConnection(
		unverified,
		identity,
	); err == nil {
		t.Fatal("unverified TLS chain was accepted")
	}
	wrongLeaf := identity
	wrongLeaf.TLSLeafSPKIHash[31] ^= 0x01
	if err := verifyFrostRetainedGroupTLSConnection(
		state,
		wrongLeaf,
	); err == nil {
		t.Fatal("wrong TLS leaf was accepted")
	}
	wrongService := identity
	wrongService.ServiceIdentity = "spiffe://retained.test/other"
	if err := verifyFrostRetainedGroupTLSConnection(
		state,
		wrongService,
	); err == nil {
		t.Fatal("wrong SPIFFE service identity was accepted")
	}
	secondURI, err := url.Parse("spiffe://retained.test/second")
	if err != nil {
		t.Fatal(err)
	}
	ambiguousLeaf := *leaf
	ambiguousLeaf.URIs = append(
		append([]*url.URL{}, leaf.URIs...),
		secondURI,
	)
	ambiguous := state
	ambiguous.PeerCertificates = []*x509.Certificate{&ambiguousLeaf}
	ambiguous.VerifiedChains = [][]*x509.Certificate{{&ambiguousLeaf}}
	if err := verifyFrostRetainedGroupTLSConnection(
		ambiguous,
		identity,
	); err == nil {
		t.Fatal("ambiguous SPIFFE URI SAN set was accepted")
	}
}

type frostRetainedGroupVerifierAttestationHandler struct {
	t                    *testing.T
	identity             FrostRetainedGroupEndpointIdentity
	privateKey           ed25519.PrivateKey
	publicKeyDER         []byte
	backendPrivateKey    ed25519.PrivateKey
	backendPublicKeyDER  []byte
	operatorPrivateKey   ed25519.PrivateKey
	operatorPublicKeyDER []byte
	omit                 atomic.Bool
}

func (handler *frostRetainedGroupVerifierAttestationHandler) ServeHTTP(
	responseWriter http.ResponseWriter,
	request *http.Request,
) {
	requestBody, err := io.ReadAll(request.Body)
	if err != nil {
		handler.t.Fatal(err)
	}
	request.Body = io.NopCloser(bytes.NewReader(requestBody))
	rpcRequest := struct {
		ID json.RawMessage `json:"id"`
	}{}
	if err := json.Unmarshal(requestBody, &rpcRequest); err != nil {
		handler.t.Fatal(err)
	}
	responseBody, err := json.Marshal(struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Result  string          `json:"result"`
	}{
		JSONRPC: "2.0",
		ID:      rpcRequest.ID,
		Result:  "0x1",
	})
	if err != nil {
		handler.t.Fatal(err)
	}
	if !handler.omit.Load() {
		localAddress, ok := request.Context().Value(
			http.LocalAddrContextKey,
		).(net.Addr)
		if !ok {
			handler.t.Fatal("missing verifier local address")
		}
		localIP, err := frostRetainedGroupRemoteIP(localAddress)
		if err != nil {
			handler.t.Fatal(err)
		}
		attestation, err := marshalFrostRetainedGroupTransportAttestation(
			request,
			http.StatusOK,
			responseBody,
			handler.identity,
			handler.privateKey,
			handler.publicKeyDER,
			handler.backendPrivateKey,
			handler.backendPublicKeyDER,
			handler.operatorPrivateKey,
			handler.operatorPublicKeyDER,
			time.Now(),
			localIP,
		)
		if err != nil {
			handler.t.Fatal(err)
		}
		responseWriter.Header().Set(
			frostRetainedGroupTransportAttestationHeader,
			attestation,
		)
	}
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	if _, err := responseWriter.Write(responseBody); err != nil {
		handler.t.Fatal(err)
	}
}

func TestFrostRetainedGroupVerifierRPC_RequiresPerResponseConformanceAttestation(
	t *testing.T,
) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	publicKeyDER, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		t.Fatal(err)
	}
	backendPublicKey, backendPrivateKey, err :=
		ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	backendPublicKeyDER, err := x509.MarshalPKIXPublicKey(backendPublicKey)
	if err != nil {
		t.Fatal(err)
	}
	operatorPublicKey, operatorPrivateKey, err :=
		ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	operatorPublicKeyDER, err := x509.MarshalPKIXPublicKey(operatorPublicKey)
	if err != nil {
		t.Fatal(err)
	}
	handler := &frostRetainedGroupVerifierAttestationHandler{
		t:                    t,
		privateKey:           privateKey,
		publicKeyDER:         publicKeyDER,
		backendPrivateKey:    backendPrivateKey,
		backendPublicKeyDER:  backendPublicKeyDER,
		operatorPrivateKey:   operatorPrivateKey,
		operatorPublicKeyDER: operatorPublicKeyDER,
	}
	const serviceIdentity = "spiffe://retained-verifier.example/verifier"
	server, leaf, roots := newFrostRetainedGroupHistoryTLSTestServer(
		t,
		handler,
		serviceIdentity,
	)
	endpointURL, canonicalEndpoint, err :=
		validateFrostRetainedGroupTLSEndpoint(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	resolvedEndpoint, err := resolveFrostRetainedGroupEndpoint(
		context.Background(),
		endpointURL,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	identity := testFrostRetainedGroupCompleteIdentity().Verifier
	identity.CanonicalEndpoint = canonicalEndpoint
	identity.CanonicalDNSName = resolvedEndpoint.canonicalDNSName
	identity.ResolvedDNSName = resolvedEndpoint.resolvedDNSName
	identity.ResolvedAddressSetHash = resolvedEndpoint.addressSetHash
	identity.TLSLeafSPKIHash = sha256.Sum256(leaf.RawSubjectPublicKeyInfo)
	identity.ServiceIdentity = serviceIdentity
	identity.BackendServiceFingerprint = sha256.Sum256(backendPublicKeyDER)
	identity.OperatorFingerprint = sha256.Sum256(operatorPublicKeyDER)
	identity.AttestationKeyHash = sha256.Sum256(publicKeyDER)
	identity.EndpointFingerprint =
		computeFrostRetainedGroupEndpointFingerprint(identity)
	handler.identity = identity
	client, transport, err := newFrostRetainedGroupAttestedHTTPClient(
		resolvedEndpoint,
		identity,
		roots,
		time.Second,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer transport.CloseIdleConnections()
	rpcClient, err := rpc.DialOptions(
		context.Background(),
		canonicalEndpoint,
		rpc.WithHTTPClient(client),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer rpcClient.Close()
	var result string
	if err := rpcClient.CallContext(
		context.Background(),
		&result,
		"eth_chainId",
	); err != nil || result != "0x1" {
		t.Fatalf("attested verifier RPC failed: [%s] [%v]", result, err)
	}
	handler.omit.Store(true)
	if err := rpcClient.CallContext(
		context.Background(),
		&result,
		"eth_chainId",
	); err == nil || !strings.Contains(err.Error(), "attestation") {
		t.Fatalf("generic unattested RPC response was accepted: [%v]", err)
	}
}

func TestFrostRetainedGroupTLSExporterProtocolIDFrozen(t *testing.T) {
	const expected = "42e4447e7981f7691f5d7a0f93fa5500bbc3dda7e58438a9db475ae6c4e39b5c"
	protocolID := frostRetainedGroupTLSExporterProtocolID()
	actual := hex.EncodeToString(protocolID[:])
	if actual != expected {
		t.Fatalf("frozen TLS exporter protocol ID changed: [%s]", actual)
	}
}
