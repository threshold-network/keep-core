package tbtc

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/btcsuite/btcd/btcec"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/covenantsigner"
)

// covenantActionScaffold holds the common self_v1 signing setup shared by the
// REDEEM and RENEW transaction-output binding tests.
type covenantActionScaffold struct {
	service              *covenantsigner.Service
	node                 *node
	walletPublicKey      *ecdsa.PublicKey
	depositorPrivateKey  *btcec.PrivateKey
	request              covenantsigner.RouteSubmitRequest
	reserve              string
	revealer             string
	vault                string
	epoch                uint64
	destinationValueSats uint64
}

// newCovenantActionScaffold builds a valid self_v1 request shell (node, service,
// template, active UTXO, transaction plan) with no destination attached. The
// caller sets request.Action + the matching destination + DestinationCommitmentHash,
// then finalizes via applyTestMigrationTransactionPlanCommitment /
// applyTestArtifactApprovals before submitting.
func newCovenantActionScaffold(t *testing.T) covenantActionScaffold {
	t.Helper()

	node, bitcoinChain, walletPublicKey := setupCovenantSignerTestNode(t)

	service, err := covenantsigner.NewService(
		newCovenantSignerMemoryHandle(),
		newCovenantSignerEngine(node, 0, true, testEIP712ChainID, testEIP712Salt),
	)
	if err != nil {
		t.Fatal(err)
	}

	depositorPrivateKey, _ := btcec.PrivKeyFromBytes(btcec.S256(), bytes.Repeat([]byte{0x42}, 32))
	depositorPublicKey := depositorPrivateKey.PubKey().SerializeCompressed()
	signerPublicKey := (*btcec.PublicKey)(walletPublicKey).SerializeCompressed()

	template := &covenantsigner.SelfV1Template{
		Template:           covenantsigner.TemplateSelfV1,
		DepositorPublicKey: "0x" + hex.EncodeToString(depositorPublicKey),
		SignerPublicKey:    "0x" + hex.EncodeToString(signerPublicKey),
		Delta2:             4320,
	}
	templateJSON, err := json.Marshal(template)
	if err != nil {
		t.Fatal(err)
	}

	maturityHeight := uint64(912345)
	witnessScript, err := buildSelfV1WitnessScript(template, maturityHeight)
	if err != nil {
		t.Fatal(err)
	}
	witnessScriptHash := bitcoin.WitnessScriptHash(witnessScript)
	activeScriptPubKey, err := bitcoin.PayToWitnessScriptHash(witnessScriptHash)
	if err != nil {
		t.Fatal(err)
	}

	const (
		inputValueSats       = uint64(1_000_000)
		destinationValueSats = uint64(998_000)
		anchorValueSats      = uint64(330)
		feeSats              = uint64(1_670)
	)

	prevTransaction := &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{
			{Outpoint: &bitcoin.TransactionOutpoint{}, Sequence: 0xffffffff},
		},
		Outputs: []*bitcoin.TransactionOutput{
			{Value: int64(inputValueSats), PublicKeyScript: activeScriptPubKey},
		},
		Locktime: 0,
	}
	bitcoinChain.transactions = append(bitcoinChain.transactions, prevTransaction)
	bitcoinChain.setTransactionConfirmations(prevTransaction.Hash(), 6)

	activeScriptHash := sha256.Sum256(activeScriptPubKey)
	reserve := "0x1111111111111111111111111111111111111111"
	revealer := "0x2222222222222222222222222222222222222222"
	vault := "0x3333333333333333333333333333333333333333"

	request := covenantsigner.RouteSubmitRequest{
		FacadeRequestID: "rf_action_1",
		IdempotencyKey:  "idem_action_1",
		RequestType:     covenantsigner.RequestTypeReconstruct,
		Route:           covenantsigner.TemplateSelfV1,
		Strategy:        "0x1234",
		Reserve:         reserve,
		Epoch:           12,
		MaturityHeight:  maturityHeight,
		ActiveOutpoint: covenantsigner.CovenantOutpoint{
			TxID:       "0x" + prevTransaction.Hash().Hex(bitcoin.ReversedByteOrder),
			Vout:       0,
			ScriptHash: "0x" + hex.EncodeToString(activeScriptHash[:]),
		},
		MigrationTransactionPlan: &covenantsigner.MigrationTransactionPlan{
			InputValueSats:       inputValueSats,
			DestinationValueSats: destinationValueSats,
			AnchorValueSats:      anchorValueSats,
			FeeSats:              feeSats,
			InputSequence:        0xfffffffd,
			LockTime:             uint32(maturityHeight),
		},
		Artifacts:      map[covenantsigner.RecoveryPathID]covenantsigner.ArtifactRecord{},
		ScriptTemplate: templateJSON,
		Signing: covenantsigner.SigningRequirements{
			SignerRequired:    true,
			CustodianRequired: false,
		},
	}

	return covenantActionScaffold{
		service:              service,
		node:                 node,
		walletPublicKey:      walletPublicKey,
		depositorPrivateKey:  depositorPrivateKey,
		request:              request,
		reserve:              reserve,
		revealer:             revealer,
		vault:                vault,
		epoch:                12,
		destinationValueSats: destinationValueSats,
	}
}

