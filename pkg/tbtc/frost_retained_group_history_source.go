package tbtc

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"mime"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/keep-network/keep-core/pkg/chain"
)

const (
	frostRetainedGroupHistoryPageSchema      = "tbtc-frost-retained-group-history-page/v1"
	frostRetainedGroupOperatorReceiptSchema  = "tbtc-frost-retained-group-operator-receipt/v1"
	frostRetainedGroupHistoryRequestSchema   = "tbtc-frost-retained-group-history-request/v1"
	frostRetainedGroupOperatorRequestSchema  = "tbtc-frost-retained-group-operator-request/v1"
	frostRetainedGroupHistorySignatureDomain = "tbtc-frost-retained-group-export-signature-v1\x00"
	frostRetainedGroupHistoryEndpointDomain  = "tbtc-frost-retained-group-endpoints-v1\x00"
	frostRetainedGroupHistoryQueryDomain     = "tbtc-frost-retained-group-history-query-v1\x00"
	frostRetainedGroupOperatorQueryDomain    = "tbtc-frost-retained-group-operator-query-v1\x00"
	frostRetainedGroupHistoryRootDomain      = "tbtc-frost-retained-group-export-history-root-v1\x00"

	frostRetainedGroupMaximumResponseBytes = 1024 * 1024
	frostRetainedGroupMaximumPages         = 4096
	frostRetainedGroupMaximumMutations     = 250000
	frostRetainedGroupMaximumCursorBytes   = 256
	frostRetainedGroupMaximumReasonBytes   = 1024
	frostRetainedGroupDefaultTimeout       = 20 * time.Second
)

// FrostRetainedGroupHistorySourceConfig configures the independent,
// receipt-complete retained-group history service. ExportURL and EthereumURL
// must be distinct from each other and from the primary Ethereum endpoint. The
// export authority's Ed25519 SPKI hash is pinned rather than learned from the
// service.
type FrostRetainedGroupHistorySourceConfig struct {
	ExportURL            string
	EthereumURL          string
	TrustDomainID        string
	TrustedSignerKeyHash string
	RequestTimeout       time.Duration
}

type frostRetainedGroupEthereumVerifier interface {
	ChainID(context.Context) (*big.Int, error)
	HeaderByNumber(context.Context, *big.Int) (*types.Header, error)
	TransactionReceipt(context.Context, common.Hash) (*types.Receipt, error)
	CodeAt(context.Context, common.Address, *big.Int) ([]byte, error)
	CallContract(context.Context, ethereum.CallMsg, *big.Int) ([]byte, error)
	Close()
}

type frostRetainedGroupHTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

// signedFrostRetainedGroupHistorySource consumes independently signed,
// paginated history receipts and checks their block commitments against a
// separately configured finalized Ethereum endpoint. It intentionally does
// not use eth_getLogs: generic RPC providers cannot prove a capped response is
// complete.
type signedFrostRetainedGroupHistorySource struct {
	exportEndpoint       *url.URL
	verifier             frostRetainedGroupEthereumVerifier
	httpClient           frostRetainedGroupHTTPClient
	chainID              uint64
	identity             FrostRetainedGroupHistoryIdentity
	trustedSignerKeyHash [32]byte
	evidenceMutex        sync.RWMutex
	evidence             *frostRetainedGroupEvidenceProfile
}

var _ FrostRetainedGroupHistorySource = (*signedFrostRetainedGroupHistorySource)(nil)

type frostRetainedGroupSignedEnvelope struct {
	Schema              string          `json:"schema"`
	Payload             json.RawMessage `json:"payload"`
	PayloadSHA256       string          `json:"payloadSha256"`
	SignerPublicKeySPKI string          `json:"signerPublicKeySpki"`
	SignatureAlgorithm  string          `json:"signatureAlgorithm"`
	Signature           string          `json:"signature"`
}

type frostRetainedGroupWireIdentity struct {
	TrustDomainID       string `json:"trustDomainID"`
	EndpointFingerprint string `json:"endpointFingerprint"`
	OperatorFingerprint string `json:"operatorFingerprint"`
}

type frostRetainedGroupWireFinality struct {
	RelayTransactionHash  string `json:"relayTransactionHash"`
	BlockNumber           uint64 `json:"blockNumber"`
	BlockHash             string `json:"blockHash"`
	TransactionIndex      uint32 `json:"transactionIndex"`
	LogIndex              uint32 `json:"logIndex"`
	AuthorizationSequence string `json:"authorizationSequence"`
}

type frostRetainedGroupWireEventPoint struct {
	BlockNumber      uint64 `json:"blockNumber"`
	BlockHash        string `json:"blockHash"`
	TransactionHash  string `json:"transactionHash"`
	TransactionIndex uint32 `json:"transactionIndex"`
	LogIndex         uint32 `json:"logIndex"`
}

type frostRetainedGroupWireMutation struct {
	Point                   frostRetainedGroupWireEventPoint `json:"point"`
	Kind                    string                           `json:"kind"`
	WalletID                string                           `json:"walletID"`
	WalletPublicKeyHash     string                           `json:"walletPublicKeyHash"`
	OperatorIDs             []uint32                         `json:"operatorIDs"`
	RetainedGroupHash       string                           `json:"retainedGroupHash"`
	DkgResultHash           string                           `json:"dkgResultHash"`
	DkgSubmissionPoint      frostRetainedGroupWireEventPoint `json:"dkgSubmissionPoint"`
	DkgApprovalPoint        frostRetainedGroupWireEventPoint `json:"dkgApprovalPoint"`
	CreationPoint           frostRetainedGroupWireEventPoint `json:"creationPoint"`
	BridgeRegistrationPoint frostRetainedGroupWireEventPoint `json:"bridgeRegistrationPoint"`
	QuarantineID            string                           `json:"quarantineID"`
	EvidenceHash            string                           `json:"evidenceHash"`
	AuthenticationHash      string                           `json:"authenticationHash"`
	Reason                  string                           `json:"reason"`
}

type frostRetainedGroupHistoryQuery struct {
	Schema string                         `json:"schema"`
	From   frostRetainedGroupWireFinality `json:"from"`
	To     frostRetainedGroupWireFinality `json:"to"`
}

