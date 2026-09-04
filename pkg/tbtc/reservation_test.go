package tbtc

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"reflect"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/tbtc/gen/pb"
)

func TestReservationActionTypes(t *testing.T) {
	for value, expected := range map[uint8]WalletActionType{
		6: ActionReservationAnchor,
		8: ActionReservationReanchor,
	} {
		parsed, err := ParseWalletActionType(value)
		if err != nil {
			t.Fatal(err)
		}
		if parsed != expected {
			t.Errorf(
				"unexpected action type for [%v]: expected [%v] got [%v]",
				value,
				expected,
				parsed,
			)
		}
	}
}

func TestReservationStateValues(t *testing.T) {
	tests := map[ReservationState]uint8{
		ReservationStateUnknown:       0,
		ReservationStateActive:        1,
		ReservationStateActionPending: 2,
		ReservationStateClosed:        3,
		ReservationStateStranded:      4,
	}

	for state, expected := range tests {
		if actual := uint8(state); actual != expected {
			t.Errorf(
				"unexpected reservation state value\nexpected: [%v]\nactual:   [%v]",
				expected,
				actual,
			)
		}
	}
}

func TestReservationActionTypeValues(t *testing.T) {
	tests := map[ReservationActionType]uint8{
		ReservationActionTypeNone:        0,
		ReservationActionTypeAcceptance:  1,
		ReservationActionTypeRedemption:  2,
		ReservationActionTypeReanchor:    3,
		ReservationActionTypeDissolution: 4,
	}

	for actionType, expected := range tests {
		if actual := uint8(actionType); actual != expected {
			t.Errorf(
				"unexpected reservation action type value\nexpected: [%v]\nactual:   [%v]",
				expected,
				actual,
			)
		}
	}
}

func TestReservationActionStateValues(t *testing.T) {
	tests := map[ReservationActionState]uint8{
		ReservationActionStateUnknown:    0,
		ReservationActionStatePending:    1,
		ReservationActionStateSettled:    2,
		ReservationActionStateTimedOut:   3,
		ReservationActionStateVetoed:     4,
		ReservationActionStateSuperseded: 5,
	}

	for state, expected := range tests {
		if actual := uint8(state); actual != expected {
			t.Errorf(
				"unexpected reservation action state value\nexpected: [%v]\nactual:   [%v]",
				expected,
				actual,
			)
		}
	}
}

func TestReservationProposals_MarshalingRoundtrip(t *testing.T) {
	anchorProposal := &ReservationAnchorProposal{
		DepositFundingTxHash:      bitcoin.Hash{0x01, 0x02},
		DepositFundingOutputIndex: 3,
		RequestNonce:              1,
		AnchorTxFee:               big.NewInt(1500),
	}

	reanchorProposal := &ReservationReanchorProposal{
		ReservationKey:            big.NewInt(54321),
		RequestNonce:              3,
		TargetWalletPublicKeyHash: [20]byte{0xaa, 0xbb},
		ReanchorTxFee:             big.NewInt(1700),
	}

	roundtrip := func(
		proposal CoordinationProposal,
		fresh CoordinationProposal,
	) {
		marshaled, err := proposal.Marshal()
		if err != nil {
			t.Fatal(err)
		}
		if err := fresh.Unmarshal(marshaled); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(proposal, fresh) {
			t.Errorf(
				"unexpected unmarshaled proposal: expected [%+v] got [%+v]",
				proposal,
				fresh,
			)
		}
	}

	roundtrip(anchorProposal, &ReservationAnchorProposal{})
	roundtrip(reanchorProposal, &ReservationReanchorProposal{})
}

