package ethereum

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"
	"os"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	geth "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	tbtcabi "github.com/keep-network/keep-core/pkg/chain/ethereum/tbtc/gen/abi"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

const (
	frostPreSignManifestVersion = "tbtc-p2tr-fraud-production-activation/v5"

	frostPreSignFinalityAgreementAttempts   = 4
	frostPreSignFinalityAgreementRetryDelay = time.Second
)

const frostPreSignBridgeABIJSON = `[
 {"type":"function","name":"previewP2TRTransactionAuthorization","stateMutability":"view","inputs":[{"name":"payload","type":"bytes"}],"outputs":[{"name":"","type":"bytes"}]},
 {"type":"function","name":"authorizeP2TRTransaction","stateMutability":"nonpayable","inputs":[{"name":"payload","type":"bytes"}],"outputs":[{"name":"","type":"bytes"}]},
 {"type":"function","name":"p2trFraudRouter","stateMutability":"view","inputs":[],"outputs":[{"name":"","type":"address"}]},
 {"type":"function","name":"frostLifecycleContext","stateMutability":"view","inputs":[{"name":"walletPubKeyHash","type":"bytes20"}],"outputs":[{"name":"frostRegistry","type":"address"},{"name":"walletID","type":"bytes32"}]},
 {"type":"function","name":"walletID","stateMutability":"view","inputs":[{"name":"walletPubKeyHash","type":"bytes20"}],"outputs":[{"name":"","type":"bytes32"}]},
 {"type":"function","name":"wallets","stateMutability":"view","inputs":[{"name":"walletPubKeyHash","type":"bytes20"}],"outputs":[{"name":"","type":"tuple","components":[{"name":"ecdsaWalletID","type":"bytes32"},{"name":"mainUtxoHash","type":"bytes32"},{"name":"pendingRedemptionsValue","type":"uint64"},{"name":"createdAt","type":"uint32"},{"name":"movingFundsRequestedAt","type":"uint32"},{"name":"closingStartedAt","type":"uint32"},{"name":"pendingMovedFundsSweepRequestsCount","type":"uint32"},{"name":"state","type":"uint8"},{"name":"movingFundsTargetWalletsCommitmentHash","type":"bytes32"}]}]
 }]
`

const frostPreSignRegistryABIJSON = `[
 {"type":"function","name":"protocolConfig","stateMutability":"view","inputs":[],"outputs":[{"name":"bridgeAddress","type":"address"},{"name":"frostRegistryAddress","type":"address"},{"name":"proposalValidatorAddress","type":"address"},{"name":"chainID","type":"uint256"},{"name":"protocolID","type":"bytes32"},{"name":"policyHash","type":"bytes32"}]},
 {"type":"function","name":"getReservation","stateMutability":"view","inputs":[{"name":"id","type":"bytes32"}],"outputs":[{"name":"walletID","type":"bytes32"},{"name":"walletPubKeyHash","type":"bytes20"},{"name":"membersIDsHash","type":"bytes32"},{"name":"snapshotHash","type":"bytes32"},{"name":"resourceHash","type":"bytes32"},{"name":"orderedInputRoot","type":"bytes32"},{"name":"applyPlanData1","type":"bytes32"},{"name":"applyPlanData2","type":"bytes32"},{"name":"feeLimitSnapshot","type":"uint64"},{"name":"action","type":"uint8"},{"name":"status","type":"uint8"}]},
 {"type":"function","name":"getAuthorizedVariantStatus","stateMutability":"view","inputs":[{"name":"transactionHash","type":"bytes32"}],"outputs":[{"name":"reservationID","type":"bytes32"},{"name":"authorizationRoot","type":"bytes32"},{"name":"applyPlanHash","type":"bytes32"},{"name":"authorizationSequence","type":"uint256"},{"name":"fraudDefenseAuthorized","type":"bool"},{"name":"signingAllowed","type":"bool"}]},
 {"type":"function","name":"latestAuthorizedVariant","stateMutability":"view","inputs":[{"name":"reservationID","type":"bytes32"}],"outputs":[{"name":"transactionHash","type":"bytes32"},{"name":"authorizationSequence","type":"uint256"},{"name":"signingAllowed","type":"bool"}]},
 {"type":"function","name":"activeReservation","stateMutability":"view","inputs":[{"name":"walletPubKeyHash","type":"bytes20"}],"outputs":[{"name":"","type":"bytes32"}]},
 {"type":"function","name":"preAuthorizationDigest","stateMutability":"view","inputs":[{"name":"authorization","type":"tuple","components":[{"name":"action","type":"uint8"},{"name":"walletPubKeyHash","type":"bytes20"},{"name":"walletID","type":"bytes32"},{"name":"membersIDsHash","type":"bytes32"},{"name":"snapshotHash","type":"bytes32"},{"name":"resourceHash","type":"bytes32"},{"name":"orderedInputRoot","type":"bytes32"},{"name":"applyPlanHash","type":"bytes32"},{"name":"applyPlanData1","type":"bytes32"},{"name":"applyPlanData2","type":"bytes32"},{"name":"feeLimitSnapshot","type":"uint64"}]},{"name":"transactionHash","type":"bytes32"},{"name":"authorizationRoot","type":"bytes32"}],"outputs":[{"name":"","type":"bytes32"}]},
 {"type":"event","name":"P2TRPreSigningReservationAuthorized","anonymous":false,"inputs":[{"indexed":true,"name":"reservationID","type":"bytes32"},{"indexed":true,"name":"transactionHash","type":"bytes32"},{"indexed":true,"name":"walletID","type":"bytes32"},{"indexed":false,"name":"authorizationRoot","type":"bytes32"},{"indexed":false,"name":"snapshotHash","type":"bytes32"},{"indexed":false,"name":"resourceHash","type":"bytes32"},{"indexed":false,"name":"action","type":"uint8"}]},
 {"type":"event","name":"P2TRAuthorizedVariantAdvanced","anonymous":false,"inputs":[{"indexed":true,"name":"reservationID","type":"bytes32"},{"indexed":true,"name":"transactionHash","type":"bytes32"},{"indexed":true,"name":"authorizationSequence","type":"uint256"}]}
]`

const frostPreSignCrosslinkABIJSON = `[
 {"type":"function","name":"bridge","stateMutability":"view","inputs":[],"outputs":[{"name":"","type":"address"}]},
 {"type":"function","name":"authorizationRegistry","stateMutability":"view","inputs":[],"outputs":[{"name":"","type":"address"}]},
 {"type":"function","name":"evidenceProtocolID","stateMutability":"view","inputs":[],"outputs":[{"name":"","type":"bytes32"}]},
 {"type":"function","name":"preauthorizationProtocolID","stateMutability":"view","inputs":[],"outputs":[{"name":"","type":"bytes32"}]},
 {"type":"function","name":"signingPolicyHash","stateMutability":"view","inputs":[],"outputs":[{"name":"","type":"bytes32"}]},
 {"type":"function","name":"sortitionPool","stateMutability":"view","inputs":[],"outputs":[{"name":"","type":"address"}]},
 {"type":"function","name":"getOperatorID","stateMutability":"view","inputs":[{"name":"operator","type":"address"}],"outputs":[{"name":"","type":"uint32"}]},
 {"type":"function","name":"getWallet","stateMutability":"view","inputs":[{"name":"walletID","type":"bytes32"}],"outputs":[{"name":"","type":"tuple","components":[{"name":"membersIdsHash","type":"bytes32"},{"name":"xOnlyOutputKey","type":"bytes32"}]}]}
]`

const frostPreSignCodecABIJSON = `[
 {"type":"function","name":"previewPayload","inputs":[{"name":"action","type":"uint8"},{"name":"transaction","type":"tuple","components":[{"name":"version","type":"bytes4"},{"name":"inputVector","type":"bytes"},{"name":"outputVector","type":"bytes"},{"name":"locktime","type":"bytes4"}]},{"name":"actionData","type":"bytes"},{"name":"membersIDsHash","type":"bytes32"}],"outputs":[]},
 {"type":"function","name":"authorizePayload","inputs":[{"name":"action","type":"uint8"},{"name":"transaction","type":"tuple","components":[{"name":"version","type":"bytes4"},{"name":"inputVector","type":"bytes"},{"name":"outputVector","type":"bytes"},{"name":"locktime","type":"bytes4"}]},{"name":"actionData","type":"bytes"},{"name":"attestation","type":"tuple","components":[{"name":"walletMembersIDs","type":"uint32[]"},{"name":"signingMemberIndices","type":"uint8[]"},{"name":"signatures","type":"bytes"}]}],"outputs":[]},
 {"type":"function","name":"depositData","inputs":[{"name":"data","type":"tuple","components":[{"name":"proposal","type":"tuple","components":[{"name":"walletPubKeyHash","type":"bytes20"},{"name":"depositsKeys","type":"tuple[]","components":[{"name":"fundingTxHash","type":"bytes32"},{"name":"fundingOutputIndex","type":"uint32"}]},{"name":"sweepTxFee","type":"uint256"},{"name":"depositsRevealBlocks","type":"uint256[]"}]},{"name":"depositsExtraInfo","type":"tuple[]","components":[{"name":"fundingTx","type":"tuple","components":[{"name":"version","type":"bytes4"},{"name":"inputVector","type":"bytes"},{"name":"outputVector","type":"bytes"},{"name":"locktime","type":"bytes4"}]},{"name":"blindingFactor","type":"bytes8"},{"name":"walletPubKeyHash","type":"bytes20"},{"name":"walletXOnlyPublicKey","type":"bytes32"},{"name":"refundPubKeyHash","type":"bytes20"},{"name":"refundXOnlyPublicKey","type":"bytes32"},{"name":"refundLocktime","type":"bytes4"}]},{"name":"mainUtxo","type":"tuple","components":[{"name":"txHash","type":"bytes32"},{"name":"txOutputIndex","type":"uint32"},{"name":"txOutputValue","type":"uint64"}]}]}],"outputs":[]},
 {"type":"function","name":"redemptionData","inputs":[{"name":"data","type":"tuple","components":[{"name":"proposal","type":"tuple","components":[{"name":"walletPubKeyHash","type":"bytes20"},{"name":"redeemersOutputScripts","type":"bytes[]"},{"name":"redemptionTxFee","type":"uint256"}]},{"name":"mainUtxo","type":"tuple","components":[{"name":"txHash","type":"bytes32"},{"name":"txOutputIndex","type":"uint32"},{"name":"txOutputValue","type":"uint64"}]}]}],"outputs":[]},
 {"type":"function","name":"movingData","inputs":[{"name":"data","type":"tuple","components":[{"name":"proposal","type":"tuple","components":[{"name":"walletPubKeyHash","type":"bytes20"},{"name":"targetWallets","type":"bytes20[]"},{"name":"movingFundsTxFee","type":"uint256"}]},{"name":"mainUtxo","type":"tuple","components":[{"name":"txHash","type":"bytes32"},{"name":"txOutputIndex","type":"uint32"},{"name":"txOutputValue","type":"uint64"}]}]}],"outputs":[]},
 {"type":"function","name":"movedSweepData","inputs":[{"name":"data","type":"tuple","components":[{"name":"proposal","type":"tuple","components":[{"name":"walletPubKeyHash","type":"bytes20"},{"name":"movingFundsTxHash","type":"bytes32"},{"name":"movingFundsTxOutputIndex","type":"uint32"},{"name":"movedFundsSweepTxFee","type":"uint256"}]},{"name":"mainUtxo","type":"tuple","components":[{"name":"txHash","type":"bytes32"},{"name":"txOutputIndex","type":"uint32"},{"name":"txOutputValue","type":"uint64"}]}]}],"outputs":[]},
 {"type":"function","name":"authorizationPreview","inputs":[{"name":"preview","type":"tuple","components":[{"name":"reservationID","type":"bytes32"},{"name":"transactionHash","type":"bytes32"},{"name":"authorizationRoot","type":"bytes32"},{"name":"digest","type":"bytes32"},{"name":"walletPubKeyHash","type":"bytes20"},{"name":"walletID","type":"bytes32"},{"name":"membersIDsHash","type":"bytes32"},{"name":"snapshotHash","type":"bytes32"},{"name":"resourceHash","type":"bytes32"},{"name":"orderedInputRoot","type":"bytes32"},{"name":"applyPlanHash","type":"bytes32"},{"name":"applyPlanData1","type":"bytes32"},{"name":"applyPlanData2","type":"bytes32"},{"name":"feeLimitSnapshot","type":"uint64"},{"name":"action","type":"uint8"}]}],"outputs":[]}
]`

var (
	frostPreSignBridgeABI    = mustParseABI(frostPreSignBridgeABIJSON)
	frostPreSignRegistryABI  = mustParseABI(frostPreSignRegistryABIJSON)
	frostPreSignCrosslinkABI = mustParseABI(frostPreSignCrosslinkABIJSON)
	frostPreSignCodecABI     = mustParseABI(frostPreSignCodecABIJSON)
)

type frostPreSignActivationEnvelope struct {
	Payload             json.RawMessage `json:"payload"`
	PayloadSHA256       string          `json:"payloadSha256"`
	SignatureAlgorithm  string          `json:"signatureAlgorithm"`
	SignerPublicKeySPKI string          `json:"signerPublicKeySpki"`
	Signature           string          `json:"signature"`
}

type frostPreSignManifestPoint struct {
	BlockNumber uint64 `json:"blockNumber"`
	BlockHash   string `json:"blockHash"`
}

type frostPreSignManifestUpgradeability struct {
	Kind                          string `json:"kind"`
	ImplementationAddress         string `json:"implementationAddress,omitempty"`
	ImplementationRuntimeCodeHash string `json:"implementationRuntimeCodeHash,omitempty"`
	AdminAddress                  string `json:"adminAddress,omitempty"`
	AdminRuntimeCodeHash          string `json:"adminRuntimeCodeHash,omitempty"`
	ImplementationSlotValue       string `json:"implementationSlotValue,omitempty"`
	AdminSlotValue                string `json:"adminSlotValue,omitempty"`
}

type frostPreSignManifestLinkReference struct {
	Start  uint64 `json:"start"`
	Length uint64 `json:"length"`
}

type frostPreSignManifestLinkedLibrary struct {
	ProtocolRole                string                              `json:"protocolRole"`
	Address                     string                              `json:"address"`
	RuntimeCodeHash             string                              `json:"runtimeCodeHash"`
	References                  []frostPreSignManifestLinkReference `json:"references"`
	LinkedLibraryDescriptorHash string                              `json:"linkedLibraryDescriptorHash"`
	LinkedLibraries             []frostPreSignManifestLinkedLibrary `json:"linkedLibraries"`
}

type frostPreSignManifestDeploymentEpoch struct {
	Start                       frostPreSignManifestPoint           `json:"start"`
	End                         *frostPreSignManifestPoint          `json:"end,omitempty"`
	Address                     string                              `json:"address"`
	RuntimeCodeHash             string                              `json:"runtimeCodeHash"`
	LinkedLibraryDescriptorHash string                              `json:"linkedLibraryDescriptorHash"`
	LinkedLibraries             []frostPreSignManifestLinkedLibrary `json:"linkedLibraries"`
	Upgradeability              frostPreSignManifestUpgradeability  `json:"upgradeability"`
}

type frostPreSignManifestContract struct {
	Address                     string                                `json:"address"`
	RuntimeCodeHash             string                                `json:"runtimeCodeHash"`
	ProtocolID                  string                                `json:"protocolID"`
	DeploymentBlock             uint64                                `json:"deploymentBlock"`
	RelevantEventStartBlock     uint64                                `json:"relevantEventStartBlock"`
	BridgeAddress               *string                               `json:"bridgeAddress,omitempty"`
	SigningPolicyHash           *string                               `json:"signingPolicyHash,omitempty"`
	LinkedLibraryDescriptorHash string                                `json:"linkedLibraryDescriptorHash"`
	LinkedLibraries             []frostPreSignManifestLinkedLibrary   `json:"linkedLibraries"`
	Upgradeability              frostPreSignManifestUpgradeability    `json:"upgradeability"`
	HistoricalDeploymentEpochs  []frostPreSignManifestDeploymentEpoch `json:"historicalDeploymentEpochs"`
}

type frostPreSignManifestContracts struct {
	Bridge                  frostPreSignManifestContract `json:"bridge"`
	CompleteRouter          frostPreSignManifestContract `json:"completeRouter"`
	AuthorizationRegistry   frostPreSignManifestContract `json:"authorizationRegistry"`
	FrostWalletRegistry     frostPreSignManifestContract `json:"frostWalletRegistry"`
	FrostProposalValidator  frostPreSignManifestContract `json:"frostProposalValidator"`
	FrostSortitionPool      frostPreSignManifestContract `json:"frostSortitionPool"`
	ECDSAFraudRouter        frostPreSignManifestContract `json:"ecdsaFraudRouter"`
	ECDSACutoverCoordinator frostPreSignManifestContract `json:"ecdsaCutoverCoordinator"`
}

type frostPreSignManifestEthereum struct {
	ChainID                         uint64                        `json:"chainID"`
	GenesisBlockHash                string                        `json:"genesisBlockHash"`
	Checkpoint                      frostPreSignManifestPoint     `json:"checkpoint"`
	ScanStartBlock                  uint64                        `json:"scanStartBlock"`
	ConfirmationDepth               uint64                        `json:"confirmationDepth"`
	MaxJournalLagBlocks             uint64                        `json:"maxJournalLagBlocks"`
	ConfigurationFingerprint        string                        `json:"configurationFingerprint"`
	DescriptorSetHash               string                        `json:"descriptorSetHash"`
	LinkedLibraryDescriptorSetHash  string                        `json:"linkedLibraryDescriptorSetHash"`
	StoreID                         string                        `json:"storeID"`
	SourceTrustDomainID             string                        `json:"sourceTrustDomainID"`
	SourceEndpointFingerprint       string                        `json:"sourceEndpointFingerprint"`
	SourceOperatorFingerprint       string                        `json:"sourceOperatorFingerprint"`
	SourceHistoryStoreID            string                        `json:"sourceHistoryStoreID"`
	SourceHistoryStoreFingerprint   string                        `json:"sourceHistoryStoreFingerprint"`
	VerifierTrustDomainID           string                        `json:"verifierTrustDomainID"`
	VerifierEndpointFingerprint     string                        `json:"verifierEndpointFingerprint"`
	VerifierOperatorFingerprint     string                        `json:"verifierOperatorFingerprint"`
	VerifierHistoryStoreID          string                        `json:"verifierHistoryStoreID"`
	VerifierHistoryStoreFingerprint string                        `json:"verifierHistoryStoreFingerprint"`
	Contracts                       frostPreSignManifestContracts `json:"contracts"`
	CompleteDepositKeyInventory     json.RawMessage               `json:"completeDepositKeyInventory"`
	FrostArchive                    json.RawMessage               `json:"frostArchive"`
}

type frostPreSignManifestCanonicalJournal struct {
	StoreID                   string                                     `json:"storeID"`
	StoreFingerprint          string                                     `json:"storeFingerprint"`
	ClusterFingerprint        string                                     `json:"clusterFingerprint"`
	Checkpoint                frostPreSignManifestPoint                  `json:"checkpoint"`
	DescriptorSetHash         string                                     `json:"descriptorSetHash"`
	SourceTrustDomainID       string                                     `json:"sourceTrustDomainID"`
	SourceEndpointFingerprint string                                     `json:"sourceEndpointFingerprint"`
	SourceOperatorFingerprint string                                     `json:"sourceOperatorFingerprint"`
	SourceIdentity            frostPreSignManifestRetainedSourceIdentity `json:"sourceIdentity"`
	MinimumGeneration         uint64                                     `json:"minimumGeneration"`
}