type frostRetainedGroupHistoryPageRequest struct {
	Query  frostRetainedGroupHistoryQuery `json:"query"`
	Cursor string                         `json:"cursor"`
}

type frostRetainedGroupHistoryReceipt struct {
	PageCount     uint64 `json:"pageCount"`
	MutationCount uint64 `json:"mutationCount"`
	HistoryRoot   string `json:"historyRoot"`
}

type frostRetainedGroupHistoryPagePayload struct {
	Schema            string                            `json:"schema"`
	Identity          frostRetainedGroupWireIdentity    `json:"identity"`
	ChainID           uint64                            `json:"chainID"`
	QueryHash         string                            `json:"queryHash"`
	SnapshotID        string                            `json:"snapshotID"`
	PageIndex         uint64                            `json:"pageIndex"`
	Cursor            string                            `json:"cursor"`
	PreviousPageHash  string                            `json:"previousPageHash"`
	From              frostRetainedGroupWireFinality    `json:"from"`
	To                frostRetainedGroupWireFinality    `json:"to"`
	EmptyAtFrom       bool                              `json:"emptyAtFrom"`
	DescriptorSetHash string                            `json:"descriptorSetHash"`
	Mutations         []frostRetainedGroupWireMutation  `json:"mutations"`
	NextCursor        string                            `json:"nextCursor"`
	Complete          bool                              `json:"complete"`
	Receipt           *frostRetainedGroupHistoryReceipt `json:"receipt"`
}

type frostRetainedGroupOperatorQuery struct {
	Schema          string                         `json:"schema"`
	OperatorAddress string                         `json:"operatorAddress"`
	At              frostRetainedGroupWireFinality `json:"at"`
}

type frostRetainedGroupOperatorReceiptPayload struct {
	Schema          string                         `json:"schema"`
	Identity        frostRetainedGroupWireIdentity `json:"identity"`
	ChainID         uint64                         `json:"chainID"`
	QueryHash       string                         `json:"queryHash"`
	OperatorAddress string                         `json:"operatorAddress"`
	At              frostRetainedGroupWireFinality `json:"at"`
	OperatorID      uint32                         `json:"operatorID"`
	Found           bool                           `json:"found"`
}

// FrostRetainedGroupHistoryEndpointFingerprint returns the identity committed
// by the activation manifest for this exact export/RPC endpoint pair.
func FrostRetainedGroupHistoryEndpointFingerprint(
	exportURL string,
	ethereumURL string,
) ([32]byte, error) {
	_, canonicalExport, err := validateFrostRetainedGroupEndpoint(exportURL, true)
	if err != nil {
		return [32]byte{}, fmt.Errorf("invalid retained-group export URL: [%w]", err)
	}
	_, canonicalEthereum, err := validateFrostRetainedGroupEndpoint(ethereumURL, false)
	if err != nil {
		return [32]byte{}, fmt.Errorf("invalid retained-group Ethereum URL: [%w]", err)
	}
	hasher := sha256.New()
	hasher.Write([]byte(frostRetainedGroupHistoryEndpointDomain))
	hasher.Write([]byte(canonicalExport))
	hasher.Write([]byte{0})
	hasher.Write([]byte(canonicalEthereum))
	var result [32]byte
	copy(result[:], hasher.Sum(nil))
	return result, nil
}

// NewFrostRetainedGroupHistorySource creates the production source. It fails
// closed if the verifier is not a distinct endpoint, does not expose finalized
// blocks, or serves a different chain.
func NewFrostRetainedGroupHistorySource(
	ctx context.Context,
	config FrostRetainedGroupHistorySourceConfig,
	primaryEthereumURL string,
	expectedChainID uint64,
) (*signedFrostRetainedGroupHistorySource, error) {
	if ctx == nil {
		return nil, fmt.Errorf("retained-group history context is nil")
	}
	endpoint, canonicalEthereum, identity, signerHash, timeout, err :=
		validateFrostRetainedGroupSourceConfig(config, primaryEthereumURL)
	if err != nil {
		return nil, err
	}
	if expectedChainID == 0 {
		return nil, fmt.Errorf("retained-group history expected chain ID is zero")
	}
	verifier, err := ethclient.DialContext(ctx, canonicalEthereum)
	if err != nil {
		return nil, fmt.Errorf("cannot connect independent retained-group Ethereum verifier: [%w]", err)
	}
	source, err := newSignedFrostRetainedGroupHistorySource(
		ctx,
		endpoint,
		verifier,
		&http.Client{
			Timeout: timeout,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return fmt.Errorf("retained-group export redirects are forbidden")
			},
		},
		expectedChainID,
		identity,
		signerHash,
	)
	if err != nil {
		verifier.Close()
		return nil, err
	}
	return source, nil
}

func newSignedFrostRetainedGroupHistorySource(
	ctx context.Context,
	exportEndpoint *url.URL,
	verifier frostRetainedGroupEthereumVerifier,
	httpClient frostRetainedGroupHTTPClient,
	chainID uint64,
	identity FrostRetainedGroupHistoryIdentity,
	signerHash [32]byte,
) (*signedFrostRetainedGroupHistorySource, error) {
	if exportEndpoint == nil || verifier == nil || httpClient == nil || chainID == 0 ||
		strings.TrimSpace(identity.TrustDomainID) == "" ||
		identity.EndpointFingerprint == [32]byte{} || signerHash == [32]byte{} ||
		identity.OperatorFingerprint != signerHash {
		return nil, fmt.Errorf("retained-group history source configuration is incomplete")
	}
	actualChainID, err := verifier.ChainID(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot identify independent retained-group Ethereum verifier: [%w]", err)
	}
	if actualChainID == nil || !actualChainID.IsUint64() || actualChainID.Uint64() != chainID {
		return nil, fmt.Errorf("independent retained-group Ethereum verifier chain ID mismatch")
	}
	source := &signedFrostRetainedGroupHistorySource{
		exportEndpoint:       exportEndpoint,
		verifier:             verifier,
		httpClient:           httpClient,
		chainID:              chainID,
		identity:             identity,
		trustedSignerKeyHash: signerHash,
	}
	if _, err := source.FinalizedHead(ctx); err != nil {
		return nil, fmt.Errorf("independent retained-group Ethereum verifier has no usable finalized head: [%w]", err)
	}
	return source, nil
}

