package tbtc

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
)

const (
	// FrostNativeSignerAnchorBootstrapClientConfigSchema is the canonical
	// owner-only transport configuration consumed by the online initialize
	// phase. It deliberately carries no anchor identity and no trust decision:
	// every semantic pin travels inside the offline-signed authorization
	// certificate, so a substituted transport config can only point the
	// request at a peer that is unable to produce a verifiable
	// acknowledgement.
	FrostNativeSignerAnchorBootstrapClientConfigSchema = "tbtc-frost-native-signer-anchor-bootstrap-client-config/v1"

	frostNativeSignerAnchorBootstrapClientConfigMaximumBytes int64 = 64 * 1024
)

// FrostNativeSignerAnchorBootstrapClientConfig is the parsed canonical
// transport configuration plus runtime-only injection points. The fields below
// the divider never come from configuration JSON: ClientPrivateKey overrides
// ClientPrivateKeyPath when set, and TLSRootCAs, Random, and Now are the same
// deployment/test seams the runtime anchor client exposes.
type FrostNativeSignerAnchorBootstrapClientConfig struct {
	Endpoint                    string
	ResponsePublicKey           [ed25519.PublicKeySize]byte
	ResponsePublicKeySPKISHA256 [32]byte
	EndpointLeafSPKIHash        [32]byte
	ClientPrivateKeyPath        string
	RequestTimeout              time.Duration

	ClientPrivateKey ed25519.PrivateKey
	TLSRootCAs       *x509.CertPool
	Random           io.Reader
	Now              func() time.Time
}

type frostNativeSignerAnchorBootstrapClientConfigWire struct {
	Schema                      string `json:"schema"`
	Endpoint                    string `json:"endpoint"`
	ResponsePublicKey           string `json:"responsePublicKey"`
	ResponsePublicKeySPKISHA256 string `json:"responsePublicKeySpkiSha256"`
	EndpointLeafSPKIHash        string `json:"endpointLeafSpkiHash"`
	ClientPrivateKeyPath        string `json:"clientPrivateKeyPath"`
	RequestTimeoutMilliseconds  string `json:"requestTimeoutMilliseconds"`
}

type frostNativeSignerAnchorInitializeRequestPayload struct {
	Kind             string                                `json:"kind"`
	Nonce            string                                `json:"nonce"`
	BindingHash      string                                `json:"bindingHash"`
	OperationID      string                                `json:"operationID"`
	TransitionDigest string                                `json:"transitionDigest"`
	Checkpoint       frostNativeSignerAnchorCheckpointWire `json:"checkpoint"`
}

type frostNativeSignerAnchorInitializeRequest struct {
	Schema              string                                          `json:"schema"`
	Payload             frostNativeSignerAnchorInitializeRequestPayload `json:"payload"`
	ClientPublicKeySPKI string                                          `json:"clientPublicKeySpki"`
	Signature           string                                          `json:"signature"`
}

// FrostNativeSignerAnchorBootstrapHTTPClient is the serialized fail-closed
// transport for the one-time create-if-absent bootstrap endpoint. Both kinds
// of bootstrap request ("initialize" and its reconciliation "read") are
// client-signed over a domain-separated fixed-width transcript and POSTed to
// the single initialize endpoint beside read/advance/history. A verified
// service statement that the stream holds a different genesis record poisons
// this client permanently; transport-shaped failures never do.
type FrostNativeSignerAnchorBootstrapHTTPClient struct {
	initializeEndpoint string
	httpClient         *http.Client
	requestTimeout     time.Duration
	clockSkew          time.Duration
	maximumAckLife     time.Duration

	responseKey      ed25519.PublicKey
	responseKeyRaw   [ed25519.PublicKeySize]byte
	responseKeyPin   [32]byte
	clientKey        ed25519.PrivateKey
	clientPublicKey  [ed25519.PublicKeySize]byte
	clientSPKIDER    []byte
	clientSPKIBase64 string
	random           io.Reader
	now              func() time.Time

	mutex    sync.Mutex
	poisoned error
}

var _ FrostNativeSignerAnchorBootstrapClient = (*FrostNativeSignerAnchorBootstrapHTTPClient)(nil)