type frostPreSignManifestRetainedEndpointIdentity struct {
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

type frostPreSignManifestRetainedSourceIdentity struct {
	Schema               string                                       `json:"schema"`
	TrustDomainID        string                                       `json:"trustDomainID"`
	EndpointFingerprint  string                                       `json:"endpointFingerprint"`
	OperatorFingerprint  string                                       `json:"operatorFingerprint"`
	HistorySignerKeyHash string                                       `json:"historySignerKeyHash"`
	Export               frostPreSignManifestRetainedEndpointIdentity `json:"export"`
	Verifier             frostPreSignManifestRetainedEndpointIdentity `json:"verifier"`
}

type frostPreSignManifestQuarantineJournal struct {
	ProtocolID                   string                              `json:"protocolID"`
	LiftProtocolID               string                              `json:"liftProtocolID"`
	TombstoneProtocolID          string                              `json:"tombstoneProtocolID"`
	CheckpointAuthorityThreshold uint64                              `json:"checkpointAuthorityThreshold"`
	CheckpointAuthorities        []frostPreSignManifestLiftAuthority `json:"checkpointAuthorities"`
	CheckpointMinimumSequence    uint64                              `json:"checkpointMinimumSequence"`
	CheckpointPredecessorHash    string                              `json:"checkpointPredecessorHash"`
	LiftAuthorityThreshold       uint64                              `json:"liftAuthorityThreshold"`
	LiftAuthorities              []frostPreSignManifestLiftAuthority `json:"liftAuthorities"`
	StoreID                      string                              `json:"storeID"`
	StoreFingerprint             string                              `json:"storeFingerprint"`
	ClusterFingerprint           string                              `json:"clusterFingerprint"`
	MinimumGeneration            uint64                              `json:"minimumGeneration"`
}

type frostPreSignManifestLiftAuthority struct {
	AuthorityID       string `json:"authorityID"`
	PublicKeySPKIHash string `json:"publicKeySpkiHash"`
}

type frostPreSignManifestNativeSignerAnchor struct {
	ProtocolID                      string `json:"protocolID"`
	StreamID                        string `json:"streamID"`
	TrustDomainID                   string `json:"trustDomainID"`
	EndpointLeafSPKIHash            string `json:"endpointLeafSpkiHash"`
	OnlineKeyHash                   string `json:"onlineKeyHash"`
	OperatorFingerprint             string `json:"operatorFingerprint"`
	HistoryStoreID                  string `json:"historyStoreID"`
	HistoryStoreFingerprint         string `json:"historyStoreFingerprint"`
	HistoryClusterFingerprint       string `json:"historyClusterFingerprint"`
	OfflineAuthorityHash            string `json:"offlineAuthorityHash"`
	ClientSPKIHash                  string `json:"clientSpkiHash"`
	SignerStoreFingerprint          string `json:"signerStoreFingerprint"`
	TransportBinding                string `json:"transportBinding"`
	WitnessMaximumRecords           uint64 `json:"witnessMaximumRecords"`
	WitnessRotationThresholdRecords uint64 `json:"witnessRotationThresholdRecords"`
}

type frostPreSignManifestFrostSigner struct {
	TrustDomainID                       string                                 `json:"trustDomainID"`
	DurableSessionStoreFingerprint      string                                 `json:"durableSessionStoreFingerprint"`
	ProtocolID                          string                                 `json:"protocolID"`
	ReservationProtocolID               string                                 `json:"reservationProtocolID"`
	BitcoinOutboxProtocolID             string                                 `json:"bitcoinOutboxProtocolID"`
	SigningPolicyHash                   string                                 `json:"signingPolicyHash"`
	CompleteRouterAddress               string                                 `json:"completeRouterAddress"`
	AuthorizationRegistryAddress        string                                 `json:"authorizationRegistryAddress"`
	AttestationSignerKeyHash            string                                 `json:"attestationSignerKeyHash"`
	HandshakeEndpointFingerprint        string                                 `json:"handshakeEndpointFingerprint"`
	HandshakeOperatorFingerprint        string                                 `json:"handshakeOperatorFingerprint"`
	Threshold                           uint64                                 `json:"threshold"`
	MaximumGroupSize                    uint64                                 `json:"maximumGroupSize"`
	RetainedGroupInventoryProtocolID    string                                 `json:"retainedGroupInventoryProtocolID"`
	CanonicalJournal                    frostPreSignManifestCanonicalJournal   `json:"canonicalJournal"`
	QuarantineJournal                   frostPreSignManifestQuarantineJournal  `json:"quarantineJournal"`
	NativeSignerAnchor                  frostPreSignManifestNativeSignerAnchor `json:"nativeSignerAnchor"`
	ExactRetainedGroupInventoryRequired bool                                   `json:"exactRetainedGroupInventoryRequired"`
	FinalizedReservationReceiptRequired bool                                   `json:"finalizedReservationReceiptRequired"`
	ExactReservationIdentityRequired    bool                                   `json:"exactReservationIdentityRequired"`
	AuthorizationRootRequired           bool                                   `json:"authorizationRootRequired"`
	DurableSessionPersistenceRequired   bool                                   `json:"durableSessionPersistenceRequired"`
	DurableBitcoinOutboxRequired        bool                                   `json:"durableBitcoinOutboxRequired"`
	QuarantineFailClosed                bool                                   `json:"quarantineFailClosed"`
}

type frostPreSignActivationManifest struct {
	Schema                       string                          `json:"schema"`
	ActivationSequence           uint64                          `json:"activationSequence"`
	ActivationID                 string                          `json:"activationID"`
	Environment                  string                          `json:"environment"`
	Migrations                   json.RawMessage                 `json:"migrations"`
	Bitcoin                      json.RawMessage                 `json:"bitcoin"`
	Ethereum                     frostPreSignManifestEthereum    `json:"ethereum"`
	ECDSACutover                 json.RawMessage                 `json:"ecdsaCutover"`
	Outbox                       json.RawMessage                 `json:"outbox"`
	FrostSigner                  frostPreSignManifestFrostSigner `json:"frostSigner"`
	manifestHash                 [32]byte
	activationAuthorityPublicKey [32]byte
	activationAuthorityKeyHash   [32]byte
}

type frostPreSignDeploymentPin struct {
	role                        string
	name                        string
	deploymentBlock             uint64
	relevantEventStartBlock     uint64
	address                     [20]byte
	runtimeCodeHash             [32]byte
	upgradeability              string
	implementationAddress       [20]byte
	implementationCodeHash      [32]byte
	adminAddress                [20]byte
	adminCodeHash               [32]byte
	implementationSlotValue     [32]byte
	adminSlotValue              [32]byte
	linkedLibraryDescriptorHash [32]byte
	linkedLibraries             []frostPreSignLinkedLibraryPin
	historicalEpochs            []frostPreSignDeploymentEpochPin
}

type frostPreSignDeploymentEpochPin struct {
	start      tbtc.FrostPreSignFinality
	end        *tbtc.FrostPreSignFinality
	descriptor frostPreSignDeploymentPin
}

type frostPreSignLinkedLibraryPin struct {
	protocolRole                string
	address                     [20]byte
	runtimeCodeHash             [32]byte
	references                  []frostPreSignManifestLinkReference
	linkedLibraryDescriptorHash [32]byte
	linkedLibraries             []frostPreSignLinkedLibraryPin
}

type frostPreSignEthereumAdapter struct {
	chain       *TbtcChain
	reader      tbtc.FrostPreSignEthereumEvidenceVerifier
	fromAddress common.Address
	profile     tbtc.FrostPreSignActivationProfile
	manifest    frostPreSignActivationManifest
	deployments []frostPreSignDeploymentPin
	bridge      *bind.BoundContract

	mutex sync.RWMutex
}

type frostPreSignExactHashReader interface {
	HeaderByHash(context.Context, common.Hash) (*types.Header, error)
	CodeAtHash(context.Context, common.Address, common.Hash) ([]byte, error)
	StorageAtHash(
		context.Context,
		common.Address,
		common.Hash,
		common.Hash,
	) ([]byte, error)
	CallContractAtHash(
		context.Context,
		geth.CallMsg,
		common.Hash,
	) ([]byte, error)
}

type frostPreSignStandardEthereumReader interface {
	HeaderByNumber(context.Context, *big.Int) (*types.Header, error)
	HeaderByHash(context.Context, common.Hash) (*types.Header, error)
	TransactionReceipt(context.Context, common.Hash) (*types.Receipt, error)
	FilterLogs(context.Context, geth.FilterQuery) ([]types.Log, error)
}

type frostPreSignBitcoinTxInfo struct {
	Version      [4]byte
	InputVector  []byte
	OutputVector []byte
	Locktime     [4]byte
}

type frostPreSignSeatAttestationABI struct {
	WalletMembersIDs     []uint32
	SigningMemberIndices []uint8
	Signatures           []byte
}

type frostPreSignDepositAuthorizationData struct {
	Proposal          tbtcabi.WalletProposalValidatorDepositSweepProposal
	DepositsExtraInfo []tbtcabi.WalletProposalValidatorTaprootDepositExtraInfo
	MainUtxo          tbtcabi.BitcoinTxUTXO3
}

type frostPreSignRedemptionAuthorizationData struct {
	Proposal tbtcabi.WalletProposalValidatorRedemptionProposal
	MainUtxo tbtcabi.BitcoinTxUTXO3
}

type frostPreSignMovingAuthorizationData struct {
	Proposal tbtcabi.WalletProposalValidatorMovingFundsProposal
	MainUtxo tbtcabi.BitcoinTxUTXO3
}

type frostPreSignMovedSweepAuthorizationData struct {
	Proposal tbtcabi.WalletProposalValidatorMovedFundsSweepProposal
	MainUtxo tbtcabi.BitcoinTxUTXO3
}

type frostPreSignAuthorizationPreview struct {
	ReservationID     [32]byte
	TransactionHash   [32]byte
	AuthorizationRoot [32]byte
	Digest            [32]byte
	WalletPubKeyHash  [20]byte
	WalletID          [32]byte
	MembersIDsHash    [32]byte
	SnapshotHash      [32]byte
	ResourceHash      [32]byte
	OrderedInputRoot  [32]byte
	ApplyPlanHash     [32]byte
	ApplyPlanData1    [32]byte
	ApplyPlanData2    [32]byte
	FeeLimitSnapshot  uint64
	Action            uint8
}

type frostPreSignPreAuthorizationABI struct {
	Action           uint8
	WalletPubKeyHash [20]byte
	WalletID         [32]byte
	MembersIDsHash   [32]byte
	SnapshotHash     [32]byte
	ResourceHash     [32]byte
	OrderedInputRoot [32]byte
	ApplyPlanHash    [32]byte
	ApplyPlanData1   [32]byte
	ApplyPlanData2   [32]byte
	FeeLimitSnapshot uint64
}

type frostPreSignWalletABI struct {
	EcdsaWalletID                          [32]byte
	MainUtxoHash                           [32]byte
	PendingRedemptionsValue                uint64
	CreatedAt                              uint32
	MovingFundsRequestedAt                 uint32
	ClosingStartedAt                       uint32
	PendingMovedFundsSweepRequestsCount    uint32
	State                                  uint8
	MovingFundsTargetWalletsCommitmentHash [32]byte
}

type frostPreSignRegistryWalletABI struct {
	MembersIdsHash [32]byte
	XOnlyOutputKey [32]byte
}

func (tc *TbtcChain) ConfigureFrostPreSignAuthorization(
	ctx context.Context,
	manifestPath string,
	trustedEnvelopeSignerKeyHash string,
	expectedLinkedLibraryDescriptorSetHash string,
	ethereumEvidenceVerifier tbtc.FrostPreSignEthereumEvidenceVerifier,
) (*tbtc.FrostPreSignActivationProfile, error) {
	if ctx == nil {
		return nil, fmt.Errorf("FROST activation context is nil")
	}
	if tc == nil || tc.baseChain == nil {
		return nil, fmt.Errorf("FROST Ethereum chain is unavailable")
	}
	if ethereumEvidenceVerifier == nil {
		return nil, fmt.Errorf(
			"independent FROST Ethereum evidence verifier is nil",
		)
	}
	manifest, err := loadFrostPreSignActivationManifest(
		manifestPath,
		trustedEnvelopeSignerKeyHash,
	)
	if err != nil {
		return nil, err
	}
	expectedDescriptorSetHash, err := frostPreSignParseBytes32(
		expectedLinkedLibraryDescriptorSetHash,
	)
	if err != nil {
		return nil, fmt.Errorf("invalid expected linked-library descriptor-set hash: [%w]", err)
	}
	manifestDescriptorSetHash, err := frostPreSignParseBytes32(
		manifest.Ethereum.LinkedLibraryDescriptorSetHash,
	)
	if err != nil || expectedDescriptorSetHash != manifestDescriptorSetHash {
		return nil, fmt.Errorf("signed activation linked-library descriptor set differs from this signer build")
	}
	primaryReader, err := newFrostPreSignPrimaryEthereumReader(
		tc.client,
		tc.rpcClient,
		tc.chainID,
		tc.frostPrimaryEthereumRequestTimeout,
		tc.rpcLimiter,
	)
	if err != nil {
		return nil, err
	}
	adapter, err := newFrostPreSignEthereumAdapter(
		ctx,
		tc,
		manifest,
		primaryReader,
		true,
	)
	if err != nil {
		return nil, err
	}
	verifier, err := newFrostPreSignEthereumAdapter(
		ctx,
		tc,
		manifest,
		ethereumEvidenceVerifier,
		false,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"independent FROST Ethereum verifier rejected activation: [%w]",
			err,
		)
	}
	if _, err := frostPreSignMatchingCurrentFinality(
		ctx,
		adapter,
		verifier,
	); err != nil {
		return nil, fmt.Errorf(
			"FROST Ethereum endpoints do not share one finalized activation point: [%w]",
			err,
		)
	}
	tc.frostPreSignAuthorizationAdapter = adapter
	tc.frostPreSignAuthorizationVerifier = verifier
	profile := adapter.profile
	return &profile, nil
}

func loadFrostPreSignActivationManifest(
	path string,
	trustedEnvelopeSignerKeyHash string,
) (*frostPreSignActivationManifest, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("FROST activation manifest path is empty")
	}
	// #nosec G304 -- the operator explicitly configures the activation manifest.
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open FROST activation manifest: [%w]", err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 1024*1024+1))
	if err != nil {
		return nil, fmt.Errorf("cannot read FROST activation envelope: [%w]", err)
	}
	if len(data) == 0 || len(data) > 1024*1024 {
		return nil, fmt.Errorf("FROST activation envelope size is invalid")
	}

	envelope := &frostPreSignActivationEnvelope{}
	if err := frostPreSignDecodeStrictJSON(data, envelope); err != nil {
		return nil, fmt.Errorf("cannot decode FROST activation envelope: [%w]", err)
	}
	if envelope.SignatureAlgorithm != "ed25519" || len(envelope.Payload) == 0 {
		return nil, fmt.Errorf("FROST activation envelope is malformed")
	}
	canonicalPayload, err := frostPreSignCanonicalJSON(envelope.Payload)
	if err != nil {
		return nil, fmt.Errorf("cannot canonicalize FROST activation payload: [%w]", err)
	}
	payloadHash := sha256.Sum256(canonicalPayload)
	declaredPayloadHash, err := frostPreSignParseBytes32(envelope.PayloadSHA256)
	if err != nil || declaredPayloadHash != payloadHash {
		return nil, fmt.Errorf("FROST activation payload hash mismatch")
	}
	trustedKeyHash, err := frostPreSignParseBytes32(trustedEnvelopeSignerKeyHash)
	if err != nil {
		return nil, fmt.Errorf("invalid trusted FROST activation signer key hash: [%w]", err)
	}
	publicKeyDER, err := base64.StdEncoding.Strict().DecodeString(
		envelope.SignerPublicKeySPKI,
	)
	if err != nil || len(publicKeyDER) == 0 || len(publicKeyDER) > 1024 {
		return nil, fmt.Errorf("FROST activation signer public key is invalid")
	}
	if sha256.Sum256(publicKeyDER) != trustedKeyHash {
		return nil, fmt.Errorf("FROST activation signer is not trusted")
	}
	parsedPublicKey, err := x509.ParsePKIXPublicKey(publicKeyDER)
	if err != nil {
		return nil, fmt.Errorf("cannot parse FROST activation signer key: [%w]", err)
	}
	publicKey, ok := parsedPublicKey.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("FROST activation signer key is not Ed25519")
	}
	if err := tbtc.ValidateFrostNativeSignerAnchorTrustEd25519PublicKey(
		publicKey,
	); err != nil {
		return nil, fmt.Errorf(
			"FROST activation signer key point is invalid: [%w]",
			err,
		)
	}
	signature, err := base64.StdEncoding.Strict().DecodeString(envelope.Signature)
	if err != nil || len(signature) != ed25519.SignatureSize ||
		!ed25519.Verify(publicKey, canonicalPayload, signature) {
		return nil, fmt.Errorf("FROST activation envelope signature is invalid")
	}

	manifest := &frostPreSignActivationManifest{}
	if err := frostPreSignDecodeStrictJSON(envelope.Payload, manifest); err != nil {
		return nil, fmt.Errorf("cannot decode FROST activation payload: [%w]", err)
	}
	manifest.manifestHash = payloadHash
	copy(manifest.activationAuthorityPublicKey[:], publicKey)
	manifest.activationAuthorityKeyHash = trustedKeyHash
	if err := validateFrostPreSignActivationManifest(manifest); err != nil {
		return nil, err
	}
	attestationKeyHash, err := frostPreSignParseBytes32(
		manifest.FrostSigner.AttestationSignerKeyHash,
	)
	if err != nil || attestationKeyHash == trustedKeyHash {
		return nil, fmt.Errorf(
			"FROST runtime attestation key must differ from the activation authority key",
		)
	}
	anchorOfflineAuthorityHash, err := frostPreSignParseBytes32(
		manifest.FrostSigner.NativeSignerAnchor.OfflineAuthorityHash,
	)
	if err != nil || anchorOfflineAuthorityHash != trustedKeyHash {
		return nil, fmt.Errorf(
			"FROST native signer anchor offline authority differs from the activation authority",
		)
	}
	return manifest, nil
}

type frostPreSignCanonicalHashReader struct {
	standardReader frostPreSignStandardEthereumReader
	headerReader   interface {
		HeaderByHash(context.Context, common.Hash) (*types.Header, error)
	}
	rpcClient      *rpc.Client
	chainID        *big.Int
	requestTimeout time.Duration
	rpcLimiter     interface {
		AcquirePermit() error
		ReleasePermit()
	}
}

func (reader *frostPreSignCanonicalHashReader) ChainID(
	context.Context,
) (*big.Int, error) {
	if reader.chainID == nil {
		return nil, fmt.Errorf("Ethereum chain ID is unavailable")
	}
	return new(big.Int).Set(reader.chainID), nil
}

func (reader *frostPreSignCanonicalHashReader) HeaderByNumber(
	ctx context.Context,
	number *big.Int,
) (*types.Header, error) {
	if reader.standardReader == nil {
		return nil, fmt.Errorf("standard Ethereum reader is unavailable")
	}
	return reader.standardReader.HeaderByNumber(ctx, number)
}

func (reader *frostPreSignCanonicalHashReader) HeaderByHash(
	ctx context.Context,
	blockHash common.Hash,
) (*types.Header, error) {
	if reader.standardReader == nil {
		if reader.headerReader == nil {
			return nil, fmt.Errorf("exact-hash Ethereum reader is unavailable")
		}
		return reader.headerReader.HeaderByHash(ctx, blockHash)
	}
	return reader.standardReader.HeaderByHash(ctx, blockHash)
}

func (reader *frostPreSignCanonicalHashReader) TransactionReceipt(
	ctx context.Context,
	transactionHash common.Hash,
) (*types.Receipt, error) {
	if reader.standardReader == nil {
		return nil, fmt.Errorf("standard Ethereum reader is unavailable")
	}
	return reader.standardReader.TransactionReceipt(ctx, transactionHash)
}

func (reader *frostPreSignCanonicalHashReader) FilterLogs(
	ctx context.Context,
	query geth.FilterQuery,
) ([]types.Log, error) {
	if reader.standardReader == nil {
		return nil, fmt.Errorf("standard Ethereum reader is unavailable")
	}
	return reader.standardReader.FilterLogs(ctx, query)
}

func (reader *frostPreSignCanonicalHashReader) CodeAtHash(
	ctx context.Context,
	account common.Address,
	blockHash common.Hash,
) ([]byte, error) {
	ctx, cancel := reader.requestContext(ctx)
	defer cancel()
	var result hexutil.Bytes
	err := reader.callContext(
		ctx,
		&result,
		"eth_getCode",
		account,
		rpc.BlockNumberOrHashWithHash(blockHash, true),
	)
	return result, err
}

func (reader *frostPreSignCanonicalHashReader) StorageAtHash(
	ctx context.Context,
	account common.Address,
	key common.Hash,
	blockHash common.Hash,
) ([]byte, error) {
	ctx, cancel := reader.requestContext(ctx)
	defer cancel()
	var result hexutil.Bytes
	err := reader.callContext(
		ctx,
		&result,
		"eth_getStorageAt",
		account,
		key,
		rpc.BlockNumberOrHashWithHash(blockHash, true),
	)
	return result, err
}

func (reader *frostPreSignCanonicalHashReader) CallContractAtHash(
	ctx context.Context,
	message geth.CallMsg,
	blockHash common.Hash,
) ([]byte, error) {
	ctx, cancel := reader.requestContext(ctx)
	defer cancel()
	var result hexutil.Bytes
	err := reader.callContext(
		ctx,
		&result,
		"eth_call",
		frostPreSignCallArgument(message),
		rpc.BlockNumberOrHashWithHash(blockHash, true),
	)
	return result, err
}

func (reader *frostPreSignCanonicalHashReader) callContext(
	ctx context.Context,
	result interface{},
	method string,
	args ...interface{},
) error {
	if reader.rpcLimiter != nil {
		if err := acquireFrostPreSignEthereumRPCPermit(
			ctx,
			reader.rpcLimiter,
		); err != nil {
			return fmt.Errorf("cannot acquire Ethereum RPC rate limiter permit: [%w]", err)
		}
		defer reader.rpcLimiter.ReleasePermit()
	}
	return reader.rpcClient.CallContext(ctx, result, method, args...)
}

func acquireFrostPreSignEthereumRPCPermit(
	ctx context.Context,
	limiter interface {
		AcquirePermit() error
		ReleasePermit()
	},
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	const (
		acquisitionPending uint32 = iota
		acquisitionCanceled
		acquisitionDelivered
	)
	var acquisitionState atomic.Uint32
	acquisitionResult := make(chan error, 1)
	go func() {
		err := limiter.AcquirePermit()
		if acquisitionState.CompareAndSwap(
			acquisitionPending,
			acquisitionDelivered,
		) {
			acquisitionResult <- err
			return
		}

		// The caller stopped waiting before the legacy limiter completed.
		// Return any permit acquired after cancellation instead of leaking it.
		if err == nil {
			limiter.ReleasePermit()
		}
	}()

	select {
	case err := <-acquisitionResult:
		return err
	case <-ctx.Done():
		if acquisitionState.CompareAndSwap(
			acquisitionPending,
			acquisitionCanceled,
		) {
			return ctx.Err()
		}

		// Acquisition won the race with cancellation. Drain the delivered
		// result and return a successfully acquired permit before exiting.
		if err := <-acquisitionResult; err == nil {
			limiter.ReleasePermit()
		}
		return ctx.Err()
	}
}

func (reader *frostPreSignCanonicalHashReader) requestContext(
	ctx context.Context,
) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	if reader.requestTimeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, reader.requestTimeout)
}

func frostPreSignCallArgument(message geth.CallMsg) map[string]interface{} {
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

func (adapter *frostPreSignEthereumAdapter) exactHashReader() (
	frostPreSignExactHashReader,
	error,
) {
	if adapter == nil || adapter.reader == nil {
		return nil, fmt.Errorf("FROST Ethereum evidence reader is unavailable")
	}
	return adapter.reader, nil
}

func newFrostPreSignPrimaryEthereumReader(
	client interface{},
	rpcClient *rpc.Client,
	chainID *big.Int,
	requestTimeout time.Duration,
	rpcLimiter interface {
		AcquirePermit() error
		ReleasePermit()
	},
) (tbtc.FrostPreSignEthereumEvidenceVerifier, error) {
	standardReader, ok := client.(frostPreSignStandardEthereumReader)
	if !ok {
		return nil, fmt.Errorf(
			"Ethereum client does not expose required canonical evidence reads",
		)
	}
	if rpcClient == nil || chainID == nil || chainID.Sign() <= 0 {
		return nil, fmt.Errorf("Ethereum client does not expose canonical EIP-1898 reads")
	}
	return &frostPreSignCanonicalHashReader{
		standardReader: standardReader,
		headerReader:   standardReader,
		rpcClient:      rpcClient,
		chainID:        new(big.Int).Set(chainID),
		requestTimeout: requestTimeout,
		rpcLimiter:     rpcLimiter,
	}, nil
}

func frostPreSignDecodeStrictJSON(data []byte, target interface{}) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("JSON contains trailing data")
	}
	return nil
}

func frostPreSignCanonicalJSON(data []byte) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value interface{}
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, fmt.Errorf("JSON contains trailing data")
	}
	buffer := bytes.NewBuffer(nil)
	if err := frostPreSignWriteCanonicalJSON(buffer, value); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func frostPreSignWriteCanonicalJSON(buffer *bytes.Buffer, value interface{}) error {
	switch typed := value.(type) {
	case nil:
		buffer.WriteString("null")
	case bool:
		if typed {
			buffer.WriteString("true")
		} else {
			buffer.WriteString("false")
		}
	case string:
		encoded, _ := json.Marshal(typed)
		buffer.Write(encoded)
	case json.Number:
		raw := typed.String()
		if strings.ContainsAny(raw, ".eE") {
			return fmt.Errorf("canonical JSON number [%s] is not an integer", raw)
		}
		integer, ok := new(big.Int).SetString(raw, 10)
		if !ok || integer.Cmp(big.NewInt(-9007199254740991)) < 0 ||
			integer.Cmp(big.NewInt(9007199254740991)) > 0 {
			return fmt.Errorf("canonical JSON number [%s] is unsafe", raw)
		}
		buffer.WriteString(integer.String())
	case []interface{}:
		buffer.WriteByte('[')
		for index, item := range typed {
			if index > 0 {
				buffer.WriteByte(',')
			}
			if err := frostPreSignWriteCanonicalJSON(buffer, item); err != nil {
				return err
			}
		}
		buffer.WriteByte(']')
	case map[string]interface{}:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		buffer.WriteByte('{')
		for index, key := range keys {
			if index > 0 {
				buffer.WriteByte(',')
			}
			encodedKey, _ := json.Marshal(key)
			buffer.Write(encodedKey)
			buffer.WriteByte(':')
			if err := frostPreSignWriteCanonicalJSON(buffer, typed[key]); err != nil {
				return err
			}
		}
		buffer.WriteByte('}')
	default:
		return fmt.Errorf("unsupported canonical JSON value [%T]", value)
	}
	return nil
}

