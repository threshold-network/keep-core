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
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/netip"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	frostRetainedGroupEndpointIdentitySchema           = "tbtc-frost-retained-group-endpoint-identity/v1"
	frostRetainedGroupSourceIdentitySchema             = "tbtc-frost-retained-group-source-identity/v1"
	frostRetainedGroupEndpointIdentityDomain           = "tbtc-frost-retained-group-endpoint-identity/v1\x00"
	frostRetainedGroupSourceIdentityDomain             = "tbtc-frost-retained-group-source-identity/v1\x00"
	frostRetainedGroupResolvedAddressSetDomain         = "tbtc-frost-retained-group-resolved-address-set/v1\x00"
	frostRetainedGroupTransportAttestationSchema       = "tbtc-frost-retained-group-transport-attestation/v1"
	frostRetainedGroupTransportAttestationDomain       = "tbtc-frost-retained-group-transport-attestation/v1\x00"
	frostRetainedGroupBackendAttestationDomain         = "tbtc-frost-retained-group-backend-attestation/v1\x00"
	frostRetainedGroupOperatorAttestationDomain        = "tbtc-frost-retained-group-operator-attestation/v1\x00"
	frostRetainedGroupTLSExporterProtocolDomain        = "tbtc-frost-retained-group-tls-exporter-protocol/v1\x00"
	frostRetainedGroupTLSExporterContextDomain         = "tbtc-frost-retained-group-tls-exporter-context/v1\x00"
	frostRetainedGroupTLSExporterValueDomain           = "tbtc-frost-retained-group-tls-exporter-value/v1\x00"
	frostRetainedGroupTLSExporterLabel                 = "EXPORTER-tbtc-frost-retained-group-v1"
	frostRetainedGroupTransportAttestationHeader       = "Tbtc-Retained-Transport-Attestation"
	frostRetainedGroupTransportChallengeHeader         = "Tbtc-Retained-Transport-Challenge"
	frostRetainedGroupMaximumTransportAttestationBytes = 16 * 1024
	frostRetainedGroupMaximumTransportBodyBytes        = 16 * 1024 * 1024
	frostRetainedGroupMaximumResolvedAddresses         = 16
	frostRetainedGroupTransportAttestationLifetime     = 30 * time.Second
	frostRetainedGroupTransportClockSkew               = 5 * time.Second
)

// FrostRetainedGroupEndpointIdentity is one manifest-authenticated endpoint
// role. EndpointFingerprint is derived from every other field by the frozen
// v1 transcript; it is never an operator-authored opaque label.
type FrostRetainedGroupEndpointIdentity struct {
	Schema                    string   `json:"schema"`
	Role                      string   `json:"role"`
	TrustDomainID             string   `json:"trustDomainID"`
	CanonicalEndpoint         string   `json:"canonicalEndpoint"`
	CanonicalDNSName          string   `json:"canonicalDNSName"`
	ResolvedDNSName           string   `json:"resolvedDNSName"`
	ResolvedAddressSetHash    [32]byte `json:"resolvedAddressSetHash"`
	TLSLeafSPKIHash           [32]byte `json:"tlsLeafSpkiHash"`
	ServiceIdentity           string   `json:"serviceIdentity"`
	BackendServiceFingerprint [32]byte `json:"backendServiceFingerprint"`
	OperatorFingerprint       [32]byte `json:"operatorFingerprint"`
	AttestationKeyHash        [32]byte `json:"attestationKeyHash"`
	TLSExporterProtocolID     [32]byte `json:"tlsExporterProtocolID"`
	EndpointFingerprint       [32]byte `json:"endpointFingerprint"`
}

// FrostRetainedGroupHistoryIdentity is the complete export/verifier trust
// boundary committed by the signed activation manifest.
type FrostRetainedGroupHistoryIdentity struct {
	Schema               string                             `json:"schema"`
	TrustDomainID        string                             `json:"trustDomainID"`
	EndpointFingerprint  [32]byte                           `json:"endpointFingerprint"`
	OperatorFingerprint  [32]byte                           `json:"operatorFingerprint"`
	HistorySignerKeyHash [32]byte                           `json:"historySignerKeyHash"`
	Export               FrostRetainedGroupEndpointIdentity `json:"export"`
	Verifier             FrostRetainedGroupEndpointIdentity `json:"verifier"`
}

type frostRetainedGroupWireEndpointIdentity struct {
	Schema                    string `json:"schema"`
	Role                      string `json:"role"`
	TrustDomainID             string `json:"trustDomainID"`
	CanonicalEndpoint         string `json:"canonicalEndpoint"`
	CanonicalDNSName          string `json:"canonicalDNSName"`
	ResolvedDNSName           string `json:"resolvedDNSName"`
	ResolvedAddressSetHash    string `json:"resolvedAddressSetHash"`
	TLSLeafSPKIHash           string `json:"tlsLeafSpkiHash"`
	ServiceIdentity           string `json:"serviceIdentity"`
	BackendServiceFingerprint string `json:"backendServiceFingerprint"`
	OperatorFingerprint       string `json:"operatorFingerprint"`
	AttestationKeyHash        string `json:"attestationKeyHash"`
	TLSExporterProtocolID     string `json:"tlsExporterProtocolID"`
	EndpointFingerprint       string `json:"endpointFingerprint"`
}

type frostRetainedGroupWireIdentity struct {
	Schema               string                                 `json:"schema"`
	TrustDomainID        string                                 `json:"trustDomainID"`
	EndpointFingerprint  string                                 `json:"endpointFingerprint"`
	OperatorFingerprint  string                                 `json:"operatorFingerprint"`
	HistorySignerKeyHash string                                 `json:"historySignerKeyHash"`
	Export               frostRetainedGroupWireEndpointIdentity `json:"export"`
	Verifier             frostRetainedGroupWireEndpointIdentity `json:"verifier"`
}

type frostRetainedGroupResolvedEndpoint struct {
	endpoint         *url.URL
	canonical        string
	canonicalDNSName string
	resolvedDNSName  string
	addresses        []netip.Addr
	addressSetHash   [32]byte
}

type frostRetainedGroupTransportAttestation struct {
	Schema                      string `json:"schema"`
	Role                        string `json:"role"`
	EndpointFingerprint         string `json:"endpointFingerprint"`
	CanonicalEndpoint           string `json:"canonicalEndpoint"`
	CanonicalDNSName            string `json:"canonicalDNSName"`
	ResolvedDNSName             string `json:"resolvedDNSName"`
	ResolvedPeerIP              string `json:"resolvedPeerIP"`
	TLSLeafSPKIHash             string `json:"tlsLeafSpkiHash"`
	ServiceIdentity             string `json:"serviceIdentity"`
	BackendServiceFingerprint   string `json:"backendServiceFingerprint"`
	OperatorFingerprint         string `json:"operatorFingerprint"`
	AttestationKeyHash          string `json:"attestationKeyHash"`
	TLSExporterProtocolID       string `json:"tlsExporterProtocolID"`
	Challenge                   string `json:"challenge"`
	RequestMethod               string `json:"requestMethod"`
	RequestTarget               string `json:"requestTarget"`
	RequestBodySHA256           string `json:"requestBodySha256"`
	ResponseStatus              uint64 `json:"responseStatus"`
	ResponseBodySHA256          string `json:"responseBodySha256"`
	IssuedAtUnixMs              string `json:"issuedAtUnixMs"`
	ExpiresAtUnixMs             string `json:"expiresAtUnixMs"`
	TLSExporterContextSHA256    string `json:"tlsExporterContextSha256"`
	TLSExporterValueSHA256      string `json:"tlsExporterValueSha256"`
	BackendSignerPublicKeySPKI  string `json:"backendSignerPublicKeySpki"`
	BackendSignatureAlgorithm   string `json:"backendSignatureAlgorithm"`
	BackendSignature            string `json:"backendSignature"`
	OperatorSignerPublicKeySPKI string `json:"operatorSignerPublicKeySpki"`
	OperatorSignatureAlgorithm  string `json:"operatorSignatureAlgorithm"`
	OperatorSignature           string `json:"operatorSignature"`
	SignerPublicKeySPKI         string `json:"signerPublicKeySpki"`
	SignatureAlgorithm          string `json:"signatureAlgorithm"`
	Signature                   string `json:"signature"`
}

type frostRetainedGroupTransportProof struct {
	role           string
	requestDigest  [32]byte
	responseDigest [32]byte
	challenge      [32]byte
}

type frostRetainedGroupTransportProofKey struct{}

type frostRetainedGroupAttestedRoundTripper struct {
	base             http.RoundTripper
	endpoint         frostRetainedGroupResolvedEndpoint
	identity         FrostRetainedGroupEndpointIdentity
	random           io.Reader
	now              func() time.Time
	maximumBodyBytes int64
	maximumClockSkew time.Duration
	maximumLifetime  time.Duration
}

type frostRetainedGroupValidatedSourceConfig struct {
	exportEndpoint   frostRetainedGroupResolvedEndpoint
	verifierEndpoint frostRetainedGroupResolvedEndpoint
	primaryEndpoint  frostRetainedGroupResolvedEndpoint
	identity         FrostRetainedGroupHistoryIdentity
	requestTimeout   time.Duration
	rootCAs          *x509.CertPool
}

type frostRetainedGroupResolver interface {
	LookupCNAME(context.Context, string) (string, error)
	LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
}

type frostRetainedGroupIndependenceMonitor struct {
	exportEndpoint   frostRetainedGroupResolvedEndpoint
	verifierEndpoint frostRetainedGroupResolvedEndpoint
	primaryEndpoint  frostRetainedGroupResolvedEndpoint
	resolver         frostRetainedGroupResolver
	timeout          time.Duration
}

func frostRetainedGroupTLSExporterProtocolID() [32]byte {
	return sha256.Sum256([]byte(frostRetainedGroupTLSExporterProtocolDomain))
}

// FrostRetainedGroupTLSExporterProtocolID is the compile-time protocol
// identity that every signed manifest endpoint descriptor must commit.
func FrostRetainedGroupTLSExporterProtocolID() [32]byte {
	return frostRetainedGroupTLSExporterProtocolID()
}