// DecodeFrostNativeSignerAnchorBootstrapClientConfig strictly decodes the
// canonical transport configuration with the provisioning JSON machinery:
// duplicate and case-folded-duplicate members, non-ASCII member names, unknown
// members, trailing data, and non-canonical numbers are all rejected. The
// endpoint/leaf pairing enforces the provisioning identity rule: a zero
// endpoint leaf SPKI hash is legal only for a canonical numeric loopback HTTP
// endpoint, and mandatory for it.
func DecodeFrostNativeSignerAnchorBootstrapClientConfig(
	data []byte,
) (*FrostNativeSignerAnchorBootstrapClientConfig, error) {
	if len(data) == 0 ||
		int64(len(data)) > frostNativeSignerAnchorBootstrapClientConfigMaximumBytes {
		return nil, fmt.Errorf(
			"native signer anchor bootstrap client config size is invalid",
		)
	}
	wire := &frostNativeSignerAnchorBootstrapClientConfigWire{}
	if err := decodeStrictFrostNativeSignerAnchorProvisioningJSON(
		data,
		wire,
	); err != nil {
		return nil, err
	}
	if wire.Schema != FrostNativeSignerAnchorBootstrapClientConfigSchema {
		return nil, fmt.Errorf(
			"unsupported native signer anchor bootstrap client config schema",
		)
	}
	_, https, err := validateFrostNativeSignerAnchorEndpoint(wire.Endpoint)
	if err != nil {
		return nil, err
	}
	responseKey, err := frostNativeSignerAnchorParseHex32(wire.ResponsePublicKey)
	if err != nil {
		return nil, fmt.Errorf("invalid bootstrap client config response key: %w", err)
	}
	if err := ValidateFrostNativeSignerAnchorTrustEd25519PublicKey(
		responseKey[:],
	); err != nil {
		return nil, fmt.Errorf("invalid bootstrap client config response key: %w", err)
	}
	responsePin, err := frostNativeSignerAnchorParseHex32(
		wire.ResponsePublicKeySPKISHA256,
	)
	if err != nil ||
		ComputeFrostNativeSignerAnchorTrustEd25519SPKISHA256(responseKey) !=
			responsePin {
		return nil, fmt.Errorf(
			"bootstrap client config response key differs from its SPKI pin",
		)
	}
	leafHash, err := frostNativeSignerAnchorParseHex32(wire.EndpointLeafSPKIHash)
	if err != nil {
		return nil, fmt.Errorf(
			"invalid bootstrap client config endpoint leaf SPKI hash: %w",
			err,
		)
	}
	if https && leafHash == [32]byte{} {
		return nil, fmt.Errorf(
			"HTTPS bootstrap client config requires a nonzero endpoint leaf SPKI hash",
		)
	}
	if !https && leafHash != [32]byte{} {
		return nil, fmt.Errorf(
			"loopback HTTP bootstrap client config must use a zero endpoint leaf SPKI hash",
		)
	}
	if !filepath.IsAbs(wire.ClientPrivateKeyPath) ||
		filepath.Clean(wire.ClientPrivateKeyPath) != wire.ClientPrivateKeyPath {
		return nil, fmt.Errorf(
			"bootstrap client config key path is not canonical absolute",
		)
	}
	timeoutMilliseconds, err := frostNativeSignerAnchorParseUint64(
		wire.RequestTimeoutMilliseconds,
	)
	if err != nil || timeoutMilliseconds == 0 ||
		timeoutMilliseconds >
			uint64(frostNativeSignerAnchorMaximumRequestTimeout/time.Millisecond) {
		return nil, fmt.Errorf(
			"bootstrap client config request timeout is invalid",
		)
	}
	return &FrostNativeSignerAnchorBootstrapClientConfig{
		Endpoint:                    wire.Endpoint,
		ResponsePublicKey:           responseKey,
		ResponsePublicKeySPKISHA256: responsePin,
		EndpointLeafSPKIHash:        leafHash,
		ClientPrivateKeyPath:        wire.ClientPrivateKeyPath,
		RequestTimeout:              time.Duration(timeoutMilliseconds) * time.Millisecond,
	}, nil
}

// LoadFrostNativeSignerAnchorBootstrapClient reads the owner-only canonical
// config artifact, loads the referenced client key, and constructs the
// hardened bootstrap transport. Construction performs no network activity.
func LoadFrostNativeSignerAnchorBootstrapClient(
	configPath string,
) (*FrostNativeSignerAnchorBootstrapHTTPClient, error) {
	configJSON, err := ReadFrostNativeSignerAnchorProvisioningArtifact(
		configPath,
		frostNativeSignerAnchorBootstrapClientConfigMaximumBytes,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot read native signer anchor bootstrap client config: %w",
			err,
		)
	}
	config, err := DecodeFrostNativeSignerAnchorBootstrapClientConfig(configJSON)
	if err != nil {
		return nil, err
	}
	return NewFrostNativeSignerAnchorBootstrapClient(*config)
}