func validateFrostPreSignActivationManifest(
	manifest *frostPreSignActivationManifest,
) error {
	if manifest == nil || manifest.Schema != frostPreSignManifestVersion ||
		manifest.ActivationSequence == 0 || manifest.ActivationID == "" ||
		len(manifest.Environment) == 0 || len(manifest.Environment) > 64 ||
		manifest.Ethereum.ChainID == 0 || manifest.manifestHash == [32]byte{} {
		return fmt.Errorf("FROST activation payload is incomplete")
	}
	genesisHash, err := frostPreSignParseBytes32(
		manifest.Ethereum.GenesisBlockHash,
	)
	if err != nil || genesisHash == [32]byte{} {
		return fmt.Errorf("FROST activation genesis block hash is invalid")
	}
	if _, err := frostPreSignParseBytes32(manifest.ActivationID); err != nil {
		return fmt.Errorf("invalid FROST activation ID: [%w]", err)
	}
	frost := manifest.FrostSigner
	if frost.Threshold != 51 || frost.MaximumGroupSize != 100 ||
		!frost.ExactRetainedGroupInventoryRequired ||
		!frost.FinalizedReservationReceiptRequired ||
		!frost.ExactReservationIdentityRequired ||
		!frost.AuthorizationRootRequired ||
		!frost.DurableSessionPersistenceRequired ||
		!frost.DurableBitcoinOutboxRequired || !frost.QuarantineFailClosed ||
		len(frost.DurableSessionStoreFingerprint) == 0 ||
		len(frost.DurableSessionStoreFingerprint) > 256 {
		return fmt.Errorf("FROST activation signer policy is incomplete")
	}
	for name, value := range map[string]string{
		"signer protocol":                   frost.ProtocolID,
		"reservation protocol":              frost.ReservationProtocolID,
		"Bitcoin outbox protocol":           frost.BitcoinOutboxProtocolID,
		"signing policy":                    frost.SigningPolicyHash,
		"attestation signer key":            frost.AttestationSignerKeyHash,
		"retained group inventory protocol": frost.RetainedGroupInventoryProtocolID,
		"quarantine journal protocol":       frost.QuarantineJournal.ProtocolID,
		"quarantine lift protocol":          frost.QuarantineJournal.LiftProtocolID,
		"quarantine tombstone protocol":     frost.QuarantineJournal.TombstoneProtocolID,
	} {
		parsed, err := frostPreSignParseBytes32(value)
		if err != nil || parsed == [32]byte{} {
			return fmt.Errorf("invalid FROST activation %s", name)
		}
	}
	durableSessionStoreFingerprint, err := frostPreSignParseBytes32(
		frost.DurableSessionStoreFingerprint,
	)
	if err != nil || durableSessionStoreFingerprint == [32]byte{} {
		return fmt.Errorf("invalid FROST durable session store fingerprint")
	}
	anchorManifest, err := frostPreSignNativeSignerAnchorManifest(manifest)
	if err != nil {
		return err
	}
	anchorIdentity := anchorManifest.Identity
	if expectedStreamID := tbtc.ComputeFrostNativeSignerAnchorStreamID(
		anchorIdentity,
	); expectedStreamID != anchorIdentity.StreamID {
		return fmt.Errorf("FROST native signer anchor stream ID mismatch")
	}
	if anchorIdentity.SignerStoreFingerprint != durableSessionStoreFingerprint {
		return fmt.Errorf(
			"FROST native signer anchor store differs from the durable signer store",
		)
	}
	if anchorIdentity.TrustDomainID == frost.TrustDomainID ||
		anchorIdentity.TrustDomainID == manifest.Ethereum.SourceTrustDomainID ||
		anchorIdentity.TrustDomainID == manifest.Ethereum.VerifierTrustDomainID ||
		anchorIdentity.TrustDomainID == frost.CanonicalJournal.SourceTrustDomainID {
		return fmt.Errorf(
			"FROST native signer anchor trust domain is not independent",
		)
	}
	if anchorIdentity.HistoryStoreID == manifest.Ethereum.StoreID ||
		anchorIdentity.HistoryStoreID == manifest.Ethereum.SourceHistoryStoreID ||
		anchorIdentity.HistoryStoreID == manifest.Ethereum.VerifierHistoryStoreID ||
		anchorIdentity.HistoryStoreID == frost.CanonicalJournal.StoreID ||
		anchorIdentity.HistoryStoreID == frost.QuarantineJournal.StoreID {
		return fmt.Errorf(
			"FROST native signer anchor history store is not independent",
		)
	}
	attestationKeyHash, err := frostPreSignParseBytes32(
		frost.AttestationSignerKeyHash,
	)
	if err != nil {
		return fmt.Errorf("invalid FROST runtime attestation key")
	}
	if anchorIdentity.OnlineKeyHash == anchorIdentity.OfflineAuthorityHash ||
		anchorIdentity.OnlineKeyHash == anchorIdentity.ClientSPKIHash ||
		anchorIdentity.OnlineKeyHash == attestationKeyHash ||
		anchorIdentity.OfflineAuthorityHash == anchorIdentity.ClientSPKIHash ||
		anchorIdentity.OfflineAuthorityHash == attestationKeyHash ||
		anchorIdentity.ClientSPKIHash == attestationKeyHash {
		return fmt.Errorf("FROST native signer anchor authority keys are not independent")
	}
	journal := frost.CanonicalJournal
	if strings.TrimSpace(journal.StoreID) == "" || len(journal.StoreID) > 255 ||
		strings.TrimSpace(journal.SourceTrustDomainID) == "" ||
		len(journal.SourceTrustDomainID) > 128 ||
		journal.Checkpoint.BlockNumber == 0 ||
		journal.Checkpoint.BlockNumber > manifest.Ethereum.Checkpoint.BlockNumber {
		return fmt.Errorf("FROST canonical retained-group journal manifest is incomplete")
	}
	journalCheckpointHash, err := frostPreSignParseBytes32(journal.Checkpoint.BlockHash)
	if err != nil || journalCheckpointHash == [32]byte{} {
		return fmt.Errorf("invalid FROST canonical journal checkpoint hash: [%w]", err)
	}
	journalValues := map[string]string{
		"store fingerprint":           journal.StoreFingerprint,
		"cluster fingerprint":         journal.ClusterFingerprint,
		"descriptor set hash":         journal.DescriptorSetHash,
		"source endpoint fingerprint": journal.SourceEndpointFingerprint,
		"source operator fingerprint": journal.SourceOperatorFingerprint,
	}
	parsedJournalValues := make(map[string][32]byte, len(journalValues))
	for name, value := range journalValues {
		parsed, err := frostPreSignParseBytes32(value)
		if err != nil || parsed == [32]byte{} {
			return fmt.Errorf("invalid FROST canonical journal %s", name)
		}
		parsedJournalValues[name] = parsed
	}
	sourceIdentity, err := frostPreSignRetainedSourceIdentity(
		journal.SourceIdentity,
	)
	if err != nil {
		return fmt.Errorf(
			"FROST canonical journal complete endpoint identity is invalid: [%w]",
			err,
		)
	}
	if sourceIdentity.TrustDomainID != journal.SourceTrustDomainID ||
		sourceIdentity.EndpointFingerprint !=
			parsedJournalValues["source endpoint fingerprint"] ||
		sourceIdentity.OperatorFingerprint !=
			parsedJournalValues["source operator fingerprint"] {
		return fmt.Errorf(
			"FROST canonical journal complete endpoint identity differs from its aggregate fields",
		)
	}
	otherRoleHashes := make(map[[32]byte]string)
	for name, value := range map[string]string{
		"Ethereum source endpoint":   manifest.Ethereum.SourceEndpointFingerprint,
		"Ethereum verifier endpoint": manifest.Ethereum.VerifierEndpointFingerprint,
		"runtime handshake endpoint": frost.HandshakeEndpointFingerprint,
		"Ethereum source operator":   manifest.Ethereum.SourceOperatorFingerprint,
		"Ethereum verifier operator": manifest.Ethereum.VerifierOperatorFingerprint,
		"runtime handshake operator": frost.HandshakeOperatorFingerprint,
		"runtime attestation signer": frost.AttestationSignerKeyHash,
	} {
		parsed, parseErr := frostPreSignParseBytes32(value)
		if parseErr != nil || parsed == [32]byte{} {
			return fmt.Errorf("FROST %s identity is invalid", name)
		}
		if previous, exists := otherRoleHashes[parsed]; exists {
			return fmt.Errorf(
				"FROST %s identity aliases %s",
				name,
				previous,
			)
		}
		otherRoleHashes[parsed] = name
	}
	if previous, exists := otherRoleHashes[manifest.activationAuthorityKeyHash]; exists {
		return fmt.Errorf(
			"FROST activation authority identity aliases %s",
			previous,
		)
	}
	otherRoleHashes[manifest.activationAuthorityKeyHash] = "activation authority"
	for name, value := range map[string][32]byte{
		"retained export endpoint":      sourceIdentity.Export.EndpointFingerprint,
		"retained verifier endpoint":    sourceIdentity.Verifier.EndpointFingerprint,
		"retained export TLS leaf":      sourceIdentity.Export.TLSLeafSPKIHash,
		"retained verifier TLS leaf":    sourceIdentity.Verifier.TLSLeafSPKIHash,
		"retained export backend":       sourceIdentity.Export.BackendServiceFingerprint,
		"retained verifier backend":     sourceIdentity.Verifier.BackendServiceFingerprint,
		"retained export operator":      sourceIdentity.Export.OperatorFingerprint,
		"retained verifier operator":    sourceIdentity.Verifier.OperatorFingerprint,
		"retained history signer":       sourceIdentity.HistorySignerKeyHash,
		"retained export attestation":   sourceIdentity.Export.AttestationKeyHash,
		"retained verifier attestation": sourceIdentity.Verifier.AttestationKeyHash,
	} {
		if other, exists := otherRoleHashes[value]; exists {
			return fmt.Errorf(
				"FROST %s identity aliases %s",
				name,
				other,
			)
		}
	}
	for name, value := range map[string]string{
		"Ethereum source endpoint":   manifest.Ethereum.SourceEndpointFingerprint,
		"Ethereum verifier endpoint": manifest.Ethereum.VerifierEndpointFingerprint,
		"runtime handshake endpoint": frost.HandshakeEndpointFingerprint,
	} {
		parsed, err := frostPreSignParseBytes32(value)
		if err != nil || parsed == parsedJournalValues["source endpoint fingerprint"] {
			return fmt.Errorf("FROST canonical journal source endpoint is not independent of %s", name)
		}
	}
	for name, value := range map[string]string{
		"Ethereum source operator":   manifest.Ethereum.SourceOperatorFingerprint,
		"Ethereum verifier operator": manifest.Ethereum.VerifierOperatorFingerprint,
		"runtime handshake operator": frost.HandshakeOperatorFingerprint,
	} {
		parsed, err := frostPreSignParseBytes32(value)
		if err != nil || parsed == parsedJournalValues["source operator fingerprint"] {
			return fmt.Errorf("FROST canonical journal source operator is not independent of %s", name)
		}
	}
	trustDomains := make(map[string]string)
	for name, value := range map[string]string{
		"Ethereum source":   manifest.Ethereum.SourceTrustDomainID,
		"Ethereum verifier": manifest.Ethereum.VerifierTrustDomainID,
		"runtime signer":    frost.TrustDomainID,
		"retained source":   journal.SourceTrustDomainID,
		"retained export":   sourceIdentity.Export.TrustDomainID,
		"retained verifier": sourceIdentity.Verifier.TrustDomainID,
	} {
		if strings.TrimSpace(value) == "" || value != strings.TrimSpace(value) {
			return fmt.Errorf("FROST %s trust domain is invalid", name)
		}
		if previous, exists := trustDomains[value]; exists {
			return fmt.Errorf(
				"FROST %s trust domain aliases %s",
				name,
				previous,
			)
		}
		trustDomains[value] = name
	}
	quarantine := frost.QuarantineJournal
	if strings.TrimSpace(quarantine.StoreID) == "" || len(quarantine.StoreID) > 255 {
		return fmt.Errorf("FROST quarantine journal manifest is incomplete")
	}
	if err := validateFrostPreSignQuarantineLiftAuthorities(
		manifest,
	); err != nil {
		return err
	}
	quarantineStoreFingerprint, err := frostPreSignParseBytes32(quarantine.StoreFingerprint)
	if err != nil || quarantineStoreFingerprint == [32]byte{} {
		return fmt.Errorf("invalid FROST quarantine journal store fingerprint")
	}
	quarantineClusterFingerprint, err := frostPreSignParseBytes32(quarantine.ClusterFingerprint)
	if err != nil || quarantineClusterFingerprint == [32]byte{} {
		return fmt.Errorf("invalid FROST quarantine journal cluster fingerprint")
	}
	if journal.StoreID == manifest.Ethereum.StoreID ||
		journal.StoreID == quarantine.StoreID ||
		parsedJournalValues["store fingerprint"] == quarantineStoreFingerprint ||
		parsedJournalValues["cluster fingerprint"] == quarantineClusterFingerprint ||
		durableSessionStoreFingerprint == parsedJournalValues["store fingerprint"] ||
		durableSessionStoreFingerprint == quarantineStoreFingerprint {
		return fmt.Errorf("FROST canonical journal storage identities are not independent")
	}
	for name, value := range map[string][32]byte{
		"canonical journal store":     parsedJournalValues["store fingerprint"],
		"canonical journal cluster":   parsedJournalValues["cluster fingerprint"],
		"quarantine journal store":    quarantineStoreFingerprint,
		"quarantine journal cluster":  quarantineClusterFingerprint,
		"durable native signer store": durableSessionStoreFingerprint,
	} {
		if anchorIdentity.HistoryStoreFingerprint == value ||
			anchorIdentity.HistoryClusterFingerprint == value {
			return fmt.Errorf(
				"FROST native signer anchor history identity aliases %s",
				name,
			)
		}
	}
	for name, encoded := range map[string]string{
		"Ethereum source endpoint":   manifest.Ethereum.SourceEndpointFingerprint,
		"Ethereum verifier endpoint": manifest.Ethereum.VerifierEndpointFingerprint,
		"canonical journal endpoint": journal.SourceEndpointFingerprint,
		"runtime handshake endpoint": frost.HandshakeEndpointFingerprint,
	} {
		value, err := frostPreSignParseBytes32(encoded)
		if err != nil {
			return fmt.Errorf("invalid %s fingerprint", name)
		}
		if anchorIdentity.EndpointLeafSPKIHash != [32]byte{} &&
			anchorIdentity.EndpointLeafSPKIHash == value {
			return fmt.Errorf(
				"FROST native signer anchor endpoint aliases %s",
				name,
			)
		}
	}
	for name, encoded := range map[string]string{
		"Ethereum source operator":   manifest.Ethereum.SourceOperatorFingerprint,
		"Ethereum verifier operator": manifest.Ethereum.VerifierOperatorFingerprint,
		"canonical journal operator": journal.SourceOperatorFingerprint,
		"runtime handshake operator": frost.HandshakeOperatorFingerprint,
	} {
		value, err := frostPreSignParseBytes32(encoded)
		if err != nil {
			return fmt.Errorf("invalid %s fingerprint", name)
		}
		if anchorIdentity.OperatorFingerprint == value {
			return fmt.Errorf(
				"FROST native signer anchor operator aliases %s",
				name,
			)
		}
	}
	return nil
}

func frostPreSignNativeSignerAnchorIdentity(
	manifest *frostPreSignActivationManifest,
) (tbtc.FrostNativeSignerAnchorIdentity, error) {
	result := tbtc.FrostNativeSignerAnchorIdentity{}
	if manifest == nil {
		return result, fmt.Errorf("FROST native signer anchor manifest is nil")
	}
	anchor := manifest.FrostSigner.NativeSignerAnchor
	if strings.TrimSpace(anchor.TrustDomainID) == "" ||
		len(anchor.TrustDomainID) > 128 ||
		strings.TrimSpace(anchor.HistoryStoreID) == "" ||
		len(anchor.HistoryStoreID) > 255 {
		return result, fmt.Errorf("FROST native signer anchor identity is incomplete")
	}
	result.ActivationManifestHash = manifest.manifestHash
	result.ActivationManifestSequence = manifest.ActivationSequence
	result.TrustDomainID = anchor.TrustDomainID
	result.HistoryStoreID = anchor.HistoryStoreID
	result.WitnessMaximumRecords = anchor.WitnessMaximumRecords
	result.WitnessRotationThresholdRecords =
		anchor.WitnessRotationThresholdRecords
	values := []struct {
		label       string
		encoded     string
		destination *[32]byte
		allowZero   bool
	}{
		{"protocol ID", anchor.ProtocolID, &result.ProtocolID, false},
		{"stream ID", anchor.StreamID, &result.StreamID, false},
		{"endpoint leaf SPKI hash", anchor.EndpointLeafSPKIHash, &result.EndpointLeafSPKIHash, true},
		{"online key hash", anchor.OnlineKeyHash, &result.OnlineKeyHash, false},
		{"operator fingerprint", anchor.OperatorFingerprint, &result.OperatorFingerprint, false},
		{"history store fingerprint", anchor.HistoryStoreFingerprint, &result.HistoryStoreFingerprint, false},
		{"history cluster fingerprint", anchor.HistoryClusterFingerprint, &result.HistoryClusterFingerprint, false},
		{"offline authority hash", anchor.OfflineAuthorityHash, &result.OfflineAuthorityHash, false},
		{"client SPKI hash", anchor.ClientSPKIHash, &result.ClientSPKIHash, false},
		{"signer store fingerprint", anchor.SignerStoreFingerprint, &result.SignerStoreFingerprint, false},
		{"transport binding", anchor.TransportBinding, &result.TransportBinding, false},
	}
	for _, value := range values {
		parsed, err := frostPreSignParseBytes32(value.encoded)
		if err != nil || (!value.allowZero && parsed == [32]byte{}) {
			return result, fmt.Errorf(
				"invalid FROST native signer anchor %s",
				value.label,
			)
		}
		*value.destination = parsed
	}
	return result, nil
}

func frostPreSignNativeSignerAnchorManifest(
	manifest *frostPreSignActivationManifest,
) (tbtc.FrostNativeSignerAnchorManifest, error) {
	result := tbtc.FrostNativeSignerAnchorManifest{}
	identity, err := frostPreSignNativeSignerAnchorIdentity(manifest)
	if err != nil {
		return result, err
	}
	anchor := manifest.FrostSigner.NativeSignerAnchor
	if anchor.WitnessMaximumRecords < 2 ||
		anchor.WitnessMaximumRecords > 1_000_000 ||
		anchor.WitnessRotationThresholdRecords < 2 ||
		anchor.WitnessRotationThresholdRecords >
			anchor.WitnessMaximumRecords-2 {
		return result, fmt.Errorf(
			"FROST native signer witness geometry is invalid",
		)
	}
	result.Identity = identity
	result.WitnessMaximumRecords = anchor.WitnessMaximumRecords
	result.WitnessRotationThresholdRecords =
		anchor.WitnessRotationThresholdRecords
	return result, nil
}

func frostPreSignRetainedEndpointIdentity(
	wire frostPreSignManifestRetainedEndpointIdentity,
) (tbtc.FrostRetainedGroupEndpointIdentity, error) {
	parse := func(name string, value string) ([32]byte, error) {
		parsed, err := frostPreSignParseBytes32(value)
		if err != nil {
			return [32]byte{}, fmt.Errorf("invalid retained %s: [%w]", name, err)
		}
		return parsed, nil
	}
	addressSet, err := parse("resolved address-set hash", wire.ResolvedAddressSetHash)
	if err != nil {
		return tbtc.FrostRetainedGroupEndpointIdentity{}, err
	}
	leaf, err := parse("TLS leaf SPKI hash", wire.TLSLeafSPKIHash)
	if err != nil {
		return tbtc.FrostRetainedGroupEndpointIdentity{}, err
	}
	backend, err := parse(
		"backend service fingerprint",
		wire.BackendServiceFingerprint,
	)
	if err != nil {
		return tbtc.FrostRetainedGroupEndpointIdentity{}, err
	}
	operator, err := parse("operator fingerprint", wire.OperatorFingerprint)
	if err != nil {
		return tbtc.FrostRetainedGroupEndpointIdentity{}, err
	}
	attestation, err := parse("attestation key hash", wire.AttestationKeyHash)
	if err != nil {
		return tbtc.FrostRetainedGroupEndpointIdentity{}, err
	}
	exporter, err := parse("TLS exporter protocol ID", wire.TLSExporterProtocolID)
	if err != nil {
		return tbtc.FrostRetainedGroupEndpointIdentity{}, err
	}
	fingerprint, err := parse("endpoint fingerprint", wire.EndpointFingerprint)
	if err != nil {
		return tbtc.FrostRetainedGroupEndpointIdentity{}, err
	}
	return tbtc.FrostRetainedGroupEndpointIdentity{
		Schema:                    wire.Schema,
		Role:                      wire.Role,
		TrustDomainID:             wire.TrustDomainID,
		CanonicalEndpoint:         wire.CanonicalEndpoint,
		CanonicalDNSName:          wire.CanonicalDNSName,
		ResolvedDNSName:           wire.ResolvedDNSName,
		ResolvedAddressSetHash:    addressSet,
		TLSLeafSPKIHash:           leaf,
		ServiceIdentity:           wire.ServiceIdentity,
		BackendServiceFingerprint: backend,
		OperatorFingerprint:       operator,
		AttestationKeyHash:        attestation,
		TLSExporterProtocolID:     exporter,
		EndpointFingerprint:       fingerprint,
	}, nil
}

func frostPreSignRetainedSourceIdentity(
	wire frostPreSignManifestRetainedSourceIdentity,
) (tbtc.FrostRetainedGroupHistoryIdentity, error) {
	endpointFingerprint, err := frostPreSignParseBytes32(
		wire.EndpointFingerprint,
	)
	if err != nil {
		return tbtc.FrostRetainedGroupHistoryIdentity{}, err
	}
	operatorFingerprint, err := frostPreSignParseBytes32(
		wire.OperatorFingerprint,
	)
	if err != nil {
		return tbtc.FrostRetainedGroupHistoryIdentity{}, err
	}
	historySignerKeyHash, err := frostPreSignParseBytes32(
		wire.HistorySignerKeyHash,
	)
	if err != nil {
		return tbtc.FrostRetainedGroupHistoryIdentity{}, err
	}
	exportIdentity, err := frostPreSignRetainedEndpointIdentity(wire.Export)
	if err != nil {
		return tbtc.FrostRetainedGroupHistoryIdentity{}, err
	}
	verifierIdentity, err := frostPreSignRetainedEndpointIdentity(wire.Verifier)
	if err != nil {
		return tbtc.FrostRetainedGroupHistoryIdentity{}, err
	}
	result := tbtc.FrostRetainedGroupHistoryIdentity{
		Schema:               wire.Schema,
		TrustDomainID:        wire.TrustDomainID,
		EndpointFingerprint:  endpointFingerprint,
		OperatorFingerprint:  operatorFingerprint,
		HistorySignerKeyHash: historySignerKeyHash,
		Export:               exportIdentity,
		Verifier:             verifierIdentity,
	}
	if err := tbtc.ValidateFrostRetainedGroupHistoryIdentity(result); err != nil {
		return tbtc.FrostRetainedGroupHistoryIdentity{}, err
	}
	return result, nil
}

func validateFrostPreSignQuarantineLiftAuthorities(
	manifest *frostPreSignActivationManifest,
) error {
	if manifest == nil {
		return fmt.Errorf("FROST quarantine lift manifest is nil")
	}
	frost := manifest.FrostSigner
	quarantine := frost.QuarantineJournal
	quarantineProtocolID, _ := frostPreSignParseBytes32(quarantine.ProtocolID)
	liftProtocolID, _ := frostPreSignParseBytes32(quarantine.LiftProtocolID)
	tombstoneProtocolID, _ := frostPreSignParseBytes32(quarantine.TombstoneProtocolID)
	if quarantineProtocolID == liftProtocolID ||
		quarantineProtocolID == tombstoneProtocolID ||
		liftProtocolID == tombstoneProtocolID {
		return fmt.Errorf("FROST quarantine protocol identities are not distinct")
	}

	forbidden := make(map[[32]byte]string)
	for name, value := range map[string]string{
		"runtime attestation":           frost.AttestationSignerKeyHash,
		"runtime exporter":              frost.HandshakeOperatorFingerprint,
		"retained history source":       frost.CanonicalJournal.SourceOperatorFingerprint,
		"retained history verifier":     frost.CanonicalJournal.SourceIdentity.Verifier.OperatorFingerprint,
		"retained history signer":       frost.CanonicalJournal.SourceIdentity.HistorySignerKeyHash,
		"retained export TLS leaf":      frost.CanonicalJournal.SourceIdentity.Export.TLSLeafSPKIHash,
		"retained verifier TLS leaf":    frost.CanonicalJournal.SourceIdentity.Verifier.TLSLeafSPKIHash,
		"retained export backend":       frost.CanonicalJournal.SourceIdentity.Export.BackendServiceFingerprint,
		"retained verifier backend":     frost.CanonicalJournal.SourceIdentity.Verifier.BackendServiceFingerprint,
		"retained export attestation":   frost.CanonicalJournal.SourceIdentity.Export.AttestationKeyHash,
		"retained verifier attestation": frost.CanonicalJournal.SourceIdentity.Verifier.AttestationKeyHash,
		"primary history source":        manifest.Ethereum.SourceOperatorFingerprint,
		"primary history verifier":      manifest.Ethereum.VerifierOperatorFingerprint,
	} {
		hash, err := frostPreSignParseBytes32(value)
		if err != nil || hash == [32]byte{} {
			return fmt.Errorf("FROST %s role key is invalid", name)
		}
		forbidden[hash] = name
	}
	if manifest.activationAuthorityKeyHash == [32]byte{} {
		return fmt.Errorf("FROST activation authority key is unavailable")
	}
	forbidden[manifest.activationAuthorityKeyHash] = "activation"

	checkpointHashes, err := validateFrostPreSignManifestAuthoritySet(
		"checkpoint",
		quarantine.CheckpointAuthorityThreshold,
		quarantine.CheckpointAuthorities,
		forbidden,
	)
	if err != nil {
		return err
	}
	for hash := range checkpointHashes {
		forbidden[hash] = "checkpoint authority"
	}
	checkpointPredecessorHash, err := frostPreSignParseBytes32(
		quarantine.CheckpointPredecessorHash,
	)
	if err != nil ||
		quarantine.CheckpointMinimumSequence == 0 ||
		quarantine.CheckpointMinimumSequence > 9007199254740991 ||
		(quarantine.CheckpointMinimumSequence == 1 &&
			checkpointPredecessorHash != [32]byte{}) ||
		(quarantine.CheckpointMinimumSequence > 1 &&
			checkpointPredecessorHash == [32]byte{}) {
		return fmt.Errorf(
			"FROST checkpoint transparency floor is invalid",
		)
	}
	if _, err := validateFrostPreSignManifestAuthoritySet(
		"quarantine lift",
		quarantine.LiftAuthorityThreshold,
		quarantine.LiftAuthorities,
		forbidden,
	); err != nil {
		return err
	}
	return nil
}