// ComputeFrostRetainedGroupEndpointIdentityFingerprint computes the frozen v1
// endpoint-descriptor transcript.
func ComputeFrostRetainedGroupEndpointIdentityFingerprint(
	identity FrostRetainedGroupEndpointIdentity,
) [32]byte {
	return computeFrostRetainedGroupEndpointFingerprint(identity)
}

// ComputeFrostRetainedGroupSourceEndpointFingerprint computes the frozen v1
// aggregate export/verifier transcript.
func ComputeFrostRetainedGroupSourceEndpointFingerprint(
	identity FrostRetainedGroupHistoryIdentity,
) [32]byte {
	return computeFrostRetainedGroupSourceEndpointFingerprint(identity)
}

// ValidateFrostRetainedGroupHistoryIdentity checks the complete endpoint-role
// separation and every derived transcript.
func ValidateFrostRetainedGroupHistoryIdentity(
	identity FrostRetainedGroupHistoryIdentity,
) error {
	return validateFrostRetainedGroupHistoryIdentity(identity)
}

func frostRetainedGroupIdentityTranscript(domain string) *frostRetainedGroupTranscript {
	transcript := &frostRetainedGroupTranscript{hasher: sha256.New()}
	_, _ = transcript.hasher.Write([]byte(domain))
	return transcript
}

type frostRetainedGroupTranscript struct {
	hasher hash.Hash
}

func (transcript *frostRetainedGroupTranscript) field(
	name string,
	value []byte,
) {
	buffer := [8]byte{}
	binary.BigEndian.PutUint64(buffer[:], uint64(len(name)))
	_, _ = transcript.hasher.Write(buffer[:])
	_, _ = transcript.hasher.Write([]byte(name))
	binary.BigEndian.PutUint64(buffer[:], uint64(len(value)))
	_, _ = transcript.hasher.Write(buffer[:])
	_, _ = transcript.hasher.Write(value)
}

func (transcript *frostRetainedGroupTranscript) text(name string, value string) {
	transcript.field(name, []byte(value))
}

func (transcript *frostRetainedGroupTranscript) bytes32(
	name string,
	value [32]byte,
) {
	transcript.field(name, value[:])
}

func (transcript *frostRetainedGroupTranscript) uint64(
	name string,
	value uint64,
) {
	buffer := [8]byte{}
	binary.BigEndian.PutUint64(buffer[:], value)
	transcript.field(name, buffer[:])
}

func (transcript *frostRetainedGroupTranscript) sum() [32]byte {
	var result [32]byte
	copy(result[:], transcript.hasher.Sum(nil))
	return result
}

func computeFrostRetainedGroupEndpointFingerprint(
	identity FrostRetainedGroupEndpointIdentity,
) [32]byte {
	transcript := frostRetainedGroupIdentityTranscript(
		frostRetainedGroupEndpointIdentityDomain,
	)
	transcript.text("schema", identity.Schema)
	transcript.text("role", identity.Role)
	transcript.text("trustDomainID", identity.TrustDomainID)
	transcript.text("canonicalEndpoint", identity.CanonicalEndpoint)
	transcript.text("canonicalDNSName", identity.CanonicalDNSName)
	transcript.text("resolvedDNSName", identity.ResolvedDNSName)
	transcript.bytes32("resolvedAddressSetHash", identity.ResolvedAddressSetHash)
	transcript.bytes32("tlsLeafSpkiHash", identity.TLSLeafSPKIHash)
	transcript.text("serviceIdentity", identity.ServiceIdentity)
	transcript.bytes32(
		"backendServiceFingerprint",
		identity.BackendServiceFingerprint,
	)
	transcript.bytes32("operatorFingerprint", identity.OperatorFingerprint)
	transcript.bytes32("attestationKeyHash", identity.AttestationKeyHash)
	transcript.bytes32(
		"tlsExporterProtocolID",
		identity.TLSExporterProtocolID,
	)
	return transcript.sum()
}

func computeFrostRetainedGroupSourceEndpointFingerprint(
	identity FrostRetainedGroupHistoryIdentity,
) [32]byte {
	transcript := frostRetainedGroupIdentityTranscript(
		frostRetainedGroupSourceIdentityDomain,
	)
	transcript.text("schema", identity.Schema)
	transcript.text("trustDomainID", identity.TrustDomainID)
	transcript.bytes32(
		"operatorFingerprint",
		identity.OperatorFingerprint,
	)
	transcript.bytes32(
		"historySignerKeyHash",
		identity.HistorySignerKeyHash,
	)
	transcript.bytes32(
		"exportEndpointFingerprint",
		identity.Export.EndpointFingerprint,
	)
	transcript.bytes32(
		"verifierEndpointFingerprint",
		identity.Verifier.EndpointFingerprint,
	)
	return transcript.sum()
}

func frostRetainedGroupEndpointIdentityToWire(
	identity FrostRetainedGroupEndpointIdentity,
) frostRetainedGroupWireEndpointIdentity {
	return frostRetainedGroupWireEndpointIdentity{
		Schema:                    identity.Schema,
		Role:                      identity.Role,
		TrustDomainID:             identity.TrustDomainID,
		CanonicalEndpoint:         identity.CanonicalEndpoint,
		CanonicalDNSName:          identity.CanonicalDNSName,
		ResolvedDNSName:           identity.ResolvedDNSName,
		ResolvedAddressSetHash:    frostActivationHex32(identity.ResolvedAddressSetHash),
		TLSLeafSPKIHash:           frostActivationHex32(identity.TLSLeafSPKIHash),
		ServiceIdentity:           identity.ServiceIdentity,
		BackendServiceFingerprint: frostActivationHex32(identity.BackendServiceFingerprint),
		OperatorFingerprint:       frostActivationHex32(identity.OperatorFingerprint),
		AttestationKeyHash:        frostActivationHex32(identity.AttestationKeyHash),
		TLSExporterProtocolID:     frostActivationHex32(identity.TLSExporterProtocolID),
		EndpointFingerprint:       frostActivationHex32(identity.EndpointFingerprint),
	}
}

func frostRetainedGroupIdentityToWire(
	identity FrostRetainedGroupHistoryIdentity,
) frostRetainedGroupWireIdentity {
	return frostRetainedGroupWireIdentity{
		Schema:               identity.Schema,
		TrustDomainID:        identity.TrustDomainID,
		EndpointFingerprint:  frostActivationHex32(identity.EndpointFingerprint),
		OperatorFingerprint:  frostActivationHex32(identity.OperatorFingerprint),
		HistorySignerKeyHash: frostActivationHex32(identity.HistorySignerKeyHash),
		Export: frostRetainedGroupEndpointIdentityToWire(
			identity.Export,
		),
		Verifier: frostRetainedGroupEndpointIdentityToWire(
			identity.Verifier,
		),
	}
}

func frostRetainedGroupEndpointIdentityFromWire(
	wire frostRetainedGroupWireEndpointIdentity,
) (FrostRetainedGroupEndpointIdentity, error) {
	parse := func(name string, value string) ([32]byte, error) {
		parsed, err := parseFrostActivationHex32(value)
		if err != nil {
			return [32]byte{}, fmt.Errorf("invalid %s: [%w]", name, err)
		}
		return parsed, nil
	}
	addressSetHash, err := parse(
		"resolved address-set hash",
		wire.ResolvedAddressSetHash,
	)
	if err != nil {
		return FrostRetainedGroupEndpointIdentity{}, err
	}
	leafHash, err := parse("TLS leaf SPKI hash", wire.TLSLeafSPKIHash)
	if err != nil {
		return FrostRetainedGroupEndpointIdentity{}, err
	}
	backend, err := parse(
		"backend service fingerprint",
		wire.BackendServiceFingerprint,
	)
	if err != nil {
		return FrostRetainedGroupEndpointIdentity{}, err
	}
	operator, err := parse("operator fingerprint", wire.OperatorFingerprint)
	if err != nil {
		return FrostRetainedGroupEndpointIdentity{}, err
	}
	attestation, err := parse("attestation key hash", wire.AttestationKeyHash)
	if err != nil {
		return FrostRetainedGroupEndpointIdentity{}, err
	}
	exporter, err := parse(
		"TLS exporter protocol ID",
		wire.TLSExporterProtocolID,
	)
	if err != nil {
		return FrostRetainedGroupEndpointIdentity{}, err
	}
	fingerprint, err := parse(
		"endpoint fingerprint",
		wire.EndpointFingerprint,
	)
	if err != nil {
		return FrostRetainedGroupEndpointIdentity{}, err
	}
	result := FrostRetainedGroupEndpointIdentity{
		Schema:                    wire.Schema,
		Role:                      wire.Role,
		TrustDomainID:             wire.TrustDomainID,
		CanonicalEndpoint:         wire.CanonicalEndpoint,
		CanonicalDNSName:          wire.CanonicalDNSName,
		ResolvedDNSName:           wire.ResolvedDNSName,
		ResolvedAddressSetHash:    addressSetHash,
		TLSLeafSPKIHash:           leafHash,
		ServiceIdentity:           wire.ServiceIdentity,
		BackendServiceFingerprint: backend,
		OperatorFingerprint:       operator,
		AttestationKeyHash:        attestation,
		TLSExporterProtocolID:     exporter,
		EndpointFingerprint:       fingerprint,
	}
	if err := validateFrostRetainedGroupEndpointIdentity(result); err != nil {
		return FrostRetainedGroupEndpointIdentity{}, err
	}
	return result, nil
}

func frostRetainedGroupIdentityFromWire(
	wire frostRetainedGroupWireIdentity,
) (FrostRetainedGroupHistoryIdentity, error) {
	endpointFingerprint, err := parseFrostActivationHex32(
		wire.EndpointFingerprint,
	)
	if err != nil {
		return FrostRetainedGroupHistoryIdentity{}, err
	}
	operatorFingerprint, err := parseFrostActivationHex32(
		wire.OperatorFingerprint,
	)
	if err != nil {
		return FrostRetainedGroupHistoryIdentity{}, err
	}
	historySignerKeyHash, err := parseFrostActivationHex32(
		wire.HistorySignerKeyHash,
	)
	if err != nil {
		return FrostRetainedGroupHistoryIdentity{}, err
	}
	exportIdentity, err := frostRetainedGroupEndpointIdentityFromWire(wire.Export)
	if err != nil {
		return FrostRetainedGroupHistoryIdentity{}, err
	}
	verifierIdentity, err := frostRetainedGroupEndpointIdentityFromWire(
		wire.Verifier,
	)
	if err != nil {
		return FrostRetainedGroupHistoryIdentity{}, err
	}
	result := FrostRetainedGroupHistoryIdentity{
		Schema:               wire.Schema,
		TrustDomainID:        wire.TrustDomainID,
		EndpointFingerprint:  endpointFingerprint,
		OperatorFingerprint:  operatorFingerprint,
		HistorySignerKeyHash: historySignerKeyHash,
		Export:               exportIdentity,
		Verifier:             verifierIdentity,
	}
	if err := validateFrostRetainedGroupHistoryIdentity(result); err != nil {
		return FrostRetainedGroupHistoryIdentity{}, err
	}
	return result, nil
}