func TestReservationProposals_UnmarshalRejectsInvalidPayloads(t *testing.T) {
	tests := map[string]struct {
		actionType    WalletActionType
		payload       []byte
		expectedError string
	}{
		// Proto3 scalar fields have no wire presence, so an entirely
		// empty payload and one with every field explicitly zeroed are
		// indistinguishable - a single "empty payload" case per type
		// covers what the old JSON test split into "empty object" and
		// "null payload" cases.
		"anchor empty payload": {
			actionType:    ActionReservationAnchor,
			payload:       marshalPb(t, &pb.ReservationAnchorProposal{}),
			expectedError: "cannot unmarshal proposal payload: [anchor transaction fee is required]",
		},
		"anchor null payload": {
			actionType:    ActionReservationAnchor,
			payload:       nil,
			expectedError: "cannot unmarshal proposal payload: [anchor transaction fee is required]",
		},
		"anchor missing nonce": {
			actionType: ActionReservationAnchor,
			payload: marshalPb(t, &pb.ReservationAnchorProposal{
				AnchorTxFee: big.NewInt(1500).Bytes(),
			}),
			expectedError: "cannot unmarshal proposal payload: [request nonce is required]",
		},
		"anchor invalid deposit funding tx hash length": {
			actionType: ActionReservationAnchor,
			payload: marshalPb(t, &pb.ReservationAnchorProposal{
				RequestNonce: 1,
				AnchorTxFee:  big.NewInt(1500).Bytes(),
			}),
			expectedError: "cannot unmarshal proposal payload: [invalid deposit funding tx hash length: [0]]",
		},
		"re-anchor null payload": {
			actionType:    ActionReservationReanchor,
			payload:       nil,
			expectedError: "cannot unmarshal proposal payload: [reservation key is required]",
		},
		"re-anchor missing nonce": {
			actionType: ActionReservationReanchor,
			payload: marshalPb(t, &pb.ReservationReanchorProposal{
				ReservationKey: big.NewInt(54321).Bytes(),
				ReanchorTxFee:  big.NewInt(1700).Bytes(),
			}),
			expectedError: "cannot unmarshal proposal payload: [request nonce is required]",
		},
		"re-anchor missing fee": {
			actionType: ActionReservationReanchor,
			payload: marshalPb(t, &pb.ReservationReanchorProposal{
				ReservationKey: big.NewInt(54321).Bytes(),
				RequestNonce:   3,
			}),
			expectedError: "cannot unmarshal proposal payload: [re-anchor transaction fee is required]",
		},
		"anchor fee exceeds 8 bytes": {
			actionType: ActionReservationAnchor,
			payload: marshalPb(t, &pb.ReservationAnchorProposal{
				AnchorTxFee:               []byte{1, 2, 3, 4, 5, 6, 7, 8, 9},
				RequestNonce:              1,
				DepositFundingTxHash:      make([]byte, 32),
				DepositFundingOutputIndex: 0,
			}),
			expectedError: "cannot unmarshal proposal payload: [invalid anchor transaction fee byte length: [9]]",
		},
		"re-anchor fee exceeds 8 bytes": {
			actionType: ActionReservationReanchor,
			payload: marshalPb(t, &pb.ReservationReanchorProposal{
				ReservationKey:            big.NewInt(54321).Bytes(),
				RequestNonce:              3,
				ReanchorTxFee:             []byte{1, 2, 3, 4, 5, 6, 7, 8, 9},
				TargetWalletPublicKeyHash: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20},
			}),
			expectedError: "cannot unmarshal proposal payload: [invalid re-anchor transaction fee byte length: [9]]",
		},
		"anchor zero fee marshaled through Marshal is rejected as missing": {
			actionType: ActionReservationAnchor,
			payload: marshalThroughProposal(t, &ReservationAnchorProposal{
				DepositFundingTxHash:      bitcoin.Hash{0x01, 0x02},
				DepositFundingOutputIndex: 3,
				RequestNonce:              1,
				AnchorTxFee:               big.NewInt(0),
			}),
			expectedError: "cannot unmarshal proposal payload: [anchor transaction fee is required]",
		},
		"re-anchor zero reservation key marshaled through Marshal is rejected as missing": {
			actionType: ActionReservationReanchor,
			payload: marshalThroughProposal(t, &ReservationReanchorProposal{
				ReservationKey:            big.NewInt(0),
				RequestNonce:              3,
				TargetWalletPublicKeyHash: [20]byte{0xaa, 0xbb},
				ReanchorTxFee:             big.NewInt(1700),
			}),
			expectedError: "cannot unmarshal proposal payload: [reservation key is required]",
		},
		"re-anchor zero fee marshaled through Marshal is rejected as missing": {
			actionType: ActionReservationReanchor,
			payload: marshalThroughProposal(t, &ReservationReanchorProposal{
				ReservationKey:            big.NewInt(54321),
				RequestNonce:              3,
				TargetWalletPublicKeyHash: [20]byte{0xaa, 0xbb},
				ReanchorTxFee:             big.NewInt(0),
			}),
			expectedError: "cannot unmarshal proposal payload: [re-anchor transaction fee is required]",
		},
		"re-anchor invalid target wallet hash length": {
			actionType: ActionReservationReanchor,
			payload: marshalPb(t, &pb.ReservationReanchorProposal{
				ReservationKey: big.NewInt(54321).Bytes(),
				RequestNonce:   3,
				ReanchorTxFee:  big.NewInt(1700).Bytes(),
			}),
			expectedError: "cannot unmarshal proposal payload: [invalid target wallet public key hash length: [0]]",
		},
		"re-anchor zero-value target wallet hash": {
			actionType: ActionReservationReanchor,
			payload: marshalPb(t, &pb.ReservationReanchorProposal{
				ReservationKey:            big.NewInt(54321).Bytes(),
				RequestNonce:              3,
				TargetWalletPublicKeyHash: make([]byte, 20),
				ReanchorTxFee:             big.NewInt(1700).Bytes(),
			}),
			expectedError: "cannot unmarshal proposal payload: [target wallet public key hash is required]",
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			_, err := unmarshalCoordinationProposal(
				uint32(test.actionType),
				test.payload,
			)
			if err == nil || err.Error() != test.expectedError {
				t.Errorf(
					"unexpected error\nexpected: [%v]\nactual:   [%v]",
					test.expectedError,
					err,
				)
			}
		})
	}
}