func validateFrostPreSignManifestAuthoritySet(
	name string,
	threshold uint64,
	authorities []frostPreSignManifestLiftAuthority,
	forbidden map[[32]byte]string,
) (map[[32]byte]bool, error) {
	if threshold < 2 || len(authorities) < 3 ||
		threshold > uint64(len(authorities)) ||
		threshold <= uint64(len(authorities))/2 {
		return nil, fmt.Errorf(
			"FROST %s authority set must be a production strict majority of at least 2-of-3",
			name,
		)
	}
	seenHashes := make(map[[32]byte]bool, len(authorities))
	previousID := ""
	for index, authority := range authorities {
		if !validFrostPreSignLiftAuthorityID(authority.AuthorityID) ||
			(index > 0 && authority.AuthorityID <= previousID) {
			return nil, fmt.Errorf(
				"FROST %s authority IDs are not canonical and strictly sorted",
				name,
			)
		}
		previousID = authority.AuthorityID
		keyHash, err := frostPreSignParseBytes32(authority.PublicKeySPKIHash)
		if err != nil || keyHash == [32]byte{} || seenHashes[keyHash] {
			return nil, fmt.Errorf(
				"FROST %s authority SPKI hashes are invalid or duplicate",
				name,
			)
		}
		if role, exists := forbidden[keyHash]; exists {
			return nil, fmt.Errorf(
				"FROST %s authority [%s] aliases the %s role",
				name,
				authority.AuthorityID,
				role,
			)
		}
		seenHashes[keyHash] = true
	}
	return seenHashes, nil
}

func validFrostPreSignLiftAuthorityID(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for index := range value {
		character := value[index]
		if !((character >= 'a' && character <= 'z') ||
			(character >= '0' && character <= '9') ||
			(index > 0 && (character == '-' || character == '_'))) {
			return false
		}
	}
	return true
}

func newFrostPreSignEthereumAdapter(
	ctx context.Context,
	tc *TbtcChain,
	manifest *frostPreSignActivationManifest,
	reader tbtc.FrostPreSignEthereumEvidenceVerifier,
	enableRelay bool,
) (*frostPreSignEthereumAdapter, error) {
	if tc == nil || tc.baseChain == nil || tc.client == nil ||
		manifest == nil || reader == nil {
		return nil, fmt.Errorf("FROST Ethereum adapter dependencies are nil")
	}
	if tc.frostWalletRegistry == nil || tc.frostSortitionPool == nil {
		return nil, fmt.Errorf("FROST wallet registry and sortition pool are required")
	}
	profile, deployments, err := frostPreSignProfileFromManifest(manifest)
	if err != nil {
		return nil, err
	}
	if new(big.Int).SetBytes(profile.DomainChainID[:]).Cmp(tc.chainID) != 0 {
		return nil, fmt.Errorf("activation manifest chain ID differs from connected Ethereum chain")
	}
	if common.Address(profile.BridgeAddress) != tc.bridgeAddress ||
		common.Address(profile.FrostRegistry) != tc.frostWalletRegistryAddr {
		return nil, fmt.Errorf("activation manifest differs from configured Bridge/FROST registry")
	}
	actualChainID, err := reader.ChainID(ctx)
	if err != nil || actualChainID == nil ||
		actualChainID.Cmp(tc.chainID) != 0 {
		return nil, fmt.Errorf(
			"activation manifest chain ID differs from Ethereum evidence reader: [%w]",
			err,
		)
	}
	expectedGenesisHash, err := frostPreSignParseBytes32(
		manifest.Ethereum.GenesisBlockHash,
	)
	if err != nil {
		return nil, err
	}
	genesisHeader, err := reader.HeaderByNumber(ctx, big.NewInt(0))
	if err != nil || genesisHeader == nil ||
		genesisHeader.Number == nil || genesisHeader.Number.Sign() != 0 ||
		genesisHeader.Hash() != common.Hash(expectedGenesisHash) {
		return nil, fmt.Errorf("activation manifest genesis block differs from connected Ethereum chain: [%w]", err)
	}
	finality, err := frostPreSignCurrentFinality(ctx, reader)
	if err != nil {
		return nil, err
	}
	adapter := &frostPreSignEthereumAdapter{
		chain:       tc,
		reader:      reader,
		fromAddress: tc.key.Address,
		profile:     profile,
		manifest:    *manifest,
		deployments: deployments,
	}
	if enableRelay {
		adapter.bridge = bind.NewBoundContract(
			common.Address(profile.BridgeAddress), frostPreSignBridgeABI,
			tc.client, tc.client, tc.client,
		)
	}
	if err := adapter.verifyDeploymentAt(ctx, finality); err != nil {
		return nil, fmt.Errorf("FROST activation manifest verification failed: [%w]", err)
	}
	return adapter, nil
}

func frostPreSignProfileFromManifest(
	manifest *frostPreSignActivationManifest,
) (tbtc.FrostPreSignActivationProfile, []frostPreSignDeploymentPin, error) {
	if err := validateFrostPreSignActivationManifest(manifest); err != nil {
		return tbtc.FrostPreSignActivationProfile{}, nil, err
	}
	profile := tbtc.FrostPreSignActivationProfile{}
	new(big.Int).SetUint64(manifest.Ethereum.ChainID).FillBytes(profile.DomainChainID[:])
	profile.ActivationManifestHash = manifest.manifestHash

	contracts := manifest.Ethereum.Contracts
	deploymentInputs := []struct {
		role     string
		name     string
		manifest frostPreSignManifestContract
		address  *[20]byte
		codeHash *[32]byte
	}{
		{"bridge", "Bridge", contracts.Bridge, &profile.BridgeAddress, &profile.BridgeCodeHash},
		{"completeRouter", "COMPLETE router", contracts.CompleteRouter, &profile.CompleteRouter, &profile.CompleteRouterCodeHash},
		{"authorizationRegistry", "authorization registry", contracts.AuthorizationRegistry, &profile.RegistryAddress, &profile.RegistryCodeHash},
		{"frostWalletRegistry", "FROST wallet registry", contracts.FrostWalletRegistry, &profile.FrostRegistry, &profile.FrostRegistryCodeHash},
		{"frostProposalValidator", "proposal validator", contracts.FrostProposalValidator, &profile.ProposalValidator, &profile.ProposalValidatorCodeHash},
		{"frostSortitionPool", "sortition pool", contracts.FrostSortitionPool, &profile.SortitionPool, &profile.SortitionPoolCodeHash},
		{"ecdsaFraudRouter", "ECDSA fraud router", contracts.ECDSAFraudRouter, nil, nil},
		{"ecdsaCutoverCoordinator", "ECDSA cutover coordinator", contracts.ECDSACutoverCoordinator, nil, nil},
	}
	deployments := make([]frostPreSignDeploymentPin, 0, len(deploymentInputs))
	for _, input := range deploymentInputs {
		pin, err := frostPreSignDeploymentPinFromManifest(
			input.role,
			input.name,
			input.manifest,
		)
		if err != nil {
			return profile, nil, err
		}
		deployments = append(deployments, pin)
		if input.address != nil {
			*input.address = pin.address
			*input.codeHash = pin.runtimeCodeHash
		}
	}
	globalDescriptorHash, err := frostPreSignLinkedLibraryDescriptorSetHash(deployments)
	if err != nil {
		return profile, nil, err
	}
	declaredGlobalDescriptorHash, err := frostPreSignParseBytes32(
		manifest.Ethereum.LinkedLibraryDescriptorSetHash,
	)
	if err != nil || globalDescriptorHash != declaredGlobalDescriptorHash {
		return profile, nil, fmt.Errorf("activation linked-library descriptor-set hash mismatch")
	}
	profile.ImplementationSetHash = frostPreSignDeploymentSetHash(deployments)

	frost := manifest.FrostSigner
	if profile.EvidenceProtocolID, err = frostPreSignParseBytes32(contracts.CompleteRouter.ProtocolID); err != nil {
		return profile, nil, err
	}
	if profile.ReservationProtocolID, err = frostPreSignParseBytes32(frost.ReservationProtocolID); err != nil {
		return profile, nil, err
	}
	if profile.SigningPolicyHash, err = frostPreSignParseBytes32(frost.SigningPolicyHash); err != nil {
		return profile, nil, err
	}
	configuredRouter, err := frostPreSignParseAddress(frost.CompleteRouterAddress)
	if err != nil || configuredRouter != profile.CompleteRouter {
		return profile, nil, fmt.Errorf("FROST signer COMPLETE router binding mismatch")
	}
	configuredRegistry, err := frostPreSignParseAddress(frost.AuthorizationRegistryAddress)
	if err != nil || configuredRegistry != profile.RegistryAddress {
		return profile, nil, fmt.Errorf("FROST signer authorization registry binding mismatch")
	}
	registryProtocol, err := frostPreSignParseBytes32(contracts.AuthorizationRegistry.ProtocolID)
	if err != nil || registryProtocol != profile.ReservationProtocolID {
		return profile, nil, fmt.Errorf("authorization registry protocol binding mismatch")
	}
	for name, contract := range map[string]frostPreSignManifestContract{
		"COMPLETE router":        contracts.CompleteRouter,
		"authorization registry": contracts.AuthorizationRegistry,
		"FROST wallet registry":  contracts.FrostWalletRegistry,
		"proposal validator":     contracts.FrostProposalValidator,
	} {
		if contract.BridgeAddress == nil {
			return profile, nil, fmt.Errorf("%s manifest lacks Bridge binding", name)
		}
		bridge, err := frostPreSignParseAddress(*contract.BridgeAddress)
		if err != nil || bridge != profile.BridgeAddress {
			return profile, nil, fmt.Errorf("%s manifest Bridge binding mismatch", name)
		}
	}
	profile.ProfileHash = profile.ComputeHash()
	if err := profile.ValidateForProduction(); err != nil {
		return profile, nil, err
	}
	return profile, deployments, nil
}

func frostPreSignDeploymentPinFromManifest(
	role string,
	name string,
	contract frostPreSignManifestContract,
) (frostPreSignDeploymentPin, error) {
	pin, err := frostPreSignDeploymentDescriptorPinFromManifest(
		role,
		name,
		contract,
	)
	if err != nil {
		return pin, err
	}
	pin.deploymentBlock = contract.DeploymentBlock
	pin.relevantEventStartBlock = contract.RelevantEventStartBlock
	if len(contract.HistoricalDeploymentEpochs) == 0 ||
		len(contract.HistoricalDeploymentEpochs) > 64 {
		return pin, fmt.Errorf("%s historical deployment epochs are missing or exceed the limit", name)
	}
	pin.historicalEpochs = make(
		[]frostPreSignDeploymentEpochPin,
		0,
		len(contract.HistoricalDeploymentEpochs),
	)
	for index, manifestEpoch := range contract.HistoricalDeploymentEpochs {
		epochContract := frostPreSignManifestContract{
			Address:                     manifestEpoch.Address,
			RuntimeCodeHash:             manifestEpoch.RuntimeCodeHash,
			ProtocolID:                  contract.ProtocolID,
			DeploymentBlock:             contract.DeploymentBlock,
			RelevantEventStartBlock:     contract.RelevantEventStartBlock,
			LinkedLibraryDescriptorHash: manifestEpoch.LinkedLibraryDescriptorHash,
			LinkedLibraries:             manifestEpoch.LinkedLibraries,
			Upgradeability:              manifestEpoch.Upgradeability,
		}
		descriptor, err := frostPreSignDeploymentDescriptorPinFromManifest(
			role,
			fmt.Sprintf("%s historical epoch %d", name, index),
			epochContract,
		)
		if err != nil {
			return pin, err
		}
		descriptor.name = name
		start, err := frostPreSignManifestFinality(manifestEpoch.Start)
		if err != nil {
			return pin, fmt.Errorf("invalid %s historical epoch [%d] start: [%w]", name, index, err)
		}
		var end *tbtc.FrostPreSignFinality
		if manifestEpoch.End != nil {
			parsedEnd, err := frostPreSignManifestFinality(*manifestEpoch.End)
			if err != nil {
				return pin, fmt.Errorf("invalid %s historical epoch [%d] end: [%w]", name, index, err)
			}
			end = &parsedEnd
		}
		if index == 0 && start.BlockNumber != contract.DeploymentBlock {
			return pin, fmt.Errorf("%s historical epochs do not start at deployment", name)
		}
		if end != nil && end.BlockNumber < start.BlockNumber {
			return pin, fmt.Errorf("%s historical epoch [%d] has an inverted range", name, index)
		}
		if index+1 < len(contract.HistoricalDeploymentEpochs) && end == nil {
			return pin, fmt.Errorf("%s historical epoch [%d] is open before the final epoch", name, index)
		}
		if index+1 == len(contract.HistoricalDeploymentEpochs) && end != nil {
			return pin, fmt.Errorf("%s final historical deployment epoch is not open", name)
		}
		if index > 0 {
			previous := pin.historicalEpochs[index-1]
			if previous.end == nil ||
				previous.end.BlockNumber == ^uint64(0) ||
				previous.end.BlockNumber+1 != start.BlockNumber {
				return pin, fmt.Errorf("%s historical deployment epochs have a gap or overlap", name)
			}
		}
		pin.historicalEpochs = append(
			pin.historicalEpochs,
			frostPreSignDeploymentEpochPin{
				start:      start,
				end:        end,
				descriptor: descriptor,
			},
		)
	}
	lastEpoch := pin.historicalEpochs[len(pin.historicalEpochs)-1]
	if contract.RelevantEventStartBlock < pin.historicalEpochs[0].start.BlockNumber ||
		(lastEpoch.end != nil &&
			contract.RelevantEventStartBlock > lastEpoch.end.BlockNumber) ||
		frostPreSignDeploymentDescriptorHash(pin) !=
			frostPreSignDeploymentDescriptorHash(lastEpoch.descriptor) {
		return pin, fmt.Errorf("%s historical deployment epochs do not cover events or current descriptor", name)
	}
	return pin, nil
}

func frostPreSignManifestFinality(
	point frostPreSignManifestPoint,
) (tbtc.FrostPreSignFinality, error) {
	blockHash, err := frostPreSignParseBytes32(point.BlockHash)
	if err != nil || point.BlockNumber == 0 || blockHash == [32]byte{} {
		return tbtc.FrostPreSignFinality{}, fmt.Errorf("manifest point is invalid")
	}
	return tbtc.FrostPreSignFinality{
		BlockNumber: point.BlockNumber,
		BlockHash:   blockHash,
	}, nil
}

func frostPreSignDeploymentDescriptorPinFromManifest(
	role string,
	name string,
	contract frostPreSignManifestContract,
) (frostPreSignDeploymentPin, error) {
	pin := frostPreSignDeploymentPin{role: role, name: name}
	var err error
	if pin.address, err = frostPreSignParseAddress(contract.Address); err != nil {
		return pin, fmt.Errorf("invalid %s address: [%w]", name, err)
	}
	if pin.runtimeCodeHash, err = frostPreSignParseBytes32(contract.RuntimeCodeHash); err != nil {
		return pin, fmt.Errorf("invalid %s runtime code hash: [%w]", name, err)
	}
	if pin.runtimeCodeHash == [32]byte{} {
		return pin, fmt.Errorf("%s runtime code hash is zero", name)
	}
	protocolID, err := frostPreSignParseBytes32(contract.ProtocolID)
	if err != nil {
		return pin, fmt.Errorf("invalid %s protocol ID: [%w]", name, err)
	}
	if protocolID == [32]byte{} {
		return pin, fmt.Errorf("%s protocol ID is zero", name)
	}
	if contract.DeploymentBlock == 0 ||
		contract.RelevantEventStartBlock < contract.DeploymentBlock {
		return pin, fmt.Errorf("invalid %s deployment/event range", name)
	}
	if pin.linkedLibraryDescriptorHash, err = frostPreSignParseBytes32(
		contract.LinkedLibraryDescriptorHash,
	); err != nil {
		return pin, fmt.Errorf("invalid %s linked-library descriptor hash: [%w]", name, err)
	}
	count := 0
	pin.linkedLibraries, err = frostPreSignLinkedLibrariesFromManifest(
		contract.LinkedLibraries,
		0,
		&count,
	)
	if err != nil {
		return pin, fmt.Errorf("invalid %s linked libraries: [%w]", name, err)
	}
	computedDescriptorHash, err := frostPreSignLinkedLibraryInventoryHash(pin.linkedLibraries)
	if err != nil || computedDescriptorHash != pin.linkedLibraryDescriptorHash {
		return pin, fmt.Errorf("%s linked-library descriptor hash mismatch", name)
	}
	pin.upgradeability = contract.Upgradeability.Kind
	switch pin.upgradeability {
	case "immutable":
		if contract.Upgradeability.ImplementationAddress != "" ||
			contract.Upgradeability.ImplementationRuntimeCodeHash != "" ||
			contract.Upgradeability.AdminAddress != "" ||
			contract.Upgradeability.AdminRuntimeCodeHash != "" ||
			contract.Upgradeability.ImplementationSlotValue != "" ||
			contract.Upgradeability.AdminSlotValue != "" {
			return pin, fmt.Errorf("immutable %s carries proxy metadata", name)
		}
	case "eip1967":
		upgradeability := contract.Upgradeability
		if pin.implementationAddress, err = frostPreSignParseAddress(upgradeability.ImplementationAddress); err != nil {
			return pin, fmt.Errorf("invalid %s implementation address: [%w]", name, err)
		}
		if pin.implementationCodeHash, err = frostPreSignParseBytes32(upgradeability.ImplementationRuntimeCodeHash); err != nil {
			return pin, fmt.Errorf("invalid %s implementation code hash: [%w]", name, err)
		}
		if pin.adminAddress, err = frostPreSignParseAddress(upgradeability.AdminAddress); err != nil {
			return pin, fmt.Errorf("invalid %s admin address: [%w]", name, err)
		}
		if pin.adminCodeHash, err = frostPreSignParseBytes32(upgradeability.AdminRuntimeCodeHash); err != nil {
			return pin, fmt.Errorf("invalid %s admin code hash: [%w]", name, err)
		}
		if pin.implementationSlotValue, err = frostPreSignParseBytes32(upgradeability.ImplementationSlotValue); err != nil {
			return pin, fmt.Errorf("invalid %s implementation slot: [%w]", name, err)
		}
		if pin.adminSlotValue, err = frostPreSignParseBytes32(upgradeability.AdminSlotValue); err != nil {
			return pin, fmt.Errorf("invalid %s admin slot: [%w]", name, err)
		}
		if !frostPreSignSlotValueBindsAddress(pin.implementationSlotValue, pin.implementationAddress) ||
			!frostPreSignSlotValueBindsAddress(pin.adminSlotValue, pin.adminAddress) ||
			pin.implementationAddress == pin.adminAddress ||
			pin.implementationAddress == pin.address || pin.adminAddress == pin.address {
			return pin, fmt.Errorf("unsafe %s EIP-1967 address/slot binding", name)
		}
		if pin.implementationCodeHash == [32]byte{} || pin.adminCodeHash == [32]byte{} {
			return pin, fmt.Errorf("%s EIP-1967 code hash is zero", name)
		}
	default:
		return pin, fmt.Errorf("unsupported %s upgradeability [%s]", name, pin.upgradeability)
	}
	return pin, nil
}

func frostPreSignLinkedLibrariesFromManifest(
	libraries []frostPreSignManifestLinkedLibrary,
	depth int,
	count *int,
) ([]frostPreSignLinkedLibraryPin, error) {
	if depth > 16 || count == nil {
		return nil, fmt.Errorf("linked-library descriptor tree is too deep")
	}
	result := make([]frostPreSignLinkedLibraryPin, 0, len(libraries))
	roles := make(map[string]struct{})
	addresses := make(map[[20]byte]struct{})
	for _, library := range libraries {
		(*count)++
		if *count > 256 || !frostPreSignValidProtocolRole(library.ProtocolRole) {
			return nil, fmt.Errorf("linked-library descriptor tree is too large or has an invalid role")
		}
		if _, exists := roles[library.ProtocolRole]; exists {
			return nil, fmt.Errorf("duplicate linked-library role [%s]", library.ProtocolRole)
		}
		roles[library.ProtocolRole] = struct{}{}
		pin := frostPreSignLinkedLibraryPin{protocolRole: library.ProtocolRole}
		var err error
		if pin.address, err = frostPreSignParseAddress(library.Address); err != nil {
			return nil, err
		}
		if _, exists := addresses[pin.address]; exists {
			return nil, fmt.Errorf("duplicate linked-library address")
		}
		addresses[pin.address] = struct{}{}
		if pin.runtimeCodeHash, err = frostPreSignParseBytes32(library.RuntimeCodeHash); err != nil ||
			pin.runtimeCodeHash == [32]byte{} {
			return nil, fmt.Errorf("invalid linked-library runtime code hash")
		}
		if pin.linkedLibraryDescriptorHash, err = frostPreSignParseBytes32(
			library.LinkedLibraryDescriptorHash,
		); err != nil {
			return nil, err
		}
		if len(library.References) == 0 {
			return nil, fmt.Errorf("linked library [%s] has no references", library.ProtocolRole)
		}
		pin.references = append([]frostPreSignManifestLinkReference{}, library.References...)
		sort.Slice(pin.references, func(i, j int) bool {
			return pin.references[i].Start < pin.references[j].Start
		})
		for index, reference := range pin.references {
			if reference.Length != 20 ||
				(index > 0 && pin.references[index-1].Start+20 > reference.Start) {
				return nil, fmt.Errorf("linked library [%s] has invalid references", library.ProtocolRole)
			}
		}
		pin.linkedLibraries, err = frostPreSignLinkedLibrariesFromManifest(
			library.LinkedLibraries,
			depth+1,
			count,
		)
		if err != nil {
			return nil, err
		}
		computedDescriptorHash, err := frostPreSignLinkedLibraryInventoryHash(pin.linkedLibraries)
		if err != nil || computedDescriptorHash != pin.linkedLibraryDescriptorHash {
			return nil, fmt.Errorf("linked library [%s] descriptor hash mismatch", library.ProtocolRole)
		}
		result = append(result, pin)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].protocolRole < result[j].protocolRole
	})
	return result, nil
}

func frostPreSignValidProtocolRole(value string) bool {
	if len(value) == 0 || len(value) > 255 {
		return false
	}
	for _, character := range []byte(value) {
		if (character >= 'A' && character <= 'Z') ||
			(character >= 'a' && character <= 'z') ||
			(character >= '0' && character <= '9') ||
			strings.ContainsRune("._:/-", rune(character)) {
			continue
		}
		return false
	}
	return true
}

type frostPreSignLinkedLibraryDescriptor struct {
	ProtocolRole    string                                `json:"protocolRole"`
	References      []frostPreSignManifestLinkReference   `json:"references"`
	LinkedLibraries []frostPreSignLinkedLibraryDescriptor `json:"linkedLibraries"`
}

func frostPreSignLinkedLibraryDescriptors(
	libraries []frostPreSignLinkedLibraryPin,
) []frostPreSignLinkedLibraryDescriptor {
	result := make([]frostPreSignLinkedLibraryDescriptor, 0, len(libraries))
	for _, library := range libraries {
		result = append(result, frostPreSignLinkedLibraryDescriptor{
			ProtocolRole:    library.protocolRole,
			References:      append([]frostPreSignManifestLinkReference{}, library.references...),
			LinkedLibraries: frostPreSignLinkedLibraryDescriptors(library.linkedLibraries),
		})
	}
	return result
}

func frostPreSignLinkedLibraryInventoryHash(
	libraries []frostPreSignLinkedLibraryPin,
) ([32]byte, error) {
	canonical, err := frostPreSignCanonicalValue(map[string]interface{}{
		"schema":          "tbtc-p2tr-linked-library-inventory/v1",
		"linkedLibraries": frostPreSignLinkedLibraryDescriptors(libraries),
	})
	if err != nil {
		return [32]byte{}, err
	}
	return sha256.Sum256(canonical), nil
}