func validateFrostRetainedGroupEndpointIdentity(
	identity FrostRetainedGroupEndpointIdentity,
) error {
	if identity.Schema != frostRetainedGroupEndpointIdentitySchema ||
		(identity.Role != "retained-history-export" &&
			identity.Role != "retained-history-verifier") ||
		!validFrostRetainedGroupIdentityLabel(identity.TrustDomainID) ||
		identity.CanonicalEndpoint == "" ||
		identity.CanonicalDNSName == "" ||
		identity.ResolvedDNSName == "" ||
		identity.ResolvedAddressSetHash == [32]byte{} ||
		identity.TLSLeafSPKIHash == [32]byte{} ||
		identity.BackendServiceFingerprint == [32]byte{} ||
		identity.OperatorFingerprint == [32]byte{} ||
		identity.AttestationKeyHash == [32]byte{} ||
		identity.TLSExporterProtocolID !=
			frostRetainedGroupTLSExporterProtocolID() ||
		identity.EndpointFingerprint == [32]byte{} {
		return fmt.Errorf("retained-group endpoint identity is incomplete")
	}
	endpoint, canonical, err := validateFrostRetainedGroupTLSEndpoint(
		identity.CanonicalEndpoint,
	)
	if err != nil || canonical != identity.CanonicalEndpoint ||
		endpoint.Hostname() != identity.CanonicalDNSName ||
		!validFrostRetainedGroupEndpointHostname(identity.ResolvedDNSName) {
		return fmt.Errorf("retained-group endpoint identity is not canonical")
	}
	if err := validateFrostRetainedGroupServiceIdentity(
		identity.ServiceIdentity,
	); err != nil {
		return err
	}
	serviceIdentity, err := url.Parse(identity.ServiceIdentity)
	if err != nil || serviceIdentity.Hostname() != identity.TrustDomainID {
		return fmt.Errorf(
			"retained-group endpoint trust domain differs from its SPIFFE authority",
		)
	}
	if computeFrostRetainedGroupEndpointFingerprint(identity) !=
		identity.EndpointFingerprint {
		return fmt.Errorf("retained-group endpoint fingerprint mismatch")
	}
	roleHashes := map[[32]byte]string{}
	for name, value := range map[string][32]byte{
		"TLS leaf":           identity.TLSLeafSPKIHash,
		"backend service":    identity.BackendServiceFingerprint,
		"operator":           identity.OperatorFingerprint,
		"attestation signer": identity.AttestationKeyHash,
	} {
		if previous, ok := roleHashes[value]; ok {
			return fmt.Errorf(
				"retained-group endpoint reuses %s identity as %s",
				name,
				previous,
			)
		}
		roleHashes[value] = name
	}
	return nil
}

func validateFrostRetainedGroupHistoryIdentity(
	identity FrostRetainedGroupHistoryIdentity,
) error {
	if identity.Schema != frostRetainedGroupSourceIdentitySchema ||
		!validFrostRetainedGroupIdentityLabel(identity.TrustDomainID) ||
		identity.EndpointFingerprint == [32]byte{} ||
		identity.OperatorFingerprint == [32]byte{} ||
		identity.HistorySignerKeyHash == [32]byte{} ||
		identity.Export.Role != "retained-history-export" ||
		identity.Verifier.Role != "retained-history-verifier" ||
		identity.OperatorFingerprint != identity.Export.OperatorFingerprint {
		return fmt.Errorf("retained-group source identity is incomplete")
	}
	if err := validateFrostRetainedGroupEndpointIdentity(identity.Export); err != nil {
		return fmt.Errorf("invalid retained-group export identity: [%w]", err)
	}
	if err := validateFrostRetainedGroupEndpointIdentity(identity.Verifier); err != nil {
		return fmt.Errorf("invalid retained-group verifier identity: [%w]", err)
	}
	if identity.Export.TrustDomainID == identity.Verifier.TrustDomainID ||
		identity.TrustDomainID == identity.Export.TrustDomainID ||
		identity.TrustDomainID == identity.Verifier.TrustDomainID ||
		identity.Export.CanonicalEndpoint == identity.Verifier.CanonicalEndpoint ||
		identity.Export.CanonicalDNSName == identity.Verifier.CanonicalDNSName ||
		identity.Export.ResolvedDNSName == identity.Verifier.ResolvedDNSName ||
		identity.Export.ResolvedAddressSetHash ==
			identity.Verifier.ResolvedAddressSetHash ||
		identity.Export.ServiceIdentity == identity.Verifier.ServiceIdentity {
		return fmt.Errorf("retained-group export and verifier identities are aliased")
	}
	roleHashes := map[[32]byte]string{}
	for name, value := range map[string][32]byte{
		"export endpoint":      identity.Export.EndpointFingerprint,
		"export TLS leaf":      identity.Export.TLSLeafSPKIHash,
		"export backend":       identity.Export.BackendServiceFingerprint,
		"export operator":      identity.Export.OperatorFingerprint,
		"export attestation":   identity.Export.AttestationKeyHash,
		"history signer":       identity.HistorySignerKeyHash,
		"verifier endpoint":    identity.Verifier.EndpointFingerprint,
		"verifier TLS leaf":    identity.Verifier.TLSLeafSPKIHash,
		"verifier backend":     identity.Verifier.BackendServiceFingerprint,
		"verifier operator":    identity.Verifier.OperatorFingerprint,
		"verifier attestation": identity.Verifier.AttestationKeyHash,
	} {
		if previous, ok := roleHashes[value]; ok {
			return fmt.Errorf(
				"retained-group source reuses %s identity as %s",
				name,
				previous,
			)
		}
		roleHashes[value] = name
	}
	if computeFrostRetainedGroupSourceEndpointFingerprint(identity) !=
		identity.EndpointFingerprint {
		return fmt.Errorf("retained-group source endpoint fingerprint mismatch")
	}
	return nil
}

func validFrostRetainedGroupIdentityLabel(value string) bool {
	return value != "" &&
		value == strings.TrimSpace(value) &&
		len(value) <= 128 &&
		!strings.ContainsAny(value, "\x00\r\n\t")
}

func validateFrostRetainedGroupServiceIdentity(value string) error {
	if value == "" || value != strings.TrimSpace(value) || len(value) > 2048 {
		return fmt.Errorf("retained-group service identity is invalid")
	}
	identity, err := url.Parse(value)
	if err != nil || identity.Scheme != "spiffe" ||
		identity.Host == "" || identity.User != nil ||
		identity.Port() != "" || identity.Host != identity.Hostname() ||
		!validFrostRetainedGroupSPIFFETrustDomain(identity.Hostname()) ||
		identity.RawQuery != "" || identity.Fragment != "" ||
		identity.RawPath != "" || identity.Host != strings.ToLower(identity.Host) ||
		identity.Path == "" || identity.Path == "/" ||
		path.Clean(identity.Path) != identity.Path ||
		strings.HasSuffix(identity.Path, "/") ||
		strings.Contains(identity.Path, "//") ||
		identity.String() != value {
		return fmt.Errorf("retained-group service identity is not a canonical SPIFFE URI")
	}
	for _, segment := range strings.Split(strings.TrimPrefix(identity.Path, "/"), "/") {
		if segment == "" || segment == "." || segment == ".." {
			return fmt.Errorf("retained-group service identity is not a canonical SPIFFE URI")
		}
		for _, character := range segment {
			if (character < 'a' || character > 'z') &&
				(character < 'A' || character > 'Z') &&
				(character < '0' || character > '9') &&
				!strings.ContainsRune("-._", character) {
				return fmt.Errorf("retained-group service identity is not a canonical SPIFFE URI")
			}
		}
	}
	return nil
}

func validFrostRetainedGroupSPIFFETrustDomain(value string) bool {
	if value == "" || len(value) > 255 || value != strings.ToLower(value) {
		return false
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') &&
			(character < '0' || character > '9') &&
			!strings.ContainsRune(".-_", character) {
			return false
		}
	}
	return true
}

func validateFrostRetainedGroupTLSEndpoint(
	raw string,
) (*url.URL, string, error) {
	if raw == "" || raw != strings.TrimSpace(raw) {
		return nil, "", fmt.Errorf("URL is empty or has surrounding whitespace")
	}
	endpoint, err := url.Parse(raw)
	if err != nil || endpoint.Scheme != "https" || endpoint.Host == "" ||
		endpoint.User != nil || endpoint.Fragment != "" ||
		endpoint.RawQuery != "" || endpoint.Opaque != "" ||
		endpoint.RawPath != "" || strings.Contains(raw, "\\") ||
		strings.Contains(endpoint.EscapedPath(), "%") {
		return nil, "", fmt.Errorf("URL is not an unambiguous HTTPS endpoint")
	}
	hostname := endpoint.Hostname()
	if hostname == "" || hostname != strings.ToLower(hostname) ||
		strings.HasSuffix(hostname, ".") ||
		!validFrostRetainedGroupEndpointHostname(hostname) {
		return nil, "", fmt.Errorf("URL hostname is not canonical")
	}
	port := endpoint.Port()
	if port == "" {
		return nil, "", fmt.Errorf("HTTPS endpoint must use an explicit port")
	}
	parsedPort, err := strconv.ParseUint(port, 10, 16)
	if err != nil || parsedPort == 0 || strconv.FormatUint(parsedPort, 10) != port {
		return nil, "", fmt.Errorf("URL port is not canonical")
	}
	expectedHost := net.JoinHostPort(hostname, port)
	if endpoint.Host != expectedHost {
		return nil, "", fmt.Errorf("URL authority is not canonical")
	}
	if endpoint.Path == "" {
		endpoint.Path = "/"
	}
	if !strings.HasPrefix(endpoint.Path, "/") ||
		path.Clean(endpoint.Path) != endpoint.Path ||
		(endpoint.Path != "/" && strings.HasSuffix(endpoint.Path, "/")) ||
		strings.Contains(endpoint.Path, "//") {
		return nil, "", fmt.Errorf("URL path is not canonical")
	}
	canonical := endpoint.String()
	if canonical != raw {
		return nil, "", fmt.Errorf("URL is not in canonical form")
	}
	return endpoint, canonical, nil
}

