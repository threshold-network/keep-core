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
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
)

const (
	frostNativeSignerAnchorDefaultRequestTimeout          = 3 * time.Second
	frostNativeSignerAnchorMaximumRequestTimeout          = 30 * time.Second
	frostNativeSignerAnchorDefaultClockSkew               = 5 * time.Second
	frostNativeSignerAnchorMaximumClockSkew               = 5 * time.Second
	frostNativeSignerAnchorDefaultAcknowledgementLifetime = 30 * time.Second
	frostNativeSignerAnchorMaximumAcknowledgementLifetime = 30 * time.Second

	// frostNativeSignerAnchorReadAttempts bounds how many times one
	// authenticated anchor read may be re-issued when the service could not be
	// reached. It is small on purpose: the point is to ride out a single blip,
	// not to wait out an outage, and the attempts share one request-timeout
	// budget rather than each getting their own.
	frostNativeSignerAnchorReadAttempts = 3

	// frostNativeSignerAnchorReadRetryBackoff is the pause before the second
	// attempt and doubles for the third. Two pauses plus three attempts stay
	// well inside the smallest sensible request timeout, so the backoff never
	// becomes the reason a budget is exhausted.
	frostNativeSignerAnchorReadRetryBackoff = 50 * time.Millisecond
)

// frostNativeSignerAnchorStatusError reports the HTTP status an anchor request
// was answered with. It exists so an idempotent read can tell a service that is
// temporarily unable to answer from one that answered something wrong; its
// message is the one this failure has always carried.
type frostNativeSignerAnchorStatusError struct {
	statusCode int
}

func (err *frostNativeSignerAnchorStatusError) Error() string {
	return fmt.Sprintf(
		"native signer anchor returned HTTP status [%d]",
		err.statusCode,
	)
}

func (err *frostNativeSignerAnchorStatusError) HTTPStatusCode() int {
	return err.statusCode
}

// FrostNativeSignerAnchorClientConfig contains runtime transport material and
// the complete identity extracted from an independently authenticated
// activation manifest. OnlinePublicKeySPKI and ClientPrivateKey are copied by
// the constructor; their hashes must exactly match Identity.
type FrostNativeSignerAnchorClientConfig struct {
	Endpoint                       string
	RequestTimeout                 time.Duration
	MaximumClockSkew               time.Duration
	MaximumAcknowledgementLifetime time.Duration
	TLSRootCAs                     *x509.CertPool
	ClientPrivateKey               ed25519.PrivateKey
	OnlinePublicKeySPKI            []byte
	Identity                       FrostNativeSignerAnchorIdentity
	Random                         io.Reader
	Now                            func() time.Time
}

// FrostNativeSignerAnchorClient is a serialized fail-closed authenticated
// client for the independent state-witness history service.
type FrostNativeSignerAnchorClient struct {
	readEndpoint    string
	advanceEndpoint string
	historyEndpoint string
	httpClient      *http.Client
	requestTimeout  time.Duration
	clockSkew       time.Duration
	maximumAckLife  time.Duration

	identity            FrostNativeSignerAnchorIdentity
	bindingHash         [32]byte
	clientKey           ed25519.PrivateKey
	clientSPKIDER       []byte
	clientSPKIBase64    string
	onlineKey           ed25519.PublicKey
	certifiedTrustFloor *FrostNativeSignerAnchorTrustCertificate
	random              io.Reader
	now                 func() time.Time

	mutex             sync.Mutex
	last              *FrostNativeSignerCheckpointAcknowledgement
	readPermit        *FrostNativeSignerStateWitnessCheckpoint
	readPermitExpires uint64
	poisoned          error
}

// NewFrostNativeSignerAnchorClient validates all immutable pins before it
// constructs a transport. HTTPS uses normal PKIX verification plus an exact
// leaf-SPKI pin; plaintext HTTP is restricted to a canonical numeric loopback
// endpoint. Proxies and redirects are always disabled.
func NewFrostNativeSignerAnchorClient(
	config FrostNativeSignerAnchorClientConfig,
) (*FrostNativeSignerAnchorClient, error) {
	return newFrostNativeSignerAnchorClient(config, nil)
}

// newFrostNativeSignerAnchorClientWithTrustFloor is deliberately private. A
// revision-one acknowledgement with a non-zero predecessor crosses a service
// epoch and therefore cannot be admitted from caller-supplied client
// configuration. The only way to obtain the capability accepted here is the
// full offline-authority certificate-chain validator.
func newFrostNativeSignerAnchorClientWithTrustFloor(
	config FrostNativeSignerAnchorClientConfig,
	trustFloor *frostNativeSignerAnchorVerifiedTrustFloor,
) (*FrostNativeSignerAnchorClient, error) {
	if trustFloor == nil {
		return nil, fmt.Errorf(
			"verified native signer anchor trust-floor capability is nil",
		)
	}
	return newFrostNativeSignerAnchorClient(config, trustFloor)
}

func newFrostNativeSignerAnchorClient(
	config FrostNativeSignerAnchorClientConfig,
	trustFloor *frostNativeSignerAnchorVerifiedTrustFloor,
) (*FrostNativeSignerAnchorClient, error) {
	endpoint, https, err := validateFrostNativeSignerAnchorEndpoint(config.Endpoint)
	if err != nil {
		return nil, err
	}
	if err := validateFrostNativeSignerAnchorIdentity(config.Identity, https); err != nil {
		return nil, fmt.Errorf("invalid native signer anchor identity: %w", err)
	}
	if ComputeFrostNativeSignerAnchorTransportBinding(config.Endpoint) !=
		config.Identity.TransportBinding {
		return nil, fmt.Errorf("native signer anchor endpoint differs from its transport binding")
	}

	if len(config.ClientPrivateKey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("native signer anchor client key is not Ed25519")
	}
	clientSPKIDER, err := x509.MarshalPKIXPublicKey(config.ClientPrivateKey.Public())
	if err != nil {
		return nil, fmt.Errorf("cannot encode native signer anchor client key: %w", err)
	}
	if sha256.Sum256(clientSPKIDER) != config.Identity.ClientSPKIHash {
		return nil, fmt.Errorf("native signer anchor client key differs from its identity")
	}
	onlineKey, err := parseFrostNativeSignerAnchorOnlineKey(config.OnlinePublicKeySPKI)
	if err != nil {
		return nil, err
	}
	if sha256.Sum256(config.OnlinePublicKeySPKI) != config.Identity.OnlineKeyHash {
		return nil, fmt.Errorf("native signer anchor online key differs from its identity")
	}
	var certifiedTrustFloor *FrostNativeSignerAnchorTrustCertificate
	if trustFloor != nil {
		certificate := frostNativeSignerAnchorTrustCloneCertificate(
			&trustFloor.certificate,
		)
		rawOnlineKey := [ed25519.PublicKeySize]byte{}
		copy(rawOnlineKey[:], onlineKey)
		if certificate.ProtocolID != config.Identity.ProtocolID ||
			certificate.StreamID != config.Identity.StreamID ||
			certificate.SignerStoreFingerprint !=
				config.Identity.SignerStoreFingerprint ||
			certificate.To.BindingHash !=
				ComputeFrostNativeSignerAnchorBindingHash(config.Identity) ||
			certificate.To.ResponsePublicKey != rawOnlineKey ||
			certificate.To.ResponsePublicKeySPKISHA256 !=
				config.Identity.OnlineKeyHash {
			return nil, fmt.Errorf(
				"native signer certified trust floor differs from the client identity",
			)
		}
		certifiedTrustFloor = &certificate
	}

	requestTimeout := config.RequestTimeout
	if requestTimeout == 0 {
		requestTimeout = frostNativeSignerAnchorDefaultRequestTimeout
	}
	if requestTimeout <= 0 || requestTimeout > frostNativeSignerAnchorMaximumRequestTimeout {
		return nil, fmt.Errorf("native signer anchor request timeout is invalid")
	}
	clockSkew := config.MaximumClockSkew
	if clockSkew == 0 {
		clockSkew = frostNativeSignerAnchorDefaultClockSkew
	}
	if clockSkew < 0 || clockSkew > frostNativeSignerAnchorMaximumClockSkew {
		return nil, fmt.Errorf("native signer anchor maximum clock skew is invalid")
	}
	maximumAckLife := config.MaximumAcknowledgementLifetime
	if maximumAckLife == 0 {
		maximumAckLife = frostNativeSignerAnchorDefaultAcknowledgementLifetime
	}
	if maximumAckLife <= 0 ||
		maximumAckLife > frostNativeSignerAnchorMaximumAcknowledgementLifetime {
		return nil, fmt.Errorf("native signer anchor acknowledgement lifetime is invalid")
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
		expectedLeafSPKIHash := config.Identity.EndpointLeafSPKIHash
		var rootCAs *x509.CertPool
		if config.TLSRootCAs != nil {
			rootCAs = config.TLSRootCAs.Clone()
		}
		transport.TLSClientConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
			RootCAs:    rootCAs,
			VerifyConnection: func(state tls.ConnectionState) error {
				if len(state.VerifiedChains) == 0 || len(state.PeerCertificates) == 0 {
					return fmt.Errorf("native signer anchor TLS peer is not PKIX-verified")
				}
				if sha256.Sum256(state.PeerCertificates[0].RawSubjectPublicKeyInfo) !=
					expectedLeafSPKIHash {
					return fmt.Errorf("native signer anchor TLS leaf SPKI mismatch")
				}
				return nil
			},
		}
	} else if config.TLSRootCAs != nil {
		return nil, fmt.Errorf("loopback HTTP native signer anchor cannot configure TLS roots")
	}

	readEndpoint := frostNativeSignerAnchorOperationEndpoint(endpoint, "read")
	advanceEndpoint := frostNativeSignerAnchorOperationEndpoint(endpoint, "advance")
	historyEndpoint := frostNativeSignerAnchorOperationEndpoint(endpoint, "history")
	// Copy secret material only after every fallible validation/construction
	// step. Failed constructors therefore never leave an additional secret copy
	// waiting for garbage collection.
	clientKey := append(ed25519.PrivateKey{}, config.ClientPrivateKey...)
	return &FrostNativeSignerAnchorClient{
		readEndpoint:    readEndpoint,
		advanceEndpoint: advanceEndpoint,
		historyEndpoint: historyEndpoint,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   requestTimeout,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return fmt.Errorf("native signer anchor redirects are disabled")
			},
		},
		requestTimeout:      requestTimeout,
		clockSkew:           clockSkew,
		maximumAckLife:      maximumAckLife,
		identity:            config.Identity,
		bindingHash:         ComputeFrostNativeSignerAnchorBindingHash(config.Identity),
		clientKey:           clientKey,
		clientSPKIDER:       append([]byte{}, clientSPKIDER...),
		clientSPKIBase64:    base64StdEncoding(clientSPKIDER),
		onlineKey:           append(ed25519.PublicKey{}, onlineKey...),
		certifiedTrustFloor: certifiedTrustFloor,
		random:              randomSource,
		now:                 now,
	}, nil
}