// marshalPb marshals a protobuf message for use as a test fixture payload.
func marshalPb(t *testing.T, msg proto.Message) []byte {
	t.Helper()
	data, err := proto.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// marshalThroughProposal marshals a CoordinationProposal via its own Marshal
// method, for use as a test fixture payload. Unlike marshalPb, this exercises
// the proposal's real wire-encoding path (e.g. *big.Int.Bytes()) rather than
// hand-constructing the protobuf message directly.
func marshalThroughProposal(t *testing.T, proposal CoordinationProposal) []byte {
	t.Helper()
	data, err := proposal.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func signReservationTransaction(
	t *testing.T,
	builder *bitcoin.TransactionBuilder,
	publicKey *ecdsa.PublicKey,
	privateKeyValue *big.Int,
) *bitcoin.Transaction {
	t.Helper()

	sigHashes, err := builder.ComputeSignatureHashes()
	if err != nil {
		t.Fatal(err)
	}

	privateKey := &ecdsa.PrivateKey{
		PublicKey: *publicKey,
		D:         privateKeyValue,
	}
	signatures := make([]*bitcoin.SignatureContainer, len(sigHashes))
	for i, sigHash := range sigHashes {
		r, s, err := ecdsa.Sign(rand.Reader, privateKey, sigHash.Bytes())
		if err != nil {
			t.Fatal(err)
		}
		signatures[i] = &bitcoin.SignatureContainer{
			R:         r,
			S:         s,
			PublicKey: publicKey,
		}
	}

	transaction, err := builder.AddSignatures(signatures)
	if err != nil {
		t.Fatal(err)
	}

	return transaction
}

func TestAssembleReservationTransactions_InputValidation(t *testing.T) {
	bitcoinChain := newLocalBitcoinChain()
	walletPublicKeyHash := [20]byte{0x01}

	anchorUtxo := &bitcoin.UnspentTransactionOutput{
		Outpoint: &bitcoin.TransactionOutpoint{
			TransactionHash: bitcoin.Hash{0x03},
			OutputIndex:     0,
		},
		Value: 100000,
	}

	assertError := func(err error, expected string) {
		if err == nil || err.Error() != expected {
			t.Errorf("expected error [%v], got [%v]", expected, err)
		}
	}

	var err error
	_, err = AssembleReservationAnchorTransaction(
		bitcoinChain,
		nil,
		walletPublicKeyHash,
		nil,
		1500,
	)
	assertError(err, "deposit is required")

	deposit := &Deposit{Utxo: anchorUtxo}

	_, err = AssembleReservationAnchorTransaction(
		bitcoinChain,
		deposit,
		walletPublicKeyHash,
		nil,
		1500,
	)
	assertError(err, "reservation action is required")

	_, err = AssembleReservationAnchorTransaction(
		bitcoinChain,
		deposit,
		walletPublicKeyHash,
		&ReservationAction{TxMaxFee: 2000},
		0,
	)
	assertError(err, "fee must be positive")

	_, err = AssembleReservationAnchorTransaction(
		bitcoinChain,
		deposit,
		walletPublicKeyHash,
		&ReservationAction{TxMaxFee: 1000},
		1500,
	)
	assertError(err, "fee exceeds the maximum allowed fee")

	_, err = AssembleReservationReanchorTransaction(
		bitcoinChain,
		nil,
		walletPublicKeyHash,
		nil,
		1500,
	)
	assertError(err, "anchor UTXO is required")

	_, err = AssembleReservationReanchorTransaction(
		bitcoinChain,
		anchorUtxo,
		walletPublicKeyHash,
		nil,
		1500,
	)
	assertError(err, "reservation action is required")

	_, err = AssembleReservationReanchorTransaction(
		bitcoinChain,
		anchorUtxo,
		walletPublicKeyHash,
		&ReservationAction{TxMaxFee: 2000},
		0,
	)
	assertError(err, "fee must be positive")

	_, err = AssembleReservationReanchorTransaction(
		bitcoinChain,
		anchorUtxo,
		walletPublicKeyHash,
		&ReservationAction{TxMaxFee: 1000},
		1500,
	)
	assertError(err, "fee exceeds the maximum allowed fee")
}

func TestAssembleReservationTransactions_FeeBoundaries(t *testing.T) {
	bitcoinChain := newLocalBitcoinChain()
	walletPublicKeyHash := [20]byte{0x01}

	// Anchor boundary
	_, err := AssembleReservationAnchorTransaction(
		bitcoinChain,
		&Deposit{Utxo: &bitcoin.UnspentTransactionOutput{Value: 100000}},
		walletPublicKeyHash,
		&ReservationAction{TxMaxFee: 200000},
		100000,
	)
	if err == nil || err.Error() != "transaction fee exceeds the deposit amount" {
		t.Errorf("expected error [transaction fee exceeds the deposit amount], got [%v]", err)
	}

	// Reanchor boundary
	_, err = AssembleReservationReanchorTransaction(
		bitcoinChain,
		&bitcoin.UnspentTransactionOutput{Value: 100000},
		walletPublicKeyHash,
		&ReservationAction{TxMaxFee: 200000},
		100000,
	)
	if err == nil || err.Error() != "transaction fee exceeds the anchor value" {
		t.Errorf("expected error [transaction fee exceeds the anchor value], got [%v]", err)
	}
}

// TestAssembleReservationTransactions_HappyPathShape verifies the actual
// shape of a successfully assembled reservation anchor/re-anchor
// transaction: exactly one input, exactly one output, the output value
// equal to input value minus fee, and a P2WPKH locking script paying the
// target wallet. Existing tests only exercise error/boundary paths; none
// assert the happy-path output shape.
func TestAssembleReservationTransactions_HappyPathShape(t *testing.T) {
	targetWalletPublicKeyHash := [20]byte{
		0x8d, 0xb5, 0x0e, 0xb5, 0x20, 0x63, 0xea, 0x9d, 0x98, 0xb3,
		0xea, 0xc9, 0x14, 0x89, 0xa9, 0x0f, 0x73, 0x89, 0x86, 0xf6,
	}
	const fee = int64(1500)
	const depositValue = int64(100000)

	expectedOutputScript, err := bitcoin.PayToWitnessPublicKeyHash(
		targetWalletPublicKeyHash,
	)
	if err != nil {
		t.Fatal(err)
	}

	btcecKey, err := btcec.NewPrivateKey(btcec.S256())
	if err != nil {
		t.Fatal(err)
	}
	signingKey := (*ecdsa.PrivateKey)(btcecKey)

	t.Run("anchor transaction", func(t *testing.T) {
		bitcoinChain := newLocalBitcoinChain()

		deposit := &Deposit{
			Depositor:           "0x934b98637ca318a4d6e7ca6ffd1690b8e77df637",
			WalletPublicKeyHash: [20]byte{0xaa},
			RefundPublicKeyHash: [20]byte{0xbb},
			RefundLocktime:      [4]byte{0x60, 0xbc, 0xea, 0x61},
		}
		depositScript, err := deposit.Script()
		if err != nil {
			t.Fatal(err)
		}
		scriptHash := sha256.Sum256(depositScript)
		fundingOutputScript, err := bitcoin.PayToWitnessScriptHash(scriptHash)
		if err != nil {
			t.Fatal(err)
		}

		fundingTx := &bitcoin.Transaction{
			Outputs: []*bitcoin.TransactionOutput{{
				Value:           depositValue,
				PublicKeyScript: fundingOutputScript,
			}},
		}
		if err := bitcoinChain.BroadcastTransaction(fundingTx); err != nil {
			t.Fatal(err)
		}

		deposit.Utxo = &bitcoin.UnspentTransactionOutput{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: fundingTx.Hash(),
				OutputIndex:     0,
			},
			Value: depositValue,
		}

		builder, err := AssembleReservationAnchorTransaction(
			bitcoinChain,
			deposit,
			targetWalletPublicKeyHash,
			&ReservationAction{TxMaxFee: 2000},
			fee,
		)
		if err != nil {
			t.Fatal(err)
		}

		signedTx := signReservationTransaction(
			t,
			builder,
			&signingKey.PublicKey,
			signingKey.D,
		)

		assertReservationTransactionShape(
			t,
			signedTx,
			depositValue,
			fee,
			expectedOutputScript,
		)
	})

	t.Run("re-anchor transaction", func(t *testing.T) {
		bitcoinChain := newLocalBitcoinChain()

		anchorOutputScript, err := bitcoin.PayToWitnessPublicKeyHash(
			[20]byte{0xcc},
		)
		if err != nil {
			t.Fatal(err)
		}

		anchorFundingTx := &bitcoin.Transaction{
			Outputs: []*bitcoin.TransactionOutput{{
				Value:           depositValue,
				PublicKeyScript: anchorOutputScript,
			}},
		}
		if err := bitcoinChain.BroadcastTransaction(anchorFundingTx); err != nil {
			t.Fatal(err)
		}

		anchorUtxo := &bitcoin.UnspentTransactionOutput{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: anchorFundingTx.Hash(),
				OutputIndex:     0,
			},
			Value: depositValue,
		}

		builder, err := AssembleReservationReanchorTransaction(
			bitcoinChain,
			anchorUtxo,
			targetWalletPublicKeyHash,
			&ReservationAction{TxMaxFee: 2000},
			fee,
		)
		if err != nil {
			t.Fatal(err)
		}

		signedTx := signReservationTransaction(
			t,
			builder,
			&signingKey.PublicKey,
			signingKey.D,
		)

		assertReservationTransactionShape(
			t,
			signedTx,
			depositValue,
			fee,
			expectedOutputScript,
		)
	})
}