// NewFrostNativeSignerAnchorBootstrapClient validates every transport pin
// before constructing an HTTP client. HTTPS uses normal PKIX verification
// plus an exact leaf-SPKI pin; plaintext HTTP is restricted to a canonical
// numeric loopback endpoint with a zero leaf pin. Proxies and redirects are
// always disabled, mirroring the runtime anchor client exactly.
func NewFrostNativeSignerAnchorBootstrapClient(
	config FrostNativeSignerAnchorBootstrapClientConfig,
) (*FrostNativeSignerAnchorBootstrapHTTPClient, error) {
	endpoint, https, err := validateFrostNativeSignerAnchorEndpoint(config.Endpoint)
	if err != nil {
		return nil, err
	}
	if https {
		if config.EndpointLeafSPKIHash == [32]byte{} {
			return nil, fmt.Errorf(
				"HTTPS native signer anchor bootstrap endpoint requires a leaf SPKI pin",
			)
		}
	} else {
		if config.EndpointLeafSPKIHash != [32]byte{} {
			return nil, fmt.Errorf(
				"loopback HTTP native signer anchor bootstrap endpoint must use a zero leaf SPKI pin",
			)
		}
		if config.TLSRootCAs != nil {
			return nil, fmt.Errorf(
				"loopback HTTP native signer anchor bootstrap cannot configure TLS roots",
			)
		}
	}
	if err := ValidateFrostNativeSignerAnchorTrustEd25519PublicKey(
		config.ResponsePublicKey[:],
	); err != nil {
		return nil, fmt.Errorf(
			"invalid native signer anchor bootstrap response key: %w",
			err,
		)
	}
	if ComputeFrostNativeSignerAnchorTrustEd25519SPKISHA256(
		config.ResponsePublicKey,
	) != config.ResponsePublicKeySPKISHA256 {
		return nil, fmt.Errorf(
			"native signer anchor bootstrap response key differs from its SPKI pin",
		)
	}
	clientKey := config.ClientPrivateKey
	if clientKey == nil {
		clientKey, err = loadFrostNativeSignerAnchorClientPrivateKey(
			config.ClientPrivateKeyPath,
		)
		if err != nil {
			return nil, err
		}
	}
	if len(clientKey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf(
			"native signer anchor bootstrap client key is not Ed25519",
		)
	}
	clientPublic := [ed25519.PublicKeySize]byte{}
	copy(clientPublic[:], clientKey.Public().(ed25519.PublicKey))
	if err := ValidateFrostNativeSignerAnchorTrustEd25519PublicKey(
		clientPublic[:],
	); err != nil {
		return nil, fmt.Errorf(
			"native signer anchor bootstrap client key point is invalid: %w",
			err,
		)
	}
	if clientPublic == config.ResponsePublicKey {
		return nil, fmt.Errorf(
			"native signer anchor bootstrap client and response keys must be distinct",
		)
	}
	clientSPKIDER, err := x509.MarshalPKIXPublicKey(clientKey.Public())
	if err != nil {
		return nil, fmt.Errorf(
			"cannot encode native signer anchor bootstrap client key: %w",
			err,
		)
	}
	requestTimeout := config.RequestTimeout
	if requestTimeout == 0 {
		requestTimeout = frostNativeSignerAnchorDefaultRequestTimeout
	}
	if requestTimeout <= 0 ||
		requestTimeout > frostNativeSignerAnchorMaximumRequestTimeout {
		return nil, fmt.Errorf(
			"native signer anchor bootstrap request timeout is invalid",
		)
	}
	randomSource := config.Random
	if randomSource == nil {
		randomSource = rand.Reader
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}

	transport := &http.Transport{
		Proxy:                  nil,
		DialContext:            (&net.Dialer{Timeout: requestTimeout, KeepAlive: -1}).DialContext,
		DisableKeepAlives:      true,
		DisableCompression:     true,
		ForceAttemptHTTP2:      false,
		MaxConnsPerHost:        1,
		ResponseHeaderTimeout:  requestTimeout,
		TLSHandshakeTimeout:    requestTimeout,
		ExpectContinueTimeout:  time.Second,
		MaxResponseHeaderBytes: 16 * 1024,
	}
	if https {
		expectedLeafSPKIHash := config.EndpointLeafSPKIHash
		var rootCAs *x509.CertPool
		if config.TLSRootCAs != nil {
			rootCAs = config.TLSRootCAs.Clone()
		}
		transport.TLSClientConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
			RootCAs:    rootCAs,
			VerifyConnection: func(state tls.ConnectionState) error {
				if len(state.VerifiedChains) == 0 || len(state.PeerCertificates) == 0 {
					return fmt.Errorf(
						"native signer anchor bootstrap TLS peer is not PKIX-verified",
					)
				}
				if sha256.Sum256(state.PeerCertificates[0].RawSubjectPublicKeyInfo) !=
					expectedLeafSPKIHash {
					return fmt.Errorf(
						"native signer anchor bootstrap TLS leaf SPKI mismatch",
					)
				}
				return nil
			},
		}
	}

	// Copy secret material only after every fallible validation/construction
	// step, mirroring the runtime anchor client constructor.
	keyCopy := append(ed25519.PrivateKey{}, clientKey...)
	return &FrostNativeSignerAnchorBootstrapHTTPClient{
		initializeEndpoint: frostNativeSignerAnchorOperationEndpoint(
			endpoint,
			"initialize",
		),
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   requestTimeout,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return fmt.Errorf(
					"native signer anchor bootstrap redirects are disabled",
				)
			},
		},
		requestTimeout:   requestTimeout,
		clockSkew:        frostNativeSignerAnchorDefaultClockSkew,
		maximumAckLife:   frostNativeSignerAnchorDefaultAcknowledgementLifetime,
		responseKey:      append(ed25519.PublicKey{}, config.ResponsePublicKey[:]...),
		responseKeyRaw:   config.ResponsePublicKey,
		responseKeyPin:   config.ResponsePublicKeySPKISHA256,
		clientKey:        keyCopy,
		clientPublicKey:  clientPublic,
		clientSPKIDER:    append([]byte{}, clientSPKIDER...),
		clientSPKIBase64: base64StdEncoding(clientSPKIDER),
		random:           randomSource,
		now:              now,
	}, nil
}