// ReadFrostNativeSignerStateWitnessAnchor performs a fresh nonce-bound signed
// remote read. Callers must execute this read before every native operation;
// cached records are exposed only for monotonicity enforcement, never as a
// substitute for the remote read.
func (client *FrostNativeSignerAnchorClient) ReadFrostNativeSignerStateWitnessAnchor(
	ctx context.Context,
) (*FrostNativeSignerStateWitnessAnchorRecord, error) {
	if client == nil {
		return nil, fmt.Errorf("native signer anchor client is nil")
	}
	if ctx == nil {
		return nil, fmt.Errorf("native signer anchor context is nil")
	}
	client.mutex.Lock()
	defer client.mutex.Unlock()
	if client.poisoned != nil {
		return nil, fmt.Errorf("native signer anchor client is poisoned: %w", client.poisoned)
	}
	acknowledgement, err := client.readLocked(ctx)
	if err != nil {
		return nil, err
	}
	if err := client.acceptMonotonicAcknowledgementLocked(acknowledgement); err != nil {
		client.poisoned = err
		return nil, fmt.Errorf("native signer anchor client is poisoned: %w", err)
	}
	checkpoint := acknowledgement.Checkpoint
	client.readPermit = &checkpoint
	client.readPermitExpires = acknowledgement.ReadRecoveryExpiresAt
	return frostNativeSignerAnchorRecord(acknowledgement), nil
}

// CompareAndSwapFrostNativeSignerStateWitnessAnchor atomically advances the
// independent checkpoint. A transport-ambiguous CAS is reconciled by a fresh
// signed read: only the exact candidate plus operation/transition succeeds,
// and only the exact expected checkpoint permits one retry.
func (client *FrostNativeSignerAnchorClient) CompareAndSwapFrostNativeSignerStateWitnessAnchor(
	ctx context.Context,
	expected FrostNativeSignerStateWitnessCheckpoint,
	candidate FrostNativeSignerStateWitnessCheckpoint,
	proof []frostsigning.NativeTBTCSignerStateWitnessProofEntry,
) (*FrostNativeSignerStateWitnessAnchorCASResult, error) {
	if client == nil {
		return nil, fmt.Errorf("native signer anchor client is nil")
	}
	if ctx == nil {
		return nil, fmt.Errorf("native signer anchor context is nil")
	}
	if err := validateFrostNativeSignerAnchorTransition(
		expected,
		candidate,
		proof,
		client.identity.SignerStoreFingerprint,
	); err != nil {
		return nil, err
	}
	proofCopy := append([]frostsigning.NativeTBTCSignerStateWitnessProofEntry{}, proof...)

	client.mutex.Lock()
	defer client.mutex.Unlock()
	if client.poisoned != nil {
		return nil, fmt.Errorf("native signer anchor client is poisoned: %w", client.poisoned)
	}
	nowUnixMs := client.now().UnixMilli()
	if client.last == nil || client.last.Checkpoint != expected ||
		client.readPermit == nil || *client.readPermit != expected ||
		nowUnixMs < 0 || client.readPermitExpires <= uint64(nowUnixMs) {
		return nil, fmt.Errorf(
			"native signer anchor CAS requires a fresh authenticated exact-expected read",
		)
	}
	client.readPermit = nil
	client.readPermitExpires = 0
	operationID, err := client.randomBytes32()
	if err != nil {
		return nil, fmt.Errorf("cannot create native signer anchor operation ID: %w", err)
	}
	transitionDigest := computeFrostNativeSignerAnchorTransitionDigest(
		client.identity,
		operationID,
		expected,
		candidate,
		proofCopy,
	)

	acknowledgement, ambiguous, err := client.casAttemptLocked(
		ctx,
		expected,
		candidate,
		proofCopy,
		operationID,
		transitionDigest,
	)
	if err == nil {
		if err := client.acceptMonotonicAcknowledgementLocked(acknowledgement); err != nil {
			client.poisoned = err
			return nil, fmt.Errorf("native signer anchor client is poisoned: %w", err)
		}
		return &FrostNativeSignerStateWitnessAnchorCASResult{
			Acknowledgement: *acknowledgement,
		}, nil
	}
	if !ambiguous {
		return nil, err
	}

	recovered, retry, recoveryErr := client.reconcileAmbiguousCASLocked(
		ctx,
		expected,
		candidate,
		operationID,
		transitionDigest,
	)
	if recoveryErr != nil {
		client.poisoned = recoveryErr
		return nil, fmt.Errorf("native signer anchor CAS outcome is unsafe: %w", recoveryErr)
	}
	if recovered != nil {
		return &FrostNativeSignerStateWitnessAnchorCASResult{
			Acknowledgement: *recovered,
			Recovered:       true,
		}, nil
	}
	if !retry {
		return nil, fmt.Errorf("native signer anchor CAS failed without a safe retry state: %w", err)
	}

	acknowledgement, ambiguous, retryErr := client.casAttemptLocked(
		ctx,
		expected,
		candidate,
		proofCopy,
		operationID,
		transitionDigest,
	)
	if retryErr == nil {
		if err := client.acceptMonotonicAcknowledgementLocked(acknowledgement); err != nil {
			client.poisoned = err
			return nil, fmt.Errorf("native signer anchor client is poisoned: %w", err)
		}
		return &FrostNativeSignerStateWitnessAnchorCASResult{
			Acknowledgement: *acknowledgement,
			Recovered:       true,
		}, nil
	}
	if !ambiguous {
		return nil, retryErr
	}
	recovered, retry, recoveryErr = client.reconcileAmbiguousCASLocked(
		ctx,
		expected,
		candidate,
		operationID,
		transitionDigest,
	)
	if recoveryErr != nil {
		client.poisoned = recoveryErr
		return nil, fmt.Errorf("native signer anchor CAS retry outcome is unsafe: %w", recoveryErr)
	}
	if recovered != nil {
		return &FrostNativeSignerStateWitnessAnchorCASResult{
			Acknowledgement: *recovered,
			Recovered:       true,
		}, nil
	}
	if retry {
		return nil, fmt.Errorf(
			"native signer anchor CAS remained ambiguous but a fresh signed read retained the exact expected checkpoint",
		)
	}
	return nil, retryErr
}

