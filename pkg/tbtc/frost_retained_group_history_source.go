package tbtc

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
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
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/keep-network/keep-core/pkg/chain"
)

const (
	frostRetainedGroupHistoryPageSchema      = "tbtc-frost-retained-group-history-page/v5"
	frostRetainedGroupOperatorReceiptSchema  = "tbtc-frost-retained-group-operator-receipt/v4"
	frostRetainedGroupHistoryRequestSchema   = "tbtc-frost-retained-group-history-request/v4"
	frostRetainedGroupOperatorRequestSchema  = "tbtc-frost-retained-group-operator-request/v4"
	frostRetainedGroupHistorySignatureDomain = "tbtc-frost-retained-group-export-signature-v5\x00"
	frostRetainedGroupHistoryQueryDomain     = "tbtc-frost-retained-group-history-query-v4\x00"
	frostRetainedGroupOperatorQueryDomain    = "tbtc-frost-retained-group-operator-query-v4\x00"
	frostRetainedGroupHistoryRootDomain      = "tbtc-frost-retained-group-export-history-root-v4\x00"
	frostRetainedGroupProtocolBindingDomain  = "tbtc-frost-retained-group-protocol-binding-v4\x00"

	frostRetainedGroupMaximumResponseBytes          = 1024 * 1024
	frostRetainedGroupMaximumAggregateResponseBytes = 16 * 1024 * 1024
	frostRetainedGroupMaximumPages                  = 256
	frostRetainedGroupMaximumMutations              = 4096
	frostRetainedGroupMaximumWallets                = 2048
	frostRetainedGroupMaximumUniqueBlocks           = 8192
	frostRetainedGroupMaximumEvidenceReceipts       = 8192
	frostRetainedGroupMaximumEvidenceCodePoints     = 16384
	frostRetainedGroupMaximumReceiptLogs            = 4096
	frostRetainedGroupMaximumContractCodeBytes      = 64 * 1024
	frostRetainedGroupMaximumCursorBytes            = 256
	frostRetainedGroupMaximumReasonBytes            = 1024
	frostRetainedGroupDefaultTimeout                = 20 * time.Second
	frostRetainedGroupMaximumReconciliationDuration = 5 * time.Minute
)

// FrostRetainedGroupHistorySourceConfig configures the independent,
// receipt-complete retained-group history service. ExportURL and EthereumURL
// must be distinct from each other and from the primary Ethereum endpoint. The
// history-envelope, backend, operator, and transport-attestation Ed25519 SPKI
// hashes are separate manifest roles and are pinned rather than learned from
// either service.
type FrostRetainedGroupHistorySourceConfig struct {
	ExportURL                         string
	EthereumURL                       string
	TrustDomainID                     string
	ExportTrustDomainID               string
	EthereumTrustDomainID             string
	ExportServiceIdentity             string
	EthereumServiceIdentity           string
	ExportBackendServiceFingerprint   string
	EthereumBackendServiceFingerprint string
	ExportOperatorFingerprint         string
	EthereumOperatorFingerprint       string
	TrustedSignerKeyHash              string
	ExportAttestationKeyHash          string
	EthereumAttestationKeyHash        string
	ExportTLSLeafSPKIHash             string
	EthereumTLSLeafSPKIHash           string
	RequestTimeout                    time.Duration
	TLSRootCAs                        *x509.CertPool `mapstructure:"-"`
	PrimaryTLSRootCAs                 *x509.CertPool `mapstructure:"-"`
	Resolver                          *net.Resolver  `mapstructure:"-"`
}

// FrostPreSignEthereumEvidenceVerifier is the read-only Ethereum view used to
// independently authenticate security-sensitive FROST authorization evidence.
// Exact-hash methods must use EIP-1898 with requireCanonical=true.
type FrostPreSignEthereumEvidenceVerifier interface {
	ChainID(context.Context) (*big.Int, error)
	HeaderByNumber(context.Context, *big.Int) (*types.Header, error)
	HeaderByHash(context.Context, common.Hash) (*types.Header, error)
	TransactionReceipt(context.Context, common.Hash) (*types.Receipt, error)
	FilterLogs(context.Context, ethereum.FilterQuery) ([]types.Log, error)
	CodeAtHash(context.Context, common.Address, common.Hash) ([]byte, error)
	StorageAtHash(context.Context, common.Address, common.Hash, common.Hash) ([]byte, error)
	CallContractAtHash(context.Context, ethereum.CallMsg, common.Hash) ([]byte, error)
}

// FrostPreSignEthereumEvidenceVerifierSource exposes an independently
// identity-bound Ethereum verifier to the production authorization adapter.
type FrostPreSignEthereumEvidenceVerifierSource interface {
	FrostPreSignEthereumEvidenceVerifier(
		context.Context,
	) (FrostPreSignEthereumEvidenceVerifier, error)
}

type frostRetainedGroupEthereumVerifier interface {
	FrostPreSignEthereumEvidenceVerifier
	Close()
}

type frostRetainedGroupHTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

type frostRetainedGroupHistoryStatusError struct {
	statusCode int
}

func (err *frostRetainedGroupHistoryStatusError) Error() string {
	return fmt.Sprintf(
		"retained-group export returned HTTP status [%d]",
		err.statusCode,
	)
}

func (err *frostRetainedGroupHistoryStatusError) HTTPStatusCode() int {
	return err.statusCode
}

type canonicalFrostRetainedGroupEthereumVerifier struct {
	*ethclient.Client
	rpcClient *rpc.Client
}

func (verifier *canonicalFrostRetainedGroupEthereumVerifier) CodeAtHash(
	ctx context.Context,
	account common.Address,
	blockHash common.Hash,
) ([]byte, error) {
	var result hexutil.Bytes
	err := verifier.rpcClient.CallContext(
		ctx,
		&result,
		"eth_getCode",
		account,
		rpc.BlockNumberOrHashWithHash(blockHash, true),
	)
	return result, err
}

func (verifier *canonicalFrostRetainedGroupEthereumVerifier) StorageAtHash(
	ctx context.Context,
	account common.Address,
	key common.Hash,
	blockHash common.Hash,
) ([]byte, error) {
	var result hexutil.Bytes
	err := verifier.rpcClient.CallContext(
		ctx,
		&result,
		"eth_getStorageAt",
		account,
		key,
		rpc.BlockNumberOrHashWithHash(blockHash, true),
	)
	return result, err
}

func (verifier *canonicalFrostRetainedGroupEthereumVerifier) CallContractAtHash(
	ctx context.Context,
	message ethereum.CallMsg,
	blockHash common.Hash,
) ([]byte, error) {
	var result hexutil.Bytes
	err := verifier.rpcClient.CallContext(
		ctx,
		&result,
		"eth_call",
		frostRetainedGroupCallArgument(message),
		rpc.BlockNumberOrHashWithHash(blockHash, true),
	)
	return result, err
}

