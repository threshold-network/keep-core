package tbtc

import (
	"bytes"
	"context"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"math/big"
	stdnet "net"
	"net/http"
	"os"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	ethereumRPC "github.com/ethereum/go-ethereum/rpc"
	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

type testFrostProductionAuthorizationReadiness struct {
	err               error
	unchangedErr      error
	calls             uint64
	unchangedCalls    uint64
	points            []FrostPreSignFinality
	headrooms         []uint64
	activeQuarantines []frostRetainedGroupActiveQuarantine
	// failingCalls is how many leading reconciliations fail with err before the
	// verifier starts succeeding. Zero with a non-nil err means every call
	// fails, which is the pre-existing behaviour.
	failingCalls uint64
}

func (readiness *testFrostProductionAuthorizationReadiness) verifyFrostProductionSignerReadiness(
	_ context.Context,
	point FrostPreSignFinality,
) (*frostProductionSignerReadinessSnapshot, error) {
	readiness.calls++
	readiness.points = append(readiness.points, point)
	if readiness.err != nil &&
		(readiness.failingCalls == 0 || readiness.calls <= readiness.failingCalls) {
		return nil, readiness.err
	}
	headroom := uint64(FrostNativeSignerAnchorMaximumHistoryEvents)
	if len(readiness.headrooms) != 0 {
		index := int(readiness.calls - 1)
		if index >= len(readiness.headrooms) {
			index = len(readiness.headrooms) - 1
		}
		headroom = readiness.headrooms[index]
	}
	snapshot := testFrostAnchorAdmissionReadinessSnapshot(headroom, headroom)
	snapshot.Journal = &frostRetainedGroupJournalSnapshot{
		Schema: frostRetainedGroupJournalSnapshotSchema,
		ActiveQuarantines: append(
			[]frostRetainedGroupActiveQuarantine{},
			readiness.activeQuarantines...,
		),
		QuarantineCount: uint64(len(readiness.activeQuarantines)),
		Complete:        true,
	}
	return snapshot, nil
}

func (readiness *testFrostProductionAuthorizationReadiness) verifyFrostProductionSignerReadinessUnchanged(
	_ context.Context,
	_ *frostProductionSignerReadinessSnapshot,
) error {
	readiness.unchangedCalls++
	return readiness.unchangedErr
}

func testFrostPreSignTransaction(
	t *testing.T,
	unsignedTx *bitcoin.TransactionBuilder,
) *FrostPreSignTransaction {
	t.Helper()
	signatureHashes, err := unsignedTx.ComputeSignatureHashes()
	if err != nil {
		t.Fatal(err)
	}
	transaction, err := newFrostPreSignTransaction(
		ActionDepositSweep,
		[20]byte{0x11},
		unsignedTx,
		signatureHashes,
	)
	if err != nil {
		t.Fatal(err)
	}
	return transaction
}

func TestFrostPreSignTransaction_StrippedHashAndLeadingZeroSighash(t *testing.T) {
	unsignedTx, _ := buildTaprootKeyPathUnsignedTxForTest(t)
	signatureHashes, err := unsignedTx.ComputeSignatureHashes()
	if err != nil {
		t.Fatal(err)
	}
	transaction, err := newFrostPreSignTransaction(
		ActionDepositSweep,
		[20]byte{0x11},
		unsignedTx,
		signatureHashes,
	)
	if err != nil {
		t.Fatal(err)
	}

	expectedRaw := unsignedTx.UnsignedTransaction().Serialize(bitcoin.Standard)
	if string(transaction.RawTransaction) != string(expectedRaw) {
		t.Fatal("pre-sign proposal did not retain exact stripped bytes")
	}
	if transaction.TransactionHash != bitcoin.ComputeHash(expectedRaw) {
		t.Fatal("pre-sign proposal reversed or changed the raw SHA256d txid")
	}
	if transaction.TransactionHash != unsignedTx.UnsignedTransaction().Hash() {
		t.Fatal("pre-sign proposal hash differs from keep-core internal txid")
	}
	if len(transaction.SignatureHashes) != 1 {
		t.Fatal("unexpected signature hash count")
	}
	expectedSignatureHash, err := fixedFrostPreSignSignatureHash(signatureHashes[0])
	if err != nil {
		t.Fatal(err)
	}
	if transaction.SignatureHashes[0] != expectedSignatureHash {
		t.Fatal("canonical fixed-width sighash was not preserved")
	}
	if transaction.SighashTypes[0] != 0 || transaction.SpendTypes[0] != 0 {
		t.Fatal("proposal is not frozen to DEFAULT/key-path/no-annex")
	}
}

func TestFixedFrostPreSignSignatureHash_PreservesLeadingZeroes(t *testing.T) {
	result, err := fixedFrostPreSignSignatureHash(big.NewInt(1))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < len(result)-1; i++ {
		if result[i] != 0 {
			t.Fatal("leading-zero sighash padding was not preserved")
		}
	}
	if result[len(result)-1] != 1 {
		t.Fatal("unexpected fixed-width sighash value")
	}
}

func TestFrostPreSignTransaction_RejectsCallerSuppliedSighashMismatch(
	t *testing.T,
) {
	unsignedTx, _ := buildTaprootKeyPathUnsignedTxForTest(t)
	canonical, err := unsignedTx.ComputeSignatureHashes()
	if err != nil {
		t.Fatal(err)
	}
	malicious := new(big.Int).Xor(canonical[0], big.NewInt(1))
	if _, err := newFrostPreSignTransaction(
		ActionDepositSweep,
		[20]byte{0x11},
		unsignedTx,
		[]*big.Int{malicious},
	); err == nil || !strings.Contains(err.Error(), "canonical BIP-341 digest") {
		t.Fatalf("unexpected mismatched-sighash result: [%v]", err)
	}
}

func TestFrostPreSignTransaction_RejectsFlexibleSighashAndNonKeyPathMode(t *testing.T) {
	unsignedTx, _ := buildTaprootKeyPathUnsignedTxForTest(t)
	transaction := testFrostPreSignTransaction(t, unsignedTx)

	transaction.SighashTypes[0] = 1
	if err := transaction.validate(); err == nil ||
		!strings.Contains(err.Error(), "SIGHASH_DEFAULT") {
		t.Fatalf("unexpected flexible-sighash validation result: [%v]", err)
	}
	transaction.SighashTypes[0] = 0
	transaction.SpendTypes[0] = 1
	if err := transaction.validate(); err == nil ||
		!strings.Contains(err.Error(), "key-path/no-annex") {
		t.Fatalf("unexpected spend-mode validation result: [%v]", err)
	}
}

func TestFrostPreSignTransaction_RejectsPostConstructionSighashMetadataMutation(
	t *testing.T,
) {
	unsignedTx, _ := buildTaprootKeyPathUnsignedTxForTest(t)
	transaction := testFrostPreSignTransaction(t, unsignedTx)

	changedValue := cloneFrostPreSignTransaction(transaction)
	changedValue.InputValues[0]++
	if err := changedValue.validate(); err == nil ||
		!strings.Contains(err.Error(), "differs from the stripped transaction") {
		t.Fatalf("mutated input value retained a cached BIP-341 digest: [%v]", err)
	}

	changedKey := cloneFrostPreSignTransaction(transaction)
	changedKey.SigningKeys[0][0] ^= 0xff
	if err := changedKey.validate(); err == nil ||
		!strings.Contains(err.Error(), "differs from the stripped transaction") {
		t.Fatalf("mutated signing key retained a cached BIP-341 digest: [%v]", err)
	}
}

func TestValidateFrostPreSignAuthorizationState_RejectsReorgAndInactiveWallet(t *testing.T) {
	unsignedTx, _ := buildTaprootKeyPathUnsignedTxForTest(t)
	transaction := testFrostPreSignTransaction(t, unsignedTx)
	proposal := completeTestFrostPreSignProposal(transaction)
	finality := FrostPreSignFinality{
		RelayTransactionHash:  [32]byte{0x71},
		BlockNumber:           100,
		BlockHash:             [32]byte{0x72},
		AuthorizationSequence: [32]byte{31: 1},
	}
	state := completeTestFrostPreSignState(proposal, finality)
	sequence := frostPreSignVariantSequence(finality)
	if err := validateFrostPreSignAuthorizationState(proposal, finality, sequence, state); err != nil {
		t.Fatalf("valid finalized state was rejected: [%v]", err)
	}

	reorged := *state
	reorged.Finality.BlockHash = [32]byte{0xff}
	if err := validateFrostPreSignAuthorizationState(proposal, finality, sequence, &reorged); err == nil {
		t.Fatal("expected pinned finalized block hash mismatch to fail")
	}
	inactive := *state
	inactive.WalletActive = false
	if err := validateFrostPreSignAuthorizationState(proposal, finality, sequence, &inactive); err == nil {
		t.Fatal("expected inactive/archived wallet to fail")
	}
	superseded := *state
	superseded.VariantSigningAllowed = false
	superseded.LatestVariantTransactionHash = bitcoin.Hash{0xff}
	superseded.LatestVariantAuthorizationSequence = [32]byte{31: 2}
	if err := validateFrostPreSignAuthorizationState(proposal, finality, sequence, &superseded); err == nil {
		t.Fatal("expected a superseded RBF variant to fail signing authorization")
	}
}

func TestThresholdFrostPreSignAuthorizationGate_RevalidatesCurrentFinalizedState(
	t *testing.T,
) {
	unsignedTx, _ := buildTaprootKeyPathUnsignedTxForTest(t)
	transaction := testFrostPreSignTransaction(t, unsignedTx)
	proposal := completeTestFrostPreSignProposal(transaction)
	relayFinality := FrostPreSignFinality{
		RelayTransactionHash:  [32]byte{0x71},
		BlockNumber:           100,
		BlockHash:             [32]byte{0x72},
		AuthorizationSequence: [32]byte{31: 1},
	}
	currentFinality := FrostPreSignFinality{
		BlockNumber: 101,
		BlockHash:   [32]byte{0x73},
	}
	backend := &testFrostPreSignAuthorizationBackend{
		proposal:        proposal,
		currentFinality: &currentFinality,
		states: map[uint64]*FrostPreSignAuthorizationState{
			relayFinality.BlockNumber: completeTestFrostPreSignState(
				proposal,
				relayFinality,
			),
			currentFinality.BlockNumber: completeTestFrostPreSignState(
				proposal,
				currentFinality,
			),
		},
	}
	gate := &thresholdFrostPreSignAuthorizationGate{
		backend:             backend,
		activationProfile:   activationProfileForTestProposal(proposal),
		storeBinding:        testFrostDurableSessionStoreBinding(t),
		productionReadiness: &testFrostProductionAuthorizationReadiness{},
	}
	authorization := &frostPreSignAuthorization{
		ActivationProfileHash: gate.activationProfile.ProfileHash,
		AuthorizationID:       proposal.Digest,
		ReservationID:         proposal.ReservationID,
		VariantRoot:           proposal.AuthorizationRoot,
		TransactionHash:       proposal.Transaction.TransactionHash,
		Finality:              relayFinality,
		VariantSequence:       frostPreSignVariantSequence(relayFinality),
		proposal:              proposal,
	}
	if err := gate.revalidate(context.Background(), authorization); err != nil {
		t.Fatalf("current active authorization was rejected: [%v]", err)
	}

	superseded := *backend.states[currentFinality.BlockNumber]
	superseded.VariantSigningAllowed = false
	superseded.LatestVariantTransactionHash = bitcoin.Hash{0xff}
	superseded.LatestVariantAuthorizationSequence = [32]byte{31: 2}
	backend.states[currentFinality.BlockNumber] = &superseded
	if err := gate.revalidate(context.Background(), authorization); err == nil ||
		!strings.Contains(err.Error(), "transaction variant") {
		t.Fatalf("superseded current authorization was accepted: [%v]", err)
	}
}

func TestThresholdFrostPreSignAuthorizationGate_RefusesUnreadyInteractiveSigner(
	t *testing.T,
) {
	unsignedTx, _ := buildTaprootKeyPathUnsignedTxForTest(t)
	transaction := testFrostPreSignTransaction(t, unsignedTx)
	proposal := completeTestFrostPreSignProposal(transaction)
	currentFinality := &FrostPreSignFinality{
		BlockNumber: 101,
		BlockHash:   [32]byte{0x73},
	}
	for _, condition := range []string{
		"all interactive flags absent",
		"interactive engine absent",
		"interactive-only gate absent",
	} {
		t.Run(condition, func(t *testing.T) {
			readiness := &testFrostProductionAuthorizationReadiness{
				err: fmt.Errorf("%s", condition),
			}
			gate := &thresholdFrostPreSignAuthorizationGate{
				backend: &testFrostPreSignAuthorizationBackend{
					proposal:        proposal,
					currentFinality: currentFinality,
				},
				activationProfile:   activationProfileForTestProposal(proposal),
				storeBinding:        testFrostDurableSessionStoreBinding(t),
				productionReadiness: readiness,
			}
			if _, err := gate.authorize(context.Background(), transaction); err == nil ||
				!strings.Contains(err.Error(), "not authorization-ready") {
				t.Fatalf("authorization accepted unready signer: [%v]", err)
			}
			if readiness.calls != 1 {
				t.Fatalf("authorization readiness called [%d] times", readiness.calls)
			}

			authorization := &frostPreSignAuthorization{proposal: proposal}
			if err := gate.revalidate(context.Background(), authorization); err == nil ||
				!strings.Contains(err.Error(), "not authorization-ready") {
				t.Fatalf("revalidation accepted unready signer: [%v]", err)
			}
			if readiness.calls != 2 {
				t.Fatalf("revalidation readiness called [%d] total times", readiness.calls)
			}
		})
	}
}

func TestThresholdFrostPreSignAuthorizationGate_ReusesReadinessAtUnchangedFinality(
	t *testing.T,
) {
	unsignedTx, _ := buildTaprootKeyPathUnsignedTxForTest(t)
	transaction := testFrostPreSignTransaction(t, unsignedTx)
	proposal := completeTestFrostPreSignProposal(transaction)
	relayFinality := FrostPreSignFinality{
		RelayTransactionHash:  [32]byte{0x71},
		BlockNumber:           100,
		BlockHash:             [32]byte{0x72},
		AuthorizationSequence: [32]byte{31: 1},
	}
	currentFinality := FrostPreSignFinality{
		BlockNumber: 101,
		BlockHash:   [32]byte{0x73},
	}
	laterFinality := FrostPreSignFinality{
		BlockNumber: 102,
		BlockHash:   [32]byte{0x74},
	}
	backend := &testFrostPreSignAuthorizationBackend{
		currentFinality: &currentFinality,
		states: map[uint64]*FrostPreSignAuthorizationState{
			relayFinality.BlockNumber: completeTestFrostPreSignState(
				proposal,
				relayFinality,
			),
			currentFinality.BlockNumber: completeTestFrostPreSignState(
				proposal,
				currentFinality,
			),
			laterFinality.BlockNumber: completeTestFrostPreSignState(
				proposal,
				laterFinality,
			),
		},
	}
	readiness := &testFrostProductionAuthorizationReadiness{}
	gate := &thresholdFrostPreSignAuthorizationGate{
		backend:             backend,
		activationProfile:   activationProfileForTestProposal(proposal),
		storeBinding:        testFrostDurableSessionStoreBinding(t),
		productionReadiness: readiness,
	}
	authorization := &frostPreSignAuthorization{
		ActivationProfileHash: gate.activationProfile.ProfileHash,
		AuthorizationID:       proposal.Digest,
		ReservationID:         proposal.ReservationID,
		VariantRoot:           proposal.AuthorizationRoot,
		TransactionHash:       proposal.Transaction.TransactionHash,
		Finality:              relayFinality,
		VariantSequence:       frostPreSignVariantSequence(relayFinality),
		proposal:              proposal,
	}

	if err := gate.revalidate(context.Background(), authorization); err != nil {
		t.Fatal(err)
	}
	if err := gate.revalidate(context.Background(), authorization); err != nil {
		t.Fatal(err)
	}
	if readiness.calls != 1 || readiness.unchangedCalls != 3 {
		t.Fatalf(
			"unchanged finality repeated full readiness reconciliation: full [%d], unchanged [%d]",
			readiness.calls,
			readiness.unchangedCalls,
		)
	}

	backend.currentFinality = &laterFinality
	if err := gate.revalidate(context.Background(), authorization); err != nil {
		t.Fatal(err)
	}
	if readiness.calls != 2 || readiness.unchangedCalls != 4 {
		t.Fatalf(
			"new finality did not refresh exactly one readiness reconciliation: full [%d], unchanged [%d]",
			readiness.calls,
			readiness.unchangedCalls,
		)
	}
	if backend.currentFinalityCalls != 3 {
		t.Fatalf(
			"revalidation did not poll finalized point on every pass: [%d]",
			backend.currentFinalityCalls,
		)
	}
}

func TestFrostPreSignActivationProfile_IndependentlyPinsProposal(t *testing.T) {
	unsignedTx, _ := buildTaprootKeyPathUnsignedTxForTest(t)
	transaction := testFrostPreSignTransaction(t, unsignedTx)
	proposal := completeTestFrostPreSignProposal(transaction)
	profile := activationProfileForTestProposal(proposal)
	if err := profile.validateProposal(proposal); err != nil {
		t.Fatalf("valid locally pinned proposal was rejected: [%v]", err)
	}

	backendSubstitution := *proposal
	backendSubstitution.RegistryCodeHash = [32]byte{0xff}
	if err := profile.validateProposal(&backendSubstitution); err == nil {
		t.Fatal("backend-controlled code hash bypassed local activation profile")
	}
	tamperedProfile := profile
	tamperedProfile.BridgeAddress[0] ^= 0xff
	if err := tamperedProfile.validate(); err == nil {
		t.Fatal("activation profile mutation did not invalidate its manifest hash")
	}
}

func TestFrostPreSignAuthorizationProposal_COMPLETEV2CommitmentVectors(
	t *testing.T,
) {
	repeat := func(value byte, count int) []byte {
		result := make([]byte, count)
		for i := range result {
			result[i] = value
		}
		return result
	}
	bytes20 := func(value byte) [20]byte {
		result := [20]byte{}
		copy(result[:], repeat(value, len(result)))
		return result
	}
	bytes32 := func(value byte) [32]byte {
		result := [32]byte{}
		copy(result[:], repeat(value, len(result)))
		return result
	}
	fromHex := func(value string) [32]byte {
		decoded, err := hex.DecodeString(value)
		if err != nil || len(decoded) != 32 {
			t.Fatalf("invalid expected commitment vector [%s]: [%v]", value, err)
		}
		result := [32]byte{}
		copy(result[:], decoded)
		return result
	}

	baseProposal := &FrostPreSignAuthorizationProposal{
		Transaction: &FrostPreSignTransaction{
			Action:              FrostPreSignActionDepositSweep,
			WalletPublicKeyHash: bytes20(0x55),
			TransactionHash:     bitcoin.Hash(bytes32(0xdd)),
			SigningKeys:         [][32]byte{bytes32(0xf1), bytes32(0xf2)},
			SignatureHashes:     [][32]byte{bytes32(0xa1), bytes32(0xa2)},
		},
		WalletID:              bytes32(0x66),
		WalletMembersIDsHash:  bytes32(0x77),
		SnapshotHash:          bytes32(0x88),
		ResourceHash:          bytes32(0x99),
		OrderedInputRoot:      bytes32(0xaa),
		ApplyPlanHash:         bytes32(0xee),
		FeeLimitSnapshot:      12345,
		DomainChainID:         [32]byte{31: 1},
		BridgeAddress:         bytes20(0x11),
		RegistryAddress:       bytes20(0x22),
		FrostRegistry:         bytes20(0x33),
		ProposalValidator:     bytes20(0x44),
		ReservationProtocolID: frostPreSignReservationProtocolID(),
		SigningPolicyHash:     frostPreSignSigningPolicyHash(),
	}

	authorizationRoot, err := baseProposal.computeAuthorizationRoot()
	if err != nil {
		t.Fatal(err)
	}
	for name, actualExpected := range map[string][2][32]byte{
		"reservation protocol ID": {
			baseProposal.ReservationProtocolID,
			fromHex("abd8644248fc5423764f05fbeee2ba1c29e4e7067062568d624974470830571f"),
		},
		"signing policy hash": {
			baseProposal.SigningPolicyHash,
			fromHex("742307b79bb33abdff195fbb3e5b3aebdccdec6a7194c4659c71a46223dbebf0"),
		},
		"authorization root": {
			authorizationRoot,
			fromHex("1cbed17d3761265884413bd0b96f1afc08cb83ca4d97cd1a922e3a453b569297"),
		},
	} {
		if actualExpected[0] != actualExpected[1] {
			t.Fatalf("%s differs from Solidity/ethers vector: [%x]", name, actualExpected[0])
		}
	}

	// These vectors were independently generated with ethers' Solidity ABI
	// encoder. COMPLETE_V2 binds two plan-data words for deposit sweep, one for
	// redemption, and two intentional zero words for both moving-funds actions.
	for _, test := range []struct {
		name             string
		action           FrostPreSignAction
		applyPlanData1   [32]byte
		applyPlanData2   [32]byte
		lockedPlanHash   string
		reservationID    string
		authorizationDig string
	}{
		{
			"deposit sweep",
			FrostPreSignActionDepositSweep,
			bytes32(0xbb),
			bytes32(0xcc),
			"992974de2148fe99daf8dd8bf45f62f39927e9ee4e00d346c519ab5664b99970",
			"0a1aa347dc581bc1b611c8f487b331616b55503cb031d7724feda25b00ed0fae",
			"71c5a0592f0ac17c7ce7ee04ab6d211318a850fa6995acf34c8826269d8c5c8b",
		},
		{
			"redemption",
			FrostPreSignActionRedemption,
			bytes32(0xbb),
			[32]byte{},
			"8e4d3b1c018e48f2fe7ff0b798ae3475309f07ecba1b6495c65da1f80448b213",
			"6b2a67b13cf42b5fab432b66d6a0df560b0a97eb20bb23b17fde80261ec10767",
			"ea513ab39cc9119f1c082d4df5593d571fe8505ffb8915351f878e502d54f298",
		},
		{
			"moving funds",
			FrostPreSignActionMovingFunds,
			[32]byte{},
			[32]byte{},
			"dd16ba2ae1af6da990302560cbefad6b7dd80cce6ed4cf09d19265d6d3166b9e",
			"1e21da4908e2a150a39f9c01a49c45976f8b452abbcb93dacafebb56196462c6",
			"f1d45fdb68c0a714e7252d99849dc83e9d8dc79ce5385a520c9c8ef71a30fb38",
		},
		{
			"moved funds sweep",
			FrostPreSignActionMovedFundsSweep,
			[32]byte{},
			[32]byte{},
			"dd16ba2ae1af6da990302560cbefad6b7dd80cce6ed4cf09d19265d6d3166b9e",
			"95a5b080aa3436611347f8cd00674fdff9e100a8b3df224cd4c1278ad62bed5d",
			"7a028a7a2c1cb69db8525531d2ea96ac1b1d5c8b573b790ba1f08612ec8c00b8",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			proposal := *baseProposal
			transaction := *baseProposal.Transaction
			transaction.Action = test.action
			proposal.Transaction = &transaction
			proposal.ApplyPlanData1 = test.applyPlanData1
			proposal.ApplyPlanData2 = test.applyPlanData2

			for name, actualExpected := range map[string][2][32]byte{
				"locked plan hash": {
					proposal.computeLockedPlanHash(),
					fromHex(test.lockedPlanHash),
				},
				"reservation ID": {
					proposal.computeReservationID(),
					fromHex(test.reservationID),
				},
				"pre-authorization digest": {
					proposal.computeDigest(authorizationRoot),
					fromHex(test.authorizationDig),
				},
			} {
				if actualExpected[0] != actualExpected[1] {
					t.Fatalf(
						"%s differs from Solidity/ethers vector: [%x]",
						name,
						actualExpected[0],
					)
				}
			}
		})
	}
}