func validFrostRetainedGroupEndpointHostname(hostname string) bool {
	if parsed, err := netip.ParseAddr(hostname); err == nil {
		return parsed.Zone() == "" && parsed.String() == hostname
	}
	if len(hostname) > 253 {
		return false
	}
	labels := strings.Split(hostname, ".")
	if len(labels) < 2 {
		return false
	}
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 ||
			label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, character := range label {
			if (character < 'a' || character > 'z') &&
				(character < '0' || character > '9') &&
				character != '-' {
				return false
			}
		}
	}
	return true
}

func resolveFrostRetainedGroupEndpoint(
	ctx context.Context,
	endpoint *url.URL,
	resolver frostRetainedGroupResolver,
) (frostRetainedGroupResolvedEndpoint, error) {
	if ctx == nil || endpoint == nil {
		return frostRetainedGroupResolvedEndpoint{},
			fmt.Errorf("retained-group endpoint resolution is incomplete")
	}
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	hostname := endpoint.Hostname()
	addresses := make([]netip.Addr, 0)
	canonicalDNSName := hostname
	if parsed, err := netip.ParseAddr(hostname); err == nil {
		addresses = append(addresses, parsed.Unmap())
	} else {
		canonicalName, err := resolver.LookupCNAME(ctx, hostname)
		if err != nil {
			return frostRetainedGroupResolvedEndpoint{},
				fmt.Errorf("cannot resolve retained-group endpoint CNAME: [%w]", err)
		}
		canonicalDNSName = strings.TrimSuffix(
			strings.ToLower(canonicalName),
			".",
		)
		if !validFrostRetainedGroupEndpointHostname(canonicalDNSName) {
			return frostRetainedGroupResolvedEndpoint{},
				fmt.Errorf("retained-group endpoint CNAME is not canonical")
		}
		resolved, err := resolver.LookupNetIP(ctx, "ip", hostname)
		if err != nil {
			return frostRetainedGroupResolvedEndpoint{},
				fmt.Errorf("cannot resolve retained-group endpoint addresses: [%w]", err)
		}
		for _, address := range resolved {
			if address.IsValid() && address.Zone() == "" {
				addresses = append(addresses, address.Unmap())
			}
		}
	}
	addresses = canonicalFrostRetainedGroupAddresses(addresses)
	if len(addresses) == 0 ||
		len(addresses) > frostRetainedGroupMaximumResolvedAddresses {
		return frostRetainedGroupResolvedEndpoint{},
			fmt.Errorf("retained-group endpoint address set is invalid")
	}
	addressSetHash := frostRetainedGroupResolvedAddressSetHash(addresses)
	return frostRetainedGroupResolvedEndpoint{
		endpoint:         endpoint,
		canonical:        endpoint.String(),
		canonicalDNSName: endpoint.Hostname(),
		resolvedDNSName:  canonicalDNSName,
		addresses:        addresses,
		addressSetHash:   addressSetHash,
	}, nil
}

func validateAndResolveFrostRetainedGroupSourceConfig(
	ctx context.Context,
	config FrostRetainedGroupHistorySourceConfig,
	primaryEthereumURL string,
) (*frostRetainedGroupValidatedSourceConfig, error) {
	if ctx == nil {
		return nil, fmt.Errorf("retained-group source validation context is nil")
	}
	requestTimeout := config.RequestTimeout
	if requestTimeout == 0 {
		requestTimeout = frostRetainedGroupDefaultTimeout
	}
	if requestTimeout < time.Second || requestTimeout > time.Minute {
		return nil, fmt.Errorf(
			"retained-group request timeout is outside supported bounds",
		)
	}
	exportURL, canonicalExport, err := validateFrostRetainedGroupTLSEndpoint(
		config.ExportURL,
	)
	if err != nil {
		return nil, fmt.Errorf("invalid retained-group export URL: [%w]", err)
	}
	verifierURL, canonicalVerifier, err := validateFrostRetainedGroupTLSEndpoint(
		config.EthereumURL,
	)
	if err != nil {
		return nil, fmt.Errorf("invalid retained-group Ethereum URL: [%w]", err)
	}
	primaryURL, err := validateFrostRetainedGroupPrimaryEndpoint(
		primaryEthereumURL,
	)
	if err != nil {
		return nil, fmt.Errorf("invalid primary Ethereum URL: [%w]", err)
	}
	resolver := config.Resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	resolveContext, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	exportEndpoint, err := resolveFrostRetainedGroupEndpoint(
		resolveContext,
		exportURL,
		resolver,
	)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve retained-group export URL: [%w]", err)
	}
	verifierEndpoint, err := resolveFrostRetainedGroupEndpoint(
		resolveContext,
		verifierURL,
		resolver,
	)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve retained-group Ethereum URL: [%w]", err)
	}
	primaryEndpoint, err := resolveFrostRetainedGroupEndpoint(
		resolveContext,
		primaryURL,
		resolver,
	)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve primary Ethereum URL: [%w]", err)
	}
	if frostRetainedGroupEndpointSetsOverlap(exportEndpoint, verifierEndpoint) {
		return nil, fmt.Errorf(
			"retained-group exporter and Ethereum verifier resolve to an alias or shared backend",
		)
	}
	if frostRetainedGroupEndpointSetsOverlap(exportEndpoint, primaryEndpoint) ||
		frostRetainedGroupEndpointSetsOverlap(verifierEndpoint, primaryEndpoint) {
		return nil, fmt.Errorf(
			"retained-group source is not independent of the primary endpoint",
		)
	}
	for name, value := range map[string]string{
		"source trust domain":   config.TrustDomainID,
		"export trust domain":   config.ExportTrustDomainID,
		"verifier trust domain": config.EthereumTrustDomainID,
	} {
		if !validFrostRetainedGroupIdentityLabel(value) {
			return nil, fmt.Errorf("retained-group %s is invalid", name)
		}
	}
	if config.TrustDomainID == config.ExportTrustDomainID ||
		config.TrustDomainID == config.EthereumTrustDomainID ||
		config.ExportTrustDomainID == config.EthereumTrustDomainID {
		return nil, fmt.Errorf("retained-group trust-domain identities are aliased")
	}
	if err := validateFrostRetainedGroupServiceIdentity(
		config.ExportServiceIdentity,
	); err != nil {
		return nil, fmt.Errorf("invalid retained-group export service identity: [%w]", err)
	}
	if err := validateFrostRetainedGroupServiceIdentity(
		config.EthereumServiceIdentity,
	); err != nil {
		return nil, fmt.Errorf("invalid retained-group verifier service identity: [%w]", err)
	}
	if config.ExportServiceIdentity == config.EthereumServiceIdentity {
		return nil, fmt.Errorf("retained-group service identities are aliased")
	}
	parseRequired := func(name string, value string) ([32]byte, error) {
		parsed, err := parseFrostActivationHex32(strings.TrimSpace(value))
		if err != nil || parsed == [32]byte{} {
			return [32]byte{}, fmt.Errorf("retained-group %s is invalid", name)
		}
		return parsed, nil
	}
	exportBackend, err := parseRequired(
		"export backend service fingerprint",
		config.ExportBackendServiceFingerprint,
	)
	if err != nil {
		return nil, err
	}
	verifierBackend, err := parseRequired(
		"verifier backend service fingerprint",
		config.EthereumBackendServiceFingerprint,
	)
	if err != nil {
		return nil, err
	}
	exportOperator, err := parseRequired(
		"export operator fingerprint",
		config.ExportOperatorFingerprint,
	)
	if err != nil {
		return nil, err
	}
	verifierOperator, err := parseRequired(
		"verifier operator fingerprint",
		config.EthereumOperatorFingerprint,
	)
	if err != nil {
		return nil, err
	}
	exportHistorySigner, err := parseRequired(
		"export history signer key hash",
		config.TrustedSignerKeyHash,
	)
	if err != nil {
		return nil, err
	}
	exportAttestation, err := parseRequired(
		"export attestation key hash",
		config.ExportAttestationKeyHash,
	)
	if err != nil {
		return nil, err
	}
	verifierAttestation, err := parseRequired(
		"verifier attestation key hash",
		config.EthereumAttestationKeyHash,
	)
	if err != nil {
		return nil, err
	}
	exportLeaf, err := parseRequired(
		"export TLS leaf SPKI hash",
		config.ExportTLSLeafSPKIHash,
	)
	if err != nil {
		return nil, err
	}
	verifierLeaf, err := parseRequired(
		"verifier TLS leaf SPKI hash",
		config.EthereumTLSLeafSPKIHash,
	)
	if err != nil {
		return nil, err
	}
	allRoleHashes := map[[32]byte]string{}
	for name, value := range map[string][32]byte{
		"export TLS leaf":       exportLeaf,
		"export backend":        exportBackend,
		"export operator":       exportOperator,
		"export attestation":    exportAttestation,
		"export history signer": exportHistorySigner,
		"verifier TLS leaf":     verifierLeaf,
		"verifier backend":      verifierBackend,
		"verifier operator":     verifierOperator,
		"verifier attestation":  verifierAttestation,
	} {
		if previous, exists := allRoleHashes[value]; exists {
			return nil, fmt.Errorf(
				"retained-group %s identity aliases %s",
				name,
				previous,
			)
		}
		allRoleHashes[value] = name
	}
	protocolID := frostRetainedGroupTLSExporterProtocolID()
	exportIdentity := FrostRetainedGroupEndpointIdentity{
		Schema:                    frostRetainedGroupEndpointIdentitySchema,
		Role:                      "retained-history-export",
		TrustDomainID:             config.ExportTrustDomainID,
		CanonicalEndpoint:         canonicalExport,
		CanonicalDNSName:          exportEndpoint.canonicalDNSName,
		ResolvedDNSName:           exportEndpoint.resolvedDNSName,
		ResolvedAddressSetHash:    exportEndpoint.addressSetHash,
		TLSLeafSPKIHash:           exportLeaf,
		ServiceIdentity:           config.ExportServiceIdentity,
		BackendServiceFingerprint: exportBackend,
		OperatorFingerprint:       exportOperator,
		AttestationKeyHash:        exportAttestation,
		TLSExporterProtocolID:     protocolID,
	}
	exportIdentity.EndpointFingerprint =
		computeFrostRetainedGroupEndpointFingerprint(exportIdentity)
	verifierIdentity := FrostRetainedGroupEndpointIdentity{
		Schema:                    frostRetainedGroupEndpointIdentitySchema,
		Role:                      "retained-history-verifier",
		TrustDomainID:             config.EthereumTrustDomainID,
		CanonicalEndpoint:         canonicalVerifier,
		CanonicalDNSName:          verifierEndpoint.canonicalDNSName,
		ResolvedDNSName:           verifierEndpoint.resolvedDNSName,
		ResolvedAddressSetHash:    verifierEndpoint.addressSetHash,
		TLSLeafSPKIHash:           verifierLeaf,
		ServiceIdentity:           config.EthereumServiceIdentity,
		BackendServiceFingerprint: verifierBackend,
		OperatorFingerprint:       verifierOperator,
		AttestationKeyHash:        verifierAttestation,
		TLSExporterProtocolID:     protocolID,
	}
	verifierIdentity.EndpointFingerprint =
		computeFrostRetainedGroupEndpointFingerprint(verifierIdentity)
	identity := FrostRetainedGroupHistoryIdentity{
		Schema:               frostRetainedGroupSourceIdentitySchema,
		TrustDomainID:        config.TrustDomainID,
		OperatorFingerprint:  exportOperator,
		HistorySignerKeyHash: exportHistorySigner,
		Export:               exportIdentity,
		Verifier:             verifierIdentity,
	}
	identity.EndpointFingerprint =
		computeFrostRetainedGroupSourceEndpointFingerprint(identity)
	if err := validateFrostRetainedGroupHistoryIdentity(identity); err != nil {
		return nil, err
	}
	var roots *x509.CertPool
	if config.TLSRootCAs != nil {
		roots = config.TLSRootCAs.Clone()
	}
	return &frostRetainedGroupValidatedSourceConfig{
		exportEndpoint:   exportEndpoint,
		verifierEndpoint: verifierEndpoint,
		primaryEndpoint:  primaryEndpoint,
		identity:         identity,
		requestTimeout:   requestTimeout,
		rootCAs:          roots,
	}, nil
}