func validateFrostRetainedGroupSourceConfig(
	config FrostRetainedGroupHistorySourceConfig,
	primaryEthereumURL string,
) (*url.URL, string, FrostRetainedGroupHistoryIdentity, [32]byte, time.Duration, error) {
	exportEndpoint, _, err := validateFrostRetainedGroupEndpoint(config.ExportURL, true)
	if err != nil {
		return nil, "", FrostRetainedGroupHistoryIdentity{}, [32]byte{}, 0,
			fmt.Errorf("invalid retained-group export URL: [%w]", err)
	}
	ethereumEndpoint, canonicalEthereum, err :=
		validateFrostRetainedGroupEndpoint(config.EthereumURL, false)
	if err != nil {
		return nil, "", FrostRetainedGroupHistoryIdentity{}, [32]byte{}, 0,
			fmt.Errorf("invalid retained-group Ethereum URL: [%w]", err)
	}
	primaryEndpoint, _, err := validateFrostRetainedGroupEndpoint(primaryEthereumURL, false)
	if err != nil {
		return nil, "", FrostRetainedGroupHistoryIdentity{}, [32]byte{}, 0,
			fmt.Errorf("invalid primary Ethereum URL: [%w]", err)
	}
	if frostRetainedGroupEndpointsShareAuthority(primaryEndpoint, ethereumEndpoint) ||
		frostRetainedGroupEndpointsShareAuthority(primaryEndpoint, exportEndpoint) {
		return nil, "", FrostRetainedGroupHistoryIdentity{}, [32]byte{}, 0,
			fmt.Errorf("retained-group source is not independent of the primary endpoint")
	}
	if frostRetainedGroupEndpointsShareAuthority(exportEndpoint, ethereumEndpoint) {
		return nil, "", FrostRetainedGroupHistoryIdentity{}, [32]byte{}, 0,
			fmt.Errorf("retained-group exporter and Ethereum verifier are not independent")
	}
	trustDomainID := strings.TrimSpace(config.TrustDomainID)
	if trustDomainID == "" || len(trustDomainID) > 128 || strings.ContainsAny(trustDomainID, "\x00\r\n\t") {
		return nil, "", FrostRetainedGroupHistoryIdentity{}, [32]byte{}, 0,
			fmt.Errorf("retained-group trust domain ID is invalid")
	}
	signerHash, err := parseFrostActivationHex32(strings.TrimSpace(config.TrustedSignerKeyHash))
	if err != nil || signerHash == [32]byte{} {
		return nil, "", FrostRetainedGroupHistoryIdentity{}, [32]byte{}, 0,
			fmt.Errorf("retained-group trusted signer key hash is invalid")
	}
	endpointFingerprint, err := FrostRetainedGroupHistoryEndpointFingerprint(
		config.ExportURL,
		config.EthereumURL,
	)
	if err != nil {
		return nil, "", FrostRetainedGroupHistoryIdentity{}, [32]byte{}, 0, err
	}
	timeout := config.RequestTimeout
	if timeout == 0 {
		timeout = frostRetainedGroupDefaultTimeout
	}
	if timeout < time.Second || timeout > time.Minute {
		return nil, "", FrostRetainedGroupHistoryIdentity{}, [32]byte{}, 0,
			fmt.Errorf("retained-group request timeout is outside supported bounds")
	}
	return exportEndpoint, canonicalEthereum, FrostRetainedGroupHistoryIdentity{
		TrustDomainID:       trustDomainID,
		EndpointFingerprint: endpointFingerprint,
		OperatorFingerprint: signerHash,
	}, signerHash, timeout, nil
}

func frostRetainedGroupEndpointsShareAuthority(left *url.URL, right *url.URL) bool {
	if left == nil || right == nil || !strings.EqualFold(left.Hostname(), right.Hostname()) {
		return false
	}
	leftIP := net.ParseIP(left.Hostname())
	rightIP := net.ParseIP(right.Hostname())
	if leftIP != nil && rightIP != nil && leftIP.IsLoopback() && rightIP.IsLoopback() {
		return strings.EqualFold(left.Host, right.Host)
	}
	return true
}

func validateFrostRetainedGroupEndpoint(
	raw string,
	export bool,
) (*url.URL, string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed != raw {
		return nil, "", fmt.Errorf("URL is empty or has surrounding whitespace")
	}
	endpoint, err := url.Parse(trimmed)
	if err != nil || endpoint.Host == "" || endpoint.User != nil || endpoint.Fragment != "" {
		return nil, "", fmt.Errorf("URL is not an absolute credential-free endpoint")
	}
	if export && endpoint.RawQuery != "" {
		return nil, "", fmt.Errorf("export URL must not contain a query")
	}
	scheme := strings.ToLower(endpoint.Scheme)
	secure := scheme == "https" || (!export && scheme == "wss")
	insecure := scheme == "http" || (!export && scheme == "ws")
	if !secure && !insecure {
		return nil, "", fmt.Errorf("URL scheme is unsupported")
	}
	if insecure {
		ip := net.ParseIP(endpoint.Hostname())
		if ip == nil || !ip.IsLoopback() {
			return nil, "", fmt.Errorf("non-TLS endpoint must use a numeric loopback address")
		}
	}
	endpoint.Scheme = scheme
	endpoint.Host = strings.ToLower(endpoint.Host)
	if endpoint.Path == "" {
		endpoint.Path = "/"
	}
	return endpoint, endpoint.String(), nil
}

func (source *signedFrostRetainedGroupHistorySource) Close() {
	if source != nil && source.verifier != nil {
		source.verifier.Close()
	}
}

