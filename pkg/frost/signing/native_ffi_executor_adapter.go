package signing

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ipfs/go-log/v2"
	"github.com/keep-network/keep-core/pkg/frost"
	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// NativeExecutionFFISigningRequest is the canonical request passed to a native
// FFI signing primitive.
type NativeExecutionFFISigningRequest struct {
	Message             *big.Int
	SessionID           string
	RoastSessionID      string
	MemberIndex         group.MemberIndex
	GroupSize           int
	DishonestThreshold  int
	Channel             net.BroadcastChannel
	MembershipValidator *group.MembershipValidator
	SignerMaterial      *NativeSignerMaterial
	TaprootMerkleRoot   *[32]byte
	Attempt             *Attempt
}

// NativeExecutionFFISigningPrimitive is a minimal cryptographic primitive
// interface used by the reusable native FFI executor adapter.
type NativeExecutionFFISigningPrimitive interface {
	Sign(
		ctx context.Context,
		logger log.StandardLogger,
		request *NativeExecutionFFISigningRequest,
	) (*frost.Signature, error)
	RegisterUnmarshallers(channel net.BroadcastChannel)
}

type nativeExecutionFFIExecutorAdapter struct {
	primitive NativeExecutionFFISigningPrimitive
}

// NewNativeExecutionFFIExecutorAdapter wraps a native FFI signing primitive as
// a NativeExecutionFFIExecutor.
func NewNativeExecutionFFIExecutorAdapter(
	primitive NativeExecutionFFISigningPrimitive,
) (NativeExecutionFFIExecutor, error) {
	if primitive == nil {
		return nil, fmt.Errorf("native execution FFI signing primitive is nil")
	}

	return &nativeExecutionFFIExecutorAdapter{
		primitive: primitive,
	}, nil
}

// RegisterNativeExecutionFFISigningPrimitive registers a native FFI signing
// primitive by adapting it to NativeExecutionFFIExecutor.
func RegisterNativeExecutionFFISigningPrimitive(
	primitive NativeExecutionFFISigningPrimitive,
) error {
	executor, err := NewNativeExecutionFFIExecutorAdapter(primitive)
	if err != nil {
		return err
	}

	return RegisterNativeExecutionFFIExecutor(executor)
}