// payoutScript is a distinct P2WPKH scriptPubKey used as the redeem/renew output.
func payoutScript(t *testing.T, fill byte) bitcoin.Script {
	t.Helper()
	var hash [20]byte
	for i := range hash {
		hash[i] = fill
	}
	script, err := bitcoin.PayToWitnessPublicKeyHash(hash)
	if err != nil {
		t.Fatal(err)
	}
	return script
}

type testRedeemCommitmentPayload struct {
	Reserve          string `json:"reserve"`
	Epoch            uint64 `json:"epoch"`
	Route            string `json:"route"`
	Revealer         string `json:"revealer"`
	Vault            string `json:"vault"`
	Network          string `json:"network"`
	OutputScriptHash string `json:"outputScriptHash"`
	OutputValueSats  uint64 `json:"outputValueSats"`
}

func testRedeemCommitmentHash(t *testing.T, r *covenantsigner.RedeemDestinationReservation) string {
	t.Helper()
	payload, err := json.Marshal(testRedeemCommitmentPayload{
		Reserve:          strings.ToLower(r.Reserve),
		Epoch:            r.Epoch,
		Route:            string(r.Route),
		Revealer:         strings.ToLower(r.Revealer),
		Vault:            strings.ToLower(r.Vault),
		Network:          strings.TrimSpace(r.Network),
		OutputScriptHash: strings.ToLower(r.OutputScriptHash),
		OutputValueSats:  r.OutputValueSats,
	})
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(payload)
	return "0x" + hex.EncodeToString(sum[:])
}

type testRenewCommitmentPayload struct {
	Reserve                string `json:"reserve"`
	Epoch                  uint64 `json:"epoch"`
	Route                  string `json:"route"`
	Revealer               string `json:"revealer"`
	Vault                  string `json:"vault"`
	Network                string `json:"network"`
	NextCovenantScriptHash string `json:"nextCovenantScriptHash"`
	NextMaturityHeight     uint64 `json:"nextMaturityHeight"`
	OutputValueSats        uint64 `json:"outputValueSats"`
}

func testRenewCommitmentHash(t *testing.T, r *covenantsigner.RenewDestinationReservation) string {
	t.Helper()
	payload, err := json.Marshal(testRenewCommitmentPayload{
		Reserve:                strings.ToLower(r.Reserve),
		Epoch:                  r.Epoch,
		Route:                  string(r.Route),
		Revealer:               strings.ToLower(r.Revealer),
		Vault:                  strings.ToLower(r.Vault),
		Network:                strings.TrimSpace(r.Network),
		NextCovenantScriptHash: strings.ToLower(r.NextCovenantScriptHash),
		NextMaturityHeight:     r.NextMaturityHeight,
		OutputValueSats:        r.OutputValueSats,
	})
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(payload)
	return "0x" + hex.EncodeToString(sum[:])
}

