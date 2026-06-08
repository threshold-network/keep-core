package tbtc

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"testing"
	"time"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/frost/signing"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

func TestSigningRetryLoop(t *testing.T) {
	message := big.NewInt(100)

	groupParameters := &GroupParameters{
		GroupSize:       10,
		HonestThreshold: 6,
	}

	signingGroupOperators := chain.Addresses{
		"address-1",
		"address-2",
		"address-8",
		"address-4",
		"address-2",
		"address-6",
		"address-7",
		"address-8",
		"address-9",
		"address-8",
	}

	signingGroupMembersIndexes := make([]group.MemberIndex, 0)
	for i := range signingGroupOperators {
		signingGroupMembersIndexes = append(
			signingGroupMembersIndexes,
			group.MemberIndex(i+1),
		)
	}

	testResult := &signing.Result{
		Signature: mustFrostSignatureFromBigInts(big.NewInt(300), big.NewInt(400)),
	}

	var tests = map[string]struct {
		signingGroupMemberIndex     group.MemberIndex
		ctxFn                       func() (context.Context, context.CancelFunc)
		currentBlockFn              getCurrentBlockFn
		incomingAnnouncementsFn     func(sessionID string) ([]group.MemberIndex, error)
		signingAttemptFn            signingAttemptFn
		waitUntilAllDoneOutcomeFn   func(attemptNumber uint64) (*signing.Result, uint64, error)
		expectedOutgoingDoneChecks  []*signingDoneMessage
		expectedErr                 error
		expectedResult              *signingRetryLoopResult
		expectedLastExecutedAttempt *signingAttemptParams
		outgoingAnnouncementsCount  uint
	}{
		"success on initial attempt": {
			signingGroupMemberIndex: 1,
			ctxFn: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 10*time.Second)
			},
			currentBlockFn: func() (uint64, error) {
				return 200, nil // same as the initial start block
			},
			incomingAnnouncementsFn: func(
				sessionID string,
			) ([]group.MemberIndex, error) {
				return signingGroupMembersIndexes, nil
			},
			signingAttemptFn: func(
				attempt *signingAttemptParams,
			) (*signing.Result, uint64, error) {
				return testResult, 215, nil // an arbitrary end block
			},
			waitUntilAllDoneOutcomeFn: func(attemptNumber uint64) (*signing.Result, uint64, error) {
				// Simulate that the done check phase determines the same
				// end block as the executing signer.
				return testResult, 215, nil
			},
			expectedOutgoingDoneChecks: []*signingDoneMessage{
				{
					senderID:      1,
					message:       message,
					attemptNumber: 1,
					signature:     testResult.Signature,
					endBlock:      215,
				},
			},
			expectedErr: nil,
			expectedResult: &signingRetryLoopResult{
				result: testResult,
				activityReport: &signingActivityReport{
					activeMembers:   signingGroupMembersIndexes,
					inactiveMembers: []group.MemberIndex{},
				},
				latestEndBlock:      215, // the end block resolved by the done check phase
				attemptTimeoutBlock: 236, // start block of the first attempt + 30
			},
			// The signing random retry algorithm invoked with the test seed
			// excludes 4 members (6 is the honest threshold) from the first
			// attempt: 3, 7, 8 and 10.
			expectedLastExecutedAttempt: &signingAttemptParams{
				number:                 1,
				startBlock:             206,
				timeoutBlock:           236, // start block of the first attempt + 30
				excludedMembersIndexes: []group.MemberIndex{3, 7, 8, 10},
			},
			outgoingAnnouncementsCount: 1,
		},
		"success on initial attempt with missing announcements and honest majority": {
			signingGroupMemberIndex: 1,
			ctxFn: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 10*time.Second)
			},
			currentBlockFn: func() (uint64, error) {
				return 200, nil // same as the initial start block
			},
			incomingAnnouncementsFn: func(
				sessionID string,
			) ([]group.MemberIndex, error) {
				// Honest majority of members announced their readiness.
				return []group.MemberIndex{1, 2, 3, 6, 7, 9}, nil
			},
			signingAttemptFn: func(
				attempt *signingAttemptParams,
			) (*signing.Result, uint64, error) {
				return testResult, 215, nil // an arbitrary end block
			},
			waitUntilAllDoneOutcomeFn: func(attemptNumber uint64) (*signing.Result, uint64, error) {
				// Simulate that the done check phase determines the same
				// end block as the executing signer.
				return testResult, 215, nil
			},
			expectedOutgoingDoneChecks: []*signingDoneMessage{
				{
					senderID:      1,
					message:       message,
					attemptNumber: 1,
					signature:     testResult.Signature,
					endBlock:      215,
				},
			},
			expectedErr: nil,
			expectedResult: &signingRetryLoopResult{
				result: testResult,
				activityReport: &signingActivityReport{
					activeMembers:   []group.MemberIndex{1, 2, 3, 6, 7, 9},
					inactiveMembers: []group.MemberIndex{4, 5, 8, 10},
				},
				latestEndBlock:      215, // the end block resolved by the done check phase
				attemptTimeoutBlock: 236, // start block of the first attempt + 30
			},
			// As only 6 members (honest threshold) announced their readiness,
			// we don't have any other option than select them for the attempt.
			// Not ready members are excluded.
			expectedLastExecutedAttempt: &signingAttemptParams{
				number:                 1,
				startBlock:             206,
				timeoutBlock:           236, // start block of the first attempt + 30
				excludedMembersIndexes: []group.MemberIndex{4, 5, 8, 10},
			},
			outgoingAnnouncementsCount: 1,
		},
		"missing announcements without honest majority on initial attempt": {
			signingGroupMemberIndex: 3,
			ctxFn: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 10*time.Second)
			},
			currentBlockFn: func() (uint64, error) {
				return 200, nil // same as the initial start block
			},
			incomingAnnouncementsFn: func(
				sessionID string,
			) ([]group.MemberIndex, error) {
				if sessionID == fmt.Sprintf("%v-%v", message, 1) {
					// Minority of members announced their readiness.
					return []group.MemberIndex{1, 2, 3, 6, 7}, nil
				}

				return signingGroupMembersIndexes, nil
			},
			signingAttemptFn: func(
				attempt *signingAttemptParams,
			) (*signing.Result, uint64, error) {
				return testResult, 260, nil // an arbitrary end block
			},
			waitUntilAllDoneOutcomeFn: func(attemptNumber uint64) (*signing.Result, uint64, error) {
				// Simulate that the done check phase determines the same
				// end block as the executing signer.
				return testResult, 260, nil
			},
			expectedOutgoingDoneChecks: []*signingDoneMessage{
				{
					senderID:      3,
					message:       message,
					attemptNumber: 2,
					signature:     testResult.Signature,
					endBlock:      260,
				},
			},
			expectedErr: nil,
			expectedResult: &signingRetryLoopResult{
				result: testResult,
				activityReport: &signingActivityReport{
					activeMembers:   signingGroupMembersIndexes,
					inactiveMembers: []group.MemberIndex{},
				},
				latestEndBlock:      260, // the end block resolved by the done check phase
				attemptTimeoutBlock: 277, // start block of the second attempt + 30
			},
			// Member 3 is the executing one. The first attempt's announcement
			// fails and the signing random retry algorithm invoked with the
			// test seed excludes 3 members (6 is the honest threshold) from the
			// second attempt: 1, 2 and 5. The additional exclusion round that
			// trims the included members list to the honest threshold size
			// adds member 9 to the final excluded members list.
			expectedLastExecutedAttempt: &signingAttemptParams{
				number:                 2,
				startBlock:             247, // 206 + 1 * (6 + 30 + 5)
				timeoutBlock:           277, // start block of the second attempt + 30
				excludedMembersIndexes: []group.MemberIndex{1, 2, 5, 9},
			},
			outgoingAnnouncementsCount: 2,
		},
		"announcement error on initial attempt": {
			signingGroupMemberIndex: 4,
			ctxFn: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 10*time.Second)
			},
			currentBlockFn: func() (uint64, error) {
				return 200, nil // same as the initial start block
			},
			incomingAnnouncementsFn: func(
				sessionID string,
			) ([]group.MemberIndex, error) {
				if sessionID == fmt.Sprintf("%v-%v", message, 1) {
					return nil, fmt.Errorf("unexpected error")
				}

				return signingGroupMembersIndexes, nil
			},
			signingAttemptFn: func(
				attempt *signingAttemptParams,
			) (*signing.Result, uint64, error) {
				return testResult, 260, nil // an arbitrary end block
			},
			waitUntilAllDoneOutcomeFn: func(attemptNumber uint64) (*signing.Result, uint64, error) {
				// Simulate that the done check phase determines the same
				// end block as the executing signer.
				return testResult, 260, nil
			},
			expectedOutgoingDoneChecks: []*signingDoneMessage{
				{
					senderID:      4,
					message:       message,
					attemptNumber: 2,
					signature:     testResult.Signature,
					endBlock:      260,
				},
			},
			expectedErr: nil,
			expectedResult: &signingRetryLoopResult{
				result: testResult,
				activityReport: &signingActivityReport{
					activeMembers:   signingGroupMembersIndexes,
					inactiveMembers: []group.MemberIndex{},
				},
				latestEndBlock:      260, // the end block resolved by the done check phase
				attemptTimeoutBlock: 277, // start block of the second attempt + 30
			},
			// Member 4 is the executing one. The first attempt fails and
			// the signing random retry algorithm invoked with the test seed
			// excludes 3 members (6 is the honest threshold) from the second
			// attempt: 1, 2 and 5. The additional exclusion round that trims
			// the included members list to the honest threshold size adds
			// member 9 to the final excluded members list.
			expectedLastExecutedAttempt: &signingAttemptParams{
				number:                 2,
				startBlock:             247, // 206 + 1 * (6 + 30 + 5)
				timeoutBlock:           277, // start block of the second attempt + 30
				excludedMembersIndexes: []group.MemberIndex{1, 2, 5, 9},
			},
			outgoingAnnouncementsCount: 2,
		},
		"signing error on initial attempt": {
			signingGroupMemberIndex: 4,
			ctxFn: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 10*time.Second)
			},
			currentBlockFn: func() (uint64, error) {
				return 200, nil // same as the initial start block
			},
			incomingAnnouncementsFn: func(
				sessionID string,
			) ([]group.MemberIndex, error) {
				return signingGroupMembersIndexes, nil
			},
			signingAttemptFn: func(
				attempt *signingAttemptParams,
			) (*signing.Result, uint64, error) {
				if attempt.number <= 1 {
					return nil, 0, fmt.Errorf("invalid data")
				}

				return testResult, 260, nil // an arbitrary end block
			},
			waitUntilAllDoneOutcomeFn: func(attemptNumber uint64) (*signing.Result, uint64, error) {
				// Simulate that the done check phase determines the same
				// end block as the executing signer.
				return testResult, 260, nil
			},
			expectedOutgoingDoneChecks: []*signingDoneMessage{
				{
					senderID:      4,
					message:       message,
					attemptNumber: 2,
					signature:     testResult.Signature,
					endBlock:      260,
				},
			},
			expectedErr: nil,
			expectedResult: &signingRetryLoopResult{
				result: testResult,
				activityReport: &signingActivityReport{
					activeMembers:   signingGroupMembersIndexes,
					inactiveMembers: []group.MemberIndex{},
				},
				latestEndBlock:      260, // the end block resolved by the done check phase
				attemptTimeoutBlock: 277, // start block of the second attempt + 30
			},
			// Member 4 is the executing one. The first attempt fails and
			// the signing random retry algorithm invoked with the test seed
			// excludes 3 members (6 is the honest threshold) from the second
			// attempt: 1, 2 and 5. The additional exclusion round that trims
			// the included members list to the honest threshold size adds
			// member 9 to the final excluded members list.
			expectedLastExecutedAttempt: &signingAttemptParams{
				number:                 2,
				startBlock:             247, // 206 + 1 * (6 + 30 + 5)
				timeoutBlock:           277, // start block of the second attempt + 30
				excludedMembersIndexes: []group.MemberIndex{1, 2, 5, 9},
			},
			outgoingAnnouncementsCount: 2,
		},
		"executing member excluded": {
			signingGroupMemberIndex: 2,
			ctxFn: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 10*time.Second)
			},
			currentBlockFn: func() (uint64, error) {
				return 200, nil // same as the initial start block
			},
			incomingAnnouncementsFn: func(
				sessionID string,
			) ([]group.MemberIndex, error) {
				return signingGroupMembersIndexes, nil
			},
			signingAttemptFn: func(
				attempt *signingAttemptParams,
			) (*signing.Result, uint64, error) {
				return nil, 0, fmt.Errorf("invalid data")
			},
			waitUntilAllDoneOutcomeFn: func(attemptNumber uint64) (*signing.Result, uint64, error) {
				// Simulate the result and the end block have been determined
				// by listening for signing done checks.
				if attemptNumber == 2 {
					return testResult, 260, nil
				}

				panic("undefined behavior")
			},
			expectedOutgoingDoneChecks: nil,
			expectedErr:                nil,
			expectedResult: &signingRetryLoopResult{
				result: testResult,
				activityReport: &signingActivityReport{
					activeMembers:   signingGroupMembersIndexes,
					inactiveMembers: []group.MemberIndex{},
				},
				latestEndBlock:      260, // the end block resolved by the done check phase
				attemptTimeoutBlock: 277, // start block of the second attempt + 30
			},
			// Member 2 is the executing one. The first attempt fails
			// and is the last attempt executed by this member because member
			// 2 is excluded from the second attempt that produced the signature.
			expectedLastExecutedAttempt: &signingAttemptParams{
				number:                 1,
				startBlock:             206,
				timeoutBlock:           236, // start block of the first attempt + 30
				excludedMembersIndexes: []group.MemberIndex{3, 7, 8, 10},
			},
			// The second announcement is done at the beginning of the
			// second attempt for which member 2 is eventually excluded.
			outgoingAnnouncementsCount: 2,
		},
		"done checks wait error": {
			signingGroupMemberIndex: 4,
			ctxFn: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 10*time.Second)
			},
			currentBlockFn: func() (uint64, error) {
				return 200, nil // same as the initial start block
			},
			incomingAnnouncementsFn: func(
				sessionID string,
			) ([]group.MemberIndex, error) {
				return signingGroupMembersIndexes, nil
			},
			signingAttemptFn: func(
				attempt *signingAttemptParams,
			) (*signing.Result, uint64, error) {
				if attempt.number == 1 {
					return testResult, 215, nil // an arbitrary end block
				}

				if attempt.number == 2 {
					return testResult, 260, nil // an arbitrary end block
				}

				panic("undefined behavior")
			},
			waitUntilAllDoneOutcomeFn: func(attemptNumber uint64) (*signing.Result, uint64, error) {
				// Fail the done check for the first attempt.
				if attemptNumber == 1 {
					return nil, 0, fmt.Errorf("network error")
				}

				// Simulate that the done check phase determines the same
				// end block as the executing signer.
				return testResult, 260, nil
			},
			expectedOutgoingDoneChecks: []*signingDoneMessage{
				{
					senderID:      4,
					message:       message,
					attemptNumber: 1,
					signature:     testResult.Signature,
					endBlock:      215,
				},
				{
					senderID:      4,
					message:       message,
					attemptNumber: 2,
					signature:     testResult.Signature,
					endBlock:      260,
				},
			},
			expectedErr: nil,
			expectedResult: &signingRetryLoopResult{
				result: testResult,
				activityReport: &signingActivityReport{
					activeMembers:   signingGroupMembersIndexes,
					inactiveMembers: []group.MemberIndex{},
				},
				latestEndBlock:      260, // the end block resolved by the done check phase
				attemptTimeoutBlock: 277, // start block of the second attempt + 30
			},
			// Member 4 is the executing one. The first attempt done check
			// exchange fails and the signing random retry algorithm invoked
			// with the test seed excludes 3 members (6 is the honest threshold)
			// from the second attempt: 1, 2 and 5. The additional exclusion
			// round that trims the included members list to the honest
			// threshold size adds member 9 to the final excluded members list.
			expectedLastExecutedAttempt: &signingAttemptParams{
				number:                 2,
				startBlock:             247, // 206 + 1 * (6 + 30 + 5)
				timeoutBlock:           277, // start block of the second attempt + 30
				excludedMembersIndexes: []group.MemberIndex{1, 2, 5, 9},
			},
			outgoingAnnouncementsCount: 2,
		},
		"loop context done": {
			signingGroupMemberIndex: 1,
			ctxFn: func() (context.Context, context.CancelFunc) {
				ctx, cancelCtx := context.WithCancel(context.Background())
				// Cancel the context deliberately.
				cancelCtx()
				return ctx, cancelCtx
			},
			currentBlockFn: func() (uint64, error) {
				return 200, nil // same as the initial start block
			},
			incomingAnnouncementsFn: func(
				sessionID string,
			) ([]group.MemberIndex, error) {
				return signingGroupMembersIndexes, nil
			},
			signingAttemptFn: func(
				attempt *signingAttemptParams,
			) (*signing.Result, uint64, error) {
				return nil, 0, fmt.Errorf("invalid data")
			},
			expectedErr:                 context.Canceled,
			expectedResult:              nil,
			expectedLastExecutedAttempt: nil,
		},
		"signing in the past": {
			signingGroupMemberIndex: 1,
			ctxFn: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 50*time.Millisecond)
			},
			currentBlockFn: func() (uint64, error) {
				return math.MaxUint64, nil // all attempts should be skipped
			},
			incomingAnnouncementsFn: func(
				sessionID string,
			) ([]group.MemberIndex, error) {
				return signingGroupMembersIndexes, nil
			},
			signingAttemptFn: func(
				attempt *signingAttemptParams,
			) (*signing.Result, uint64, error) {
				return nil, 0, fmt.Errorf("invalid data")
			},
			// The retry loop keeps skipping all attempts because they are all
			// ending announcement phase in the past block. It keeps retrying
			// until the context deadline is exceeded.
			expectedErr:                 context.DeadlineExceeded,
			expectedResult:              nil,
			expectedLastExecutedAttempt: nil,
		},
		"first attempt in the past": {
			signingGroupMemberIndex: 3,
			ctxFn: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 1*time.Second)
			},
			currentBlockFn: func() (uint64, error) {
				// The initial start block is 200 and the announcement takes 6
				// blocks; we are at the end of the announcement phase so the
				// first attempt should be skipped.
				return 206, nil
			},
			incomingAnnouncementsFn: func(
				sessionID string,
			) ([]group.MemberIndex, error) {
				return signingGroupMembersIndexes, nil
			},
			signingAttemptFn: func(
				attempt *signingAttemptParams,
			) (*signing.Result, uint64, error) {
				return testResult, 260, nil // an arbitrary end block
			},
			waitUntilAllDoneOutcomeFn: func(attemptNumber uint64) (*signing.Result, uint64, error) {
				// Simulate that the done check phase determines the same
				// end block as the executing signer.
				return testResult, 260, nil
			},
			expectedOutgoingDoneChecks: []*signingDoneMessage{
				{
					senderID:      3,
					message:       message,
					attemptNumber: 2,
					signature:     testResult.Signature,
					endBlock:      260,
				},
			},
			expectedErr: nil,
			expectedResult: &signingRetryLoopResult{
				result: testResult,
				activityReport: &signingActivityReport{
					activeMembers:   signingGroupMembersIndexes,
					inactiveMembers: []group.MemberIndex{},
				},
				latestEndBlock:      260, // the end block resolved by the done check phase
				attemptTimeoutBlock: 277, // start block of the second attempt + 30
			},
			// Member 3 is the executing one. The first attempt's announcement
			// is skipped and the signing random retry algorithm invoked with the
			// test seed excludes 3 members (6 is the honest threshold) from the
			// second attempt: 1, 2 and 5. The additional exclusion round that
			// trims the included members list to the honest threshold size
			// adds member 9 to the final excluded members list.
			expectedLastExecutedAttempt: &signingAttemptParams{
				number:                 2,
				startBlock:             247, // 206 + 1 * (6 + 30 + 5)
				timeoutBlock:           277, // start block of the second attempt + 30
				excludedMembersIndexes: []group.MemberIndex{1, 2, 5, 9},
			},
			// just the second announcement, the first one was skipped
			outgoingAnnouncementsCount: 1,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			announcer := &mockSigningAnnouncer{
				outgoingAnnouncements:   make(map[string]group.MemberIndex),
				incomingAnnouncementsFn: test.incomingAnnouncementsFn,
			}

			doneCheck := &mockSigningDoneCheck{
				waitUntilAllDoneOutcomeFn: test.waitUntilAllDoneOutcomeFn,
			}

			retryLoop := newSigningRetryLoop(
				&testutils.MockLogger{},
				message,
				200,
				test.signingGroupMemberIndex,
				signingGroupOperators,
				groupParameters,
				announcer,
				doneCheck,
			)

			ctx, cancelCtx := test.ctxFn()
			defer cancelCtx()

			var lastExecutedAttempt *signingAttemptParams

			result, err := retryLoop.start(
				ctx,
				func(context.Context, uint64) error {
					return nil
				},
				test.currentBlockFn,
				func(params *signingAttemptParams) (*signing.Result, uint64, error) {
					lastExecutedAttempt = params
					return test.signingAttemptFn(params)
				},
			)

			if !reflect.DeepEqual(test.expectedErr, err) {
				t.Errorf(
					"unexpected error\n"+
						"expected: [%+v]\n"+
						"actual:   [%+v]",
					test.expectedErr,
					err,
				)
			}

			if !reflect.DeepEqual(test.expectedResult, result) {
				t.Errorf(
					"unexpected result\n"+
						"expected: [%+v]\n"+
						"actual:   [%+v]",
					test.expectedResult,
					result,
				)
			}

			if !reflect.DeepEqual(test.expectedLastExecutedAttempt, lastExecutedAttempt) {
				t.Errorf(
					"unexpected last executed attempt\n"+
						"expected: [%+v]\n"+
						"actual:   [%+v]",
					test.expectedLastExecutedAttempt,
					lastExecutedAttempt,
				)
			}

			if test.expectedLastExecutedAttempt != nil {
				testutils.AssertIntsEqual(
					t,
					"outgoing announcements count",
					int(test.outgoingAnnouncementsCount),
					len(announcer.outgoingAnnouncements),
				)

				for sessionID, memberIndex := range announcer.outgoingAnnouncements {
					testutils.AssertIntsEqual(
						t,
						fmt.Sprintf(
							"outgoing announcement's member index "+
								"for session [%v]",
							sessionID,
						),
						int(test.signingGroupMemberIndex),
						int(memberIndex),
					)
				}
			}

			if !reflect.DeepEqual(
				test.expectedOutgoingDoneChecks,
				doneCheck.outgoingDoneChecks,
			) {
				t.Errorf(
					"unexpected outgoing done checks\n"+
						"expected: [%v]\n"+
						"actual:   [%v]",
					test.expectedOutgoingDoneChecks,
					doneCheck.outgoingDoneChecks,
				)
			}
		})
	}
}