// assertReservationTransactionShape asserts the invariants a successfully
// assembled and signed reservation anchor/re-anchor transaction must
// satisfy: exactly 1 input, exactly 1 output, output value == inputValue -
// fee, and the output's locking script matches expectedOutputScript exactly.
func assertReservationTransactionShape(
	t *testing.T,
	transaction *bitcoin.Transaction,
	inputValue int64,
	fee int64,
	expectedOutputScript bitcoin.Script,
) {
	t.Helper()

	if len(transaction.Inputs) != 1 {
		t.Fatalf("expected exactly 1 input, got %d", len(transaction.Inputs))
	}
	if len(transaction.Outputs) != 1 {
		t.Fatalf("expected exactly 1 output, got %d", len(transaction.Outputs))
	}

	expectedValue := inputValue - fee
	if transaction.Outputs[0].Value != expectedValue {
		t.Errorf(
			"unexpected output value\nexpected: %d\nactual:   %d",
			expectedValue,
			transaction.Outputs[0].Value,
		)
	}

	if !reflect.DeepEqual(
		[]byte(transaction.Outputs[0].PublicKeyScript),
		[]byte(expectedOutputScript),
	) {
		t.Errorf(
			"unexpected output locking script\nexpected: %x\nactual:   %x",
			expectedOutputScript,
			transaction.Outputs[0].PublicKeyScript,
		)
	}
}

