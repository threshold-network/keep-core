package tbtc

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/exp/slices"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/chain/local_v1"
	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/net/local"
	"github.com/keep-network/keep-core/pkg/operator"
	"github.com/keep-network/keep-core/pkg/protocol/group"
	"github.com/keep-network/keep-core/pkg/tecdsa"
	"github.com/keep-network/keep-core/pkg/tecdsa/signing"
)

// TestSigningDoneCheck is a happy path test.
func TestSigningDoneCheck(t *testing.T) {
	groupParameters := &GroupParameters{
		GroupSize:       5,
		GroupQuorum:     4,
		HonestThreshold: 3,
	}

	doneCheck := setupSigningDoneCheck(t, groupParameters)

	memberIndexes := make([]group.MemberIndex, doneCheck.groupSize)
	for i := range memberIndexes {
		memberIndex := group.MemberIndex(i + 1)
		memberIndexes[i] = memberIndex
	}

	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	message := big.NewInt(100)
	attemptNumber := uint64(2)
	attemptTimeoutBlock := uint64(1000)
	attemptMemberIndexes := memberIndexes[:groupParameters.HonestThreshold]
	result := &signing.Result{
		Signature: &tecdsa.Signature{
			R:          big.NewInt(200),
			S:          big.NewInt(300),
			RecoveryID: 2,
		},
	}

	type outcome struct {
		memberIndex group.MemberIndex
		result      *signing.Result
		endBlock    uint64
		err         error
	}

	// listen is called once, matching how a single local node uses
	// signingDoneCheck for one attempt; the other simulated members'
	// perspectives are represented by their signalDone messages arriving
	// over the (shared, in-memory) broadcast channel below, not by each of
	// them separately calling listen on this same instance.
	doneCheck.listen(
		ctx,
		message,
		attemptNumber,
		attemptTimeoutBlock,
		attemptMemberIndexes,
	)

	wg := sync.WaitGroup{}
	wg.Add(len(memberIndexes))
	outcomesChan := make(chan *outcome, len(memberIndexes))

	for _, memberIndex := range memberIndexes {
		go func(memberIndex group.MemberIndex) {
			defer wg.Done()

			if slices.Contains(attemptMemberIndexes, memberIndex) {
				err := doneCheck.signalDone(
					ctx,
					memberIndex,
					message,
					attemptNumber,
					result,
					500+uint64(memberIndex),
				)
				if err != nil {
					outcomesChan <- &outcome{err: err}
					return
				}
			}

			result, endBlock, err := doneCheck.waitUntilAllDone(ctx)

			outcomesChan <- &outcome{
				memberIndex: memberIndex,
				result:      result,
				endBlock:    endBlock,
				err:         err,
			}
		}(memberIndex)
	}

	wg.Wait()
	close(outcomesChan)

	// We exchanged `500 + uint64(memberIndex)` and latest member has index 3.
	expectedEndBlock := 503

	for outcome := range outcomesChan {
		if outcome.err != nil {
			t.Errorf(
				"unexpected error for member [%v]: [%v]",
				outcome.memberIndex,
				outcome.err,
			)
		}

		if outcome.result == nil {
			t.Errorf("unexpected nil result")
		}

		if !result.Signature.Equals(outcome.result.Signature) {
			t.Errorf(
				"unexpected signature for member [%v]\n"+
					"expected: [%v]\n"+
					"actual:   [%v]",
				outcome.memberIndex,
				result.Signature,
				outcome.result.Signature,
			)
		}

		testutils.AssertIntsEqual(
			t,
			fmt.Sprintf("end block for member [%v]", outcome.memberIndex),
			expectedEndBlock,
			int(outcome.endBlock),
		)
	}
}