func (source *signedFrostRetainedGroupHistorySource) Identity(
	ctx context.Context,
) (FrostRetainedGroupHistoryIdentity, error) {
	if ctx == nil {
		return FrostRetainedGroupHistoryIdentity{}, fmt.Errorf("retained-group identity context is nil")
	}
	select {
	case <-ctx.Done():
		return FrostRetainedGroupHistoryIdentity{}, ctx.Err()
	default:
		return source.identity, nil
	}
}

func (source *signedFrostRetainedGroupHistorySource) FinalizedHead(
	ctx context.Context,
) (FrostPreSignFinality, error) {
	if ctx == nil {
		return FrostPreSignFinality{}, fmt.Errorf("retained-group finalized-head context is nil")
	}
	header, err := source.verifier.HeaderByNumber(
		ctx,
		big.NewInt(int64(rpc.FinalizedBlockNumber)),
	)
	if err != nil {
		return FrostPreSignFinality{}, err
	}
	if header == nil || header.Number == nil || !header.Number.IsUint64() ||
		header.Number.Sign() <= 0 || header.Hash() == (common.Hash{}) {
		return FrostPreSignFinality{}, fmt.Errorf("retained-group finalized header is invalid")
	}
	return FrostPreSignFinality{
		BlockNumber: header.Number.Uint64(),
		BlockHash:   header.Hash(),
	}, nil
}

func (source *signedFrostRetainedGroupHistorySource) VerifyPoint(
	ctx context.Context,
	point FrostPreSignFinality,
) error {
	head, err := source.FinalizedHead(ctx)
	if err != nil {
		return err
	}
	return source.verifyPointAtFinalizedHead(ctx, point, head)
}

func (source *signedFrostRetainedGroupHistorySource) verifyPointAtFinalizedHead(
	ctx context.Context,
	point FrostPreSignFinality,
	head FrostPreSignFinality,
) error {
	if point.BlockNumber == 0 || point.BlockHash == [32]byte{} ||
		point.BlockNumber > head.BlockNumber {
		return fmt.Errorf("retained-group point is invalid or not finalized")
	}
	header, err := source.verifier.HeaderByNumber(
		ctx,
		new(big.Int).SetUint64(point.BlockNumber),
	)
	if err != nil {
		return err
	}
	if header == nil || header.Number == nil || !header.Number.IsUint64() ||
		header.Number.Uint64() != point.BlockNumber || header.Hash() != common.Hash(point.BlockHash) {
		return fmt.Errorf("retained-group point does not match the independent canonical chain")
	}
	if point.BlockNumber == head.BlockNumber && point.BlockHash != head.BlockHash {
		return fmt.Errorf("retained-group point conflicts with the finalized head")
	}
	return nil
}

func (source *signedFrostRetainedGroupHistorySource) ReadCompleteHistory(
	ctx context.Context,
	from FrostPreSignFinality,
	to FrostPreSignFinality,
) (*FrostRetainedGroupHistory, error) {
	if ctx == nil {
		return nil, fmt.Errorf("retained-group history context is nil")
	}
	if from.BlockNumber == 0 || from.BlockHash == [32]byte{} ||
		to.BlockNumber < from.BlockNumber || to.BlockHash == [32]byte{} {
		return nil, fmt.Errorf("retained-group history bounds are invalid")
	}
	evidence, err := source.activationEvidence()
	if err != nil {
		return nil, err
	}
	headBefore, err := source.FinalizedHead(ctx)
	if err != nil {
		return nil, err
	}
	if err := source.verifyPointAtFinalizedHead(ctx, from, headBefore); err != nil {
		return nil, fmt.Errorf("retained-group checkpoint is not canonical: [%w]", err)
	}
	if err := source.verifyPointAtFinalizedHead(ctx, to, headBefore); err != nil {
		return nil, fmt.Errorf("retained-group target is not canonical: [%w]", err)
	}

	query := frostRetainedGroupHistoryQuery{
		Schema: frostRetainedGroupHistoryRequestSchema,
		From:   frostRetainedGroupFinalityToWire(from),
		To:     frostRetainedGroupFinalityToWire(to),
	}
	queryHash, err := frostRetainedGroupDomainHash(
		frostRetainedGroupHistoryQueryDomain,
		query,
	)
	if err != nil {
		return nil, err
	}

	mutations := make([]FrostRetainedGroupMutation, 0)
	wireMutations := make([]frostRetainedGroupWireMutation, 0)
	seenCursors := make(map[string]bool)
	blockHashes := map[uint64][32]byte{
		from.BlockNumber: from.BlockHash,
		to.BlockNumber:   to.BlockHash,
	}
	cursor := ""
	var snapshotID [32]byte
	var descriptorSetHash [32]byte
	var previousPageHash [32]byte
	for pageIndex := uint64(0); pageIndex < frostRetainedGroupMaximumPages; pageIndex++ {
		if seenCursors[cursor] {
			return nil, fmt.Errorf("retained-group history cursor repeated")
		}
		seenCursors[cursor] = true
		request := frostRetainedGroupHistoryPageRequest{Query: query, Cursor: cursor}
		payload := &frostRetainedGroupHistoryPagePayload{}
		if err := source.postSigned(ctx, "history", request, payload); err != nil {
			return nil, fmt.Errorf("cannot read retained-group history page [%d]: [%w]", pageIndex, err)
		}
		pageHash, err := source.validateHistoryPage(
			payload,
			queryHash,
			from,
			to,
			cursor,
			pageIndex,
			previousPageHash,
			snapshotID,
			descriptorSetHash,
		)
		if err != nil {
			return nil, err
		}
		parsedSnapshotID, _ := parseFrostActivationHex32(payload.SnapshotID)
		parsedDescriptorSetHash, _ := parseFrostActivationHex32(payload.DescriptorSetHash)
		if pageIndex == 0 {
			snapshotID = parsedSnapshotID
			descriptorSetHash = parsedDescriptorSetHash
		}
		for _, wireMutation := range payload.Mutations {
			mutation, err := frostRetainedGroupMutationFromWire(wireMutation)
			if err != nil {
				return nil, fmt.Errorf("retained-group history contains malformed mutation: [%w]", err)
			}
			if err := addFrostRetainedGroupMutationBlockHashes(blockHashes, mutation); err != nil {
				return nil, err
			}
			mutations = append(mutations, mutation)
			wireMutations = append(wireMutations, wireMutation)
			if len(mutations) > frostRetainedGroupMaximumMutations {
				return nil, fmt.Errorf("retained-group history exceeds the mutation limit")
			}
		}
		if payload.Complete {
			if payload.Receipt == nil || payload.NextCursor != "" ||
				payload.Receipt.PageCount != pageIndex+1 ||
				payload.Receipt.MutationCount != uint64(len(mutations)) {
				return nil, fmt.Errorf("retained-group final history receipt is inconsistent")
			}
			receiptRoot, err := parseFrostActivationHex32(payload.Receipt.HistoryRoot)
			if err != nil {
				return nil, fmt.Errorf("retained-group history receipt root is invalid")
			}
			computedRoot, err := frostRetainedGroupHistoryRoot(queryHash, wireMutations)
			if err != nil || receiptRoot != computedRoot {
				return nil, fmt.Errorf("retained-group history receipt does not cover the exact mutation sequence")
			}
			history := &FrostRetainedGroupHistory{
				From:              from,
				To:                to,
				Mutations:         mutations,
				Complete:          true,
				EmptyAtFrom:       true,
				DescriptorSetHash: descriptorSetHash,
			}
			if err := validateCompleteFrostRetainedGroupHistory(history); err != nil {
				return nil, err
			}
			for blockNumber, blockHash := range blockHashes {
				if err := source.verifyPointAtFinalizedHead(ctx, FrostPreSignFinality{
					BlockNumber: blockNumber,
					BlockHash:   blockHash,
				}, headBefore); err != nil {
					return nil, fmt.Errorf("retained-group history references a noncanonical block: [%w]", err)
				}
			}
			if err := source.verifyHistoryEvidence(ctx, mutations, evidence); err != nil {
				return nil, fmt.Errorf("retained-group history has unauthenticated semantic evidence: [%w]", err)
			}
			headAfter, err := source.FinalizedHead(ctx)
			if err != nil {
				return nil, err
			}
			if headAfter.BlockNumber < headBefore.BlockNumber ||
				(headAfter.BlockNumber == headBefore.BlockNumber && headAfter.BlockHash != headBefore.BlockHash) {
				return nil, fmt.Errorf("retained-group finalized head changed inconsistently during export")
			}
			if err := source.verifyPointAtFinalizedHead(ctx, to, headAfter); err != nil {
				return nil, fmt.Errorf("retained-group target changed during export: [%w]", err)
			}
			return history, nil
		}
		if payload.Receipt != nil || len(payload.Mutations) == 0 ||
			!validFrostRetainedGroupCursor(payload.NextCursor) || payload.NextCursor == cursor {
			return nil, fmt.Errorf("retained-group nonfinal history page is malformed")
		}
		cursor = payload.NextCursor
		previousPageHash = pageHash
	}
	return nil, fmt.Errorf("retained-group history exceeded the page limit without a final receipt")
}