// InitializeFrostNativeSignerAnchor submits the offline-authorized
// create-if-absent request and then always reconciles through a fresh signed
// exact read before reporting success. The returned record therefore carries
// both the exact stored genesis acknowledgement and the exact read-recovery
// JSON required by InitializeFrostNativeSignerAnchorBootstrap. Outcome
// classes: pre-send failures are retryable; post-send verification failures
// are ambiguous and resolved only by the read; an authenticated read showing a
// different genesis record permanently poisons the client.
func (client *FrostNativeSignerAnchorBootstrapHTTPClient) InitializeFrostNativeSignerAnchor(
	ctx context.Context,
	authorization FrostNativeSignerAnchorBootstrapAuthorization,
) (*FrostNativeSignerAnchorBootstrapClientResult, error) {
	if client == nil {
		return nil, fmt.Errorf("native signer anchor bootstrap client is nil")
	}
	if ctx == nil {
		return nil, fmt.Errorf("native signer anchor bootstrap context is nil")
	}
	client.mutex.Lock()
	defer client.mutex.Unlock()
	if client.poisoned != nil {
		return nil, fmt.Errorf(
			"native signer anchor bootstrap client is poisoned: %w",
			client.poisoned,
		)
	}
	certificate := authorization.Certificate
	if err := client.validateBootstrapAuthorization(&certificate); err != nil {
		return nil, err
	}
	verifier := client.bootstrapAnchorVerifierLocked(&certificate)
	sentinel, sent, initializeErr := client.initializeAttemptLocked(
		ctx,
		verifier,
		&certificate,
	)
	if initializeErr != nil && !sent {
		// The request never left this process, so the stream state is
		// untouched and the idempotent ceremony can simply be retried.
		return nil, initializeErr
	}
	acknowledgement, err := client.reconcileBootstrapReadLocked(
		ctx,
		verifier,
		&certificate,
		sentinel,
		initializeErr,
	)
	if err != nil {
		return nil, err
	}
	return &FrostNativeSignerAnchorBootstrapClientResult{
		Record: frostNativeSignerAnchorRecord(acknowledgement),
	}, nil
}

