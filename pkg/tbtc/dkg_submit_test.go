package tbtc

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/chain"

	"golang.org/x/crypto/sha3"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/internal/tecdsatest"
	"github.com/keep-network/keep-core/pkg/protocol/group"
	"github.com/keep-network/keep-core/pkg/tecdsa"
	"github.com/keep-network/keep-core/pkg/tecdsa/dkg"
)

func TestSignResult_SigningSuccessful(t *testing.T) {
	chain := Connect()
	dkgStartBlock := uint64(2000)
	dkgResultSigner := newDkgResultSigner(chain, dkgStartBlock)

	testData, err := tecdsatest.LoadPrivateKeyShareTestFixtures(1)
	if err != nil {
		t.Fatalf("failed to load test data: [%v]", err)
	}
	result := &dkg.Result{
		Group:           group.NewGroup(2, 5),
		PrivateKeyShare: tecdsa.NewPrivateKeyShare(testData[0]),
	}

	signedResult, err := dkgResultSigner.SignResult(result)
	if err != nil {
		t.Fatal(err)
	}

	expectedPublicKey := chain.Signing().PublicKey()
	if !reflect.DeepEqual(
		expectedPublicKey,
		signedResult.PublicKey,
	) {
		t.Errorf(
			"unexpected public key\n"+
				"expected: %v\n"+
				"actual:   %v\n",
			expectedPublicKey,
			signedResult.PublicKey,
		)
	}

	groupPublicKey, err := result.GroupPublicKey()
	if err != nil {
		t.Fatal(err)
	}

	expectedDKGResultHash := dkg.ResultSignatureHash(
		sha3.Sum256(
			[]byte(fmt.Sprint(
				groupPublicKey,
				result.MisbehavedMembersIndexes(),
				dkgStartBlock,
			)),
		),
	)
	if expectedDKGResultHash != signedResult.ResultHash {
		t.Errorf(
			"unexpected result hash\n"+
				"expected: %v\n"+
				"actual:   %v\n",
			expectedDKGResultHash,
			signedResult.ResultHash,
		)
	}

	// Since signature is different on every run (even if the same private key
	// and result hash are used), simply verify if it's correct
	signatureVerification, err := chain.Signing().Verify(
		signedResult.ResultHash[:],
		signedResult.Signature,
	)
	if err != nil {
		t.Fatal(err)
	}

	if !signatureVerification {
		t.Errorf(
			"Signature [0x%x] was not generated properly for the result hash "+
				"[0x%x]",
			signedResult.Signature,
			signedResult.ResultHash,
		)
	}
}

func TestSignResult_ErrorDuringDkgResultHashCalculation(t *testing.T) {
	chain := Connect()
	dkgStartBlock := uint64(2000)
	dkgResultSigner := newDkgResultSigner(chain, dkgStartBlock)

	// Use nil as the DKG result to cause hash calculation error
	_, err := dkgResultSigner.SignResult(nil)

	expectedError := fmt.Errorf("result is nil")
	if !reflect.DeepEqual(expectedError, err) {
		t.Errorf(
			"unexpected error\nexpected: %v\nactual:   %v\n",
			expectedError,
			err,
		)
	}
}

func TestVerifySignature_VerificationSuccessful(t *testing.T) {
	chain := Connect()
	dkgStartBlock := uint64(2000)
	dkgResultSigner := newDkgResultSigner(chain, dkgStartBlock)

	testData, err := tecdsatest.LoadPrivateKeyShareTestFixtures(1)
	if err != nil {
		t.Fatalf("failed to load test data: [%v]", err)
	}
	result := &dkg.Result{
		Group:           group.NewGroup(2, 5),
		PrivateKeyShare: tecdsa.NewPrivateKeyShare(testData[0]),
	}

	signedResult, err := dkgResultSigner.SignResult(result)
	if err != nil {
		t.Fatal(err)
	}

	verificationSuccessful, err := dkgResultSigner.VerifySignature(signedResult)
	if err != nil {
		t.Fatal(err)
	}

	if !verificationSuccessful {
		t.Fatal(
			"Expected successful verification of signature, but it was " +
				"unsuccessful",
		)
	}
}

