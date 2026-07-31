package covenantsigner

import "encoding/json"

type TemplateID string

const (
	TemplateQcV1   TemplateID = "qc_v1"
	TemplateSelfV1 TemplateID = "self_v1"
)

type RequestType string

const (
	RequestTypeReconstruct   RequestType = "reconstruct"
	RequestTypePresignSelfV1 RequestType = "presign_self_v1"
)

type RecoveryPathID string

const (
	PathCooperative RecoveryPathID = "COOPERATIVE"
	PathMigration   RecoveryPathID = "MIGRATION"
	PathEarlyExit   RecoveryPathID = "EARLY_EXIT"
	PathLastResort  RecoveryPathID = "LAST_RESORT"
)

type RecoveryStage string

const (
	StageSignerCoordination RecoveryStage = "SIGNER_COORDINATION"
)

type FailureReason string

const (
	ReasonAuthFailed          FailureReason = "AUTH_FAILED"
	ReasonPolicyRejected      FailureReason = "POLICY_REJECTED"
	ReasonInvalidInput        FailureReason = "INVALID_INPUT"
	ReasonProviderUnavailable FailureReason = "PROVIDER_UNAVAILABLE"
	ReasonJobNotFound         FailureReason = "JOB_NOT_FOUND"
	ReasonJobPending          FailureReason = "JOB_PENDING"
	ReasonProviderFailed      FailureReason = "PROVIDER_FAILED"
	ReasonMalformedArtifact   FailureReason = "MALFORMED_ARTIFACT"
)

type ReservationRoute string

const (
	ReservationRouteMigration ReservationRoute = "MIGRATION"
	ReservationRouteRedeem    ReservationRoute = "REDEEM"
	ReservationRouteRenew     ReservationRoute = "RENEW"
)

// CovenantAction discriminates the covenant lifecycle action a submit request
// performs. It selects which destination/plan the request carries and which
// transaction output the signer builds. An empty Action defaults to MIGRATION.
type CovenantAction string

const (
	CovenantActionMigration CovenantAction = "MIGRATION"
	CovenantActionRedeem    CovenantAction = "REDEEM"
	CovenantActionRenew     CovenantAction = "RENEW"
)

// ResolvedAction returns the request's action, defaulting an empty value to
// MIGRATION.
func (r RouteSubmitRequest) ResolvedAction() CovenantAction {
	if r.Action == "" {
		return CovenantActionMigration
	}
	return r.Action
}

type ReservationStatus string

const (
	ReservationStatusReserved         ReservationStatus = "RESERVED"
	ReservationStatusCommittedToEpoch ReservationStatus = "COMMITTED_TO_EPOCH"
	ReservationStatusRevealed         ReservationStatus = "REVEALED"
	ReservationStatusRetired          ReservationStatus = "RETIRED"
	ReservationStatusExpired          ReservationStatus = "EXPIRED"
)

type StepStatus string

const (
	StepStatusPending StepStatus = "PENDING"
	StepStatusReady   StepStatus = "READY"
	StepStatusFailed  StepStatus = "FAILED"
)

type JobState string

const (
	JobStateSubmitted     JobState = "SUBMITTED"
	JobStateValidating    JobState = "VALIDATING"
	JobStateSigning       JobState = "SIGNING"
	JobStatePending       JobState = "PENDING"
	JobStateArtifactReady JobState = "ARTIFACT_READY"
	JobStateHandoffReady  JobState = "HANDOFF_READY"
	JobStateFailed        JobState = "FAILED"
)

type CovenantOutpoint struct {
	TxID       string `json:"txid"`
	Vout       uint32 `json:"vout"`
	ScriptHash string `json:"scriptHash,omitempty"`
}

type ArtifactRecord struct {
	PSBTHash                  string `json:"psbtHash"`
	DestinationCommitmentHash string `json:"destinationCommitmentHash"`
	TransactionHex            string `json:"transactionHex,omitempty"`
	TransactionID             string `json:"transactionId,omitempty"`
}

