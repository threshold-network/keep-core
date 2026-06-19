package tbtc

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/keep-network/keep-core/pkg/clientinfo"
	"github.com/keep-network/keep-core/pkg/frost"
	"github.com/keep-network/keep-core/pkg/frost/roast"
	"github.com/keep-network/keep-core/pkg/frost/signing"
	"github.com/keep-network/keep-core/pkg/generator"
	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/protocol/announcer"
	"github.com/keep-network/keep-core/pkg/protocol/group"
	"go.uber.org/zap"
	"golang.org/x/sync/semaphore"
)

const (
	// signingBatchInterludeBlocks determines the block duration of the
	// interlude preserved between subsequent signings in a signing batch.
	// If the signing of the previous message completed at block X, the signing
	// of the next message starts at `X + signingBatchInterludeBlocks`.
	// This is the additional time signers have to realize that the signing is
	// done by receiving the signingDoneMessage. Note that the end block of the
	// previous signing used to establish the start block of the next signing
	// comes from signingDoneMessage received and there is no guarantee all
	// signing group members received signingDoneMessage before the highest
	// endBlock is reached on the chain. The interlude is an additional time for
	// the broadcast channel to spread information about signing successfully
	// completed by the slowest signing group member (the one who sends the
	// signingDoneMessage as the last one).
	signingBatchInterludeBlocks = 2
)

// errSigningExecutorBusy is an error returned when the signing executor
// cannot execute the requested signature due to an ongoing signing.
var errSigningExecutorBusy = fmt.Errorf("signing executor is busy")

func signingSessionID(
	message *big.Int,
	taprootMerkleRoot *[32]byte,
	startBlock uint64,
	attemptNumber uint,
) string {
	if taprootMerkleRoot == nil {
		return fmt.Sprintf("%v-%v", message.Text(16), attemptNumber)
	}

	var startBlockBytes [8]byte
	binary.BigEndian.PutUint64(startBlockBytes[:], startBlock)

	sessionDigest := sha256.New()
	sessionDigest.Write([]byte(message.Text(16)))
	sessionDigest.Write([]byte{0})
	sessionDigest.Write(taprootMerkleRoot[:])
	sessionDigest.Write([]byte{0})
	sessionDigest.Write(startBlockBytes[:])

	return fmt.Sprintf("tr-%x-%v", sessionDigest.Sum(nil), attemptNumber)
}

// roastSessionID is the STABLE per-signing ROAST session id: the signing
// session key WITHOUT the attempt number, so it is constant across every
// attempt of one message-signing. RFC-21 Phase 7.3 ROAST orchestration, the
// transition-record registry, and the participant selector key off it so
// attempt N+1's selector can find attempt N's transition record; the
// coarse/legacy path keeps using the attempt-specific signingSessionID for its
// replay isolation. The "roast-" prefix keeps this id disjoint from any
// signingSessionID value.
func roastSessionID(
	message *big.Int,
	taprootMerkleRoot *[32]byte,
	startBlock uint64,
) string {
	if taprootMerkleRoot == nil {
		// startBlock is included so two independent signings of the SAME message
		// at different start blocks get distinct stable ids (and so distinct
		// transition-record / interactive-engine namespaces); without it a later
		// signing could collide with retained ROAST state from an earlier one.
		// The taproot branch below already binds startBlock.
		return fmt.Sprintf("roast-%v-%v", message.Text(16), startBlock)
	}

	var startBlockBytes [8]byte
	binary.BigEndian.PutUint64(startBlockBytes[:], startBlock)

	sessionDigest := sha256.New()
	sessionDigest.Write([]byte(message.Text(16)))
	sessionDigest.Write([]byte{0})
	sessionDigest.Write(taprootMerkleRoot[:])
	sessionDigest.Write([]byte{0})
	sessionDigest.Write(startBlockBytes[:])

	return fmt.Sprintf("roast-tr-%x", sessionDigest.Sum(nil))
}