func (nefea *nativeExecutionFFIExecutorAdapter) Execute(
	ctx context.Context,
	logger log.StandardLogger,
	request *Request,
) (result *Result, err error) {
	// Recover any panic originating along the cgo/FFI signing path (e.g. a
	// malformed or oversized response from the native signer) and surface it as
	// a failed attempt instead of crashing the whole signing process. The outer
	// tBTC signingRetryLoop then handles this attempt cleanly.
	defer func() {
		if r := recover(); r != nil {
			result = nil
			err = fmt.Errorf(
				"native FFI signing panicked at the cgo boundary: %v", r,
			)
		}
	}()

	if request == nil {
		return nil, fmt.Errorf("request is nil")
	}

	if request.Message == nil {
		return nil, fmt.Errorf("request message is nil")
	}

	signerMaterial, err := request.NativeSignerMaterial()
	if err != nil {
		return nil, fmt.Errorf("%w: [%v]", ErrNativeCryptographyUnavailable, err)
	}

	ffiRequest := &NativeExecutionFFISigningRequest{
		Message:             request.Message,
		SessionID:           request.SessionID,
		RoastSessionID:      request.RoastSessionID,
		MemberIndex:         request.MemberIndex,
		GroupSize:           request.GroupSize,
		DishonestThreshold:  request.DishonestThreshold,
		Channel:             request.Channel,
		MembershipValidator: request.MembershipValidator,
		SignerMaterial:      signerMaterial,
		TaprootMerkleRoot:   cloneTaprootMerkleRoot(request.TaprootMerkleRoot),
		Attempt:             cloneAttempt(request.Attempt),
	}

	// RFC-21 Phase 6.3: ROAST orchestration entry. The helper
	// returns (cleanup, error):
	//   - cleanup non-nil, error nil -> orchestration active;
	//     defer cleanup so success and failure return paths converge.
	//   - cleanup nil, error nil     -> static-configuration fallback
	//     (env var unset, no coordinator registered, or material
	//     format not extractable). Proceed without orchestration; the
	//     receive loops use NoOp recorder semantics (Phase 5 behaviour).
	//   - cleanup nil, error non-nil -> RUNTIME orchestration failure.
	//     HARD FAIL to prevent group fracture across honest signers.
	// In the default build (no frost_native tag) the helper is a
	// permanent no-op returning (nil, nil).
	// RFC-21 Phase 6.3 + 7.3: ROAST orchestration + gated interactive signing.
	// attemptRoastRetryOrchestrationFromRequest sets up per-session
	// orchestration and, when the operator audit gate
	// (KEEP_CORE_FROST_INTERACTIVE_SIGNING_ENABLED) is on and an engine is
	// registered, drives ONE interactive attempt with the handle minted for THIS
	// Execute (the outer tBTC signingRetryLoop owns retries; this is one
	// attempt). It returns (signature, cleanup, error):
	//   - signature non-nil -> interactive signing produced it; return it.
	//   - signature nil, cleanup non-nil -> orchestration active but interactive
	//     not enabled; defer cleanup, fall through to the coarse primitive.
	//   - signature nil, cleanup nil -> static fallback; coarse primitive.
	//   - error non-nil -> RUNTIME/committed failure; HARD FAIL (cleanup, when
	//     non-nil, is deferred first so a failed interactive attempt still
	//     stashes its transition bundle).
	// In the default build (no frost_native) the helper is a permanent no-op
	// returning (nil, nil, nil).
	interactiveSignature, orchCleanup, orchErr :=
		attemptRoastRetryOrchestrationFromRequest(ctx, ffiRequest, logger)
	if orchCleanup != nil {
		defer orchCleanup()
	}
	if orchErr != nil {
		return nil, orchErr
	}
	if interactiveSignature != nil {
		return &Result{
			Signature: interactiveSignature,
			Attempt:   cloneAttempt(request.Attempt),
		}, nil
	}

	if InteractiveSigningOnlyEnabled() {
		// Interactive-only mode (coarse-path retirement): the orchestration produced
		// no interactive signature - the audit gate is off, no engine is registered,
		// OR any static fallback fired (readiness off, no coordinator, unsupported
		// material). Refuse the inner coarse primitive here; the OUTER bridge/adapter
		// legacy fallback is closed separately via nativeExecutionFallbackAllowed().
		// Mark the refusal TERMINAL so the tBTC signingRetryLoop aborts immediately
		// instead of retrying a deterministic configuration failure until timeout.
		recordCoarseFallbackRefused()
		return nil, fmt.Errorf(
			"%w: interactive-only signing mode (%s) is set but interactive signing did "+
				"not run (%s off, no engine, or static fallback); refusing the coarse fallback",
			ErrTerminalSigningFailure, InteractiveSigningOnlyEnvVar, InteractiveSigningOptInEnvVar,
		)
	}

	signature, err := nefea.primitive.Sign(ctx, logger, ffiRequest)
	if err != nil {
		return nil, err
	}

	if signature == nil {
		return nil, fmt.Errorf("native FFI signing primitive returned nil signature")
	}

	return &Result{
		Signature: signature,
		Attempt:   cloneAttempt(request.Attempt),
	}, nil
}

func (nefea *nativeExecutionFFIExecutorAdapter) RegisterUnmarshallers(
	channel net.BroadcastChannel,
) {
	nefea.primitive.RegisterUnmarshallers(channel)
}

func cloneTaprootMerkleRoot(taprootMerkleRoot *[32]byte) *[32]byte {
	if taprootMerkleRoot == nil {
		return nil
	}

	result := new([32]byte)
	copy(result[:], taprootMerkleRoot[:])

	return result
}