func (source *signedFrostRetainedGroupHistorySource) validateHistoryPage(
	payload *frostRetainedGroupHistoryPagePayload,
	queryHash [32]byte,
	from FrostPreSignFinality,
	to FrostPreSignFinality,
	cursor string,
	pageIndex uint64,
	previousPageHash [32]byte,
	snapshotID [32]byte,
	descriptorSetHash [32]byte,
) ([32]byte, error) {
	if payload == nil || payload.Schema != frostRetainedGroupHistoryPageSchema ||
		payload.ChainID != source.chainID || payload.PageIndex != pageIndex ||
		payload.Cursor != cursor || !payload.EmptyAtFrom ||
		!source.validWireIdentity(payload.Identity) {
		return [32]byte{}, fmt.Errorf("retained-group history page has the wrong identity or position")
	}
	declaredQueryHash, err := parseFrostActivationHex32(payload.QueryHash)
	if err != nil || declaredQueryHash != queryHash {
		return [32]byte{}, fmt.Errorf("retained-group history page is bound to a different query")
	}
	pageSnapshotID, err := parseFrostActivationHex32(payload.SnapshotID)
	if err != nil || pageSnapshotID == [32]byte{} || (pageIndex > 0 && pageSnapshotID != snapshotID) {
		return [32]byte{}, fmt.Errorf("retained-group history snapshot changed between pages")
	}
	pageDescriptorSetHash, err := parseFrostActivationHex32(payload.DescriptorSetHash)
	evidence, evidenceErr := source.activationEvidence()
	if err != nil || evidenceErr != nil || pageDescriptorSetHash != evidence.descriptorSetHash ||
		(pageIndex > 0 && pageDescriptorSetHash != descriptorSetHash) {
		return [32]byte{}, fmt.Errorf("retained-group descriptor set differs from the signed activation manifest")
	}
	pageFrom, err := frostRetainedGroupFinalityFromWire(payload.From)
	if err != nil || pageFrom != from {
		return [32]byte{}, fmt.Errorf("retained-group history page checkpoint mismatch")
	}
	pageTo, err := frostRetainedGroupFinalityFromWire(payload.To)
	if err != nil || pageTo != to {
		return [32]byte{}, fmt.Errorf("retained-group history page target mismatch")
	}
	declaredPreviousPageHash, err := parseFrostActivationHex32(payload.PreviousPageHash)
	if err != nil || declaredPreviousPageHash != previousPageHash {
		return [32]byte{}, fmt.Errorf("retained-group history page hash chain is broken")
	}
	if len(payload.Mutations) > frostRetainedGroupMaximumMutations {
		return [32]byte{}, fmt.Errorf("retained-group history page exceeds the mutation limit")
	}
	canonical, err := canonicalFrostActivationValue(payload)
	if err != nil {
		return [32]byte{}, err
	}
	return sha256.Sum256(canonical), nil
}