func (client *FrostNativeSignerAnchorClient) reconcileAmbiguousCASLocked(
	ctx context.Context,
	expected FrostNativeSignerStateWitnessCheckpoint,
	candidate FrostNativeSignerStateWitnessCheckpoint,
	operationID [32]byte,
	transitionDigest [32]byte,
) (*FrostNativeSignerCheckpointAcknowledgement, bool, error) {
	acknowledgement, err := client.readLocked(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("cannot reconcile with a fresh authenticated read: %w", err)
	}
	switch acknowledgement.Checkpoint {
	case candidate:
		if acknowledgement.OperationID != operationID ||
			acknowledgement.TransitionDigest != transitionDigest {
			return nil, false, fmt.Errorf(
				"candidate checkpoint is bound to another operation or transition",
			)
		}
		if err := client.acceptMonotonicAcknowledgementLocked(acknowledgement); err != nil {
			return nil, false, err
		}
		return acknowledgement, false, nil
	case expected:
		if err := client.acceptMonotonicAcknowledgementLocked(acknowledgement); err != nil {
			return nil, false, err
		}
		return nil, true, nil
	default:
		return nil, false, fmt.Errorf(
			"authenticated checkpoint is neither the exact expected nor exact candidate state",
		)
	}
}

func (client *FrostNativeSignerAnchorClient) ReadFrostNativeSignerStateWitnessAnchorHistory(
	ctx context.Context,
	floor FrostNativeSignerStateWitnessAnchorReference,
) (*FrostNativeSignerStateWitnessAnchorHistory, error) {
	if client == nil {
		return nil, fmt.Errorf("native signer anchor client is nil")
	}
	if ctx == nil {
		return nil, fmt.Errorf("native signer anchor history context is nil")
	}
	if floor.Revision == ^uint64(0) {
		return nil, fmt.Errorf("native signer anchor history floor revision cannot advance")
	}

	client.mutex.Lock()
	defer client.mutex.Unlock()
	if client.poisoned != nil {
		return nil, fmt.Errorf("native signer anchor client is poisoned: %w", client.poisoned)
	}
	client.readPermit = nil
	client.readPermitExpires = 0
	firstTargetAcknowledgement, err := client.readLocked(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot read native signer anchor history target: %w", err)
	}
	target := frostNativeSignerAnchorReferenceFromAcknowledgement(
		firstTargetAcknowledgement,
	)
	if err := validateFrostNativeSignerAnchorHistoryBounds(
		floor,
		target,
		client.identity.SignerStoreFingerprint,
	); err != nil {
		return nil, err
	}
	if client.last != nil {
		lastReference := frostNativeSignerAnchorReferenceFromAcknowledgement(client.last)
		if lastReference != floor && lastReference != target {
			return nil, fmt.Errorf(
				"native signer anchor history floor differs from the authenticated process baseline",
			)
		}
		if lastReference == target &&
			!equalFrostNativeSignerCheckpointAcknowledgements(
				client.last,
				firstTargetAcknowledgement,
			) {
			return nil, fmt.Errorf(
				"native signer anchor target acknowledgement differs at an equal revision",
			)
		}
	}

	startRevision := floor.Revision + 1
	current := floor
	events := make([]FrostNativeSignerStateWitnessAnchorHistoryEvent, 0)
	totalProofEntries := 0
	complete := false
	for page := 0; page < FrostNativeSignerAnchorMaximumHistoryPages; page++ {
		remainingEvents := FrostNativeSignerAnchorMaximumHistoryEvents - len(events)
		remainingProof := FrostNativeSignerAnchorMaximumHistoryProofEntries -
			totalProofEntries
		if remainingEvents <= 0 || remainingProof <= 0 {
			return nil, fmt.Errorf("native signer anchor history exceeds aggregate bounds")
		}
		maximumEvents := remainingEvents
		if maximumEvents > FrostNativeSignerAnchorMaximumHistoryEventsPerPage {
			maximumEvents = FrostNativeSignerAnchorMaximumHistoryEventsPerPage
		}
		maximumProofEntries := remainingProof
		if maximumProofEntries >
			FrostNativeSignerAnchorMaximumHistoryProofEntriesPerPage {
			maximumProofEntries =
				FrostNativeSignerAnchorMaximumHistoryProofEntriesPerPage
		}
		pageEvents, pageCurrent, nextRevision, pageComplete, err :=
			client.readHistoryPageLocked(
				ctx,
				floor,
				target,
				current,
				startRevision,
				uint64(maximumEvents),
				uint64(maximumProofEntries),
			)
		if err != nil {
			return nil, err
		}
		for _, event := range pageEvents {
			totalProofEntries += len(event.WitnessProof)
		}
		events = append(events, pageEvents...)
		current = pageCurrent
		if pageComplete {
			complete = true
			break
		}
		startRevision = nextRevision
	}
	if !complete || current != target {
		return nil, fmt.Errorf("native signer anchor history did not reach the exact target")
	}

	finalAcknowledgement, err := client.readLocked(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot perform final native signer anchor read: %w", err)
	}
	if frostNativeSignerAnchorReferenceFromAcknowledgement(finalAcknowledgement) != target {
		return nil, fmt.Errorf("native signer anchor changed after history validation")
	}
	if !equalFrostNativeSignerCheckpointAcknowledgements(
		firstTargetAcknowledgement,
		finalAcknowledgement,
	) {
		return nil, fmt.Errorf("native signer anchor target acknowledgement changed during history validation")
	}
	copy := *finalAcknowledgement
	copy.ExactAcknowledgement = append(
		[]byte{},
		finalAcknowledgement.ExactAcknowledgement...,
	)
	copy.ExactReadRecovery = append(
		[]byte{},
		finalAcknowledgement.ExactReadRecovery...,
	)
	client.last = &copy
	checkpoint := finalAcknowledgement.Checkpoint
	client.readPermit = &checkpoint
	client.readPermitExpires = finalAcknowledgement.ReadRecoveryExpiresAt
	return &FrostNativeSignerStateWitnessAnchorHistory{
		Floor:     floor,
		Target:    target,
		Events:    events,
		FinalRead: frostNativeSignerAnchorRecord(finalAcknowledgement),
	}, nil
}