// reservationTestWallet returns a wallet with a real ECDSA public key so
// bitcoin.PublicKeyHash (called at the top of both execute() methods)
// doesn't panic on a nil key.
func reservationTestWallet(t *testing.T) wallet {
	t.Helper()

	publicKeyBytes, err := hex.DecodeString(
		"0471e30bca60f6548d7b42582a478ea37ada63b402af7b3ddd57f0c95bb6843175" +
			"aa0d2053a91a050a6797d85c38f2909cb7027f2344a01986aa2f9f8ca7a0c289",
	)
	if err != nil {
		t.Fatal(err)
	}

	return wallet{publicKey: mustUnmarshalPublicKey(t, publicKeyBytes)}
}

func TestReservationAnchorAction_Execute(t *testing.T) {
	const fundingOutputIndex = 0

	custodyWallet := reservationTestWallet(t)
	walletPublicKeyHash := bitcoin.PublicKeyHash(custodyWallet.publicKey)

	newAction := func(
		chain Chain,
		btcChain bitcoin.Chain,
		fundingTxHash bitcoin.Hash,
	) *reservationAnchorAction {
		return newReservationAnchorAction(
			zap.NewNop().Sugar(),
			chain,
			btcChain,
			custodyWallet,
			nil, // signing executor unreached by these negative-path cases
			&ReservationAnchorProposal{
				DepositFundingTxHash:      fundingTxHash,
				DepositFundingOutputIndex: fundingOutputIndex,
				RequestNonce:              1,
				AnchorTxFee:               big.NewInt(1500),
			},
			300000,
			300000+600,
			nil,
			nil,
		)
	}

	t.Run("no matching DepositRevealed event", func(t *testing.T) {
		chain := Connect()
		btcChain := newLocalBitcoinChain()

		fundingTx := &bitcoin.Transaction{
			Outputs: []*bitcoin.TransactionOutput{{Value: 100000}},
		}
		if err := btcChain.BroadcastTransaction(fundingTx); err != nil {
			t.Fatal(err)
		}
		fundingTxHash := fundingTx.Hash()

		// A DepositRevealed event exists for this wallet, but for a
		// different funding outpoint - the matching loop must walk past
		// it and still report no match, not silently accept it.
		if err := chain.setPastDepositRevealedEvents(
			&DepositRevealedEventFilter{
				WalletPublicKeyHash: [][20]byte{walletPublicKeyHash},
				StartBlock:          300000 - reservationLookBackBlocks,
			},
			[]*DepositRevealedEvent{{
				FundingTxHash:       bitcoin.Hash{0x99},
				FundingOutputIndex:  0,
				WalletPublicKeyHash: walletPublicKeyHash,
			}},
		); err != nil {
			t.Fatal(err)
		}

		err := newAction(chain, btcChain, fundingTxHash).execute()
		if err == nil || err.Error() != "no matching DepositRevealed event for deposit" {
			t.Errorf(
				"unexpected error\nexpected: [no matching DepositRevealed event for deposit]\nactual:   [%v]",
				err,
			)
		}
	})

	t.Run("deposit request not found", func(t *testing.T) {
		chain := Connect()
		btcChain := newLocalBitcoinChain()

		fundingTx := &bitcoin.Transaction{
			Outputs: []*bitcoin.TransactionOutput{{Value: 100000}},
		}
		if err := btcChain.BroadcastTransaction(fundingTx); err != nil {
			t.Fatal(err)
		}
		fundingTxHash := fundingTx.Hash()

		if err := chain.setPastDepositRevealedEvents(
			&DepositRevealedEventFilter{
				WalletPublicKeyHash: [][20]byte{walletPublicKeyHash},
				StartBlock:          300000 - reservationLookBackBlocks,
			},
			[]*DepositRevealedEvent{{
				FundingTxHash:       fundingTxHash,
				FundingOutputIndex:  fundingOutputIndex,
				WalletPublicKeyHash: walletPublicKeyHash,
			}},
		); err != nil {
			t.Fatal(err)
		}
		// Deliberately no setDepositRequest call: the Bridge has no
		// request record for this funding outpoint.

		err := newAction(chain, btcChain, fundingTxHash).execute()
		if err == nil || err.Error() != "deposit request not found" {
			t.Errorf(
				"unexpected error\nexpected: [deposit request not found]\nactual:   [%v]",
				err,
			)
		}
	})

	t.Run("full happy path up to the signing boundary", func(t *testing.T) {
		chain := Connect()
		btcChain := newLocalBitcoinChain()

		depositForScript := &Deposit{
			Depositor:           "0x0000000000000000000000000000000000000001",
			WalletPublicKeyHash: walletPublicKeyHash,
		}
		depositScript, err := depositForScript.Script()
		if err != nil {
			t.Fatal(err)
		}
		scriptHash := sha256.Sum256(depositScript)
		fundingOutputScript, err := bitcoin.PayToWitnessScriptHash(scriptHash)
		if err != nil {
			t.Fatal(err)
		}

		fundingTx := &bitcoin.Transaction{
			Outputs: []*bitcoin.TransactionOutput{{
				Value:           100000,
				PublicKeyScript: fundingOutputScript,
			}},
		}
		if err := btcChain.BroadcastTransaction(fundingTx); err != nil {
			t.Fatal(err)
		}
		fundingTxHash := fundingTx.Hash()

		if err := chain.setPastDepositRevealedEvents(
			&DepositRevealedEventFilter{
				WalletPublicKeyHash: [][20]byte{walletPublicKeyHash},
				StartBlock:          300000 - reservationLookBackBlocks,
			},
			[]*DepositRevealedEvent{{
				FundingTxHash:       fundingTxHash,
				FundingOutputIndex:  fundingOutputIndex,
				WalletPublicKeyHash: walletPublicKeyHash,
				Amount:              100000,
				Depositor:           "0x0000000000000000000000000000000000000001",
			}},
		); err != nil {
			t.Fatal(err)
		}
		chain.setDepositRequest(fundingTxHash, fundingOutputIndex, &DepositChainRequest{
			Amount:     100000,
			RevealedAt: time.Now(),
		})
		chain.setReservationAction(&ReservationAction{
			ActionType:                ReservationActionTypeAcceptance,
			State:                     ReservationActionStatePending,
			TargetWalletPublicKeyHash: walletPublicKeyHash,
			TxMaxFee:                  2000,
		})

		action := newAction(chain, btcChain, fundingTxHash)
		// Below reservationActionSigningTimeoutSafetyMarginBlocks (300):
		// every real upstream step (event match, deposit request fetch,
		// reservation key derivation, action load, target wallet match,
		// on-chain validation, transaction assembly) must succeed before
		// this guard is reached and rejects the proposal - reaching this
		// exact error is the test's proof that all of it worked.
		action.expiryBlock = 100

		err = action.execute()
		if err == nil || err.Error() != "invalid proposal expiry block" {
			t.Errorf(
				"unexpected error\nexpected: [invalid proposal expiry block]\nactual:   [%v]",
				err,
			)
		}
	})

	t.Run("target wallet mismatch is rejected before signing", func(t *testing.T) {
		chain := Connect()
		btcChain := newLocalBitcoinChain()

		depositForScript := &Deposit{
			Depositor:           "0x0000000000000000000000000000000000000001",
			WalletPublicKeyHash: walletPublicKeyHash,
		}
		depositScript, err := depositForScript.Script()
		if err != nil {
			t.Fatal(err)
		}
		scriptHash := sha256.Sum256(depositScript)
		fundingOutputScript, err := bitcoin.PayToWitnessScriptHash(scriptHash)
		if err != nil {
			t.Fatal(err)
		}

		fundingTx := &bitcoin.Transaction{
			Outputs: []*bitcoin.TransactionOutput{{
				Value:           100000,
				PublicKeyScript: fundingOutputScript,
			}},
		}
		if err := btcChain.BroadcastTransaction(fundingTx); err != nil {
			t.Fatal(err)
		}
		fundingTxHash := fundingTx.Hash()

		if err := chain.setPastDepositRevealedEvents(
			&DepositRevealedEventFilter{
				WalletPublicKeyHash: [][20]byte{walletPublicKeyHash},
				StartBlock:          300000 - reservationLookBackBlocks,
			},
			[]*DepositRevealedEvent{{
				FundingTxHash:       fundingTxHash,
				FundingOutputIndex:  fundingOutputIndex,
				WalletPublicKeyHash: walletPublicKeyHash,
				Amount:              100000,
				Depositor:           "0x0000000000000000000000000000000000000001",
			}},
		); err != nil {
			t.Fatal(err)
		}
		chain.setDepositRequest(fundingTxHash, fundingOutputIndex, &DepositChainRequest{
			Amount:     100000,
			RevealedAt: time.Now(),
		})
		chain.setReservationAction(&ReservationAction{
			ActionType:                ReservationActionTypeAcceptance,
			State:                     ReservationActionStatePending,
			TargetWalletPublicKeyHash: [20]byte{0xff}, // does not match the signing wallet
			TxMaxFee:                  2000,
		})

		action := newAction(chain, btcChain, fundingTxHash)
		action.expiryBlock = 100

		err = action.execute()
		if err == nil || err.Error() != "reservation action targets a different wallet" {
			t.Errorf(
				"unexpected error\nexpected: [reservation action targets a different wallet]\nactual:   [%v]",
				err,
			)
		}
	})
}