func TestVerifySignature_VerificationFailure(t *testing.T) {
	chain := Connect()
	dkgStartBlock := uint64(2000)
	dkgResultSigner := newDkgResultSigner(chain, dkgStartBlock)

	testData, err := tecdsatest.LoadPrivateKeyShareTestFixtures(1)
	if err != nil {
		t.Fatalf("failed to load test data: [%v]", err)
	}

	result := &dkg.Result{
		Group:           group.NewGroup(2, 5),
		PrivateKeyShare: tecdsa.NewPrivateKeyShare(testData[0]),
	}
	signedResult, err := dkgResultSigner.SignResult(result)
	if err != nil {
		t.Fatal(err)
	}

	anotherResult := &dkg.Result{
		Group:           group.NewGroup(2, 5),
		PrivateKeyShare: tecdsa.NewPrivateKeyShare(testData[0]),
	}
	anotherResult.Group.MarkMemberAsInactive(3)
	anotherSignedResult, err := dkgResultSigner.SignResult(anotherResult)
	if err != nil {
		t.Fatal(err)
	}

	// Assign signature from another result to cause a signature verification
	// failure
	signedResult.Signature = anotherSignedResult.Signature

	verificationSuccessful, err := dkgResultSigner.VerifySignature(signedResult)
	if err != nil {
		t.Fatal(err)
	}

	if verificationSuccessful {
		t.Errorf(
			"expected unsuccessful verification of signature, " +
				"but it was successful",
		)
	}
}

func TestVerifySignature_VerificationError(t *testing.T) {
	chain := Connect()
	dkgStartBlock := uint64(2000)
	dkgResultSigner := newDkgResultSigner(chain, dkgStartBlock)

	testData, err := tecdsatest.LoadPrivateKeyShareTestFixtures(1)
	if err != nil {
		t.Fatalf("failed to load test data: [%v]", err)
	}

	result := &dkg.Result{
		Group:           group.NewGroup(2, 5),
		PrivateKeyShare: tecdsa.NewPrivateKeyShare(testData[0]),
	}
	signedResult, err := dkgResultSigner.SignResult(result)
	if err != nil {
		t.Fatal(err)
	}

	// Drop the last byte of the signature to cause an error during signature
	// verification
	signedResult.Signature = signedResult.Signature[:len(signedResult.Signature)-1]

	_, err = dkgResultSigner.VerifySignature(signedResult)

	expectedError := fmt.Errorf(
		"failed to unmarshal signature: [asn1: syntax error: data truncated]",
	)
	if !reflect.DeepEqual(expectedError, err) {
		t.Errorf(
			"unexpected error\n"+
				"expected: [%+v]\n"+
				"actual:   [%+v]",
			expectedError,
			err,
		)
	}
}

func TestSubmitResult_MemberSubmitsResult(t *testing.T) {
	groupParameters := &GroupParameters{
		GroupSize:       5,
		GroupQuorum:     4,
		HonestThreshold: 3,
	}

	localChain := Connect()

	err := localChain.startDKG()
	if err != nil {
		t.Fatal(err)
	}

	operatorAddress, err := localChain.operatorAddress()
	if err != nil {
		t.Fatal(err)
	}

	operatorID, err := localChain.GetOperatorID(operatorAddress)
	if err != nil {
		t.Fatal(err)
	}

	var operatorsIDs chain.OperatorIDs
	var operatorsAddresses chain.Addresses

	for memberIndex := uint8(1); int(memberIndex) <= groupParameters.GroupSize; memberIndex++ {
		operatorsIDs = append(operatorsIDs, operatorID)
		operatorsAddresses = append(operatorsAddresses, operatorAddress)
	}

	groupSelectionResult := &GroupSelectionResult{
		OperatorsIDs:       operatorsIDs,
		OperatorsAddresses: operatorsAddresses,
	}

	dkgResultSubmitter := newDkgResultSubmitter(
		&testutils.MockLogger{},
		localChain,
		groupParameters,
		groupSelectionResult,
		testWaitForBlockFn(localChain),
	)

	testData, err := tecdsatest.LoadPrivateKeyShareTestFixtures(1)
	if err != nil {
		t.Fatalf("failed to load test data: [%v]", err)
	}
	result := &dkg.Result{
		Group:           group.NewGroup(groupParameters.DishonestThreshold(), groupParameters.GroupSize),
		PrivateKeyShare: tecdsa.NewPrivateKeyShare(testData[0]),
	}

	memberIndex := group.MemberIndex(1)
	signatures := map[group.MemberIndex][]byte{
		1: []byte("signature 1"),
		2: []byte("signature 2"),
		3: []byte("signature 3"),
		4: []byte("signature 4"),
	}

	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	if err = localChain.setDKGResultValidity(true); err != nil {
		t.Fatal(err)
	}

	err = dkgResultSubmitter.SubmitResult(
		ctx,
		memberIndex,
		result,
		signatures,
	)
	if err != nil {
		t.Fatal(err)
	}

	expectedGroupPublicKey, err := result.GroupPublicKeyBytes()
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(expectedGroupPublicKey, localChain.dkgResult.GroupPublicKey) {
		t.Errorf(
			"unexpected group public key \nexpected: [0x%x]\nactual:   [0x%x]\n",
			expectedGroupPublicKey,
			localChain.dkgResult.GroupPublicKey,
		)
	}
}

