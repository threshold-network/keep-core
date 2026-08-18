package covenantsigner

import (
	"context"
	"errors"
)

var errJobNotFound = errors.New("covenant signer job not found")

type Transition struct {
	State          JobState
	Detail         string
	Reason         FailureReason
	PSBTHash       string
	TransactionHex string
	Handoff        map[string]any
}

// Engine drives the signing lifecycle for a job. Both methods receive the
// job's current state and return an optional Transition describing the next
// state. Returning (nil, nil) is valid and has method-specific semantics:
//
//   - OnSubmit: nil Transition causes the service to apply a default Pending
//     transition, moving the job to JobStatePending with a generic detail
//     message. The engine should return a non-nil Transition when it has
//     already initiated work synchronously or wants to override the detail.
//
//   - OnPoll: nil Transition means no state change; the job stays at its
//     current state. The engine should return a non-nil Transition only when
//     it has observable progress to report (or when the job has failed).
//
// Returning errJobNotFound signals that the engine can no longer locate the
// underlying signing job. The service treats this as a terminal failure and
// transitions the job to JobStateFailed.
//
// An Engine may also implement SignerApprovalVerifier,
// CurrentBlockHeightProvider, and SignerApprovalCertificateIssuer; see their
// doc comments for the constraints between them.
type Engine interface {
	OnSubmit(ctx context.Context, job *Job) (*Transition, error)
	OnPoll(ctx context.Context, job *Job) (*Transition, error)
}

// CurrentBlockHeightProvider is implemented by Engines that can provide the
// current block height of the host chain (e.g. Ethereum) that signer approval
// certificate EndBlock values are denominated in -- never the Bitcoin chain.
// The service uses it to check signer approval certificate expiration during
// Submit and Poll.
//
// Any engine that implements SignerApprovalVerifier must also implement this
// interface: Initialize rejects engines that verify signer approvals but
// cannot report a current block height, because such an engine could never
// determine whether a certificate has expired. There is no fail-open "never
// expires" behavior for engines that omit this interface -- Submit and Poll
// reject certificate-bearing requests outright when no provider is
// configured.
type CurrentBlockHeightProvider interface {
	CurrentBlockHeight(ctx context.Context) (uint64, error)
}

type SignerApprovalVerifier interface {
	VerifySignerApproval(request RouteSubmitRequest) error
}

type SignerApprovalVerifierFunc func(request RouteSubmitRequest) error

func (savf SignerApprovalVerifierFunc) VerifySignerApproval(
	request RouteSubmitRequest,
) error {
	return savf(request)
}

// SignerApprovalCertificateIssuer is implemented by Engines that can produce a
// v2 SignerApprovalCertificate for a wallet they control by running a threshold
// signing round over the certificate digest. The admin HTTP issuer endpoint
// type-asserts the engine to this interface; engines that omit it return 501.
//
// approvalDigest must be the 32-byte artifact-approval payload digest that the
// eventual Submit request will carry — VerifySignerApproval rejects certificates
// whose ApprovalDigest does not match request.artifactApprovals.payload.
// endBlock is a host-chain (e.g. Ethereum) height after which the certificate
// must be rejected as expired.
type SignerApprovalCertificateIssuer interface {
	IssueSignerApprovalCertificate(
		ctx context.Context,
		walletPublicKeyHash [20]byte,
		approvalDigest []byte,
		endBlock uint64,
	) (*SignerApprovalCertificate, error)
}

// ErrSignerApprovalCertificateIssuerUnsupported is returned when the configured
// engine does not implement SignerApprovalCertificateIssuer.
var ErrSignerApprovalCertificateIssuerUnsupported = errors.New(
	"covenant signer engine does not support signer approval certificate issuance",
)

// ErrSignerApprovalCertificateIssuerBusy is returned when the engine's signing
// executor for the requested wallet is already running another signing
// operation (e.g. a concurrent Submit or a concurrent issuance). Callers
// should retry after a short delay; the request was rejected before any
// threshold signing began, so retrying is always safe.
var ErrSignerApprovalCertificateIssuerBusy = errors.New(
	"covenant signer engine is busy signing for this wallet",
)

type passiveEngine struct{}

func NewPassiveEngine() Engine {
	return &passiveEngine{}
}

func (pe *passiveEngine) OnSubmit(context.Context, *Job) (*Transition, error) {
	return &Transition{
		State:  JobStatePending,
		Detail: "accepted for covenant signing",
	}, nil
}

func (pe *passiveEngine) OnPoll(context.Context, *Job) (*Transition, error) {
	return nil, nil
}

func (pe *passiveEngine) CurrentBlockHeight(context.Context) (uint64, error) {
	return 0, errors.New("passiveEngine does not provide current block height")
}