func TestFrostPreSignAuthorizationProposal_AcceptsActionSpecificZeroPlanData(
	t *testing.T,
) {
	unsignedTx, _ := buildTaprootKeyPathUnsignedTxForTest(t)

	for _, test := range []struct {
		name           string
		action         FrostPreSignAction
		applyPlanData1 [32]byte
	}{
		{"redemption", FrostPreSignActionRedemption, [32]byte{0x38}},
		{"moving funds", FrostPreSignActionMovingFunds, [32]byte{}},
		{"moved funds sweep", FrostPreSignActionMovedFundsSweep, [32]byte{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			transaction := testFrostPreSignTransaction(t, unsignedTx)
			transaction.Action = test.action
			proposal := completeTestFrostPreSignProposal(transaction)
			proposal.ApplyPlanData1 = test.applyPlanData1
			proposal.ApplyPlanData2 = [32]byte{}
			proposal.ReservationID = proposal.computeReservationID()
			proposal.Digest = proposal.computeDigest(proposal.AuthorizationRoot)

			if err := proposal.validate(); err != nil {
				t.Fatalf("valid action-specific zero plan data was rejected: [%v]", err)
			}
		})
	}
}

func TestFrostPreSignAuthorizationProposal_RejectsBackendCommitmentSubstitution(
	t *testing.T,
) {
	unsignedTx, _ := buildTaprootKeyPathUnsignedTxForTest(t)
	transaction := testFrostPreSignTransaction(t, unsignedTx)
	valid := completeTestFrostPreSignProposal(transaction)
	if err := valid.validate(); err != nil {
		t.Fatalf("valid locally recomputed proposal was rejected: [%v]", err)
	}

	for name, mutate := range map[string]func(*FrostPreSignAuthorizationProposal){
		"digest": func(proposal *FrostPreSignAuthorizationProposal) {
			proposal.Digest[0] ^= 0xff
		},
		"authorization root": func(proposal *FrostPreSignAuthorizationProposal) {
			proposal.AuthorizationRoot[0] ^= 0xff
		},
		"reservation ID": func(proposal *FrostPreSignAuthorizationProposal) {
			proposal.ReservationID[0] ^= 0xff
		},
		"wallet members hash": func(proposal *FrostPreSignAuthorizationProposal) {
			proposal.WalletMembersIDsHash[0] ^= 0xff
		},
		"resource hash": func(proposal *FrostPreSignAuthorizationProposal) {
			proposal.ResourceHash[0] ^= 0xff
		},
	} {
		t.Run(name, func(t *testing.T) {
			proposal := cloneFrostPreSignAuthorizationProposal(valid)
			mutate(proposal)
			if err := proposal.validate(); err == nil {
				t.Fatal("backend-controlled commitment substitution was accepted")
			}
		})
	}
}

