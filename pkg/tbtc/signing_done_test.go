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
	"github.com/keep-network/keep-core/pkg/frost/signing"
	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/net/local"
	"github.com/keep-network/keep-core/pkg/operator"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// TestSigningDoneCheck is a happy path test.
func TestSigningDoneCheck(t *testing.T) {
	groupParameters := &GroupParameters{
		GroupSize:       5,
		GroupQuorum:     4,
		HonestThreshold: 3,
	}

	// Use components (shared channel + validator) so each goroutine below
	// gets its own signingDoneCheck instance. In production every operator
	// runs their own instance; sharing one here would race on its fields.
	components := setupSigningDoneCheckComponents(t, groupParameters)

	memberIndexes := make([]group.MemberIndex, components.groupSize)
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
		Signature: mustFrostSignatureFromBigInts(big.NewInt(200), big.NewInt(300)),
	}

	type outcome struct {
		memberIndex group.MemberIndex
		result      *signing.Result
		endBlock    uint64
		err         error
	}

	wg := sync.WaitGroup{}
	wg.Add(len(memberIndexes))
	outcomesChan := make(chan *outcome, len(memberIndexes))

	for _, memberIndex := range memberIndexes {
		go func(memberIndex group.MemberIndex) {
			defer wg.Done()

			doneCheck := components.newCheck()

			doneCheck.listen(
				ctx,
				message,
				attemptNumber,
				attemptTimeoutBlock,
				attemptMemberIndexes,
			)

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
		Signature: mustFrostSignatureFromBigInts(big.NewInt(200), big.NewInt(300)),
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
		Signature: mustFrostSignatureFromBigInts(big.NewInt(200), big.NewInt(300)),
	}
	incorrectResult := &signing.Result{
		Signature: mustFrostSignatureFromBigInts(big.NewInt(201), big.NewInt(300)),
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

// signingDoneCheckComponents holds the shared state used to construct one or
// more signingDoneCheck instances that communicate over the same channel.
type signingDoneCheckComponents struct {
	groupSize           int
	broadcastChannel    net.BroadcastChannel
	membershipValidator *group.MembershipValidator
}

func (c *signingDoneCheckComponents) newCheck() *signingDoneCheck {
	return newSigningDoneCheck(c.groupSize, c.broadcastChannel, c.membershipValidator)
}

// setupSigningDoneCheckComponents builds the shared channel and validator
// without constructing the signingDoneCheck itself, so callers that need
// multiple independent instances (e.g. simulating N operators) can each
// call newCheck() separately.
func setupSigningDoneCheckComponents(
	t *testing.T,
	groupParameters *GroupParameters,
) *signingDoneCheckComponents {
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

	return &signingDoneCheckComponents{
		groupSize:           groupParameters.GroupSize,
		broadcastChannel:    broadcastChannel,
		membershipValidator: membershipValidator,
	}
}

// setupSigningDoneCheck sets up an instance of the signing done check ready
// to perform test checks.
func setupSigningDoneCheck(
	t *testing.T,
	groupParameters *GroupParameters,
) *signingDoneCheck {
	return setupSigningDoneCheckComponents(t, groupParameters).newCheck()
}