func (source *signedFrostRetainedGroupHistorySource) ResolveOperatorID(
	ctx context.Context,
	operator chain.Address,
	at FrostPreSignFinality,
) (chain.OperatorID, error) {
	if ctx == nil {
		return 0, fmt.Errorf("retained-group operator-resolution context is nil")
	}
	evidence, err := source.activationEvidence()
	if err != nil {
		return 0, err
	}
	canonicalAddress, err := canonicalFrostRetainedGroupOperatorAddress(operator)
	if err != nil {
		return 0, err
	}
	headBefore, err := source.FinalizedHead(ctx)
	if err != nil {
		return 0, err
	}
	if err := source.verifyPointAtFinalizedHead(ctx, at, headBefore); err != nil {
		return 0, err
	}
	query := frostRetainedGroupOperatorQuery{
		Schema:          frostRetainedGroupOperatorRequestSchema,
		OperatorAddress: canonicalAddress,
		At:              frostRetainedGroupFinalityToWire(at),
	}
	queryHash, err := frostRetainedGroupDomainHash(frostRetainedGroupOperatorQueryDomain, query)
	if err != nil {
		return 0, err
	}
	payload := &frostRetainedGroupOperatorReceiptPayload{}
	if err := source.postSigned(ctx, "operator-id", query, payload); err != nil {
		return 0, fmt.Errorf("cannot resolve retained-group operator ID: [%w]", err)
	}
	declaredQueryHash, err := parseFrostActivationHex32(payload.QueryHash)
	receiptAt, pointErr := frostRetainedGroupFinalityFromWire(payload.At)
	if payload.Schema != frostRetainedGroupOperatorReceiptSchema ||
		payload.ChainID != source.chainID || !source.validWireIdentity(payload.Identity) ||
		err != nil || declaredQueryHash != queryHash || pointErr != nil || receiptAt != at ||
		payload.OperatorAddress != canonicalAddress || !payload.Found || payload.OperatorID == 0 {
		return 0, fmt.Errorf("retained-group operator receipt is incomplete or differently bound")
	}
	onChainOperatorID, err := source.resolveOperatorIDAt(
		ctx,
		common.HexToAddress(canonicalAddress),
		at,
		evidence,
	)
	if err != nil {
		return 0, fmt.Errorf("cannot independently authenticate retained-group operator ID: [%w]", err)
	}
	if onChainOperatorID != payload.OperatorID {
		return 0, fmt.Errorf("retained-group operator receipt disagrees with exact finalized sortition-pool state")
	}
	headAfter, err := source.FinalizedHead(ctx)
	if err != nil {
		return 0, err
	}
	if headAfter.BlockNumber < headBefore.BlockNumber ||
		(headAfter.BlockNumber == headBefore.BlockNumber && headAfter.BlockHash != headBefore.BlockHash) {
		return 0, fmt.Errorf("retained-group finalized head changed during operator resolution")
	}
	if err := source.verifyPointAtFinalizedHead(ctx, at, headAfter); err != nil {
		return 0, fmt.Errorf("retained-group operator-resolution point changed: [%w]", err)
	}
	return chain.OperatorID(payload.OperatorID), nil
}

func (source *signedFrostRetainedGroupHistorySource) postSigned(
	ctx context.Context,
	operation string,
	requestPayload interface{},
	responsePayload interface{},
) error {
	requestBody, err := json.Marshal(requestPayload)
	if err != nil {
		return err
	}
	endpoint := *source.exportEndpoint
	endpoint.Path = path.Join(endpoint.Path, operation)
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		endpoint.String(),
		bytes.NewReader(requestBody),
	)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	response, err := source.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("retained-group export returned HTTP status [%d]", response.StatusCode)
	}
	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return fmt.Errorf("retained-group export response is not application/json")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, frostRetainedGroupMaximumResponseBytes+1))
	if err != nil {
		return err
	}
	if len(data) == 0 || len(data) > frostRetainedGroupMaximumResponseBytes {
		return fmt.Errorf("retained-group export response size is invalid")
	}
	envelope := &frostRetainedGroupSignedEnvelope{}
	if err := decodeStrictFrostActivationJSON(data, envelope); err != nil {
		return fmt.Errorf("cannot decode retained-group signed envelope: [%w]", err)
	}
	return source.verifySignedEnvelope(envelope, responsePayload)
}

func (source *signedFrostRetainedGroupHistorySource) verifySignedEnvelope(
	envelope *frostRetainedGroupSignedEnvelope,
	target interface{},
) error {
	if envelope == nil || envelope.Schema != "tbtc-frost-retained-group-signed-envelope/v1" ||
		envelope.SignatureAlgorithm != "ed25519" || len(envelope.Payload) == 0 {
		return fmt.Errorf("retained-group signed envelope is malformed")
	}
	canonical, err := canonicalFrostActivationValue(envelope.Payload)
	if err != nil {
		return err
	}
	payloadHash := sha256.Sum256(canonical)
	declaredHash, err := parseFrostActivationHex32(envelope.PayloadSHA256)
	if err != nil || declaredHash != payloadHash {
		return fmt.Errorf("retained-group signed envelope payload hash mismatch")
	}
	publicKeyDER, err := base64.StdEncoding.Strict().DecodeString(envelope.SignerPublicKeySPKI)
	if err != nil || len(publicKeyDER) == 0 || len(publicKeyDER) > 1024 ||
		sha256.Sum256(publicKeyDER) != source.trustedSignerKeyHash {
		return fmt.Errorf("retained-group export signer is not trusted")
	}
	parsedKey, err := x509.ParsePKIXPublicKey(publicKeyDER)
	if err != nil {
		return fmt.Errorf("cannot parse retained-group export signer: [%w]", err)
	}
	publicKey, ok := parsedKey.(ed25519.PublicKey)
	if !ok {
		return fmt.Errorf("retained-group export signer is not Ed25519")
	}
	signature, err := base64.StdEncoding.Strict().DecodeString(envelope.Signature)
	signed := append([]byte(frostRetainedGroupHistorySignatureDomain), canonical...)
	if err != nil || len(signature) != ed25519.SignatureSize ||
		!ed25519.Verify(publicKey, signed, signature) {
		return fmt.Errorf("retained-group export signature is invalid")
	}
	if err := decodeStrictFrostActivationJSON(envelope.Payload, target); err != nil {
		return fmt.Errorf("cannot decode retained-group signed payload: [%w]", err)
	}
	return nil
}

