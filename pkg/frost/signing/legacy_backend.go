package signing

import (
	"context"
	"fmt"

	"github.com/ipfs/go-log/v2"
	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/protocol/group"
	legacySigning "github.com/keep-network/keep-core/pkg/tecdsa/signing"
)

const legacyExecutionBackendName = "legacy-tecdsa-bridge"

type legacyExecutionBackend struct{}

func newLegacyExecutionBackend() *legacyExecutionBackend {
	return &legacyExecutionBackend{}
}

func (leb *legacyExecutionBackend) Name() string {
	return legacyExecutionBackendName
}

func (leb *legacyExecutionBackend) Execute(
	ctx context.Context,
	logger log.StandardLogger,
	request *Request,
) (*Result, error) {
	if request == nil {
		return nil, fmt.Errorf("request is nil")
	}

	if InteractiveSigningOnlyEnabled() {
		// Interactive-only mode (coarse-path retirement): the legacy backend is the
		// tECDSA/coarse signer the flag retires. Refuse at the action itself so the
		// safety switch holds even under the DEFAULT backend selection, where the
		// native bridge/adapter guards never run. Terminal so the retry loop aborts.
		recordCoarseFallbackRefused()
		return nil, fmt.Errorf(
			"%w: interactive-only signing mode (%s) is set but the legacy (coarse "+
				"tECDSA) backend is selected; the coarse path is retired",
			ErrTerminalSigningFailure, InteractiveSigningOnlyEnvVar,
		)
	}

	if request.Attempt != nil {
		logger.Infof(
			"[member:%v] executing FROST signing attempt [%v] "+
				"with coordinator [%v] (included: [%v], excluded: [%v])",
			request.MemberIndex,
			request.Attempt.Number,
			request.Attempt.CoordinatorMemberIndex,
			request.Attempt.IncludedMembersIndexes,
			request.Attempt.ExcludedMembersIndexes,
		)
	}

	excludedMembersIndexes := []group.MemberIndex{}
	if request.Attempt != nil {
		excludedMembersIndexes = request.Attempt.ExcludedMembersIndexes
	}

	privateKeyShare, err := request.LegacyPrivateKeyShare()
	if err != nil {
		return nil, err
	}

	legacyResult, err := legacySigning.Execute(
		ctx,
		logger,
		request.Message,
		request.SessionID,
		request.MemberIndex,
		privateKeyShare,
		request.GroupSize,
		request.DishonestThreshold,
		excludedMembersIndexes,
		request.Channel,
		request.MembershipValidator,
	)
	if err != nil {
		return nil, err
	}

	signature, err := FromTECDSASignature(legacyResult.Signature)
	if err != nil {
		return nil, err
	}

	return &Result{
		Signature: signature,
		Attempt:   cloneAttempt(request.Attempt),
	}, nil
}

func (leb *legacyExecutionBackend) RegisterUnmarshallers(
	channel net.BroadcastChannel,
) {
	legacySigning.RegisterUnmarshallers(channel)
}