func TestSubmitResult_AnotherMemberSubmitsResult(t *testing.T) {
	groupParameters := &GroupParameters{
		GroupSize:       5,
		GroupQuorum:     4,
		HonestThreshold: 3,
	}

	localChain := Connect()

	err := localChain.startDKG()
	if err != nil {
		t.Fatal(err)
	}

	operatorAddress, err := localChain.operatorAddress()
	if err != nil {
		t.Fatal(err)
	}

	operatorID, err := localChain.GetOperatorID(operatorAddress)
	if err != nil {
		t.Fatal(err)
	}

	var operatorsIDs chain.OperatorIDs
	var operatorsAddresses chain.Addresses

	for memberIndex := uint8(1); int(memberIndex) <= groupParameters.GroupSize; memberIndex++ {
		operatorsIDs = append(operatorsIDs, operatorID)
		operatorsAddresses = append(operatorsAddresses, operatorAddress)
	}

	groupSelectionResult := &GroupSelectionResult{
		OperatorsIDs:       operatorsIDs,
		OperatorsAddresses: operatorsAddresses,
	}

	dkgResultSubmitter := newDkgResultSubmitter(
		&testutils.MockLogger{},
		localChain,
		groupParameters,
		groupSelectionResult,
		testWaitForBlockFn(localChain),
	)

	testData, err := tecdsatest.LoadPrivateKeyShareTestFixtures(1)
	if err != nil {
		t.Fatalf("failed to load test data: [%v]", err)
	}
	result := &dkg.Result{
		Group:           group.NewGroup(groupParameters.DishonestThreshold(), groupParameters.GroupSize),
		PrivateKeyShare: tecdsa.NewPrivateKeyShare(testData[0]),
	}
	signatures := map[group.MemberIndex][]byte{
		1: []byte("signature 1"),
		2: []byte("signature 2"),
		3: []byte("signature 3"),
		4: []byte("signature 4"),
	}

	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	// Set up a global listener that will cancel the common context upon result
	// submission. That mimics the real-world scenario.
	localChain.OnDKGResultSubmitted(
		func(event *DKGResultSubmittedEvent) {
			cancelCtx()
		})

	if err = localChain.setDKGResultValidity(true); err != nil {
		t.Fatal(err)
	}

	secondMemberSubmissionChannel := make(chan error)

	// Attempt to submit result for the second member on a separate goroutine.
	go func() {
		secondMemberIndex := group.MemberIndex(2)
		secondMemberErr := dkgResultSubmitter.SubmitResult(
			ctx,
			secondMemberIndex,
			result,
			signatures,
		)
		secondMemberSubmissionChannel <- secondMemberErr
	}()

	// This sleep is needed to give enough time for the second member to
	// register their result submission event handler and act properly on
	// the result submitted by the first member.
	time.Sleep(1 * time.Second)

	// While the second member is waiting for submission eligibility, submit the
	// result with the first member.
	firstMemberIndex := group.MemberIndex(1)
	firstMemberErr := dkgResultSubmitter.SubmitResult(
		ctx,
		firstMemberIndex,
		result,
		signatures,
	)
	if firstMemberErr != nil {
		t.Fatal(firstMemberErr)
	}

	// Check that the second member returned without errors
	secondMemberErr := <-secondMemberSubmissionChannel
	if secondMemberErr != nil {
		t.Fatal(secondMemberErr)
	}

	if localChain.dkgResult.SubmitterMemberIndex != firstMemberIndex {
		t.Errorf(
			"unexpected result submitter index \nexpected: %v\nactual:   %v\n",
			firstMemberIndex,
			localChain.dkgResult.SubmitterMemberIndex,
		)
	}

	expectedGroupPublicKey, err := result.GroupPublicKeyBytes()
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(expectedGroupPublicKey, localChain.dkgResult.GroupPublicKey) {
		t.Errorf(
			"unexpected group public key \nexpected: [0x%x]\nactual:   [0x%x]\n",
			expectedGroupPublicKey,
			localChain.dkgResult.GroupPublicKey,
		)
	}
}

