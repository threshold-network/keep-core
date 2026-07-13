package tbtc

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sync"

	"github.com/ipfs/go-log/v2"
	"go.uber.org/zap"
	"golang.org/x/sync/semaphore"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/generator"
	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/protocol/group"
	"github.com/keep-network/keep-core/pkg/protocol/inactivity"
)

const (
	// inactivityClaimSubmissionDelayStepBlocks determines the delay step in blocks
	// that is used to calculate the submission delay period that should be respected
	// by the given member to avoid all members submitting the same inactivity claim
	// at the same time.
	inactivityClaimSubmissionDelayStepBlocks = 2
)

// errInactivityClaimExecutorBusy is an error returned when the inactivity claim
// executor cannot execute the inactivity claim due to another inactivity claim
// execution in progress.
var errInactivityClaimExecutorBusy = fmt.Errorf("inactivity claim executor is busy")

type inactivityClaimExecutor struct {
	lock *semaphore.Weighted

	chain               Chain
	signers             []*signer
	broadcastChannel    net.BroadcastChannel
	membershipValidator *group.MembershipValidator
	groupParameters     *GroupParameters
	protocolLatch       *generator.ProtocolLatch

	waitForBlockFn waitForBlockFn
}

func newInactivityClaimExecutor(
	chain Chain,
	signers []*signer,
	broadcastChannel net.BroadcastChannel,
	membershipValidator *group.MembershipValidator,
	groupParameters *GroupParameters,
	protocolLatch *generator.ProtocolLatch,
	waitForBlockFn waitForBlockFn,
) *inactivityClaimExecutor {
	return &inactivityClaimExecutor{
		lock:                semaphore.NewWeighted(1),
		chain:               chain,
		signers:             signers,
		broadcastChannel:    broadcastChannel,
		membershipValidator: membershipValidator,
		groupParameters:     groupParameters,
		protocolLatch:       protocolLatch,
		waitForBlockFn:      waitForBlockFn,
	}
}

func (ice *inactivityClaimExecutor) claimInactivity(
	ctx context.Context,
	inactiveMembersIndexes []group.MemberIndex,
	heartbeatFailed bool,
	sessionID *big.Int,
) error {
	if lockAcquired := ice.lock.TryAcquire(1); !lockAcquired {
		return errInactivityClaimExecutorBusy
	}
	defer ice.lock.Release(1)

	wallet := ice.wallet()

	walletPublicKeyHash := bitcoin.PublicKeyHash(wallet.publicKey)
	walletPublicKeyBytes, err := marshalPublicKey(wallet.publicKey)
	if err != nil {
		return fmt.Errorf("cannot marshal wallet public key: [%v]", err)
	}

	execLogger := logger.With(
		zap.String("wallet", fmt.Sprintf("0x%x", walletPublicKeyBytes)),
		zap.String("sessionID", fmt.Sprintf("0x%x", sessionID)),
	)

	walletRegistryData, err := ice.chain.GetWallet(walletPublicKeyHash)
	if err != nil {
		return fmt.Errorf("could not get registry data on wallet: [%v]", err)
	}

	nonce, err := ice.chain.GetInactivityClaimNonce(
		walletRegistryData.EcdsaWalletID,
	)
	if err != nil {
		return fmt.Errorf("could not get nonce for wallet: [%v]", err)
	}

	claim := inactivity.NewClaimPreimage(
		nonce,
		wallet.publicKey,
		inactiveMembersIndexes,
		heartbeatFailed,
	)

	groupMembers, err := ice.getWalletOperatorsIDs()
	if err != nil {
		return fmt.Errorf("could not get wallet members info: [%v]", err)
	}

	wg := sync.WaitGroup{}
	wg.Add(len(ice.signers))

	for _, currentSigner := range ice.signers {
		go func(signer *signer) {
			ice.protocolLatch.Lock()
			defer ice.protocolLatch.Unlock()

			defer wg.Done()

			execLogger.Infof(
				"[member:%v] starting inactivity claim publishing",
				signer.signingGroupMemberIndex,
			)

			signerCtx, cancelSignerCtx := context.WithCancel(ctx)
			defer cancelSignerCtx()

			subscription := ice.chain.OnInactivityClaimed(
				func(event *InactivityClaimedEvent) {
					defer cancelSignerCtx()

					execLogger.Infof(
						"[member:%v] Inactivity claim submitted for wallet "+
							"with ID [0x%x] and nonce [%v] by notifier [%v] "+
							"at block [%v]",
						signer.signingGroupMemberIndex,
						event.WalletID,
						event.Nonce,
						event.Notifier,
						event.BlockNumber,
					)
				})
			defer subscription.Unsubscribe()

			err := ice.publishInactivityClaim(
				signerCtx,
				execLogger,
				sessionID,
				signer.signingGroupMemberIndex,
				wallet.groupSize(),
				wallet.groupDishonestThreshold(
					ice.groupParameters.HonestThreshold,
				),
				groupMembers,
				ice.membershipValidator,
				claim,
			)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					execLogger.Infof(
						"[member:%v] inactivity claim is no longer awaiting "+
							"publishing; aborting inactivity claim publishing",
						signer.signingGroupMemberIndex,
					)
					return
				}

				execLogger.Errorf(
					"[member:%v] inactivity claim publishing failed [%v]",
					signer.signingGroupMemberIndex,
					err,
				)
				return
			}
		}(currentSigner)
	}

	// Wait until all controlled signers complete their routine.
	wg.Wait()

	return nil
}

