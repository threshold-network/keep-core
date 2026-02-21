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
	MemberIndex         group.MemberIndex
	GroupSize           int
	DishonestThreshold  int
	Channel             net.BroadcastChannel
	MembershipValidator *group.MembershipValidator
	SignerMaterial      *NativeSignerMaterial
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
) (*Result, error) {
	if request == nil {
		return nil, fmt.Errorf("request is nil")
	}

	if request.Message == nil {
		return nil, fmt.Errorf("request message is nil")
	}

	signerMaterial, err := request.NativeSignerMaterial()
	if err != nil {
		return nil, err
	}

	signature, err := nefea.primitive.Sign(
		ctx,
		logger,
		&NativeExecutionFFISigningRequest{
			Message:             request.Message,
			SessionID:           request.SessionID,
			MemberIndex:         request.MemberIndex,
			GroupSize:           request.GroupSize,
			DishonestThreshold:  request.DishonestThreshold,
			Channel:             request.Channel,
			MembershipValidator: request.MembershipValidator,
			SignerMaterial:      signerMaterial,
			Attempt:             cloneAttempt(request.Attempt),
		},
	)
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