func frostRetainedGroupCallArgument(message ethereum.CallMsg) map[string]interface{} {
	result := map[string]interface{}{
		"from": message.From,
		"to":   message.To,
	}
	if len(message.Data) > 0 {
		result["input"] = hexutil.Bytes(message.Data)
	}
	if message.Value != nil {
		result["value"] = (*hexutil.Big)(message.Value)
	}
	if message.Gas != 0 {
		result["gas"] = hexutil.Uint64(message.Gas)
	}
	if message.GasPrice != nil {
		result["gasPrice"] = (*hexutil.Big)(message.GasPrice)
	}
	if message.GasFeeCap != nil {
		result["maxFeePerGas"] = (*hexutil.Big)(message.GasFeeCap)
	}
	if message.GasTipCap != nil {
		result["maxPriorityFeePerGas"] = (*hexutil.Big)(message.GasTipCap)
	}
	if message.AccessList != nil {
		result["accessList"] = message.AccessList
	}
	if message.BlobGasFeeCap != nil {
		result["maxFeePerBlobGas"] = (*hexutil.Big)(message.BlobGasFeeCap)
	}
	if message.BlobHashes != nil {
		result["blobVersionedHashes"] = message.BlobHashes
	}
	return result
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
	httpTransports       []*http.Transport
	independenceMonitor  *frostRetainedGroupIndependenceMonitor
	chainID              uint64
	identity             FrostRetainedGroupHistoryIdentity
	trustedSignerKeyHash [32]byte
	evidenceMutex        sync.RWMutex
	evidence             *frostRetainedGroupEvidenceProfile
	maximumPages         uint64
	maximumMutations     uint64
	maximumResponseBytes uint64
	maximumUniqueBlocks  uint64
	maximumReadDuration  time.Duration
	requestTimeout       time.Duration
}

var _ FrostRetainedGroupHistorySource = (*signedFrostRetainedGroupHistorySource)(nil)
var _ FrostPreSignEthereumEvidenceVerifierSource = (*signedFrostRetainedGroupHistorySource)(nil)

type frostRetainedGroupSignedEnvelope struct {
	Schema              string          `json:"schema"`
	BindingHash         string          `json:"bindingHash"`
	Payload             json.RawMessage `json:"payload"`
	PayloadSHA256       string          `json:"payloadSha256"`
	SignerPublicKeySPKI string          `json:"signerPublicKeySpki"`
	SignatureAlgorithm  string          `json:"signatureAlgorithm"`
	Signature           string          `json:"signature"`
}

type frostRetainedGroupWireFinality struct {
	RelayTransactionHash  string `json:"relayTransactionHash"`
	BlockNumber           uint64 `json:"blockNumber"`
	BlockHash             string `json:"blockHash"`
	TransactionIndex      uint32 `json:"transactionIndex"`
	LogIndex              uint32 `json:"logIndex"`
	AuthorizationSequence string `json:"authorizationSequence"`
}

type frostRetainedGroupWireBlockPoint struct {
	BlockNumber uint64 `json:"blockNumber"`
	BlockHash   string `json:"blockHash"`
}

type frostRetainedGroupProtocolBinding struct {
	Schema                         string                           `json:"schema"`
	ChainID                        uint64                           `json:"chainID"`
	DomainChainID                  string                           `json:"domainChainID"`
	GenesisBlockHash               string                           `json:"genesisBlockHash"`
	Checkpoint                     frostRetainedGroupWireBlockPoint `json:"checkpoint"`
	ManifestHash                   string                           `json:"manifestHash"`
	ProfileHash                    string                           `json:"profileHash"`
	ImplementationSetHash          string                           `json:"implementationSetHash"`
	DescriptorSetHash              string                           `json:"descriptorSetHash"`
	LinkedLibraryDescriptorSetHash string                           `json:"linkedLibraryDescriptorSetHash"`
	EndpointIdentitySetHash        string                           `json:"endpointIdentitySetHash"`
	SignerProtocolID               string                           `json:"signerProtocolID"`
	ReservationProtocolID          string                           `json:"reservationProtocolID"`
	EvidenceProtocolID             string                           `json:"evidenceProtocolID"`
	BitcoinOutboxProtocolID        string                           `json:"bitcoinOutboxProtocolID"`
	InventoryProtocolID            string                           `json:"inventoryProtocolID"`
	QuarantineProtocolID           string                           `json:"quarantineProtocolID"`
	LiftProtocolID                 string                           `json:"liftProtocolID"`
	TombstoneProtocolID            string                           `json:"tombstoneProtocolID"`
	LiftAuthoritySetHash           string                           `json:"liftAuthoritySetHash"`
	CheckpointAuthoritySetHash     string                           `json:"checkpointAuthoritySetHash"`
	CheckpointMinimumSequence      uint64                           `json:"checkpointMinimumSequence"`
	CheckpointPredecessorHash      string                           `json:"checkpointPredecessorHash"`
	SigningPolicyHash              string                           `json:"signingPolicyHash"`
	CanonicalStoreID               string                           `json:"canonicalStoreID"`
	CanonicalStoreFingerprint      string                           `json:"canonicalStoreFingerprint"`
	CanonicalClusterFingerprint    string                           `json:"canonicalClusterFingerprint"`
	QuarantineStoreID              string                           `json:"quarantineStoreID"`
	QuarantineStoreFingerprint     string                           `json:"quarantineStoreFingerprint"`
	QuarantineClusterFingerprint   string                           `json:"quarantineClusterFingerprint"`
	SourceIdentity                 frostRetainedGroupWireIdentity   `json:"sourceIdentity"`
}

type frostRetainedGroupWireEventPoint struct {
	BlockNumber      uint64 `json:"blockNumber"`
	BlockHash        string `json:"blockHash"`
	TransactionHash  string `json:"transactionHash"`
	TransactionIndex uint32 `json:"transactionIndex"`
	LogIndex         uint32 `json:"logIndex"`
}

type frostRetainedGroupWireQuarantineRaisedRecord struct {
	QuarantineID     string                           `json:"quarantineID"`
	WalletID         string                           `json:"walletID"`
	EvidenceHash     string                           `json:"evidenceHash"`
	Reason           string                           `json:"reason"`
	RecoveryRequired bool                             `json:"recoveryRequired"`
	RaisedAt         frostRetainedGroupWireEventPoint `json:"raisedAt"`
}