// submitAndDecode finalizes plan+approvals, submits, and returns the signed
// transaction's outputs for assertion.
func submitAndDecode(t *testing.T, s covenantActionScaffold, id string) *bitcoin.Transaction {
	t.Helper()

	applyTestMigrationTransactionPlanCommitment(t, &s.request)
	applyTestArtifactApprovals(t, s.node, s.walletPublicKey, &s.request, s.depositorPrivateKey, nil)

	result, err := s.service.Submit(context.Background(), covenantsigner.TemplateSelfV1, covenantsigner.SignerSubmitInput{
		RouteRequestID: id,
		Stage:          covenantsigner.StageSignerCoordination,
		Request:        s.request,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != covenantsigner.StepStatusReady {
		t.Fatalf("expected READY, got %s (%s)", result.Status, result.Detail)
	}

	transactionBytes, err := hex.DecodeString(strings.TrimPrefix(result.TransactionHex, "0x"))
	if err != nil {
		t.Fatal(err)
	}
	transaction := &bitcoin.Transaction{}
	if err := transaction.Deserialize(transactionBytes); err != nil {
		t.Fatal(err)
	}
	return transaction
}

func TestCovenantSignerEngine_SubmitRedeemPaysCommittedOutput(t *testing.T) {
	s := newCovenantActionScaffold(t)
	outputScript := payoutScript(t, 0xbb)

	dest := &covenantsigner.RedeemDestinationReservation{
		ReservationID:    "crdr_1",
		Reserve:          s.reserve,
		Epoch:            s.epoch,
		Route:            covenantsigner.ReservationRouteRedeem,
		Revealer:         s.revealer,
		Vault:            s.vault,
		Network:          "regtest",
		Status:           covenantsigner.ReservationStatusReserved,
		OutputScript:     "0x" + hex.EncodeToString(outputScript),
		OutputScriptHash: testDepositScriptHash(t, outputScript),
		OutputValueSats:  s.destinationValueSats,
	}
	dest.DestinationCommitmentHash = testRedeemCommitmentHash(t, dest)

	s.request.Action = covenantsigner.CovenantActionRedeem
	s.request.RedeemDestination = dest
	s.request.DestinationCommitmentHash = dest.DestinationCommitmentHash

	transaction := submitAndDecode(t, s, "ors_redeem_tx")

	if len(transaction.Outputs) != 2 {
		t.Fatalf("unexpected output count: %d", len(transaction.Outputs))
	}
	if transaction.Outputs[0].Value != int64(s.destinationValueSats) {
		t.Fatalf("redeem output value: got %d, want %d", transaction.Outputs[0].Value, s.destinationValueSats)
	}
	if !bytes.Equal(transaction.Outputs[0].PublicKeyScript, outputScript) {
		t.Fatal("redeem output script does not match the committed payout script")
	}
}

func TestCovenantSignerEngine_SubmitRenewPaysCommittedOutput(t *testing.T) {
	s := newCovenantActionScaffold(t)
	nextScript := payoutScript(t, 0xcc)

	dest := &covenantsigner.RenewDestinationReservation{
		ReservationID:          "crnr_1",
		Reserve:                s.reserve,
		Epoch:                  s.epoch,
		Route:                  covenantsigner.ReservationRouteRenew,
		Revealer:               s.revealer,
		Vault:                  s.vault,
		Network:                "regtest",
		Status:                 covenantsigner.ReservationStatusReserved,
		NextCovenantScript:     "0x" + hex.EncodeToString(nextScript),
		NextCovenantScriptHash: testDepositScriptHash(t, nextScript),
		NextMaturityHeight:     987654,
		OutputValueSats:        s.destinationValueSats,
	}
	dest.DestinationCommitmentHash = testRenewCommitmentHash(t, dest)

	s.request.Action = covenantsigner.CovenantActionRenew
	s.request.RenewDestination = dest
	s.request.DestinationCommitmentHash = dest.DestinationCommitmentHash

	transaction := submitAndDecode(t, s, "ors_renew_tx")

	if len(transaction.Outputs) != 2 {
		t.Fatalf("unexpected output count: %d", len(transaction.Outputs))
	}
	if transaction.Outputs[0].Value != int64(s.destinationValueSats) {
		t.Fatalf("renew output value: got %d, want %d", transaction.Outputs[0].Value, s.destinationValueSats)
	}
	if !bytes.Equal(transaction.Outputs[0].PublicKeyScript, nextScript) {
		t.Fatal("renew output script does not match the committed next-covenant script")
	}
}

// nonLiveCovenantSigningStates enumerates every wallet state other than
// StateLive. Listing them explicitly (rather than deriving them) means a newly
// added WalletState does not silently escape the matrix below: it stays absent
// until someone decides, and records here, whether covenant signing may occur
// in it.
var nonLiveCovenantSigningStates = []WalletState{
	StateUnknown,
	StateMovingFunds,
	StateClosing,
	StateClosed,
	StateTerminated,
}

// approvedRedeemActionRequest returns a REDEEM request that is valid in every
// respect except, potentially, the wallet's on-chain state.
func approvedRedeemActionRequest(
	t *testing.T,
	s covenantActionScaffold,
) covenantsigner.RouteSubmitRequest {
	t.Helper()

	outputScript := payoutScript(t, 0xbb)
	dest := &covenantsigner.RedeemDestinationReservation{
		ReservationID:    "crdr_state_matrix",
		Reserve:          s.reserve,
		Epoch:            s.epoch,
		Route:            covenantsigner.ReservationRouteRedeem,
		Revealer:         s.revealer,
		Vault:            s.vault,
		Network:          "regtest",
		Status:           covenantsigner.ReservationStatusReserved,
		OutputScript:     "0x" + hex.EncodeToString(outputScript),
		OutputScriptHash: testDepositScriptHash(t, outputScript),
		OutputValueSats:  s.destinationValueSats,
	}
	dest.DestinationCommitmentHash = testRedeemCommitmentHash(t, dest)

	s.request.Action = covenantsigner.CovenantActionRedeem
	s.request.RedeemDestination = dest
	s.request.DestinationCommitmentHash = dest.DestinationCommitmentHash

	applyTestMigrationTransactionPlanCommitment(t, &s.request)
	applyTestArtifactApprovals(t, s.node, s.walletPublicKey, &s.request, s.depositorPrivateKey, nil)

	return s.request
}

// approvedRenewActionRequest returns a RENEW request that is valid in every
// respect except, potentially, the wallet's on-chain state.
func approvedRenewActionRequest(
	t *testing.T,
	s covenantActionScaffold,
) covenantsigner.RouteSubmitRequest {
	t.Helper()

	nextScript := payoutScript(t, 0xcc)
	dest := &covenantsigner.RenewDestinationReservation{
		ReservationID:          "crnr_state_matrix",
		Reserve:                s.reserve,
		Epoch:                  s.epoch,
		Route:                  covenantsigner.ReservationRouteRenew,
		Revealer:               s.revealer,
		Vault:                  s.vault,
		Network:                "regtest",
		Status:                 covenantsigner.ReservationStatusReserved,
		NextCovenantScript:     "0x" + hex.EncodeToString(nextScript),
		NextCovenantScriptHash: testDepositScriptHash(t, nextScript),
		NextMaturityHeight:     987654,
		OutputValueSats:        s.destinationValueSats,
	}
	dest.DestinationCommitmentHash = testRenewCommitmentHash(t, dest)

	s.request.Action = covenantsigner.CovenantActionRenew
	s.request.RenewDestination = dest
	s.request.DestinationCommitmentHash = dest.DestinationCommitmentHash

	applyTestMigrationTransactionPlanCommitment(t, &s.request)
	applyTestArtifactApprovals(t, s.node, s.walletPublicKey, &s.request, s.depositorPrivateKey, nil)

	return s.request
}

// TestCovenantSignerEngine_VerifySignerApprovalWalletStateMatrixForActions pins
// the action x wallet-state authorization matrix for the cooperative actions.
// REDEEM and RENEW are held to the same live-only rule as MIGRATION: a signer
// approval certificate binds wallet identity, members hash, and threshold, but
// nothing about wallet state, so a certificate issued while the wallet was live
// would otherwise stay replayable against a wallet the protocol has since
// deauthorized. Widening any cell here is a protocol decision, and this matrix
// is what makes such a widening deliberate rather than incidental.
func TestCovenantSignerEngine_VerifySignerApprovalWalletStateMatrixForActions(t *testing.T) {
	actions := []struct {
		name  string
		build func(*testing.T, covenantActionScaffold) covenantsigner.RouteSubmitRequest
	}{
		{"REDEEM", approvedRedeemActionRequest},
		{"RENEW", approvedRenewActionRequest},
	}

	for _, action := range actions {
		t.Run(action.name, func(t *testing.T) {
			s := newCovenantActionScaffold(t)
			request := action.build(t, s)

			localChain, ok := s.node.chain.(*localChain)
			if !ok {
				t.Fatal("expected local chain implementation")
			}

			walletPublicKeyHash := bitcoin.PublicKeyHash(s.walletPublicKey)
			live, err := localChain.GetWallet(walletPublicKeyHash)
			if err != nil {
				t.Fatal(err)
			}

			cse := &covenantSignerEngine{node: s.node}

			// Control: while the wallet is live the request is otherwise valid, so
			// a rejection below is attributable to the wallet state alone.
			if err := cse.VerifySignerApproval(request); err != nil {
				t.Fatalf("expected a live wallet to be accepted, got: %v", err)
			}

			for _, state := range nonLiveCovenantSigningStates {
				t.Run(state.String(), func(t *testing.T) {
					mutated := *live
					mutated.State = state
					localChain.setWallet(walletPublicKeyHash, &mutated)
					defer localChain.setWallet(walletPublicKeyHash, live)

					err := cse.VerifySignerApproval(request)
					if err == nil {
						t.Fatalf(
							"expected %s to be rejected for a wallet in state %v",
							action.name,
							state,
						)
					}
					if !strings.Contains(err.Error(), "not eligible for covenant signing") {
						t.Fatalf("unexpected error for state %v: %v", state, err)
					}
				})
			}
		})
	}
}