func (client *FrostNativeSignerAnchorClient) readHistoryPageLocked(
	ctx context.Context,
	floor FrostNativeSignerStateWitnessAnchorReference,
	target FrostNativeSignerStateWitnessAnchorReference,
	prior FrostNativeSignerStateWitnessAnchorReference,
	startRevision uint64,
	maximumEvents uint64,
	maximumProofEntries uint64,
) (
	[]FrostNativeSignerStateWitnessAnchorHistoryEvent,
	FrostNativeSignerStateWitnessAnchorReference,
	uint64,
	bool,
	error,
) {
	nonce, err := client.randomBytes32()
	if err != nil {
		return nil, prior, 0, false, fmt.Errorf(
			"cannot create native signer anchor history nonce: %w",
			err,
		)
	}
	transcript := frostNativeSignerAnchorHistoryRequestTranscript(
		client.identity,
		nonce,
		floor,
		target,
		startRevision,
		maximumEvents,
		maximumProofEntries,
		client.clientSPKIDER,
	)
	requestDigest := sha256.Sum256(transcript)
	request := frostNativeSignerAnchorHistoryRequest{
		Schema: FrostNativeSignerAnchorHistoryRequestSchema,
		Payload: frostNativeSignerAnchorHistoryRequestPayload{
			Kind:                "history",
			Nonce:               frostNativeSignerAnchorHex32(nonce),
			BindingHash:         frostNativeSignerAnchorHex32(client.bindingHash),
			Identity:            frostNativeSignerAnchorIdentityToWire(client.identity),
			FloorRef:            frostNativeSignerAnchorHistoryReferenceToWire(floor),
			TargetRef:           frostNativeSignerAnchorHistoryReferenceToWire(target),
			StartRevision:       strconv.FormatUint(startRevision, 10),
			MaximumEvents:       strconv.FormatUint(maximumEvents, 10),
			MaximumProofEntries: strconv.FormatUint(maximumProofEntries, 10),
		},
		ClientPublicKeySPKI: client.clientSPKIBase64,
		Signature: frostNativeSignerAnchorSignatureHex(
			ed25519.Sign(client.clientKey, transcript),
		),
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, prior, 0, false, err
	}
	responseBytes, _, err := client.postWithResponseLimit(
		ctx,
		client.historyEndpoint,
		payload,
		frostNativeSignerAnchorMaximumHistoryResponseBytes,
	)
	if err != nil {
		return nil, prior, 0, false, err
	}
	response := frostNativeSignerAnchorHistoryResponse{}
	if err := decodeStrictFrostNativeSignerAnchorJSON(
		responseBytes,
		&response,
	); err != nil {
		return nil, prior, 0, false, err
	}
	return client.validateHistoryPageLocked(
		response,
		requestDigest,
		nonce,
		floor,
		target,
		prior,
		startRevision,
		maximumEvents,
		maximumProofEntries,
	)
}

func (client *FrostNativeSignerAnchorClient) validateHistoryPageLocked(
	response frostNativeSignerAnchorHistoryResponse,
	requestDigest [32]byte,
	nonce [32]byte,
	floor FrostNativeSignerStateWitnessAnchorReference,
	target FrostNativeSignerStateWitnessAnchorReference,
	prior FrostNativeSignerStateWitnessAnchorReference,
	startRevision uint64,
	maximumEvents uint64,
	maximumProofEntries uint64,
) (
	[]FrostNativeSignerStateWitnessAnchorHistoryEvent,
	FrostNativeSignerStateWitnessAnchorReference,
	uint64,
	bool,
	error,
) {
	if response.Schema != FrostNativeSignerAnchorHistoryResponseSchema ||
		response.Events == nil {
		return nil, prior, 0, false, fmt.Errorf(
			"native signer anchor history response is incomplete",
		)
	}
	bindingHash, err := frostNativeSignerAnchorParseHex32(response.BindingHash)
	if err != nil || bindingHash != client.bindingHash {
		return nil, prior, 0, false, fmt.Errorf("history response binding hash mismatch")
	}
	responseRequestDigest, err := frostNativeSignerAnchorParseHex32(
		response.RequestDigest,
	)
	if err != nil || responseRequestDigest != requestDigest {
		return nil, prior, 0, false, fmt.Errorf("history response request digest mismatch")
	}
	responseNonce, err := frostNativeSignerAnchorParseHex32(response.Nonce)
	if err != nil || responseNonce != nonce {
		return nil, prior, 0, false, fmt.Errorf("history response nonce mismatch")
	}
	serviceEpoch, err := frostNativeSignerAnchorParseUint64(response.ServiceEpoch)
	if err != nil || serviceEpoch != target.ServiceEpoch {
		return nil, prior, 0, false, fmt.Errorf("history response service epoch mismatch")
	}
	responseFloor, err := frostNativeSignerAnchorHistoryReferenceFromWire(
		response.FloorRef,
	)
	if err != nil || responseFloor != floor {
		return nil, prior, 0, false, fmt.Errorf("history response floor mismatch")
	}
	responseTarget, err := frostNativeSignerAnchorHistoryReferenceFromWire(
		response.TargetRef,
	)
	if err != nil || responseTarget != target {
		return nil, prior, 0, false, fmt.Errorf("history response target mismatch")
	}
	responseStart, err := frostNativeSignerAnchorParseUint64(response.StartRevision)
	if err != nil || responseStart != startRevision ||
		startRevision != prior.Revision+1 {
		return nil, prior, 0, false, fmt.Errorf("history response start revision mismatch")
	}
	nextRevision, err := frostNativeSignerAnchorParseUint64(response.NextRevision)
	if err != nil {
		return nil, prior, 0, false, err
	}
	eventCount, err := frostNativeSignerAnchorParseUint64(response.EventCount)
	if err != nil || eventCount != uint64(len(*response.Events)) ||
		eventCount > maximumEvents ||
		eventCount > FrostNativeSignerAnchorMaximumHistoryEventsPerPage {
		return nil, prior, 0, false, fmt.Errorf("history response event count is invalid")
	}
	proofEntryCount, err := frostNativeSignerAnchorParseUint64(
		response.ProofEntryCount,
	)
	if err != nil || proofEntryCount > maximumProofEntries ||
		proofEntryCount > FrostNativeSignerAnchorMaximumHistoryProofEntriesPerPage {
		return nil, prior, 0, false, fmt.Errorf("history response proof count is invalid")
	}

	events := make([]FrostNativeSignerStateWitnessAnchorHistoryEvent, len(*response.Events))
	eventDigests := make([][32]byte, len(*response.Events))
	actualProofCount := uint64(0)
	for index, wireEvent := range *response.Events {
		proof, err := frostNativeSignerAnchorProofFromWire(wireEvent.WitnessProof)
		if err != nil {
			return nil, prior, 0, false, fmt.Errorf("invalid history witness proof: %w", err)
		}
		actualProofCount += uint64(len(proof))
		if actualProofCount > proofEntryCount {
			return nil, prior, 0, false, fmt.Errorf("history witness proof count exceeds its summary")
		}
		acknowledgement, err := client.verifyAcknowledgement(
			wireEvent.CheckpointAck,
			nil,
			nil,
			nil,
			nil,
			false,
			"applied",
			"already-applied",
		)
		if err != nil {
			return nil, prior, 0, false, fmt.Errorf(
				"invalid history checkpoint acknowledgement: %w",
				err,
			)
		}
		events[index] = FrostNativeSignerStateWitnessAnchorHistoryEvent{
			Acknowledgement: *acknowledgement,
			WitnessProof:    proof,
		}
		eventDigests[index] = computeFrostNativeSignerAnchorHistoryEventDigest(
			acknowledgement.Revision,
			acknowledgement.AcknowledgementDigest,
			wireEvent.CheckpointAck,
			proof,
		)
	}
	if actualProofCount != proofEntryCount {
		return nil, prior, 0, false, fmt.Errorf("history witness proof count mismatch")
	}
	responseDigest, err := frostNativeSignerAnchorHistoryResponseTranscript(
		response,
		eventDigests,
	)
	if err != nil {
		return nil, prior, 0, false, err
	}
	responseSignature, err := frostNativeSignerAnchorParseSignature(response.Signature)
	if err != nil || !ed25519.Verify(client.onlineKey, responseDigest, responseSignature[:]) {
		return nil, prior, 0, false, fmt.Errorf("history response signature is invalid")
	}
	committedAt, err := frostNativeSignerAnchorParseUint64(response.CommittedAtUnixMs)
	if err != nil {
		return nil, prior, 0, false, err
	}
	expiresAt, err := frostNativeSignerAnchorParseUint64(response.ExpiresAtUnixMs)
	if err != nil {
		return nil, prior, 0, false, err
	}
	nowUnixMs := client.now().UnixMilli()
	if nowUnixMs < 0 || committedAt == 0 || expiresAt <= committedAt ||
		expiresAt-committedAt > uint64(client.maximumAckLife/time.Millisecond) ||
		committedAt > uint64(nowUnixMs)+uint64(client.clockSkew/time.Millisecond) ||
		expiresAt <= uint64(nowUnixMs) {
		return nil, prior, 0, false, fmt.Errorf(
			"history response is stale or has an invalid lifetime",
		)
	}

	current := prior
	for index := range events {
		event := &events[index]
		acknowledgement := &event.Acknowledgement
		if acknowledgement.ServiceEpoch != target.ServiceEpoch ||
			acknowledgement.Revision != current.Revision+1 ||
			acknowledgement.PreviousEventRoot != current.EventRoot {
			return nil, prior, 0, false, fmt.Errorf(
				"history acknowledgement event chain is discontinuous",
			)
		}
		if err := validateFrostNativeSignerAnchorTransition(
			current.Checkpoint,
			acknowledgement.Checkpoint,
			event.WitnessProof,
			client.identity.SignerStoreFingerprint,
		); err != nil {
			return nil, prior, 0, false, fmt.Errorf(
				"invalid history native state transition: %w",
				err,
			)
		}
		expectedTransitionDigest := computeFrostNativeSignerAnchorTransitionDigest(
			client.identity,
			acknowledgement.OperationID,
			current.Checkpoint,
			acknowledgement.Checkpoint,
			event.WitnessProof,
		)
		if acknowledgement.TransitionDigest != expectedTransitionDigest {
			return nil, prior, 0, false, fmt.Errorf(
				"history checkpoint transition digest mismatch",
			)
		}
		current = frostNativeSignerAnchorReferenceFromAcknowledgement(acknowledgement)
	}

	switch response.Status {
	case "partial":
		if len(events) == 0 || nextRevision != current.Revision+1 ||
			nextRevision > target.Revision || current == target {
			return nil, prior, 0, false, fmt.Errorf("invalid partial history page")
		}
		return events, current, nextRevision, false, nil
	case "complete":
		if nextRevision != 0 || current != target ||
			(len(events) == 0 && prior != target) {
			return nil, prior, 0, false, fmt.Errorf("invalid complete history page")
		}
		return events, current, 0, true, nil
	default:
		return nil, prior, 0, false, fmt.Errorf("unsupported history response status")
	}
}

