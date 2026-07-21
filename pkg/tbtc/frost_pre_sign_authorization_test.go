package tbtc

import (
	"context"
	"encoding/hex"
	"math/big"
	"strings"
	"sync"
	"testing"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

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
		backend:           backend,
		activationProfile: activationProfileForTestProposal(proposal),
		storeBinding:      testFrostDurableSessionStoreBinding(t),
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
	gate := &thresholdFrostPreSignAuthorizationGate{
		backend:           &testFrostPreSignAuthorizationBackend{proposal: proposal},
		activationProfile: profile,
		storeBinding:      testFrostDurableSessionStoreBinding(t),
		signing:           countingSigner,
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

type testFrostPreSignAuthorizationGate struct {
	mutex           sync.Mutex
	authorizeErr    error
	revalidateErr   error
	authorizeCalls  int
	revalidateCalls int
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
	proposal        *FrostPreSignAuthorizationProposal
	currentFinality *FrostPreSignFinality
	states          map[uint64]*FrostPreSignAuthorizationState
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