func TestSigningRetryLoop_GetCurrentBlockErrorCausesRetry(t *testing.T) {
	message := big.NewInt(100)

	groupParameters := &GroupParameters{
		GroupSize:       10,
		HonestThreshold: 6,
	}

	signingGroupOperators := chain.Addresses{
		"address-1", "address-2", "address-8", "address-4",
		"address-2", "address-6", "address-7", "address-8",
		"address-9", "address-8",
	}

	retryLoop := newSigningRetryLoop(
		&testutils.MockLogger{},
		message,
		200,
		1,
		signingGroupOperators,
		groupParameters,
		&mockSigningAnnouncer{
			outgoingAnnouncements: make(map[string]group.MemberIndex),
			incomingAnnouncementsFn: func(string) ([]group.MemberIndex, error) {
				panic("should not be reached: announcer invoked when getCurrentBlock always errors")
			},
		},
		&mockSigningDoneCheck{
			waitUntilAllDoneOutcomeFn: func(uint64) (*signing.Result, uint64, error) {
				panic("should not be reached")
			},
		},
	)

	ctx, cancelCtx := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancelCtx()

	_, err := retryLoop.start(
		ctx,
		func(context.Context, uint64) error { return nil },
		func() (uint64, error) { return 0, fmt.Errorf("rpc unavailable") },
		func(*signingAttemptParams) (*signing.Result, uint64, error) {
			panic("should not be reached: signing invoked when getCurrentBlock always errors")
		},
	)

	if err != context.DeadlineExceeded {
		t.Errorf(
			"unexpected error\nexpected: [%v]\nactual:   [%v]",
			context.DeadlineExceeded,
			err,
		)
	}
}