func frostNativeSignerAnchorReferenceFromAcknowledgement(
	acknowledgement *FrostNativeSignerCheckpointAcknowledgement,
) FrostNativeSignerStateWitnessAnchorReference {
	if acknowledgement == nil {
		return FrostNativeSignerStateWitnessAnchorReference{}
	}
	return FrostNativeSignerStateWitnessAnchorReference{
		ServiceEpoch:          acknowledgement.ServiceEpoch,
		Revision:              acknowledgement.Revision,
		EventRoot:             acknowledgement.EventRoot,
		AcknowledgementDigest: acknowledgement.AcknowledgementDigest,
		Checkpoint:            acknowledgement.Checkpoint,
	}
}

// readLocked performs one authenticated anchor read, retrying a small bounded
// number of times when the service could not be reached at all.
//
// Every native signer operation is preceded by one of these reads, and its
// failure is expensive out of proportion to its cause: the state-anchor barrier
// treats an unauthenticated anchor as a reason to refuse the operation, so a
// single connection reset during an anchor-service redeploy takes down a
// signing round that nothing was actually wrong with. A read is idempotent -
// it is a fresh nonce-bound signed request that mutates nothing here and
// nothing at the service - so the blip can simply be ridden out.
//
// The whole loop runs inside ONE requestTimeout budget, so this never lengthens
// how long a caller can wait: total wall time is exactly what a single attempt
// could already take. That is also why the retries are worth having, because
// the failures they cover - refused connection, reset connection, unresolvable
// name, 503 from a restarting service - come back in milliseconds and leave
// nearly the whole budget for another try, while a genuine hang consumes the
// budget on the first attempt and correctly gets no second one.
//
// Only a failure to reach the service is retried. Anything the service actually
// answered - a bad signature, a nonce or digest mismatch, an absent stream, a
// non-monotonic acknowledgement - is a fact about the anchor, is deterministic,
// and must surface on the first attempt rather than be papered over.
func (client *FrostNativeSignerAnchorClient) readLocked(
	ctx context.Context,
) (*FrostNativeSignerCheckpointAcknowledgement, error) {
	budgetContext, cancel := context.WithTimeout(ctx, client.requestTimeout)
	defer cancel()
	backoff := frostNativeSignerAnchorReadRetryBackoff
	for attempt := 1; ; attempt++ {
		acknowledgement, err := client.readOnceLocked(budgetContext)
		if err == nil {
			return acknowledgement, nil
		}
		if attempt >= frostNativeSignerAnchorReadAttempts ||
			budgetContext.Err() != nil ||
			!isFrostNativeSignerAnchorRetryableReadFailure(err) {
			return nil, err
		}
		backoffTimer := time.NewTimer(backoff)
		select {
		case <-budgetContext.Done():
			backoffTimer.Stop()
			return nil, err
		case <-backoffTimer.C:
		}
		backoff *= 2
	}
}

// isFrostNativeSignerAnchorRetryableReadFailure reports whether a failed
// authenticated read may be retried within its budget.
//
// The transport cases are exactly isFrostPreSignTransientAuthorizationFailure's
// - the anchor was not reached, so nothing was learned about it. The status
// cases are 408, 429, and 5xx responses an HTTP intermediary or a restarting
// service produces while it is unable to answer at all; every other status is
// the service answering something wrong and stays fatal on the first attempt.
func isFrostNativeSignerAnchorRetryableReadFailure(err error) bool {
	return isFrostPreSignTransientAuthorizationFailure(err)
}

func (client *FrostNativeSignerAnchorClient) readOnceLocked(
	ctx context.Context,
) (*FrostNativeSignerCheckpointAcknowledgement, error) {
	nonce, err := client.randomBytes32()
	if err != nil {
		return nil, fmt.Errorf("cannot create native signer anchor read nonce: %w", err)
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
		Signature:           frostNativeSignerAnchorSignatureHex(ed25519.Sign(client.clientKey, transcript)),
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("cannot encode native signer anchor read: %w", err)
	}
	response, _, err := client.post(ctx, client.readEndpoint, payload)
	if err != nil {
		return nil, err
	}
	readResponse := frostNativeSignerAnchorReadResponse{}
	if err := decodeStrictFrostNativeSignerAnchorJSON(response, &readResponse); err != nil {
		return nil, fmt.Errorf("invalid native signer anchor read response: %w", err)
	}
	if readResponse.Schema != FrostNativeSignerAnchorReadResponseSchema {
		return nil, fmt.Errorf("unsupported native signer anchor read response schema")
	}
	responseDigest, err := frostNativeSignerAnchorReadResponseTranscript(readResponse)
	if err != nil {
		return nil, fmt.Errorf("invalid native signer anchor read response: %w", err)
	}
	responseSignature, err := frostNativeSignerAnchorParseSignature(readResponse.Signature)
	if err != nil || !ed25519.Verify(client.onlineKey, responseDigest, responseSignature[:]) {
		return nil, fmt.Errorf("native signer anchor read response signature is invalid")
	}
	responseBindingHash, err := frostNativeSignerAnchorParseHex32(readResponse.BindingHash)
	if err != nil || responseBindingHash != client.bindingHash {
		return nil, fmt.Errorf("native signer anchor read binding hash mismatch")
	}
	responseRequestDigest, err := frostNativeSignerAnchorParseHex32(readResponse.RequestDigest)
	if err != nil || responseRequestDigest != requestDigest {
		return nil, fmt.Errorf("native signer anchor read request digest mismatch")
	}
	responseNonce, err := frostNativeSignerAnchorParseHex32(readResponse.Nonce)
	if err != nil || responseNonce != nonce {
		return nil, fmt.Errorf("native signer anchor read nonce mismatch")
	}
	if readResponse.Status != "present" {
		return nil, fmt.Errorf("native signer anchor stream is absent")
	}
	if readResponse.Checkpoint == nil {
		return nil, fmt.Errorf("native signer anchor read checkpoint is absent")
	}
	checkpoint, err := frostNativeSignerAnchorCheckpointFromWire(*readResponse.Checkpoint)
	if err != nil {
		return nil, err
	}
	operationID, err := frostNativeSignerAnchorParseHex32(readResponse.OperationID)
	if err != nil {
		return nil, err
	}
	transitionDigest, err := frostNativeSignerAnchorParseHex32(readResponse.TransitionDigest)
	if err != nil {
		return nil, err
	}
	var acknowledgement *FrostNativeSignerCheckpointAcknowledgement
	if client.certifiedTrustFloor != nil &&
		bytes.Equal(
			readResponse.CheckpointAck,
			client.certifiedTrustFloor.TargetAcknowledgement,
		) {
		acknowledgement, err =
			verifyFrostNativeSignerAnchorTrustTargetAcknowledgement(
				client.certifiedTrustFloor,
				readResponse.CheckpointAck,
			)
		if err == nil &&
			(acknowledgement.Checkpoint != checkpoint ||
				acknowledgement.OperationID != operationID) {
			err = fmt.Errorf(
				"certified trust-floor acknowledgement differs from its Read summary",
			)
		}
	} else {
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
	}
	if err != nil {
		return nil, fmt.Errorf("invalid stored native signer checkpoint acknowledgement: %w", err)
	}
	if acknowledgement.TransitionDigest != transitionDigest {
		return nil, fmt.Errorf("native signer anchor read transition digest mismatch")
	}
	serviceEpoch, err := frostNativeSignerAnchorParseUint64(readResponse.ServiceEpoch)
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
	committedAt, err := frostNativeSignerAnchorParseUint64(readResponse.CommittedAtUnixMs)
	if err != nil {
		return nil, err
	}
	expiresAt, err := frostNativeSignerAnchorParseUint64(readResponse.ExpiresAtUnixMs)
	if err != nil {
		return nil, err
	}
	nowUnixMs := client.now().UnixMilli()
	if nowUnixMs < 0 || committedAt == 0 || expiresAt <= committedAt ||
		expiresAt-committedAt > uint64(client.maximumAckLife/time.Millisecond) ||
		committedAt > uint64(nowUnixMs)+uint64(client.clockSkew/time.Millisecond) ||
		expiresAt <= uint64(nowUnixMs) {
		return nil, fmt.Errorf("native signer anchor read response is stale or has an invalid lifetime")
	}
	acknowledgementDigest, err := frostNativeSignerAnchorParseHex32(
		readResponse.CheckpointAckDigest,
	)
	if err != nil ||
		serviceEpoch != acknowledgement.ServiceEpoch ||
		revision != acknowledgement.Revision ||
		eventRoot != acknowledgement.EventRoot ||
		acknowledgementDigest != acknowledgement.AcknowledgementDigest {
		return nil, fmt.Errorf("native signer anchor read summary differs from its stored acknowledgement")
	}
	acknowledgement.ExactReadRecovery = append([]byte{}, response...)
	acknowledgement.ReadRecoveryExpiresAt = expiresAt
	return acknowledgement, nil
}