func (ice *inactivityClaimExecutor) getWalletOperatorsIDs() ([]uint32, error) {
	// Cache mapping operator addresses to their wallet member IDs. It helps to
	// limit the number of calls to the ETH client if some operator addresses
	// occur on the list multiple times.
	operatorIDCache := make(map[chain.Address]uint32)

	walletMemberIDs := make([]uint32, 0)

	for _, operatorAddress := range ice.wallet().signingGroupOperators {
		// Search for the operator address in the cache. Store the operator
		// address in the cache if it's not there.
		if operatorID, found := operatorIDCache[operatorAddress]; !found {
			fetchedOperatorID, err := ice.chain.GetOperatorID(operatorAddress)
			if err != nil {
				return nil, fmt.Errorf("could not get operator ID: [%w]", err)
			}
			operatorIDCache[operatorAddress] = fetchedOperatorID
			walletMemberIDs = append(walletMemberIDs, fetchedOperatorID)
		} else {
			walletMemberIDs = append(walletMemberIDs, operatorID)
		}
	}

	return walletMemberIDs, nil
}

func (ice *inactivityClaimExecutor) publishInactivityClaim(
	ctx context.Context,
	inactivityLogger log.StandardLogger,
	sessionID *big.Int,
	memberIndex group.MemberIndex,
	groupSize int,
	dishonestThreshold int,
	groupMembers []uint32,
	membershipValidator *group.MembershipValidator,
	inactivityClaim *inactivity.ClaimPreimage,
) error {
	return inactivity.PublishClaim(
		ctx,
		inactivityLogger,
		sessionID.Text(16),
		memberIndex,
		ice.broadcastChannel,
		groupSize,
		dishonestThreshold,
		membershipValidator,
		newInactivityClaimSigner(ice.chain),
		newInactivityClaimSubmitter(
			inactivityLogger,
			ice.chain,
			ice.groupParameters,
			groupMembers,
			ice.waitForBlockFn,
		),
		inactivityClaim,
	)
}

func (ice *inactivityClaimExecutor) wallet() wallet {
	// All signers belong to one wallet. Take that wallet from the
	// first signer.
	return ice.signers[0].wallet
}

// inactivityClaimSigner is responsible for signing the inactivity claim and
// verification of signatures generated by other group members.
type inactivityClaimSigner struct {
	chain Chain
}

func newInactivityClaimSigner(
	chain Chain,
) *inactivityClaimSigner {
	return &inactivityClaimSigner{
		chain: chain,
	}
}

func (ics *inactivityClaimSigner) SignClaim(claim *inactivity.ClaimPreimage) (
	*inactivity.SignedClaimHash,
	error,
) {
	if claim == nil {
		return nil, fmt.Errorf("claim is nil")
	}

	claimHash, err := ics.chain.CalculateInactivityClaimHash(claim)
	if err != nil {
		return nil, fmt.Errorf(
			"inactivity claim hash calculation failed [%w]",
			err,
		)
	}

	signing := ics.chain.Signing()

	signature, err := signing.Sign(claimHash[:])
	if err != nil {
		return nil, fmt.Errorf(
			"inactivity claim hash signing failed [%w]",
			err,
		)
	}

	return &inactivity.SignedClaimHash{
		PublicKey: signing.PublicKey(),
		Signature: signature,
		ClaimHash: claimHash,
	}, nil
}

// VerifySignature verifies if the signature was generated from the provided
// inactivity claim using the provided public key.
func (ics *inactivityClaimSigner) VerifySignature(
	signedClaim *inactivity.SignedClaimHash,
) (
	bool,
	error,
) {
	return ics.chain.Signing().VerifyWithPublicKey(
		signedClaim.ClaimHash[:],
		signedClaim.Signature,
		signedClaim.PublicKey,
	)
}

type inactivityClaimSubmitter struct {
	inactivityLogger log.StandardLogger

	chain           Chain
	groupParameters *GroupParameters
	groupMembers    []uint32

	waitForBlockFn waitForBlockFn
}

func newInactivityClaimSubmitter(
	inactivityLogger log.StandardLogger,
	chain Chain,
	groupParameters *GroupParameters,
	groupMembers []uint32,
	waitForBlockFn waitForBlockFn,
) *inactivityClaimSubmitter {
	return &inactivityClaimSubmitter{
		inactivityLogger: inactivityLogger,
		chain:            chain,
		groupParameters:  groupParameters,
		groupMembers:     groupMembers,
		waitForBlockFn:   waitForBlockFn,
	}
}