type MigrationDestinationReservation struct {
	ReservationID             string            `json:"reservationId,omitempty"`
	Reserve                   string            `json:"reserve"`
	Epoch                     uint64            `json:"epoch"`
	Route                     ReservationRoute  `json:"route"`
	Revealer                  string            `json:"revealer"`
	Vault                     string            `json:"vault"`
	Network                   string            `json:"network"`
	Status                    ReservationStatus `json:"status"`
	DepositScript             string            `json:"depositScript"`
	DepositScriptHash         string            `json:"depositScriptHash"`
	MigrationExtraData        string            `json:"migrationExtraData"`
	DestinationCommitmentHash string            `json:"destinationCommitmentHash"`
}

type MigrationTransactionPlan struct {
	PlanVersion          uint32 `json:"planVersion"`
	PlanCommitmentHash   string `json:"planCommitmentHash"`
	InputValueSats       uint64 `json:"inputValueSats"`
	DestinationValueSats uint64 `json:"destinationValueSats"`
	AnchorValueSats      uint64 `json:"anchorValueSats"`
	FeeSats              uint64 `json:"feeSats"`
	InputSequence        uint32 `json:"inputSequence"`
	LockTime             uint32 `json:"lockTime"`
}

type MigrationDestinationPlanQuoteSignature struct {
	SignatureVersion uint32 `json:"signatureVersion"`
	Algorithm        string `json:"algorithm"`
	KeyID            string `json:"keyId"`
	Signature        string `json:"signature"`
}

type MigrationDestinationPlanQuote struct {
	QuoteID                   string                                 `json:"quoteId"`
	QuoteVersion              uint32                                 `json:"quoteVersion"`
	ReservationID             string                                 `json:"reservationId"`
	Reserve                   string                                 `json:"reserve"`
	Epoch                     uint64                                 `json:"epoch"`
	Route                     ReservationRoute                       `json:"route"`
	Revealer                  string                                 `json:"revealer"`
	Vault                     string                                 `json:"vault"`
	Network                   string                                 `json:"network"`
	DestinationCommitmentHash string                                 `json:"destinationCommitmentHash"`
	ActiveOutpointTxID        string                                 `json:"activeOutpointTxid"`
	ActiveOutpointVout        uint32                                 `json:"activeOutpointVout"`
	PlanCommitmentHash        string                                 `json:"planCommitmentHash"`
	MigrationTransactionPlan  *MigrationTransactionPlan              `json:"migrationTransactionPlan"`
	IdempotencyKey            string                                 `json:"idempotencyKey"`
	ExpiresInSeconds          uint64                                 `json:"expiresInSeconds"`
	IssuedAt                  string                                 `json:"issuedAt"`
	ExpiresAt                 string                                 `json:"expiresAt"`
	Signature                 MigrationDestinationPlanQuoteSignature `json:"signature"`
}

type MigrationPlanQuoteTrustRoot struct {
	KeyID        string `json:"keyId" mapstructure:"keyId"`
	PublicKeyPEM string `json:"publicKeyPem" mapstructure:"publicKeyPem"`
}

type DepositorTrustRoot struct {
	Route     TemplateID `json:"route" mapstructure:"route"`
	Reserve   string     `json:"reserve" mapstructure:"reserve"`
	Network   string     `json:"network" mapstructure:"network"`
	PublicKey string     `json:"publicKey" mapstructure:"publicKey"`
	// EthAddress optionally pins the depositor's Ethereum identity (20-byte
	// address). When set, the v2 artifact approval's depositor signature is
	// verified against it via ecrecover-and-compare, enabling wallet-signed
	// (eth_signTypedData_v4) approvals. When empty, verification falls back to
	// the secp256k1 PublicKey above.
	EthAddress string `json:"ethAddress" mapstructure:"ethAddress"`
}

type CustodianTrustRoot struct {
	Route     TemplateID `json:"route" mapstructure:"route"`
	Reserve   string     `json:"reserve" mapstructure:"reserve"`
	Network   string     `json:"network" mapstructure:"network"`
	PublicKey string     `json:"publicKey" mapstructure:"publicKey"`
}

type ArtifactApprovalRole string

const (
	ArtifactApprovalRoleDepositor ArtifactApprovalRole = "D"
	ArtifactApprovalRoleCustodian ArtifactApprovalRole = "C"
)