func (client *FrostNativeSignerAnchorClient) casAttemptLocked(
	ctx context.Context,
	expected FrostNativeSignerStateWitnessCheckpoint,
	candidate FrostNativeSignerStateWitnessCheckpoint,
	proof []frostsigning.NativeTBTCSignerStateWitnessProofEntry,
	operationID [32]byte,
	transitionDigest [32]byte,
) (*FrostNativeSignerCheckpointAcknowledgement, bool, error) {
	nonce, err := client.randomBytes32()
	if err != nil {
		return nil, false, fmt.Errorf("cannot create native signer anchor CAS nonce: %w", err)
	}
	transcript := frostNativeSignerAnchorCASRequestTranscript(
		client.identity,
		nonce,
		operationID,
		transitionDigest,
		expected,
		candidate,
		proof,
		client.clientSPKIDER,
	)
	requestDigest := sha256.Sum256(transcript)
	request := frostNativeSignerAnchorCASRequest{
		Schema: FrostNativeSignerAnchorCASRequestSchema,
		Payload: frostNativeSignerAnchorCASRequestPayload{
			Kind:             "advance",
			Nonce:            frostNativeSignerAnchorHex32(nonce),
			BindingHash:      frostNativeSignerAnchorHex32(client.bindingHash),
			Identity:         frostNativeSignerAnchorIdentityToWire(client.identity),
			OperationID:      frostNativeSignerAnchorHex32(operationID),
			TransitionDigest: frostNativeSignerAnchorHex32(transitionDigest),
			Expected:         frostNativeSignerAnchorCheckpointToWire(expected),
			Candidate:        frostNativeSignerAnchorCheckpointToWire(candidate),
			Proof:            frostNativeSignerAnchorProofToWire(proof),
		},
		ClientPublicKeySPKI: client.clientSPKIBase64,
		Signature:           frostNativeSignerAnchorSignatureHex(ed25519.Sign(client.clientKey, transcript)),
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, false, fmt.Errorf("cannot encode native signer anchor CAS: %w", err)
	}
	if len(payload) > frostNativeSignerAnchorMaximumRequestBytes {
		return nil, false, fmt.Errorf("native signer anchor CAS request exceeds the size bound")
	}
	response, sent, err := client.post(ctx, client.advanceEndpoint, payload)
	if err != nil {
		return nil, sent, err
	}
	acknowledgement, err := client.verifyAcknowledgement(
		response,
		&requestDigest,
		&nonce,
		&candidate,
		&operationID,
		true,
		"applied",
		"already-applied",
	)
	if err != nil {
		// The server may have committed before returning a malformed, truncated,
		// or stale response, so any post-send verification error is ambiguous.
		return nil, true, fmt.Errorf("invalid native signer anchor CAS acknowledgement: %w", err)
	}
	if acknowledgement.TransitionDigest != transitionDigest {
		return nil, true, fmt.Errorf("native signer anchor CAS transition digest mismatch")
	}
	return acknowledgement, false, nil
}

func (client *FrostNativeSignerAnchorClient) verifyAcknowledgement(
	payload []byte,
	requestDigest *[32]byte,
	nonce *[32]byte,
	expectedCheckpoint *FrostNativeSignerStateWitnessCheckpoint,
	expectedOperationID *[32]byte,
	requireFresh bool,
	allowedStatuses ...string,
) (*FrostNativeSignerCheckpointAcknowledgement, error) {
	wire := frostNativeSignerAnchorAcknowledgementWire{}
	if err := decodeStrictFrostNativeSignerAnchorJSON(payload, &wire); err != nil {
		return nil, err
	}
	if wire.Schema != FrostNativeSignerCheckpointAcknowledgementSchema {
		return nil, fmt.Errorf("unsupported checkpoint acknowledgement schema")
	}
	statusAllowed := false
	for _, status := range allowedStatuses {
		statusAllowed = statusAllowed || wire.Status == status
	}
	if !statusAllowed {
		return nil, fmt.Errorf("checkpoint acknowledgement status is invalid")
	}
	transcript, err := frostNativeSignerAnchorAcknowledgementTranscript(wire)
	if err != nil {
		return nil, err
	}
	signature, err := frostNativeSignerAnchorParseSignature(wire.Signature)
	if err != nil {
		return nil, err
	}
	if !ed25519.Verify(client.onlineKey, transcript, signature[:]) {
		return nil, fmt.Errorf("checkpoint acknowledgement signature is invalid")
	}
	acknowledgement, err := frostNativeSignerAnchorAcknowledgementFromWire(wire)
	if err != nil {
		return nil, err
	}
	if acknowledgement.BindingHash != client.bindingHash ||
		(requestDigest != nil && acknowledgement.RequestDigest != *requestDigest) ||
		(nonce != nil && acknowledgement.Nonce != *nonce) {
		return nil, fmt.Errorf("checkpoint acknowledgement request binding mismatch")
	}
	if expectedCheckpoint != nil && acknowledgement.Checkpoint != *expectedCheckpoint {
		return nil, fmt.Errorf("checkpoint acknowledgement does not contain the exact candidate")
	}
	if expectedOperationID != nil && acknowledgement.OperationID != *expectedOperationID {
		return nil, fmt.Errorf("checkpoint acknowledgement operation ID mismatch")
	}
	if err := validateFrostNativeSignerAnchorCheckpoint(
		acknowledgement.Checkpoint,
		client.identity.SignerStoreFingerprint,
	); err != nil {
		return nil, err
	}
	if acknowledgement.OperationID == [32]byte{} ||
		acknowledgement.TransitionDigest == [32]byte{} ||
		acknowledgement.ServiceEpoch == 0 ||
		acknowledgement.Revision == 0 ||
		acknowledgement.EventRoot == [32]byte{} ||
		(acknowledgement.Revision == 1 &&
			acknowledgement.PreviousEventRoot != [32]byte{}) ||
		(acknowledgement.Revision > 1 &&
			acknowledgement.PreviousEventRoot == [32]byte{}) {
		return nil, fmt.Errorf("checkpoint acknowledgement audit identity is invalid")
	}
	if computeFrostNativeSignerAnchorEventRoot(*acknowledgement) != acknowledgement.EventRoot {
		return nil, fmt.Errorf("checkpoint acknowledgement event root mismatch")
	}
	nowUnixMs := client.now().UnixMilli()
	if nowUnixMs < 0 ||
		acknowledgement.CommittedAtUnixMs == 0 ||
		acknowledgement.ExpiresAtUnixMs <= acknowledgement.CommittedAtUnixMs ||
		acknowledgement.ExpiresAtUnixMs-acknowledgement.CommittedAtUnixMs >
			uint64(client.maximumAckLife/time.Millisecond) ||
		acknowledgement.CommittedAtUnixMs >
			uint64(nowUnixMs)+uint64(client.clockSkew/time.Millisecond) ||
		(requireFresh && acknowledgement.ExpiresAtUnixMs <= uint64(nowUnixMs)) {
		return nil, fmt.Errorf("checkpoint acknowledgement is stale or has an invalid lifetime")
	}
	acknowledgement.Signature = signature
	copy(acknowledgement.SigningDigest[:], transcript)
	acknowledgement.AcknowledgementDigest =
		computeFrostNativeSignerCheckpointAcknowledgementDigest(
			acknowledgement.SigningDigest,
			signature,
			client.identity.OnlineKeyHash,
		)
	acknowledgement.ExactAcknowledgement = append([]byte{}, payload...)
	return acknowledgement, nil
}