func validateFrostRetainedGroupPrimaryEndpoint(raw string) (*url.URL, error) {
	if raw == "" || raw != strings.TrimSpace(raw) {
		return nil, fmt.Errorf("URL is empty or has surrounding whitespace")
	}
	endpoint, err := url.Parse(raw)
	if err != nil || endpoint.Host == "" || endpoint.User != nil ||
		endpoint.Fragment != "" || endpoint.Opaque != "" {
		return nil, fmt.Errorf("URL is not an absolute credential-free endpoint")
	}
	switch endpoint.Scheme {
	case "https", "wss":
		if endpoint.Port() == "" {
			endpoint.Host = net.JoinHostPort(endpoint.Hostname(), "443")
		}
	case "http", "ws":
		if endpoint.Port() == "" {
			endpoint.Host = net.JoinHostPort(endpoint.Hostname(), "80")
		}
	default:
		return nil, fmt.Errorf("URL scheme is unsupported")
	}
	hostname := endpoint.Hostname()
	if hostname == "" || hostname != strings.ToLower(hostname) ||
		strings.HasSuffix(hostname, ".") ||
		!validFrostRetainedGroupEndpointHostname(hostname) {
		return nil, fmt.Errorf("URL hostname is not canonical")
	}
	port := endpoint.Port()
	parsedPort, err := strconv.ParseUint(port, 10, 16)
	if err != nil || parsedPort == 0 {
		return nil, fmt.Errorf("URL port is invalid")
	}
	endpoint.Host = net.JoinHostPort(hostname, strconv.FormatUint(parsedPort, 10))
	if endpoint.Path == "" {
		endpoint.Path = "/"
	}
	return endpoint, nil
}

func (monitor *frostRetainedGroupIndependenceMonitor) verify(
	ctx context.Context,
) error {
	if monitor == nil || ctx == nil ||
		monitor.exportEndpoint.endpoint == nil ||
		monitor.verifierEndpoint.endpoint == nil ||
		monitor.primaryEndpoint.endpoint == nil ||
		monitor.timeout < time.Second || monitor.timeout > time.Minute {
		return fmt.Errorf("retained-group endpoint independence monitor is incomplete")
	}
	resolver := monitor.resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	resolveContext, cancel := context.WithTimeout(ctx, monitor.timeout)
	defer cancel()
	currentPrimary, err := resolveFrostRetainedGroupEndpoint(
		resolveContext,
		monitor.primaryEndpoint.endpoint,
		resolver,
	)
	if err != nil {
		return fmt.Errorf(
			"cannot re-resolve primary Ethereum endpoint: [%w]",
			err,
		)
	}
	if frostRetainedGroupEndpointSetsOverlap(
		currentPrimary,
		monitor.exportEndpoint,
	) || frostRetainedGroupEndpointSetsOverlap(
		currentPrimary,
		monitor.verifierEndpoint,
	) {
		return fmt.Errorf(
			"primary Ethereum endpoint now aliases a retained-group endpoint",
		)
	}
	return nil
}

func canonicalFrostRetainedGroupAddresses(
	addresses []netip.Addr,
) []netip.Addr {
	unique := make(map[netip.Addr]bool)
	for _, address := range addresses {
		if address.IsValid() && address.Zone() == "" {
			unique[address.Unmap()] = true
		}
	}
	result := make([]netip.Addr, 0, len(unique))
	for address := range unique {
		result = append(result, address)
	}
	sort.Slice(result, func(left int, right int) bool {
		return result[left].Compare(result[right]) < 0
	})
	return result
}

func frostRetainedGroupResolvedAddressSetHash(
	addresses []netip.Addr,
) [32]byte {
	transcript := frostRetainedGroupIdentityTranscript(
		frostRetainedGroupResolvedAddressSetDomain,
	)
	canonical := canonicalFrostRetainedGroupAddresses(addresses)
	transcript.uint64("addressCount", uint64(len(canonical)))
	for index, address := range canonical {
		transcript.text(
			fmt.Sprintf("address[%d]", index),
			address.String(),
		)
	}
	return transcript.sum()
}

func frostRetainedGroupEndpointSetsOverlap(
	left frostRetainedGroupResolvedEndpoint,
	right frostRetainedGroupResolvedEndpoint,
) bool {
	if left.canonicalDNSName == right.canonicalDNSName ||
		left.resolvedDNSName == right.resolvedDNSName ||
		left.canonicalDNSName == right.resolvedDNSName ||
		left.resolvedDNSName == right.canonicalDNSName {
		return true
	}
	addresses := make(map[netip.Addr]bool, len(left.addresses))
	for _, address := range left.addresses {
		addresses[address] = true
	}
	for _, address := range right.addresses {
		if addresses[address] {
			return true
		}
	}
	return false
}

func frostRetainedGroupPinnedDialContext(
	endpoint frostRetainedGroupResolvedEndpoint,
	timeout time.Duration,
) func(context.Context, string, string) (net.Conn, error) {
	dialer := &net.Dialer{
		Timeout:   timeout,
		KeepAlive: -1,
	}
	return func(
		ctx context.Context,
		network string,
		address string,
	) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil ||
			host != endpoint.endpoint.Hostname() ||
			port != endpoint.endpoint.Port() {
			return nil, fmt.Errorf(
				"retained-group transport attempted an unpinned endpoint",
			)
		}
		var lastErr error
		for _, pinned := range endpoint.addresses {
			connection, err := dialer.DialContext(
				ctx,
				network,
				net.JoinHostPort(pinned.String(), port),
			)
			if err == nil {
				return connection, nil
			}
			lastErr = err
		}
		return nil, fmt.Errorf(
			"cannot connect any pinned retained-group endpoint address: [%w]",
			lastErr,
		)
	}
}

func newFrostRetainedGroupPinnedTLSConfig(
	identity FrostRetainedGroupEndpointIdentity,
	rootCAs *x509.CertPool,
) (*tls.Config, error) {
	if err := validateFrostRetainedGroupEndpointIdentity(identity); err != nil {
		return nil, err
	}
	var roots *x509.CertPool
	if rootCAs != nil {
		roots = rootCAs.Clone()
	}
	return &tls.Config{
		MinVersion: tls.VersionTLS13,
		MaxVersion: tls.VersionTLS13,
		ServerName: identity.CanonicalDNSName,
		RootCAs:    roots,
		NextProtos: []string{"http/1.1"},
		VerifyConnection: func(state tls.ConnectionState) error {
			return verifyFrostRetainedGroupTLSConnection(state, identity)
		},
	}, nil
}