func TestSigningRetryLoop_WaitForBlockErrorCausesRetry(t *testing.T) {
	message := big.NewInt(100)

	groupParameters := &GroupParameters{
		GroupSize:       10,
		HonestThreshold: 6,
	}

	signingGroupOperators := chain.Addresses{
		"address-1", "address-2", "address-8", "address-4",
		"address-2", "address-6", "address-7", "address-8",
		"address-9", "address-8",
	}

	retryLoop := newSigningRetryLoop(
		&testutils.MockLogger{},
		message,
		200,
		1,
		signingGroupOperators,
		groupParameters,
		&mockSigningAnnouncer{
			outgoingAnnouncements: make(map[string]group.MemberIndex),
			incomingAnnouncementsFn: func(string) ([]group.MemberIndex, error) {
				panic("should not be reached: announcer invoked when waitForBlock always errors")
			},
		},
		&mockSigningDoneCheck{
			waitUntilAllDoneOutcomeFn: func(uint64) (*signing.Result, uint64, error) {
				panic("should not be reached")
			},
		},
	)

	ctx, cancelCtx := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancelCtx()

	_, err := retryLoop.start(
		ctx,
		func(context.Context, uint64) error { return fmt.Errorf("rpc timeout") },
		func() (uint64, error) { return 200, nil }, // behind announcementEndBlock so attempt is not skipped
		func(*signingAttemptParams) (*signing.Result, uint64, error) {
			panic("should not be reached: signing invoked when waitForBlock always errors")
		},
	)

	if err != context.DeadlineExceeded {
		t.Errorf(
			"unexpected error\nexpected: [%v]\nactual:   [%v]",
			context.DeadlineExceeded,
			err,
		)
	}
}