func frostPreSignLinkedLibraryDescriptorSetHash(
	deployments []frostPreSignDeploymentPin,
) ([32]byte, error) {
	type epochDescriptor struct {
		StartBlock      uint64                                `json:"startBlock"`
		EndBlock        *uint64                               `json:"endBlock"`
		CodeKind        string                                `json:"codeKind"`
		LinkedLibraries []frostPreSignLinkedLibraryDescriptor `json:"linkedLibraries"`
	}
	type contractDescriptor struct {
		ContractRole     string                                `json:"contractRole"`
		CodeKind         string                                `json:"codeKind"`
		LinkedLibraries  []frostPreSignLinkedLibraryDescriptor `json:"linkedLibraries"`
		HistoricalEpochs []epochDescriptor                     `json:"historicalEpochs"`
	}
	contracts := make([]contractDescriptor, 0, len(deployments))
	for _, deployment := range deployments {
		codeKind := "runtime"
		if deployment.upgradeability == "eip1967" {
			codeKind = "implementation-runtime"
		}
		historicalEpochs := make(
			[]epochDescriptor,
			0,
			len(deployment.historicalEpochs),
		)
		for _, epoch := range deployment.historicalEpochs {
			epochCodeKind := "runtime"
			if epoch.descriptor.upgradeability == "eip1967" {
				epochCodeKind = "implementation-runtime"
			}
			var endBlock *uint64
			if epoch.end != nil {
				value := epoch.end.BlockNumber
				endBlock = &value
			}
			historicalEpochs = append(historicalEpochs, epochDescriptor{
				StartBlock:      epoch.start.BlockNumber,
				EndBlock:        endBlock,
				CodeKind:        epochCodeKind,
				LinkedLibraries: frostPreSignLinkedLibraryDescriptors(epoch.descriptor.linkedLibraries),
			})
		}
		contracts = append(contracts, contractDescriptor{
			ContractRole:     deployment.role,
			CodeKind:         codeKind,
			LinkedLibraries:  frostPreSignLinkedLibraryDescriptors(deployment.linkedLibraries),
			HistoricalEpochs: historicalEpochs,
		})
	}
	sort.Slice(contracts, func(i, j int) bool {
		return contracts[i].ContractRole < contracts[j].ContractRole
	})
	canonical, err := frostPreSignCanonicalValue(map[string]interface{}{
		"schema":    "tbtc-p2tr-linked-library-descriptor-set/v2",
		"contracts": contracts,
	})
	if err != nil {
		return [32]byte{}, err
	}
	return sha256.Sum256(canonical), nil
}

func frostPreSignCanonicalValue(value interface{}) ([]byte, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return frostPreSignCanonicalJSON(encoded)
}

func frostPreSignSlotValueBindsAddress(value [32]byte, address [20]byte) bool {
	return bytes.Equal(value[:12], make([]byte, 12)) && bytes.Equal(value[12:], address[:])
}

func frostPreSignDeploymentSetHash(deployments []frostPreSignDeploymentPin) [32]byte {
	hasher := sha256.New()
	hasher.Write([]byte("tbtc-frost-pre-sign-deployment-set-v2\x00"))
	for _, deployment := range deployments {
		frostPreSignWriteHashString(hasher, deployment.role)
		frostPreSignWriteHashString(hasher, deployment.name)
		frostPreSignWriteHashUint64(hasher, deployment.deploymentBlock)
		frostPreSignWriteHashUint64(hasher, deployment.relevantEventStartBlock)
		descriptorHash := frostPreSignDeploymentDescriptorHash(deployment)
		hasher.Write(descriptorHash[:])
		frostPreSignWriteHashUint64(hasher, uint64(len(deployment.historicalEpochs)))
		for _, epoch := range deployment.historicalEpochs {
			frostPreSignWriteHashUint64(hasher, epoch.start.BlockNumber)
			hasher.Write(epoch.start.BlockHash[:])
			if epoch.end == nil {
				hasher.Write([]byte{0})
			} else {
				hasher.Write([]byte{1})
				frostPreSignWriteHashUint64(hasher, epoch.end.BlockNumber)
				hasher.Write(epoch.end.BlockHash[:])
			}
			epochDescriptorHash := frostPreSignDeploymentDescriptorHash(
				epoch.descriptor,
			)
			hasher.Write(epochDescriptorHash[:])
		}
	}
	result := [32]byte{}
	copy(result[:], hasher.Sum(nil))
	return result
}

func frostPreSignDeploymentDescriptorHash(
	deployment frostPreSignDeploymentPin,
) [32]byte {
	hasher := sha256.New()
	hasher.Write([]byte("tbtc-frost-deployment-descriptor-v1\x00"))
	hasher.Write(deployment.address[:])
	hasher.Write(deployment.runtimeCodeHash[:])
	frostPreSignWriteHashString(hasher, deployment.upgradeability)
	hasher.Write(deployment.implementationAddress[:])
	hasher.Write(deployment.implementationCodeHash[:])
	hasher.Write(deployment.adminAddress[:])
	hasher.Write(deployment.adminCodeHash[:])
	hasher.Write(deployment.implementationSlotValue[:])
	hasher.Write(deployment.adminSlotValue[:])
	hasher.Write(deployment.linkedLibraryDescriptorHash[:])
	frostPreSignWriteHashUint64(hasher, uint64(len(deployment.linkedLibraries)))
	for _, library := range deployment.linkedLibraries {
		frostPreSignWriteLinkedLibraryHash(hasher, library)
	}
	result := [32]byte{}
	copy(result[:], hasher.Sum(nil))
	return result
}

func frostPreSignWriteLinkedLibraryHash(
	hasher io.Writer,
	library frostPreSignLinkedLibraryPin,
) {
	frostPreSignWriteHashString(hasher, library.protocolRole)
	_, _ = hasher.Write(library.address[:])
	_, _ = hasher.Write(library.runtimeCodeHash[:])
	_, _ = hasher.Write(library.linkedLibraryDescriptorHash[:])
	frostPreSignWriteHashUint64(hasher, uint64(len(library.references)))
	for _, reference := range library.references {
		frostPreSignWriteHashUint64(hasher, reference.Start)
		frostPreSignWriteHashUint64(hasher, reference.Length)
	}
	frostPreSignWriteHashUint64(hasher, uint64(len(library.linkedLibraries)))
	for _, child := range library.linkedLibraries {
		frostPreSignWriteLinkedLibraryHash(hasher, child)
	}
}

func frostPreSignWriteHashString(hasher io.Writer, value string) {
	frostPreSignWriteHashUint64(hasher, uint64(len(value)))
	_, _ = hasher.Write([]byte(value))
}

func frostPreSignWriteHashUint64(hasher io.Writer, value uint64) {
	buffer := [8]byte{}
	binary.BigEndian.PutUint64(buffer[:], value)
	_, _ = hasher.Write(buffer[:])
}

func frostPreSignParseAddress(value string) ([20]byte, error) {
	if !common.IsHexAddress(value) || len(value) != 42 ||
		value != strings.ToLower(value) || !strings.HasPrefix(value, "0x") {
		return [20]byte{}, fmt.Errorf("invalid activation contract address [%s]", value)
	}
	return [20]byte(common.HexToAddress(value)), nil
}

func frostPreSignParseBytes32(value string) ([32]byte, error) {
	if len(value) != 66 || value != strings.ToLower(value) ||
		!strings.HasPrefix(value, "0x") {
		return [32]byte{}, fmt.Errorf("invalid activation bytes32 value [%s]", value)
	}
	decoded, err := hex.DecodeString(strings.TrimPrefix(value, "0x"))
	if err != nil || len(decoded) != 32 {
		return [32]byte{}, fmt.Errorf("invalid activation bytes32 value [%s]", value)
	}
	result := [32]byte{}
	copy(result[:], decoded)
	return result, nil
}

func frostPreSignCurrentFinality(
	ctx context.Context,
	client interface {
		HeaderByNumber(context.Context, *big.Int) (*types.Header, error)
	},
) (*tbtc.FrostPreSignFinality, error) {
	header, err := client.HeaderByNumber(
		ctx,
		big.NewInt(int64(rpc.FinalizedBlockNumber)),
	)
	if err != nil {
		return nil, fmt.Errorf("cannot obtain finalized Ethereum header: [%w]", err)
	}
	if header == nil || header.Number == nil ||
		!header.Number.IsUint64() || header.Number.Sign() <= 0 ||
		header.Hash() == (common.Hash{}) {
		return nil, fmt.Errorf("finalized Ethereum header is invalid")
	}
	return &tbtc.FrostPreSignFinality{
		BlockNumber: header.Number.Uint64(),
		BlockHash:   [32]byte(header.Hash()),
	}, nil
}

func (adapter *frostPreSignEthereumAdapter) verifyDeploymentAt(
	ctx context.Context,
	finality *tbtc.FrostPreSignFinality,
) error {
	if err := adapter.requireCanonicalFinality(ctx, finality); err != nil {
		return err
	}
	exactReader, err := adapter.exactHashReader()
	if err != nil {
		return err
	}
	blockHash := common.Hash(finality.BlockHash)
	profile := adapter.profile
	implementationSlot := frostPreSignEIP1967Slot("eip1967.proxy.implementation")
	adminSlot := frostPreSignEIP1967Slot("eip1967.proxy.admin")
	for _, expected := range adapter.deployments {
		code, err := exactReader.CodeAtHash(
			ctx,
			common.Address(expected.address),
			blockHash,
		)
		if err != nil {
			return fmt.Errorf("cannot read %s runtime code: [%w]", expected.name, err)
		}
		if len(code) == 0 ||
			[32]byte(crypto.Keccak256Hash(code)) != expected.runtimeCodeHash {
			return fmt.Errorf("%s runtime code hash mismatch", expected.name)
		}
		ownerCode := code
		implementationValue, err := exactReader.StorageAtHash(
			ctx,
			common.Address(expected.address),
			implementationSlot,
			blockHash,
		)
		if err != nil || len(implementationValue) != 32 {
			return fmt.Errorf("cannot read %s EIP-1967 implementation slot: [%w]", expected.name, err)
		}
		adminValue, err := exactReader.StorageAtHash(
			ctx,
			common.Address(expected.address),
			adminSlot,
			blockHash,
		)
		if err != nil || len(adminValue) != 32 {
			return fmt.Errorf("cannot read %s EIP-1967 admin slot: [%w]", expected.name, err)
		}
		switch expected.upgradeability {
		case "immutable":
			if !bytes.Equal(implementationValue, make([]byte, 32)) ||
				!bytes.Equal(adminValue, make([]byte, 32)) {
				return fmt.Errorf("immutable %s has populated EIP-1967 slots", expected.name)
			}
		case "eip1967":
			if !bytes.Equal(implementationValue, expected.implementationSlotValue[:]) ||
				!bytes.Equal(adminValue, expected.adminSlotValue[:]) {
				return fmt.Errorf("%s EIP-1967 slot value mismatch", expected.name)
			}
			implementationCode, err := exactReader.CodeAtHash(
				ctx,
				common.Address(expected.implementationAddress),
				blockHash,
			)
			if err != nil || len(implementationCode) == 0 ||
				[32]byte(crypto.Keccak256Hash(implementationCode)) != expected.implementationCodeHash {
				return fmt.Errorf("%s implementation runtime code hash mismatch: [%w]", expected.name, err)
			}
			ownerCode = implementationCode
			adminCode, err := exactReader.CodeAtHash(
				ctx,
				common.Address(expected.adminAddress),
				blockHash,
			)
			if err != nil || [32]byte(crypto.Keccak256Hash(adminCode)) != expected.adminCodeHash {
				return fmt.Errorf("%s admin runtime code hash mismatch: [%w]", expected.name, err)
			}
		default:
			return fmt.Errorf("unsupported %s upgradeability", expected.name)
		}
		if err := adapter.verifyLinkedLibrariesAt(
			ctx,
			expected.name,
			ownerCode,
			expected.linkedLibraries,
			blockHash,
			exactReader,
		); err != nil {
			return err
		}
	}

	bridgeRouter, err := adapter.callAddressAtHash(
		ctx,
		common.Address(profile.BridgeAddress),
		frostPreSignBridgeABI,
		"p2trFraudRouter",
		blockHash,
	)
	if err != nil || bridgeRouter != common.Address(profile.CompleteRouter) {
		return fmt.Errorf("Bridge COMPLETE router crosslink mismatch: [%w]", err)
	}
	routerBridge, err := adapter.callAddressAtHash(
		ctx,
		common.Address(profile.CompleteRouter),
		frostPreSignCrosslinkABI,
		"bridge",
		blockHash,
	)
	if err != nil || routerBridge != common.Address(profile.BridgeAddress) {
		return fmt.Errorf("COMPLETE router Bridge crosslink mismatch: [%w]", err)
	}
	routerRegistry, err := adapter.callAddressAtHash(
		ctx,
		common.Address(profile.CompleteRouter),
		frostPreSignCrosslinkABI,
		"authorizationRegistry",
		blockHash,
	)
	if err != nil || routerRegistry != common.Address(profile.RegistryAddress) {
		return fmt.Errorf("COMPLETE router registry crosslink mismatch: [%w]", err)
	}
	for method, expected := range map[string][32]byte{
		"evidenceProtocolID":         profile.EvidenceProtocolID,
		"preauthorizationProtocolID": profile.ReservationProtocolID,
		"signingPolicyHash":          profile.SigningPolicyHash,
	} {
		actual, err := adapter.callBytes32AtHash(
			ctx,
			common.Address(profile.CompleteRouter),
			frostPreSignCrosslinkABI,
			method,
			blockHash,
		)
		if err != nil || actual != expected {
			return fmt.Errorf("COMPLETE router %s mismatch: [%w]", method, err)
		}
	}

	config, err := adapter.callAtHash(
		ctx,
		common.Address(profile.RegistryAddress),
		frostPreSignRegistryABI,
		"protocolConfig",
		blockHash,
	)
	if err != nil || len(config) != 6 {
		return fmt.Errorf("cannot read authorization registry protocol config: [%w]", err)
	}
	registryBridge := *abi.ConvertType(config[0], new(common.Address)).(*common.Address)
	registryFrost := *abi.ConvertType(config[1], new(common.Address)).(*common.Address)
	registryValidator := *abi.ConvertType(config[2], new(common.Address)).(*common.Address)
	registryChainID := *abi.ConvertType(config[3], new(*big.Int)).(**big.Int)
	registryProtocol := *abi.ConvertType(config[4], new([32]byte)).(*[32]byte)
	registryPolicy := *abi.ConvertType(config[5], new([32]byte)).(*[32]byte)
	if registryBridge != common.Address(profile.BridgeAddress) ||
		registryFrost != common.Address(profile.FrostRegistry) ||
		registryValidator != common.Address(profile.ProposalValidator) ||
		registryChainID.Cmp(new(big.Int).SetBytes(profile.DomainChainID[:])) != 0 ||
		registryProtocol != profile.ReservationProtocolID ||
		registryPolicy != profile.SigningPolicyHash {
		return fmt.Errorf("authorization registry protocol config mismatch")
	}

	sortitionPool, err := adapter.callAddressAtHash(
		ctx,
		common.Address(profile.FrostRegistry),
		frostPreSignCrosslinkABI,
		"sortitionPool",
		blockHash,
	)
	if err != nil || sortitionPool != common.Address(profile.SortitionPool) {
		return fmt.Errorf("FROST registry sortition pool crosslink mismatch: [%w]", err)
	}
	validatorBridge, err := adapter.callAddressAtHash(
		ctx,
		common.Address(profile.ProposalValidator),
		frostPreSignCrosslinkABI,
		"bridge",
		blockHash,
	)
	if err != nil || validatorBridge != common.Address(profile.BridgeAddress) {
		return fmt.Errorf("proposal validator Bridge crosslink mismatch: [%w]", err)
	}
	// Bracket every historical deployment read with a second finalized-head
	// and exact-hash check. A target header that exists is not sufficient: the
	// target must remain at or below the RPC's independently reported finalized
	// tip for the entire read set.
	return adapter.requireCanonicalFinality(ctx, finality)
}

func (adapter *frostPreSignEthereumAdapter) verifyLinkedLibrariesAt(
	ctx context.Context,
	owner string,
	ownerCode []byte,
	libraries []frostPreSignLinkedLibraryPin,
	blockHash common.Hash,
	reader frostPreSignExactHashReader,
) error {
	for _, library := range libraries {
		for _, reference := range library.references {
			if reference.Start > uint64(len(ownerCode)) ||
				reference.Start+20 < reference.Start ||
				reference.Start+20 > uint64(len(ownerCode)) ||
				!bytes.Equal(
					ownerCode[int(reference.Start):int(reference.Start+20)],
					library.address[:],
				) {
				return fmt.Errorf(
					"%s linked-library reference [%s:%d] mismatch",
					owner,
					library.protocolRole,
					reference.Start,
				)
			}
		}
		libraryCode, err := reader.CodeAtHash(
			ctx,
			common.Address(library.address),
			blockHash,
		)
		if err != nil || len(libraryCode) == 0 ||
			[32]byte(crypto.Keccak256Hash(libraryCode)) != library.runtimeCodeHash {
			return fmt.Errorf(
				"%s linked-library [%s] runtime code hash mismatch: [%w]",
				owner,
				library.protocolRole,
				err,
			)
		}
		if err := adapter.verifyLinkedLibrariesAt(
			ctx,
			owner+"/"+library.protocolRole,
			libraryCode,
			library.linkedLibraries,
			blockHash,
			reader,
		); err != nil {
			return err
		}
	}
	return nil
}

func frostPreSignEIP1967Slot(label string) common.Hash {
	value := crypto.Keccak256Hash([]byte(label)).Big()
	value.Sub(value, big.NewInt(1))
	return common.BigToHash(value)
}

func (adapter *frostPreSignEthereumAdapter) requireCanonicalFinality(
	ctx context.Context,
	finality *tbtc.FrostPreSignFinality,
) error {
	if finality == nil || finality.BlockNumber == 0 || finality.BlockHash == [32]byte{} {
		return fmt.Errorf("Ethereum finality checkpoint is invalid")
	}
	before, err := frostPreSignCurrentFinality(ctx, adapter.reader)
	if err != nil {
		return err
	}
	if finality.BlockNumber > before.BlockNumber {
		return fmt.Errorf("Ethereum checkpoint [%d] is above finalized head [%d]", finality.BlockNumber, before.BlockNumber)
	}
	if finality.BlockNumber == before.BlockNumber && finality.BlockHash != before.BlockHash {
		return fmt.Errorf("Ethereum checkpoint disagrees with finalized head")
	}
	exactReader, err := adapter.exactHashReader()
	if err != nil {
		return err
	}
	exactHeader, err := exactReader.HeaderByHash(
		ctx,
		common.Hash(finality.BlockHash),
	)
	if err != nil {
		return fmt.Errorf("cannot read exact finalized Ethereum header: [%w]", err)
	}
	if exactHeader == nil || exactHeader.Number == nil ||
		!exactHeader.Number.IsUint64() ||
		exactHeader.Number.Uint64() != finality.BlockNumber ||
		exactHeader.Hash() != common.Hash(finality.BlockHash) {
		return fmt.Errorf("exact finalized Ethereum header mismatch")
	}
	header, err := adapter.reader.HeaderByNumber(
		ctx,
		new(big.Int).SetUint64(finality.BlockNumber),
	)
	if err != nil {
		return fmt.Errorf("cannot reread finalized Ethereum header: [%w]", err)
	}
	if header == nil || [32]byte(header.Hash()) != finality.BlockHash {
		return fmt.Errorf("finalized Ethereum block hash mismatch")
	}
	after, err := frostPreSignCurrentFinality(ctx, adapter.reader)
	if err != nil {
		return err
	}
	if after.BlockNumber < before.BlockNumber ||
		finality.BlockNumber > after.BlockNumber ||
		(finality.BlockNumber == after.BlockNumber && finality.BlockHash != after.BlockHash) ||
		(before.BlockNumber == after.BlockNumber && before.BlockHash != after.BlockHash) {
		return fmt.Errorf("Ethereum finalized head changed inconsistently while verifying checkpoint")
	}
	headerAfter, err := adapter.reader.HeaderByNumber(
		ctx,
		new(big.Int).SetUint64(finality.BlockNumber),
	)
	if err != nil {
		return fmt.Errorf("cannot bracket finalized Ethereum header: [%w]", err)
	}
	if headerAfter == nil || [32]byte(headerAfter.Hash()) != finality.BlockHash {
		return fmt.Errorf("finalized Ethereum block hash changed while verifying checkpoint")
	}
	exactHeaderAfter, err := exactReader.HeaderByHash(
		ctx,
		common.Hash(finality.BlockHash),
	)
	if err != nil {
		return fmt.Errorf("cannot bracket exact finalized Ethereum header: [%w]", err)
	}
	if exactHeaderAfter == nil || exactHeaderAfter.Number == nil ||
		!exactHeaderAfter.Number.IsUint64() ||
		exactHeaderAfter.Number.Uint64() != finality.BlockNumber ||
		exactHeaderAfter.Hash() != common.Hash(finality.BlockHash) {
		return fmt.Errorf("exact finalized Ethereum header changed while verifying checkpoint")
	}
	return nil
}

func (adapter *frostPreSignEthereumAdapter) callAtHash(
	ctx context.Context,
	address common.Address,
	contractABI abi.ABI,
	method string,
	blockHash common.Hash,
	parameters ...interface{},
) ([]interface{}, error) {
	if address == (common.Address{}) || blockHash == (common.Hash{}) {
		return nil, fmt.Errorf("exact-hash contract call identity is incomplete")
	}
	exactReader, err := adapter.exactHashReader()
	if err != nil {
		return nil, err
	}
	callData, err := contractABI.Pack(method, parameters...)
	if err != nil {
		return nil, fmt.Errorf("cannot encode exact-hash call [%s]: [%w]", method, err)
	}
	output, err := exactReader.CallContractAtHash(
		ctx,
		geth.CallMsg{
			From: adapter.fromAddress,
			To:   &address,
			Data: callData,
		},
		blockHash,
	)
	if err != nil {
		return nil, err
	}
	abiMethod, ok := contractABI.Methods[method]
	if !ok {
		return nil, fmt.Errorf("exact-hash call method [%s] is absent", method)
	}
	return abiMethod.Outputs.Unpack(output)
}

func (adapter *frostPreSignEthereumAdapter) callAddressAtHash(
	ctx context.Context,
	address common.Address,
	contractABI abi.ABI,
	method string,
	blockHash common.Hash,
	parameters ...interface{},
) (common.Address, error) {
	result, err := adapter.callAtHash(
		ctx,
		address,
		contractABI,
		method,
		blockHash,
		parameters...,
	)
	if err != nil || len(result) != 1 {
		return common.Address{}, err
	}
	return *abi.ConvertType(result[0], new(common.Address)).(*common.Address), nil
}

func (adapter *frostPreSignEthereumAdapter) callBytes32AtHash(
	ctx context.Context,
	address common.Address,
	contractABI abi.ABI,
	method string,
	blockHash common.Hash,
	parameters ...interface{},
) ([32]byte, error) {
	result, err := adapter.callAtHash(
		ctx,
		address,
		contractABI,
		method,
		blockHash,
		parameters...,
	)
	if err != nil || len(result) != 1 {
		return [32]byte{}, err
	}
	return *abi.ConvertType(result[0], new([32]byte)).(*[32]byte), nil
}

func (tc *TbtcChain) PrepareFrostPreSignAuthorization(
	ctx context.Context,
	transaction *tbtc.FrostPreSignTransaction,
	walletOperators []chain.Address,
) (*tbtc.FrostPreSignAuthorizationProposal, error) {
	adapter, verifier, err := tc.frostPreSignAdapterPair()
	if err != nil {
		return nil, err
	}
	finality, err := frostPreSignMatchingCurrentFinality(
		ctx,
		adapter,
		verifier,
	)
	if err != nil {
		return nil, err
	}
	primaryProposal, err := adapter.prepareAtFinality(
		ctx,
		transaction,
		walletOperators,
		finality,
	)
	if err != nil {
		return nil, err
	}
	verifiedProposal, err := verifier.prepareAtFinality(
		ctx,
		transaction,
		walletOperators,
		finality,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"independent FROST authorization preparation failed: [%w]",
			err,
		)
	}
	if err := frostPreSignRequireMatchingEvidence(
		"authorization preparation",
		primaryProposal,
		verifiedProposal,
	); err != nil {
		return nil, err
	}
	return primaryProposal, nil
}

func (tc *TbtcChain) VerifyFrostPreSignActivationPoint(
	ctx context.Context,
	finality tbtc.FrostPreSignFinality,
) error {
	adapter, verifier, err := tc.frostPreSignAdapterPair()
	if err != nil {
		return err
	}
	if err := adapter.verifyDeploymentAt(ctx, &finality); err != nil {
		return err
	}
	if err := verifier.verifyDeploymentAt(ctx, &finality); err != nil {
		return fmt.Errorf(
			"independent FROST activation-point verification failed: [%w]",
			err,
		)
	}
	return nil
}