// signingExecutor is a component responsible for executing signing related to
// a specific wallet whose part is controlled by this node.
type signingExecutor struct {
	lock *semaphore.Weighted

	signers             []*signer
	broadcastChannel    net.BroadcastChannel
	membershipValidator *group.MembershipValidator
	groupParameters     *GroupParameters
	protocolLatch       *generator.ProtocolLatch

	// getCurrentBlockFn is a function used to get the current block.
	getCurrentBlockFn getCurrentBlockFn
	// waitForBlockFn is a function used to wait for the given block.
	waitForBlockFn waitForBlockFn

	// signingAttemptsLimit determines the maximum attempts count that will
	// be made by a single signer for the given message. Once the attempts
	// limit is hit the signer gives up.
	signingAttemptsLimit uint

	// metricsRecorder is optional and used for recording performance metrics
	metricsRecorder interface {
		IncrementCounter(name string, value float64)
		SetGauge(name string, value float64)
		RecordDuration(name string, duration time.Duration)
	}

	// livenessTracker is optional and feeds the RFC-21 Annex B implied-f
	// signing-attempt liveness gauges. It is shared across all wallets'
	// executors of this node (acquired from the metrics recorder).
	livenessTracker *clientinfo.SigningAttemptLivenessTracker
}

// signingLivenessTrackerProvider is implemented by metrics recorders that
// own the process-wide signing-attempt liveness tracker.
type signingLivenessTrackerProvider interface {
	SigningAttemptLivenessTracker() *clientinfo.SigningAttemptLivenessTracker
}

var _ schnorrWalletSigningExecutor = (*signingExecutor)(nil)

func newSigningExecutor(
	signers []*signer,
	broadcastChannel net.BroadcastChannel,
	membershipValidator *group.MembershipValidator,
	groupParameters *GroupParameters,
	protocolLatch *generator.ProtocolLatch,
	getCurrentBlockFn getCurrentBlockFn,
	waitForBlockFn waitForBlockFn,
	signingAttemptsLimit uint,
) *signingExecutor {
	return &signingExecutor{
		lock:                 semaphore.NewWeighted(1),
		signers:              signers,
		broadcastChannel:     broadcastChannel,
		membershipValidator:  membershipValidator,
		groupParameters:      groupParameters,
		protocolLatch:        protocolLatch,
		getCurrentBlockFn:    getCurrentBlockFn,
		waitForBlockFn:       waitForBlockFn,
		signingAttemptsLimit: signingAttemptsLimit,
	}
}

func (se *signingExecutor) usesSchnorrSignatures() bool {
	for _, signer := range se.signers {
		if signingMaterialUsesSchnorrSignatures(signer.signingMaterial()) {
			return true
		}
	}

	return false
}

// signBatch performs the signing process for each message from the given
// messages batch, one after another. If at least one message cannot be signed,
// this function returns an error. If all messages were signed successfully,
// a slice of signatures is returned. Order of the returned signatures matches
// the order of the messages in the batch, i.e. the first signature corresponds
// to the first message, and so on.
func (se *signingExecutor) signBatch(
	ctx context.Context,
	messages []*big.Int,
	startBlock uint64,
) ([]*frost.Signature, error) {
	return se.signBatchWithTaprootMerkleRoots(ctx, messages, nil, startBlock)
}