type frostRetainedGroupWireQuarantineLiftBody struct {
	Schema                 string                                       `json:"schema"`
	ProtocolBindingHash    string                                       `json:"protocolBindingHash"`
	ManifestHash           string                                       `json:"manifestHash"`
	ProfileHash            string                                       `json:"profileHash"`
	ImplementationSetHash  string                                       `json:"implementationSetHash"`
	ChainID                uint64                                       `json:"chainID"`
	DomainChainID          string                                       `json:"domainChainID"`
	GenesisBlockHash       string                                       `json:"genesisBlockHash"`
	QuarantineProtocolID   string                                       `json:"quarantineProtocolID"`
	LiftProtocolID         string                                       `json:"liftProtocolID"`
	TombstoneProtocolID    string                                       `json:"tombstoneProtocolID"`
	AuthoritySetHash       string                                       `json:"authoritySetHash"`
	QuarantineID           string                                       `json:"quarantineID"`
	WalletID               string                                       `json:"walletID"`
	OriginalRaisedRecord   frostRetainedGroupWireQuarantineRaisedRecord `json:"originalRaisedRecord"`
	PriorGeneration        uint64                                       `json:"priorGeneration"`
	PriorEventRoot         string                                       `json:"priorEventRoot"`
	PriorActiveRoot        string                                       `json:"priorActiveRoot"`
	PriorTombstoneRoot     string                                       `json:"priorTombstoneRoot"`
	LiftPoint              frostRetainedGroupWireEventPoint             `json:"liftPoint"`
	ResolutionEvidenceHash string                                       `json:"resolutionEvidenceHash"`
	ResolutionFinality     frostRetainedGroupWireFinality               `json:"resolutionFinality"`
	NotBeforeBlock         uint64                                       `json:"notBeforeBlock"`
	ExpiresAtBlock         uint64                                       `json:"expiresAtBlock"`
}

type frostRetainedGroupWireQuarantineLiftSignature struct {
	AuthorityID         string `json:"authorityID"`
	SignerPublicKeySPKI string `json:"signerPublicKeySpki"`
	Signature           string `json:"signature"`
}

type frostRetainedGroupWireQuarantineLiftCertificate struct {
	Schema     string                                          `json:"schema"`
	Body       frostRetainedGroupWireQuarantineLiftBody        `json:"body"`
	BodyHash   string                                          `json:"bodyHash"`
	Signatures []frostRetainedGroupWireQuarantineLiftSignature `json:"signatures"`
}

type frostRetainedGroupWireMutation struct {
	Point                   frostRetainedGroupWireEventPoint                 `json:"point"`
	Kind                    string                                           `json:"kind"`
	WalletID                string                                           `json:"walletID"`
	WalletPublicKeyHash     string                                           `json:"walletPublicKeyHash"`
	OperatorIDs             []uint32                                         `json:"operatorIDs"`
	RetainedGroupHash       string                                           `json:"retainedGroupHash"`
	DkgResultHash           string                                           `json:"dkgResultHash"`
	DkgSubmissionPoint      frostRetainedGroupWireEventPoint                 `json:"dkgSubmissionPoint"`
	DkgApprovalPoint        frostRetainedGroupWireEventPoint                 `json:"dkgApprovalPoint"`
	CreationPoint           frostRetainedGroupWireEventPoint                 `json:"creationPoint"`
	BridgeRegistrationPoint frostRetainedGroupWireEventPoint                 `json:"bridgeRegistrationPoint"`
	QuarantineID            string                                           `json:"quarantineID"`
	EvidenceHash            string                                           `json:"evidenceHash"`
	LiftCertificateHash     string                                           `json:"liftCertificateHash"`
	LiftCertificate         *frostRetainedGroupWireQuarantineLiftCertificate `json:"liftCertificate"`
	Reason                  string                                           `json:"reason"`
}

type frostRetainedGroupHistoryQuery struct {
	Schema      string                         `json:"schema"`
	BindingHash string                         `json:"bindingHash"`
	From        frostRetainedGroupWireFinality `json:"from"`
	To          frostRetainedGroupWireFinality `json:"to"`
}

type frostRetainedGroupHistoryPageRequest struct {
	BindingHash     string                                 `json:"bindingHash"`
	Query           frostRetainedGroupHistoryQuery         `json:"query"`
	CheckpointAfter frostRetainedGroupWireCheckpointCursor `json:"checkpointAfter"`
	Cursor          string                                 `json:"cursor"`
}

type frostRetainedGroupWireCheckpointCursor struct {
	Sequence        uint64 `json:"sequence"`
	CertificateHash string `json:"certificateHash"`
}

type frostRetainedGroupHistoryReceipt struct {
	PageCount              uint64                                        `json:"pageCount"`
	MutationCount          uint64                                        `json:"mutationCount"`
	BindingHash            string                                        `json:"bindingHash"`
	HistoryRoot            string                                        `json:"historyRoot"`
	CheckpointAfter        frostRetainedGroupWireCheckpointCursor        `json:"checkpointAfter"`
	CheckpointCertificates []frostRetainedGroupWireCheckpointCertificate `json:"checkpointCertificates"`
	CheckpointChainRoot    string                                        `json:"checkpointChainRoot"`
	CheckpointTipHash      string                                        `json:"checkpointTipHash"`
	CheckpointComplete     bool                                          `json:"checkpointComplete"`
}

type frostRetainedGroupHistoryPagePayload struct {
	Schema            string                                 `json:"schema"`
	BindingHash       string                                 `json:"bindingHash"`
	Identity          frostRetainedGroupWireIdentity         `json:"identity"`
	ChainID           uint64                                 `json:"chainID"`
	QueryHash         string                                 `json:"queryHash"`
	SnapshotID        string                                 `json:"snapshotID"`
	PageIndex         uint64                                 `json:"pageIndex"`
	Cursor            string                                 `json:"cursor"`
	PreviousPageHash  string                                 `json:"previousPageHash"`
	From              frostRetainedGroupWireFinality         `json:"from"`
	To                frostRetainedGroupWireFinality         `json:"to"`
	EmptyAtFrom       bool                                   `json:"emptyAtFrom"`
	DescriptorSetHash string                                 `json:"descriptorSetHash"`
	CheckpointAfter   frostRetainedGroupWireCheckpointCursor `json:"checkpointAfter"`
	Mutations         []frostRetainedGroupWireMutation       `json:"mutations"`
	NextCursor        string                                 `json:"nextCursor"`
	Complete          bool                                   `json:"complete"`
	Receipt           *frostRetainedGroupHistoryReceipt      `json:"receipt"`
}

type frostRetainedGroupOperatorQuery struct {
	Schema          string                         `json:"schema"`
	BindingHash     string                         `json:"bindingHash"`
	OperatorAddress string                         `json:"operatorAddress"`
	At              frostRetainedGroupWireFinality `json:"at"`
}

type frostRetainedGroupOperatorReceiptPayload struct {
	Schema          string                         `json:"schema"`
	BindingHash     string                         `json:"bindingHash"`
	Identity        frostRetainedGroupWireIdentity `json:"identity"`
	ChainID         uint64                         `json:"chainID"`
	QueryHash       string                         `json:"queryHash"`
	OperatorAddress string                         `json:"operatorAddress"`
	At              frostRetainedGroupWireFinality `json:"at"`
	OperatorID      uint32                         `json:"operatorID"`
	Found           bool                           `json:"found"`
}

// FrostRetainedGroupHistoryEndpointFingerprint returns the complete identity
// committed by the activation manifest. The caller must supply the explicit
// endpoint descriptors; URL strings alone are not an authenticated endpoint
// identity.
func FrostRetainedGroupHistoryEndpointFingerprint(
	identity FrostRetainedGroupHistoryIdentity,
) ([32]byte, error) {
	if err := validateFrostRetainedGroupHistoryIdentity(identity); err != nil {
		return [32]byte{}, err
	}
	return computeFrostRetainedGroupSourceEndpointFingerprint(identity), nil
}