func (ics *inactivityClaimSubmitter) SubmitClaim(
	ctx context.Context,
	memberIndex group.MemberIndex,
	claim *inactivity.ClaimPreimage,
	signatures map[group.MemberIndex][]byte,
) error {
	if len(signatures) < ics.groupParameters.HonestThreshold {
		return fmt.Errorf(
			"could not submit inactivity claim with [%v] signatures for "+
				"group honest threshold [%v]",
			len(signatures),
			ics.groupParameters.HonestThreshold,
		)
	}

	// The inactivity nonce at the beginning of the execution process.
	inactivityNonce := claim.Nonce

	walletPublicKeyHash := bitcoin.PublicKeyHash(claim.WalletPublicKey)

	walletRegistryData, err := ics.chain.GetWallet(walletPublicKeyHash)
	if err != nil {
		return fmt.Errorf("could not get registry data on wallet: [%v]", err)
	}

	ecdsaWalletID := walletRegistryData.EcdsaWalletID

	currentNonce, err := ics.chain.GetInactivityClaimNonce(
		ecdsaWalletID,
	)
	if err != nil {
		return fmt.Errorf("could not get nonce for wallet: [%v]", err)
	}

	if currentNonce.Cmp(inactivityNonce) > 0 {
		// Someone who was ahead of us in the queue submitted the claim. Giving up.
		ics.inactivityLogger.Infof(
			"[member:%v] inactivity claim already submitted; "+
				"aborting inactivity claim on-chain submission",
			memberIndex,
		)
		return nil
	}

	chainClaim, err := ics.chain.AssembleInactivityClaim(
		ecdsaWalletID,
		claim.InactiveMembersIndexes,
		signatures,
		claim.HeartbeatFailed,
	)
	if err != nil {
		return fmt.Errorf("could not assemble inactivity chain claim [%w]", err)
	}

	blockCounter, err := ics.chain.BlockCounter()
	if err != nil {
		return err
	}

	// We can't determine a common block at which the publication starts.
	// However, all we want here is to ensure the members does not submit
	// in the same time. This can be achieved by simply using the index-based
	// delay starting from the current block.
	currentBlock, err := blockCounter.CurrentBlock()
	if err != nil {
		return fmt.Errorf("cannot get current block: [%v]", err)
	}
	delayBlocks := uint64(memberIndex-1) * inactivityClaimSubmissionDelayStepBlocks
	submissionBlock := currentBlock + delayBlocks

	ics.inactivityLogger.Infof(
		"[member:%v] waiting for block [%v] to submit inactivity claim",
		memberIndex,
		submissionBlock,
	)

	err = ics.waitForBlockFn(ctx, submissionBlock)
	if err != nil {
		return fmt.Errorf(
			"error while waiting for inactivity claim submission block: [%v]",
			err,
		)
	}

	if ctx.Err() != nil {
		// The context was cancelled by the upstream. Regardless of the cause,
		// that means the inactivity execution is no longer awaiting the result,
		//  and we can safely return.
		ics.inactivityLogger.Infof(
			"[member:%v] inactivity execution is no longer awaiting the "+
				"result; aborting inactivity claim on-chain submission",
			memberIndex,
		)
		return nil
	}

	// Re-check the nonce after the delay wait. Another member may have
	// submitted the claim while we were waiting.
	currentNonce, err = ics.chain.GetInactivityClaimNonce(ecdsaWalletID)
	if err != nil {
		return fmt.Errorf("could not get nonce for wallet: [%v]", err)
	}

	if currentNonce.Cmp(inactivityNonce) > 0 {
		ics.inactivityLogger.Infof(
			"[member:%v] inactivity claim already submitted; "+
				"aborting inactivity claim on-chain submission",
			memberIndex,
		)
		return nil
	}

	ics.inactivityLogger.Infof(
		"[member:%v] submitting inactivity claim with [%v] supporting "+
			"member signatures",
		memberIndex,
		len(signatures),
	)

	err = ics.chain.SubmitInactivityClaim(
		chainClaim,
		inactivityNonce,
		ics.groupMembers,
	)
	if err == nil {
		return nil
	}

	// The nonce can become stale between the initial check and actual
	// submission if another member submits the same claim first. In that
	// case, treat this as expected and return without error.
	currentNonce, nonceErr := ics.chain.GetInactivityClaimNonce(
		ecdsaWalletID,
	)
	if nonceErr == nil && currentNonce.Cmp(inactivityNonce) > 0 {
		ics.inactivityLogger.Infof(
			"[member:%v] inactivity claim already submitted by another "+
				"member; aborting inactivity claim on-chain submission",
			memberIndex,
		)
		return nil
	}

	return err
}