func TestReservationReanchorAction_Execute(t *testing.T) {
	custodyWallet := reservationTestWallet(t)
	walletPublicKeyHash := bitcoin.PublicKeyHash(custodyWallet.publicKey)

	reservationKey := big.NewInt(777)

	newAction := func(
		chain Chain,
		btcChain bitcoin.Chain,
	) *reservationReanchorAction {
		return newReservationReanchorAction(
			zap.NewNop().Sugar(),
			chain,
			btcChain,
			custodyWallet,
			nil, // signing executor unreached by these negative-path cases
			&ReservationReanchorProposal{
				ReservationKey:            reservationKey,
				RequestNonce:              1,
				TargetWalletPublicKeyHash: walletPublicKeyHash,
				ReanchorTxFee:             big.NewInt(1500),
			},
			300000,
			100, // below reservationActionSigningTimeoutSafetyMarginBlocks
			nil,
			nil,
		)
	}

	t.Run("full happy path up to the signing boundary", func(t *testing.T) {
		chain := Connect()
		btcChain := newLocalBitcoinChain()

		anchorOutputScript, err := bitcoin.PayToWitnessPublicKeyHash(walletPublicKeyHash)
		if err != nil {
			t.Fatal(err)
		}
		priorAnchorTx := &bitcoin.Transaction{
			Outputs: []*bitcoin.TransactionOutput{
				{Value: 10000},
				{Value: 100000, PublicKeyScript: anchorOutputScript},
			},
		}
		if err := btcChain.BroadcastTransaction(priorAnchorTx); err != nil {
			t.Fatal(err)
		}

		chain.setReservation(&Reservation{
			WalletPublicKeyHash: walletPublicKeyHash,
			AnchorUtxo: &bitcoin.UnspentTransactionOutput{
				Outpoint: &bitcoin.TransactionOutpoint{
					TransactionHash: priorAnchorTx.Hash(),
					OutputIndex:     1,
				},
				Value: 100000,
			},
		})
		chain.setReservationAction(&ReservationAction{
			ActionType:                ReservationActionTypeReanchor,
			State:                     ReservationActionStatePending,
			TargetWalletPublicKeyHash: walletPublicKeyHash,
			TxMaxFee:                  2000,
		})

		// Every real upstream step (reservation load, action load,
		// type/state check, target wallet match, on-chain validation,
		// transaction assembly) must succeed before the expiry-block
		// guard is reached and rejects the proposal - reaching this
		// exact error is the test's proof that all of it worked.
		err = newAction(chain, btcChain).execute()
		if err == nil || err.Error() != "invalid proposal expiry block" {
			t.Errorf(
				"unexpected error\nexpected: [invalid proposal expiry block]\nactual:   [%v]",
				err,
			)
		}
	})

	t.Run("target wallet mismatch is rejected before signing", func(t *testing.T) {
		chain := Connect()
		btcChain := newLocalBitcoinChain()

		chain.setReservation(&Reservation{
			WalletPublicKeyHash: walletPublicKeyHash,
			AnchorUtxo: &bitcoin.UnspentTransactionOutput{
				Outpoint: &bitcoin.TransactionOutpoint{
					TransactionHash: bitcoin.Hash{0x01},
					OutputIndex:     0,
				},
				Value: 100000,
			},
		})
		chain.setReservationAction(&ReservationAction{
			ActionType:                ReservationActionTypeReanchor,
			State:                     ReservationActionStatePending,
			TargetWalletPublicKeyHash: [20]byte{0xff}, // does not match the proposal's target
			TxMaxFee:                  2000,
		})

		err := newAction(chain, btcChain).execute()
		if err == nil || err.Error() != "reservation action targets a different wallet" {
			t.Errorf(
				"unexpected error\nexpected: [reservation action targets a different wallet]\nactual:   [%v]",
				err,
			)
		}
	})
}