// NewFrostRetainedGroupHistorySource creates the production source. It fails
// closed if the verifier is not a distinct endpoint, does not expose finalized
// blocks, or serves a different chain.
func NewFrostRetainedGroupHistorySource(
	ctx context.Context,
	config FrostRetainedGroupHistorySourceConfig,
	primaryTransport *FrostPrimaryEthereumTransport,
	expectedChainID uint64,
) (*signedFrostRetainedGroupHistorySource, error) {
	if ctx == nil {
		return nil, fmt.Errorf("retained-group history context is nil")
	}
	validated, err := validateAndResolveFrostRetainedGroupSourceConfig(
		ctx,
		config,
		primaryTransport,
	)
	if err != nil {
		return nil, err
	}
	if expectedChainID == 0 {
		return nil, fmt.Errorf("retained-group history expected chain ID is zero")
	}
	separationPolicy, err := primaryTransport.bindRetainedEndpoints(
		validated.exportEndpoint,
		validated.identity.Export,
		validated.verifierEndpoint,
		validated.identity.Verifier,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot bind primary Ethereum transport to retained endpoints: [%w]",
			err,
		)
	}
	rpcHTTPClient, rpcTransport, err :=
		newFrostRetainedGroupAttestedHTTPClientWithSeparationPolicy(
			validated.verifierEndpoint,
			validated.identity.Verifier,
			validated.rootCAs,
			validated.requestTimeout,
			separationPolicy,
		)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot configure independent retained-group Ethereum verifier transport: [%w]",
			err,
		)
	}
	rpcClient, err := rpc.DialOptions(
		ctx,
		validated.verifierEndpoint.canonical,
		rpc.WithHTTPClient(rpcHTTPClient),
	)
	if err != nil {
		rpcTransport.CloseIdleConnections()
		return nil, fmt.Errorf("cannot connect independent retained-group Ethereum verifier: [%w]", err)
	}
	verifier := &canonicalFrostRetainedGroupEthereumVerifier{
		Client:    ethclient.NewClient(rpcClient),
		rpcClient: rpcClient,
	}
	exportHTTPClient, exportTransport, err :=
		newFrostRetainedGroupAttestedHTTPClientWithSeparationPolicy(
			validated.exportEndpoint,
			validated.identity.Export,
			validated.rootCAs,
			validated.requestTimeout,
			separationPolicy,
		)
	if err != nil {
		verifier.Close()
		rpcTransport.CloseIdleConnections()
		return nil, fmt.Errorf(
			"cannot configure retained-group export transport: [%w]",
			err,
		)
	}
	source, err := newSignedFrostRetainedGroupHistorySource(
		ctx,
		validated.exportEndpoint.endpoint,
		verifier,
		exportHTTPClient,
		expectedChainID,
		validated.identity,
		validated.identity.HistorySignerKeyHash,
		validated.requestTimeout,
		&frostRetainedGroupIndependenceMonitor{
			exportEndpoint:   validated.exportEndpoint,
			verifierEndpoint: validated.verifierEndpoint,
			primaryTransport: primaryTransport,
		},
	)
	if err != nil {
		verifier.Close()
		rpcTransport.CloseIdleConnections()
		exportTransport.CloseIdleConnections()
		return nil, err
	}
	source.httpTransports = []*http.Transport{exportTransport, rpcTransport}
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
	requestTimeout time.Duration,
	independenceMonitor *frostRetainedGroupIndependenceMonitor,
) (*signedFrostRetainedGroupHistorySource, error) {
	if exportEndpoint == nil || verifier == nil || httpClient == nil || chainID == 0 ||
		validateFrostRetainedGroupHistoryIdentity(identity) != nil ||
		signerHash == [32]byte{} ||
		identity.HistorySignerKeyHash != signerHash ||
		independenceMonitor == nil ||
		requestTimeout < time.Second || requestTimeout > time.Minute {
		return nil, fmt.Errorf("retained-group history source configuration is incomplete")
	}
	if err := independenceMonitor.verify(ctx); err != nil {
		return nil, err
	}
	requestContext, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	actualChainID, err := verifier.ChainID(requestContext)
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
		independenceMonitor:  independenceMonitor,
		chainID:              chainID,
		identity:             identity,
		trustedSignerKeyHash: signerHash,
		maximumPages:         frostRetainedGroupMaximumPages,
		maximumMutations:     frostRetainedGroupMaximumMutations,
		maximumResponseBytes: frostRetainedGroupMaximumAggregateResponseBytes,
		maximumUniqueBlocks:  frostRetainedGroupMaximumUniqueBlocks,
		maximumReadDuration:  frostRetainedGroupMaximumReconciliationDuration,
		requestTimeout:       requestTimeout,
	}
	if _, err := source.FinalizedHead(ctx); err != nil {
		return nil, fmt.Errorf("independent retained-group Ethereum verifier has no usable finalized head: [%w]", err)
	}
	return source, nil
}

func (source *signedFrostRetainedGroupHistorySource) Close() {
	if source == nil {
		return
	}
	if source.verifier != nil {
		source.verifier.Close()
	}
	for _, transport := range source.httpTransports {
		if transport != nil {
			transport.CloseIdleConnections()
		}
	}
}

func (source *signedFrostRetainedGroupHistorySource) FrostPreSignEthereumEvidenceVerifier(
	ctx context.Context,
) (FrostPreSignEthereumEvidenceVerifier, error) {
	if source == nil || ctx == nil || source.verifier == nil ||
		source.independenceMonitor == nil {
		return nil, fmt.Errorf(
			"independent FROST Ethereum verifier is unavailable",
		)
	}
	if err := source.independenceMonitor.verify(ctx); err != nil {
		return nil, fmt.Errorf(
			"independent FROST Ethereum verifier lost endpoint separation: [%w]",
			err,
		)
	}
	return source.verifier, nil
}