// validateBootstrapAuthorization re-derives every commitment of the offline
// core before any network activity. The client refuses to transmit an
// authorization whose digests, offline signature, genesis checkpoint, or
// response-key pins do not verify against its own transport configuration:
// pre-send validation failures are retryable and never poison.
func (client *FrostNativeSignerAnchorBootstrapHTTPClient) validateBootstrapAuthorization(
	certificate *FrostNativeSignerAnchorTrustCertificate,
) error {
	if certificate.Kind != FrostNativeSignerAnchorTrustCertificateBootstrap ||
		certificate.CertificateSequence != 1 ||
		certificate.PreviousCertificateDigest != [32]byte{} ||
		certificate.From != nil ||
		certificate.ProtocolID == [32]byte{} ||
		certificate.StreamID == [32]byte{} ||
		certificate.SignerStoreFingerprint == [32]byte{} {
		return fmt.Errorf(
			"bootstrap authorization is not a first bootstrap certificate",
		)
	}
	endpoint := certificate.To
	if endpoint.ActivationManifestHash == [32]byte{} ||
		endpoint.ActivationManifestSequence == 0 ||
		endpoint.BindingHash == [32]byte{} {
		return fmt.Errorf("bootstrap authorization endpoint pins are invalid")
	}
	if err := frostsigning.ValidateNativeTBTCSignerStateWitnessGeometry(
		endpoint.WitnessMaximumRecords,
		endpoint.WitnessRotationThresholdRecords,
	); err != nil {
		return fmt.Errorf(
			"bootstrap authorization endpoint witness geometry is invalid: %w",
			err,
		)
	}
	if endpoint.ResponsePublicKey != client.responseKeyRaw ||
		endpoint.ResponsePublicKeySPKISHA256 != client.responseKeyPin {
		return fmt.Errorf(
			"bootstrap authorization response key differs from the transport pin",
		)
	}
	if err := ValidateFrostNativeSignerAnchorTrustEd25519PublicKey(
		endpoint.OfflineAuthorityPublicKey[:],
	); err != nil {
		return fmt.Errorf(
			"bootstrap authorization offline authority key is invalid: %w",
			err,
		)
	}
	if ComputeFrostNativeSignerAnchorTrustEd25519SPKISHA256(
		endpoint.OfflineAuthorityPublicKey,
	) != endpoint.OfflineAuthoritySPKISHA256 ||
		endpoint.OfflineAuthorityPublicKey == endpoint.ResponsePublicKey ||
		endpoint.OfflineAuthorityPublicKey == client.clientPublicKey ||
		endpoint.ResponsePublicKey == client.clientPublicKey {
		return fmt.Errorf(
			"bootstrap authorization cryptographic roles are not pairwise distinct",
		)
	}
	reference := endpoint.Reference
	if reference.ServiceEpoch != 1 || reference.Revision != 0 ||
		reference.PreviousEventRoot != [32]byte{} ||
		reference.EventRoot != [32]byte{} ||
		reference.AcknowledgementDigest != [32]byte{} {
		return fmt.Errorf(
			"bootstrap authorization reference must be the unassigned first epoch",
		)
	}
	checkpoint := reference.Checkpoint
	if err := validateFrostNativeSignerAnchorCheckpoint(
		checkpoint,
		certificate.SignerStoreFingerprint,
	); err != nil {
		return err
	}
	if checkpoint.Generation != 1 ||
		checkpoint.PreviousStateCommitment !=
			frostsigning.ComputeNativeTBTCSignerStateWitnessGenesis(
				checkpoint.StoreFingerprint,
			) {
		return fmt.Errorf(
			"bootstrap authorization checkpoint is not the exact genesis",
		)
	}
	if len(certificate.TargetAcknowledgement) != 0 ||
		certificate.TargetAcknowledgementSHA256 != [32]byte{} ||
		certificate.FinalSignature != [ed25519.SignatureSize]byte{} ||
		certificate.CertificateDigest != [32]byte{} {
		return fmt.Errorf(
			"bootstrap authorization already carries service or final material",
		)
	}
	coreDigest, err := ComputeFrostNativeSignerAnchorTrustCoreDigest(certificate)
	if err != nil || coreDigest != certificate.CoreDigest {
		return fmt.Errorf("bootstrap authorization core digest mismatch")
	}
	operationID := ComputeFrostNativeSignerAnchorTrustOperationID(coreDigest)
	if operationID != certificate.OperationID ||
		ComputeFrostNativeSignerAnchorTrustTransitionDigest(
			coreDigest,
			operationID,
		) != certificate.TransitionDigest {
		return fmt.Errorf(
			"bootstrap authorization operation or transition digest mismatch",
		)
	}
	if !ed25519.Verify(
		ed25519.PublicKey(endpoint.OfflineAuthorityPublicKey[:]),
		coreDigest[:],
		certificate.CoreSignature[:],
	) {
		return fmt.Errorf(
			"bootstrap authorization offline core signature is invalid",
		)
	}
	return nil
}

// bootstrapAnchorVerifierLocked builds a deliberately partial runtime anchor
// client value in order to reuse its hardened POST, nonce, and acknowledgement
// verification methods against the certificate-scoped binding without
// re-implementing them. Exactly the fields those three methods read are
// populated; the value never escapes this client, and its stateful
// Read/CAS/History entry points are never invoked (their guards require state
// this value does not have).
func (client *FrostNativeSignerAnchorBootstrapHTTPClient) bootstrapAnchorVerifierLocked(
	certificate *FrostNativeSignerAnchorTrustCertificate,
) *FrostNativeSignerAnchorClient {
	return &FrostNativeSignerAnchorClient{
		httpClient:     client.httpClient,
		requestTimeout: client.requestTimeout,
		clockSkew:      client.clockSkew,
		maximumAckLife: client.maximumAckLife,
		identity: FrostNativeSignerAnchorIdentity{
			SignerStoreFingerprint: certificate.SignerStoreFingerprint,
			OnlineKeyHash:          certificate.To.ResponsePublicKeySPKISHA256,
		},
		bindingHash: certificate.To.BindingHash,
		onlineKey:   client.responseKey,
		random:      client.random,
		now:         client.now,
	}
}