func TestThresholdFrostPreSignAuthorizationGate_ProfileMismatchPrecedesSeatSignature(
	t *testing.T,
) {
	unsignedTx, _ := buildTaprootKeyPathUnsignedTxForTest(t)
	transaction := testFrostPreSignTransaction(t, unsignedTx)
	proposal := completeTestFrostPreSignProposal(transaction)
	profile := activationProfileForTestProposal(proposal)
	proposal.RegistryCodeHash = [32]byte{0xff}
	countingSigner := &countingFrostPreSignChainSigning{
		Signing: Connect().Signing(),
	}
	currentFinality := &FrostPreSignFinality{BlockNumber: 9, BlockHash: [32]byte{0x70}}
	gate := &thresholdFrostPreSignAuthorizationGate{
		backend: &testFrostPreSignAuthorizationBackend{
			proposal:        proposal,
			currentFinality: currentFinality,
		},
		activationProfile:   profile,
		storeBinding:        testFrostDurableSessionStoreBinding(t),
		productionReadiness: &testFrostProductionAuthorizationReadiness{},
		anchorAdmission:     &frostNativeSignerAnchorAdmissionController{},
		signing:             countingSigner,
		wallet: wallet{
			signingGroupOperators: make(
				[]chain.Address,
				frostPreSignAuthorizationMaximumSeats,
			),
		},
		localMemberIndexes: []group.MemberIndex{1},
		threshold:          frostPreSignAuthorizationThreshold,
	}

	if _, err := gate.authorize(context.Background(), transaction); err == nil {
		t.Fatal("expected local activation profile mismatch to fail")
	}
	if countingSigner.signCalls != 0 {
		t.Fatal("seat signature was produced before local activation-profile validation")
	}
}