func TestSubmitResult_InvalidResult(t *testing.T) {
	groupParameters := &GroupParameters{
		GroupSize:       5,
		GroupQuorum:     4,
		HonestThreshold: 3,
	}

	localChain := Connect()

	err := localChain.startDKG()
	if err != nil {
		t.Fatal(err)
	}

	operatorAddress, err := localChain.operatorAddress()
	if err != nil {
		t.Fatal(err)
	}

	operatorID, err := localChain.GetOperatorID(operatorAddress)
	if err != nil {
		t.Fatal(err)
	}

	var operatorsIDs chain.OperatorIDs
	var operatorsAddresses chain.Addresses

	for memberIndex := uint8(1); int(memberIndex) <= groupParameters.GroupSize; memberIndex++ {
		operatorsIDs = append(operatorsIDs, operatorID)
		operatorsAddresses = append(operatorsAddresses, operatorAddress)
	}

	groupSelectionResult := &GroupSelectionResult{
		OperatorsIDs:       operatorsIDs,
		OperatorsAddresses: operatorsAddresses,
	}

	dkgResultSubmitter := newDkgResultSubmitter(
		&testutils.MockLogger{},
		localChain,
		groupParameters,
		groupSelectionResult,
		testWaitForBlockFn(localChain),
	)

	testData, err := tecdsatest.LoadPrivateKeyShareTestFixtures(1)
	if err != nil {
		t.Fatalf("failed to load test data: [%v]", err)
	}
	result := &dkg.Result{
		Group:           group.NewGroup(groupParameters.DishonestThreshold(), groupParameters.GroupSize),
		PrivateKeyShare: tecdsa.NewPrivateKeyShare(testData[0]),
	}

	memberIndex := group.MemberIndex(1)
	signatures := map[group.MemberIndex][]byte{
		1: []byte("signature 1"),
		2: []byte("signature 2"),
		3: []byte("signature 3"),
		4: []byte("signature 4"),
	}

	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	if err = localChain.setDKGResultValidity(false); err != nil {
		t.Fatal(err)
	}

	err = dkgResultSubmitter.SubmitResult(
		ctx,
		memberIndex,
		result,
		signatures,
	)

	expectedErr := fmt.Errorf("invalid DKG result")
	if !reflect.DeepEqual(expectedErr, err) {
		t.Errorf(
			"unexpected error \nexpected: [%v]\nactual:   [%v]\n",
			expectedErr,
			err,
		)
	}
}