func newFrostRetainedGroupAttestedHTTPClient(
	endpoint frostRetainedGroupResolvedEndpoint,
	identity FrostRetainedGroupEndpointIdentity,
	rootCAs *x509.CertPool,
	timeout time.Duration,
) (*http.Client, *http.Transport, error) {
	if endpoint.endpoint == nil || endpoint.canonical != identity.CanonicalEndpoint ||
		endpoint.canonicalDNSName != identity.CanonicalDNSName ||
		endpoint.resolvedDNSName != identity.ResolvedDNSName ||
		endpoint.addressSetHash != identity.ResolvedAddressSetHash {
		return nil, nil, fmt.Errorf(
			"retained-group transport endpoint differs from its identity",
		)
	}
	tlsConfig, err := newFrostRetainedGroupPinnedTLSConfig(identity, rootCAs)
	if err != nil {
		return nil, nil, err
	}
	transport := &http.Transport{
		Proxy:                  nil,
		DialContext:            frostRetainedGroupPinnedDialContext(endpoint, timeout),
		DisableKeepAlives:      true,
		DisableCompression:     true,
		ForceAttemptHTTP2:      false,
		MaxConnsPerHost:        1,
		ResponseHeaderTimeout:  timeout,
		TLSHandshakeTimeout:    timeout,
		ExpectContinueTimeout:  time.Second,
		MaxResponseHeaderBytes: 32 * 1024,
		TLSClientConfig:        tlsConfig,
	}
	attested := &frostRetainedGroupAttestedRoundTripper{
		base:             transport,
		endpoint:         endpoint,
		identity:         identity,
		maximumBodyBytes: frostRetainedGroupMaximumTransportBodyBytes,
		maximumClockSkew: frostRetainedGroupTransportClockSkew,
		maximumLifetime:  frostRetainedGroupTransportAttestationLifetime,
	}
	client := &http.Client{
		Transport: attested,
		Timeout:   timeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return fmt.Errorf("retained-group redirects are forbidden")
		},
	}
	return client, transport, nil
}

func verifyFrostRetainedGroupTLSConnection(
	state tls.ConnectionState,
	identity FrostRetainedGroupEndpointIdentity,
) error {
	if len(state.VerifiedChains) == 0 || len(state.PeerCertificates) == 0 {
		return fmt.Errorf("retained-group TLS peer is not PKIX-verified")
	}
	if state.Version != tls.VersionTLS13 ||
		state.NegotiatedProtocol != "http/1.1" {
		return fmt.Errorf("retained-group TLS protocol profile mismatch")
	}
	leaf := state.PeerCertificates[0]
	if !leaf.BasicConstraintsValid || leaf.IsCA ||
		leaf.KeyUsage&x509.KeyUsageDigitalSignature == 0 ||
		leaf.KeyUsage&(x509.KeyUsageCertSign|x509.KeyUsageCRLSign) != 0 {
		return fmt.Errorf("retained-group TLS leaf is not an X.509-SVID leaf")
	}
	if len(leaf.ExtKeyUsage) > 0 {
		hasServerAuth := false
		hasClientAuth := false
		for _, usage := range leaf.ExtKeyUsage {
			hasServerAuth = hasServerAuth || usage == x509.ExtKeyUsageServerAuth
			hasClientAuth = hasClientAuth || usage == x509.ExtKeyUsageClientAuth
		}
		if !hasServerAuth || !hasClientAuth {
			return fmt.Errorf("retained-group TLS leaf has incomplete X.509-SVID EKU")
		}
	}
	if sha256.Sum256(leaf.RawSubjectPublicKeyInfo) !=
		identity.TLSLeafSPKIHash {
		return fmt.Errorf("retained-group TLS leaf SPKI mismatch")
	}
	if len(leaf.URIs) != 1 || leaf.URIs[0].String() != identity.ServiceIdentity ||
		validateFrostRetainedGroupServiceIdentity(
			leaf.URIs[0].String(),
		) != nil {
		return fmt.Errorf("retained-group TLS service identity mismatch")
	}
	return nil
}

func frostRetainedGroupAttestationTranscript(
	attestation frostRetainedGroupTransportAttestation,
) ([32]byte, error) {
	parse := func(name string, value string) ([32]byte, error) {
		parsed, err := parseFrostActivationHex32(value)
		if err != nil {
			return [32]byte{}, fmt.Errorf("invalid %s: [%w]", name, err)
		}
		return parsed, nil
	}
	endpoint, err := parse("endpoint fingerprint", attestation.EndpointFingerprint)
	if err != nil {
		return [32]byte{}, err
	}
	addressSet := map[string]string{
		"tlsLeafSpkiHash":           attestation.TLSLeafSPKIHash,
		"backendServiceFingerprint": attestation.BackendServiceFingerprint,
		"operatorFingerprint":       attestation.OperatorFingerprint,
		"attestationKeyHash":        attestation.AttestationKeyHash,
		"tlsExporterProtocolID":     attestation.TLSExporterProtocolID,
		"challenge":                 attestation.Challenge,
		"requestBodySha256":         attestation.RequestBodySHA256,
		"responseBodySha256":        attestation.ResponseBodySHA256,
		"tlsExporterContextSha256":  attestation.TLSExporterContextSHA256,
		"tlsExporterValueSha256":    attestation.TLSExporterValueSHA256,
	}
	parsed := make(map[string][32]byte, len(addressSet))
	for name, value := range addressSet {
		parsed[name], err = parse(name, value)
		if err != nil {
			return [32]byte{}, err
		}
	}
	issued, err := parseFrostRetainedGroupUint64(attestation.IssuedAtUnixMs)
	if err != nil {
		return [32]byte{}, err
	}
	expires, err := parseFrostRetainedGroupUint64(attestation.ExpiresAtUnixMs)
	if err != nil {
		return [32]byte{}, err
	}
	transcript := frostRetainedGroupIdentityTranscript(
		frostRetainedGroupTransportAttestationDomain,
	)
	transcript.text("schema", attestation.Schema)
	transcript.text("role", attestation.Role)
	transcript.bytes32("endpointFingerprint", endpoint)
	transcript.text("canonicalEndpoint", attestation.CanonicalEndpoint)
	transcript.text("canonicalDNSName", attestation.CanonicalDNSName)
	transcript.text("resolvedDNSName", attestation.ResolvedDNSName)
	transcript.text("resolvedPeerIP", attestation.ResolvedPeerIP)
	transcript.bytes32("tlsLeafSpkiHash", parsed["tlsLeafSpkiHash"])
	transcript.text("serviceIdentity", attestation.ServiceIdentity)
	transcript.bytes32(
		"backendServiceFingerprint",
		parsed["backendServiceFingerprint"],
	)
	transcript.bytes32(
		"operatorFingerprint",
		parsed["operatorFingerprint"],
	)
	transcript.bytes32("attestationKeyHash", parsed["attestationKeyHash"])
	transcript.bytes32(
		"tlsExporterProtocolID",
		parsed["tlsExporterProtocolID"],
	)
	transcript.bytes32("challenge", parsed["challenge"])
	transcript.text("requestMethod", attestation.RequestMethod)
	transcript.text("requestTarget", attestation.RequestTarget)
	transcript.bytes32("requestBodySha256", parsed["requestBodySha256"])
	transcript.uint64("responseStatus", attestation.ResponseStatus)
	transcript.bytes32("responseBodySha256", parsed["responseBodySha256"])
	transcript.uint64("issuedAtUnixMs", issued)
	transcript.uint64("expiresAtUnixMs", expires)
	transcript.bytes32(
		"tlsExporterContextSha256",
		parsed["tlsExporterContextSha256"],
	)
	transcript.bytes32(
		"tlsExporterValueSha256",
		parsed["tlsExporterValueSha256"],
	)
	return transcript.sum(), nil
}

func frostRetainedGroupTLSExporterContext(
	identity FrostRetainedGroupEndpointIdentity,
	challenge [32]byte,
	method string,
	target string,
	requestDigest [32]byte,
	responseStatus uint64,
	responseDigest [32]byte,
) [32]byte {
	transcript := frostRetainedGroupIdentityTranscript(
		frostRetainedGroupTLSExporterContextDomain,
	)
	transcript.bytes32("endpointFingerprint", identity.EndpointFingerprint)
	transcript.bytes32("challenge", challenge)
	transcript.text("requestMethod", method)
	transcript.text("requestTarget", target)
	transcript.bytes32("requestBodySha256", requestDigest)
	transcript.uint64("responseStatus", responseStatus)
	transcript.bytes32("responseBodySha256", responseDigest)
	return transcript.sum()
}

func frostRetainedGroupTLSExporterValueHash(material []byte) [32]byte {
	transcript := frostRetainedGroupIdentityTranscript(
		frostRetainedGroupTLSExporterValueDomain,
	)
	transcript.field("exporterValue", material)
	return transcript.sum()
}

func frostRetainedGroupBackendAttestationDigest(
	transportAttestationDigest [32]byte,
) [32]byte {
	transcript := frostRetainedGroupIdentityTranscript(
		frostRetainedGroupBackendAttestationDomain,
	)
	transcript.bytes32(
		"transportAttestationDigest",
		transportAttestationDigest,
	)
	return transcript.sum()
}

func frostRetainedGroupOperatorAttestationDigest(
	transportAttestationDigest [32]byte,
) [32]byte {
	transcript := frostRetainedGroupIdentityTranscript(
		frostRetainedGroupOperatorAttestationDomain,
	)
	transcript.bytes32(
		"transportAttestationDigest",
		transportAttestationDigest,
	)
	return transcript.sum()
}

func frostRetainedGroupExportKeyingMaterial(
	state *tls.ConnectionState,
	contextHash [32]byte,
) ([32]byte, error) {
	if state == nil || !state.HandshakeComplete {
		return [32]byte{}, fmt.Errorf("retained-group response has no completed TLS state")
	}
	material, err := state.ExportKeyingMaterial(
		frostRetainedGroupTLSExporterLabel,
		contextHash[:],
		32,
	)
	if err != nil {
		return [32]byte{}, fmt.Errorf(
			"cannot derive retained-group TLS exporter: [%w]",
			err,
		)
	}
	return frostRetainedGroupTLSExporterValueHash(material), nil
}