func TestBuildFrostPreSignSeatAttestation_UsesExactlyLowestThresholdSeats(t *testing.T) {
	members := make([]uint32, frostPreSignAuthorizationMaximumSeats)
	for i := range members {
		members[i] = uint32(i + 1)
	}
	signatures := make(map[group.MemberIndex][]byte)
	for seat := group.MemberIndex(1); seat <= 60; seat++ {
		signature := make([]byte, 65)
		signature[0] = byte(seat)
		signatures[seat] = signature
	}

	attestation, err := buildFrostPreSignSeatAttestation(
		members,
		signatures,
		frostPreSignAuthorizationThreshold,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(attestation.SigningMemberIndices) != frostPreSignAuthorizationThreshold ||
		len(attestation.Signatures) != frostPreSignAuthorizationThreshold*65 {
		t.Fatal("attestation did not contain exactly the Solidity threshold")
	}
	for i, seat := range attestation.SigningMemberIndices {
		if seat != uint8(i+1) || attestation.Signatures[i*65] != seat {
			t.Fatal("attestation did not select deterministic lowest unique seats")
		}
	}
}

func TestFrostPreSignTransportPublicKeyMatches(t *testing.T) {
	if !frostPreSignTransportPublicKeyMatches([]byte{1, 2}, []byte{1, 2}) {
		t.Fatal("matching payload and transport key was rejected")
	}
	if frostPreSignTransportPublicKeyMatches([]byte{1, 2}, []byte{1, 3}) {
		t.Fatal("payload public key substitution was accepted")
	}
}

func TestClaimFrostPreSignRemoteSeat_BoundsAdmissionPerAuthenticatedSeat(
	t *testing.T,
) {
	local := map[group.MemberIndex]struct{}{1: {}}
	seen := make(map[group.MemberIndex]struct{})
	mutex := &sync.Mutex{}

	if !claimFrostPreSignRemoteSeat(2, 100, local, seen, mutex) {
		t.Fatal("first remote-seat attestation was not admitted")
	}
	for i := 0; i < 1000; i++ {
		if claimFrostPreSignRemoteSeat(2, 100, local, seen, mutex) {
			t.Fatal("duplicate Byzantine-seat flood was admitted")
		}
	}
	if claimFrostPreSignRemoteSeat(1, 100, local, seen, mutex) {
		t.Fatal("local-seat network echo was admitted")
	}
	if claimFrostPreSignRemoteSeat(0, 100, local, seen, mutex) ||
		claimFrostPreSignRemoteSeat(101, 100, local, seen, mutex) {
		t.Fatal("out-of-range seat was admitted")
	}
}

func TestThresholdFrostPreSignAuthorizationGate_RejectsStaleDigestBeforeSeatAdmission(
	t *testing.T,
) {
	localChain := Connect()
	remoteChain := Connect()
	localSigning := &testFrostPreSignFixedSignatureSigning{
		Signing: localChain.Signing(),
	}
	remoteSigning := &testFrostPreSignFixedSignatureSigning{
		Signing: remoteChain.Signing(),
	}
	proposal := &FrostPreSignAuthorizationProposal{
		Digest:           [32]byte{0x11},
		WalletMembersIDs: []uint32{1, 2},
	}
	staleDigest := proposal.Digest
	staleDigest[0] ^= 0xff
	staleSignature, err := remoteSigning.Sign(staleDigest[:])
	if err != nil {
		t.Fatal(err)
	}
	currentSignature, err := remoteSigning.Sign(proposal.Digest[:])
	if err != nil {
		t.Fatal(err)
	}
	remotePublicKey := remoteSigning.PublicKey()
	channel := &testFrostPreSignAuthorizationBroadcastChannel{
		messages: []net.Message{
			&testFrostPreSignAuthorizationNetworkMessage{
				publicKey: remotePublicKey,
				payload: &frostPreSignAuthorizationMessage{
					SenderIDValue: 2,
					Digest:        staleDigest[:],
					PublicKey:     remotePublicKey,
					Signature:     staleSignature,
				},
			},
			&testFrostPreSignAuthorizationNetworkMessage{
				publicKey: remotePublicKey,
				payload: &frostPreSignAuthorizationMessage{
					SenderIDValue: 2,
					Digest:        proposal.Digest[:],
					PublicKey:     remotePublicKey,
					Signature:     currentSignature,
				},
			},
		},
	}
	operators := []chain.Address{
		localSigning.Address(),
		remoteSigning.Address(),
	}
	gate := &thresholdFrostPreSignAuthorizationGate{
		signing:          localSigning,
		broadcastChannel: channel,
		membershipValidator: group.NewMembershipValidator(
			&testutils.MockLogger{},
			operators,
			localSigning,
		),
		wallet: wallet{
			signingGroupOperators: operators,
		},
		localMemberIndexes: []group.MemberIndex{1},
		threshold:          2,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	attestation, err := gate.collectSeatAttestations(ctx, proposal)
	if err != nil {
		t.Fatalf("current attestation was suppressed by stale replay: [%v]", err)
	}
	if len(attestation.SigningMemberIndices) != 2 ||
		attestation.SigningMemberIndices[0] != 1 ||
		attestation.SigningMemberIndices[1] != 2 {
		t.Fatalf("unexpected seat attestation: [%+v]", attestation)
	}
}

type testFrostPreSignFixedSignatureSigning struct {
	chain.Signing
}

func (signing *testFrostPreSignFixedSignatureSigning) Sign(
	message []byte,
) ([]byte, error) {
	return testFrostPreSignFixedSignature(
		signing.PublicKey(),
		message,
	), nil
}

func (signing *testFrostPreSignFixedSignatureSigning) Verify(
	message []byte,
	signature []byte,
) (bool, error) {
	return signing.VerifyWithPublicKey(
		message,
		signature,
		signing.PublicKey(),
	)
}

func (*testFrostPreSignFixedSignatureSigning) VerifyWithPublicKey(
	message []byte,
	signature []byte,
	publicKey []byte,
) (bool, error) {
	return bytes.Equal(
		signature,
		testFrostPreSignFixedSignature(publicKey, message),
	), nil
}

func testFrostPreSignFixedSignature(
	publicKey []byte,
	message []byte,
) []byte {
	payload := make([]byte, 0, len(publicKey)+len(message))
	payload = append(payload, publicKey...)
	payload = append(payload, message...)
	digest := sha512.Sum512(payload)
	return append(digest[:], byte(0))
}

type testFrostPreSignAuthorizationBroadcastChannel struct {
	messages []net.Message
}

func (*testFrostPreSignAuthorizationBroadcastChannel) Name() string {
	return "test-frost-pre-sign-authorization"
}

func (*testFrostPreSignAuthorizationBroadcastChannel) Send(
	context.Context,
	net.TaggedMarshaler,
	...net.RetransmissionStrategy,
) error {
	return nil
}

func (channel *testFrostPreSignAuthorizationBroadcastChannel) Recv(
	ctx context.Context,
	handler func(net.Message),
) {
	for _, message := range channel.messages {
		select {
		case <-ctx.Done():
			return
		default:
			handler(message)
		}
	}
}

func (*testFrostPreSignAuthorizationBroadcastChannel) SetUnmarshaler(
	func() net.TaggedUnmarshaler,
) {
}

func (*testFrostPreSignAuthorizationBroadcastChannel) SetFilter(
	net.BroadcastChannelFilter,
) error {
	return nil
}

type testFrostPreSignAuthorizationNetworkMessage struct {
	publicKey []byte
	payload   *frostPreSignAuthorizationMessage
}

func (*testFrostPreSignAuthorizationNetworkMessage) TransportSenderID() net.TransportIdentifier {
	return nil
}

func (message *testFrostPreSignAuthorizationNetworkMessage) SenderPublicKey() []byte {
	return message.publicKey
}

func (message *testFrostPreSignAuthorizationNetworkMessage) Payload() interface{} {
	return message.payload
}

func (*testFrostPreSignAuthorizationNetworkMessage) Type() string {
	return "test-frost-pre-sign-authorization"
}

func (*testFrostPreSignAuthorizationNetworkMessage) Seqno() uint64 {
	return 0
}

type testFrostPreSignAuthorizationGate struct {
	mutex           sync.Mutex
	authorizeErr    error
	revalidateErr   error
	admitInputErr   error
	authorizeCalls  int
	revalidateCalls int
	admitInputCalls int
	admitInputHeld  int
	admitInputPeak  int
	finalizedBlock  uint64
	proposal        *FrostPreSignAuthorizationProposal
}

func (tfpsag *testFrostPreSignAuthorizationGate) authorize(
	ctx context.Context,
	transaction *FrostPreSignTransaction,
) (*frostPreSignAuthorization, error) {
	tfpsag.authorizeCalls++
	if tfpsag.authorizeErr != nil {
		return nil, tfpsag.authorizeErr
	}
	proposal := completeTestFrostPreSignProposal(transaction)
	if tfpsag.proposal != nil {
		proposal = tfpsag.proposal
		proposal.Transaction = transaction
	}
	finalizedBlock := tfpsag.finalizedBlock
	if finalizedBlock == 0 {
		finalizedBlock = 10
	}
	authorization := &frostPreSignAuthorization{
		ActivationProfileHash: testOutboxActivationProfile,
		AuthorizationID:       proposal.Digest,
		ReservationID:         proposal.ReservationID,
		VariantRoot:           proposal.AuthorizationRoot,
		TransactionHash:       transaction.TransactionHash,
		Finality: FrostPreSignFinality{
			RelayTransactionHash:  [32]byte{0x71},
			BlockNumber:           finalizedBlock,
			BlockHash:             [32]byte{0x72},
			AuthorizationSequence: [32]byte{31: 1},
		},
		VariantSequence: FrostPreSignVariantSequence{
			AuthorizationSequence: [32]byte{31: 1},
		},
		proposal: proposal,
	}
	return authorization, nil
}

func (tfpsag *testFrostPreSignAuthorizationGate) revalidate(
	ctx context.Context,
	authorization *frostPreSignAuthorization,
) error {
	tfpsag.mutex.Lock()
	defer tfpsag.mutex.Unlock()
	tfpsag.revalidateCalls++
	return tfpsag.revalidateErr
}

// admitInput stands in for the node-wide anchor admission controller. It takes
// nothing real, but it counts: the batch loop must call it once per input and
// run the release it returns before the next one, so admitInputHeld is back to
// zero between inputs and admitInputPeak never exceeds one. A stub that simply
// returned a no-op would let a regression to a single batch-wide reservation
// pass unnoticed here.
func (tfpsag *testFrostPreSignAuthorizationGate) admitInput(
	ctx context.Context,
	authorization *frostPreSignAuthorization,
) (func(), error) {
	tfpsag.mutex.Lock()
	defer tfpsag.mutex.Unlock()
	tfpsag.admitInputCalls++
	if tfpsag.admitInputErr != nil {
		return nil, tfpsag.admitInputErr
	}
	tfpsag.admitInputHeld++
	if tfpsag.admitInputHeld > tfpsag.admitInputPeak {
		tfpsag.admitInputPeak = tfpsag.admitInputHeld
	}
	released := false
	return func() {
		tfpsag.mutex.Lock()
		defer tfpsag.mutex.Unlock()
		if released {
			return
		}
		released = true
		tfpsag.admitInputHeld--
	}, nil
}

func (tfpsag *testFrostPreSignAuthorizationGate) setRevalidateError(err error) {
	tfpsag.mutex.Lock()
	defer tfpsag.mutex.Unlock()
	tfpsag.revalidateErr = err
}

func completeTestFrostPreSignProposal(
	transaction *FrostPreSignTransaction,
) *FrostPreSignAuthorizationProposal {
	members := make([]uint32, frostPreSignAuthorizationMaximumSeats)
	for i := range members {
		members[i] = uint32(i + 1)
	}
	resourceIDs := [][32]byte{{0x26}}
	proposal := &FrostPreSignAuthorizationProposal{
		Transaction:               transaction,
		WalletID:                  [32]byte{0x21},
		SnapshotHash:              [32]byte{0x22},
		OrderedInputRoot:          [32]byte{0x24},
		ApplyPlanHash:             [32]byte{0x25},
		ApplyPlanData1:            [32]byte{0x38},
		ApplyPlanData2:            [32]byte{0x39},
		FeeLimitSnapshot:          10000,
		ResourceIDs:               resourceIDs,
		WalletMembersIDs:          members,
		DomainChainID:             [32]byte{0x2b},
		ActivationManifestHash:    [32]byte{0x3e},
		ImplementationSetHash:     [32]byte{0x3f},
		BridgeAddress:             [20]byte{0x2c},
		RegistryAddress:           [20]byte{0x2d},
		CompleteRouter:            [20]byte{0x3c},
		FrostRegistry:             [20]byte{0x2e},
		ProposalValidator:         [20]byte{0x3a},
		SortitionPool:             [20]byte{0x2f},
		BridgeCodeHash:            [32]byte{0x30},
		RegistryCodeHash:          [32]byte{0x31},
		CompleteRouterCodeHash:    [32]byte{0x3d},
		FrostRegistryCodeHash:     [32]byte{0x32},
		ProposalValidatorCodeHash: [32]byte{0x3b},
		SortitionPoolCodeHash:     [32]byte{0x33},
		ReservationProtocolID:     frostPreSignReservationProtocolID(),
		EvidenceProtocolID:        frostCompleteEvidenceProtocolID(),
		SigningPolicyHash:         frostPreSignSigningPolicyHash(),
		PreparationFinality: FrostPreSignFinality{
			RelayTransactionHash: [32]byte{0x36},
			BlockNumber:          8,
			BlockHash:            [32]byte{0x37},
		},
	}
	proposal.WalletMembersIDsHash = frostPreSignKeccak256(
		frostPreSignABIUint32Array(members),
	)
	proposal.ResourceHash = frostPreSignKeccak256(
		frostPreSignABIBytes32Array(resourceIDs),
	)
	proposal.AuthorizationRoot, _ = proposal.computeAuthorizationRoot()
	proposal.ReservationID = proposal.computeReservationID()
	proposal.Digest = proposal.computeDigest(proposal.AuthorizationRoot)
	return proposal
}

func completeTestFrostPreSignState(
	proposal *FrostPreSignAuthorizationProposal,
	finality FrostPreSignFinality,
) *FrostPreSignAuthorizationState {
	return &FrostPreSignAuthorizationState{
		Finality:                           finality,
		DomainChainID:                      proposal.DomainChainID,
		ActivationManifestHash:             proposal.ActivationManifestHash,
		ImplementationSetHash:              proposal.ImplementationSetHash,
		BridgeAddress:                      proposal.BridgeAddress,
		RegistryAddress:                    proposal.RegistryAddress,
		CompleteRouter:                     proposal.CompleteRouter,
		FrostRegistry:                      proposal.FrostRegistry,
		ProposalValidator:                  proposal.ProposalValidator,
		SortitionPool:                      proposal.SortitionPool,
		BridgeCodeHash:                     proposal.BridgeCodeHash,
		RegistryCodeHash:                   proposal.RegistryCodeHash,
		CompleteRouterCodeHash:             proposal.CompleteRouterCodeHash,
		FrostRegistryCodeHash:              proposal.FrostRegistryCodeHash,
		ProposalValidatorCodeHash:          proposal.ProposalValidatorCodeHash,
		SortitionPoolCodeHash:              proposal.SortitionPoolCodeHash,
		ReservationProtocolID:              proposal.ReservationProtocolID,
		EvidenceProtocolID:                 proposal.EvidenceProtocolID,
		SigningPolicyHash:                  proposal.SigningPolicyHash,
		WalletActive:                       true,
		WalletID:                           proposal.WalletID,
		WalletPublicKeyHash:                proposal.Transaction.WalletPublicKeyHash,
		WalletMembersIDsHash:               proposal.WalletMembersIDsHash,
		WalletXOnlyOutputKey:               proposal.WalletID,
		ActiveReservationID:                proposal.ReservationID,
		ReservationWalletID:                proposal.WalletID,
		ReservationWalletPublicKeyHash:     proposal.Transaction.WalletPublicKeyHash,
		ReservationSnapshotHash:            proposal.SnapshotHash,
		ReservationResourceHash:            proposal.ResourceHash,
		ReservationOrderedInputRoot:        proposal.OrderedInputRoot,
		ReservationApplyPlanData1:          proposal.ApplyPlanData1,
		ReservationApplyPlanData2:          proposal.ApplyPlanData2,
		ReservationFeeLimitSnapshot:        proposal.FeeLimitSnapshot,
		ReservationAction:                  proposal.Transaction.Action,
		ReservationActive:                  true,
		VariantTransactionHash:             proposal.Transaction.TransactionHash,
		VariantReservationID:               proposal.ReservationID,
		VariantAuthorizationRoot:           proposal.AuthorizationRoot,
		VariantApplyPlanHash:               proposal.ApplyPlanHash,
		VariantAuthorizationSequence:       [32]byte{31: 1},
		VariantFraudDefenseAuthorized:      true,
		VariantSigningAllowed:              true,
		LatestVariantTransactionHash:       proposal.Transaction.TransactionHash,
		LatestVariantAuthorizationSequence: [32]byte{31: 1},
		LatestVariantSigningAllowed:        true,
	}
}

func activationProfileForTestProposal(
	proposal *FrostPreSignAuthorizationProposal,
) FrostPreSignActivationProfile {
	profile := FrostPreSignActivationProfile{
		DomainChainID:             proposal.DomainChainID,
		ActivationManifestHash:    proposal.ActivationManifestHash,
		ImplementationSetHash:     proposal.ImplementationSetHash,
		BridgeAddress:             proposal.BridgeAddress,
		RegistryAddress:           proposal.RegistryAddress,
		CompleteRouter:            proposal.CompleteRouter,
		FrostRegistry:             proposal.FrostRegistry,
		ProposalValidator:         proposal.ProposalValidator,
		SortitionPool:             proposal.SortitionPool,
		BridgeCodeHash:            proposal.BridgeCodeHash,
		RegistryCodeHash:          proposal.RegistryCodeHash,
		CompleteRouterCodeHash:    proposal.CompleteRouterCodeHash,
		FrostRegistryCodeHash:     proposal.FrostRegistryCodeHash,
		ProposalValidatorCodeHash: proposal.ProposalValidatorCodeHash,
		SortitionPoolCodeHash:     proposal.SortitionPoolCodeHash,
		ReservationProtocolID:     proposal.ReservationProtocolID,
		EvidenceProtocolID:        proposal.EvidenceProtocolID,
		SigningPolicyHash:         proposal.SigningPolicyHash,
	}
	profile.ProfileHash = profile.ComputeHash()
	return profile
}

type countingFrostPreSignChainSigning struct {
	chain.Signing
	signCalls int
}

func (cfpscs *countingFrostPreSignChainSigning) Sign(message []byte) ([]byte, error) {
	cfpscs.signCalls++
	return cfpscs.Signing.Sign(message)
}

type testFrostPreSignAuthorizationBackend struct {
	proposal             *FrostPreSignAuthorizationProposal
	currentFinality      *FrostPreSignFinality
	currentFinalityCalls uint64
	states               map[uint64]*FrostPreSignAuthorizationState
}

func (tfpsab *testFrostPreSignAuthorizationBackend) PrepareFrostPreSignAuthorization(
	ctx context.Context,
	transaction *FrostPreSignTransaction,
	walletOperators []chain.Address,
) (*FrostPreSignAuthorizationProposal, error) {
	return tfpsab.proposal, nil
}

func (tfpsab *testFrostPreSignAuthorizationBackend) RelayFrostPreSignAuthorization(
	ctx context.Context,
	proposal *FrostPreSignAuthorizationProposal,
	attestation *FrostPreSignSeatAttestation,
) ([32]byte, error) {
	return [32]byte{}, nil
}

func (tfpsab *testFrostPreSignAuthorizationBackend) WaitForFrostPreSignAuthorizationFinality(
	ctx context.Context,
	relayTransactionHash [32]byte,
	proposal *FrostPreSignAuthorizationProposal,
) (*FrostPreSignFinality, error) {
	return nil, nil
}

func (tfpsab *testFrostPreSignAuthorizationBackend) CurrentFrostPreSignFinality(
	ctx context.Context,
) (*FrostPreSignFinality, error) {
	tfpsab.currentFinalityCalls++
	if tfpsab.currentFinality == nil {
		return nil, nil
	}
	result := *tfpsab.currentFinality
	return &result, nil
}

func (tfpsab *testFrostPreSignAuthorizationBackend) ReadFrostPreSignAuthorizationState(
	ctx context.Context,
	proposal *FrostPreSignAuthorizationProposal,
	finality FrostPreSignFinality,
) (*FrostPreSignAuthorizationState, error) {
	if tfpsab.states == nil || tfpsab.states[finality.BlockNumber] == nil {
		return nil, nil
	}
	result := *tfpsab.states[finality.BlockNumber]
	return &result, nil
}

// TestThresholdFrostPreSignAuthorizationGate_RefusesQuarantinedSigningWallet
// pins the scope of an active canonical quarantine on the signing path. Raising
// one is an authenticated operational stop for exactly one wallet, so that
// wallet must stop signing Bitcoin immediately, while wallets the record does
// not name keep working.
func TestThresholdFrostPreSignAuthorizationGate_RefusesQuarantinedSigningWallet(
	t *testing.T,
) {
	unsignedTx, _ := buildTaprootKeyPathUnsignedTxForTest(t)
	transaction := testFrostPreSignTransaction(t, unsignedTx)
	proposal := completeTestFrostPreSignProposal(transaction)
	relayFinality := FrostPreSignFinality{
		RelayTransactionHash:  [32]byte{0x71},
		BlockNumber:           100,
		BlockHash:             [32]byte{0x72},
		AuthorizationSequence: [32]byte{31: 1},
	}
	currentFinality := FrostPreSignFinality{
		BlockNumber: 101,
		BlockHash:   [32]byte{0x73},
	}
	tests := map[string]struct {
		walletID            [32]byte
		walletPublicKeyHash [20]byte
		accepted            bool
	}{
		"signing wallet quarantined": {
			walletID:            proposal.WalletID,
			walletPublicKeyHash: transaction.WalletPublicKeyHash,
		},
		"unrelated wallet quarantined": {
			walletID:            [32]byte{0x91},
			walletPublicKeyHash: [20]byte{0x92},
			accepted:            true,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			backend := &testFrostPreSignAuthorizationBackend{
				proposal:        proposal,
				currentFinality: &currentFinality,
				states: map[uint64]*FrostPreSignAuthorizationState{
					relayFinality.BlockNumber: completeTestFrostPreSignState(
						proposal,
						relayFinality,
					),
					currentFinality.BlockNumber: completeTestFrostPreSignState(
						proposal,
						currentFinality,
					),
				},
			}
			gate := &thresholdFrostPreSignAuthorizationGate{
				backend:           backend,
				activationProfile: activationProfileForTestProposal(proposal),
				storeBinding:      testFrostDurableSessionStoreBinding(t),
				productionReadiness: &testFrostProductionAuthorizationReadiness{
					activeQuarantines: []frostRetainedGroupActiveQuarantine{{
						QuarantineID:        [32]byte{0x51},
						WalletID:            test.walletID,
						WalletPublicKeyHash: test.walletPublicKeyHash,
						RecoveryRequired:    true,
					}},
				},
			}
			authorization := &frostPreSignAuthorization{
				ActivationProfileHash: gate.activationProfile.ProfileHash,
				AuthorizationID:       proposal.Digest,
				ReservationID:         proposal.ReservationID,
				VariantRoot:           proposal.AuthorizationRoot,
				TransactionHash:       proposal.Transaction.TransactionHash,
				Finality:              relayFinality,
				VariantSequence:       frostPreSignVariantSequence(relayFinality),
				proposal:              proposal,
			}
			err := gate.revalidate(context.Background(), authorization)
			if test.accepted {
				if err != nil {
					t.Fatalf(
						"unrelated wallet quarantine blocked signing: [%v]",
						err,
					)
				}
				return
			}
			if err == nil ||
				!strings.Contains(err.Error(), "active canonical quarantine") {
				t.Fatalf("quarantined wallet was authorized to sign: [%v]", err)
			}
		})
	}
}

func TestIsFrostPreSignTransientAuthorizationFailure(t *testing.T) {
	tests := map[string]struct {
		err       error
		transient bool
	}{
		"no failure": {},
		"observed authorization change": {
			err: fmt.Errorf(
				"FROST pre-sign authorization identity changed",
			),
		},
		"caller cancelled": {
			err: fmt.Errorf("readiness: [%w]", context.Canceled),
		},
		"history read timed out": {
			err: fmt.Errorf(
				"FROST production signer is not authorization-ready: [%w]",
				fmt.Errorf(
					"cannot read retained-group history page [3]: [%w]",
					context.DeadlineExceeded,
				),
			),
			transient: true,
		},
		"endpoint refused the connection": {
			err: fmt.Errorf("anchor read: [%w]", &stdnet.OpError{
				Op:  "dial",
				Net: "tcp",
				Err: syscall.ECONNREFUSED,
			}),
			transient: true,
		},
		"read deadline elapsed": {
			err:       fmt.Errorf("anchor read: [%w]", os.ErrDeadlineExceeded),
			transient: true,
		},
		"Ethereum RPC request timed out": {
			err: fmt.Errorf("Ethereum RPC: [%w]", ethereumRPC.HTTPError{
				StatusCode: http.StatusRequestTimeout,
				Status:     "408 Request Timeout",
			}),
			transient: true,
		},
		"Ethereum RPC rate limited": {
			err: fmt.Errorf("Ethereum RPC: [%w]", ethereumRPC.HTTPError{
				StatusCode: http.StatusTooManyRequests,
				Status:     "429 Too Many Requests",
			}),
			transient: true,
		},
		"Ethereum RPC service unavailable": {
			err: fmt.Errorf("Ethereum RPC: [%w]", ethereumRPC.HTTPError{
				StatusCode: http.StatusServiceUnavailable,
				Status:     "503 Service Unavailable",
			}),
			transient: true,
		},
		"Ethereum RPC authentication rejected": {
			err: fmt.Errorf("Ethereum RPC: [%w]", ethereumRPC.HTTPError{
				StatusCode: http.StatusUnauthorized,
				Status:     "401 Unauthorized",
			}),
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if got := isFrostPreSignTransientAuthorizationFailure(
				test.err,
			); got != test.transient {
				t.Fatalf(
					"classified [%v] as transient=[%t], want [%t]",
					test.err,
					got,
					test.transient,
				)
			}
		})
	}
}