func (tc *TbtcChain) FrostPreSignActivationRuntimeManifest() (
	tbtc.FrostPreSignActivationRuntimeManifest,
	error,
) {
	adapter, err := tc.frostPreSignAdapter()
	if err != nil {
		return tbtc.FrostPreSignActivationRuntimeManifest{}, err
	}
	frost := adapter.manifest.FrostSigner
	parse := func(value string) ([32]byte, error) {
		return frostPreSignParseBytes32(value)
	}
	signerProtocolID, err := parse(frost.ProtocolID)
	if err != nil {
		return tbtc.FrostPreSignActivationRuntimeManifest{}, err
	}
	bitcoinOutboxProtocolID, err := parse(frost.BitcoinOutboxProtocolID)
	if err != nil {
		return tbtc.FrostPreSignActivationRuntimeManifest{}, err
	}
	attestationSignerKeyHash, err := parse(frost.AttestationSignerKeyHash)
	if err != nil {
		return tbtc.FrostPreSignActivationRuntimeManifest{}, err
	}
	retainedGroupInventoryProtocolID, err := parse(frost.RetainedGroupInventoryProtocolID)
	if err != nil {
		return tbtc.FrostPreSignActivationRuntimeManifest{}, err
	}
	linkedLibraryDescriptorSetHash, err := parse(
		adapter.manifest.Ethereum.LinkedLibraryDescriptorSetHash,
	)
	if err != nil {
		return tbtc.FrostPreSignActivationRuntimeManifest{}, err
	}
	genesisBlockHash, err := parse(adapter.manifest.Ethereum.GenesisBlockHash)
	if err != nil {
		return tbtc.FrostPreSignActivationRuntimeManifest{}, err
	}
	endpointIdentitySetHash, err := frostPreSignEndpointIdentitySetHash(
		adapter.manifest,
	)
	if err != nil {
		return tbtc.FrostPreSignActivationRuntimeManifest{}, err
	}
	quarantine := frost.QuarantineJournal
	quarantineJournalProtocolID, err := parse(quarantine.ProtocolID)
	if err != nil {
		return tbtc.FrostPreSignActivationRuntimeManifest{}, err
	}
	quarantineLiftProtocolID, err := parse(quarantine.LiftProtocolID)
	if err != nil {
		return tbtc.FrostPreSignActivationRuntimeManifest{}, err
	}
	quarantineTombstoneProtocolID, err := parse(quarantine.TombstoneProtocolID)
	if err != nil {
		return tbtc.FrostPreSignActivationRuntimeManifest{}, err
	}
	liftAuthorities := make(
		[]tbtc.FrostRetainedGroupAuthority,
		len(quarantine.LiftAuthorities),
	)
	for index, authority := range quarantine.LiftAuthorities {
		publicKeySPKIHash, err := parse(authority.PublicKeySPKIHash)
		if err != nil {
			return tbtc.FrostPreSignActivationRuntimeManifest{}, err
		}
		liftAuthorities[index] = tbtc.FrostRetainedGroupAuthority{
			AuthorityID:       authority.AuthorityID,
			PublicKeySPKIHash: publicKeySPKIHash,
		}
	}
	checkpointAuthorities := make(
		[]tbtc.FrostRetainedGroupAuthority,
		len(quarantine.CheckpointAuthorities),
	)
	for index, authority := range quarantine.CheckpointAuthorities {
		publicKeySPKIHash, err := parse(authority.PublicKeySPKIHash)
		if err != nil {
			return tbtc.FrostPreSignActivationRuntimeManifest{}, err
		}
		checkpointAuthorities[index] = tbtc.FrostRetainedGroupAuthority{
			AuthorityID:       authority.AuthorityID,
			PublicKeySPKIHash: publicKeySPKIHash,
		}
	}
	checkpointPredecessorHash, err := parse(
		quarantine.CheckpointPredecessorHash,
	)
	if err != nil {
		return tbtc.FrostPreSignActivationRuntimeManifest{}, err
	}
	verifierOperatorFingerprint, err := parse(
		adapter.manifest.Ethereum.VerifierOperatorFingerprint,
	)
	if err != nil {
		return tbtc.FrostPreSignActivationRuntimeManifest{}, err
	}
	handshakeOperatorFingerprint, err := parse(
		adapter.manifest.FrostSigner.HandshakeOperatorFingerprint,
	)
	if err != nil {
		return tbtc.FrostPreSignActivationRuntimeManifest{}, err
	}
	journal := frost.CanonicalJournal
	storeFingerprint, err := parse(journal.StoreFingerprint)
	if err != nil {
		return tbtc.FrostPreSignActivationRuntimeManifest{}, err
	}
	clusterFingerprint, err := parse(journal.ClusterFingerprint)
	if err != nil {
		return tbtc.FrostPreSignActivationRuntimeManifest{}, err
	}
	descriptorSetHash, err := parse(journal.DescriptorSetHash)
	if err != nil {
		return tbtc.FrostPreSignActivationRuntimeManifest{}, err
	}
	sourceEndpointFingerprint, err := parse(journal.SourceEndpointFingerprint)
	if err != nil {
		return tbtc.FrostPreSignActivationRuntimeManifest{}, err
	}
	sourceOperatorFingerprint, err := parse(journal.SourceOperatorFingerprint)
	if err != nil {
		return tbtc.FrostPreSignActivationRuntimeManifest{}, err
	}
	sourceIdentity, err := frostPreSignRetainedSourceIdentity(
		journal.SourceIdentity,
	)
	if err != nil {
		return tbtc.FrostPreSignActivationRuntimeManifest{}, err
	}
	quarantineStoreFingerprint, err := parse(quarantine.StoreFingerprint)
	if err != nil {
		return tbtc.FrostPreSignActivationRuntimeManifest{}, err
	}
	quarantineClusterFingerprint, err := parse(quarantine.ClusterFingerprint)
	if err != nil {
		return tbtc.FrostPreSignActivationRuntimeManifest{}, err
	}
	checkpointHash, err := parse(journal.Checkpoint.BlockHash)
	if err != nil {
		return tbtc.FrostPreSignActivationRuntimeManifest{}, err
	}
	nativeSignerAnchor, err := frostPreSignNativeSignerAnchorManifest(
		&adapter.manifest,
	)
	if err != nil {
		return tbtc.FrostPreSignActivationRuntimeManifest{}, err
	}
	return tbtc.FrostPreSignActivationRuntimeManifest{
		ManifestHash:                     adapter.profile.ActivationManifestHash,
		ActivationAuthorityKeyHash:       adapter.manifest.activationAuthorityKeyHash,
		VerifierOperatorFingerprint:      verifierOperatorFingerprint,
		HandshakeOperatorFingerprint:     handshakeOperatorFingerprint,
		DomainChainID:                    adapter.profile.DomainChainID,
		GenesisBlockHash:                 genesisBlockHash,
		ProfileHash:                      adapter.profile.ProfileHash,
		ImplementationSetHash:            adapter.profile.ImplementationSetHash,
		LinkedLibraryDescriptorSetHash:   linkedLibraryDescriptorSetHash,
		EndpointIdentitySetHash:          endpointIdentitySetHash,
		Deployments:                      frostPreSignRuntimeDeploymentEvidence(adapter.deployments),
		SignerProtocolID:                 signerProtocolID,
		ReservationProtocolID:            adapter.profile.ReservationProtocolID,
		BitcoinOutboxProtocolID:          bitcoinOutboxProtocolID,
		SigningPolicyHash:                adapter.profile.SigningPolicyHash,
		DurableSessionStoreFingerprint:   frost.DurableSessionStoreFingerprint,
		CompleteRouterAddress:            adapter.profile.CompleteRouter,
		AuthorizationRegistryAddress:     adapter.profile.RegistryAddress,
		AttestationSignerKeyHash:         attestationSignerKeyHash,
		Threshold:                        frost.Threshold,
		MaximumGroupSize:                 frost.MaximumGroupSize,
		RetainedGroupInventoryProtocolID: retainedGroupInventoryProtocolID,
		NativeSignerAnchor:               nativeSignerAnchor,
		ActivationAuthorityPublicKey:     adapter.manifest.activationAuthorityPublicKey,
		CanonicalJournal: tbtc.FrostRetainedGroupCanonicalJournalManifest{
			StoreID:            journal.StoreID,
			StoreFingerprint:   storeFingerprint,
			ClusterFingerprint: clusterFingerprint,
			Checkpoint: tbtc.FrostPreSignFinality{
				BlockNumber: journal.Checkpoint.BlockNumber,
				BlockHash:   checkpointHash,
			},
			DescriptorSetHash:         descriptorSetHash,
			SourceTrustDomainID:       journal.SourceTrustDomainID,
			SourceEndpointFingerprint: sourceEndpointFingerprint,
			SourceOperatorFingerprint: sourceOperatorFingerprint,
			SourceIdentity:            sourceIdentity,
			MinimumGeneration:         journal.MinimumGeneration,
		},
		QuarantineJournal: tbtc.FrostRetainedGroupQuarantineJournalManifest{
			ProtocolID:                   quarantineJournalProtocolID,
			LiftProtocolID:               quarantineLiftProtocolID,
			TombstoneProtocolID:          quarantineTombstoneProtocolID,
			CheckpointAuthorityThreshold: quarantine.CheckpointAuthorityThreshold,
			CheckpointAuthorities:        checkpointAuthorities,
			CheckpointMinimumSequence:    quarantine.CheckpointMinimumSequence,
			CheckpointPredecessorHash:    checkpointPredecessorHash,
			LiftAuthorityThreshold:       quarantine.LiftAuthorityThreshold,
			LiftAuthorities:              liftAuthorities,
			StoreID:                      quarantine.StoreID,
			StoreFingerprint:             quarantineStoreFingerprint,
			ClusterFingerprint:           quarantineClusterFingerprint,
			MinimumGeneration:            quarantine.MinimumGeneration,
		},
	}, nil
}

func frostPreSignEndpointIdentitySetHash(
	manifest frostPreSignActivationManifest,
) ([32]byte, error) {
	ethereum := manifest.Ethereum
	frost := manifest.FrostSigner
	type identity struct {
		role                 string
		trustDomainID        string
		endpointFingerprint  string
		tlsLeafSPKIHash      string
		operatorFingerprint  string
		backendFingerprint   string
		attestationKeyHash   string
		historySignerKeyHash string
		storeID              string
		storeFingerprint     string
		clusterFingerprint   string
	}
	identities := []identity{
		{
			role:                "ethereum-source",
			trustDomainID:       ethereum.SourceTrustDomainID,
			endpointFingerprint: ethereum.SourceEndpointFingerprint,
			operatorFingerprint: ethereum.SourceOperatorFingerprint,
			storeID:             ethereum.SourceHistoryStoreID,
			storeFingerprint:    ethereum.SourceHistoryStoreFingerprint,
		},
		{
			role:                "ethereum-verifier",
			trustDomainID:       ethereum.VerifierTrustDomainID,
			endpointFingerprint: ethereum.VerifierEndpointFingerprint,
			operatorFingerprint: ethereum.VerifierOperatorFingerprint,
			storeID:             ethereum.VerifierHistoryStoreID,
			storeFingerprint:    ethereum.VerifierHistoryStoreFingerprint,
		},
		{
			role:                 "retained-group-source",
			trustDomainID:        frost.CanonicalJournal.SourceTrustDomainID,
			endpointFingerprint:  frost.CanonicalJournal.SourceEndpointFingerprint,
			operatorFingerprint:  frost.CanonicalJournal.SourceOperatorFingerprint,
			historySignerKeyHash: frost.CanonicalJournal.SourceIdentity.HistorySignerKeyHash,
			storeID:              frost.CanonicalJournal.StoreID,
			storeFingerprint:     frost.CanonicalJournal.StoreFingerprint,
			clusterFingerprint:   frost.CanonicalJournal.ClusterFingerprint,
		},
		{
			role:                "retained-history-export",
			trustDomainID:       frost.CanonicalJournal.SourceIdentity.Export.TrustDomainID,
			endpointFingerprint: frost.CanonicalJournal.SourceIdentity.Export.EndpointFingerprint,
			tlsLeafSPKIHash:     frost.CanonicalJournal.SourceIdentity.Export.TLSLeafSPKIHash,
			operatorFingerprint: frost.CanonicalJournal.SourceIdentity.Export.OperatorFingerprint,
			backendFingerprint:  frost.CanonicalJournal.SourceIdentity.Export.BackendServiceFingerprint,
			attestationKeyHash:  frost.CanonicalJournal.SourceIdentity.Export.AttestationKeyHash,
		},
		{
			role:                "retained-history-verifier",
			trustDomainID:       frost.CanonicalJournal.SourceIdentity.Verifier.TrustDomainID,
			endpointFingerprint: frost.CanonicalJournal.SourceIdentity.Verifier.EndpointFingerprint,
			tlsLeafSPKIHash:     frost.CanonicalJournal.SourceIdentity.Verifier.TLSLeafSPKIHash,
			operatorFingerprint: frost.CanonicalJournal.SourceIdentity.Verifier.OperatorFingerprint,
			backendFingerprint:  frost.CanonicalJournal.SourceIdentity.Verifier.BackendServiceFingerprint,
			attestationKeyHash:  frost.CanonicalJournal.SourceIdentity.Verifier.AttestationKeyHash,
		},
		{
			role:                "runtime-handshake",
			trustDomainID:       frost.TrustDomainID,
			endpointFingerprint: frost.HandshakeEndpointFingerprint,
			operatorFingerprint: frost.HandshakeOperatorFingerprint,
		},
	}
	hasher := sha256.New()
	hasher.Write([]byte("tbtc-frost-endpoint-identity-set-v3\x00"))
	for _, entry := range identities {
		endpointFingerprint, err := frostPreSignParseBytes32(
			entry.endpointFingerprint,
		)
		if err != nil {
			return [32]byte{}, err
		}
		operatorFingerprint, err := frostPreSignParseBytes32(
			entry.operatorFingerprint,
		)
		if err != nil {
			return [32]byte{}, err
		}
		frostPreSignWriteHashString(hasher, entry.role)
		frostPreSignWriteHashString(hasher, entry.trustDomainID)
		hasher.Write(endpointFingerprint[:])
		hasher.Write(operatorFingerprint[:])
		for _, value := range []string{
			entry.tlsLeafSPKIHash,
			entry.backendFingerprint,
			entry.attestationKeyHash,
			entry.historySignerKeyHash,
		} {
			if value == "" {
				hasher.Write(make([]byte, 32))
				continue
			}
			parsed, err := frostPreSignParseBytes32(value)
			if err != nil || parsed == [32]byte{} {
				return [32]byte{}, fmt.Errorf(
					"invalid endpoint identity role hash",
				)
			}
			hasher.Write(parsed[:])
		}
		frostPreSignWriteHashString(hasher, entry.storeID)
		if entry.storeFingerprint == "" {
			hasher.Write(make([]byte, 32))
		} else {
			storeFingerprint, err := frostPreSignParseBytes32(
				entry.storeFingerprint,
			)
			if err != nil {
				return [32]byte{}, err
			}
			hasher.Write(storeFingerprint[:])
		}
		if entry.clusterFingerprint == "" {
			hasher.Write(make([]byte, 32))
		} else {
			clusterFingerprint, err := frostPreSignParseBytes32(
				entry.clusterFingerprint,
			)
			if err != nil {
				return [32]byte{}, err
			}
			hasher.Write(clusterFingerprint[:])
		}
	}
	result := [32]byte{}
	copy(result[:], hasher.Sum(nil))
	return result, nil
}

func frostPreSignRuntimeDeploymentEvidence(
	deployments []frostPreSignDeploymentPin,
) []tbtc.FrostPreSignDeploymentEvidence {
	result := make([]tbtc.FrostPreSignDeploymentEvidence, 0, len(deployments))
	for _, deployment := range deployments {
		epochs := make(
			[]tbtc.FrostPreSignDeploymentEpochEvidence,
			0,
			len(deployment.historicalEpochs),
		)
		for _, epoch := range deployment.historicalEpochs {
			var end *tbtc.FrostPreSignFinality
			if epoch.end != nil {
				copied := *epoch.end
				end = &copied
			}
			epochs = append(epochs, tbtc.FrostPreSignDeploymentEpochEvidence{
				Start:      epoch.start,
				End:        end,
				Descriptor: frostPreSignRuntimeDeploymentDescriptor(epoch.descriptor),
			})
		}
		result = append(result, tbtc.FrostPreSignDeploymentEvidence{
			Role:                    deployment.role,
			Name:                    deployment.name,
			DeploymentBlock:         deployment.deploymentBlock,
			RelevantEventStartBlock: deployment.relevantEventStartBlock,
			Current:                 frostPreSignRuntimeDeploymentDescriptor(deployment),
			HistoricalEpochs:        epochs,
		})
	}
	return result
}

func frostPreSignRuntimeDeploymentDescriptor(
	deployment frostPreSignDeploymentPin,
) tbtc.FrostPreSignDeploymentDescriptorEvidence {
	return tbtc.FrostPreSignDeploymentDescriptorEvidence{
		Address:                     deployment.address,
		RuntimeCodeHash:             deployment.runtimeCodeHash,
		Upgradeability:              deployment.upgradeability,
		ImplementationAddress:       deployment.implementationAddress,
		ImplementationCodeHash:      deployment.implementationCodeHash,
		AdminAddress:                deployment.adminAddress,
		AdminCodeHash:               deployment.adminCodeHash,
		ImplementationSlotValue:     deployment.implementationSlotValue,
		AdminSlotValue:              deployment.adminSlotValue,
		LinkedLibraryDescriptorHash: deployment.linkedLibraryDescriptorHash,
		LinkedLibraries:             frostPreSignRuntimeLinkedLibraries(deployment.linkedLibraries),
		DescriptorHash:              frostPreSignDeploymentDescriptorHash(deployment),
	}
}

func frostPreSignRuntimeLinkedLibraries(
	libraries []frostPreSignLinkedLibraryPin,
) []tbtc.FrostPreSignLinkedLibraryEvidence {
	result := make([]tbtc.FrostPreSignLinkedLibraryEvidence, 0, len(libraries))
	for _, library := range libraries {
		references := make(
			[]tbtc.FrostPreSignLinkedLibraryReference,
			0,
			len(library.references),
		)
		for _, reference := range library.references {
			references = append(references, tbtc.FrostPreSignLinkedLibraryReference{
				Start:  reference.Start,
				Length: reference.Length,
			})
		}
		result = append(result, tbtc.FrostPreSignLinkedLibraryEvidence{
			ProtocolRole:                library.protocolRole,
			Address:                     library.address,
			RuntimeCodeHash:             library.runtimeCodeHash,
			References:                  references,
			LinkedLibraryDescriptorHash: library.linkedLibraryDescriptorHash,
			LinkedLibraries:             frostPreSignRuntimeLinkedLibraries(library.linkedLibraries),
		})
	}
	return result
}

func (adapter *frostPreSignEthereumAdapter) prepare(
	ctx context.Context,
	transaction *tbtc.FrostPreSignTransaction,
	walletOperators []chain.Address,
) (*tbtc.FrostPreSignAuthorizationProposal, error) {
	if ctx == nil || transaction == nil {
		return nil, fmt.Errorf("FROST authorization preparation input is nil")
	}
	finality, err := frostPreSignCurrentFinality(ctx, adapter.reader)
	if err != nil {
		return nil, err
	}
	return adapter.prepareAtFinality(
		ctx,
		transaction,
		walletOperators,
		finality,
	)
}

func (adapter *frostPreSignEthereumAdapter) prepareAtFinality(
	ctx context.Context,
	transaction *tbtc.FrostPreSignTransaction,
	walletOperators []chain.Address,
	finality *tbtc.FrostPreSignFinality,
) (*tbtc.FrostPreSignAuthorizationProposal, error) {
	if ctx == nil || transaction == nil || finality == nil {
		return nil, fmt.Errorf("FROST authorization preparation input is nil")
	}
	if err := adapter.verifyDeploymentAt(ctx, finality); err != nil {
		return nil, err
	}
	blockHash := common.Hash(finality.BlockHash)
	members, err := adapter.resolveWalletMembersAt(ctx, walletOperators, blockHash)
	if err != nil {
		return nil, err
	}
	membersHash, err := frostPreSignHashABIArray("uint32[]", members)
	if err != nil {
		return nil, err
	}
	actionData, err := adapter.encodeActionData(transaction)
	if err != nil {
		return nil, err
	}
	transactionInfo := frostPreSignBitcoinTxInfo{
		Version:      transaction.Version,
		InputVector:  append([]byte{}, transaction.InputVector...),
		OutputVector: append([]byte{}, transaction.OutputVector...),
		Locktime:     transaction.Locktime,
	}
	payload, err := frostPreSignCodecABI.Methods["previewPayload"].Inputs.Pack(
		uint8(transaction.Action),
		transactionInfo,
		actionData,
		membersHash,
	)
	if err != nil {
		return nil, fmt.Errorf("cannot encode COMPLETE preview payload: [%w]", err)
	}
	result, err := adapter.callAtHash(
		ctx,
		common.Address(adapter.profile.BridgeAddress),
		frostPreSignBridgeABI,
		"previewP2TRTransactionAuthorization",
		blockHash,
		payload,
	)
	if err != nil || len(result) != 1 {
		return nil, fmt.Errorf("COMPLETE preview call failed: [%w]", err)
	}
	encodedPreview := *abi.ConvertType(result[0], new([]byte)).(*[]byte)
	decoded, err := frostPreSignCodecABI.Methods["authorizationPreview"].Inputs.Unpack(
		encodedPreview,
	)
	if err != nil || len(decoded) != 1 {
		return nil, fmt.Errorf("cannot decode COMPLETE preview: [%w]", err)
	}
	preview := *abi.ConvertType(
		decoded[0],
		new(frostPreSignAuthorizationPreview),
	).(*frostPreSignAuthorizationPreview)
	if preview.TransactionHash != [32]byte(transaction.TransactionHash) ||
		preview.WalletPubKeyHash != transaction.WalletPublicKeyHash ||
		preview.MembersIDsHash != membersHash ||
		preview.Action != uint8(transaction.Action) {
		return nil, fmt.Errorf("COMPLETE preview identity differs from local signing batch")
	}

	resourceIDs, orderedInputRoot, err := adapter.deriveResourcesAt(
		ctx,
		transaction,
		preview.WalletID,
		blockHash,
	)
	if err != nil {
		return nil, err
	}
	resourceHash, err := frostPreSignHashABIArray("bytes32[]", resourceIDs)
	if err != nil {
		return nil, err
	}
	if preview.ResourceHash != resourceHash || preview.OrderedInputRoot != orderedInputRoot {
		return nil, fmt.Errorf("COMPLETE preview resource commitments differ from local derivation")
	}

	if err := adapter.requireCanonicalFinality(ctx, finality); err != nil {
		return nil, fmt.Errorf("preparation finality changed during exact-hash reads: [%w]", err)
	}
	profile := adapter.profile
	return &tbtc.FrostPreSignAuthorizationProposal{
		Transaction:               transaction,
		WalletID:                  preview.WalletID,
		SnapshotHash:              preview.SnapshotHash,
		ResourceHash:              preview.ResourceHash,
		OrderedInputRoot:          preview.OrderedInputRoot,
		ApplyPlanHash:             preview.ApplyPlanHash,
		ApplyPlanData1:            preview.ApplyPlanData1,
		ApplyPlanData2:            preview.ApplyPlanData2,
		FeeLimitSnapshot:          preview.FeeLimitSnapshot,
		ResourceIDs:               resourceIDs,
		WalletMembersIDs:          members,
		WalletMembersIDsHash:      preview.MembersIDsHash,
		ReservationID:             preview.ReservationID,
		AuthorizationRoot:         preview.AuthorizationRoot,
		Digest:                    preview.Digest,
		DomainChainID:             profile.DomainChainID,
		ActivationManifestHash:    profile.ActivationManifestHash,
		ImplementationSetHash:     profile.ImplementationSetHash,
		BridgeAddress:             profile.BridgeAddress,
		RegistryAddress:           profile.RegistryAddress,
		CompleteRouter:            profile.CompleteRouter,
		FrostRegistry:             profile.FrostRegistry,
		ProposalValidator:         profile.ProposalValidator,
		SortitionPool:             profile.SortitionPool,
		BridgeCodeHash:            profile.BridgeCodeHash,
		RegistryCodeHash:          profile.RegistryCodeHash,
		CompleteRouterCodeHash:    profile.CompleteRouterCodeHash,
		FrostRegistryCodeHash:     profile.FrostRegistryCodeHash,
		ProposalValidatorCodeHash: profile.ProposalValidatorCodeHash,
		SortitionPoolCodeHash:     profile.SortitionPoolCodeHash,
		ReservationProtocolID:     profile.ReservationProtocolID,
		EvidenceProtocolID:        profile.EvidenceProtocolID,
		SigningPolicyHash:         profile.SigningPolicyHash,
		PreparationFinality:       *finality,
	}, nil
}