// TestSigningDoneCheck_MissingConfirmation covers scenario when one member
// did not provide a done check on time.
func TestSigningDoneCheck_MissingConfirmation(t *testing.T) {
	groupParameters := &GroupParameters{
		GroupSize:       5,
		GroupQuorum:     4,
		HonestThreshold: 3,
	}

	doneCheck := setupSigningDoneCheck(t, groupParameters)

	memberIndexes := make([]group.MemberIndex, doneCheck.groupSize)
	for i := range memberIndexes {
		memberIndex := group.MemberIndex(i + 1)
		memberIndexes[i] = memberIndex
	}

	ctx, cancelCtx := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancelCtx()

	message := big.NewInt(100)
	attemptNumber := uint64(1)
	attemptTimeoutBlock := uint64(1000)
	attemptMemberIndexes := memberIndexes[:groupParameters.HonestThreshold]
	result := &signing.Result{
		Signature: &tecdsa.Signature{
			R:          big.NewInt(200),
			S:          big.NewInt(300),
			RecoveryID: 2,
		},
	}

	doneCheck.listen(
		ctx,
		message,
		attemptNumber,
		attemptTimeoutBlock,
		attemptMemberIndexes,
	)

	for i := 1; i < groupParameters.HonestThreshold; i++ {
		err := doneCheck.signalDone(
			ctx,
			uint8(i),
			message,
			attemptNumber,
			result,
			100,
		)
		if err != nil {
			t.Fatal(err)
		}
	}

	returnedResult, endBlock, err := doneCheck.waitUntilAllDone(ctx)

	if returnedResult != nil {
		t.Errorf("expected nil result, has [%v]", returnedResult)
	}
	testutils.AssertIntsEqual(t, "end block", 0, int(endBlock))
	testutils.AssertErrorsSame(t, errWaitDoneTimedOut, err)
}

// TestSigningDoneCheck_AnotherSignature covers scenario when one member
// did provide signature other than other members.
func TestSigningDoneCheck_AnotherSignature(t *testing.T) {
	groupParameters := &GroupParameters{
		GroupSize:       5,
		GroupQuorum:     4,
		HonestThreshold: 3,
	}

	doneCheck := setupSigningDoneCheck(t, groupParameters)

	memberIndexes := make([]group.MemberIndex, doneCheck.groupSize)
	for i := range memberIndexes {
		memberIndex := group.MemberIndex(i + 1)
		memberIndexes[i] = memberIndex
	}

	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	message := big.NewInt(100)
	attemptNumber := uint64(1)
	attemptTimeoutBlock := uint64(1000)
	attemptMemberIndexes := memberIndexes[:groupParameters.HonestThreshold]
	correctResult := &signing.Result{
		Signature: &tecdsa.Signature{
			R:          big.NewInt(200),
			S:          big.NewInt(300),
			RecoveryID: 2,
		},
	}
	incorrectResult := &signing.Result{
		Signature: &tecdsa.Signature{
			R:          big.NewInt(201),
			S:          big.NewInt(300),
			RecoveryID: 2,
		},
	}

	doneCheck.listen(
		ctx,
		message,
		attemptNumber,
		attemptTimeoutBlock,
		attemptMemberIndexes,
	)

	// groupParameters.HonestThreshold members provide correct signature
	for i := 1; i < groupParameters.HonestThreshold; i++ {
		err := doneCheck.signalDone(
			ctx,
			uint8(i),
			message,
			attemptNumber,
			correctResult,
			100,
		)
		if err != nil {
			t.Fatal(err)
		}
	}

	// one member provides incorrect signature
	err := doneCheck.signalDone(
		ctx,
		uint8(groupParameters.HonestThreshold),
		message,
		attemptNumber,
		incorrectResult,
		100,
	)
	if err != nil {
		t.Fatal(err)
	}

	// Give some time for the message handler goroutine
	time.Sleep(100 * time.Millisecond)

	returnedResult, endBlock, err := doneCheck.waitUntilAllDone(ctx)

	if returnedResult != nil {
		t.Errorf("expected nil result, has [%v]", returnedResult)
	}
	testutils.AssertIntsEqual(t, "end block", 0, int(endBlock))
	if !strings.Contains(err.Error(), "not matching signatures detected") {
		t.Errorf("unexpected error: [%v]", err)
	}
}