func (source *signedFrostRetainedGroupHistorySource) requestContext(
	ctx context.Context,
) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, source.requestTimeout)
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
	if err := source.independenceMonitor.verify(ctx); err != nil {
		return FrostPreSignFinality{}, err
	}
	requestContext, cancel := source.requestContext(ctx)
	defer cancel()
	header, err := source.verifier.HeaderByNumber(
		requestContext,
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
	requestContext, cancel := source.requestContext(ctx)
	defer cancel()
	header, err := source.verifier.HeaderByHash(
		requestContext,
		common.Hash(point.BlockHash),
	)
	if err != nil {
		return err
	}
	if header == nil || header.Number == nil || !header.Number.IsUint64() ||
		header.Number.Uint64() != point.BlockNumber || header.Hash() != common.Hash(point.BlockHash) {
		return fmt.Errorf("retained-group point does not match the independent canonical chain")
	}
	canonicalContext, canonicalCancel := source.requestContext(ctx)
	defer canonicalCancel()
	canonicalHeader, err := source.verifier.HeaderByNumber(
		canonicalContext,
		new(big.Int).SetUint64(point.BlockNumber),
	)
	if err != nil {
		return err
	}
	if canonicalHeader == nil || canonicalHeader.Number == nil ||
		!canonicalHeader.Number.IsUint64() ||
		canonicalHeader.Number.Uint64() != point.BlockNumber ||
		canonicalHeader.Hash() != common.Hash(point.BlockHash) {
		return fmt.Errorf("retained-group point is not canonical by height")
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
	checkpointAfter FrostRetainedGroupCheckpointCursor,
) (*FrostRetainedGroupHistory, error) {
	if ctx == nil {
		return nil, fmt.Errorf("retained-group history context is nil")
	}
	if from.BlockNumber == 0 || from.BlockHash == [32]byte{} ||
		to.BlockNumber < from.BlockNumber || to.BlockHash == [32]byte{} {
		return nil, fmt.Errorf("retained-group history bounds are invalid")
	}
	if source.maximumPages == 0 || source.maximumMutations == 0 ||
		source.maximumResponseBytes == 0 || source.maximumUniqueBlocks == 0 ||
		source.maximumReadDuration <= 0 {
		return nil, fmt.Errorf("retained-group history resource limits are invalid")
	}
	readContext, cancel := context.WithTimeout(ctx, source.maximumReadDuration)
	defer cancel()
	ctx = readContext
	evidence, err := source.activationEvidence()
	if err != nil {
		return nil, err
	}
	if checkpointAfter.Sequence == evidence.checkpointPolicy.MinimumSequence-1 {
		if checkpointAfter.CertificateHash !=
			evidence.checkpointPolicy.PredecessorHash {
			return nil, fmt.Errorf(
				"retained-group checkpoint cursor differs from the manifest floor",
			)
		}
	} else if checkpointAfter.Sequence <
		evidence.checkpointPolicy.MinimumSequence ||
		checkpointAfter.CertificateHash == [32]byte{} {
		return nil, fmt.Errorf(
			"retained-group checkpoint cursor is below the manifest floor",
		)
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
		Schema:      frostRetainedGroupHistoryRequestSchema,
		BindingHash: frostActivationHex32(evidence.bindingHash),
		From:        frostRetainedGroupFinalityToWire(from),
		To:          frostRetainedGroupFinalityToWire(to),
	}
	queryHash, err := frostRetainedGroupDomainHash(
		frostRetainedGroupHistoryQueryDomain,
		query,
	)
	if err != nil {
		return nil, err
	}

	mutations := make([]FrostRetainedGroupMutation, 0)
	mutationHashes := make([][32]byte, 0)
	seenCursors := make(map[string]bool)
	blockHashes := map[uint64][32]byte{
		from.BlockNumber: from.BlockHash,
		to.BlockNumber:   to.BlockHash,
	}
	cursor := ""
	var snapshotID [32]byte
	var descriptorSetHash [32]byte
	var previousPageHash [32]byte
	var aggregateResponseBytes uint64
	for pageIndex := uint64(0); pageIndex < source.maximumPages; pageIndex++ {
		if seenCursors[cursor] {
			return nil, fmt.Errorf("retained-group history cursor repeated")
		}
		seenCursors[cursor] = true
		request := frostRetainedGroupHistoryPageRequest{
			BindingHash: frostActivationHex32(evidence.bindingHash),
			Query:       query,
			CheckpointAfter: frostRetainedGroupWireCheckpointCursor{
				Sequence: checkpointAfter.Sequence,
				CertificateHash: frostActivationHex32(
					checkpointAfter.CertificateHash,
				),
			},
			Cursor: cursor,
		}
		payload := &frostRetainedGroupHistoryPagePayload{}
		responseBytes, err := source.postSigned(ctx, "history", request, payload)
		if err != nil {
			return nil, fmt.Errorf("cannot read retained-group history page [%d]: [%w]", pageIndex, err)
		}
		if responseBytes > source.maximumResponseBytes-aggregateResponseBytes {
			return nil, fmt.Errorf("retained-group history exceeds the aggregate response-byte limit")
		}
		aggregateResponseBytes += responseBytes
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
			checkpointAfter,
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
			canonicalMutation, err := canonicalFrostActivationValue(wireMutation)
			if err != nil {
				return nil, fmt.Errorf("cannot hash retained-group history mutation: [%w]", err)
			}
			mutation, err := frostRetainedGroupMutationFromWire(wireMutation)
			if err != nil {
				return nil, fmt.Errorf("retained-group history contains malformed mutation: [%w]", err)
			}
			if err := addFrostRetainedGroupMutationBlockHashes(blockHashes, mutation); err != nil {
				return nil, err
			}
			if uint64(len(blockHashes)) > source.maximumUniqueBlocks {
				return nil, fmt.Errorf("retained-group history exceeds the unique-block limit")
			}
			mutations = append(mutations, mutation)
			mutationHashes = append(
				mutationHashes,
				sha256.Sum256(canonicalMutation),
			)
			if uint64(len(mutations)) > source.maximumMutations {
				return nil, fmt.Errorf("retained-group history exceeds the mutation limit")
			}
		}
		if payload.Complete {
			if payload.Receipt == nil || payload.NextCursor != "" ||
				payload.Receipt.PageCount != pageIndex+1 ||
				payload.Receipt.MutationCount != uint64(len(mutations)) ||
				payload.Receipt.BindingHash !=
					frostActivationHex32(evidence.bindingHash) ||
				payload.Receipt.CheckpointAfter !=
					request.CheckpointAfter {
				return nil, fmt.Errorf("retained-group final history receipt is inconsistent")
			}
			receiptRoot, err := parseFrostActivationHex32(payload.Receipt.HistoryRoot)
			if err != nil {
				return nil, fmt.Errorf("retained-group history receipt root is invalid")
			}
			computedRoot := frostRetainedGroupHistoryRootFromHashes(
				evidence.bindingHash,
				queryHash,
				mutationHashes,
			)
			if receiptRoot != computedRoot {
				return nil, fmt.Errorf("retained-group history receipt does not cover the exact mutation sequence")
			}
			if len(payload.Receipt.CheckpointCertificates) >
				frostRetainedGroupMaximumCheckpointsPerPage {
				return nil, fmt.Errorf(
					"retained-group checkpoint page exceeds its bound",
				)
			}
			checkpoints := make(
				[]FrostRetainedGroupCheckpointCertificate,
				len(payload.Receipt.CheckpointCertificates),
			)
			for index, wireCertificate := range payload.Receipt.CheckpointCertificates {
				checkpoint, err :=
					frostRetainedGroupCheckpointCertificateFromWire(
						wireCertificate,
					)
				if err != nil {
					return nil, fmt.Errorf(
						"retained-group checkpoint certificate [%d] is malformed: [%w]",
						index,
						err,
					)
				}
				checkpoints[index] = checkpoint
			}
			checkpointHashes, err :=
				validateFrostRetainedGroupCheckpointSuffix(
					evidence.checkpointPolicy,
					checkpointAfter,
					checkpoints,
				)
			if err != nil {
				return nil, err
			}
			checkpointChainRoot, err := parseFrostActivationHex32(
				payload.Receipt.CheckpointChainRoot,
			)
			if err != nil ||
				checkpointChainRoot !=
					frostRetainedGroupCheckpointChainRoot(
						evidence.bindingHash,
						checkpointAfter,
						checkpointHashes,
					) {
				return nil, fmt.Errorf(
					"retained-group final receipt checkpoint-chain root mismatch",
				)
			}
			checkpointTipHash, err := parseFrostActivationHex32(
				payload.Receipt.CheckpointTipHash,
			)
			if err != nil ||
				(len(checkpointHashes) == 0 &&
					checkpointTipHash != checkpointAfter.CertificateHash) ||
				(len(checkpointHashes) > 0 &&
					checkpointTipHash !=
						checkpointHashes[len(checkpointHashes)-1]) {
				return nil, fmt.Errorf(
					"retained-group final receipt checkpoint-tip mismatch",
				)
			}
			history := &FrostRetainedGroupHistory{
				From:                from,
				To:                  to,
				Mutations:           mutations,
				HistoryRoot:         receiptRoot,
				CheckpointAfter:     checkpointAfter,
				Checkpoints:         checkpoints,
				CheckpointChainRoot: checkpointChainRoot,
				CheckpointTipHash:   checkpointTipHash,
				CheckpointComplete:  payload.Receipt.CheckpointComplete,
				Complete:            true,
				EmptyAtFrom:         true,
				DescriptorSetHash:   descriptorSetHash,
			}
			if err := validateCompleteFrostRetainedGroupHistory(
				history,
				evidence.liftPolicy,
			); err != nil {
				return nil, err
			}
			if !history.CheckpointComplete && len(checkpoints) == 0 {
				return nil, fmt.Errorf(
					"retained-group nonfinal checkpoint page made no progress",
				)
			}
			if len(checkpoints) > 0 {
				if err := validateFrostRetainedGroupCheckpointSemantics(
					evidence.checkpointPolicy,
					history,
					checkpointHashes,
				); err != nil {
					return nil, err
				}
			}
			for _, checkpoint := range checkpoints {
				blockNumber := checkpoint.Body.Point.BlockNumber
				blockHash := checkpoint.Body.Point.BlockHash
				if existing, ok := blockHashes[blockNumber]; ok &&
					existing != blockHash {
					return nil, fmt.Errorf(
						"retained-group checkpoint chain contains conflicting block hashes",
					)
				}
				blockHashes[blockNumber] = blockHash
				if uint64(len(blockHashes)) > source.maximumUniqueBlocks {
					return nil, fmt.Errorf(
						"retained-group history exceeds the unique-block limit",
					)
				}
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
	checkpointAfter FrostRetainedGroupCheckpointCursor,
) ([32]byte, error) {
	evidence, evidenceErr := source.activationEvidence()
	if evidenceErr != nil {
		return [32]byte{}, evidenceErr
	}
	if payload == nil || payload.Schema != frostRetainedGroupHistoryPageSchema ||
		payload.BindingHash != frostActivationHex32(evidence.bindingHash) ||
		payload.ChainID != source.chainID || payload.PageIndex != pageIndex ||
		payload.Cursor != cursor || !payload.EmptyAtFrom ||
		!source.validWireIdentity(payload.Identity) {
		return [32]byte{}, fmt.Errorf("retained-group history page has the wrong identity or position")
	}
	if payload.CheckpointAfter.Sequence != checkpointAfter.Sequence ||
		payload.CheckpointAfter.CertificateHash !=
			frostActivationHex32(checkpointAfter.CertificateHash) {
		return [32]byte{}, fmt.Errorf(
			"retained-group history page checkpoint cursor mismatch",
		)
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
	if err != nil || pageDescriptorSetHash != evidence.descriptorSetHash ||
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
	if uint64(len(payload.Mutations)) > source.maximumMutations {
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
	resolveContext, cancel := context.WithTimeout(ctx, source.maximumReadDuration)
	defer cancel()
	ctx = resolveContext
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
		BindingHash:     frostActivationHex32(evidence.bindingHash),
		OperatorAddress: canonicalAddress,
		At:              frostRetainedGroupFinalityToWire(at),
	}
	queryHash, err := frostRetainedGroupDomainHash(frostRetainedGroupOperatorQueryDomain, query)
	if err != nil {
		return 0, err
	}
	payload := &frostRetainedGroupOperatorReceiptPayload{}
	if _, err := source.postSigned(ctx, "operator-id", query, payload); err != nil {
		return 0, fmt.Errorf("cannot resolve retained-group operator ID: [%w]", err)
	}
	declaredQueryHash, err := parseFrostActivationHex32(payload.QueryHash)
	receiptAt, pointErr := frostRetainedGroupFinalityFromWire(payload.At)
	if payload.Schema != frostRetainedGroupOperatorReceiptSchema ||
		payload.BindingHash != frostActivationHex32(evidence.bindingHash) ||
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
) (uint64, error) {
	if err := source.independenceMonitor.verify(ctx); err != nil {
		return 0, err
	}
	requestContext, cancel := source.requestContext(ctx)
	defer cancel()
	requestBody, err := json.Marshal(requestPayload)
	if err != nil {
		return 0, err
	}
	endpoint := *source.exportEndpoint
	endpoint.Path = path.Join(endpoint.Path, operation)
	request, err := http.NewRequestWithContext(
		requestContext,
		http.MethodPost,
		endpoint.String(),
		bytes.NewReader(requestBody),
	)
	if err != nil {
		return 0, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	response, err := source.httpClient.Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	if err := requireFrostRetainedGroupTransportProof(
		response,
		source.identity.Export.Role,
	); err != nil {
		return 0, err
	}
	if response.StatusCode != http.StatusOK {
		return 0, &frostRetainedGroupHistoryStatusError{
			statusCode: response.StatusCode,
		}
	}
	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return 0, fmt.Errorf("retained-group export response is not application/json")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, frostRetainedGroupMaximumResponseBytes+1))
	if err != nil {
		return 0, err
	}
	if len(data) == 0 || len(data) > frostRetainedGroupMaximumResponseBytes {
		return 0, fmt.Errorf("retained-group export response size is invalid")
	}
	envelope := &frostRetainedGroupSignedEnvelope{}
	if err := decodeStrictFrostActivationJSON(data, envelope); err != nil {
		return 0, fmt.Errorf("cannot decode retained-group signed envelope: [%w]", err)
	}
	if err := source.verifySignedEnvelope(envelope, responsePayload); err != nil {
		return 0, err
	}
	return uint64(len(data)), nil
}

func (source *signedFrostRetainedGroupHistorySource) verifySignedEnvelope(
	envelope *frostRetainedGroupSignedEnvelope,
	target interface{},
) error {
	evidence, evidenceErr := source.activationEvidence()
	if envelope == nil || evidenceErr != nil ||
		envelope.Schema != "tbtc-frost-retained-group-signed-envelope/v3" ||
		envelope.BindingHash != frostActivationHex32(evidence.bindingHash) ||
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
	publicKeyDER, err := decodeCanonicalFrostRetainedGroupBase64(
		envelope.SignerPublicKeySPKI,
	)
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
	signature, err := decodeCanonicalFrostRetainedGroupBase64(
		envelope.Signature,
	)
	signed := append([]byte(frostRetainedGroupHistorySignatureDomain), canonical...)
	if err != nil || len(signature) != ed25519.SignatureSize ||
		!ed25519.Verify(publicKey, signed, signature) {
		return fmt.Errorf("retained-group export signature is invalid")
	}
	if err := decodeStrictFrostActivationJSON(canonical, target); err != nil {
		return fmt.Errorf("cannot decode retained-group signed payload: [%w]", err)
	}
	return nil
}

func (source *signedFrostRetainedGroupHistorySource) validWireIdentity(
	identity frostRetainedGroupWireIdentity,
) bool {
	parsed, err := frostRetainedGroupIdentityFromWire(identity)
	return err == nil && parsed == source.identity
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
	bindingHash [32]byte,
	queryHash [32]byte,
	mutations []frostRetainedGroupWireMutation,
) ([32]byte, error) {
	hashes := make([][32]byte, 0, len(mutations))
	for _, mutation := range mutations {
		canonical, err := canonicalFrostActivationValue(mutation)
		if err != nil {
			return [32]byte{}, err
		}
		hashes = append(hashes, sha256.Sum256(canonical))
	}
	return frostRetainedGroupHistoryRootFromHashes(
		bindingHash,
		queryHash,
		hashes,
	), nil
}

func frostRetainedGroupHistoryRootFromHashes(
	bindingHash [32]byte,
	queryHash [32]byte,
	mutationHashes [][32]byte,
) [32]byte {
	hasher := sha256.New()
	hasher.Write([]byte(frostRetainedGroupHistoryRootDomain))
	hasher.Write(bindingHash[:])
	hasher.Write(queryHash[:])
	count := make([]byte, 8)
	for i := uint(0); i < 8; i++ {
		count[7-i] = byte(uint64(len(mutationHashes)) >> (i * 8))
	}
	hasher.Write(count)
	for _, mutationHash := range mutationHashes {
		hasher.Write(mutationHash[:])
	}
	var result [32]byte
	copy(result[:], hasher.Sum(nil))
	return result
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
	liftCertificateHash, err := parseFrostActivationHex32(
		mutation.LiftCertificateHash,
	)
	if err != nil {
		return FrostRetainedGroupMutation{}, err
	}
	liftCertificate, err := frostRetainedGroupLiftCertificateFromWire(
		mutation.LiftCertificate,
	)
	if err != nil {
		return FrostRetainedGroupMutation{}, err
	}
	if len(mutation.OperatorIDs) > 100 || len(mutation.Reason) > frostRetainedGroupMaximumReasonBytes {
		return FrostRetainedGroupMutation{}, fmt.Errorf("retained-group mutation exceeds field bounds")
	}
	operatorIDs := append([]uint32{}, mutation.OperatorIDs...)
	for _, operatorID := range operatorIDs {
		if operatorID == 0 {
			return FrostRetainedGroupMutation{}, fmt.Errorf("retained-group mutation has a zero operator ID")
		}
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
		LiftCertificateHash:     liftCertificateHash,
		LiftCertificate:         liftCertificate,
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
		LiftCertificateHash:     frostActivationHex32(mutation.LiftCertificateHash),
		LiftCertificate:         frostRetainedGroupLiftCertificateToWire(mutation.LiftCertificate),
		Reason:                  mutation.Reason,
	}
}

func frostRetainedGroupLiftCertificateToWire(
	certificate *FrostRetainedGroupQuarantineLiftCertificate,
) *frostRetainedGroupWireQuarantineLiftCertificate {
	if certificate == nil {
		return nil
	}
	body := certificate.Body
	signatures := make(
		[]frostRetainedGroupWireQuarantineLiftSignature,
		len(certificate.Signatures),
	)
	for index, signature := range certificate.Signatures {
		signatures[index] = frostRetainedGroupWireQuarantineLiftSignature{
			AuthorityID:         signature.AuthorityID,
			SignerPublicKeySPKI: signature.SignerPublicKeySPKI,
			Signature:           signature.Signature,
		}
	}
	return &frostRetainedGroupWireQuarantineLiftCertificate{
		Schema: certificate.Schema,
		Body: frostRetainedGroupWireQuarantineLiftBody{
			Schema:                body.Schema,
			ProtocolBindingHash:   frostActivationHex32(body.ProtocolBindingHash),
			ManifestHash:          frostActivationHex32(body.ManifestHash),
			ProfileHash:           frostActivationHex32(body.ProfileHash),
			ImplementationSetHash: frostActivationHex32(body.ImplementationSetHash),
			ChainID:               body.ChainID,
			DomainChainID:         frostActivationHex32(body.DomainChainID),
			GenesisBlockHash:      frostActivationHex32(body.GenesisBlockHash),
			QuarantineProtocolID:  frostActivationHex32(body.QuarantineProtocolID),
			LiftProtocolID:        frostActivationHex32(body.LiftProtocolID),
			TombstoneProtocolID:   frostActivationHex32(body.TombstoneProtocolID),
			AuthoritySetHash:      frostActivationHex32(body.AuthoritySetHash),
			QuarantineID:          frostActivationHex32(body.QuarantineID),
			WalletID:              frostActivationHex32(body.WalletID),
			OriginalRaisedRecord: frostRetainedGroupWireQuarantineRaisedRecord{
				QuarantineID: frostActivationHex32(
					body.OriginalRaisedRecord.QuarantineID,
				),
				WalletID: frostActivationHex32(
					body.OriginalRaisedRecord.WalletID,
				),
				EvidenceHash: frostActivationHex32(
					body.OriginalRaisedRecord.EvidenceHash,
				),
				Reason:           body.OriginalRaisedRecord.Reason,
				RecoveryRequired: body.OriginalRaisedRecord.RecoveryRequired,
				RaisedAt: frostRetainedGroupEventPointToWire(
					body.OriginalRaisedRecord.RaisedAt,
				),
			},
			PriorGeneration:        body.PriorGeneration,
			PriorEventRoot:         frostActivationHex32(body.PriorEventRoot),
			PriorActiveRoot:        frostActivationHex32(body.PriorActiveRoot),
			PriorTombstoneRoot:     frostActivationHex32(body.PriorTombstoneRoot),
			LiftPoint:              frostRetainedGroupEventPointToWire(body.LiftPoint),
			ResolutionEvidenceHash: frostActivationHex32(body.ResolutionEvidenceHash),
			ResolutionFinality: frostRetainedGroupFinalityToWire(
				body.ResolutionFinality,
			),
			NotBeforeBlock: body.NotBeforeBlock,
			ExpiresAtBlock: body.ExpiresAtBlock,
		},
		BodyHash:   frostActivationHex32(certificate.BodyHash),
		Signatures: signatures,
	}
}

func frostRetainedGroupLiftCertificateFromWire(
	certificate *frostRetainedGroupWireQuarantineLiftCertificate,
) (*FrostRetainedGroupQuarantineLiftCertificate, error) {
	if certificate == nil {
		return nil, nil
	}
	body := certificate.Body
	parse := func(name string, value string) ([32]byte, error) {
		parsed, err := parseFrostActivationHex32(value)
		if err != nil {
			return [32]byte{}, fmt.Errorf(
				"invalid FROST quarantine lift %s: [%w]",
				name,
				err,
			)
		}
		return parsed, nil
	}
	protocolBindingHash, err := parse(
		"protocol binding hash",
		body.ProtocolBindingHash,
	)
	if err != nil {
		return nil, err
	}
	manifestHash, err := parse("manifest hash", body.ManifestHash)
	if err != nil {
		return nil, err
	}
	profileHash, err := parse("profile hash", body.ProfileHash)
	if err != nil {
		return nil, err
	}
	implementationSetHash, err := parse(
		"implementation set hash",
		body.ImplementationSetHash,
	)
	if err != nil {
		return nil, err
	}
	domainChainID, err := parse("domain chain ID", body.DomainChainID)
	if err != nil {
		return nil, err
	}
	genesisBlockHash, err := parse("genesis block hash", body.GenesisBlockHash)
	if err != nil {
		return nil, err
	}
	quarantineProtocolID, err := parse(
		"quarantine protocol ID",
		body.QuarantineProtocolID,
	)
	if err != nil {
		return nil, err
	}
	liftProtocolID, err := parse("lift protocol ID", body.LiftProtocolID)
	if err != nil {
		return nil, err
	}
	tombstoneProtocolID, err := parse(
		"tombstone protocol ID",
		body.TombstoneProtocolID,
	)
	if err != nil {
		return nil, err
	}
	authoritySetHash, err := parse("authority set hash", body.AuthoritySetHash)
	if err != nil {
		return nil, err
	}
	quarantineID, err := parse("quarantine ID", body.QuarantineID)
	if err != nil {
		return nil, err
	}
	walletID, err := parse("wallet ID", body.WalletID)
	if err != nil {
		return nil, err
	}
	raisedQuarantineID, err := parse(
		"raised quarantine ID",
		body.OriginalRaisedRecord.QuarantineID,
	)
	if err != nil {
		return nil, err
	}
	raisedWalletID, err := parse(
		"raised wallet ID",
		body.OriginalRaisedRecord.WalletID,
	)
	if err != nil {
		return nil, err
	}
	raisedEvidenceHash, err := parse(
		"raised evidence hash",
		body.OriginalRaisedRecord.EvidenceHash,
	)
	if err != nil {
		return nil, err
	}
	raisedAt, err := frostRetainedGroupEventPointFromWire(
		body.OriginalRaisedRecord.RaisedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("invalid FROST quarantine raised point: [%w]", err)
	}
	priorEventRoot, err := parse("prior event root", body.PriorEventRoot)
	if err != nil {
		return nil, err
	}
	priorActiveRoot, err := parse("prior active root", body.PriorActiveRoot)
	if err != nil {
		return nil, err
	}
	priorTombstoneRoot, err := parse(
		"prior tombstone root",
		body.PriorTombstoneRoot,
	)
	if err != nil {
		return nil, err
	}
	liftPoint, err := frostRetainedGroupEventPointFromWire(body.LiftPoint)
	if err != nil {
		return nil, fmt.Errorf("invalid FROST quarantine lift point: [%w]", err)
	}
	resolutionEvidenceHash, err := parse(
		"resolution evidence hash",
		body.ResolutionEvidenceHash,
	)
	if err != nil {
		return nil, err
	}
	resolutionFinality, err := frostRetainedGroupFinalityFromWire(
		body.ResolutionFinality,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"invalid FROST quarantine resolution finality: [%w]",
			err,
		)
	}
	bodyHash, err := parse("body hash", certificate.BodyHash)
	if err != nil {
		return nil, err
	}
	signatures := make(
		[]FrostRetainedGroupQuarantineLiftSignature,
		len(certificate.Signatures),
	)
	for index, signature := range certificate.Signatures {
		signatures[index] = FrostRetainedGroupQuarantineLiftSignature{
			AuthorityID:         signature.AuthorityID,
			SignerPublicKeySPKI: signature.SignerPublicKeySPKI,
			Signature:           signature.Signature,
		}
	}
	return &FrostRetainedGroupQuarantineLiftCertificate{
		Schema: certificate.Schema,
		Body: FrostRetainedGroupQuarantineLiftBody{
			Schema:                body.Schema,
			ProtocolBindingHash:   protocolBindingHash,
			ManifestHash:          manifestHash,
			ProfileHash:           profileHash,
			ImplementationSetHash: implementationSetHash,
			ChainID:               body.ChainID,
			DomainChainID:         domainChainID,
			GenesisBlockHash:      genesisBlockHash,
			QuarantineProtocolID:  quarantineProtocolID,
			LiftProtocolID:        liftProtocolID,
			TombstoneProtocolID:   tombstoneProtocolID,
			AuthoritySetHash:      authoritySetHash,
			QuarantineID:          quarantineID,
			WalletID:              walletID,
			OriginalRaisedRecord: FrostRetainedGroupQuarantineRaisedRecord{
				QuarantineID:     raisedQuarantineID,
				WalletID:         raisedWalletID,
				EvidenceHash:     raisedEvidenceHash,
				Reason:           body.OriginalRaisedRecord.Reason,
				RecoveryRequired: body.OriginalRaisedRecord.RecoveryRequired,
				RaisedAt:         raisedAt,
			},
			PriorGeneration:        body.PriorGeneration,
			PriorEventRoot:         priorEventRoot,
			PriorActiveRoot:        priorActiveRoot,
			PriorTombstoneRoot:     priorTombstoneRoot,
			LiftPoint:              liftPoint,
			ResolutionEvidenceHash: resolutionEvidenceHash,
			ResolutionFinality:     resolutionFinality,
			NotBeforeBlock:         body.NotBeforeBlock,
			ExpiresAtBlock:         body.ExpiresAtBlock,
		},
		BodyHash:   bodyHash,
		Signatures: signatures,
	}, nil
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
	if mutation.LiftCertificate != nil {
		finality := mutation.LiftCertificate.Body.ResolutionFinality
		if finality.BlockNumber == 0 || finality.BlockHash == [32]byte{} {
			return fmt.Errorf(
				"retained-group quarantine lift resolution finality is invalid",
			)
		}
		if existing, ok := blocks[finality.BlockNumber]; ok &&
			existing != finality.BlockHash {
			return fmt.Errorf(
				"retained-group history contains conflicting hashes for block [%d]",
				finality.BlockNumber,
			)
		}
		blocks[finality.BlockNumber] = finality.BlockHash
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