// initializeAttemptLocked posts the create-if-absent request. On success it
// returns the verified signed sentinel ("applied" on first create,
// "already-applied" on idempotent replay), each bound to this exact request
// digest and nonce. Once the POST may have reached the service every failure
// is reported with sent=true: it must be resolved by a fresh signed read,
// never by trusting local state.
func (client *FrostNativeSignerAnchorBootstrapHTTPClient) initializeAttemptLocked(
	ctx context.Context,
	verifier *FrostNativeSignerAnchorClient,
	certificate *FrostNativeSignerAnchorTrustCertificate,
) (*FrostNativeSignerCheckpointAcknowledgement, bool, error) {
	checkpoint := certificate.To.Reference.Checkpoint
	response, requestDigest, nonce, sent, err := client.postBootstrapRequestLocked(
		ctx,
		verifier,
		certificate,
		"initialize",
	)
	if err != nil {
		return nil, sent, err
	}
	acknowledgement, err := verifier.verifyAcknowledgement(
		response,
		&requestDigest,
		&nonce,
		&checkpoint,
		&certificate.OperationID,
		true,
		"applied",
		"already-applied",
	)
	if err != nil {
		// The service may have committed before answering with a malformed,
		// truncated, or stale body, so any post-send verification failure is
		// ambiguous rather than terminal.
		return nil, true, fmt.Errorf(
			"invalid native signer anchor initialize acknowledgement: %w",
			err,
		)
	}
	if acknowledgement.TransitionDigest != certificate.TransitionDigest ||
		acknowledgement.ServiceEpoch != 1 ||
		acknowledgement.Revision != 1 ||
		acknowledgement.PreviousEventRoot != [32]byte{} {
		return nil, true, fmt.Errorf(
			"native signer anchor initialize acknowledgement is outside the exact genesis identity",
		)
	}
	return acknowledgement, true, nil
}

// postBootstrapRequestLocked signs and posts one bootstrap request of the
// given kind. Kind is bound first inside the signed transcript, so an
// "initialize" signature can never be replayed as a "read" or vice versa.
func (client *FrostNativeSignerAnchorBootstrapHTTPClient) postBootstrapRequestLocked(
	ctx context.Context,
	verifier *FrostNativeSignerAnchorClient,
	certificate *FrostNativeSignerAnchorTrustCertificate,
	kind string,
) ([]byte, [32]byte, [32]byte, bool, error) {
	checkpoint := certificate.To.Reference.Checkpoint
	nonce, err := verifier.randomBytes32()
	if err != nil {
		return nil, [32]byte{}, [32]byte{}, false, fmt.Errorf(
			"cannot create native signer anchor bootstrap nonce: %w",
			err,
		)
	}
	transcript := frostNativeSignerAnchorInitializeRequestTranscript(
		kind,
		certificate.To.BindingHash,
		nonce,
		certificate.OperationID,
		certificate.TransitionDigest,
		checkpoint,
		client.clientSPKIDER,
	)
	requestDigest := sha256.Sum256(transcript)
	request := frostNativeSignerAnchorInitializeRequest{
		Schema: FrostNativeSignerAnchorInitializeRequestSchema,
		Payload: frostNativeSignerAnchorInitializeRequestPayload{
			Kind:             kind,
			Nonce:            frostNativeSignerAnchorHex32(nonce),
			BindingHash:      frostNativeSignerAnchorHex32(certificate.To.BindingHash),
			OperationID:      frostNativeSignerAnchorHex32(certificate.OperationID),
			TransitionDigest: frostNativeSignerAnchorHex32(certificate.TransitionDigest),
			Checkpoint:       frostNativeSignerAnchorCheckpointToWire(checkpoint),
		},
		ClientPublicKeySPKI: client.clientSPKIBase64,
		Signature: frostNativeSignerAnchorSignatureHex(
			ed25519.Sign(client.clientKey, transcript),
		),
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, [32]byte{}, [32]byte{}, false, fmt.Errorf(
			"cannot encode native signer anchor bootstrap request: %w",
			err,
		)
	}
	response, sent, err := verifier.post(ctx, client.initializeEndpoint, payload)
	if err != nil {
		return nil, [32]byte{}, [32]byte{}, sent, err
	}
	return response, requestDigest, nonce, true, nil
}