// TestSigningDoneCheck_ConcurrentDoneSignersAccess exercises the concurrent
// access to the doneSigners map: the listen goroutine writes to the map as done
// messages arrive while waitUntilAllDone reads it on every tick. It is meant to
// be run with the race detector (go test -race); without the doneSigners mutex
// guarding every access, the concurrent read/write is a data race.
//
// Done messages are broadcast with the standard, stateless retransmission
// strategy so this test isolates the doneSigners race and does not depend on
// the separate backoff-strategy synchronization.
func TestSigningDoneCheck_ConcurrentDoneSignersAccess(t *testing.T) {
	groupParameters := &GroupParameters{
		GroupSize:       5,
		GroupQuorum:     4,
		HonestThreshold: 3,
	}

	doneCheck := setupSigningDoneCheck(t, groupParameters)

	memberIndexes := make([]group.MemberIndex, doneCheck.groupSize)
	for i := range memberIndexes {
		memberIndexes[i] = group.MemberIndex(i + 1)
	}

	ctx, cancelCtx := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelCtx()

	message := big.NewInt(100)
	attemptNumber := uint64(1)
	attemptTimeoutBlock := uint64(1000)
	attemptMemberIndexes := memberIndexes[:groupParameters.HonestThreshold]
	result := &signing.Result{
		Signature: &tecdsa.Signature{
			R:          big.NewInt(200),
			S:          big.NewInt(300),
			RecoveryID: 2,
		},
	}

	// Start the listener goroutine, which writes to doneSigners as messages
	// arrive.
	doneCheck.listen(
		ctx,
		message,
		attemptNumber,
		attemptTimeoutBlock,
		attemptMemberIndexes,
	)

	// Concurrently broadcast every attempt member's done message so the listener
	// writes to doneSigners while waitUntilAllDone reads it below.
	for _, memberIndex := range attemptMemberIndexes {
		go func(memberIndex group.MemberIndex) {
			_ = doneCheck.broadcastChannel.Send(
				ctx,
				&signingDoneMessage{
					senderID:      memberIndex,
					message:       message,
					attemptNumber: attemptNumber,
					signature:     result.Signature,
					endBlock:      500 + uint64(memberIndex),
				},
				net.StandardRetransmissionStrategy,
			)
		}(memberIndex)
	}

	returnedResult, _, err := doneCheck.waitUntilAllDone(ctx)
	if err != nil {
		t.Fatalf("unexpected error: [%v]", err)
	}
	if returnedResult == nil {
		t.Fatal("unexpected nil result")
	}
	if !result.Signature.Equals(returnedResult.Signature) {
		t.Errorf(
			"unexpected signature\nexpected: [%v]\nactual:   [%v]",
			result.Signature,
			returnedResult.Signature,
		)
	}
}

