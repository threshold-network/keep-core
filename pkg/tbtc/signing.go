package tbtc

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/keep-network/keep-core/pkg/clientinfo"
	"github.com/keep-network/keep-core/pkg/generator"
	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/protocol/announcer"
	"github.com/keep-network/keep-core/pkg/protocol/group"
	"github.com/keep-network/keep-core/pkg/tecdsa"
	"github.com/keep-network/keep-core/pkg/tecdsa/signing"
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
	metricsRecorder      *clientinfo.MetricsRecorder
}

func newSigningExecutor(
	signers []*signer,
	broadcastChannel net.BroadcastChannel,
	membershipValidator *group.MembershipValidator,
	groupParameters *GroupParameters,
	protocolLatch *generator.ProtocolLatch,
	getCurrentBlockFn getCurrentBlockFn,
	waitForBlockFn waitForBlockFn,
	signingAttemptsLimit uint,
	metricsRecorder *clientinfo.MetricsRecorder,
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
		metricsRecorder:      metricsRecorder,
	}
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
) ([]*tecdsa.Signature, error) {
	batchStartTime := time.Now()
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
	signatures := make([]*tecdsa.Signature, len(messages))
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

		signature, _, endBlock, err := se.sign(ctx, message, signingStartBlock)
		if err != nil {
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

	// Record batch signing metrics
	if se.metricsRecorder != nil {
		batchDuration := time.Since(batchStartTime)
		se.metricsRecorder.RecordSigningBatch(len(messages), batchDuration)
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
) (*tecdsa.Signature, *signingActivityReport, uint64, error) {
	if lockAcquired := se.lock.TryAcquire(1); !lockAcquired {
		return nil, nil, 0, errSigningExecutorBusy
	}
	defer se.lock.Release(1)

	wallet := se.wallet()

	walletPublicKeyBytes, err := marshalPublicKey(wallet.publicKey)
	if err != nil {
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
		signature      *tecdsa.Signature
		activityReport *signingActivityReport
		endBlock       uint64
	}

	wg := sync.WaitGroup{}
	wg.Add(len(se.signers))
	signingOutcomeChan := make(chan *signingOutcome, len(se.signers))

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
				startBlock,
				signer.signingGroupMemberIndex,
				wallet.signingGroupOperators,
				se.groupParameters,
				announcer,
				doneCheck,
			)

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

			signingStartTime := time.Now()
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

					attemptStartTime := time.Now()

					signingAttemptLogger.Infof(
						"[member:%v] starting signing protocol "+
							"with [%v] group members (excluded: [%v])",
						signer.signingGroupMemberIndex,
						wallet.groupSize()-len(attempt.excludedMembersIndexes),
						attempt.excludedMembersIndexes,
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

					sessionID := fmt.Sprintf(
						"%v-%v",
						message.Text(16),
						attempt.number,
					)

					result, err := signing.Execute(
						attemptCtx,
						signingAttemptLogger,
						message,
						sessionID,
						signer.signingGroupMemberIndex,
						signer.privateKeyShare,
						wallet.groupSize(),
						wallet.groupDishonestThreshold(
							se.groupParameters.HonestThreshold,
						),
						attempt.excludedMembersIndexes,
						se.broadcastChannel,
						se.membershipValidator,
					)
					if err != nil {
						// Record failed signing attempt
						if se.metricsRecorder != nil {
							attemptDuration := time.Since(attemptStartTime)
							status := "failure"
							if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
								status = "timeout"
								se.metricsRecorder.RecordContextTimeout("signing_execution")
							}
							se.metricsRecorder.RecordSigningExecutionDuration(attemptDuration, status)
							se.metricsRecorder.RecordProtocolError("signing", "execution_failure")
						}
						return nil, 0, err
					}

					// Record successful signing attempt
					if se.metricsRecorder != nil {
						attemptDuration := time.Since(attemptStartTime)
						se.metricsRecorder.RecordSigningExecutionDuration(attemptDuration, "success")
					}

					endBlock, err := se.getCurrentBlockFn()
					if err != nil {
						return nil, 0, err
					}

					return result, endBlock, nil
				},
			)
			signingDuration := time.Since(signingStartTime)
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

				// Record failed signing metrics
				if se.metricsRecorder != nil {
					se.metricsRecorder.RecordSigningExecutionDuration(signingDuration, "failure")
					if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
						se.metricsRecorder.RecordOperationTimeout("signing")
					}
				}

				return
			}

			// Record successful signing metrics
			if se.metricsRecorder != nil {
				se.metricsRecorder.RecordSigningExecutionDuration(signingDuration, "success")
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
		return outcome.signature, outcome.activityReport, outcome.endBlock, nil
	default:
		return nil, nil, 0, fmt.Errorf("all signers failed")
	}
}

func (se *signingExecutor) wallet() wallet {
	// All signers belong to one wallet. Take that wallet from the
	// first signer.
	return se.signers[0].wallet
}