// testFrostPreSignRevalidationFixture builds a gate and a matching finalized
// authorization whose revalidation passes, so a test only has to perturb the
// one dependency it is about.
func testFrostPreSignRevalidationFixture(
	t *testing.T,
	readiness *testFrostProductionAuthorizationReadiness,
) (
	*thresholdFrostPreSignAuthorizationGate,
	*frostPreSignAuthorization,
) {
	t.Helper()
	unsignedTx, _ := buildTaprootKeyPathUnsignedTxForTest(t)
	transaction := testFrostPreSignTransaction(t, unsignedTx)
	proposal := completeTestFrostPreSignProposal(transaction)
	relayFinality := FrostPreSignFinality{
		RelayTransactionHash:  [32]byte{0x71},
		BlockNumber:           100,
		BlockHash:             [32]byte{0x72},
		AuthorizationSequence: [32]byte{31: 1},
	}
	currentFinality := FrostPreSignFinality{
		BlockNumber: 101,
		BlockHash:   [32]byte{0x73},
	}
	backend := &testFrostPreSignAuthorizationBackend{
		proposal:        proposal,
		currentFinality: &currentFinality,
		states: map[uint64]*FrostPreSignAuthorizationState{
			relayFinality.BlockNumber: completeTestFrostPreSignState(
				proposal,
				relayFinality,
			),
			currentFinality.BlockNumber: completeTestFrostPreSignState(
				proposal,
				currentFinality,
			),
		},
	}
	gate := &thresholdFrostPreSignAuthorizationGate{
		backend:               backend,
		activationProfile:     activationProfileForTestProposal(proposal),
		storeBinding:          testFrostDurableSessionStoreBinding(t),
		productionReadiness:   readiness,
		transientRetryBudget:  time.Second,
		transientRetryBackoff: time.Millisecond,
	}
	authorization := &frostPreSignAuthorization{
		ActivationProfileHash: gate.activationProfile.ProfileHash,
		AuthorizationID:       proposal.Digest,
		ReservationID:         proposal.ReservationID,
		VariantRoot:           proposal.AuthorizationRoot,
		TransactionHash:       proposal.Transaction.TransactionHash,
		Finality:              relayFinality,
		VariantSequence:       frostPreSignVariantSequence(relayFinality),
		proposal:              proposal,
	}
	return gate, authorization
}