// TestSigningDoneCheck_RejectsNonAttemptMember covers the scenario where a
// valid wallet group member that was not selected for the current signing
// attempt sends a done message. Such a message must not count toward the
// attempt completion threshold; otherwise the check could complete before all
// selected signers finish.
func TestSigningDoneCheck_RejectsNonAttemptMember(t *testing.T) {
	groupParameters := &GroupParameters{
		GroupSize:       5,
		GroupQuorum:     4,
		HonestThreshold: 3,
	}

	doneCheck := setupSigningDoneCheck(t, groupParameters)

	memberIndexes := make([]group.MemberIndex, doneCheck.groupSize)
	for i := range memberIndexes {
		memberIndexes[i] = group.MemberIndex(i + 1)
	}

	// The attempt selects members 1, 2, and 3. Member 5 is a valid wallet group
	// member but is excluded from this attempt.
	attemptMemberIndexes := memberIndexes[:groupParameters.HonestThreshold]
	nonAttemptMember := memberIndexes[groupParameters.GroupSize-1]

	ctx, cancelCtx := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancelCtx()

	message := big.NewInt(100)
	attemptNumber := uint64(1)
	attemptTimeoutBlock := uint64(1000)
	result := &signing.Result{
		Signature: &tecdsa.Signature{
			R:          big.NewInt(200),
			S:          big.NewInt(300),
			RecoveryID: 2,
		},
	}

	doneCheck.listen(
		ctx,
		message,
		attemptNumber,
		attemptTimeoutBlock,
		attemptMemberIndexes,
	)

	// Two attempt members (1 and 2) plus the excluded member (5) send done
	// messages. If the excluded member were counted, waitUntilAllDone would see
	// three messages, match the expected count, and complete before the third
	// attempt member (3) reports.
	for _, memberIndex := range []group.MemberIndex{
		attemptMemberIndexes[0],
		attemptMemberIndexes[1],
		nonAttemptMember,
	} {
		err := doneCheck.signalDone(
			ctx,
			memberIndex,
			message,
			attemptNumber,
			result,
			100,
		)
		if err != nil {
			t.Fatal(err)
		}
	}

	// The excluded member must not count toward the completion threshold, so
	// waitUntilAllDone must time out rather than returning a result.
	returnedResult, endBlock, err := doneCheck.waitUntilAllDone(ctx)
	if returnedResult != nil {
		t.Errorf("expected nil result, got [%v]", returnedResult)
	}
	testutils.AssertIntsEqual(t, "end block", 0, int(endBlock))
	testutils.AssertErrorsSame(t, errWaitDoneTimedOut, err)
}

// TestSigningDoneCheck_StaleAttemptDoesNotAffectNextAttempt verifies the fix
// for the retry-interleaving hazard: production code (signing.go,
// signing_loop.go) constructs a fresh signingDoneCheck for every retry
// attempt instead of reusing one instance. This test reproduces the
// interleaving that motivates that fix - an earlier attempt's listener is
// still running (it failed before reaching waitUntilAllDone, so nothing
// canceled its receive context yet) when the next attempt starts listening,
// and a stale done message from the earlier attempt is delivered after that
// - and asserts the stale message cannot be miscounted into the next
// attempt's result. Run with -race: reusing one instance for both attempts
// here would both data-race on receiveCtx/doneSigners and let the stale
// message land in the next attempt's map, since isValidDoneMessage's
// membership map lookup and both listener goroutines would share it.
func TestSigningDoneCheck_StaleAttemptDoesNotAffectNextAttempt(t *testing.T) {
	groupParameters := &GroupParameters{
		GroupSize:       5,
		GroupQuorum:     4,
		HonestThreshold: 3,
	}

	newDoneCheck := setupSigningDoneCheckFactory(t, groupParameters)

	memberIndexes := make([]group.MemberIndex, groupParameters.GroupSize)
	for i := range memberIndexes {
		memberIndexes[i] = group.MemberIndex(i + 1)
	}
	attemptMemberIndexes := memberIndexes[:groupParameters.HonestThreshold]

	message := big.NewInt(100)
	attemptTimeoutBlock := uint64(1000)

	ctx, cancelCtx := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelCtx()

	// Attempt 1 starts listening but never reaches waitUntilAllDone, as if
	// signingAttemptFn or signalDone had failed right after listen - the
	// exact interleaving signing_loop.go's early continue paths handle.
	attempt1DoneCheck := newDoneCheck()
	defer attempt1DoneCheck.stopListening()
	attempt1DoneCheck.listen(ctx, message, 1, attemptTimeoutBlock, attemptMemberIndexes)

	// Attempt 2 gets its own, fresh instance sharing the same broadcast
	// channel - this is the fix under test.
	attempt2DoneCheck := newDoneCheck()
	defer attempt2DoneCheck.stopListening()
	attempt2DoneCheck.listen(ctx, message, 2, attemptTimeoutBlock, attemptMemberIndexes)

	staleResult := &signing.Result{
		Signature: &tecdsa.Signature{R: big.NewInt(999), S: big.NewInt(999), RecoveryID: 1},
	}
	attempt2Result := &signing.Result{
		Signature: &tecdsa.Signature{R: big.NewInt(200), S: big.NewInt(300), RecoveryID: 2},
	}

	// A stale attempt-1 done message (e.g. a retransmission) arrives late,
	// after attempt 2 has already started listening.
	if err := attempt1DoneCheck.signalDone(
		ctx, attemptMemberIndexes[0], message, 1, staleResult, 600,
	); err != nil {
		t.Fatal(err)
	}

	// The real attempt-2 quorum signals done.
	for _, memberIndex := range attemptMemberIndexes {
		if err := attempt2DoneCheck.signalDone(
			ctx, memberIndex, message, 2, attempt2Result, 500+uint64(memberIndex),
		); err != nil {
			t.Fatal(err)
		}
	}

	returnedResult, _, err := attempt2DoneCheck.waitUntilAllDone(ctx)
	if err != nil {
		t.Fatalf("unexpected error: [%v]", err)
	}
	if returnedResult == nil || !returnedResult.Signature.Equals(attempt2Result.Signature) {
		t.Fatalf(
			"expected attempt 2 to complete with its own signature, got [%v]",
			returnedResult,
		)
	}

	// The stale attempt-1 message must not have been miscounted into
	// attempt 2's map: exactly the attempt-2 quorum, no more.
	attempt2DoneCheck.doneSignersMutex.Lock()
	doneSignersCount := len(attempt2DoneCheck.doneSigners)
	attempt2DoneCheck.doneSignersMutex.Unlock()
	testutils.AssertIntsEqual(
		t,
		"attempt 2 doneSigners count",
		len(attemptMemberIndexes),
		doneSignersCount,
	)
}