// TestAssembleReservationAnchorTransaction verifies the happy-path output
// shape of AssembleReservationAnchorTransaction: a 1-input-1-output
// transaction spending the reserved deposit's P2WSH UTXO into a single
// P2WPKH output controlled by the target wallet, valued at the deposit
// amount less the transaction fee. Prior to this test, existing coverage
// (TestAssembleReservationTransactions_InputValidation,
// TestAssembleReservationTransactions_FeeBoundaries) exercised only
// validation-error and fee-boundary-error paths; no test asserted the
// happy-path output shape.
func TestAssembleReservationAnchorTransaction(t *testing.T) {
	bitcoinChain := newLocalBitcoinChain()

	privateKeyValue := big.NewInt(100)
	testWallet := generateWallet(privateKeyValue)
	walletPublicKeyHash := bitcoin.PublicKeyHash(testWallet.publicKey)

	targetPrivateKeyValue := big.NewInt(200)
	targetWallet := generateWallet(targetPrivateKeyValue)
	targetWalletPublicKeyHash := bitcoin.PublicKeyHash(targetWallet.publicKey)
	targetWalletScript, err := bitcoin.PayToWitnessPublicKeyHash(targetWalletPublicKeyHash)
	if err != nil {
		t.Fatal(err)
	}

	deposit := &Deposit{
		Depositor:           chain.Address("0x1111111111111111111111111111111111111111"),
		BlindingFactor:      [8]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
		WalletPublicKeyHash: walletPublicKeyHash,
		RefundPublicKeyHash: [20]byte{0x02},
		RefundLocktime:      [4]byte{0x03, 0x04, 0x05, 0x06},
	}

	depositScript, err := deposit.Script()
	if err != nil {
		t.Fatal(err)
	}

	depositScriptHash := sha256.Sum256(depositScript)
	depositLockingScript, err := bitcoin.PayToWitnessScriptHash(depositScriptHash)
	if err != nil {
		t.Fatal(err)
	}

	fundingTransaction := &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{
			{
				Outpoint: &bitcoin.TransactionOutpoint{
					TransactionHash: bitcoin.Hash{0x09},
					OutputIndex:     0,
				},
				Sequence: 0xffffffff,
			},
		},
		Outputs: []*bitcoin.TransactionOutput{
			{
				Value:           100000,
				PublicKeyScript: depositLockingScript,
			},
		},
	}
	if err := bitcoinChain.BroadcastTransaction(fundingTransaction); err != nil {
		t.Fatal(err)
	}

	deposit.Utxo = &bitcoin.UnspentTransactionOutput{
		Outpoint: &bitcoin.TransactionOutpoint{
			TransactionHash: fundingTransaction.Hash(),
			OutputIndex:     0,
		},
		Value: 100000,
	}

	builder, err := AssembleReservationAnchorTransaction(
		bitcoinChain,
		deposit,
		targetWalletPublicKeyHash,
		&ReservationAction{TxMaxFee: 1500},
		1500,
	)
	if err != nil {
		t.Fatal(err)
	}

	transaction := signReservationTransaction(
		t,
		builder,
		testWallet.publicKey,
		privateKeyValue,
	)

	expectedOutputs := []*bitcoin.TransactionOutput{
		{
			Value:           98500,
			PublicKeyScript: targetWalletScript,
		},
	}

	if !reflect.DeepEqual(expectedOutputs, transaction.Outputs) {
		t.Errorf(
			"unexpected outputs\nexpected: [%+v]\nactual:   [%+v]",
			expectedOutputs,
			transaction.Outputs,
		)
	}

	testutils.AssertIntsEqual(t, "inputs count", 1, len(transaction.Inputs))
}