// TestThresholdFrostPreSignAuthorizationGate_SurvivesTransientReadinessFailure
// pins the monitor path against a single unreachable dependency. The monitor
// latches the first error revalidation returns and cancels the signing session
// permanently, so a readiness reconciliation that times out - which the
// cache-miss route performs over paginated network reads every time the
// finalized point advances - must not be reported as an authorization change.
func TestThresholdFrostPreSignAuthorizationGate_SurvivesTransientReadinessFailure(
	t *testing.T,
) {
	readiness := &testFrostProductionAuthorizationReadiness{
		err: fmt.Errorf(
			"cannot reconstruct complete FROST retained-group history: [%w]",
			context.DeadlineExceeded,
		),
		failingCalls: 2,
	}
	gate, authorization := testFrostPreSignRevalidationFixture(t, readiness)
	if err := gate.revalidate(
		context.Background(),
		authorization,
	); err != nil {
		t.Fatalf("a transient readiness failure killed the session: [%v]", err)
	}
	if readiness.calls != 3 {
		t.Fatalf(
			"expected the unreachable reconciliation to be retried, saw [%d] attempts",
			readiness.calls,
		)
	}
}

func TestThresholdFrostPreSignAuthorizationGate_SurvivesTemporaryEthereumHTTPFailure(
	t *testing.T,
) {
	for name, statusCode := range map[string]int{
		"request timeout":     http.StatusRequestTimeout,
		"rate limited":        http.StatusTooManyRequests,
		"service unavailable": http.StatusServiceUnavailable,
	} {
		t.Run(name, func(t *testing.T) {
			readiness := &testFrostProductionAuthorizationReadiness{
				err: fmt.Errorf("Ethereum RPC: [%w]", ethereumRPC.HTTPError{
					StatusCode: statusCode,
					Status:     http.StatusText(statusCode),
				}),
				failingCalls: 2,
			}
			gate, authorization := testFrostPreSignRevalidationFixture(t, readiness)
			if err := gate.revalidate(
				context.Background(),
				authorization,
			); err != nil {
				t.Fatalf("a temporary Ethereum HTTP failure killed the session: [%v]", err)
			}
			if readiness.calls != 3 {
				t.Fatalf(
					"expected the Ethereum HTTP failure to be retried, saw [%d] attempts",
					readiness.calls,
				)
			}
		})
	}
}