// setupSigningDoneCheck sets up an instance of the signing done check ready
// to perform test checks.
func setupSigningDoneCheck(
	t *testing.T,
	groupParameters *GroupParameters,
) *signingDoneCheck {
	return setupSigningDoneCheckFactory(t, groupParameters)()
}

// setupSigningDoneCheckFactory sets up the shared network plumbing (broadcast
// channel, membership validator) a signing done check needs, and returns a
// factory that constructs a fresh signingDoneCheck sharing that plumbing -
// mirroring how production code (see signing.go) constructs one instance per
// retry attempt rather than reusing a single instance.
func setupSigningDoneCheckFactory(
	t *testing.T,
	groupParameters *GroupParameters,
) func() *signingDoneCheck {
	operatorPrivateKey, operatorPublicKey, err := operator.GenerateKeyPair(
		local_v1.DefaultCurve,
	)
	if err != nil {
		t.Fatal(err)
	}

	localChain := ConnectWithKey(operatorPrivateKey)

	localProvider := local.ConnectWithKey(operatorPublicKey)

	operatorAddress, err := localChain.Signing().PublicKeyToAddress(
		operatorPublicKey,
	)
	if err != nil {
		t.Fatal(err)
	}

	var operators []chain.Address
	for i := 0; i < groupParameters.GroupSize; i++ {
		operators = append(operators, operatorAddress)
	}

	broadcastChannel, err := localProvider.BroadcastChannelFor("channel")
	if err != nil {
		t.Fatal(err)
	}

	broadcastChannel.SetUnmarshaler(func() net.TaggedUnmarshaler {
		return &signingDoneMessage{}
	})

	membershipValidator := group.NewMembershipValidator(
		&testutils.MockLogger{},
		operators,
		localChain.Signing(),
	)

	return func() *signingDoneCheck {
		return newSigningDoneCheck(
			groupParameters.GroupSize,
			broadcastChannel,
			membershipValidator,
		)
	}
}