type ArtifactApprovalPayload struct {
	ApprovalVersion           uint32     `json:"approvalVersion"`
	Route                     TemplateID `json:"route"`
	ScriptTemplateID          TemplateID `json:"scriptTemplateId"`
	DestinationCommitmentHash string     `json:"destinationCommitmentHash"`
	PlanCommitmentHash        string     `json:"planCommitmentHash"`
}

type ArtifactRoleApproval struct {
	Role      ArtifactApprovalRole `json:"role"`
	Signature string               `json:"signature"`
}

type ArtifactApprovalEnvelope struct {
	Payload   ArtifactApprovalPayload `json:"payload"`
	Approvals []ArtifactRoleApproval  `json:"approvals"`
}

type SignerApprovalCertificate struct {
	CertificateVersion uint32   `json:"certificateVersion"`
	SignatureAlgorithm string   `json:"signatureAlgorithm"`
	ApprovalDigest     string   `json:"approvalDigest"`
	WalletPublicKey    string   `json:"walletPublicKey"`
	SignerSetHash      string   `json:"signerSetHash"`
	Signature          string   `json:"signature"`
	ActiveMembers      []uint32 `json:"activeMembers,omitempty"`
	InactiveMembers    []uint32 `json:"inactiveMembers,omitempty"`
	EndBlock           *uint64  `json:"endBlock"`
}

type SigningRequirements struct {
	SignerRequired    bool `json:"signerRequired"`
	CustodianRequired bool `json:"custodianRequired"`
}

type RouteSubmitRequest struct {
	FacadeRequestID           string                            `json:"facadeRequestId"`
	IdempotencyKey            string                            `json:"idempotencyKey"`
	RequestType               RequestType                       `json:"requestType"`
	Route                     TemplateID                        `json:"route"`
	Action                    CovenantAction                    `json:"action,omitempty"`
	Strategy                  string                            `json:"strategy"`
	Reserve                   string                            `json:"reserve"`
	Epoch                     uint64                            `json:"epoch"`
	MaturityHeight            uint64                            `json:"maturityHeight"`
	ActiveOutpoint            CovenantOutpoint                  `json:"activeOutpoint"`
	DestinationCommitmentHash string                            `json:"destinationCommitmentHash"`
	MigrationDestination      *MigrationDestinationReservation  `json:"migrationDestination,omitempty"`
	RedeemDestination         *RedeemDestinationReservation     `json:"redeemDestination,omitempty"`
	RenewDestination          *RenewDestinationReservation      `json:"renewDestination,omitempty"`
	MigrationPlanQuote        *MigrationDestinationPlanQuote    `json:"migrationPlanQuote,omitempty"`
	MigrationTransactionPlan  *MigrationTransactionPlan         `json:"migrationTransactionPlan,omitempty"`
	ArtifactApprovals         *ArtifactApprovalEnvelope         `json:"artifactApprovals,omitempty"`
	SignerApproval            *SignerApprovalCertificate        `json:"signerApproval,omitempty"`
	ArtifactSignatures        []string                          `json:"artifactSignatures"`
	Artifacts                 map[RecoveryPathID]ArtifactRecord `json:"artifacts"`
	ScriptTemplate            json.RawMessage                   `json:"scriptTemplate"`
	Signing                   SigningRequirements               `json:"signing"`
}

// RedeemDestinationReservation is the cooperative-REDEEM destination: a payout
// output the signer pays directly. Its commitment binds the covenant identity
// (reserve/epoch/route/revealer/vault/network) to the payout (output scriptPubKey
// + value), so the depositor's artifact approval authorizes exactly that payout.
type RedeemDestinationReservation struct {
	ReservationID             string            `json:"reservationId,omitempty"`
	Reserve                   string            `json:"reserve"`
	Epoch                     uint64            `json:"epoch"`
	Route                     ReservationRoute  `json:"route"`
	Revealer                  string            `json:"revealer"`
	Vault                     string            `json:"vault"`
	Network                   string            `json:"network"`
	Status                    ReservationStatus `json:"status"`
	OutputScript              string            `json:"outputScript"`
	OutputScriptHash          string            `json:"outputScriptHash"`
	OutputValueSats           uint64            `json:"outputValueSats"`
	DestinationCommitmentHash string            `json:"destinationCommitmentHash"`
}

