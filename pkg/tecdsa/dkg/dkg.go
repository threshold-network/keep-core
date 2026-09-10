package dkg

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ipfs/go-log/v2"

	"github.com/keep-network/keep-core/pkg/generator"
	"github.com/keep-network/keep-core/pkg/keepcommon/persistence"
	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/protocol/group"
	"github.com/keep-network/keep-core/pkg/protocol/state"
)

// Executor represents an ECDSA distributed key generation process executor.
type Executor struct {
	tssPreParamsPool         *tssPreParamsPool
	keyGenerationConcurrency int
}

// NewExecutor creates a new Executor instance.
func NewExecutor(
	logger log.StandardLogger,
	scheduler *generator.Scheduler,
	persistence persistence.BasicHandle,
	preParamsPoolSize int,
	preParamsGenerationTimeout time.Duration,
	preParamsGenerationDelay time.Duration,
	preParamsGenerationConcurrency int,
	keyGenerationConcurrency int,
) *Executor {
	logger.Infof(
		"ECDSA key generation concurrency level is [%d]",
		keyGenerationConcurrency,
	)
	return &Executor{
		tssPreParamsPool: newTssPreParamsPool(
			logger,
			scheduler,
			persistence,
			preParamsPoolSize,
			preParamsGenerationTimeout,
			preParamsGenerationDelay,
			preParamsGenerationConcurrency,
		),
		keyGenerationConcurrency: keyGenerationConcurrency,
	}
}

// Execute runs the tECDSA distributed key generation protocol, given a
// broadcast channel to mediate with, a block counter used for time tracking,
// a member index to use in the group, dishonest threshold, and block height
// when DKG protocol should start.
//
// This function also supports DKG execution with a subset of the selected
// group by passing a non-empty excludedMembers slice holding the members that
// should be excluded.
func (e *Executor) Execute(
	ctx context.Context,
	logger log.StandardLogger,
	seed *big.Int,
	sessionID string,
	memberIndex group.MemberIndex,
	groupSize int,
	dishonestThreshold int,
	excludedMembersIndexes []group.MemberIndex,
	channel net.BroadcastChannel,
	membershipValidator *group.MembershipValidator,
) (*Result, error) {
	logger.Debugf("[member:%v] initializing member", memberIndex)

	member := newMember(
		logger,
		seed,
		memberIndex,
		groupSize,
		dishonestThreshold,
		membershipValidator,
		sessionID,
		e.tssPreParamsPool.GetNow,
		e.keyGenerationConcurrency,
	)

	// Mark excluded members as disqualified in order to not exchange messages
	// with them and have them recorded as misbehaving in the final result.
	for _, excludedMemberIndex := range excludedMembersIndexes {
		if excludedMemberIndex != member.id {
			member.group.MarkMemberAsDisqualified(excludedMemberIndex)
		}
	}

	initialState := &ephemeralKeyPairGenerationState{
		BaseAsyncState: state.NewBaseAsyncState(),
		channel:        channel,
		member:         member.initializeEphemeralKeysGeneration(),
	}

	stateMachine := state.NewAsyncMachine(logger, ctx, channel, initialState)

	lastState, err := stateMachine.Execute()
	if err != nil {
		return nil, err
	}

	finalizationState, ok := lastState.(*finalizationState)
	if !ok {
		return nil, fmt.Errorf("execution ended on state: %T", lastState)
	}

	return finalizationState.result(), nil
}

// PreParamsCount returns the current count of the DKG pre-parameters.
func (e *Executor) PreParamsCount() int {
	return e.tssPreParamsPool.ParametersCount()
}

// SignedResult represents information pertaining to the process of signing
// a DKG result: the public key used during signing, the resulting signature and
// the hash of the DKG result that was used during signing.
type SignedResult struct {
	PublicKey  []byte
	Signature  []byte
	ResultHash ResultSignatureHash
}

// ResultSigner is the interface that provides ability to sign the DKG result
// and verify the results received from other group members.
type ResultSigner interface {
	// SignResult signs the provided DKG result. It returns the information
	// pertaining to the signing process: public key, signature, result hash.
	SignResult(result *Result) (*SignedResult, error)
	// VerifySignature verifies if the signature was generated from the provided
	// DKG result has using the provided public key.
	VerifySignature(signedResult *SignedResult) (bool, error)
}

// ResultSubmitter is the interface that provides ability to submit the DKG
// result to the chain.
type ResultSubmitter interface {
	// SubmitResult submits the DKG result along with the supporting signatures.
	SubmitResult(
		ctx context.Context,
		memberIndex group.MemberIndex,
		result *Result,
		signatures map[group.MemberIndex][]byte,
	) error
}

// Publish signs the DKG result for the given group member, collects signatures
// from other members and verifies them, and submits the DKG result.
func Publish(
	ctx context.Context,
	logger log.StandardLogger,
	sessionID string,
	memberIndex group.MemberIndex,
	channel net.BroadcastChannel,
	membershipValidator *group.MembershipValidator,
	resultSigner ResultSigner,
	resultSubmitter ResultSubmitter,
	result *Result,
) error {
	initialState := &resultSigningState{
		BaseAsyncState:  state.NewBaseAsyncState(),
		channel:         channel,
		resultSigner:    resultSigner,
		resultSubmitter: resultSubmitter,
		member: newSigningMember(
			logger,
			memberIndex,
			result.Group,
			membershipValidator,
			sessionID,
		),
		result: result,
	}

	stateMachine := state.NewAsyncMachine(logger, ctx, channel, initialState)

	lastState, err := stateMachine.Execute()
	if err != nil {
		return err
	}

	_, ok := lastState.(*resultSubmissionState)
	if !ok {
		return fmt.Errorf("execution ended on state %T", lastState)
	}

	return nil
}

// RegisterUnmarshallers initializes the given broadcast channel to be able to
// perform DKG protocol interactions by registering all the required protocol
// message unmarshallers.
func RegisterUnmarshallers(channel net.BroadcastChannel) {
	channel.SetUnmarshaler(func() net.TaggedUnmarshaler {
		return &ephemeralPublicKeyMessage{}
	})
	channel.SetUnmarshaler(func() net.TaggedUnmarshaler {
		return &tssRoundOneMessage{}
	})
	channel.SetUnmarshaler(func() net.TaggedUnmarshaler {
		return &tssRoundTwoMessage{}
	})
	channel.SetUnmarshaler(func() net.TaggedUnmarshaler {
		return &tssRoundThreeMessage{}
	})
	channel.SetUnmarshaler(func() net.TaggedUnmarshaler {
		return &tssFinalizationMessage{}
	})
	channel.SetUnmarshaler(func() net.TaggedUnmarshaler {
		return &resultSignatureMessage{}
	})
}