func (tc *TbtcChain) frostPreSignAdapter() (*frostPreSignEthereumAdapter, error) {
	if tc == nil || tc.frostPreSignAuthorizationAdapter == nil {
		return nil, fmt.Errorf("production FROST authorization adapter is not configured")
	}
	return tc.frostPreSignAuthorizationAdapter, nil
}

func (tc *TbtcChain) frostPreSignAdapterPair() (
	*frostPreSignEthereumAdapter,
	*frostPreSignEthereumAdapter,
	error,
) {
	if tc == nil || tc.frostPreSignAuthorizationAdapter == nil ||
		tc.frostPreSignAuthorizationVerifier == nil {
		return nil, nil, fmt.Errorf(
			"production FROST authorization verifier pair is not configured",
		)
	}
	return tc.frostPreSignAuthorizationAdapter,
		tc.frostPreSignAuthorizationVerifier,
		nil
}

func frostPreSignMatchingCurrentFinality(
	ctx context.Context,
	primary *frostPreSignEthereumAdapter,
	verifier *frostPreSignEthereumAdapter,
) (*tbtc.FrostPreSignFinality, error) {
	return frostPreSignMatchingCurrentFinalityWithRetry(
		ctx,
		primary,
		verifier,
		frostPreSignFinalityAgreementAttempts,
		frostPreSignFinalityAgreementRetryDelay,
	)
}

func frostPreSignMatchingCurrentFinalityWithRetry(
	ctx context.Context,
	primary *frostPreSignEthereumAdapter,
	verifier *frostPreSignEthereumAdapter,
	attempts int,
	retryDelay time.Duration,
) (*tbtc.FrostPreSignFinality, error) {
	if ctx == nil || primary == nil || verifier == nil ||
		primary.reader == nil || verifier.reader == nil {
		return nil, fmt.Errorf("FROST Ethereum verifier pair is incomplete")
	}
	if attempts <= 0 || retryDelay < 0 {
		return nil, fmt.Errorf("FROST Ethereum finality retry policy is invalid")
	}

	var primaryFinality *tbtc.FrostPreSignFinality
	var verifiedFinality *tbtc.FrostPreSignFinality
	for attempt := 0; attempt < attempts; attempt++ {
		var err error
		primaryFinality, err = frostPreSignCurrentFinality(
			ctx,
			primary.reader,
		)
		if err != nil {
			return nil, err
		}
		verifiedFinality, err = frostPreSignCurrentFinality(
			ctx,
			verifier.reader,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"cannot obtain independent finalized Ethereum header: [%w]",
				err,
			)
		}
		if primaryFinality.BlockNumber == verifiedFinality.BlockNumber &&
			primaryFinality.BlockHash == verifiedFinality.BlockHash {
			break
		}
		if attempt == attempts-1 {
			return nil, fmt.Errorf(
				"FROST Ethereum endpoints disagree on the current finalized block",
			)
		}
		if retryDelay == 0 {
			continue
		}
		timer := time.NewTimer(retryDelay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return nil, fmt.Errorf(
				"cannot retry independent finalized Ethereum headers: [%w]",
				ctx.Err(),
			)
		case <-timer.C:
		}
	}

	if err := primary.requireCanonicalFinality(
		ctx,
		primaryFinality,
	); err != nil {
		return nil, err
	}
	if err := verifier.requireCanonicalFinality(
		ctx,
		verifiedFinality,
	); err != nil {
		return nil, fmt.Errorf(
			"independent finalized Ethereum checkpoint verification failed: [%w]",
			err,
		)
	}
	return primaryFinality, nil
}

func (adapter *frostPreSignEthereumAdapter) resolveWalletMembersAt(
	ctx context.Context,
	operators []chain.Address,
	blockHash common.Hash,
) ([]uint32, error) {
	if len(operators) < 51 || len(operators) > 100 {
		return nil, fmt.Errorf("invalid FROST wallet seat count [%d]", len(operators))
	}
	cache := make(map[chain.Address]uint32)
	result := make([]uint32, len(operators))
	for i, operator := range operators {
		id, found := cache[operator]
		if !found {
			if !common.IsHexAddress(operator.String()) {
				return nil, fmt.Errorf("invalid FROST wallet operator address [%s]", operator)
			}
			operatorAddress := common.HexToAddress(operator.String())
			callResult, err := adapter.callAtHash(
				ctx,
				common.Address(adapter.profile.SortitionPool),
				frostPreSignCrosslinkABI,
				"getOperatorID",
				blockHash,
				operatorAddress,
			)
			if err != nil || len(callResult) != 1 {
				return nil, fmt.Errorf("cannot resolve finalized FROST seat [%d]: [%w]", i+1, err)
			}
			id = *abi.ConvertType(callResult[0], new(uint32)).(*uint32)
			if id == 0 {
				return nil, fmt.Errorf("FROST wallet seat [%d] has no sortition-pool ID", i+1)
			}
			cache[operator] = id
		}
		result[i] = id
	}
	return result, nil
}

func (adapter *frostPreSignEthereumAdapter) encodeActionData(
	transaction *tbtc.FrostPreSignTransaction,
) ([]byte, error) {
	context := transaction.ActionContext
	if context == nil {
		return nil, fmt.Errorf("FROST action authorization context is absent")
	}
	branches := 0
	for _, present := range []bool{
		context.DepositSweep != nil,
		context.Redemption != nil,
		context.MovingFunds != nil,
		context.MovedFundsSweep != nil,
	} {
		if present {
			branches++
		}
	}
	if branches != 1 {
		return nil, fmt.Errorf("FROST action authorization context has [%d] branches", branches)
	}

	switch transaction.Action {
	case tbtc.FrostPreSignActionDepositSweep:
		data := context.DepositSweep
		if data == nil || data.Proposal == nil || len(data.Deposits) == 0 ||
			len(data.Deposits) != len(data.Proposal.DepositsKeys) {
			return nil, fmt.Errorf("invalid deposit-sweep authorization context")
		}
		extra := make(
			[]tbtcabi.WalletProposalValidatorTaprootDepositExtraInfo,
			len(data.Deposits),
		)
		for i, deposit := range data.Deposits {
			if deposit == nil || !deposit.IsTaproot() || deposit.FundingTx == nil {
				return nil, fmt.Errorf("deposit [%d] lacks validated Taproot funding context", i)
			}
			extra[i] = tbtcabi.WalletProposalValidatorTaprootDepositExtraInfo{
				FundingTx: tbtcabi.BitcoinTxInfo2{
					Version:      deposit.FundingTx.SerializeVersion(),
					InputVector:  deposit.FundingTx.SerializeInputs(),
					OutputVector: deposit.FundingTx.SerializeOutputs(),
					Locktime:     deposit.FundingTx.SerializeLocktime(),
				},
				BlindingFactor:       deposit.BlindingFactor,
				WalletPubKeyHash:     deposit.WalletPublicKeyHash,
				WalletXOnlyPublicKey: *deposit.WalletXOnlyPublicKey,
				RefundPubKeyHash:     deposit.RefundPublicKeyHash,
				RefundXOnlyPublicKey: *deposit.RefundXOnlyPublicKey,
				RefundLocktime:       deposit.RefundLocktime,
			}
		}
		return frostPreSignCodecABI.Methods["depositData"].Inputs.Pack(
			frostPreSignDepositAuthorizationData{
				Proposal: convertDepositSweepProposalToAbiType(
					transaction.WalletPublicKeyHash,
					data.Proposal,
				),
				DepositsExtraInfo: extra,
				MainUtxo:          frostPreSignABIUtxo(data.MainUtxo),
			},
		)
	case tbtc.FrostPreSignActionRedemption:
		data := context.Redemption
		if data == nil || data.Proposal == nil || data.MainUtxo == nil {
			return nil, fmt.Errorf("invalid redemption authorization context")
		}
		proposal, err := convertRedemptionProposalToAbiType(
			transaction.WalletPublicKeyHash,
			data.Proposal,
		)
		if err != nil {
			return nil, err
		}
		return frostPreSignCodecABI.Methods["redemptionData"].Inputs.Pack(
			frostPreSignRedemptionAuthorizationData{
				Proposal: proposal,
				MainUtxo: frostPreSignABIUtxo(data.MainUtxo),
			},
		)
	case tbtc.FrostPreSignActionMovingFunds:
		data := context.MovingFunds
		if data == nil || data.Proposal == nil || data.MainUtxo == nil {
			return nil, fmt.Errorf("invalid moving-funds authorization context")
		}
		return frostPreSignCodecABI.Methods["movingData"].Inputs.Pack(
			frostPreSignMovingAuthorizationData{
				Proposal: tbtcabi.WalletProposalValidatorMovingFundsProposal{
					WalletPubKeyHash: transaction.WalletPublicKeyHash,
					TargetWallets:    data.Proposal.TargetWallets,
					MovingFundsTxFee: data.Proposal.MovingFundsTxFee,
				},
				MainUtxo: frostPreSignABIUtxo(data.MainUtxo),
			},
		)
	case tbtc.FrostPreSignActionMovedFundsSweep:
		data := context.MovedFundsSweep
		if data == nil || data.Proposal == nil {
			return nil, fmt.Errorf("invalid moved-funds-sweep authorization context")
		}
		return frostPreSignCodecABI.Methods["movedSweepData"].Inputs.Pack(
			frostPreSignMovedSweepAuthorizationData{
				Proposal: tbtcabi.WalletProposalValidatorMovedFundsSweepProposal{
					WalletPubKeyHash:         transaction.WalletPublicKeyHash,
					MovingFundsTxHash:        data.Proposal.MovingFundsTxHash,
					MovingFundsTxOutputIndex: data.Proposal.MovingFundsTxOutputIndex,
					MovedFundsSweepTxFee:     data.Proposal.SweepTxFee,
				},
				MainUtxo: frostPreSignABIUtxo(data.MainUtxo),
			},
		)
	default:
		return nil, fmt.Errorf("unsupported FROST action [%d]", transaction.Action)
	}
}

func frostPreSignABIUtxo(
	utxo *bitcoin.UnspentTransactionOutput,
) tbtcabi.BitcoinTxUTXO3 {
	if utxo == nil || utxo.Outpoint == nil {
		return tbtcabi.BitcoinTxUTXO3{}
	}
	return tbtcabi.BitcoinTxUTXO3{
		TxHash:        utxo.Outpoint.TransactionHash,
		TxOutputIndex: utxo.Outpoint.OutputIndex,
		TxOutputValue: uint64(utxo.Value),
	}
}

func (adapter *frostPreSignEthereumAdapter) deriveResourcesAt(
	ctx context.Context,
	transaction *tbtc.FrostPreSignTransaction,
	walletID [32]byte,
	blockHash common.Hash,
) ([][32]byte, [32]byte, error) {
	bitcoinTx := &bitcoin.Transaction{}
	if err := bitcoinTx.Deserialize(transaction.RawTransaction); err != nil {
		return nil, [32]byte{}, fmt.Errorf("cannot decode FROST transaction resources: [%w]", err)
	}
	ordered := make([][32]byte, len(bitcoinTx.Inputs))
	for i, input := range bitcoinTx.Inputs {
		if input == nil || input.Outpoint == nil {
			return nil, [32]byte{}, fmt.Errorf("transaction input [%d] has no outpoint", i)
		}
		resource, err := frostPreSignResource(
			"bitcoin-outpoint",
			[32]byte(input.Outpoint.TransactionHash),
			input.Outpoint.OutputIndex,
		)
		if err != nil {
			return nil, [32]byte{}, err
		}
		ordered[i] = resource
	}
	orderedRoot, err := frostPreSignHashABIArray("bytes32[]", ordered)
	if err != nil {
		return nil, [32]byte{}, err
	}
	mainSlot, err := frostPreSignResource("wallet-main-slot", walletID)
	if err != nil {
		return nil, [32]byte{}, err
	}
	resources := append([][32]byte{}, ordered...)
	resources = append(resources, mainSlot)

	switch transaction.Action {
	case tbtc.FrostPreSignActionDepositSweep:
		// Every physical input outpoint plus the wallet main slot is locked.
	case tbtc.FrostPreSignActionRedemption:
		proposal, err := convertRedemptionProposalToAbiType(
			transaction.WalletPublicKeyHash,
			transaction.ActionContext.Redemption.Proposal,
		)
		if err != nil {
			return nil, [32]byte{}, err
		}
		for _, script := range proposal.RedeemersOutputScripts {
			scriptHash := crypto.Keccak256Hash(script)
			keyHash := crypto.Keccak256Hash(
				append(append([]byte{}, scriptHash[:]...), transaction.WalletPublicKeyHash[:]...),
			)
			resource, err := frostPreSignResource(
				"redemption-request",
				new(big.Int).SetBytes(keyHash[:]),
			)
			if err != nil {
				return nil, [32]byte{}, err
			}
			resources = append(resources, resource)
		}
	case tbtc.FrostPreSignActionMovingFunds:
		for _, target := range transaction.ActionContext.MovingFunds.Proposal.TargetWallets {
			targetID, err := adapter.callBytes32AtHash(
				ctx,
				common.Address(adapter.profile.BridgeAddress),
				frostPreSignBridgeABI,
				"walletID",
				blockHash,
				target,
			)
			if err != nil || targetID == [32]byte{} {
				return nil, [32]byte{}, fmt.Errorf("cannot resolve moving-funds target wallet ID: [%w]", err)
			}
			resource, err := frostPreSignResource("wallet-main-slot", targetID)
			if err != nil {
				return nil, [32]byte{}, err
			}
			resources = append(resources, resource)
		}
	case tbtc.FrostPreSignActionMovedFundsSweep:
		proposal := transaction.ActionContext.MovedFundsSweep.Proposal
		var index [4]byte
		binary.BigEndian.PutUint32(index[:], proposal.MovingFundsTxOutputIndex)
		keyHash := crypto.Keccak256Hash(
			append(append([]byte{}, proposal.MovingFundsTxHash[:]...), index[:]...),
		)
		resource, err := frostPreSignResource(
			"moved-funds-request",
			new(big.Int).SetBytes(keyHash[:]),
		)
		if err != nil {
			return nil, [32]byte{}, err
		}
		resources = append(resources, resource)
	default:
		return nil, [32]byte{}, fmt.Errorf("unsupported FROST resource action [%d]", transaction.Action)
	}

	sort.Slice(resources, func(i, j int) bool {
		return bytes.Compare(resources[i][:], resources[j][:]) < 0
	})
	for i, resource := range resources {
		if resource == [32]byte{} ||
			(i > 0 && resources[i-1] == resource) {
			return nil, [32]byte{}, fmt.Errorf("derived FROST resource set is zero or ambiguous")
		}
	}
	return resources, orderedRoot, nil
}

func frostPreSignHashABIArray(kind string, value interface{}) ([32]byte, error) {
	typeValue, err := abi.NewType(kind, "", nil)
	if err != nil {
		return [32]byte{}, err
	}
	encoded, err := (abi.Arguments{{Type: typeValue}}).Pack(value)
	if err != nil {
		return [32]byte{}, err
	}
	return [32]byte(crypto.Keccak256Hash(encoded)), nil
}

func frostPreSignResource(label string, values ...interface{}) ([32]byte, error) {
	arguments := abi.Arguments{}
	encodedValues := []interface{}{"tbtc-p2tr-pre-signing-resource-v1", label}
	for _, kind := range []string{"string", "string"} {
		typeValue, _ := abi.NewType(kind, "", nil)
		arguments = append(arguments, abi.Argument{Type: typeValue})
	}
	for _, value := range values {
		var kind string
		switch value.(type) {
		case [32]byte:
			kind = "bytes32"
		case uint32:
			kind = "uint32"
		case *big.Int:
			kind = "uint256"
		default:
			return [32]byte{}, fmt.Errorf("unsupported FROST resource component [%T]", value)
		}
		typeValue, err := abi.NewType(kind, "", nil)
		if err != nil {
			return [32]byte{}, err
		}
		arguments = append(arguments, abi.Argument{Type: typeValue})
		encodedValues = append(encodedValues, value)
	}
	encoded, err := arguments.Pack(encodedValues...)
	if err != nil {
		return [32]byte{}, err
	}
	return [32]byte(crypto.Keccak256Hash(encoded)), nil
}

func (tc *TbtcChain) RelayFrostPreSignAuthorization(
	ctx context.Context,
	proposal *tbtc.FrostPreSignAuthorizationProposal,
	attestation *tbtc.FrostPreSignSeatAttestation,
) ([32]byte, error) {
	adapter, err := tc.frostPreSignAdapter()
	if err != nil {
		return [32]byte{}, err
	}
	return adapter.relay(ctx, proposal, attestation)
}

func (adapter *frostPreSignEthereumAdapter) relay(
	ctx context.Context,
	proposal *tbtc.FrostPreSignAuthorizationProposal,
	attestation *tbtc.FrostPreSignSeatAttestation,
) ([32]byte, error) {
	if ctx == nil || proposal == nil || proposal.Transaction == nil || attestation == nil {
		return [32]byte{}, fmt.Errorf("FROST authorization relay input is nil")
	}
	if adapter == nil || adapter.chain == nil || adapter.bridge == nil {
		return [32]byte{}, fmt.Errorf(
			"FROST authorization relay adapter is unavailable",
		)
	}
	if !bytes.Equal(frostPreSignUint32SliceBytes(proposal.WalletMembersIDs), frostPreSignUint32SliceBytes(attestation.WalletMembersIDs)) {
		return [32]byte{}, fmt.Errorf("FROST relay attestation wallet members differ from preview")
	}
	actionData, err := adapter.encodeActionData(proposal.Transaction)
	if err != nil {
		return [32]byte{}, err
	}
	transactionInfo := frostPreSignBitcoinTxInfo{
		Version:      proposal.Transaction.Version,
		InputVector:  proposal.Transaction.InputVector,
		OutputVector: proposal.Transaction.OutputVector,
		Locktime:     proposal.Transaction.Locktime,
	}
	payload, err := frostPreSignCodecABI.Methods["authorizePayload"].Inputs.Pack(
		uint8(proposal.Transaction.Action),
		transactionInfo,
		actionData,
		frostPreSignSeatAttestationABI{
			WalletMembersIDs:     attestation.WalletMembersIDs,
			SigningMemberIndices: attestation.SigningMemberIndices,
			Signatures:           attestation.Signatures,
		},
	)
	if err != nil {
		return [32]byte{}, fmt.Errorf("cannot encode COMPLETE authorization payload: [%w]", err)
	}

	adapter.chain.transactionMutex.Lock()
	defer adapter.chain.transactionMutex.Unlock()
	transactor, err := bind.NewKeyedTransactorWithChainID(
		adapter.chain.key.PrivateKey,
		adapter.chain.chainID,
	)
	if err != nil {
		return [32]byte{}, fmt.Errorf("cannot create COMPLETE relay transactor: [%w]", err)
	}
	transactor.Context = ctx
	nonce, err := adapter.chain.nonceManager.CurrentNonce()
	if err != nil {
		return [32]byte{}, fmt.Errorf("cannot obtain COMPLETE relay nonce: [%w]", err)
	}
	transactor.Nonce = new(big.Int).SetUint64(nonce)
	transaction, err := adapter.bridge.Transact(
		transactor,
		"authorizeP2TRTransaction",
		payload,
	)
	if err != nil {
		return [32]byte{}, fmt.Errorf("COMPLETE authorization transaction failed: [%w]", err)
	}
	adapter.chain.nonceManager.IncrementNonce()

	go adapter.chain.miningWaiter.ForceMining(
		transaction,
		transactor,
		func(options *bind.TransactOpts) (*types.Transaction, error) {
			return adapter.bridge.Transact(
				options,
				"authorizeP2TRTransaction",
				payload,
			)
		},
	)
	return [32]byte(transaction.Hash()), nil
}

func frostPreSignUint32SliceBytes(values []uint32) []byte {
	result := make([]byte, len(values)*4)
	for i, value := range values {
		binary.BigEndian.PutUint32(result[i*4:], value)
	}
	return result
}

func (tc *TbtcChain) WaitForFrostPreSignAuthorizationFinality(
	ctx context.Context,
	relayTransactionHash [32]byte,
	proposal *tbtc.FrostPreSignAuthorizationProposal,
) (*tbtc.FrostPreSignFinality, error) {
	adapter, verifier, err := tc.frostPreSignAdapterPair()
	if err != nil {
		return nil, err
	}
	verifiedFinality, err := verifier.waitForFinality(
		ctx,
		relayTransactionHash,
		proposal,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"independent COMPLETE relay finality verification failed: [%w]",
			err,
		)
	}
	primaryFinality, err := adapter.waitForFinality(
		ctx,
		relayTransactionHash,
		proposal,
	)
	if err != nil {
		return nil, err
	}
	if err := frostPreSignRequireMatchingEvidence(
		"COMPLETE relay finality",
		primaryFinality,
		verifiedFinality,
	); err != nil {
		return nil, err
	}
	return primaryFinality, nil
}