func (source *signedFrostRetainedGroupHistorySource) validWireIdentity(
	identity frostRetainedGroupWireIdentity,
) bool {
	endpoint, endpointErr := parseFrostActivationHex32(identity.EndpointFingerprint)
	operator, operatorErr := parseFrostActivationHex32(identity.OperatorFingerprint)
	return identity.TrustDomainID == source.identity.TrustDomainID &&
		endpointErr == nil && endpoint == source.identity.EndpointFingerprint &&
		operatorErr == nil && operator == source.identity.OperatorFingerprint
}

func frostRetainedGroupDomainHash(domain string, value interface{}) ([32]byte, error) {
	canonical, err := canonicalFrostActivationValue(value)
	if err != nil {
		return [32]byte{}, err
	}
	hasher := sha256.New()
	hasher.Write([]byte(domain))
	hasher.Write(canonical)
	var result [32]byte
	copy(result[:], hasher.Sum(nil))
	return result, nil
}

func frostRetainedGroupHistoryRoot(
	queryHash [32]byte,
	mutations []frostRetainedGroupWireMutation,
) ([32]byte, error) {
	hasher := sha256.New()
	hasher.Write([]byte(frostRetainedGroupHistoryRootDomain))
	hasher.Write(queryHash[:])
	count := make([]byte, 8)
	for i := uint(0); i < 8; i++ {
		count[7-i] = byte(uint64(len(mutations)) >> (i * 8))
	}
	hasher.Write(count)
	for _, mutation := range mutations {
		canonical, err := canonicalFrostActivationValue(mutation)
		if err != nil {
			return [32]byte{}, err
		}
		itemHash := sha256.Sum256(canonical)
		hasher.Write(itemHash[:])
	}
	var result [32]byte
	copy(result[:], hasher.Sum(nil))
	return result, nil
}

func frostRetainedGroupFinalityToWire(
	point FrostPreSignFinality,
) frostRetainedGroupWireFinality {
	return frostRetainedGroupWireFinality{
		RelayTransactionHash:  frostActivationHex32(point.RelayTransactionHash),
		BlockNumber:           point.BlockNumber,
		BlockHash:             frostActivationHex32(point.BlockHash),
		TransactionIndex:      point.TransactionIndex,
		LogIndex:              point.LogIndex,
		AuthorizationSequence: frostActivationHex32(point.AuthorizationSequence),
	}
}

func frostRetainedGroupFinalityFromWire(
	point frostRetainedGroupWireFinality,
) (FrostPreSignFinality, error) {
	relayHash, err := parseFrostActivationHex32(point.RelayTransactionHash)
	if err != nil {
		return FrostPreSignFinality{}, err
	}
	blockHash, err := parseFrostActivationHex32(point.BlockHash)
	if err != nil || point.BlockNumber == 0 || blockHash == [32]byte{} {
		return FrostPreSignFinality{}, fmt.Errorf("retained-group finality block is invalid")
	}
	sequence, err := parseFrostActivationHex32(point.AuthorizationSequence)
	if err != nil {
		return FrostPreSignFinality{}, err
	}
	return FrostPreSignFinality{
		RelayTransactionHash:  relayHash,
		BlockNumber:           point.BlockNumber,
		BlockHash:             blockHash,
		TransactionIndex:      point.TransactionIndex,
		LogIndex:              point.LogIndex,
		AuthorizationSequence: sequence,
	}, nil
}

func frostRetainedGroupEventPointToWire(
	point FrostRetainedGroupEventPoint,
) frostRetainedGroupWireEventPoint {
	return frostRetainedGroupWireEventPoint{
		BlockNumber:      point.BlockNumber,
		BlockHash:        frostActivationHex32(point.BlockHash),
		TransactionHash:  frostActivationHex32(point.TransactionHash),
		TransactionIndex: point.TransactionIndex,
		LogIndex:         point.LogIndex,
	}
}

func frostRetainedGroupEventPointFromWire(
	point frostRetainedGroupWireEventPoint,
) (FrostRetainedGroupEventPoint, error) {
	blockHash, err := parseFrostActivationHex32(point.BlockHash)
	if err != nil {
		return FrostRetainedGroupEventPoint{}, err
	}
	transactionHash, err := parseFrostActivationHex32(point.TransactionHash)
	if err != nil {
		return FrostRetainedGroupEventPoint{}, err
	}
	result := FrostRetainedGroupEventPoint{
		BlockNumber:      point.BlockNumber,
		BlockHash:        blockHash,
		TransactionHash:  transactionHash,
		TransactionIndex: point.TransactionIndex,
		LogIndex:         point.LogIndex,
	}
	if result.BlockNumber == 0 {
		if result != (FrostRetainedGroupEventPoint{}) {
			return FrostRetainedGroupEventPoint{}, fmt.Errorf("zero retained-group event point is noncanonical")
		}
		return result, nil
	}
	if !result.valid() {
		return FrostRetainedGroupEventPoint{}, fmt.Errorf("retained-group event point is invalid")
	}
	return result, nil
}

