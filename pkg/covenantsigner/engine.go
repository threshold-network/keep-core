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
// CurrentBlockHeight is optional. When implemented, it is called during Submit
// and Poll to determine whether a signer approval certificate has expired.
type Engine interface {
	OnSubmit(ctx context.Context, job *Job) (*Transition, error)
	OnPoll(ctx context.Context, job *Job) (*Transition, error)
}

// CurrentBlockHeightProvider returns the current height of the host/anchoring
// chain in the same block-number domain as SignerApprovalCertificate.EndBlock.
// For tBTC this is the Ethereum chain, not the Bitcoin chain. The service uses
// it to check signer approval certificate expiration during Submit and Poll.
// Engines that accept signer-approval certificates must provide it: expiration
// must not fail open when it is absent, so the service rejects certificate-
// bearing requests that cannot be checked against a current height.
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