// reconcileBootstrapReadLocked performs the mandatory fresh signed exact read
// after every sent initialize attempt and requires field-for-field agreement
// with the expected genesis record. Divergence policy: only a response that is
// authentic (service-signed) and bound to this exact request digest and nonce
// can poison the client; every transport-shaped or unverifiable outcome stays
// a retryable error.
func (client *FrostNativeSignerAnchorBootstrapHTTPClient) reconcileBootstrapReadLocked(
	ctx context.Context,
	verifier *FrostNativeSignerAnchorClient,
	certificate *FrostNativeSignerAnchorTrustCertificate,
	sentinel *FrostNativeSignerCheckpointAcknowledgement,
	initializeErr error,
) (*FrostNativeSignerCheckpointAcknowledgement, error) {
	checkpoint := certificate.To.Reference.Checkpoint
	response, requestDigest, nonce, _, err := client.postBootstrapRequestLocked(
		ctx,
		verifier,
		certificate,
		"read",
	)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot reconcile native signer anchor bootstrap with a fresh signed read: %w",
			err,
		)
	}
	readResponse := frostNativeSignerAnchorReadResponse{}
	if err := decodeStrictFrostNativeSignerAnchorJSON(
		response,
		&readResponse,
	); err != nil {
		return nil, fmt.Errorf(
			"invalid native signer anchor bootstrap read response: %w",
			err,
		)
	}
	if readResponse.Schema != FrostNativeSignerAnchorReadResponseSchema {
		return nil, fmt.Errorf(
			"unsupported native signer anchor bootstrap read response schema",
		)
	}
	responseDigest, err := frostNativeSignerAnchorReadResponseTranscript(
		readResponse,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"invalid native signer anchor bootstrap read response: %w",
			err,
		)
	}
	responseSignature, err := frostNativeSignerAnchorParseSignature(
		readResponse.Signature,
	)
	if err != nil || !ed25519.Verify(
		client.responseKey,
		responseDigest,
		responseSignature[:],
	) {
		return nil, fmt.Errorf(
			"native signer anchor bootstrap read signature is invalid",
		)
	}
	responseRequestDigest, err := frostNativeSignerAnchorParseHex32(
		readResponse.RequestDigest,
	)
	if err != nil || responseRequestDigest != requestDigest {
		return nil, fmt.Errorf(
			"native signer anchor bootstrap read request digest mismatch",
		)
	}
	responseNonce, err := frostNativeSignerAnchorParseHex32(readResponse.Nonce)
	if err != nil || responseNonce != nonce {
		return nil, fmt.Errorf(
			"native signer anchor bootstrap read nonce mismatch",
		)
	}
	// From this point the response is authentic and bound to this exact
	// request; the statements below are the service's own, so contradictions
	// are divergence, not transport noise.
	responseBindingHash, err := frostNativeSignerAnchorParseHex32(
		readResponse.BindingHash,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"invalid native signer anchor bootstrap read binding hash",
		)
	}
	if responseBindingHash != certificate.To.BindingHash {
		return nil, client.poisonLocked(fmt.Errorf(
			"authenticated read binding hash differs from the authorized stream",
		))
	}
	switch readResponse.Status {
	case "absent":
		if sentinel != nil {
			return nil, client.poisonLocked(fmt.Errorf(
				"service acknowledged the genesis event and then reported an absent stream",
			))
		}
		return nil, fmt.Errorf(
			"native signer anchor bootstrap did not commit and can be retried: %w",
			initializeErr,
		)
	case "present":
	default:
		return nil, fmt.Errorf(
			"unsupported native signer anchor bootstrap read status",
		)
	}
	if readResponse.Checkpoint == nil {
		return nil, fmt.Errorf(
			"native signer anchor bootstrap read checkpoint is absent",
		)
	}
	storedCheckpoint, err := frostNativeSignerAnchorCheckpointFromWire(
		*readResponse.Checkpoint,
	)
	if err != nil {
		return nil, err
	}
	operationID, err := frostNativeSignerAnchorParseHex32(
		readResponse.OperationID,
	)
	if err != nil {
		return nil, err
	}
	transitionDigest, err := frostNativeSignerAnchorParseHex32(
		readResponse.TransitionDigest,
	)
	if err != nil {
		return nil, err
	}
	if storedCheckpoint != checkpoint ||
		operationID != certificate.OperationID ||
		transitionDigest != certificate.TransitionDigest {
		return nil, client.poisonLocked(fmt.Errorf(
			"stream already holds a different genesis record: checkpoint, operation, or transition differs",
		))
	}
	// The stored revision-one event of a bootstrap-created stream must be the
	// "applied" acknowledgement: that exact JSON is what the offline final
	// signature and the native recovery ABI ratify.
	acknowledgement, err := verifier.verifyAcknowledgement(
		readResponse.CheckpointAck,
		nil,
		nil,
		&checkpoint,
		&operationID,
		false,
		"applied",
	)
	if err != nil {
		return nil, fmt.Errorf(
			"invalid stored native signer bootstrap acknowledgement: %w",
			err,
		)
	}
	if acknowledgement.TransitionDigest != transitionDigest ||
		acknowledgement.ServiceEpoch != 1 ||
		acknowledgement.Revision != 1 ||
		acknowledgement.PreviousEventRoot != [32]byte{} {
		return nil, fmt.Errorf(
			"stored native signer bootstrap acknowledgement is outside the exact genesis identity",
		)
	}
	serviceEpoch, err := frostNativeSignerAnchorParseUint64(
		readResponse.ServiceEpoch,
	)
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
			"native signer anchor bootstrap read summary differs from its stored acknowledgement",
		)
	}
	committedAt, err := frostNativeSignerAnchorParseUint64(
		readResponse.CommittedAtUnixMs,
	)
	if err != nil {
		return nil, err
	}
	expiresAt, err := frostNativeSignerAnchorParseUint64(
		readResponse.ExpiresAtUnixMs,
	)
	if err != nil {
		return nil, err
	}
	nowUnixMs := client.now().UnixMilli()
	if nowUnixMs < 0 || committedAt == 0 || expiresAt <= committedAt ||
		expiresAt-committedAt > uint64(client.maximumAckLife/time.Millisecond) ||
		committedAt > uint64(nowUnixMs)+uint64(client.clockSkew/time.Millisecond) ||
		expiresAt <= uint64(nowUnixMs) {
		return nil, fmt.Errorf(
			"native signer anchor bootstrap read response is stale or has an invalid lifetime",
		)
	}
	// A first-create sentinel and the stored event are the same revision-one
	// acknowledgement, so any difference is service equivocation at an equal
	// revision. An "already-applied" sentinel is a fresh signature bound to
	// its own request and legitimately differs from the stored event bytes.
	if sentinel != nil && sentinel.Status == "applied" &&
		!equalFrostNativeSignerCheckpointAcknowledgements(
			sentinel,
			acknowledgement,
		) {
		return nil, client.poisonLocked(fmt.Errorf(
			"stored genesis acknowledgement differs from the freshly applied acknowledgement at an equal revision",
		))
	}
	acknowledgement.ExactReadRecovery = append([]byte{}, response...)
	acknowledgement.ReadRecoveryExpiresAt = expiresAt
	return acknowledgement, nil
}