func (se *signingExecutor) signBatchWithTaprootMerkleRoots(
	ctx context.Context,
	messages []*big.Int,
	taprootMerkleRoots []*[32]byte,
	startBlock uint64,
) ([]*frost.Signature, error) {
	if taprootMerkleRoots != nil && len(taprootMerkleRoots) != len(messages) {
		return nil, fmt.Errorf(
			"taproot merkle roots count [%v] does not match messages count [%v]",
			len(taprootMerkleRoots),
			len(messages),
		)
	}

	wallet := se.wallet()

	walletPublicKeyBytes, err := marshalPublicKey(wallet.publicKey)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal wallet public key: [%v]", err)
	}

	messagesDigests := make([]string, len(messages))
	for i, message := range messages {
		bytes := message.Bytes()

		// Real-world messages are usually 32-byte however, test ones can be
		// much shorter. The distinction of displaying whole messages up
		// to 8 bytes is arbitrary though can be justified as digesting shorter
		// values is rather an overkill. Having full messages while inspecting
		// test logs may help while debugging.
		var messageDigest string
		if len(bytes) > 8 {
			messageDigest = fmt.Sprintf(
				"0x%x...%x",
				bytes[:2],
				bytes[len(bytes)-2:],
			)
		} else {
			messageDigest = fmt.Sprintf("0x%x", bytes)
		}

		messagesDigests[i] = messageDigest
	}

	signingBatchLogger := logger.With(
		zap.String("wallet", fmt.Sprintf("0x%x", walletPublicKeyBytes)),
		zap.String("signedMessages", strings.Join(messagesDigests, ", ")),
	)

	signingStartBlock := startBlock // start block for the first signing
	signatures := make([]*frost.Signature, len(messages))
	endBlocks := make([]uint64, len(messages))

	for i, message := range messages {
		signingBatchMessageLogger := signingBatchLogger.With(
			zap.String("signedMessage", fmt.Sprintf("0x%x", message)),
			zap.String("index", fmt.Sprintf("%v/%v", i+1, len(messages))),
		)

		signingBatchMessageLogger.Infof("generating signature for message")

		if i > 0 {
			signingStartBlock = endBlocks[i-1] + signingBatchInterludeBlocks
		}

		var taprootMerkleRoot *[32]byte
		if taprootMerkleRoots != nil {
			taprootMerkleRoot = taprootMerkleRoots[i]
		}

		signature, _, endBlock, err := se.signWithTaprootMerkleRoot(
			ctx,
			message,
			taprootMerkleRoot,
			signingStartBlock,
		)
		if err != nil {
			// Error metrics are recorded in the sign() method for all error paths.
			return nil, err
		}

		signingBatchMessageLogger.Infof(
			"generated signature [%v] for message at block [%v]",
			signature,
			endBlock,
		)

		signatures[i] = signature
		endBlocks[i] = endBlock
	}

	return signatures, nil
}

// sign performs the signing process for the given message. The process is
// triggered according to the given start block. If the message cannot be signed
// within a limited time window, an error is returned. If the message was
// signed successfully, this function returns the signature along with the
// number of active members that participated in signing, the block at which the
// signature was calculated. The end block is common for all wallet signers so
// can be used as a synchronization point.
func (se *signingExecutor) sign(
	ctx context.Context,
	message *big.Int,
	startBlock uint64,
) (*frost.Signature, *signingActivityReport, uint64, error) {
	return se.signWithTaprootMerkleRoot(ctx, message, nil, startBlock)
}