func TestSubmitResult_ContextCancelled(t *testing.T) {
	groupParameters := &GroupParameters{
		GroupSize:       5,
		GroupQuorum:     4,
		HonestThreshold: 3,
	}

	localChain := Connect()

	err := localChain.startDKG()
	if err != nil {
		t.Fatal(err)
	}

	operatorAddress, err := localChain.operatorAddress()
	if err != nil {
		t.Fatal(err)
	}

	operatorID, err := localChain.GetOperatorID(operatorAddress)
	if err != nil {
		t.Fatal(err)
	}

	var operatorsIDs chain.OperatorIDs
	var operatorsAddresses chain.Addresses

	for memberIndex := uint8(1); int(memberIndex) <= groupParameters.GroupSize; memberIndex++ {
		operatorsIDs = append(operatorsIDs, operatorID)
		operatorsAddresses = append(operatorsAddresses, operatorAddress)
	}

	groupSelectionResult := &GroupSelectionResult{
		OperatorsIDs:       operatorsIDs,
		OperatorsAddresses: operatorsAddresses,
	}

	dkgResultSubmitter := newDkgResultSubmitter(
		&testutils.MockLogger{},
		localChain,
		groupParameters,
		groupSelectionResult,
		testWaitForBlockFn(localChain),
	)

	testData, err := tecdsatest.LoadPrivateKeyShareTestFixtures(1)
	if err != nil {
		t.Fatalf("failed to load test data: [%v]", err)
	}
	result := &dkg.Result{
		Group:           group.NewGroup(groupParameters.DishonestThreshold(), groupParameters.GroupSize),
		PrivateKeyShare: tecdsa.NewPrivateKeyShare(testData[0]),
	}

	memberIndex := group.MemberIndex(1)
	signatures := map[group.MemberIndex][]byte{
		1: []byte("signature 1"),
		2: []byte("signature 2"),
		3: []byte("signature 3"),
		4: []byte("signature 4"),
	}

	ctx, cancelCtx := context.WithCancel(context.Background())

	// Simulate the case when timeout occurs and the context gets cancelled.
	cancelCtx()

	if err = localChain.setDKGResultValidity(true); err != nil {
		t.Fatal(err)
	}

	err = dkgResultSubmitter.SubmitResult(
		ctx,
		memberIndex,
		result,
		signatures,
	)
	if err != nil {
		t.Errorf("unexpected error [%v]", err)
	}
}

func TestSubmitResult_TooFewSignatures(t *testing.T) {
	groupParameters := &GroupParameters{
		GroupSize:       5,
		GroupQuorum:     4,
		HonestThreshold: 3,
	}

	localChain := Connect()

	err := localChain.startDKG()
	if err != nil {
		t.Fatal(err)
	}

	operatorAddress, err := localChain.operatorAddress()
	if err != nil {
		t.Fatal(err)
	}

	operatorID, err := localChain.GetOperatorID(operatorAddress)
	if err != nil {
		t.Fatal(err)
	}

	var operatorsIDs chain.OperatorIDs
	var operatorsAddresses chain.Addresses

	for memberIndex := uint8(1); int(memberIndex) <= groupParameters.GroupSize; memberIndex++ {
		operatorsIDs = append(operatorsIDs, operatorID)
		operatorsAddresses = append(operatorsAddresses, operatorAddress)
	}

	groupSelectionResult := &GroupSelectionResult{
		OperatorsIDs:       operatorsIDs,
		OperatorsAddresses: operatorsAddresses,
	}

	dkgResultSubmitter := newDkgResultSubmitter(
		&testutils.MockLogger{},
		localChain,
		groupParameters,
		groupSelectionResult,
		testWaitForBlockFn(localChain),
	)

	testData, err := tecdsatest.LoadPrivateKeyShareTestFixtures(1)
	if err != nil {
		t.Fatalf("failed to load test data: [%v]", err)
	}
	result := &dkg.Result{
		Group:           group.NewGroup(groupParameters.DishonestThreshold(), groupParameters.GroupSize),
		PrivateKeyShare: tecdsa.NewPrivateKeyShare(testData[0]),
	}

	memberIndex := group.MemberIndex(1)
	signatures := map[group.MemberIndex][]byte{
		1: []byte("signature 1"),
		2: []byte("signature 2"),
		3: []byte("signature 3"),
	}

	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	err = dkgResultSubmitter.SubmitResult(
		ctx,
		memberIndex,
		result,
		signatures,
	)

	expectedError := fmt.Errorf(
		"could not submit result with [3] signatures for group quorum [4]",
	)
	if !reflect.DeepEqual(expectedError, err) {
		t.Errorf(
			"unexpected error\n"+
				"expected: [%+v]\n"+
				"actual:   [%+v]",
			expectedError,
			err,
		)
	}
}