func parseFrostRetainedGroupUint64(value string) (uint64, error) {
	if value == "" || (len(value) > 1 && value[0] == '0') {
		return 0, fmt.Errorf("retained-group uint64 is not canonical")
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil || strconv.FormatUint(parsed, 10) != value {
		return 0, fmt.Errorf("retained-group uint64 is invalid")
	}
	return parsed, nil
}

func (roundTripper *frostRetainedGroupAttestedRoundTripper) RoundTrip(
	request *http.Request,
) (*http.Response, error) {
	if roundTripper == nil || roundTripper.base == nil || request == nil ||
		request.URL == nil || request.Method != http.MethodPost {
		return nil, fmt.Errorf("retained-group attested transport request is invalid")
	}
	base := roundTripper.endpoint.endpoint
	basePath := strings.TrimSuffix(base.Path, "/")
	if request.URL.Scheme != base.Scheme ||
		request.URL.Host != base.Host ||
		request.URL.User != nil ||
		request.URL.Fragment != "" ||
		request.URL.RawQuery != "" ||
		request.URL.Opaque != "" ||
		request.URL.RawPath != "" ||
		request.URL.Path == "" ||
		path.Clean(request.URL.Path) != request.URL.Path ||
		strings.Contains(request.URL.Path, "//") ||
		(request.URL.Path != base.Path &&
			!strings.HasPrefix(request.URL.Path, basePath+"/")) ||
		(request.Host != "" && request.Host != base.Host) {
		return nil, fmt.Errorf("retained-group transport target escaped its pinned endpoint")
	}
	target := request.URL.String()
	if request.Header.Get(frostRetainedGroupTransportChallengeHeader) != "" ||
		request.Header.Get("Accept-Encoding") != "" {
		return nil, fmt.Errorf("retained-group transport headers are ambiguous")
	}
	var requestBodyReader io.Reader = http.NoBody
	if request.Body != nil {
		requestBodyReader = request.Body
	}
	body, err := io.ReadAll(
		io.LimitReader(
			requestBodyReader,
			roundTripper.maximumBodyBytes+1,
		),
	)
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > roundTripper.maximumBodyBytes {
		return nil, fmt.Errorf("retained-group request body is too large")
	}
	if request.Body != nil {
		_ = request.Body.Close()
	}
	request.Body = io.NopCloser(bytes.NewReader(body))
	request.ContentLength = int64(len(body))
	requestDigest := sha256.Sum256(body)
	var challenge [32]byte
	randomSource := roundTripper.random
	if randomSource == nil {
		randomSource = rand.Reader
	}
	if _, err := io.ReadFull(randomSource, challenge[:]); err != nil {
		return nil, fmt.Errorf("cannot create retained-group transport challenge: [%w]", err)
	}
	request.Header.Set(
		frostRetainedGroupTransportChallengeHeader,
		hex.EncodeToString(challenge[:]),
	)
	request.Header.Set("Accept-Encoding", "identity")

	var remote net.Addr
	trace := &httptrace.ClientTrace{
		GotConn: func(info httptrace.GotConnInfo) {
			if info.Conn != nil {
				remote = info.Conn.RemoteAddr()
			}
		},
	}
	request = request.WithContext(
		httptrace.WithClientTrace(request.Context(), trace),
	)
	response, err := roundTripper.base.RoundTrip(request)
	if err != nil {
		return nil, err
	}
	bodyLimit := roundTripper.maximumBodyBytes
	responseBody, readErr := io.ReadAll(
		io.LimitReader(response.Body, bodyLimit+1),
	)
	_ = response.Body.Close()
	if readErr != nil {
		return nil, readErr
	}
	if int64(len(responseBody)) > bodyLimit {
		return nil, fmt.Errorf("retained-group response body is too large")
	}
	response.Body = io.NopCloser(bytes.NewReader(responseBody))
	response.ContentLength = int64(len(responseBody))
	if response.Uncompressed ||
		(response.Header.Get("Content-Encoding") != "" &&
			response.Header.Get("Content-Encoding") != "identity") {
		return nil, fmt.Errorf("retained-group response transformation is forbidden")
	}
	responseDigest := sha256.Sum256(responseBody)
	if err := verifyFrostRetainedGroupTransportAttestation(
		response,
		remote,
		roundTripper.endpoint,
		roundTripper.identity,
		challenge,
		request.Method,
		target,
		requestDigest,
		responseDigest,
		roundTripper.now,
		roundTripper.maximumClockSkew,
		roundTripper.maximumLifetime,
	); err != nil {
		return nil, err
	}
	proof := frostRetainedGroupTransportProof{
		role:           roundTripper.identity.Role,
		requestDigest:  requestDigest,
		responseDigest: responseDigest,
		challenge:      challenge,
	}
	if response.Request == nil {
		response.Request = request
	}
	response.Request = response.Request.Clone(
		context.WithValue(
			response.Request.Context(),
			frostRetainedGroupTransportProofKey{},
			proof,
		),
	)
	return response, nil
}

func verifyFrostRetainedGroupTransportAttestation(
	response *http.Response,
	remote net.Addr,
	endpoint frostRetainedGroupResolvedEndpoint,
	identity FrostRetainedGroupEndpointIdentity,
	challenge [32]byte,
	requestMethod string,
	requestTarget string,
	requestDigest [32]byte,
	responseDigest [32]byte,
	now func() time.Time,
	maximumClockSkew time.Duration,
	maximumLifetime time.Duration,
) error {
	if response == nil || response.TLS == nil || !response.TLS.HandshakeComplete ||
		remote == nil || response.Request == nil {
		return fmt.Errorf("retained-group response transport is unauthenticated")
	}
	if err := verifyFrostRetainedGroupTLSConnection(*response.TLS, identity); err != nil {
		return err
	}
	remoteIP, err := frostRetainedGroupRemoteIP(remote)
	if err != nil || !frostRetainedGroupAddressPinned(endpoint.addresses, remoteIP) {
		return fmt.Errorf("retained-group response peer is outside the pinned address set")
	}
	values := response.Header.Values(
		frostRetainedGroupTransportAttestationHeader,
	)
	if len(values) != 1 || len(values[0]) == 0 ||
		len(values[0]) > frostRetainedGroupMaximumTransportAttestationBytes {
		return fmt.Errorf("retained-group transport attestation header is missing or ambiguous")
	}
	raw, err := decodeCanonicalFrostRetainedGroupBase64(values[0])
	if err != nil || len(raw) == 0 ||
		len(raw) > frostRetainedGroupMaximumTransportAttestationBytes {
		return fmt.Errorf("retained-group transport attestation encoding is invalid")
	}
	attestation := frostRetainedGroupTransportAttestation{}
	if err := decodeStrictFrostActivationJSON(raw, &attestation); err != nil {
		return fmt.Errorf("cannot decode retained-group transport attestation: [%w]", err)
	}
	issued, issuedErr := parseFrostRetainedGroupUint64(
		attestation.IssuedAtUnixMs,
	)
	expires, expiresErr := parseFrostRetainedGroupUint64(
		attestation.ExpiresAtUnixMs,
	)
	if now == nil {
		now = time.Now
	}
	if maximumClockSkew <= 0 {
		maximumClockSkew = frostRetainedGroupTransportClockSkew
	}
	if maximumLifetime <= 0 {
		maximumLifetime = frostRetainedGroupTransportAttestationLifetime
	}
	nowMilliseconds := now().UnixMilli()
	maximumInt64 := uint64(^uint64(0) >> 1)
	maximumLifetimeMilliseconds := uint64(maximumLifetime / time.Millisecond)
	maximumClockSkewMilliseconds := maximumClockSkew.Milliseconds()
	if issuedErr != nil || expiresErr != nil || nowMilliseconds < 0 ||
		issued > maximumInt64 || expires > maximumInt64 ||
		expires <= issued ||
		expires-issued > maximumLifetimeMilliseconds {
		return fmt.Errorf("retained-group transport attestation is stale")
	}
	issuedMilliseconds := int64(issued)
	expiresMilliseconds := int64(expires)
	if (issuedMilliseconds > nowMilliseconds &&
		issuedMilliseconds-nowMilliseconds > maximumClockSkewMilliseconds) ||
		(expiresMilliseconds <= nowMilliseconds &&
			nowMilliseconds-expiresMilliseconds >= maximumClockSkewMilliseconds) {
		return fmt.Errorf("retained-group transport attestation is stale")
	}
	contextHash := frostRetainedGroupTLSExporterContext(
		identity,
		challenge,
		requestMethod,
		requestTarget,
		requestDigest,
		uint64(response.StatusCode),
		responseDigest,
	)
	exporterValueHash, err := frostRetainedGroupExportKeyingMaterial(
		response.TLS,
		contextHash,
	)
	if err != nil {
		return err
	}
	if attestation.Schema != frostRetainedGroupTransportAttestationSchema ||
		attestation.Role != identity.Role ||
		attestation.EndpointFingerprint !=
			frostActivationHex32(identity.EndpointFingerprint) ||
		attestation.CanonicalEndpoint != identity.CanonicalEndpoint ||
		attestation.CanonicalDNSName != identity.CanonicalDNSName ||
		attestation.ResolvedDNSName != identity.ResolvedDNSName ||
		attestation.ResolvedPeerIP != remoteIP.String() ||
		attestation.TLSLeafSPKIHash !=
			frostActivationHex32(identity.TLSLeafSPKIHash) ||
		attestation.ServiceIdentity != identity.ServiceIdentity ||
		attestation.BackendServiceFingerprint !=
			frostActivationHex32(identity.BackendServiceFingerprint) ||
		attestation.OperatorFingerprint !=
			frostActivationHex32(identity.OperatorFingerprint) ||
		attestation.AttestationKeyHash !=
			frostActivationHex32(identity.AttestationKeyHash) ||
		attestation.TLSExporterProtocolID !=
			frostActivationHex32(identity.TLSExporterProtocolID) ||
		attestation.Challenge != frostActivationHex32(challenge) ||
		attestation.RequestMethod != requestMethod ||
		attestation.RequestTarget != requestTarget ||
		attestation.RequestBodySHA256 != frostActivationHex32(requestDigest) ||
		attestation.ResponseStatus != uint64(response.StatusCode) ||
		attestation.ResponseBodySHA256 != frostActivationHex32(responseDigest) ||
		attestation.TLSExporterContextSHA256 !=
			frostActivationHex32(contextHash) ||
		attestation.TLSExporterValueSHA256 !=
			frostActivationHex32(exporterValueHash) {
		return fmt.Errorf("retained-group transport attestation is differently bound")
	}
	digest, err := frostRetainedGroupAttestationTranscript(attestation)
	if err != nil {
		return fmt.Errorf("retained-group transport attestation transcript is invalid: [%w]", err)
	}
	backendDigest := frostRetainedGroupBackendAttestationDigest(digest)
	if err := verifyFrostRetainedGroupEd25519RoleSignature(
		"backend",
		identity.BackendServiceFingerprint,
		attestation.BackendSignerPublicKeySPKI,
		attestation.BackendSignatureAlgorithm,
		attestation.BackendSignature,
		backendDigest,
	); err != nil {
		return err
	}
	operatorDigest := frostRetainedGroupOperatorAttestationDigest(digest)
	if err := verifyFrostRetainedGroupEd25519RoleSignature(
		"operator",
		identity.OperatorFingerprint,
		attestation.OperatorSignerPublicKeySPKI,
		attestation.OperatorSignatureAlgorithm,
		attestation.OperatorSignature,
		operatorDigest,
	); err != nil {
		return err
	}
	if err := verifyFrostRetainedGroupEd25519RoleSignature(
		"transport attestation",
		identity.AttestationKeyHash,
		attestation.SignerPublicKeySPKI,
		attestation.SignatureAlgorithm,
		attestation.Signature,
		digest,
	); err != nil {
		return err
	}
	return nil
}

func verifyFrostRetainedGroupEd25519RoleSignature(
	role string,
	expectedKeyHash [32]byte,
	publicKeySPKI string,
	algorithm string,
	signatureValue string,
	digest [32]byte,
) error {
	publicKeyDER, err := decodeCanonicalFrostRetainedGroupBase64(
		publicKeySPKI,
	)
	if err != nil || len(publicKeyDER) == 0 || len(publicKeyDER) > 1024 ||
		sha256.Sum256(publicKeyDER) != expectedKeyHash {
		return fmt.Errorf("retained-group %s signer is not trusted", role)
	}
	parsedPublicKey, err := x509.ParsePKIXPublicKey(publicKeyDER)
	if err != nil {
		return fmt.Errorf(
			"cannot parse retained-group %s signer: [%w]",
			role,
			err,
		)
	}
	publicKey, ok := parsedPublicKey.(ed25519.PublicKey)
	if !ok || algorithm != "ed25519" {
		return fmt.Errorf("retained-group %s signer is not Ed25519", role)
	}
	signature, err := decodeCanonicalFrostRetainedGroupBase64(
		signatureValue,
	)
	if err != nil || len(signature) != ed25519.SignatureSize ||
		!ed25519.Verify(publicKey, digest[:], signature) {
		return fmt.Errorf("retained-group %s signature is invalid", role)
	}
	return nil
}

func decodeCanonicalFrostRetainedGroupBase64(value string) ([]byte, error) {
	decoded, err := base64.StdEncoding.Strict().DecodeString(value)
	if err != nil || base64.StdEncoding.EncodeToString(decoded) != value {
		return nil, fmt.Errorf("retained-group base64 is not canonical")
	}
	return decoded, nil
}

func frostRetainedGroupRemoteIP(address net.Addr) (netip.Addr, error) {
	host, _, err := net.SplitHostPort(address.String())
	if err != nil {
		return netip.Addr{}, err
	}
	parsed, err := netip.ParseAddr(host)
	if err != nil || parsed.Zone() != "" {
		return netip.Addr{}, fmt.Errorf("retained-group peer address is invalid")
	}
	return parsed.Unmap(), nil
}

func frostRetainedGroupAddressPinned(
	addresses []netip.Addr,
	candidate netip.Addr,
) bool {
	for _, address := range addresses {
		if address == candidate {
			return true
		}
	}
	return false
}

func requireFrostRetainedGroupTransportProof(
	response *http.Response,
	role string,
) error {
	if response == nil || response.Request == nil {
		return fmt.Errorf("retained-group response has no transport proof")
	}
	proof, ok := response.Request.Context().Value(
		frostRetainedGroupTransportProofKey{},
	).(frostRetainedGroupTransportProof)
	if !ok || proof.role != role ||
		proof.requestDigest == [32]byte{} ||
		proof.responseDigest == [32]byte{} ||
		proof.challenge == [32]byte{} {
		return fmt.Errorf("retained-group response has no valid transport proof")
	}
	return nil
}

// marshalFrostRetainedGroupTransportAttestation constructs the exact header
// an independently implemented service must emit. It is intentionally kept
// package-private; production servers should implement the frozen transcript,
// not import signer-client internals.
func marshalFrostRetainedGroupTransportAttestation(
	request *http.Request,
	responseStatus int,
	responseBody []byte,
	identity FrostRetainedGroupEndpointIdentity,
	attestationPrivateKey ed25519.PrivateKey,
	attestationPublicKeyDER []byte,
	backendPrivateKey ed25519.PrivateKey,
	backendPublicKeyDER []byte,
	operatorPrivateKey ed25519.PrivateKey,
	operatorPublicKeyDER []byte,
	now time.Time,
	localIP netip.Addr,
) (string, error) {
	if request == nil || request.TLS == nil ||
		len(attestationPrivateKey) != ed25519.PrivateKeySize ||
		sha256.Sum256(attestationPublicKeyDER) != identity.AttestationKeyHash ||
		len(backendPrivateKey) != ed25519.PrivateKeySize ||
		sha256.Sum256(backendPublicKeyDER) !=
			identity.BackendServiceFingerprint ||
		len(operatorPrivateKey) != ed25519.PrivateKeySize ||
		sha256.Sum256(operatorPublicKeyDER) != identity.OperatorFingerprint ||
		now.UnixMilli() < 0 || !localIP.IsValid() {
		return "", fmt.Errorf("retained-group transport attestation inputs are invalid")
	}
	challengeBytes, err := hex.DecodeString(
		request.Header.Get(frostRetainedGroupTransportChallengeHeader),
	)
	if err != nil || len(challengeBytes) != 32 {
		return "", fmt.Errorf("retained-group transport challenge is invalid")
	}
	var challenge [32]byte
	copy(challenge[:], challengeBytes)
	var requestBodyReader io.Reader = http.NoBody
	if request.Body != nil {
		requestBodyReader = request.Body
	}
	requestBody, err := io.ReadAll(
		io.LimitReader(
			requestBodyReader,
			frostRetainedGroupMaximumTransportBodyBytes+1,
		),
	)
	if err != nil || len(requestBody) > frostRetainedGroupMaximumTransportBodyBytes {
		return "", fmt.Errorf("retained-group transport request body is invalid")
	}
	request.Body = io.NopCloser(bytes.NewReader(requestBody))
	requestDigest := sha256.Sum256(requestBody)
	responseDigest := sha256.Sum256(responseBody)
	requestTarget, err := frostRetainedGroupServerRequestTarget(identity, request)
	if err != nil {
		return "", err
	}
	contextHash := frostRetainedGroupTLSExporterContext(
		identity,
		challenge,
		request.Method,
		requestTarget,
		requestDigest,
		uint64(responseStatus),
		responseDigest,
	)
	exporterValueHash, err := frostRetainedGroupExportKeyingMaterial(
		request.TLS,
		contextHash,
	)
	if err != nil {
		return "", err
	}
	issued := uint64(now.UnixMilli())
	expires := issued + uint64(
		frostRetainedGroupTransportAttestationLifetime.Milliseconds(),
	)
	attestation := frostRetainedGroupTransportAttestation{
		Schema:                    frostRetainedGroupTransportAttestationSchema,
		Role:                      identity.Role,
		EndpointFingerprint:       frostActivationHex32(identity.EndpointFingerprint),
		CanonicalEndpoint:         identity.CanonicalEndpoint,
		CanonicalDNSName:          identity.CanonicalDNSName,
		ResolvedDNSName:           identity.ResolvedDNSName,
		ResolvedPeerIP:            localIP.Unmap().String(),
		TLSLeafSPKIHash:           frostActivationHex32(identity.TLSLeafSPKIHash),
		ServiceIdentity:           identity.ServiceIdentity,
		BackendServiceFingerprint: frostActivationHex32(identity.BackendServiceFingerprint),
		OperatorFingerprint:       frostActivationHex32(identity.OperatorFingerprint),
		AttestationKeyHash:        frostActivationHex32(identity.AttestationKeyHash),
		TLSExporterProtocolID:     frostActivationHex32(identity.TLSExporterProtocolID),
		Challenge:                 frostActivationHex32(challenge),
		RequestMethod:             request.Method,
		RequestTarget:             requestTarget,
		RequestBodySHA256:         frostActivationHex32(requestDigest),
		ResponseStatus:            uint64(responseStatus),
		ResponseBodySHA256:        frostActivationHex32(responseDigest),
		IssuedAtUnixMs:            strconv.FormatUint(issued, 10),
		ExpiresAtUnixMs:           strconv.FormatUint(expires, 10),
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
	digest, err := frostRetainedGroupAttestationTranscript(attestation)
	if err != nil {
		return "", err
	}
	backendDigest := frostRetainedGroupBackendAttestationDigest(digest)
	attestation.BackendSignature = base64.StdEncoding.EncodeToString(
		ed25519.Sign(backendPrivateKey, backendDigest[:]),
	)
	operatorDigest := frostRetainedGroupOperatorAttestationDigest(digest)
	attestation.OperatorSignature = base64.StdEncoding.EncodeToString(
		ed25519.Sign(operatorPrivateKey, operatorDigest[:]),
	)
	attestation.Signature = base64.StdEncoding.EncodeToString(
		ed25519.Sign(attestationPrivateKey, digest[:]),
	)
	encoded, err := json.Marshal(attestation)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(encoded), nil
}

func frostRetainedGroupServerRequestTarget(
	identity FrostRetainedGroupEndpointIdentity,
	request *http.Request,
) (string, error) {
	if request == nil || request.URL == nil {
		return "", fmt.Errorf("retained-group server request target is missing")
	}
	base, _, err := validateFrostRetainedGroupTLSEndpoint(
		identity.CanonicalEndpoint,
	)
	if err != nil {
		return "", err
	}
	basePath := strings.TrimSuffix(base.Path, "/")
	if request.Host != base.Host ||
		request.URL.RawQuery != "" || request.URL.RawPath != "" ||
		request.URL.Fragment != "" || request.URL.Opaque != "" ||
		request.URL.Path == "" ||
		path.Clean(request.URL.Path) != request.URL.Path ||
		strings.Contains(request.URL.Path, "//") ||
		(request.URL.Path != base.Path &&
			!strings.HasPrefix(request.URL.Path, basePath+"/")) {
		return "", fmt.Errorf("retained-group server request escaped its endpoint")
	}
	base.Path = request.URL.Path
	base.RawPath = ""
	base.RawQuery = ""
	return base.String(), nil
}