func (se *signingExecutor) signWithTaprootMerkleRoot(
	ctx context.Context,
	message *big.Int,
	taprootMerkleRoot *[32]byte,
	startBlock uint64,
) (*frost.Signature, *signingActivityReport, uint64, error) {
	if lockAcquired := se.lock.TryAcquire(1); !lockAcquired {
		// Record failure metrics for lock acquisition failure
		if se.metricsRecorder != nil {
			se.metricsRecorder.IncrementCounter(clientinfo.MetricSigningOperationsTotal, 1)
			se.metricsRecorder.IncrementCounter(clientinfo.MetricSigningFailedTotal, 1)
		}
		return nil, nil, 0, errSigningExecutorBusy
	}
	defer se.lock.Release(1)

	startTime := time.Now()

	if se.metricsRecorder != nil {
		se.metricsRecorder.IncrementCounter(clientinfo.MetricSigningOperationsTotal, 1)
	}

	wallet := se.wallet()

	walletPublicKeyBytes, err := marshalPublicKey(wallet.publicKey)
	if err != nil {
		// Record failure metrics for marshal error
		if se.metricsRecorder != nil {
			se.metricsRecorder.IncrementCounter(clientinfo.MetricSigningFailedTotal, 1)
		}
		return nil, nil, 0, fmt.Errorf("cannot marshal wallet public key: [%v]", err)
	}

	loopTimeoutBlock := startBlock +
		uint64(se.signingAttemptsLimit*signingAttemptMaximumBlocks())

	signingLogger := logger.With(
		zap.String("wallet", fmt.Sprintf("0x%x", walletPublicKeyBytes)),
		zap.String("signedMessage", fmt.Sprintf("0x%x", message)),
		zap.Uint64("signingStartBlock", startBlock),
		zap.Uint64("signingTimeoutBlock", loopTimeoutBlock),
	)

	type signingOutcome struct {
		signature      *frost.Signature
		activityReport *signingActivityReport
		endBlock       uint64
	}

	wg := sync.WaitGroup{}
	wg.Add(len(se.signers))
	signingOutcomeChan := make(chan *signingOutcome, len(se.signers))

	// roastSID is the STABLE ROAST session id (no attempt number) shared by every
	// signer's retry loop and signing request, so the ROAST participant selector
	// and transition-record registry are keyed consistently across this signing's
	// attempts. Computed once; constant across signers and attempts.
	roastSID := roastSessionID(message, taprootMerkleRoot, startBlock)

	for _, currentSigner := range se.signers {
		go func(signer *signer) {
			se.protocolLatch.Lock()
			defer se.protocolLatch.Unlock()

			defer wg.Done()

			announcer := announcer.New(
				fmt.Sprintf("%v-%v", ProtocolName, "signing"),
				se.broadcastChannel,
				se.membershipValidator,
			)

			doneCheck := newSigningDoneCheck(
				se.groupParameters.GroupSize,
				se.broadcastChannel,
				se.membershipValidator,
			)

			retryLoop := newSigningRetryLoop(
				signingLogger,
				message,
				roastSID,
				startBlock,
				signer.signingGroupMemberIndex,
				wallet.signingGroupOperators,
				se.groupParameters,
				announcer,
				doneCheck,
			)

			// Every local signer of this wallet observes the same
			// network-wide attempts, so exactly one of them reports
			// outcomes to the liveness tracker to avoid multiplying
			// observations of a single attempt.
			if signer == se.signers[0] && se.livenessTracker != nil {
				retryLoop.setAttemptOutcomeReporter(
					se.livenessTracker.RecordAttemptOutcome,
				)
			}

			// Set up the loop timeout signal. This context is associated with
			// all attempts and gets canceled in three situations:
			// - one of the attempts failed with an error,
			// - we gave up after executing multiple attempts,
			// - one of the attempts succeeded.
			// In the last case, the context is not canceled immediately but,
			// we wait until the timeout for the successful attempt is done.
			// This lets the inner context to retransmit the signing done
			// messages for a longer period of time than just for the actual
			// execution time from the perspective of the current member.
			// This is important to ensure everyone has a chance to receive
			// the signing done message broadcasted by the current member.
			loopCtx, cancelLoopCtx := withCancelOnBlock(
				ctx,
				loopTimeoutBlock,
				se.waitForBlockFn,
			)

			// RFC-21 Phase 7.3 PR2b-1b: install the per-signer ROAST transition
			// controller, scoped to loopCtx (the session lifetime). It observes
			// every attempt so this seat can verify the attempt's transition bundle
			// and run NextAttempt for participant selection. nil in builds/
			// deployments without ROAST retry, in which case the loop skips all
			// transition steps. The request template carries the static signing
			// material; the controller stamps each attempt's metadata itself.
			retryLoop.setTransitionController(newRoastTransitionController(
				loopCtx,
				signingLogger,
				&signing.Request{
					Message:           message,
					RoastSessionID:    roastSID,
					MemberIndex:       signer.signingGroupMemberIndex,
					SignerMaterial:    signer.signingMaterial(),
					TaprootMerkleRoot: taprootMerkleRoot,
					GroupSize:         wallet.groupSize(),
					DishonestThreshold: wallet.groupDishonestThreshold(
						se.groupParameters.HonestThreshold,
					),
					Channel:             se.broadcastChannel,
					MembershipValidator: se.membershipValidator,
				},
				se.waitForBlockFn,
			))

			loopResult, err := retryLoop.start(
				loopCtx,
				se.waitForBlockFn,
				se.getCurrentBlockFn,
				func(attempt *signingAttemptParams) (*signing.Result, uint64, error) {
					signingAttemptLogger := signingLogger.With(
						zap.Uint("attemptNumber", attempt.number),
						zap.Uint64("attemptStartBlock", attempt.startBlock),
						zap.Uint64("attemptTimeoutBlock", attempt.timeoutBlock),
					)

					includedMembersIndexes := attemptIncludedMembersIndexes(
						wallet.groupSize(),
						attempt.excludedMembersIndexes,
					)

					coordinatorMemberIndex, err := roast.SelectCoordinator(
						includedMembersIndexes,
						signingAttemptSeed(message),
						attempt.number,
					)
					if err != nil {
						return nil, 0, fmt.Errorf(
							"cannot select signing coordinator for attempt [%v]: [%w]",
							attempt.number,
							err,
						)
					}

					// RFC-21 Phase 7.3 PR2b-1b: attempt.number is the committed roast
					// attempt number under active ROAST retry (set by the loop), so the
					// coordinator election, session id, and this AttemptContext all key
					// off the committed identity -- matching the observe/transition
					// context. TransientlyParkedMembersIndexes is carried through so the
					// active context's parking is byte-identical to the observe context's
					// (BuildAttemptContextFromRequest splits Excluded into permanent +
					// parked from it).
					attemptInfo := &signing.Attempt{
						Number:                          attempt.number,
						CoordinatorMemberIndex:          coordinatorMemberIndex,
						IncludedMembersIndexes:          includedMembersIndexes,
						ExcludedMembersIndexes:          attempt.excludedMembersIndexes,
						TransientlyParkedMembersIndexes: attempt.transientlyParkedMembersIndexes,
					}

					signingAttemptLogger.Infof(
						"[member:%v] starting signing protocol "+
							"with [%v] group members (coordinator: [%v], excluded: [%v])",
						signer.signingGroupMemberIndex,
						len(includedMembersIndexes),
						coordinatorMemberIndex,
						attemptInfo.ExcludedMembersIndexes,
					)

					// Set up the attempt timeout signal.
					// This context is associated with the current attempt and
					// gets canceled when the timeout for the current attempt is
					// hit. The context is not canceled earlier, even if the
					// execution succeeded. This is needed to ensure all
					// protocol participants, even the slowest one, have
					// a chance to receive all messages sent by this member
					// and complete the protocol.
					attemptCtx, _ := withCancelOnBlock(
						loopCtx,
						attempt.timeoutBlock,
						se.waitForBlockFn,
					)

					sessionID := signingSessionID(
						message,
						taprootMerkleRoot,
						startBlock,
						attempt.number,
					)

					result, err := signing.ExecuteRequest(
						attemptCtx,
						signingAttemptLogger,
						&signing.Request{
							Message:           message,
							SessionID:         sessionID,
							RoastSessionID:    roastSID,
							MemberIndex:       signer.signingGroupMemberIndex,
							SignerMaterial:    signer.signingMaterial(),
							PrivateKeyShare:   signer.privateKeyShare,
							TaprootMerkleRoot: taprootMerkleRoot,
							GroupSize:         wallet.groupSize(),
							DishonestThreshold: wallet.groupDishonestThreshold(
								se.groupParameters.HonestThreshold,
							),
							Channel:             se.broadcastChannel,
							MembershipValidator: se.membershipValidator,
							Attempt:             attemptInfo,
						},
					)
					if err != nil {
						return nil, 0, err
					}

					endBlock, err := se.getCurrentBlockFn()
					if err != nil {
						return nil, 0, err
					}

					return result, endBlock, nil
				},
			)
			if err != nil {
				// Signer failed so there is no point to hold the loopCtx.
				// Cancel it regardless of their timeout.
				cancelLoopCtx()

				signingLogger.Errorf(
					"[member:%v] all retries for the signing failed; "+
						"giving up: [%v]",
					signer.signingGroupMemberIndex,
					err,
				)

				return
			}

			// Just as mentioned in the comment above the definition of loopCtx,
			// do not cancel the loopCtx upon function exit immediately and
			// continue to broadcast signing done checks until the successful
			// attempt timeout. This way we maximize the chance that other
			// members, especially the ones not participating in the successful
			// signature attempt, receive the done checks as well.
			go func() {
				defer cancelLoopCtx()

				err := se.waitForBlockFn(
					loopCtx,
					loopResult.attemptTimeoutBlock,
				)
				if err != nil {
					signingLogger.Warnf(
						"[member:%v] failed waiting for signing "+
							"loop stop signal: [%v]",
						signer.signingGroupMemberIndex,
						err,
					)
				}
			}()

			signingLogger.Infof(
				"[member:%v] generated signature [%v] at block [%v]",
				signer.signingGroupMemberIndex,
				loopResult.result.Signature,
				loopResult.latestEndBlock,
			)

			signingOutcomeChan <- &signingOutcome{
				signature:      loopResult.result.Signature,
				activityReport: loopResult.activityReport,
				endBlock:       loopResult.latestEndBlock,
			}
		}(currentSigner)
	}

	// Wait until all controlled signers complete their signing routines,
	// regardless of their result.
	wg.Wait()

	// Take the first outcome from the channel as the outcome of all members.
	// This assumption is totally valid because the signing loop produces a
	// result only if all signers who participated in signing confirmed they
	// are done by sending a valid `signingDoneMessage` during the signing done
	// check phase. If the result was not inserted to the channel by any
	// signer, that means all signers failed and have not produced a signature.
	select {
	case outcome := <-signingOutcomeChan:
		if se.metricsRecorder != nil {
			se.metricsRecorder.IncrementCounter(clientinfo.MetricSigningSuccessTotal, 1)
			se.metricsRecorder.RecordDuration(clientinfo.MetricSigningDurationSeconds, time.Since(startTime))
		}
		return outcome.signature, outcome.activityReport, outcome.endBlock, nil
	default:
		if se.metricsRecorder != nil {
			// All signers failed to produce a signature within the timeout period.
			// This is counted as both a failure and a timeout.
			// Note: Non-timeout errors (e.g., member selection failures) cause
			// early return via cancelLoopCtx() and never reach this default case.
			// Therefore, all failures reaching here are actual timeouts.
			se.metricsRecorder.IncrementCounter(clientinfo.MetricSigningFailedTotal, 1)
			se.metricsRecorder.IncrementCounter(clientinfo.MetricSigningTimeoutsTotal, 1)
			se.metricsRecorder.RecordDuration(clientinfo.MetricSigningDurationSeconds, time.Since(startTime))
		}
		return nil, nil, 0, fmt.Errorf("all signers failed")
	}
}