// poisonLocked records the terminal divergence cause and returns the exact
// error every subsequent call will repeat immediately without touching the
// network.
func (client *FrostNativeSignerAnchorBootstrapHTTPClient) poisonLocked(
	cause error,
) error {
	client.poisoned = cause
	return fmt.Errorf(
		"native signer anchor bootstrap client is poisoned: %w",
		cause,
	)
}

// frostNativeSignerAnchorInitializeRequestTranscript is the fixed-width
// client-signed bootstrap request commitment. The kind is bound immediately
// after the schema so the create ("initialize") and reconciliation ("read")
// signatures can never be replayed for one another, and every semantic pin of
// the offline-authorized operation is bound directly: no JSON bytes are ever
// signed.
func frostNativeSignerAnchorInitializeRequestTranscript(
	kind string,
	bindingHash [32]byte,
	nonce [32]byte,
	operationID [32]byte,
	transitionDigest [32]byte,
	checkpoint FrostNativeSignerStateWitnessCheckpoint,
	clientSPKIDER []byte,
) []byte {
	transcript := newFrostNativeSignerAnchorTranscript(
		frostNativeSignerAnchorInitializeRequestDomain,
	)
	transcript.string("schema", FrostNativeSignerAnchorInitializeRequestSchema)
	transcript.string("kind", kind)
	transcript.bytes32("bindingHash", bindingHash)
	transcript.bytes32("nonce", nonce)
	transcript.bytes32("operationID", operationID)
	transcript.bytes32("transitionDigest", transitionDigest)
	frostNativeSignerAnchorWriteCheckpoint(transcript, "checkpoint", checkpoint)
	transcript.field("clientPublicKeySpki", clientSPKIDER)
	return transcript.bytes()
}
