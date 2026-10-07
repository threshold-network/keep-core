package tbtc

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/keep-network/keep-core/pkg/bitcoin"
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
	// marshalPb encodes an arbitrary protobuf message the same way
	// proto.Marshal would, for building deliberately incomplete/invalid
	// wire payloads. mustMarshal panics on error since every message
	// here is well-formed at the protobuf level - only the domain-level
	// validation performed by each proposal's Unmarshal is under test.
	marshalPb := func(msg proto.Message) []byte {
		bytes, err := proto.Marshal(msg)
		if err != nil {
			t.Fatal(err)
		}
		return bytes
	}

	validHash := make([]byte, 32)
	validHash[0] = 0x01
	validWalletHash := make([]byte, 20)
	validWalletHash[0] = 0xaa
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
			payload:       marshalPb(&pb.ReservationAnchorProposal{}),
			expectedError: "cannot unmarshal proposal payload: [anchor transaction fee is required]",
		},
		"anchor null payload": {
			actionType:    ActionReservationAnchor,
			payload:       nil,
			expectedError: "cannot unmarshal proposal payload: [anchor transaction fee is required]",
		},
		"anchor missing nonce": {
			actionType: ActionReservationAnchor,
			payload: marshalPb(&pb.ReservationAnchorProposal{
				DepositFundingTxHash: validHash,
				AnchorTxFee:          big.NewInt(1500).Bytes(),
			}),
			expectedError: "cannot unmarshal proposal payload: [request nonce is required]",
		},
		"anchor missing fee": {
			actionType: ActionReservationAnchor,
			payload: marshalPb(&pb.ReservationAnchorProposal{
				DepositFundingTxHash: validHash,
				RequestNonce:         1,
			}),
			expectedError: "cannot unmarshal proposal payload: [anchor transaction fee is required]",
		},
		"anchor invalid deposit funding tx hash length": {
			actionType: ActionReservationAnchor,
			payload: marshalPb(&pb.ReservationAnchorProposal{
				RequestNonce: 1,
				AnchorTxFee:  big.NewInt(1500).Bytes(),
			}),
			expectedError: "cannot unmarshal proposal payload: [invalid deposit funding tx hash length: [0]]",
		},
		"re-anchor empty payload": {
			actionType:    ActionReservationReanchor,
			payload:       marshalPb(&pb.ReservationReanchorProposal{}),
			expectedError: "cannot unmarshal proposal payload: [reservation key is required]",
		},
		"re-anchor invalid target wallet hash length": {
			actionType: ActionReservationReanchor,
			payload: marshalPb(&pb.ReservationReanchorProposal{
				ReservationKey: big.NewInt(54321).Bytes(),
				RequestNonce:   3,
				ReanchorTxFee:  big.NewInt(1700).Bytes(),
			}),
			expectedError: "cannot unmarshal proposal payload: [invalid target wallet public key hash length: [0]]",
		},
		"re-anchor missing nonce": {
			actionType: ActionReservationReanchor,
			payload: marshalPb(&pb.ReservationReanchorProposal{
				ReservationKey:            big.NewInt(54321).Bytes(),
				TargetWalletPublicKeyHash: validWalletHash,
				ReanchorTxFee:             big.NewInt(1700).Bytes(),
			}),
			expectedError: "cannot unmarshal proposal payload: [request nonce is required]",
		},
		"re-anchor missing fee": {
			actionType: ActionReservationReanchor,
			payload: marshalPb(&pb.ReservationReanchorProposal{
				ReservationKey:            big.NewInt(54321).Bytes(),
				TargetWalletPublicKeyHash: validWalletHash,
				RequestNonce:              3,
			}),
			expectedError: "cannot unmarshal proposal payload: [re-anchor transaction fee is required]",
		},
		"anchor fee exceeds 8 bytes": {
			actionType: ActionReservationAnchor,
			payload: marshalPb(&pb.ReservationAnchorProposal{
				AnchorTxFee:               []byte{1, 2, 3, 4, 5, 6, 7, 8, 9},
				RequestNonce:              1,
				DepositFundingTxHash:      make([]byte, 32),
				DepositFundingOutputIndex: 0,
			}),
			expectedError: "cannot unmarshal proposal payload: [invalid anchor transaction fee byte length: [9]]",
		},
		"re-anchor fee exceeds 8 bytes": {
			actionType: ActionReservationReanchor,
			payload: marshalPb(&pb.ReservationReanchorProposal{
				ReservationKey:            big.NewInt(54321).Bytes(),
				RequestNonce:              3,
				ReanchorTxFee:             []byte{1, 2, 3, 4, 5, 6, 7, 8, 9},
				TargetWalletPublicKeyHash: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20},
			}),
			expectedError: "cannot unmarshal proposal payload: [invalid re-anchor transaction fee byte length: [9]]",
		},
		"re-anchor zero-value target wallet hash": {
			actionType: ActionReservationReanchor,
			payload: marshalPb(&pb.ReservationReanchorProposal{
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

// TestReservationProposals_MarshalRejectsZeroValues asserts that the
// tightened Marshal validation rejects zero-value fees and reservation
// keys directly, before any bytes are ever produced - closing the
// producer-side gap where a zero-fee/zero-key proposal previously marshaled
// successfully and was only rejected by every follower downstream.
// (The UnmarshalRejects* tests above cannot cover this because the shared
// marshalThroughProposal helper Fatalf's on any Marshal error.)
func TestReservationProposals_MarshalRejectsZeroValues(t *testing.T) {
	validHash := bitcoin.Hash{0x01, 0x02}
	validWalletHash := [20]byte{0xaa, 0xbb}

	tests := map[string]struct {
		proposal      CoordinationProposal
		expectedError string
	}{
		"anchor zero fee rejected by Marshal": {
			proposal: &ReservationAnchorProposal{
				DepositFundingTxHash:      validHash,
				DepositFundingOutputIndex: 3,
				RequestNonce:              1,
				AnchorTxFee:               big.NewInt(0),
			},
			expectedError: "anchor transaction fee must be positive",
		},
		"re-anchor zero reservation key rejected by Marshal": {
			proposal: &ReservationReanchorProposal{
				ReservationKey:            big.NewInt(0),
				RequestNonce:              3,
				TargetWalletPublicKeyHash: validWalletHash,
				ReanchorTxFee:             big.NewInt(1700),
			},
			expectedError: "reservation key is required",
		},
		"re-anchor zero fee rejected by Marshal": {
			proposal: &ReservationReanchorProposal{
				ReservationKey:            big.NewInt(54321),
				RequestNonce:              3,
				TargetWalletPublicKeyHash: validWalletHash,
				ReanchorTxFee:             big.NewInt(0),
			},
			expectedError: "re-anchor transaction fee must be positive",
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			_, err := test.proposal.Marshal()
			if err == nil || err.Error() != test.expectedError {
				t.Errorf(
					"unexpected Marshal error\nexpected: [%v]\nactual:   [%v]",
					test.expectedError,
					err,
				)
			}
		})
	}
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

// newAnchorActionWithFee builds an anchor action over a chain on which every
// read and the on-chain validation of the anchor execute path succeed (the
// same setup as the happy path of TestReservationAnchorAction_Execute), with
// the given anchor fee. The low expiry block makes execute stop with
// "invalid proposal expiry block" right before signing, so reaching that
// error proves the fee gate passed.
func newAnchorActionWithFee(t *testing.T, fee *big.Int) *reservationAnchorAction {
	t.Helper()

	const fundingOutputIndex = 0

	custodyWallet := reservationTestWallet(t)
	walletPublicKeyHash := bitcoin.PublicKeyHash(custodyWallet.publicKey)

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

	btcChain := newLocalBitcoinChain()
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

	chain := newRangeRevealChain(&DepositRevealedEvent{
		BlockNumber:         299990,
		FundingTxHash:       fundingTxHash,
		FundingOutputIndex:  fundingOutputIndex,
		WalletPublicKeyHash: walletPublicKeyHash,
		Amount:              100000,
		Depositor:           "0x0000000000000000000000000000000000000001",
	})
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

	return newReservationAnchorAction(
		zap.NewNop().Sugar(),
		chain,
		btcChain,
		custodyWallet,
		nil, // signing executor unreached: execute stops before signing
		&ReservationAnchorProposal{
			DepositFundingTxHash:      fundingTxHash,
			DepositFundingOutputIndex: fundingOutputIndex,
			RequestNonce:              1,
			AnchorTxFee:               fee,
		},
		300000,
		lowExpiryBlock,
		nil,
		nil,
	)
}

func anchorVsize(t *testing.T) int64 {
	t.Helper()

	vsize, err := bitcoin.NewTransactionSizeEstimator().
		AddScriptHashInputs(1, DepositScriptByteSize, true).
		AddPublicKeyHashOutputs(1, true).
		VirtualSize()
	if err != nil {
		t.Fatal(err)
	}

	return vsize
}

// TestReservationAnchorAction_RejectsBelowProtocolMinimum verifies that an
// anchor proposal with a fee below ProtocolMinWalletTxSatPerVByteFee *
// (vsize - 1) is rejected before signing. On the base code the check only
// warned, so execute went on to the signing boundary.
func TestReservationAnchorAction_RejectsBelowProtocolMinimum(t *testing.T) {
	vsize := anchorVsize(t)
	threshold := ProtocolMinWalletTxSatPerVByteFee * (vsize - 1)

	scenarios := map[string]*big.Int{
		"one satoshi below the threshold": big.NewInt(threshold - 1),
		"relay floor fee":                 big.NewInt(vsize),
	}

	for name, fee := range scenarios {
		t.Run(name, func(t *testing.T) {
			err := newAnchorActionWithFee(t, fee).execute()
			if err == nil {
				t.Fatalf(
					"expected fee [%v] to be rejected (threshold [%d])",
					fee,
					threshold,
				)
			}
			if !strings.Contains(err.Error(), "protocol minimum") {
				t.Errorf("expected a protocol minimum error, got: [%v]", err)
			}
		})
	}
}

// TestReservationAnchorAction_AcceptsAtProtocolMinimum verifies that fees at
// the reject threshold and at the full protocol minimum pass the fee gate and
// the action proceeds to the signing boundary.
func TestReservationAnchorAction_AcceptsAtProtocolMinimum(t *testing.T) {
	vsize := anchorVsize(t)

	scenarios := map[string]*big.Int{
		"exactly at the threshold": big.NewInt(
			ProtocolMinWalletTxSatPerVByteFee * (vsize - 1),
		),
		"exactly at the full protocol minimum": big.NewInt(
			ProtocolMinWalletTxSatPerVByteFee * vsize,
		),
	}

	for name, fee := range scenarios {
		t.Run(name, func(t *testing.T) {
			err := newAnchorActionWithFee(t, fee).execute()
			if err == nil || err.Error() != reachedSigningBoundaryError {
				t.Errorf(
					"expected fee [%v] to reach the signing boundary "+
						"(error [%s]), got: [%v]",
					fee,
					reachedSigningBoundaryError,
					err,
				)
			}
		})
	}
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
		btcChain := newLocalBitcoinChain()

		fundingTx := &bitcoin.Transaction{
			Outputs: []*bitcoin.TransactionOutput{{Value: 100000}},
		}
		if err := btcChain.BroadcastTransaction(fundingTx); err != nil {
			t.Fatal(err)
		}
		fundingTxHash := fundingTx.Hash()

		// A DepositRevealed event exists for this wallet in the searched
		// window, but for a different funding outpoint - the matching
		// must walk past it and still report no match, not silently
		// accept it.
		chain := newRangeRevealChain(&DepositRevealedEvent{
			BlockNumber:         299990,
			FundingTxHash:       bitcoin.Hash{0x99},
			FundingOutputIndex:  0,
			WalletPublicKeyHash: walletPublicKeyHash,
		})
		chain.setDepositRequest(fundingTxHash, fundingOutputIndex, &DepositChainRequest{
			Amount:     100000,
			RevealedAt: time.Now(),
		})

		err := newAction(chain, btcChain, fundingTxHash).execute()
		if err == nil || err.Error() != "no matching DepositRevealed event for deposit" {
			t.Errorf(
				"unexpected error\nexpected: [no matching DepositRevealed event for deposit]\nactual:   [%v]",
				err,
			)
		}
	})

	t.Run("deposit request not found", func(t *testing.T) {
		btcChain := newLocalBitcoinChain()

		fundingTx := &bitcoin.Transaction{
			Outputs: []*bitcoin.TransactionOutput{{Value: 100000}},
		}
		if err := btcChain.BroadcastTransaction(fundingTx); err != nil {
			t.Fatal(err)
		}
		fundingTxHash := fundingTx.Hash()

		chain := newRangeRevealChain(&DepositRevealedEvent{
			BlockNumber:         299990,
			FundingTxHash:       fundingTxHash,
			FundingOutputIndex:  fundingOutputIndex,
			WalletPublicKeyHash: walletPublicKeyHash,
		})
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

	// The happy path runs for a recent reveal, for a reveal 250000 blocks
	// (about 35 days) back, and on a chain with 1-second blocks: a
	// depositor may request acceptance long after revealing, and the
	// signer locates the reveal from its timestamp and the chain's block
	// time. The 250000-block reveal's event sits 3000 blocks off the block
	// estimated from its timestamp, inside the lookup margin, as
	// block-time drift would place it.
	happyPathCases := map[string]struct {
		revealAge   time.Duration
		revealBlock uint64
		blockTime   time.Duration
	}{
		"recent reveal": {
			revealAge:   0,
			revealBlock: 299990,
		},
		"reveal 250000 blocks back": {
			revealAge:   250000 * 12 * time.Second,
			revealBlock: 300000 - 250000 + 3000,
		},
		"reveal 7200 one-second blocks back": {
			revealAge:   2 * time.Hour,
			revealBlock: 300000 - 7200,
			blockTime:   time.Second,
		},
	}
	for name, happyPathCase := range happyPathCases {
		t.Run("full happy path up to the signing boundary: "+name, func(t *testing.T) {
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

			chain := newRangeRevealChain(&DepositRevealedEvent{
				BlockNumber:         happyPathCase.revealBlock,
				FundingTxHash:       fundingTxHash,
				FundingOutputIndex:  fundingOutputIndex,
				WalletPublicKeyHash: walletPublicKeyHash,
				Amount:              100000,
				Depositor:           "0x0000000000000000000000000000000000000001",
			})
			chain.setDepositRequest(fundingTxHash, fundingOutputIndex, &DepositChainRequest{
				Amount:     100000,
				RevealedAt: time.Now().Add(-happyPathCase.revealAge),
			})
			chain.setReservationAction(&ReservationAction{
				ActionType:                ReservationActionTypeAcceptance,
				State:                     ReservationActionStatePending,
				TargetWalletPublicKeyHash: walletPublicKeyHash,
				TxMaxFee:                  2000,
			})

			var actionChain Chain = chain
			if happyPathCase.blockTime != 0 {
				actionChain = &timedRangeRevealChain{
					rangeRevealChain: chain,
					blockTime:        happyPathCase.blockTime,
				}
			}
			action := newAction(actionChain, btcChain, fundingTxHash)
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
	}

	t.Run("target wallet mismatch is rejected before signing", func(t *testing.T) {
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

		chain := newRangeRevealChain(&DepositRevealedEvent{
			BlockNumber:         299990,
			FundingTxHash:       fundingTxHash,
			FundingOutputIndex:  fundingOutputIndex,
			WalletPublicKeyHash: walletPublicKeyHash,
			Amount:              100000,
			Depositor:           "0x0000000000000000000000000000000000000001",
		})
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

// reanchorTestCreationPeriod is the Bridge wallet creation period used by the
// re-anchor tests, in seconds.
const reanchorTestCreationPeriod = uint32(30 * 24 * 60 * 60)

// reanchorFixture holds a chain on which every on-chain read of the
// re-anchor execute path succeeds: the reservation, a pending re-anchor
// action targeting targetWalletPKH, the target wallet, the wallet parameters
// and the chain head timestamp.
type reanchorFixture struct {
	chain           *localChain
	btcChain        *localBitcoinChain
	custodyWallet   wallet
	targetWalletPKH [20]byte
}

// newReanchorFixture builds a reanchorFixture whose target wallet was
// created targetAge before the chain head.
func newReanchorFixture(t *testing.T, targetAge time.Duration) *reanchorFixture {
	t.Helper()

	custodyWallet := reservationTestWallet(t)
	walletPublicKeyHash := bitcoin.PublicKeyHash(custodyWallet.publicKey)
	targetWalletPKH := [20]byte{0x02}

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

	headTime := time.Unix(1_700_000_000, 0)

	chain := Connect()
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
		TargetWalletPublicKeyHash: targetWalletPKH,
		TxMaxFee:                  2000,
	})
	chain.setWallet(targetWalletPKH, &WalletChainData{
		CreatedAt: headTime.Add(-targetAge),
	})
	chain.SetWalletParameters(WalletParameters{
		CreationPeriod: reanchorTestCreationPeriod,
	})
	chain.SetLatestBlockTimestamp(headTime)

	return &reanchorFixture{
		chain:           chain,
		btcChain:        btcChain,
		custodyWallet:   custodyWallet,
		targetWalletPKH: targetWalletPKH,
	}
}

// newAgedReanchorFixture returns a fixture whose target wallet is well past the
// creation period.
func newAgedReanchorFixture(t *testing.T) *reanchorFixture {
	t.Helper()

	return newReanchorFixture(
		t,
		2*time.Duration(reanchorTestCreationPeriod)*time.Second,
	)
}

// action builds a re-anchor action over the given chain. An expiry block
// below reservationActionSigningTimeoutSafetyMarginBlocks makes execute stop
// with "invalid proposal expiry block" right before signing, so reaching
// that error proves every earlier check passed.
func (f *reanchorFixture) action(
	chain Chain,
	fee *big.Int,
	expiryBlock uint64,
) *reservationReanchorAction {
	return newReservationReanchorAction(
		zap.NewNop().Sugar(),
		chain,
		f.btcChain,
		f.custodyWallet,
		nil, // signing executor unreached: execute stops before signing
		&ReservationReanchorProposal{
			ReservationKey:            big.NewInt(777),
			RequestNonce:              1,
			TargetWalletPublicKeyHash: f.targetWalletPKH,
			ReanchorTxFee:             fee,
		},
		300000,
		expiryBlock,
		nil,
		nil,
	)
}

const (
	// reachedSigningBoundaryError is the error execute returns when every
	// check before signing passed and only the test's low expiry block
	// stops it.
	reachedSigningBoundaryError = "invalid proposal expiry block"
	// lowExpiryBlock is below reservationActionSigningTimeoutSafetyMarginBlocks.
	lowExpiryBlock = 100
)

func reanchorVsize(t *testing.T) int64 {
	t.Helper()

	vsize, err := bitcoin.NewTransactionSizeEstimator().
		AddPublicKeyHashInputs(1, true).
		AddPublicKeyHashOutputs(1, true).
		VirtualSize()
	if err != nil {
		t.Fatal(err)
	}

	return vsize
}

// TestReservationReanchorAction_RejectsBelowProtocolMinimum verifies that a
// re-anchor proposal with a fee below ProtocolMinWalletTxSatPerVByteFee *
// (vsize - 1) is rejected before signing. On the base code the check only
// warned, so execute went on to the signing boundary.
func TestReservationReanchorAction_RejectsBelowProtocolMinimum(t *testing.T) {
	vsize := reanchorVsize(t)
	threshold := ProtocolMinWalletTxSatPerVByteFee * (vsize - 1)

	scenarios := map[string]*big.Int{
		"one satoshi below the threshold": big.NewInt(threshold - 1),
		"relay floor fee":                 big.NewInt(vsize),
	}

	for name, fee := range scenarios {
		t.Run(name, func(t *testing.T) {
			fixture := newAgedReanchorFixture(t)

			err := fixture.action(fixture.chain, fee, lowExpiryBlock).execute()
			if err == nil {
				t.Fatalf(
					"expected fee [%v] to be rejected (threshold [%d])",
					fee,
					threshold,
				)
			}
			if !strings.Contains(err.Error(), "protocol minimum") {
				t.Errorf("expected a protocol minimum error, got: [%v]", err)
			}
		})
	}
}

// TestReservationReanchorAction_AcceptsAtProtocolMinimum verifies that fees at
// the reject threshold and at the full protocol minimum pass the fee gate and
// the action proceeds to the signing boundary.
func TestReservationReanchorAction_AcceptsAtProtocolMinimum(t *testing.T) {
	vsize := reanchorVsize(t)

	scenarios := map[string]*big.Int{
		"exactly at the threshold": big.NewInt(
			ProtocolMinWalletTxSatPerVByteFee * (vsize - 1),
		),
		"exactly at the full protocol minimum": big.NewInt(
			ProtocolMinWalletTxSatPerVByteFee * vsize,
		),
	}

	for name, fee := range scenarios {
		t.Run(name, func(t *testing.T) {
			fixture := newAgedReanchorFixture(t)

			err := fixture.action(fixture.chain, fee, lowExpiryBlock).execute()
			if err == nil || err.Error() != reachedSigningBoundaryError {
				t.Errorf(
					"expected fee [%v] to reach the signing boundary "+
						"(error [%s]), got: [%v]",
					fee,
					reachedSigningBoundaryError,
					err,
				)
			}
		})
	}
}

// reanchorReadFailureChain makes the chain reads of the re-anchor age gate
// fail on demand.
type reanchorReadFailureChain struct {
	*localChain
	failGetWallet            bool
	failWalletParameters     bool
	failLatestBlockTimestamp bool
}

func (c *reanchorReadFailureChain) GetWallet(
	walletPublicKeyHash [20]byte,
) (*WalletChainData, error) {
	if c.failGetWallet {
		return nil, fmt.Errorf("simulated GetWallet failure")
	}
	return c.localChain.GetWallet(walletPublicKeyHash)
}

func (c *reanchorReadFailureChain) GetWalletParameters() (WalletParameters, error) {
	if c.failWalletParameters {
		return WalletParameters{}, fmt.Errorf("simulated GetWalletParameters failure")
	}
	return c.localChain.GetWalletParameters()
}

func (c *reanchorReadFailureChain) GetLatestBlockTimestamp() (time.Time, error) {
	if c.failLatestBlockTimestamp {
		return time.Time{}, fmt.Errorf("simulated GetLatestBlockTimestamp failure")
	}
	return c.localChain.GetLatestBlockTimestamp()
}

// TestReservationReanchorAction_RejectsYoungTarget verifies the follower-side
// target age gate: a re-anchor to a wallet younger than the Bridge creation
// period is refused (the leader may be malicious), an aged target and one
// within the two-block tolerance at the boundary are signed, and any failed
// read fails closed. Target ages are relative to the chain head timestamp.
func TestReservationReanchorAction_RejectsYoungTarget(t *testing.T) {
	period := time.Duration(reanchorTestCreationPeriod) * time.Second

	ageCases := []struct {
		name      string
		targetAge time.Duration
		// wantError is empty when the action must reach the signing
		// boundary, otherwise a substring of the expected rejection.
		wantError string
	}{
		{
			name:      "young target is rejected",
			targetAge: 24 * time.Hour,
			wantError: "younger than the creation period",
		},
		{
			name:      "brand new target is rejected",
			targetAge: 0,
			wantError: "younger than the creation period",
		},
		{
			name:      "aged target is accepted",
			targetAge: 2 * period,
		},
		{
			name:      "target exactly at the creation period is accepted",
			targetAge: period,
		},
		{
			name:      "target at the tolerance boundary is accepted",
			targetAge: period - ReanchorTargetAgeTolerance,
		},
		{
			name:      "target one second past the tolerance is rejected",
			targetAge: period - ReanchorTargetAgeTolerance - time.Second,
			wantError: "younger than the creation period",
		},
	}

	for _, tc := range ageCases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newReanchorFixture(t, tc.targetAge)

			err := fixture.action(
				fixture.chain,
				big.NewInt(1500),
				lowExpiryBlock,
			).execute()

			assertReanchorOutcome(t, err, tc.wantError)
		})
	}

	readFailureCases := map[string]struct {
		configure func(*reanchorReadFailureChain)
		wantError string
	}{
		"target wallet read failure is rejected": {
			configure: func(c *reanchorReadFailureChain) { c.failGetWallet = true },
			wantError: "cannot get target wallet",
		},
		"wallet parameters read failure is rejected": {
			configure: func(c *reanchorReadFailureChain) { c.failWalletParameters = true },
			wantError: "cannot get wallet parameters",
		},
		"chain head read failure is rejected": {
			configure: func(c *reanchorReadFailureChain) { c.failLatestBlockTimestamp = true },
			wantError: "cannot get chain head timestamp",
		},
	}

	for name, tc := range readFailureCases {
		t.Run(name, func(t *testing.T) {
			// The target is aged, so only the failed read can reject it.
			fixture := newAgedReanchorFixture(t)
			chain := &reanchorReadFailureChain{localChain: fixture.chain}
			tc.configure(chain)

			err := fixture.action(
				chain,
				big.NewInt(1500),
				lowExpiryBlock,
			).execute()

			assertReanchorOutcome(t, err, tc.wantError)
		})
	}
}

// assertReanchorOutcome checks that err is the signing-boundary error when
// wantError is empty, and otherwise contains wantError.
func assertReanchorOutcome(t *testing.T, err error, wantError string) {
	t.Helper()

	if wantError == "" {
		if err == nil || err.Error() != reachedSigningBoundaryError {
			t.Errorf(
				"expected to reach the signing boundary (error [%s]), got: [%v]",
				reachedSigningBoundaryError,
				err,
			)
		}
		return
	}

	if err == nil || !strings.Contains(err.Error(), wantError) {
		t.Errorf("expected error containing [%s], got: [%v]", wantError, err)
	}
}

// hangingBitcoinChain blocks GetTransaction until release is closed.
type hangingBitcoinChain struct {
	*localBitcoinChain
	release chan struct{}
}

func (h *hangingBitcoinChain) GetTransaction(
	bitcoin.Hash,
) (*bitcoin.Transaction, error) {
	<-h.release
	return nil, fmt.Errorf("released")
}

// hangingReanchorValidatorChain blocks the on-chain re-anchor proposal
// validator view until release is closed.
type hangingReanchorValidatorChain struct {
	*localChain
	release chan struct{}
}

func (h *hangingReanchorValidatorChain) ValidateReservationReanchorProposal(
	[20]byte,
	*ReservationReanchorProposal,
) error {
	<-h.release
	return fmt.Errorf("released")
}

// TestReservationExecute_RPCTimeoutReleasesDispatcher verifies that a hung
// RPC read in a reservation execute path times out, so the per-wallet
// dispatcher lock is released and the wallet accepts the next action. Without
// the bound the hung goroutine would hold the lock for the process lifetime.
func TestReservationExecute_RPCTimeoutReleasesDispatcher(t *testing.T) {
	originalTimeout := reservationRPCRequestTimeout
	reservationRPCRequestTimeout = 200 * time.Millisecond
	t.Cleanup(func() {
		reservationRPCRequestTimeout = originalTimeout
	})

	custodyWallet := reservationTestWallet(t)

	// Each case builds an action that blocks on one of the bounded reads.
	// The release channel frees the blocked goroutines when the test ends.
	newHangingActions := map[string]func(
		t *testing.T,
		release chan struct{},
	) walletAction{
		"bitcoin GetTransaction hangs in the anchor path": func(
			t *testing.T,
			release chan struct{},
		) walletAction {
			return newReservationAnchorAction(
				zap.NewNop().Sugar(),
				Connect(),
				&hangingBitcoinChain{
					localBitcoinChain: newLocalBitcoinChain(),
					release:           release,
				},
				custodyWallet,
				nil,
				&ReservationAnchorProposal{
					DepositFundingTxHash:      bitcoin.Hash{0x01},
					DepositFundingOutputIndex: 0,
					RequestNonce:              1,
					AnchorTxFee:               big.NewInt(1500),
				},
				300000,
				300000+600,
				nil,
				nil,
			)
		},
		"on-chain validator view hangs in the re-anchor path": func(
			t *testing.T,
			release chan struct{},
		) walletAction {
			fixture := newAgedReanchorFixture(t)
			fixture.custodyWallet = custodyWallet

			return fixture.action(
				&hangingReanchorValidatorChain{
					localChain: fixture.chain,
					release:    release,
				},
				big.NewInt(1500),
				300000+600,
			)
		},
	}

	for name, newAction := range newHangingActions {
		t.Run(name, func(t *testing.T) {
			release := make(chan struct{})
			t.Cleanup(func() { close(release) })

			dispatcher := newWalletDispatcher()
			dispatcherBusy := func() bool {
				dispatcher.actionsMutex.Lock()
				defer dispatcher.actionsMutex.Unlock()

				return len(dispatcher.actions) > 0
			}
			waitUntilIdle := func() {
				t.Helper()

				deadline := time.Now().Add(5 * time.Second)
				for dispatcherBusy() {
					if time.Now().After(deadline) {
						t.Fatal(
							"dispatcher lock was not released after the " +
								"RPC call timed out",
						)
					}
					time.Sleep(10 * time.Millisecond)
				}
			}

			if err := dispatcher.dispatch(newAction(t, release)); err != nil {
				t.Fatalf("first dispatch failed: [%v]", err)
			}

			// While the RPC call hangs, the wallet is busy.
			if err := dispatcher.dispatch(newAction(t, release)); err != errWalletBusy {
				t.Fatalf(
					"expected the wallet to be busy during the hung call, "+
						"got: [%v]",
					err,
				)
			}

			// The bounded call times out and execute returns, releasing the
			// lock without the hung call ever finishing.
			waitUntilIdle()

			// The wallet accepts the next action.
			if err := dispatcher.dispatch(newAction(t, release)); err != nil {
				t.Fatalf(
					"expected the wallet to accept an action after the "+
						"timeout, got: [%v]",
					err,
				)
			}
			waitUntilIdle()
		})
	}
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
		// The age gate reads the target wallet, the wallet parameters and
		// the chain head time; the target is well past the creation period.
		headTime := time.Unix(1_700_000_000, 0)
		chain.setWallet(walletPublicKeyHash, &WalletChainData{
			CreatedAt: headTime.Add(
				-2 * time.Duration(reanchorTestCreationPeriod) * time.Second,
			),
		})
		chain.SetWalletParameters(WalletParameters{
			CreationPeriod: reanchorTestCreationPeriod,
		})
		chain.SetLatestBlockTimestamp(headTime)

		// Every real upstream step (reservation load, action load,
		// type/state check, target wallet match, target age gate,
		// on-chain validation, fee gate, transaction assembly) must
		// succeed before the expiry-block guard is reached and rejects
		// the proposal - reaching this exact error is the test's proof
		// that all of it worked.
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

// TestRerun_G1R11_RPCCallsParkedAfterTimeoutAreBounded checks that calls whose
// caller timed out keep their slot until the backend answers, and that a new
// call fails at once when every slot is held, instead of parking one more
// goroutine.
func TestRerun_G1R11_RPCCallsParkedAfterTimeoutAreBounded(t *testing.T) {
	origTimeout := reservationRPCRequestTimeout
	origSlots := reservationRPCSlots
	reservationRPCRequestTimeout = 20 * time.Millisecond
	reservationRPCSlots = make(chan struct{}, 2)
	defer func() {
		reservationRPCRequestTimeout = origTimeout
		reservationRPCSlots = origSlots
	}()

	blocker := make(chan struct{})
	defer close(blocker)

	for i := range 2 {
		_, err := rpcCallWithTimeout(func() (int, error) {
			<-blocker
			return 0, nil
		}, "hung")
		if err == nil || !strings.Contains(err.Error(), "timed out") {
			t.Fatalf("call %d: expected a timeout, got: %v", i, err)
		}
	}

	called := false
	_, err := rpcCallWithTimeout(func() (int, error) {
		called = true
		return 1, nil
	}, "fresh")
	if err == nil || !strings.Contains(err.Error(), "too many") {
		t.Fatalf("expected the slot bound to reject the call, got: %v", err)
	}
	if called {
		t.Fatal("the call ran although every slot was held")
	}
}

// TestRerun_G1R11_IndependentRPCCallsRunConcurrently checks that two calls
// with the same label (for example GetWallet for two different wallets) do
// not block each other.
func TestRerun_G1R11_IndependentRPCCallsRunConcurrently(t *testing.T) {
	origTimeout := reservationRPCRequestTimeout
	reservationRPCRequestTimeout = time.Second
	defer func() { reservationRPCRequestTimeout = origTimeout }()

	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	firstDone := make(chan error, 1)
	go func() {
		_, err := rpcCallWithTimeout(func() (int, error) {
			close(firstStarted)
			<-releaseFirst
			return 1, nil
		}, "GetWallet")
		firstDone <- err
	}()
	<-firstStarted

	value, err := rpcCallWithTimeout(func() (int, error) {
		return 2, nil
	}, "GetWallet")
	if err != nil || value != 2 {
		t.Fatalf("second call blocked by the first: value=%d err=%v", value, err)
	}

	close(releaseFirst)
	if err := <-firstDone; err != nil {
		t.Fatalf("first call failed: %v", err)
	}
}

func TestRerun_G1R29_RevealLookupCentersOnChainHeadTime(t *testing.T) {
	btcChain := newLocalBitcoinChain()
	custodyWallet := reservationTestWallet(t)
	walletPublicKeyHash := bitcoin.PublicKeyHash(custodyWallet.publicKey)

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

	chainHeadTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	revealedAt := chainHeadTime.Add(-10 * time.Minute)

	chain := newRangeRevealChain(&DepositRevealedEvent{
		BlockNumber:         299990,
		FundingTxHash:       fundingTxHash,
		FundingOutputIndex:  0,
		WalletPublicKeyHash: walletPublicKeyHash,
		Amount:              100000,
		Depositor:           "0x0000000000000000000000000000000000000001",
	})
	chain.SetLatestBlockTimestamp(chainHeadTime)
	chain.setDepositRequest(fundingTxHash, 0, &DepositChainRequest{
		Amount:     100000,
		RevealedAt: revealedAt,
	})
	chain.setReservationAction(&ReservationAction{
		ActionType:                ReservationActionTypeAcceptance,
		State:                     ReservationActionStatePending,
		TargetWalletPublicKeyHash: walletPublicKeyHash,
		TxMaxFee:                  2000,
	})

	action := newReservationAnchorAction(
		zap.NewNop().Sugar(),
		chain,
		btcChain,
		custodyWallet,
		nil,
		&ReservationAnchorProposal{
			DepositFundingTxHash:      fundingTxHash,
			DepositFundingOutputIndex: 0,
			RequestNonce:              1,
			AnchorTxFee:               big.NewInt(1500),
		},
		300000,
		10,
		nil,
		nil,
	)

	err = action.execute()
	if err == nil || err.Error() != "invalid proposal expiry block" {
		t.Fatalf("unexpected error: %v", err)
	}

	// The reveal is 10 minutes before the chain head, so a lookup centered
	// on chain time finds it in the first window. A lookup centered on the
	// host clock (here, years later) starts far from block 299990.
	queries := chain.reveals.queries
	if len(queries) == 0 {
		t.Fatal("expected at least one reveal lookup query")
	}
	if first := queries[0]; first[0] > 299990 || first[1] < 299990 {
		t.Errorf(
			"first lookup window [%d, %d] does not contain the reveal block 299990",
			first[0],
			first[1],
		)
	}
}