func (adapter *frostPreSignEthereumAdapter) waitForFinality(
	ctx context.Context,
	relayTransactionHash [32]byte,
	proposal *tbtc.FrostPreSignAuthorizationProposal,
) (*tbtc.FrostPreSignFinality, error) {
	if ctx == nil || relayTransactionHash == [32]byte{} || proposal == nil ||
		proposal.Transaction == nil {
		return nil, fmt.Errorf("FROST finality input is invalid")
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		// A pre-finality receipt is provisional. Re-read it on every poll so a
		// transaction re-included at a different canonical block after a reorg
		// can still reach finality.
		receipt, err := adapter.reader.TransactionReceipt(
			ctx,
			common.Hash(relayTransactionHash),
		)
		if err != nil && err != geth.NotFound {
			return nil, fmt.Errorf("cannot obtain COMPLETE relay receipt: [%w]", err)
		}
		if err != geth.NotFound && receipt != nil {
			if receipt.BlockNumber == nil || !receipt.BlockNumber.IsUint64() ||
				receipt.BlockNumber.Sign() <= 0 {
				return nil, fmt.Errorf(
					"COMPLETE relay transaction has no valid inclusion block",
				)
			}
			finalized, err := frostPreSignCurrentFinality(ctx, adapter.reader)
			if err != nil {
				return nil, err
			}
			if finalized.BlockNumber >= receipt.BlockNumber.Uint64() {
				header, err := adapter.reader.HeaderByNumber(ctx, receipt.BlockNumber)
				if err != nil {
					return nil, fmt.Errorf(
						"cannot verify COMPLETE receipt block: [%w]",
						err,
					)
				}
				if header != nil && header.Hash() == receipt.BlockHash {
					if receipt.Status != types.ReceiptStatusSuccessful {
						return nil, fmt.Errorf(
							"COMPLETE relay transaction reverted",
						)
					}
					sequence, logIndex, err := adapter.validateAuthorizationReceipt(
						receipt,
						relayTransactionHash,
						proposal,
					)
					if err != nil {
						return nil, err
					}
					return &tbtc.FrostPreSignFinality{
						RelayTransactionHash:  relayTransactionHash,
						BlockNumber:           receipt.BlockNumber.Uint64(),
						BlockHash:             [32]byte(receipt.BlockHash),
						TransactionIndex:      uint32(receipt.TransactionIndex),
						LogIndex:              logIndex,
						AuthorizationSequence: sequence,
					}, nil
				}
				// The observed receipt was orphaned. Continue polling for the
				// same transaction's canonical re-inclusion.
			}
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

func (adapter *frostPreSignEthereumAdapter) validateAuthorizationReceipt(
	receipt *types.Receipt,
	relayTransactionHash [32]byte,
	proposal *tbtc.FrostPreSignAuthorizationProposal,
) ([32]byte, uint32, error) {
	if receipt == nil || proposal == nil || proposal.Transaction == nil ||
		receipt.TxHash != common.Hash(relayTransactionHash) ||
		receipt.BlockHash == (common.Hash{}) ||
		receipt.BlockNumber == nil ||
		!receipt.BlockNumber.IsUint64() ||
		receipt.BlockNumber.Sign() <= 0 ||
		uint64(receipt.TransactionIndex) > uint64(math.MaxUint32) {
		return [32]byte{}, 0, fmt.Errorf(
			"COMPLETE receipt transaction identity mismatch",
		)
	}
	authorizedEvent := frostPreSignRegistryABI.Events["P2TRPreSigningReservationAuthorized"]
	advancedEvent := frostPreSignRegistryABI.Events["P2TRAuthorizedVariantAdvanced"]
	registryAddress := common.Address(adapter.profile.RegistryAddress)
	var authorizedLog, advancedLog *types.Log
	for _, logEntry := range receipt.Logs {
		if logEntry == nil || logEntry.Address != registryAddress || len(logEntry.Topics) == 0 {
			continue
		}
		if logEntry.Removed ||
			logEntry.TxHash != receipt.TxHash ||
			logEntry.BlockHash != receipt.BlockHash ||
			logEntry.BlockNumber != receipt.BlockNumber.Uint64() ||
			logEntry.TxIndex != receipt.TransactionIndex {
			return [32]byte{}, 0, fmt.Errorf(
				"COMPLETE receipt log transaction identity mismatch",
			)
		}
		switch logEntry.Topics[0] {
		case authorizedEvent.ID:
			if authorizedLog != nil {
				return [32]byte{}, 0, fmt.Errorf("COMPLETE receipt has duplicate reservation events")
			}
			authorizedLog = logEntry
		case advancedEvent.ID:
			if advancedLog != nil {
				return [32]byte{}, 0, fmt.Errorf("COMPLETE receipt has duplicate variant events")
			}
			advancedLog = logEntry
		}
	}
	if authorizedLog == nil || advancedLog == nil ||
		len(authorizedLog.Topics) != 4 || len(advancedLog.Topics) != 4 {
		return [32]byte{}, 0, fmt.Errorf("COMPLETE receipt lacks the exact reservation/variant event pair")
	}
	if [32]byte(authorizedLog.Topics[1]) != proposal.ReservationID ||
		[32]byte(authorizedLog.Topics[2]) != [32]byte(proposal.Transaction.TransactionHash) ||
		[32]byte(authorizedLog.Topics[3]) != proposal.WalletID ||
		authorizedLog.Topics[1] != advancedLog.Topics[1] ||
		authorizedLog.Topics[2] != advancedLog.Topics[2] {
		return [32]byte{}, 0, fmt.Errorf("COMPLETE receipt indexed identity mismatch")
	}
	data, err := authorizedEvent.Inputs.NonIndexed().Unpack(authorizedLog.Data)
	if err != nil || len(data) != 4 {
		return [32]byte{}, 0, fmt.Errorf("cannot decode COMPLETE reservation event: [%w]", err)
	}
	root := *abi.ConvertType(data[0], new([32]byte)).(*[32]byte)
	snapshot := *abi.ConvertType(data[1], new([32]byte)).(*[32]byte)
	resource := *abi.ConvertType(data[2], new([32]byte)).(*[32]byte)
	action := *abi.ConvertType(data[3], new(uint8)).(*uint8)
	if root != proposal.AuthorizationRoot || snapshot != proposal.SnapshotHash ||
		resource != proposal.ResourceHash || action != uint8(proposal.Transaction.Action) {
		return [32]byte{}, 0, fmt.Errorf("COMPLETE receipt unindexed commitment mismatch")
	}
	sequence := [32]byte(advancedLog.Topics[3])
	if sequence == [32]byte{} {
		return [32]byte{}, 0, fmt.Errorf("COMPLETE authorization sequence is zero")
	}
	if uint64(advancedLog.Index) > uint64(math.MaxUint32) {
		return [32]byte{}, 0, fmt.Errorf(
			"COMPLETE authorization log index overflows",
		)
	}
	return sequence, uint32(advancedLog.Index), nil
}

func (tc *TbtcChain) CurrentFrostPreSignFinality(
	ctx context.Context,
) (*tbtc.FrostPreSignFinality, error) {
	adapter, verifier, err := tc.frostPreSignAdapterPair()
	if err != nil {
		return nil, err
	}
	return frostPreSignMatchingCurrentFinality(ctx, adapter, verifier)
}

func (tc *TbtcChain) ReadFrostPreSignAuthorizationState(
	ctx context.Context,
	proposal *tbtc.FrostPreSignAuthorizationProposal,
	finality tbtc.FrostPreSignFinality,
) (*tbtc.FrostPreSignAuthorizationState, error) {
	adapter, verifier, err := tc.frostPreSignAdapterPair()
	if err != nil {
		return nil, err
	}
	primaryState, err := adapter.readAuthorizationState(
		ctx,
		proposal,
		finality,
	)
	if err != nil {
		return nil, err
	}
	verifiedState, err := verifier.readAuthorizationState(
		ctx,
		proposal,
		finality,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"independent FROST authorization state verification failed: [%w]",
			err,
		)
	}
	if err := frostPreSignRequireMatchingEvidence(
		"authorization state",
		primaryState,
		verifiedState,
	); err != nil {
		return nil, err
	}
	return primaryState, nil
}

func (adapter *frostPreSignEthereumAdapter) readAuthorizationState(
	ctx context.Context,
	proposal *tbtc.FrostPreSignAuthorizationProposal,
	finality tbtc.FrostPreSignFinality,
) (*tbtc.FrostPreSignAuthorizationState, error) {
	if ctx == nil || proposal == nil || proposal.Transaction == nil {
		return nil, fmt.Errorf("FROST authorization state input is nil")
	}
	if err := adapter.verifyDeploymentAt(ctx, &finality); err != nil {
		return nil, err
	}
	blockHash := common.Hash(finality.BlockHash)

	lifecycle, err := adapter.callAtHash(
		ctx,
		common.Address(adapter.profile.BridgeAddress),
		frostPreSignBridgeABI,
		"frostLifecycleContext",
		blockHash,
		proposal.Transaction.WalletPublicKeyHash,
	)
	if err != nil || len(lifecycle) != 2 {
		return nil, fmt.Errorf("cannot read finalized FROST lifecycle context: [%w]", err)
	}
	lifecycleRegistry := *abi.ConvertType(lifecycle[0], new(common.Address)).(*common.Address)
	lifecycleWalletID := *abi.ConvertType(lifecycle[1], new([32]byte)).(*[32]byte)

	walletResult, err := adapter.callAtHash(
		ctx,
		common.Address(adapter.profile.BridgeAddress),
		frostPreSignBridgeABI,
		"wallets",
		blockHash,
		proposal.Transaction.WalletPublicKeyHash,
	)
	if err != nil || len(walletResult) != 1 {
		return nil, fmt.Errorf("cannot read finalized Bridge wallet: [%w]", err)
	}
	wallet := *abi.ConvertType(walletResult[0], new(frostPreSignWalletABI)).(*frostPreSignWalletABI)

	frostWalletResult, err := adapter.callAtHash(
		ctx,
		common.Address(adapter.profile.FrostRegistry),
		frostPreSignCrosslinkABI,
		"getWallet",
		blockHash,
		proposal.WalletID,
	)
	if err != nil || len(frostWalletResult) != 1 {
		return nil, fmt.Errorf("cannot read finalized FROST registry wallet: [%w]", err)
	}
	frostWallet := *abi.ConvertType(
		frostWalletResult[0],
		new(frostPreSignRegistryWalletABI),
	).(*frostPreSignRegistryWalletABI)

	activeReservation, err := adapter.callBytes32AtHash(
		ctx,
		common.Address(adapter.profile.RegistryAddress),
		frostPreSignRegistryABI,
		"activeReservation",
		blockHash,
		proposal.Transaction.WalletPublicKeyHash,
	)
	if err != nil {
		return nil, fmt.Errorf("cannot read finalized active reservation: [%w]", err)
	}
	reservation, err := adapter.callAtHash(
		ctx,
		common.Address(adapter.profile.RegistryAddress),
		frostPreSignRegistryABI,
		"getReservation",
		blockHash,
		proposal.ReservationID,
	)
	if err != nil || len(reservation) != 11 {
		return nil, fmt.Errorf("cannot read finalized reservation: [%w]", err)
	}
	variant, err := adapter.callAtHash(
		ctx,
		common.Address(adapter.profile.RegistryAddress),
		frostPreSignRegistryABI,
		"getAuthorizedVariantStatus",
		blockHash,
		[32]byte(proposal.Transaction.TransactionHash),
	)
	if err != nil || len(variant) != 6 {
		return nil, fmt.Errorf("cannot read finalized authorized variant: [%w]", err)
	}
	latest, err := adapter.callAtHash(
		ctx,
		common.Address(adapter.profile.RegistryAddress),
		frostPreSignRegistryABI,
		"latestAuthorizedVariant",
		blockHash,
		proposal.ReservationID,
	)
	if err != nil || len(latest) != 3 {
		return nil, fmt.Errorf("cannot read finalized latest variant: [%w]", err)
	}

	if err := adapter.requireCanonicalFinality(ctx, &finality); err != nil {
		return nil, fmt.Errorf("authorization-state finality changed during exact-hash reads: [%w]", err)
	}
	profile := adapter.profile
	return &tbtc.FrostPreSignAuthorizationState{
		Finality:                           finality,
		DomainChainID:                      profile.DomainChainID,
		ActivationManifestHash:             profile.ActivationManifestHash,
		ImplementationSetHash:              profile.ImplementationSetHash,
		BridgeAddress:                      profile.BridgeAddress,
		RegistryAddress:                    profile.RegistryAddress,
		CompleteRouter:                     profile.CompleteRouter,
		FrostRegistry:                      profile.FrostRegistry,
		ProposalValidator:                  profile.ProposalValidator,
		SortitionPool:                      profile.SortitionPool,
		BridgeCodeHash:                     profile.BridgeCodeHash,
		RegistryCodeHash:                   profile.RegistryCodeHash,
		CompleteRouterCodeHash:             profile.CompleteRouterCodeHash,
		FrostRegistryCodeHash:              profile.FrostRegistryCodeHash,
		ProposalValidatorCodeHash:          profile.ProposalValidatorCodeHash,
		SortitionPoolCodeHash:              profile.SortitionPoolCodeHash,
		ReservationProtocolID:              profile.ReservationProtocolID,
		EvidenceProtocolID:                 profile.EvidenceProtocolID,
		SigningPolicyHash:                  profile.SigningPolicyHash,
		WalletActive:                       lifecycleRegistry == common.Address(profile.FrostRegistry) && lifecycleWalletID == proposal.WalletID && wallet.EcdsaWalletID == [32]byte{} && (wallet.State == 1 || wallet.State == 2),
		WalletID:                           lifecycleWalletID,
		WalletPublicKeyHash:                proposal.Transaction.WalletPublicKeyHash,
		WalletMembersIDsHash:               frostWallet.MembersIdsHash,
		WalletXOnlyOutputKey:               frostWallet.XOnlyOutputKey,
		ActiveReservationID:                activeReservation,
		ReservationWalletID:                *abi.ConvertType(reservation[0], new([32]byte)).(*[32]byte),
		ReservationWalletPublicKeyHash:     *abi.ConvertType(reservation[1], new([20]byte)).(*[20]byte),
		ReservationSnapshotHash:            *abi.ConvertType(reservation[3], new([32]byte)).(*[32]byte),
		ReservationResourceHash:            *abi.ConvertType(reservation[4], new([32]byte)).(*[32]byte),
		ReservationOrderedInputRoot:        *abi.ConvertType(reservation[5], new([32]byte)).(*[32]byte),
		ReservationApplyPlanData1:          *abi.ConvertType(reservation[6], new([32]byte)).(*[32]byte),
		ReservationApplyPlanData2:          *abi.ConvertType(reservation[7], new([32]byte)).(*[32]byte),
		ReservationFeeLimitSnapshot:        *abi.ConvertType(reservation[8], new(uint64)).(*uint64),
		ReservationAction:                  tbtc.FrostPreSignAction(*abi.ConvertType(reservation[9], new(uint8)).(*uint8)),
		ReservationActive:                  *abi.ConvertType(reservation[10], new(uint8)).(*uint8) == 1,
		VariantTransactionHash:             proposal.Transaction.TransactionHash,
		VariantReservationID:               *abi.ConvertType(variant[0], new([32]byte)).(*[32]byte),
		VariantAuthorizationRoot:           *abi.ConvertType(variant[1], new([32]byte)).(*[32]byte),
		VariantApplyPlanHash:               *abi.ConvertType(variant[2], new([32]byte)).(*[32]byte),
		VariantAuthorizationSequence:       frostPreSignUint256Word(*abi.ConvertType(variant[3], new(*big.Int)).(**big.Int)),
		VariantFraudDefenseAuthorized:      *abi.ConvertType(variant[4], new(bool)).(*bool),
		VariantSigningAllowed:              *abi.ConvertType(variant[5], new(bool)).(*bool),
		LatestVariantTransactionHash:       bitcoin.Hash(*abi.ConvertType(latest[0], new([32]byte)).(*[32]byte)),
		LatestVariantAuthorizationSequence: frostPreSignUint256Word(*abi.ConvertType(latest[1], new(*big.Int)).(**big.Int)),
		LatestVariantSigningAllowed:        *abi.ConvertType(latest[2], new(bool)).(*bool),
	}, nil
}

func frostPreSignUint256Word(value *big.Int) [32]byte {
	result := [32]byte{}
	if value != nil && value.Sign() >= 0 && value.BitLen() <= 256 {
		value.FillBytes(result[:])
	}
	return result
}

func (tc *TbtcChain) GetCanonicalFrostBitcoinBroadcastAuthorizationStatus(
	ctx context.Context,
	request *tbtc.FrostBitcoinBroadcastAuthorizationStatusRequest,
) (*tbtc.FrostBitcoinBroadcastAuthorizationStatus, error) {
	adapter, verifier, err := tc.frostPreSignAdapterPair()
	if err != nil {
		return nil, err
	}
	current, err := frostPreSignMatchingCurrentFinality(
		ctx,
		adapter,
		verifier,
	)
	if err != nil {
		return nil, err
	}
	primaryStatus, err := adapter.canonicalBroadcastStatus(
		ctx,
		request,
		current,
	)
	if err != nil {
		return nil, err
	}
	verifiedStatus, err := verifier.canonicalBroadcastStatus(
		ctx,
		request,
		current,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"independent Bitcoin broadcast authorization verification failed: [%w]",
			err,
		)
	}
	if err := frostPreSignRequireMatchingEvidence(
		"Bitcoin broadcast authorization",
		primaryStatus,
		verifiedStatus,
	); err != nil {
		return nil, err
	}
	return primaryStatus, nil
}

func frostPreSignRequireMatchingEvidence(
	evidenceName string,
	primary interface{},
	verified interface{},
) error {
	if !reflect.DeepEqual(primary, verified) {
		return fmt.Errorf(
			"FROST Ethereum endpoints disagree on %s",
			evidenceName,
		)
	}
	return nil
}

func (adapter *frostPreSignEthereumAdapter) canonicalBroadcastStatus(
	ctx context.Context,
	request *tbtc.FrostBitcoinBroadcastAuthorizationStatusRequest,
	current *tbtc.FrostPreSignFinality,
) (*tbtc.FrostBitcoinBroadcastAuthorizationStatus, error) {
	if ctx == nil || request == nil || request.FinalizedBlock == 0 ||
		request.FinalizedBlockHash == [32]byte{} ||
		request.VariantSequence.AuthorizationSequence == [32]byte{} ||
		current == nil {
		return nil, fmt.Errorf("Bitcoin broadcast authorization request is invalid")
	}
	requestHash := request.ComputeHash()
	historical := tbtc.FrostPreSignFinality{
		BlockNumber: request.FinalizedBlock,
		BlockHash:   request.FinalizedBlockHash,
	}
	if err := adapter.requireCanonicalFinality(ctx, &historical); err != nil {
		return nil, err
	}
	if err := adapter.validateHistoricalBroadcastEvent(ctx, request); err != nil {
		return nil, err
	}
	canonical, err := adapter.validateBroadcastAuthorizationAt(
		ctx,
		request,
		common.Hash(request.FinalizedBlockHash),
		true,
	)
	if err != nil {
		return nil, err
	}
	if err := adapter.requireCanonicalFinality(ctx, &historical); err != nil {
		return nil, fmt.Errorf("historical broadcast finality changed during exact-hash reads: [%w]", err)
	}
	if !canonical {
		return &tbtc.FrostBitcoinBroadcastAuthorizationStatus{
			RequestHash: requestHash,
			Canonical:   false,
		}, nil
	}

	if err := adapter.verifyDeploymentAt(ctx, current); err != nil {
		return nil, err
	}
	allowed := false
	if request.ActivationProfileHash == adapter.profile.ProfileHash &&
		request.ActiveActivationProfileHash == adapter.profile.ProfileHash {
		allowed, err = adapter.validateBroadcastAuthorizationAt(
			ctx,
			request,
			common.Hash(current.BlockHash),
			false,
		)
		if err != nil {
			return nil, err
		}
		if err := adapter.requireCanonicalFinality(ctx, current); err != nil {
			return nil, fmt.Errorf("current broadcast finality changed during exact-hash reads: [%w]", err)
		}
	}
	return &tbtc.FrostBitcoinBroadcastAuthorizationStatus{
		RequestHash:      requestHash,
		Canonical:        true,
		BroadcastAllowed: allowed,
	}, nil
}

func (adapter *frostPreSignEthereumAdapter) validateHistoricalBroadcastEvent(
	ctx context.Context,
	request *tbtc.FrostBitcoinBroadcastAuthorizationStatusRequest,
) error {
	blockHash := common.Hash(request.FinalizedBlockHash)
	event := frostPreSignRegistryABI.Events["P2TRAuthorizedVariantAdvanced"]
	logs, err := adapter.reader.FilterLogs(ctx, geth.FilterQuery{
		BlockHash: &blockHash,
		Addresses: []common.Address{common.Address(adapter.profile.RegistryAddress)},
		Topics: [][]common.Hash{
			{event.ID},
			{common.Hash(request.ReservationID)},
			{common.Hash(request.TransactionHash)},
			{common.Hash(request.VariantSequence.AuthorizationSequence)},
		},
	})
	if err != nil {
		return fmt.Errorf("cannot read historical COMPLETE authorization event: [%w]", err)
	}
	if len(logs) != 1 || logs[0].Index != uint(request.FinalizedLogIndex) ||
		logs[0].TxIndex != uint(request.FinalizedTransactionIndex) ||
		logs[0].Removed ||
		logs[0].Address != common.Address(adapter.profile.RegistryAddress) ||
		logs[0].BlockHash != blockHash ||
		logs[0].TxHash == (common.Hash{}) ||
		len(logs[0].Topics) != 4 ||
		logs[0].Topics[0] != event.ID ||
		logs[0].Topics[1] != common.Hash(request.ReservationID) ||
		logs[0].Topics[2] != common.Hash(request.TransactionHash) ||
		logs[0].Topics[3] !=
			common.Hash(request.VariantSequence.AuthorizationSequence) {
		return fmt.Errorf("historical COMPLETE authorization event identity mismatch")
	}
	return nil
}

func (adapter *frostPreSignEthereumAdapter) validateBroadcastAuthorizationAt(
	ctx context.Context,
	request *tbtc.FrostBitcoinBroadcastAuthorizationStatusRequest,
	blockHash common.Hash,
	requirePlan bool,
) (bool, error) {
	reservation, err := adapter.callAtHash(
		ctx,
		common.Address(adapter.profile.RegistryAddress),
		frostPreSignRegistryABI,
		"getReservation",
		blockHash,
		request.ReservationID,
	)
	if err != nil || len(reservation) != 11 {
		return false, fmt.Errorf("cannot read canonical broadcast reservation: [%w]", err)
	}
	variant, err := adapter.callAtHash(
		ctx,
		common.Address(adapter.profile.RegistryAddress),
		frostPreSignRegistryABI,
		"getAuthorizedVariantStatus",
		blockHash,
		[32]byte(request.TransactionHash),
	)
	if err != nil || len(variant) != 6 {
		return false, fmt.Errorf("cannot read canonical broadcast variant: [%w]", err)
	}
	latest, err := adapter.callAtHash(
		ctx,
		common.Address(adapter.profile.RegistryAddress),
		frostPreSignRegistryABI,
		"latestAuthorizedVariant",
		blockHash,
		request.ReservationID,
	)
	if err != nil || len(latest) != 3 {
		return false, fmt.Errorf("cannot read canonical broadcast latest variant: [%w]", err)
	}
	reservationWalletID := *abi.ConvertType(reservation[0], new([32]byte)).(*[32]byte)
	reservationWalletPKH := *abi.ConvertType(reservation[1], new([20]byte)).(*[20]byte)
	membersHash := *abi.ConvertType(reservation[2], new([32]byte)).(*[32]byte)
	snapshot := *abi.ConvertType(reservation[3], new([32]byte)).(*[32]byte)
	resource := *abi.ConvertType(reservation[4], new([32]byte)).(*[32]byte)
	ordered := *abi.ConvertType(reservation[5], new([32]byte)).(*[32]byte)
	data1 := *abi.ConvertType(reservation[6], new([32]byte)).(*[32]byte)
	data2 := *abi.ConvertType(reservation[7], new([32]byte)).(*[32]byte)
	feeLimit := *abi.ConvertType(reservation[8], new(uint64)).(*uint64)
	action := *abi.ConvertType(reservation[9], new(uint8)).(*uint8)
	status := *abi.ConvertType(reservation[10], new(uint8)).(*uint8)
	variantReservation := *abi.ConvertType(variant[0], new([32]byte)).(*[32]byte)
	variantRoot := *abi.ConvertType(variant[1], new([32]byte)).(*[32]byte)
	variantApplyPlan := *abi.ConvertType(variant[2], new([32]byte)).(*[32]byte)
	variantSequence := frostPreSignUint256Word(*abi.ConvertType(variant[3], new(*big.Int)).(**big.Int))
	fraudAuthorized := *abi.ConvertType(variant[4], new(bool)).(*bool)
	signingAllowed := *abi.ConvertType(variant[5], new(bool)).(*bool)
	latestHash := *abi.ConvertType(latest[0], new([32]byte)).(*[32]byte)
	latestSequence := frostPreSignUint256Word(*abi.ConvertType(latest[1], new(*big.Int)).(**big.Int))
	latestAllowed := *abi.ConvertType(latest[2], new(bool)).(*bool)
	if reservationWalletID != request.WalletID ||
		reservationWalletPKH != request.WalletPublicKeyHash ||
		snapshot != request.SnapshotHash || resource != request.ResourceHash ||
		ordered != request.OrderedInputRoot || feeLimit != request.FeeLimitSnapshot ||
		action != uint8(request.Action) || variantReservation != request.ReservationID ||
		variantRoot != request.AuthorizationRoot ||
		variantApplyPlan != request.VariantApplyPlanHash ||
		variantSequence != request.VariantSequence.AuthorizationSequence ||
		!fraudAuthorized {
		return false, nil
	}
	if requirePlan {
		lockedPlan, err := frostPreSignLockedPlanHash(
			resource,
			ordered,
			data1,
			data2,
			feeLimit,
		)
		if err != nil || lockedPlan != request.LockedPlanHash {
			return false, err
		}
		preAuthorization := frostPreSignPreAuthorizationABI{
			Action:           action,
			WalletPubKeyHash: reservationWalletPKH,
			WalletID:         reservationWalletID,
			MembersIDsHash:   membersHash,
			SnapshotHash:     snapshot,
			ResourceHash:     resource,
			OrderedInputRoot: ordered,
			ApplyPlanHash:    variantApplyPlan,
			ApplyPlanData1:   data1,
			ApplyPlanData2:   data2,
			FeeLimitSnapshot: feeLimit,
		}
		digest, err := adapter.callBytes32AtHash(
			ctx,
			common.Address(adapter.profile.RegistryAddress),
			frostPreSignRegistryABI,
			"preAuthorizationDigest",
			blockHash,
			preAuthorization,
			[32]byte(request.TransactionHash),
			variantRoot,
		)
		if err != nil || digest != request.AuthorizationID {
			return false, err
		}
	}
	return status == 1 && signingAllowed && latestAllowed &&
		latestHash == [32]byte(request.TransactionHash) &&
		latestSequence == request.VariantSequence.AuthorizationSequence, nil
}

func frostPreSignLockedPlanHash(
	resource [32]byte,
	ordered [32]byte,
	data1 [32]byte,
	data2 [32]byte,
	feeLimit uint64,
) ([32]byte, error) {
	kinds := []string{"bytes32", "bytes32", "bytes32", "bytes32", "uint64"}
	arguments := make(abi.Arguments, len(kinds))
	for i, kind := range kinds {
		typeValue, err := abi.NewType(kind, "", nil)
		if err != nil {
			return [32]byte{}, err
		}
		arguments[i] = abi.Argument{Type: typeValue}
	}
	encoded, err := arguments.Pack(resource, ordered, data1, data2, feeLimit)
	if err != nil {
		return [32]byte{}, err
	}
	return [32]byte(crypto.Keccak256Hash(encoded)), nil
}