// TestSubmitResult_StateChangesDuringWait is a regression test for a TOCTOU
// race between the initial DKG-state check and the on-chain submission: a
// member can finish its index-based wait only to discover the chain has
// already accepted another member's result. The submitter must detect this
// post-wait and abort cleanly instead of attempting a doomed submission.
func TestSubmitResult_StateChangesDuringWait(t *testing.T) {
	groupParameters := &GroupParameters{
		GroupSize:       5,
		GroupQuorum:     4,
		HonestThreshold: 3,
	}

	localChain := Connect()

	if err := localChain.startDKG(); err != nil {
		t.Fatal(err)
	}

	operatorAddress, err := localChain.operatorAddress()
	if err != nil {
		t.Fatal(err)
	}

	operatorID, err := localChain.GetOperatorID(operatorAddress)
	if err != nil {
		t.Fatal(err)
	}

	var operatorsIDs chain.OperatorIDs
	var operatorsAddresses chain.Addresses

	for memberIndex := uint8(1); int(memberIndex) <= groupParameters.GroupSize; memberIndex++ {
		operatorsIDs = append(operatorsIDs, operatorID)
		operatorsAddresses = append(operatorsAddresses, operatorAddress)
	}

	groupSelectionResult := &GroupSelectionResult{
		OperatorsIDs:       operatorsIDs,
		OperatorsAddresses: operatorsAddresses,
	}

	if err = localChain.setDKGResultValidity(true); err != nil {
		t.Fatal(err)
	}

	testData, err := tecdsatest.LoadPrivateKeyShareTestFixtures(1)
	if err != nil {
		t.Fatalf("failed to load test data: [%v]", err)
	}
	result := &dkg.Result{
		Group:           group.NewGroup(groupParameters.DishonestThreshold(), groupParameters.GroupSize),
		PrivateKeyShare: tecdsa.NewPrivateKeyShare(testData[0]),
	}
	signatures := map[group.MemberIndex][]byte{
		1: []byte("signature 1"),
		2: []byte("signature 2"),
		3: []byte("signature 3"),
		4: []byte("signature 4"),
	}

	// Pre-assemble a DKG chain result we can submit from inside the hook to
	// simulate another member winning the race during our wait window.
	groupPublicKey, err := result.GroupPublicKey()
	if err != nil {
		t.Fatal(err)
	}

	competitorResult, err := localChain.AssembleDKGResult(
		group.MemberIndex(1),
		groupPublicKey,
		result.Group.OperatingMemberIndexes(),
		result.MisbehavedMembersIndexes(),
		signatures,
		groupSelectionResult,
	)
	if err != nil {
		t.Fatal(err)
	}

	// hookedWaitForBlockFn submits a competing DKG result the first time it
	// is invoked, then delegates to the standard test waiter. This mimics a
	// peer that wins the submission race while we are blocked on our delay.
	var hookFired bool
	hookedWaitForBlockFn := func(ctx context.Context, block uint64) error {
		if !hookFired {
			hookFired = true
			if submitErr := localChain.SubmitDKGResult(competitorResult); submitErr != nil {
				return fmt.Errorf("competitor submission failed: %w", submitErr)
			}
		}
		return testWaitForBlockFn(localChain)(ctx, block)
	}

	dkgResultSubmitter := newDkgResultSubmitter(
		&testutils.MockLogger{},
		localChain,
		groupParameters,
		groupSelectionResult,
		hookedWaitForBlockFn,
	)

	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	// memberIndex=2 yields a non-zero delay so the wait actually fires and
	// the post-wait re-check is exercised.
	err = dkgResultSubmitter.SubmitResult(
		ctx,
		group.MemberIndex(2),
		result,
		signatures,
	)
	if err != nil {
		t.Fatalf(
			"expected nil error after losing the submission race, got: %v",
			err,
		)
	}

	if localChain.dkgResult.SubmitterMemberIndex != group.MemberIndex(1) {
		t.Errorf(
			"competitor's submission was overwritten\nexpected submitter: %v\nactual submitter:   %v",
			group.MemberIndex(1),
			localChain.dkgResult.SubmitterMemberIndex,
		)
	}
}