func frostNativeSignerAnchorAcknowledgementFromWire(
	wire frostNativeSignerAnchorAcknowledgementWire,
) (*FrostNativeSignerCheckpointAcknowledgement, error) {
	result := &FrostNativeSignerCheckpointAcknowledgement{Status: wire.Status}
	var err error
	if result.BindingHash, err = frostNativeSignerAnchorParseHex32(wire.BindingHash); err != nil {
		return nil, err
	}
	if result.RequestDigest, err = frostNativeSignerAnchorParseHex32(wire.RequestDigest); err != nil {
		return nil, err
	}
	if result.Nonce, err = frostNativeSignerAnchorParseHex32(wire.Nonce); err != nil {
		return nil, err
	}
	if result.ServiceEpoch, err = frostNativeSignerAnchorParseUint64(wire.ServiceEpoch); err != nil {
		return nil, err
	}
	if result.Revision, err = frostNativeSignerAnchorParseUint64(wire.Revision); err != nil {
		return nil, err
	}
	if result.PreviousEventRoot, err = frostNativeSignerAnchorParseHex32(wire.PreviousEventRoot); err != nil {
		return nil, err
	}
	if result.EventRoot, err = frostNativeSignerAnchorParseHex32(wire.EventRoot); err != nil {
		return nil, err
	}
	if result.Checkpoint, err = frostNativeSignerAnchorCheckpointFromWire(wire.Checkpoint); err != nil {
		return nil, err
	}
	if result.OperationID, err = frostNativeSignerAnchorParseHex32(wire.OperationID); err != nil {
		return nil, err
	}
	if result.TransitionDigest, err = frostNativeSignerAnchorParseHex32(wire.TransitionDigest); err != nil {
		return nil, err
	}
	if result.CommittedAtUnixMs, err = frostNativeSignerAnchorParseUint64(wire.CommittedAtUnixMs); err != nil {
		return nil, err
	}
	if result.ExpiresAtUnixMs, err = frostNativeSignerAnchorParseUint64(wire.ExpiresAtUnixMs); err != nil {
		return nil, err
	}
	return result, nil
}

func computeFrostNativeSignerCheckpointAcknowledgementDigest(
	signingDigest [32]byte,
	signature [ed25519.SignatureSize]byte,
	onlineKeyHash [32]byte,
) [32]byte {
	hasher := sha256.New()
	hasher.Write([]byte("tbtc-signer-state-anchor-acknowledgement/v1\x00"))
	hasher.Write(signingDigest[:])
	hasher.Write(signature[:])
	hasher.Write(onlineKeyHash[:])
	var result [32]byte
	copy(result[:], hasher.Sum(nil))
	return result
}

func (client *FrostNativeSignerAnchorClient) acceptMonotonicAcknowledgementLocked(
	acknowledgement *FrostNativeSignerCheckpointAcknowledgement,
) error {
	if acknowledgement == nil {
		return fmt.Errorf("checkpoint acknowledgement is nil")
	}
	if client.last != nil {
		if acknowledgement.ServiceEpoch != client.last.ServiceEpoch {
			return fmt.Errorf("checkpoint acknowledgement service epoch changed")
		}
		if acknowledgement.ServiceEpoch == client.last.ServiceEpoch {
			switch {
			case acknowledgement.Revision < client.last.Revision:
				return fmt.Errorf("checkpoint acknowledgement revision rolled back")
			case acknowledgement.Revision == client.last.Revision:
				if !equalFrostNativeSignerCheckpointAcknowledgements(
					acknowledgement,
					client.last,
				) {
					return fmt.Errorf("equal checkpoint acknowledgement revisions differ")
				}
				return nil
			case acknowledgement.Revision != client.last.Revision+1 ||
				acknowledgement.PreviousEventRoot != client.last.EventRoot:
				return fmt.Errorf("checkpoint acknowledgement event chain is discontinuous")
			}
		}
		if acknowledgement.Checkpoint.Generation < client.last.Checkpoint.Generation {
			return fmt.Errorf("checkpoint acknowledgement generation rolled back")
		}
		if acknowledgement.Checkpoint.Generation == client.last.Checkpoint.Generation &&
			acknowledgement.Checkpoint != client.last.Checkpoint {
			return fmt.Errorf("equal checkpoint generations have different state")
		}
	}
	copy := *acknowledgement
	copy.ExactAcknowledgement = append([]byte{}, acknowledgement.ExactAcknowledgement...)
	copy.ExactReadRecovery = append([]byte{}, acknowledgement.ExactReadRecovery...)
	client.last = &copy
	return nil
}

func frostNativeSignerAnchorRecord(
	acknowledgement *FrostNativeSignerCheckpointAcknowledgement,
) *FrostNativeSignerStateWitnessAnchorRecord {
	return &FrostNativeSignerStateWitnessAnchorRecord{
		Checkpoint:             acknowledgement.Checkpoint,
		BindingHash:            acknowledgement.BindingHash,
		AcknowledgementDigest:  acknowledgement.AcknowledgementDigest,
		OperationID:            acknowledgement.OperationID,
		TransitionDigest:       acknowledgement.TransitionDigest,
		ServiceEpoch:           acknowledgement.ServiceEpoch,
		Revision:               acknowledgement.Revision,
		PreviousEventRoot:      acknowledgement.PreviousEventRoot,
		EventRoot:              acknowledgement.EventRoot,
		AcknowledgementJSON:    append([]byte{}, acknowledgement.ExactAcknowledgement...),
		AcknowledgementExpires: acknowledgement.ExpiresAtUnixMs,
		ReadRecoveryJSON:       append([]byte{}, acknowledgement.ExactReadRecovery...),
		ReadRecoveryExpires:    acknowledgement.ReadRecoveryExpiresAt,
	}
}

func equalFrostNativeSignerCheckpointAcknowledgements(
	left *FrostNativeSignerCheckpointAcknowledgement,
	right *FrostNativeSignerCheckpointAcknowledgement,
) bool {
	if left == nil || right == nil {
		return left == right
	}
	return left.BindingHash == right.BindingHash &&
		left.RequestDigest == right.RequestDigest &&
		left.Nonce == right.Nonce &&
		left.Status == right.Status &&
		left.ServiceEpoch == right.ServiceEpoch &&
		left.Revision == right.Revision &&
		left.PreviousEventRoot == right.PreviousEventRoot &&
		left.EventRoot == right.EventRoot &&
		left.Checkpoint == right.Checkpoint &&
		left.OperationID == right.OperationID &&
		left.TransitionDigest == right.TransitionDigest &&
		left.CommittedAtUnixMs == right.CommittedAtUnixMs &&
		left.ExpiresAtUnixMs == right.ExpiresAtUnixMs &&
		left.Signature == right.Signature &&
		left.SigningDigest == right.SigningDigest &&
		left.AcknowledgementDigest == right.AcknowledgementDigest &&
		bytes.Equal(left.ExactAcknowledgement, right.ExactAcknowledgement)
}