func (se *signingExecutor) wallet() wallet {
	// All signers belong to one wallet. Take that wallet from the
	// first signer.
	return se.signers[0].wallet
}

func attemptIncludedMembersIndexes(
	groupSize int,
	excludedMembersIndexes []group.MemberIndex,
) []group.MemberIndex {
	excludedMembersIndexesSet := make(map[group.MemberIndex]bool)
	for _, excludedMemberIndex := range excludedMembersIndexes {
		excludedMembersIndexesSet[excludedMemberIndex] = true
	}

	includedMembersIndexes := make([]group.MemberIndex, 0)
	for i := 0; i < groupSize; i++ {
		memberIndex := group.MemberIndex(i + 1)
		if !excludedMembersIndexesSet[memberIndex] {
			includedMembersIndexes = append(includedMembersIndexes, memberIndex)
		}
	}

	return includedMembersIndexes
}

// setMetricsRecorder sets the metrics recorder for the signing executor.
func (se *signingExecutor) setMetricsRecorder(recorder interface {
	IncrementCounter(name string, value float64)
	SetGauge(name string, value float64)
	RecordDuration(name string, duration time.Duration)
}) {
	se.metricsRecorder = recorder

	// Recorders owning the process-wide signing-attempt liveness tracker
	// (RFC-21 Annex B implied-f alerting) share it with this executor;
	// no-op recorders simply leave the tracker disabled.
	if provider, ok := recorder.(signingLivenessTrackerProvider); ok {
		se.livenessTracker = provider.SigningAttemptLivenessTracker()
	}
}