// TestAssembleReservationReanchorTransaction verifies the happy-path output
// shape of AssembleReservationReanchorTransaction: a 1-input-1-output
// transaction spending the reservation's anchor UTXO into a single P2WPKH
// output controlled by the target wallet, valued at the anchor amount less
// the transaction fee. Prior to this test, existing coverage
// (TestAssembleReservationTransactions_InputValidation,
// TestAssembleReservationTransactions_FeeBoundaries) exercised only
// validation-error and fee-boundary-error paths; no test asserted the
// happy-path output shape. Note that pkg/tbtcpg does not yet exercise the
// reanchor assembly path via this function.
func TestAssembleReservationReanchorTransaction(t *testing.T) {
	bitcoinChain := newLocalBitcoinChain()

	privateKeyValue := big.NewInt(100)
	testWallet := generateWallet(privateKeyValue)
	sourceWalletPublicKeyHash := bitcoin.PublicKeyHash(testWallet.publicKey)
	sourceWalletScript, err := bitcoin.PayToWitnessPublicKeyHash(sourceWalletPublicKeyHash)
	if err != nil {
		t.Fatal(err)
	}

	targetPrivateKeyValue := big.NewInt(200)
	targetWallet := generateWallet(targetPrivateKeyValue)
	targetWalletPublicKeyHash := bitcoin.PublicKeyHash(targetWallet.publicKey)
	targetWalletScript, err := bitcoin.PayToWitnessPublicKeyHash(targetWalletPublicKeyHash)
	if err != nil {
		t.Fatal(err)
	}

	fundingTransaction := &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{
			{
				Outpoint: &bitcoin.TransactionOutpoint{
					TransactionHash: bitcoin.Hash{0x0a},
					OutputIndex:     0,
				},
				Sequence: 0xffffffff,
			},
		},
		Outputs: []*bitcoin.TransactionOutput{
			{
				Value:           100000,
				PublicKeyScript: sourceWalletScript,
			},
		},
	}
	if err := bitcoinChain.BroadcastTransaction(fundingTransaction); err != nil {
		t.Fatal(err)
	}

	anchorUtxo := &bitcoin.UnspentTransactionOutput{
		Outpoint: &bitcoin.TransactionOutpoint{
			TransactionHash: fundingTransaction.Hash(),
			OutputIndex:     0,
		},
		Value: 100000,
	}

	builder, err := AssembleReservationReanchorTransaction(
		bitcoinChain,
		anchorUtxo,
		targetWalletPublicKeyHash,
		&ReservationAction{TxMaxFee: 1500},
		1500,
	)
	if err != nil {
		t.Fatal(err)
	}

	transaction := signReservationTransaction(
		t,
		builder,
		testWallet.publicKey,
		privateKeyValue,
	)

	expectedOutputs := []*bitcoin.TransactionOutput{
		{
			Value:           98500,
			PublicKeyScript: targetWalletScript,
		},
	}

	if !reflect.DeepEqual(expectedOutputs, transaction.Outputs) {
		t.Errorf(
			"unexpected outputs\nexpected: [%+v]\nactual:   [%+v]",
			expectedOutputs,
			transaction.Outputs,
		)
	}

	testutils.AssertIntsEqual(t, "inputs count", 1, len(transaction.Inputs))
}