// RenewDestinationReservation is the cooperative-RENEW destination: the value is
// re-locked into the next-epoch covenant output. Its commitment binds the
// covenant identity to the next covenant scriptPubKey, its maturity height, and
// the re-locked value.
type RenewDestinationReservation struct {
	ReservationID             string            `json:"reservationId,omitempty"`
	Reserve                   string            `json:"reserve"`
	Epoch                     uint64            `json:"epoch"`
	Route                     ReservationRoute  `json:"route"`
	Revealer                  string            `json:"revealer"`
	Vault                     string            `json:"vault"`
	Network                   string            `json:"network"`
	Status                    ReservationStatus `json:"status"`
	NextCovenantScript        string            `json:"nextCovenantScript"`
	NextCovenantScriptHash    string            `json:"nextCovenantScriptHash"`
	NextMaturityHeight        uint64            `json:"nextMaturityHeight"`
	OutputValueSats           uint64            `json:"outputValueSats"`
	DestinationCommitmentHash string            `json:"destinationCommitmentHash"`
}

type SignerSubmitInput struct {
	RouteRequestID string             `json:"routeRequestId"`
	Request        RouteSubmitRequest `json:"request"`
	Stage          RecoveryStage      `json:"stage"`
}

type SignerPollInput struct {
	RouteRequestID string             `json:"routeRequestId"`
	RequestID      string             `json:"requestId"`
	Request        RouteSubmitRequest `json:"request"`
	Stage          RecoveryStage      `json:"stage"`
}

type StepResult struct {
	Status         StepStatus     `json:"status"`
	RequestID      string         `json:"requestId,omitempty"`
	Detail         string         `json:"detail,omitempty"`
	Reason         FailureReason  `json:"reason,omitempty"`
	PSBTHash       string         `json:"psbtHash,omitempty"`
	TransactionHex string         `json:"transactionHex,omitempty"`
	Handoff        map[string]any `json:"handoff,omitempty"`
}

type Job struct {
	RequestID       string     `json:"requestId"`
	RouteRequestID  string     `json:"routeRequestId"`
	Route           TemplateID `json:"route"`
	IdempotencyKey  string     `json:"idempotencyKey"`
	FacadeRequestID string     `json:"facadeRequestId"`
	RequestDigest   string     `json:"requestDigest"`
	// DepositorEthAddress is the depositor's ETH identity resolved from
	// depositorTrustRoots at submit time, when one was configured for this
	// request's trust-root scope; empty means the depositor's artifact
	// approval was (and continues to be) verified against the secp256k1
	// script-template key instead. Poll re-validation is policy-independent
	// (it must not depend on depositorTrustRoots possibly having changed
	// since submit), so it reuses this pinned snapshot rather than
	// re-resolving trust roots on every poll.
	DepositorEthAddress string             `json:"depositorEthAddress,omitempty"`
	State               JobState           `json:"state"`
	Detail              string             `json:"detail,omitempty"`
	Reason              FailureReason      `json:"reason,omitempty"`
	CreatedAt           string             `json:"createdAt"`
	UpdatedAt           string             `json:"updatedAt"`
	CompletedAt         string             `json:"completedAt,omitempty"`
	FailedAt            string             `json:"failedAt,omitempty"`
	Request             RouteSubmitRequest `json:"request"`
	PSBTHash            string             `json:"psbtHash,omitempty"`
	TransactionHex      string             `json:"transactionHex,omitempty"`
	Handoff             map[string]any     `json:"handoff,omitempty"`
}

type SelfV1Template struct {
	Template           TemplateID `json:"template"`
	DepositorPublicKey string     `json:"depositorPublicKey"`
	SignerPublicKey    string     `json:"signerPublicKey"`
	Delta2             uint64     `json:"delta2"`
}

type QcV1Template struct {
	Template           TemplateID `json:"template"`
	DepositorPublicKey string     `json:"depositorPublicKey"`
	CustodianPublicKey string     `json:"custodianPublicKey"`
	SignerPublicKey    string     `json:"signerPublicKey"`
	Beta               uint64     `json:"beta"`
	Delta2             uint64     `json:"delta2"`
}