func TestSigningRetryLoop_ContextCancelled(t *testing.T) {
	groupParameters := &GroupParameters{
		GroupSize:       10,
		HonestThreshold: 6,
	}

	signingGroupOperators := chain.Addresses{
		"address-1", "address-2", "address-8", "address-4",
		"address-2", "address-6", "address-7", "address-8",
		"address-9", "address-8",
	}

	retryLoop := newSigningRetryLoop(
		&testutils.MockLogger{},
		big.NewInt(100),
		200,
		1,
		signingGroupOperators,
		groupParameters,
		&mockSigningAnnouncer{
			outgoingAnnouncements: make(map[string]group.MemberIndex),
			incomingAnnouncementsFn: func(string) ([]group.MemberIndex, error) {
				panic("should not be reached: context already cancelled")
			},
		},
		&mockSigningDoneCheck{
			waitUntilAllDoneOutcomeFn: func(uint64) (*signing.Result, uint64, error) {
				panic("should not be reached")
			},
		},
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before start -- the loop should exit at the first ctx.Err() check

	_, err := retryLoop.start(
		ctx,
		func(context.Context, uint64) error { return nil },
		func() (uint64, error) { return 200, nil },
		func(*signingAttemptParams) (*signing.Result, uint64, error) {
			panic("should not be reached")
		},
	)

	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}

// TestSigningRetryLoop_SuccessAfterRetry verifies that the retry loop
// recovers when the announcer returns an error on the first attempt and
// succeeds on the second -- the retry path must actually produce a result.
func TestSigningRetryLoop_SuccessAfterRetry(t *testing.T) {
	message := big.NewInt(100)

	groupParameters := &GroupParameters{
		GroupSize:       10,
		HonestThreshold: 6,
	}

	signingGroupOperators := chain.Addresses{
		"address-1", "address-2", "address-8", "address-4",
		"address-2", "address-6", "address-7", "address-8",
		"address-9", "address-8",
	}

	testResult := &signing.Result{
		Signature: mustFrostSignatureFromBigInts(big.NewInt(300), big.NewInt(400)),
	}

	// Session IDs use fmt.Sprintf("%v-%v", message, attemptCounter).
	firstAttemptSession := fmt.Sprintf("%v-%v", message, 1)

	retryLoop := newSigningRetryLoop(
		&testutils.MockLogger{},
		message,
		200,
		1,
		signingGroupOperators,
		groupParameters,
		&mockSigningAnnouncer{
			outgoingAnnouncements: make(map[string]group.MemberIndex),
			incomingAnnouncementsFn: func(sessionID string) ([]group.MemberIndex, error) {
				if sessionID == firstAttemptSession {
					return nil, fmt.Errorf("announcer unavailable on first attempt")
				}
				return []group.MemberIndex{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, nil
			},
		},
		&mockSigningDoneCheck{
			waitUntilAllDoneOutcomeFn: func(uint64) (*signing.Result, uint64, error) {
				return testResult, 215, nil
			},
		},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := retryLoop.start(
		ctx,
		func(context.Context, uint64) error { return nil },
		func() (uint64, error) { return 200, nil },
		func(*signingAttemptParams) (*signing.Result, uint64, error) {
			return testResult, 215, nil
		},
	)

	if err != nil {
		t.Fatalf("expected no error after retry, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.result == nil || !result.result.Signature.Equals(testResult.Signature) {
		t.Errorf("unexpected result signature: %v", result)
	}
}

type mockSigningAnnouncer struct {
	// outgoingAnnouncements holds all announcements that are sent by the
	// announcer.
	outgoingAnnouncements map[string]group.MemberIndex

	// incomingAnnouncementsFn returns all announcements that are received
	// by the announcer for the given attempt.
	incomingAnnouncementsFn func(
		sessionID string,
	) ([]group.MemberIndex, error)
}

func (msa *mockSigningAnnouncer) Announce(
	ctx context.Context,
	memberIndex group.MemberIndex,
	sessionID string,
) ([]group.MemberIndex, error) {
	msa.outgoingAnnouncements[sessionID] = memberIndex

	return msa.incomingAnnouncementsFn(sessionID)
}

type mockSigningDoneCheck struct {
	outgoingDoneChecks        []*signingDoneMessage
	currentAttemptNumber      uint64
	waitUntilAllDoneOutcomeFn func(attemptNumber uint64) (*signing.Result, uint64, error)
}

func (msdc *mockSigningDoneCheck) listen(
	ctx context.Context,
	message *big.Int,
	attemptNumber uint64,
	attemptTimeoutBlock uint64,
	attemptMembersIndexes []group.MemberIndex,
) {
	msdc.currentAttemptNumber = attemptNumber
}

func (msdc *mockSigningDoneCheck) signalDone(
	ctx context.Context,
	memberIndex group.MemberIndex,
	message *big.Int,
	attemptNumber uint64,
	result *signing.Result,
	endBlock uint64,
) error {
	msdc.outgoingDoneChecks = append(msdc.outgoingDoneChecks, &signingDoneMessage{
		senderID:      memberIndex,
		message:       message,
		attemptNumber: attemptNumber,
		signature:     result.Signature,
		endBlock:      endBlock,
	})

	return nil
}

func (msdc *mockSigningDoneCheck) waitUntilAllDone(ctx context.Context) (*signing.Result, uint64, error) {
	return msdc.waitUntilAllDoneOutcomeFn(msdc.currentAttemptNumber)
}