func frostRetainedGroupMutationFromWire(
	mutation frostRetainedGroupWireMutation,
) (FrostRetainedGroupMutation, error) {
	point, err := frostRetainedGroupEventPointFromWire(mutation.Point)
	if err != nil || !point.valid() {
		return FrostRetainedGroupMutation{}, fmt.Errorf("mutation point is invalid")
	}
	walletID, err := parseFrostActivationHex32(mutation.WalletID)
	if err != nil {
		return FrostRetainedGroupMutation{}, err
	}
	walletPublicKeyHash, err := parseFrostRetainedGroupHex20(mutation.WalletPublicKeyHash)
	if err != nil {
		return FrostRetainedGroupMutation{}, err
	}
	retainedGroupHash, err := parseFrostActivationHex32(mutation.RetainedGroupHash)
	if err != nil {
		return FrostRetainedGroupMutation{}, err
	}
	dkgResultHash, err := parseFrostActivationHex32(mutation.DkgResultHash)
	if err != nil {
		return FrostRetainedGroupMutation{}, err
	}
	dkgSubmissionPoint, err := frostRetainedGroupEventPointFromWire(mutation.DkgSubmissionPoint)
	if err != nil {
		return FrostRetainedGroupMutation{}, err
	}
	dkgApprovalPoint, err := frostRetainedGroupEventPointFromWire(mutation.DkgApprovalPoint)
	if err != nil {
		return FrostRetainedGroupMutation{}, err
	}
	creationPoint, err := frostRetainedGroupEventPointFromWire(mutation.CreationPoint)
	if err != nil {
		return FrostRetainedGroupMutation{}, err
	}
	registrationPoint, err := frostRetainedGroupEventPointFromWire(mutation.BridgeRegistrationPoint)
	if err != nil {
		return FrostRetainedGroupMutation{}, err
	}
	quarantineID, err := parseFrostActivationHex32(mutation.QuarantineID)
	if err != nil {
		return FrostRetainedGroupMutation{}, err
	}
	evidenceHash, err := parseFrostActivationHex32(mutation.EvidenceHash)
	if err != nil {
		return FrostRetainedGroupMutation{}, err
	}
	authenticationHash, err := parseFrostActivationHex32(mutation.AuthenticationHash)
	if err != nil {
		return FrostRetainedGroupMutation{}, err
	}
	if len(mutation.OperatorIDs) > 100 || len(mutation.Reason) > frostRetainedGroupMaximumReasonBytes {
		return FrostRetainedGroupMutation{}, fmt.Errorf("retained-group mutation exceeds field bounds")
	}
	operatorIDs := append([]uint32{}, mutation.OperatorIDs...)
	seenOperatorIDs := make(map[uint32]bool, len(operatorIDs))
	for _, operatorID := range operatorIDs {
		if operatorID == 0 || seenOperatorIDs[operatorID] {
			return FrostRetainedGroupMutation{}, fmt.Errorf("retained-group mutation has invalid or duplicate operator IDs")
		}
		seenOperatorIDs[operatorID] = true
	}
	return FrostRetainedGroupMutation{
		Point:                   point,
		Kind:                    FrostRetainedGroupMutationKind(mutation.Kind),
		WalletID:                walletID,
		WalletPublicKeyHash:     walletPublicKeyHash,
		OperatorIDs:             operatorIDs,
		RetainedGroupHash:       retainedGroupHash,
		DkgResultHash:           dkgResultHash,
		DkgSubmissionPoint:      dkgSubmissionPoint,
		DkgApprovalPoint:        dkgApprovalPoint,
		CreationPoint:           creationPoint,
		BridgeRegistrationPoint: registrationPoint,
		QuarantineID:            quarantineID,
		EvidenceHash:            evidenceHash,
		AuthenticationHash:      authenticationHash,
		Reason:                  mutation.Reason,
	}, nil
}

func frostRetainedGroupMutationToWire(
	mutation FrostRetainedGroupMutation,
) frostRetainedGroupWireMutation {
	return frostRetainedGroupWireMutation{
		Point:                   frostRetainedGroupEventPointToWire(mutation.Point),
		Kind:                    string(mutation.Kind),
		WalletID:                frostActivationHex32(mutation.WalletID),
		WalletPublicKeyHash:     frostActivationHex20(mutation.WalletPublicKeyHash),
		OperatorIDs:             append([]uint32{}, mutation.OperatorIDs...),
		RetainedGroupHash:       frostActivationHex32(mutation.RetainedGroupHash),
		DkgResultHash:           frostActivationHex32(mutation.DkgResultHash),
		DkgSubmissionPoint:      frostRetainedGroupEventPointToWire(mutation.DkgSubmissionPoint),
		DkgApprovalPoint:        frostRetainedGroupEventPointToWire(mutation.DkgApprovalPoint),
		CreationPoint:           frostRetainedGroupEventPointToWire(mutation.CreationPoint),
		BridgeRegistrationPoint: frostRetainedGroupEventPointToWire(mutation.BridgeRegistrationPoint),
		QuarantineID:            frostActivationHex32(mutation.QuarantineID),
		EvidenceHash:            frostActivationHex32(mutation.EvidenceHash),
		AuthenticationHash:      frostActivationHex32(mutation.AuthenticationHash),
		Reason:                  mutation.Reason,
	}
}

func parseFrostRetainedGroupHex20(value string) ([20]byte, error) {
	if len(value) != 42 || !strings.HasPrefix(value, "0x") || value != strings.ToLower(value) {
		return [20]byte{}, fmt.Errorf("value is not canonical bytes20")
	}
	decoded, err := hex.DecodeString(value[2:])
	if err != nil || len(decoded) != 20 {
		return [20]byte{}, fmt.Errorf("value is not bytes20")
	}
	var result [20]byte
	copy(result[:], decoded)
	return result, nil
}

func addFrostRetainedGroupMutationBlockHashes(
	blocks map[uint64][32]byte,
	mutation FrostRetainedGroupMutation,
) error {
	points := []FrostRetainedGroupEventPoint{
		mutation.Point,
		mutation.DkgSubmissionPoint,
		mutation.DkgApprovalPoint,
		mutation.CreationPoint,
		mutation.BridgeRegistrationPoint,
	}
	for _, point := range points {
		if point.BlockNumber == 0 {
			continue
		}
		if existing, ok := blocks[point.BlockNumber]; ok && existing != point.BlockHash {
			return fmt.Errorf("retained-group history contains conflicting hashes for block [%d]", point.BlockNumber)
		}
		blocks[point.BlockNumber] = point.BlockHash
	}
	return nil
}

func canonicalFrostRetainedGroupOperatorAddress(address chain.Address) (string, error) {
	raw := strings.TrimSpace(address.String())
	if !common.IsHexAddress(raw) {
		return "", fmt.Errorf("retained-group operator address is invalid")
	}
	return strings.ToLower(common.HexToAddress(raw).Hex()), nil
}

func validFrostRetainedGroupCursor(cursor string) bool {
	if cursor == "" || len(cursor) > frostRetainedGroupMaximumCursorBytes {
		return false
	}
	for _, character := range cursor {
		if !((character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			character == '-' || character == '_') {
			return false
		}
	}
	return true
}