func computeFrostNativeSignerAnchorEventRoot(
	acknowledgement FrostNativeSignerCheckpointAcknowledgement,
) [32]byte {
	status := byte(0)
	switch acknowledgement.Status {
	case "applied":
		status = 0x01
	case "already-applied":
		status = 0x02
	default:
		return [32]byte{}
	}
	buffer := bytes.NewBuffer(nil)
	buffer.WriteString("tbtc-native-signer-state-anchor-event/v1\x00")
	buffer.Write(acknowledgement.BindingHash[:])
	_ = binary.Write(buffer, binary.BigEndian, acknowledgement.ServiceEpoch)
	_ = binary.Write(buffer, binary.BigEndian, acknowledgement.Revision)
	buffer.Write(acknowledgement.PreviousEventRoot[:])
	buffer.Write(acknowledgement.RequestDigest[:])
	buffer.Write(acknowledgement.Nonce[:])
	buffer.WriteByte(status)
	buffer.Write(acknowledgement.Checkpoint.StoreFingerprint[:])
	_ = binary.Write(buffer, binary.BigEndian, acknowledgement.Checkpoint.Generation)
	buffer.Write(acknowledgement.Checkpoint.PreviousStateCommitment[:])
	buffer.Write(acknowledgement.Checkpoint.StateImageDigest[:])
	buffer.Write(acknowledgement.Checkpoint.StateCommitment[:])
	buffer.Write(acknowledgement.OperationID[:])
	buffer.Write(acknowledgement.TransitionDigest[:])
	_ = binary.Write(buffer, binary.BigEndian, acknowledgement.CommittedAtUnixMs)
	_ = binary.Write(buffer, binary.BigEndian, acknowledgement.ExpiresAtUnixMs)
	return sha256.Sum256(buffer.Bytes())
}

func (client *FrostNativeSignerAnchorClient) post(
	ctx context.Context,
	endpoint string,
	payload []byte,
) ([]byte, bool, error) {
	return client.postWithResponseLimit(
		ctx,
		endpoint,
		payload,
		frostNativeSignerAnchorMaximumResponseBytes,
	)
}

func (client *FrostNativeSignerAnchorClient) postWithResponseLimit(
	ctx context.Context,
	endpoint string,
	payload []byte,
	responseLimit int64,
) ([]byte, bool, error) {
	if len(payload) == 0 || len(payload) > frostNativeSignerAnchorMaximumRequestBytes {
		return nil, false, fmt.Errorf("native signer anchor request size is invalid")
	}
	if responseLimit <= 0 ||
		responseLimit > frostNativeSignerAnchorMaximumHistoryResponseBytes {
		return nil, false, fmt.Errorf("native signer anchor response limit is invalid")
	}
	requestContext, cancel := context.WithTimeout(ctx, client.requestTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(
		requestContext,
		http.MethodPost,
		endpoint,
		bytes.NewReader(payload),
	)
	if err != nil {
		return nil, false, fmt.Errorf("cannot construct native signer anchor request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Cache-Control", "no-store")
	var wroteRequest atomic.Bool
	request = request.WithContext(httptrace.WithClientTrace(
		request.Context(),
		&httptrace.ClientTrace{
			WroteRequest: func(httptrace.WroteRequestInfo) {
				wroteRequest.Store(true)
			},
		},
	))
	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, wroteRequest.Load(), fmt.Errorf(
			"native signer anchor request failed: %w",
			err,
		)
	}
	defer response.Body.Close()
	limited := io.LimitReader(response.Body, responseLimit+1)
	body, readErr := io.ReadAll(limited)
	if readErr != nil {
		return nil, true, fmt.Errorf("cannot read native signer anchor response: %w", readErr)
	}
	if len(body) == 0 || int64(len(body)) > responseLimit {
		return nil, true, fmt.Errorf("native signer anchor response size is invalid")
	}
	if response.StatusCode != http.StatusOK {
		return nil, true, &frostNativeSignerAnchorStatusError{
			statusCode: response.StatusCode,
		}
	}
	mediaType, parameters, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" || len(parameters) != 0 {
		return nil, true, fmt.Errorf("native signer anchor response content type is invalid")
	}
	return body, true, nil
}

func (client *FrostNativeSignerAnchorClient) randomBytes32() ([32]byte, error) {
	var result [32]byte
	if _, err := io.ReadFull(client.random, result[:]); err != nil {
		return [32]byte{}, err
	}
	if result == [32]byte{} {
		return [32]byte{}, fmt.Errorf("random value is zero")
	}
	return result, nil
}

func parseFrostNativeSignerAnchorOnlineKey(der []byte) (ed25519.PublicKey, error) {
	if len(der) == 0 {
		return nil, fmt.Errorf("native signer anchor online key is empty")
	}
	parsed, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return nil, fmt.Errorf("cannot parse native signer anchor online key: %w", err)
	}
	publicKey, ok := parsed.(ed25519.PublicKey)
	if !ok || len(publicKey) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("native signer anchor online key is not Ed25519")
	}
	if err := ValidateFrostNativeSignerAnchorTrustEd25519PublicKey(
		publicKey,
	); err != nil {
		return nil, fmt.Errorf(
			"native signer anchor online key point is invalid: %w",
			err,
		)
	}
	canonical, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil || !bytes.Equal(canonical, der) {
		return nil, fmt.Errorf("native signer anchor online key SPKI is not canonical DER")
	}
	return append(ed25519.PublicKey{}, publicKey...), nil
}

func validateFrostNativeSignerAnchorEndpoint(
	value string,
) (*url.URL, bool, error) {
	if value == "" || len(value) > 2048 || strings.Contains(value, "%") {
		return nil, false, fmt.Errorf("native signer anchor endpoint is invalid")
	}
	endpoint, err := url.Parse(value)
	if err != nil || endpoint.String() != value ||
		(endpoint.Scheme != "https" && endpoint.Scheme != "http") ||
		endpoint.User != nil || endpoint.Opaque != "" ||
		endpoint.RawQuery != "" || endpoint.Fragment != "" ||
		endpoint.RawPath != "" || endpoint.Host == "" ||
		endpoint.Path == "" || path.Clean(endpoint.Path) != endpoint.Path ||
		(endpoint.Path != "/" && strings.HasSuffix(endpoint.Path, "/")) {
		return nil, false, fmt.Errorf("native signer anchor endpoint is not canonical")
	}
	hostname := endpoint.Hostname()
	if hostname == "" || hostname != strings.ToLower(hostname) {
		return nil, false, fmt.Errorf("native signer anchor endpoint host is not canonical")
	}
	ip := net.ParseIP(hostname)
	if ip != nil && ip.String() != hostname {
		return nil, false, fmt.Errorf("native signer anchor endpoint IP is not canonical")
	}
	if ip == nil && !frostNativeSignerAnchorCanonicalDNSName(hostname) {
		return nil, false, fmt.Errorf("native signer anchor endpoint DNS name is invalid")
	}
	if endpoint.Port() != "" {
		port, err := strconv.ParseUint(endpoint.Port(), 10, 16)
		if err != nil || port == 0 || strconv.FormatUint(port, 10) != endpoint.Port() {
			return nil, false, fmt.Errorf("native signer anchor endpoint port is invalid")
		}
	}
	if endpoint.Scheme == "http" {
		if ip == nil || !ip.IsLoopback() || endpoint.Port() == "" {
			return nil, false, fmt.Errorf(
				"native signer anchor HTTP endpoint must be numeric loopback with a fixed port",
			)
		}
		return endpoint, false, nil
	}
	return endpoint, true, nil
}

func frostNativeSignerAnchorCanonicalDNSName(hostname string) bool {
	if len(hostname) == 0 || len(hostname) > 253 || strings.HasSuffix(hostname, ".") {
		return false
	}
	for _, label := range strings.Split(hostname, ".") {
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

func frostNativeSignerAnchorOperationEndpoint(endpoint *url.URL, operation string) string {
	copy := *endpoint
	if copy.Path == "/" {
		copy.Path += operation
	} else {
		copy.Path += "/" + operation
	}
	return copy.String()
}

func base64StdEncoding(value []byte) string {
	return base64.StdEncoding.EncodeToString(value)
}