// TestThresholdFrostPreSignAuthorizationGate_LatchesObservedReadinessChange
// keeps the gate fail-closed. An authorization fact that was actually read and
// disagrees is deterministic, so it must be reported on the first attempt
// rather than retried into a delayed cancellation.
func TestThresholdFrostPreSignAuthorizationGate_LatchesObservedReadinessChange(
	t *testing.T,
) {
	readiness := &testFrostProductionAuthorizationReadiness{
		err: fmt.Errorf(
			"canonical FROST retained-group history rewrote, omitted, or reordered event [7]",
		),
	}
	gate, authorization := testFrostPreSignRevalidationFixture(t, readiness)
	err := gate.revalidate(context.Background(), authorization)
	if err == nil ||
		!strings.Contains(err.Error(), "rewrote, omitted, or reordered") {
		t.Fatalf("observed readiness change was not reported: [%v]", err)
	}
	if readiness.calls != 1 {
		t.Fatalf(
			"observed readiness change was retried [%d] times",
			readiness.calls,
		)
	}
}

// TestThresholdFrostPreSignAuthorizationGate_FailsClosedOnPersistentOutage
// bounds the tolerance. A dependency that stays unreachable is a dependency
// whose authorization facts cannot be checked at all, so the pass gives up and
// lets the monitor cancel the session.
func TestThresholdFrostPreSignAuthorizationGate_FailsClosedOnPersistentOutage(
	t *testing.T,
) {
	readiness := &testFrostProductionAuthorizationReadiness{
		err: fmt.Errorf("anchor read: [%w]", &stdnet.OpError{
			Op:  "dial",
			Net: "tcp",
			Err: syscall.ECONNREFUSED,
		}),
	}
	gate, authorization := testFrostPreSignRevalidationFixture(t, readiness)
	gate.transientRetryBudget = 20 * time.Millisecond
	err := gate.revalidate(context.Background(), authorization)
	if err == nil || !strings.Contains(err.Error(), "stayed unreachable") {
		t.Fatalf("persistent outage did not fail closed: [%v]", err)
	}
	if readiness.calls < 2 {
		t.Fatalf(
			"persistent outage was not retried at all, saw [%d] attempts",
			readiness.calls,
		)
	}
}

// TestThresholdFrostPreSignAuthorizationGate_StopsRetryingWhenCallerCancels
// keeps a cancelled caller from being held for the retry budget; the monitor
// treats its own cancellation separately and must not latch on it.
func TestThresholdFrostPreSignAuthorizationGate_StopsRetryingWhenCallerCancels(
	t *testing.T,
) {
	readiness := &testFrostProductionAuthorizationReadiness{
		err: fmt.Errorf("anchor read: [%w]", os.ErrDeadlineExceeded),
	}
	gate, authorization := testFrostPreSignRevalidationFixture(t, readiness)
	gate.transientRetryBudget = time.Minute
	gate.transientRetryBackoff = time.Hour
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	if err := gate.revalidate(ctx, authorization); err == nil {
		t.Fatal("cancelled revalidation reported success")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("cancelled revalidation waited out the backoff: [%s]", elapsed)
	}
}
