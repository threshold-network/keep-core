package tbtc

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/keep-network/keep-common/pkg/persistence"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/chain/local_v1"
	"github.com/keep-network/keep-core/pkg/covenantsigner"
	"github.com/keep-network/keep-core/pkg/generator"
	"github.com/keep-network/keep-core/pkg/internal/tecdsatest"
	"github.com/keep-network/keep-core/pkg/net/local"
	"github.com/keep-network/keep-core/pkg/operator"
	"github.com/keep-network/keep-core/pkg/protocol/group"
	"github.com/keep-network/keep-core/pkg/tecdsa"
)

type covenantSignerMemoryDescriptor struct {
	name      string
	directory string
	content   []byte
}

func (md *covenantSignerMemoryDescriptor) Name() string      { return md.name }
func (md *covenantSignerMemoryDescriptor) Directory() string { return md.directory }
func (md *covenantSignerMemoryDescriptor) Content() ([]byte, error) {
	return md.content, nil
}

type covenantSignerMemoryHandle struct {
	items map[string]*covenantSignerMemoryDescriptor
}

func newCovenantSignerMemoryHandle() *covenantSignerMemoryHandle {
	return &covenantSignerMemoryHandle{items: make(map[string]*covenantSignerMemoryDescriptor)}
}

func (h *covenantSignerMemoryHandle) key(directory, name string) string {
	return directory + "/" + name
}

func (h *covenantSignerMemoryHandle) Save(data []byte, directory, name string) error {
	h.items[h.key(directory, name)] = &covenantSignerMemoryDescriptor{
		name:      name,
		directory: directory,
		content:   append([]byte{}, data...),
	}
	return nil
}

func (h *covenantSignerMemoryHandle) Delete(directory, name string) error {
	delete(h.items, h.key(directory, name))
	return nil
}

func (h *covenantSignerMemoryHandle) ReadAll() (<-chan persistence.DataDescriptor, <-chan error) {
	dataChan := make(chan persistence.DataDescriptor, len(h.items))
	errChan := make(chan error)
	for _, item := range h.items {
		dataChan <- item
	}
	close(dataChan)
	close(errChan)
	return dataChan, errChan
}

func TestCovenantSignerEngine_SubmitSelfV1Ready(t *testing.T) {
	node, bitcoinChain, walletPublicKey := setupCovenantSignerTestNode(t)

	service, err := covenantsigner.NewService(
		newCovenantSignerMemoryHandle(),
		newCovenantSignerEngine(node),
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

	destinationScript, err := bitcoin.PayToWitnessPublicKeyHash([20]byte{
		0xaa, 0xaa, 0xaa, 0xaa, 0xaa,
		0xaa, 0xaa, 0xaa, 0xaa, 0xaa,
		0xaa, 0xaa, 0xaa, 0xaa, 0xaa,
		0xaa, 0xaa, 0xaa, 0xaa, 0xaa,
	})
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
			{
				Outpoint: &bitcoin.TransactionOutpoint{},
				Sequence: 0xffffffff,
			},
		},
		Outputs: []*bitcoin.TransactionOutput{
			{
				Value:           int64(inputValueSats),
				PublicKeyScript: activeScriptPubKey,
			},
		},
		Locktime: 0,
	}
	bitcoinChain.transactions = append(bitcoinChain.transactions, prevTransaction)

	activeScriptHash := sha256.Sum256(activeScriptPubKey)
	revealer := "0x2222222222222222222222222222222222222222"
	reserve := "0x1111111111111111111111111111111111111111"
	vault := "0x3333333333333333333333333333333333333333"

	migrationDestination := &covenantsigner.MigrationDestinationReservation{
		ReservationID: "cmdr_self_1",
		Reserve:       reserve,
		Epoch:         12,
		Route:         covenantsigner.ReservationRouteMigration,
		Revealer:      revealer,
		Vault:         vault,
		Network:       "regtest",
		Status:        covenantsigner.ReservationStatusReserved,
		DepositScript: "0x" + hex.EncodeToString(destinationScript),
	}
	migrationDestination.DepositScriptHash = testDepositScriptHash(t, destinationScript)
	migrationDestination.MigrationExtraData = testMigrationExtraData(revealer)
	migrationDestination.DestinationCommitmentHash = testDestinationCommitmentHash(t, migrationDestination)

	request := covenantsigner.RouteSubmitRequest{
		FacadeRequestID:           "rf_self_1",
		IdempotencyKey:            "idem_self_1",
		Route:                     covenantsigner.TemplateSelfV1,
		Strategy:                  "0x1234",
		Reserve:                   reserve,
		Epoch:                     12,
		MaturityHeight:            maturityHeight,
		ActiveOutpoint:            covenantsigner.CovenantOutpoint{TxID: "0x" + prevTransaction.Hash().Hex(bitcoin.ReversedByteOrder), Vout: 0, ScriptHash: "0x" + hex.EncodeToString(activeScriptHash[:])},
		DestinationCommitmentHash: migrationDestination.DestinationCommitmentHash,
		MigrationDestination:      migrationDestination,
		MigrationTransactionPlan: &covenantsigner.MigrationTransactionPlan{
			InputValueSats:       inputValueSats,
			DestinationValueSats: destinationValueSats,
			AnchorValueSats:      anchorValueSats,
			FeeSats:              feeSats,
			InputSequence:        0xfffffffd,
			LockTime:             uint32(maturityHeight),
		},
		ArtifactSignatures: []string{"0x0708"},
		Artifacts:          map[covenantsigner.RecoveryPathID]covenantsigner.ArtifactRecord{},
		ScriptTemplate:     templateJSON,
		Signing: covenantsigner.SigningRequirements{
			SignerRequired:    true,
			CustodianRequired: false,
		},
	}
	applyTestMigrationTransactionPlanCommitment(t, &request)
	applyTestArtifactApprovals(t, &request, depositorPrivateKey, nil)

	result, err := service.Submit(context.Background(), covenantsigner.TemplateSelfV1, covenantsigner.SignerSubmitInput{
		RouteRequestID: "ors_self_ready",
		Stage:          covenantsigner.StageSignerCoordination,
		Request:        request,
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.Status != covenantsigner.StepStatusReady {
		t.Fatalf("expected READY, got %s", result.Status)
	}
	if result.PSBTHash == "" || result.TransactionHex == "" {
		t.Fatalf("expected final artifact payload, got %#v", result)
	}

	transactionBytes, err := hex.DecodeString(strings.TrimPrefix(result.TransactionHex, "0x"))
	if err != nil {
		t.Fatal(err)
	}

	transaction := &bitcoin.Transaction{}
	if err := transaction.Deserialize(transactionBytes); err != nil {
		t.Fatal(err)
	}

	if transaction.Locktime != uint32(maturityHeight) {
		t.Fatalf("unexpected locktime: %d", transaction.Locktime)
	}
	if len(transaction.Inputs) != 1 {
		t.Fatalf("unexpected input count: %d", len(transaction.Inputs))
	}
	if transaction.Inputs[0].Sequence != 0xfffffffd {
		t.Fatalf("unexpected input sequence: %x", transaction.Inputs[0].Sequence)
	}
	if len(transaction.Outputs) != 2 {
		t.Fatalf("unexpected output count: %d", len(transaction.Outputs))
	}
	if transaction.Outputs[0].Value != int64(destinationValueSats) {
		t.Fatalf("unexpected destination value: %d", transaction.Outputs[0].Value)
	}
	if !bytes.Equal(transaction.Outputs[0].PublicKeyScript, destinationScript) {
		t.Fatal("unexpected destination output script")
	}

	expectedAnchorScript, err := canonicalAnchorScriptPubKey()
	if err != nil {
		t.Fatal(err)
	}
	if transaction.Outputs[1].Value != int64(anchorValueSats) {
		t.Fatalf("unexpected anchor value: %d", transaction.Outputs[1].Value)
	}
	if !bytes.Equal(transaction.Outputs[1].PublicKeyScript, expectedAnchorScript) {
		t.Fatal("unexpected anchor output script")
	}

	if len(transaction.Inputs[0].Witness) != 4 {
		t.Fatalf("unexpected witness item count: %d", len(transaction.Inputs[0].Witness))
	}
	if !bytes.Equal(transaction.Inputs[0].Witness[1], []byte{0x01}) {
		t.Fatal("missing migration selector witness item")
	}
	if len(transaction.Inputs[0].Witness[2]) != 0 {
		t.Fatal("expected empty second selector witness item")
	}
	if !bytes.Equal(transaction.Inputs[0].Witness[3], witnessScript) {
		t.Fatal("unexpected witness script")
	}

	if result.PSBTHash != "0x"+transaction.WitnessHash().Hex(bitcoin.InternalByteOrder) {
		t.Fatalf("unexpected psbtHash: %s", result.PSBTHash)
	}

	signatureWithHashType := transaction.Inputs[0].Witness[0]
	if len(signatureWithHashType) == 0 || signatureWithHashType[len(signatureWithHashType)-1] != byte(txscript.SigHashAll) {
		t.Fatal("unexpected sighash type in witness signature")
	}

	wireTransaction := wire.NewMsgTx(wire.TxVersion)
	if err := wireTransaction.Deserialize(bytes.NewReader(transaction.Serialize(bitcoin.Witness))); err != nil {
		t.Fatal(err)
	}

	sighashBytes, err := txscript.CalcWitnessSigHash(
		witnessScript,
		txscript.NewTxSigHashes(wireTransaction),
		txscript.SigHashAll,
		wireTransaction,
		0,
		int64(inputValueSats),
	)
	if err != nil {
		t.Fatal(err)
	}

	parsedSignature, err := btcec.ParseDERSignature(signatureWithHashType[:len(signatureWithHashType)-1], btcec.S256())
	if err != nil {
		t.Fatal(err)
	}
	if !ecdsa.Verify(walletPublicKey, sighashBytes, parsedSignature.R, parsedSignature.S) {
		t.Fatal("invalid covenant signature")
	}
}

func TestCovenantSignerEngine_SubmitQcV1HandoffReady(t *testing.T) {
	node, bitcoinChain, walletPublicKey := setupCovenantSignerTestNode(t)

	service, err := covenantsigner.NewService(
		newCovenantSignerMemoryHandle(),
		newCovenantSignerEngine(node),
	)
	if err != nil {
		t.Fatal(err)
	}

	depositorPrivateKey, _ := btcec.PrivKeyFromBytes(btcec.S256(), bytes.Repeat([]byte{0x42}, 32))
	depositorPublicKey := depositorPrivateKey.PubKey().SerializeCompressed()
	custodianPrivateKey, _ := btcec.PrivKeyFromBytes(btcec.S256(), bytes.Repeat([]byte{0x24}, 32))
	custodianPublicKey := custodianPrivateKey.PubKey().SerializeCompressed()
	signerPublicKey := (*btcec.PublicKey)(walletPublicKey).SerializeCompressed()

	template := &covenantsigner.QcV1Template{
		Template:           covenantsigner.TemplateQcV1,
		DepositorPublicKey: "0x" + hex.EncodeToString(depositorPublicKey),
		CustodianPublicKey: "0x" + hex.EncodeToString(custodianPublicKey),
		SignerPublicKey:    "0x" + hex.EncodeToString(signerPublicKey),
		Beta:               144,
		Delta2:             4320,
	}
	templateJSON, err := json.Marshal(template)
	if err != nil {
		t.Fatal(err)
	}

	maturityHeight := uint64(912345)
	witnessScript, err := buildQcV1WitnessScript(template, maturityHeight)
	if err != nil {
		t.Fatal(err)
	}
	witnessScriptHash := bitcoin.WitnessScriptHash(witnessScript)
	activeScriptPubKey, err := bitcoin.PayToWitnessScriptHash(witnessScriptHash)
	if err != nil {
		t.Fatal(err)
	}

	destinationScript, err := bitcoin.PayToWitnessPublicKeyHash([20]byte{
		0xbb, 0xbb, 0xbb, 0xbb, 0xbb,
		0xbb, 0xbb, 0xbb, 0xbb, 0xbb,
		0xbb, 0xbb, 0xbb, 0xbb, 0xbb,
		0xbb, 0xbb, 0xbb, 0xbb, 0xbb,
	})
	if err != nil {
		t.Fatal(err)
	}

	const (
		inputValueSats       = uint64(2_000_000)
		destinationValueSats = uint64(1_997_500)
		anchorValueSats      = uint64(330)
		feeSats              = uint64(2_170)
	)

	prevTransaction := &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{
			{
				Outpoint: &bitcoin.TransactionOutpoint{},
				Sequence: 0xffffffff,
			},
		},
		Outputs: []*bitcoin.TransactionOutput{
			{
				Value:           int64(inputValueSats),
				PublicKeyScript: activeScriptPubKey,
			},
		},
		Locktime: 0,
	}
	bitcoinChain.transactions = append(bitcoinChain.transactions, prevTransaction)

	activeScriptHash := sha256.Sum256(activeScriptPubKey)
	revealer := "0x4444444444444444444444444444444444444444"
	reserve := "0x1111111111111111111111111111111111111111"
	vault := "0x3333333333333333333333333333333333333333"

	migrationDestination := &covenantsigner.MigrationDestinationReservation{
		ReservationID: "cmdr_qc_1",
		Reserve:       reserve,
		Epoch:         21,
		Route:         covenantsigner.ReservationRouteMigration,
		Revealer:      revealer,
		Vault:         vault,
		Network:       "regtest",
		Status:        covenantsigner.ReservationStatusCommittedToEpoch,
		DepositScript: "0x" + hex.EncodeToString(destinationScript),
	}
	migrationDestination.DepositScriptHash = testDepositScriptHash(t, destinationScript)
	migrationDestination.MigrationExtraData = testMigrationExtraData(revealer)
	migrationDestination.DestinationCommitmentHash = testDestinationCommitmentHash(t, migrationDestination)

	request := covenantsigner.RouteSubmitRequest{
		FacadeRequestID:           "rf_qc_1",
		IdempotencyKey:            "idem_qc_1",
		Route:                     covenantsigner.TemplateQcV1,
		Strategy:                  "0x1234",
		Reserve:                   reserve,
		Epoch:                     21,
		MaturityHeight:            maturityHeight,
		ActiveOutpoint:            covenantsigner.CovenantOutpoint{TxID: "0x" + prevTransaction.Hash().Hex(bitcoin.ReversedByteOrder), Vout: 0, ScriptHash: "0x" + hex.EncodeToString(activeScriptHash[:])},
		DestinationCommitmentHash: migrationDestination.DestinationCommitmentHash,
		MigrationDestination:      migrationDestination,
		MigrationTransactionPlan: &covenantsigner.MigrationTransactionPlan{
			InputValueSats:       inputValueSats,
			DestinationValueSats: destinationValueSats,
			AnchorValueSats:      anchorValueSats,
			FeeSats:              feeSats,
			InputSequence:        0xfffffffd,
			LockTime:             uint32(maturityHeight),
		},
		ArtifactSignatures: []string{"0x090a"},
		Artifacts:          map[covenantsigner.RecoveryPathID]covenantsigner.ArtifactRecord{},
		ScriptTemplate:     templateJSON,
		Signing: covenantsigner.SigningRequirements{
			SignerRequired:    true,
			CustodianRequired: true,
		},
	}
	applyTestMigrationTransactionPlanCommitment(t, &request)
	applyTestArtifactApprovals(t, &request, depositorPrivateKey, custodianPrivateKey)

	result, err := service.Submit(context.Background(), covenantsigner.TemplateQcV1, covenantsigner.SignerSubmitInput{
		RouteRequestID: "ors_qc_ready",
		Stage:          covenantsigner.StageSignerCoordination,
		Request:        request,
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.Status != covenantsigner.StepStatusReady {
		t.Fatalf("expected READY, got %s", result.Status)
	}
	if result.Handoff == nil {
		t.Fatal("expected handoff payload")
	}
	if result.TransactionHex != "" || result.PSBTHash != "" {
		t.Fatalf("expected handoff-only result, got %#v", result)
	}

	handoffKind, ok := result.Handoff["kind"].(string)
	if !ok || handoffKind != qcV1SignerHandoffKind {
		t.Fatalf("unexpected handoff kind: %#v", result.Handoff["kind"])
	}
	if signerRequestID, ok := result.Handoff["signerRequestId"].(string); !ok || signerRequestID != result.RequestID {
		t.Fatalf("unexpected signerRequestId: %#v", result.Handoff["signerRequestId"])
	}
	if requiresDummy, ok := result.Handoff["requiresDummy"].(bool); !ok || !requiresDummy {
		t.Fatalf("unexpected requiresDummy: %#v", result.Handoff["requiresDummy"])
	}
	if sighashType, ok := result.Handoff["sighashType"].(uint32); !ok || sighashType != uint32(txscript.SigHashAll) {
		t.Fatalf("unexpected sighashType: %#v", result.Handoff["sighashType"])
	}

	selectorWitnessItems, ok := result.Handoff["selectorWitnessItems"].([]string)
	if !ok {
		t.Fatalf("unexpected selector witness items type: %#v", result.Handoff["selectorWitnessItems"])
	}
	if len(selectorWitnessItems) != 2 || selectorWitnessItems[0] != "0x01" || selectorWitnessItems[1] != "0x" {
		t.Fatalf("unexpected selector witness items: %#v", selectorWitnessItems)
	}

	unsignedTransactionHex, ok := result.Handoff["unsignedTransactionHex"].(string)
	if !ok || unsignedTransactionHex == "" {
		t.Fatalf("unexpected unsignedTransactionHex: %#v", result.Handoff["unsignedTransactionHex"])
	}
	unsignedTransactionBytes, err := hex.DecodeString(strings.TrimPrefix(unsignedTransactionHex, "0x"))
	if err != nil {
		t.Fatal(err)
	}
	unsignedTransaction := &bitcoin.Transaction{}
	if err := unsignedTransaction.Deserialize(unsignedTransactionBytes); err != nil {
		t.Fatal(err)
	}

	if unsignedTransaction.Locktime != uint32(maturityHeight) {
		t.Fatalf("unexpected locktime: %d", unsignedTransaction.Locktime)
	}
	if len(unsignedTransaction.Inputs) != 1 {
		t.Fatalf("unexpected input count: %d", len(unsignedTransaction.Inputs))
	}
	if unsignedTransaction.Inputs[0].Sequence != 0xfffffffd {
		t.Fatalf("unexpected input sequence: %x", unsignedTransaction.Inputs[0].Sequence)
	}
	if len(unsignedTransaction.Outputs) != 2 {
		t.Fatalf("unexpected output count: %d", len(unsignedTransaction.Outputs))
	}
	if unsignedTransaction.Outputs[0].Value != int64(destinationValueSats) {
		t.Fatalf("unexpected destination value: %d", unsignedTransaction.Outputs[0].Value)
	}
	if !bytes.Equal(unsignedTransaction.Outputs[0].PublicKeyScript, destinationScript) {
		t.Fatal("unexpected destination output script")
	}

	expectedAnchorScript, err := canonicalAnchorScriptPubKey()
	if err != nil {
		t.Fatal(err)
	}
	if unsignedTransaction.Outputs[1].Value != int64(anchorValueSats) {
		t.Fatalf("unexpected anchor value: %d", unsignedTransaction.Outputs[1].Value)
	}
	if !bytes.Equal(unsignedTransaction.Outputs[1].PublicKeyScript, expectedAnchorScript) {
		t.Fatal("unexpected anchor output script")
	}

	witnessScriptHex, ok := result.Handoff["witnessScript"].(string)
	if !ok || witnessScriptHex == "" {
		t.Fatalf("unexpected witnessScript: %#v", result.Handoff["witnessScript"])
	}
	if witnessScriptHex != "0x"+hex.EncodeToString(witnessScript) {
		t.Fatalf("unexpected witness script hex: %s", witnessScriptHex)
	}

	signatureHex, ok := result.Handoff["signerSignature"].(string)
	if !ok || signatureHex == "" {
		t.Fatalf("unexpected signerSignature: %#v", result.Handoff["signerSignature"])
	}
	signatureBytes, err := hex.DecodeString(strings.TrimPrefix(signatureHex, "0x"))
	if err != nil {
		t.Fatal(err)
	}
	if len(signatureBytes) == 0 || signatureBytes[len(signatureBytes)-1] != byte(txscript.SigHashAll) {
		t.Fatal("unexpected sighash type in handoff signature")
	}

	wireTransaction := wire.NewMsgTx(wire.TxVersion)
	if err := wireTransaction.Deserialize(bytes.NewReader(unsignedTransaction.Serialize(bitcoin.Standard))); err != nil {
		t.Fatal(err)
	}
	sighashBytes, err := txscript.CalcWitnessSigHash(
		witnessScript,
		txscript.NewTxSigHashes(wireTransaction),
		txscript.SigHashAll,
		wireTransaction,
		0,
		int64(inputValueSats),
	)
	if err != nil {
		t.Fatal(err)
	}
	parsedSignature, err := btcec.ParseDERSignature(signatureBytes[:len(signatureBytes)-1], btcec.S256())
	if err != nil {
		t.Fatal(err)
	}
	if !ecdsa.Verify(walletPublicKey, sighashBytes, parsedSignature.R, parsedSignature.S) {
		t.Fatal("invalid qc_v1 signer handoff signature")
	}

	expectedPayloadHash, err := computeQcV1SignerHandoffPayloadHash(map[string]any{
		"kind":                      qcV1SignerHandoffKind,
		"unsignedTransactionHex":    unsignedTransactionHex,
		"witnessScript":             witnessScriptHex,
		"signerSignature":           signatureHex,
		"selectorWitnessItems":      selectorWitnessItems,
		"requiresDummy":             true,
		"sighashType":               uint32(txscript.SigHashAll),
		"destinationCommitmentHash": request.DestinationCommitmentHash,
	})
	if err != nil {
		t.Fatal(err)
	}

	if payloadHash, ok := result.Handoff["payloadHash"].(string); !ok || payloadHash != expectedPayloadHash {
		t.Fatalf("unexpected payloadHash: %#v", result.Handoff["payloadHash"])
	}
	if bundleID, ok := result.Handoff["bundleId"].(string); !ok || bundleID != expectedPayloadHash {
		t.Fatalf("unexpected bundleId: %#v", result.Handoff["bundleId"])
	}
	if destinationCommitmentHash, ok := result.Handoff["destinationCommitmentHash"].(string); !ok || destinationCommitmentHash != request.DestinationCommitmentHash {
		t.Fatalf("unexpected destinationCommitmentHash: %#v", result.Handoff["destinationCommitmentHash"])
	}
}

func TestCovenantSignerEngine_SubmitQcV1RejectsInvalidBeta(t *testing.T) {
	node, _, walletPublicKey := setupCovenantSignerTestNode(t)

	service, err := covenantsigner.NewService(
		newCovenantSignerMemoryHandle(),
		newCovenantSignerEngine(node),
	)
	if err != nil {
		t.Fatal(err)
	}

	depositorPrivateKey, _ := btcec.PrivKeyFromBytes(btcec.S256(), bytes.Repeat([]byte{0x42}, 32))
	depositorPublicKey := depositorPrivateKey.PubKey().SerializeCompressed()
	custodianPrivateKey, _ := btcec.PrivKeyFromBytes(btcec.S256(), bytes.Repeat([]byte{0x24}, 32))
	custodianPublicKey := custodianPrivateKey.PubKey().SerializeCompressed()
	signerPublicKey := (*btcec.PublicKey)(walletPublicKey).SerializeCompressed()

	template := &covenantsigner.QcV1Template{
		Template:           covenantsigner.TemplateQcV1,
		DepositorPublicKey: "0x" + hex.EncodeToString(depositorPublicKey),
		CustodianPublicKey: "0x" + hex.EncodeToString(custodianPublicKey),
		SignerPublicKey:    "0x" + hex.EncodeToString(signerPublicKey),
		Beta:               500,
		Delta2:             4320,
	}
	templateJSON, err := json.Marshal(template)
	if err != nil {
		t.Fatal(err)
	}

	destinationScript, err := bitcoin.PayToWitnessPublicKeyHash([20]byte{
		0xbb, 0xbb, 0xbb, 0xbb, 0xbb,
		0xbb, 0xbb, 0xbb, 0xbb, 0xbb,
		0xbb, 0xbb, 0xbb, 0xbb, 0xbb,
		0xbb, 0xbb, 0xbb, 0xbb, 0xbb,
	})
	if err != nil {
		t.Fatal(err)
	}

	revealer := "0x4444444444444444444444444444444444444444"
	reserve := "0x1111111111111111111111111111111111111111"
	vault := "0x3333333333333333333333333333333333333333"
	request := covenantsigner.RouteSubmitRequest{
		FacadeRequestID: "rf_qc_bad_beta",
		IdempotencyKey:  "idem_qc_bad_beta",
		Route:           covenantsigner.TemplateQcV1,
		Strategy:        "0x1234",
		Reserve:         reserve,
		Epoch:           21,
		MaturityHeight:  500,
		ActiveOutpoint: covenantsigner.CovenantOutpoint{
			TxID: "0x" + strings.Repeat("11", 32),
		},
		MigrationDestination: &covenantsigner.MigrationDestinationReservation{
			ReservationID: "cmdr_qc_bad_beta",
			Reserve:       reserve,
			Epoch:         21,
			Route:         covenantsigner.ReservationRouteMigration,
			Revealer:      revealer,
			Vault:         vault,
			Network:       "regtest",
			Status:        covenantsigner.ReservationStatusReserved,
			DepositScript: "0x" + hex.EncodeToString(destinationScript),
		},
		MigrationTransactionPlan: &covenantsigner.MigrationTransactionPlan{
			InputValueSats:       2_000_000,
			DestinationValueSats: 1_997_500,
			AnchorValueSats:      330,
			FeeSats:              2_170,
			InputSequence:        0xfffffffd,
			LockTime:             500,
		},
		ArtifactSignatures: []string{"0x090a"},
		Artifacts:          map[covenantsigner.RecoveryPathID]covenantsigner.ArtifactRecord{},
		ScriptTemplate:     templateJSON,
		Signing: covenantsigner.SigningRequirements{
			SignerRequired:    true,
			CustodianRequired: true,
		},
	}
	request.MigrationDestination.DepositScriptHash = testDepositScriptHash(t, destinationScript)
	request.MigrationDestination.MigrationExtraData = testMigrationExtraData(revealer)
	request.MigrationDestination.DestinationCommitmentHash = testDestinationCommitmentHash(t, request.MigrationDestination)
	request.DestinationCommitmentHash = request.MigrationDestination.DestinationCommitmentHash
	applyTestMigrationTransactionPlanCommitment(t, &request)
	applyTestArtifactApprovals(t, &request, depositorPrivateKey, custodianPrivateKey)

	result, err := service.Submit(context.Background(), covenantsigner.TemplateQcV1, covenantsigner.SignerSubmitInput{
		RouteRequestID: "ors_qc_bad_beta",
		Stage:          covenantsigner.StageSignerCoordination,
		Request:        request,
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.Status != covenantsigner.StepStatusFailed {
		t.Fatalf("expected FAILED, got %s", result.Status)
	}
	if result.Reason != covenantsigner.ReasonInvalidInput {
		t.Fatalf("unexpected failure reason: %s", result.Reason)
	}
	if !strings.Contains(result.Detail, "qc_v1 beta must be below maturity height") {
		t.Fatalf("unexpected failure detail: %s", result.Detail)
	}
}

func TestCovenantSignerEngine_SubmitQcV1RejectsScriptHashMismatch(t *testing.T) {
	node, bitcoinChain, walletPublicKey := setupCovenantSignerTestNode(t)

	service, err := covenantsigner.NewService(
		newCovenantSignerMemoryHandle(),
		newCovenantSignerEngine(node),
	)
	if err != nil {
		t.Fatal(err)
	}

	depositorPrivateKey, _ := btcec.PrivKeyFromBytes(btcec.S256(), bytes.Repeat([]byte{0x42}, 32))
	depositorPublicKey := depositorPrivateKey.PubKey().SerializeCompressed()
	custodianPrivateKey, _ := btcec.PrivKeyFromBytes(btcec.S256(), bytes.Repeat([]byte{0x24}, 32))
	custodianPublicKey := custodianPrivateKey.PubKey().SerializeCompressed()
	signerPublicKey := (*btcec.PublicKey)(walletPublicKey).SerializeCompressed()

	template := &covenantsigner.QcV1Template{
		Template:           covenantsigner.TemplateQcV1,
		DepositorPublicKey: "0x" + hex.EncodeToString(depositorPublicKey),
		CustodianPublicKey: "0x" + hex.EncodeToString(custodianPublicKey),
		SignerPublicKey:    "0x" + hex.EncodeToString(signerPublicKey),
		Beta:               144,
		Delta2:             4320,
	}
	templateJSON, err := json.Marshal(template)
	if err != nil {
		t.Fatal(err)
	}

	maturityHeight := uint64(912345)
	witnessScript, err := buildQcV1WitnessScript(template, maturityHeight)
	if err != nil {
		t.Fatal(err)
	}
	witnessScriptHash := bitcoin.WitnessScriptHash(witnessScript)
	activeScriptPubKey, err := bitcoin.PayToWitnessScriptHash(witnessScriptHash)
	if err != nil {
		t.Fatal(err)
	}

	destinationScript, err := bitcoin.PayToWitnessPublicKeyHash([20]byte{
		0xbb, 0xbb, 0xbb, 0xbb, 0xbb,
		0xbb, 0xbb, 0xbb, 0xbb, 0xbb,
		0xbb, 0xbb, 0xbb, 0xbb, 0xbb,
		0xbb, 0xbb, 0xbb, 0xbb, 0xbb,
	})
	if err != nil {
		t.Fatal(err)
	}

	prevTransaction := &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{
			{
				Outpoint: &bitcoin.TransactionOutpoint{},
				Sequence: 0xffffffff,
			},
		},
		Outputs: []*bitcoin.TransactionOutput{
			{
				Value:           2_000_000,
				PublicKeyScript: activeScriptPubKey,
			},
		},
		Locktime: 0,
	}
	bitcoinChain.transactions = append(bitcoinChain.transactions, prevTransaction)

	revealer := "0x4444444444444444444444444444444444444444"
	reserve := "0x1111111111111111111111111111111111111111"
	vault := "0x3333333333333333333333333333333333333333"
	migrationDestination := &covenantsigner.MigrationDestinationReservation{
		ReservationID: "cmdr_qc_bad_script_hash",
		Reserve:       reserve,
		Epoch:         21,
		Route:         covenantsigner.ReservationRouteMigration,
		Revealer:      revealer,
		Vault:         vault,
		Network:       "regtest",
		Status:        covenantsigner.ReservationStatusCommittedToEpoch,
		DepositScript: "0x" + hex.EncodeToString(destinationScript),
	}
	migrationDestination.DepositScriptHash = testDepositScriptHash(t, destinationScript)
	migrationDestination.MigrationExtraData = testMigrationExtraData(revealer)
	migrationDestination.DestinationCommitmentHash = testDestinationCommitmentHash(t, migrationDestination)

	request := covenantsigner.RouteSubmitRequest{
		FacadeRequestID:           "rf_qc_bad_script_hash",
		IdempotencyKey:            "idem_qc_bad_script_hash",
		Route:                     covenantsigner.TemplateQcV1,
		Strategy:                  "0x1234",
		Reserve:                   reserve,
		Epoch:                     21,
		MaturityHeight:            maturityHeight,
		ActiveOutpoint:            covenantsigner.CovenantOutpoint{TxID: "0x" + prevTransaction.Hash().Hex(bitcoin.ReversedByteOrder), Vout: 0, ScriptHash: "0x" + strings.Repeat("aa", 32)},
		DestinationCommitmentHash: migrationDestination.DestinationCommitmentHash,
		MigrationDestination:      migrationDestination,
		MigrationTransactionPlan: &covenantsigner.MigrationTransactionPlan{
			InputValueSats:       2_000_000,
			DestinationValueSats: 1_997_500,
			AnchorValueSats:      330,
			FeeSats:              2_170,
			InputSequence:        0xfffffffd,
			LockTime:             uint32(maturityHeight),
		},
		ArtifactSignatures: []string{"0x090a"},
		Artifacts:          map[covenantsigner.RecoveryPathID]covenantsigner.ArtifactRecord{},
		ScriptTemplate:     templateJSON,
		Signing: covenantsigner.SigningRequirements{
			SignerRequired:    true,
			CustodianRequired: true,
		},
	}
	applyTestMigrationTransactionPlanCommitment(t, &request)
	applyTestArtifactApprovals(t, &request, depositorPrivateKey, custodianPrivateKey)

	result, err := service.Submit(context.Background(), covenantsigner.TemplateQcV1, covenantsigner.SignerSubmitInput{
		RouteRequestID: "ors_qc_bad_script_hash",
		Stage:          covenantsigner.StageSignerCoordination,
		Request:        request,
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.Status != covenantsigner.StepStatusFailed {
		t.Fatalf("expected FAILED, got %s", result.Status)
	}
	if result.Reason != covenantsigner.ReasonInvalidInput {
		t.Fatalf("unexpected failure reason: %s", result.Reason)
	}
	if !strings.Contains(result.Detail, "active outpoint script hash does not match qc_v1 template") {
		t.Fatalf("unexpected failure detail: %s", result.Detail)
	}
}

func TestCovenantSignerEngine_SubmitSelfV1RejectsZeroMaturityHeight(t *testing.T) {
	node, _, walletPublicKey := setupCovenantSignerTestNode(t)

	service, err := covenantsigner.NewService(
		newCovenantSignerMemoryHandle(),
		newCovenantSignerEngine(node),
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

	destinationScript, err := bitcoin.PayToWitnessPublicKeyHash([20]byte{
		0xaa, 0xaa, 0xaa, 0xaa, 0xaa,
		0xaa, 0xaa, 0xaa, 0xaa, 0xaa,
		0xaa, 0xaa, 0xaa, 0xaa, 0xaa,
		0xaa, 0xaa, 0xaa, 0xaa, 0xaa,
	})
	if err != nil {
		t.Fatal(err)
	}

	revealer := "0x2222222222222222222222222222222222222222"
	reserve := "0x1111111111111111111111111111111111111111"
	vault := "0x3333333333333333333333333333333333333333"
	request := covenantsigner.RouteSubmitRequest{
		FacadeRequestID: "rf_self_zero",
		IdempotencyKey:  "idem_self_zero",
		Route:           covenantsigner.TemplateSelfV1,
		Strategy:        "0x1234",
		Reserve:         reserve,
		Epoch:           12,
		MaturityHeight:  0,
		ActiveOutpoint: covenantsigner.CovenantOutpoint{
			TxID: "0x" + strings.Repeat("11", 32),
		},
		MigrationDestination: &covenantsigner.MigrationDestinationReservation{
			ReservationID: "cmdr_self_zero",
			Reserve:       reserve,
			Epoch:         12,
			Route:         covenantsigner.ReservationRouteMigration,
			Revealer:      revealer,
			Vault:         vault,
			Network:       "regtest",
			Status:        covenantsigner.ReservationStatusReserved,
			DepositScript: "0x" + hex.EncodeToString(destinationScript),
		},
		MigrationTransactionPlan: &covenantsigner.MigrationTransactionPlan{
			InputValueSats:       1_000_000,
			DestinationValueSats: 998_000,
			AnchorValueSats:      330,
			FeeSats:              1_670,
			InputSequence:        0xfffffffd,
			LockTime:             0,
		},
		ArtifactSignatures: []string{"0x0708"},
		Artifacts:          map[covenantsigner.RecoveryPathID]covenantsigner.ArtifactRecord{},
		ScriptTemplate:     templateJSON,
		Signing: covenantsigner.SigningRequirements{
			SignerRequired:    true,
			CustodianRequired: false,
		},
	}
	request.MigrationDestination.DepositScriptHash = testDepositScriptHash(t, destinationScript)
	request.MigrationDestination.MigrationExtraData = testMigrationExtraData(revealer)
	request.MigrationDestination.DestinationCommitmentHash = testDestinationCommitmentHash(t, request.MigrationDestination)
	request.DestinationCommitmentHash = request.MigrationDestination.DestinationCommitmentHash
	applyTestMigrationTransactionPlanCommitment(t, &request)
	applyTestArtifactApprovals(t, &request, depositorPrivateKey, nil)

	result, err := service.Submit(context.Background(), covenantsigner.TemplateSelfV1, covenantsigner.SignerSubmitInput{
		RouteRequestID: "ors_self_zero",
		Stage:          covenantsigner.StageSignerCoordination,
		Request:        request,
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.Status != covenantsigner.StepStatusFailed {
		t.Fatalf("expected FAILED, got %s", result.Status)
	}
	if result.Reason != covenantsigner.ReasonInvalidInput {
		t.Fatalf("unexpected failure reason: %s", result.Reason)
	}
	if !strings.Contains(result.Detail, "maturity height must be greater than zero") {
		t.Fatalf("unexpected failure detail: %s", result.Detail)
	}
}

func TestValidateMigrationOutputValues_RejectsValuesExceedingInt64(t *testing.T) {
	err := validateMigrationOutputValues(covenantsigner.RouteSubmitRequest{
		MigrationTransactionPlan: &covenantsigner.MigrationTransactionPlan{
			DestinationValueSats: uint64(math.MaxInt64) + 1,
			AnchorValueSats:      330,
		},
	})
	if err == nil {
		t.Fatal("expected output value validation error")
	}
	if !strings.Contains(err.Error(), "migration destination value exceeds bitcoin output value range") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func setupCovenantSignerTestNode(
	t *testing.T,
) (*node, *localBitcoinChain, *ecdsa.PublicKey) {
	t.Helper()

	groupParameters := &GroupParameters{
		GroupSize:       5,
		GroupQuorum:     4,
		HonestThreshold: 3,
	}

	operatorPrivateKey, operatorPublicKey, err := operator.GenerateKeyPair(local_v1.DefaultCurve)
	if err != nil {
		t.Fatal(err)
	}

	localChain := ConnectWithKey(operatorPrivateKey)
	localProvider := local.ConnectWithKey(operatorPublicKey)
	bitcoinChain := newLocalBitcoinChain()

	operatorAddress, err := localChain.Signing().PublicKeyToAddress(operatorPublicKey)
	if err != nil {
		t.Fatal(err)
	}

	var operators []chain.Address
	for i := 0; i < groupParameters.GroupSize; i++ {
		operators = append(operators, operatorAddress)
	}

	testData, err := tecdsatest.LoadPrivateKeyShareTestFixtures(groupParameters.GroupSize)
	if err != nil {
		t.Fatalf("failed to load test data: [%v]", err)
	}

	signers := make([]*signer, len(testData))
	for i := range testData {
		privateKeyShare := tecdsa.NewPrivateKeyShare(testData[i])
		signers[i] = &signer{
			wallet: wallet{
				publicKey:             privateKeyShare.PublicKey(),
				signingGroupOperators: operators,
			},
			signingGroupMemberIndex: group.MemberIndex(i + 1),
			privateKeyShare:         privateKeyShare,
		}
	}

	walletPublicKeyHash := bitcoin.PublicKeyHash(signers[0].wallet.publicKey)
	walletID, err := localChain.CalculateWalletID(signers[0].wallet.publicKey)
	if err != nil {
		t.Fatal(err)
	}
	membersIDsHash := sha256.Sum256([]byte("covenant-signer-test-members"))

	localChain.setWallet(
		walletPublicKeyHash,
		&WalletChainData{
			EcdsaWalletID:  walletID,
			MembersIDsHash: membersIDsHash,
			State:          StateLive,
		},
	)

	node, err := newNode(
		groupParameters,
		localChain,
		bitcoinChain,
		localProvider,
		createMockKeyStorePersistence(t, signers...),
		&mockPersistenceHandle{},
		generator.StartScheduler(),
		&mockCoordinationProposalGenerator{},
		Config{},
	)
	if err != nil {
		t.Fatal(err)
	}

	executor, ok, err := node.getSigningExecutor(signers[0].wallet.publicKey)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("node is supposed to control wallet signers")
	}
	executor.signingAttemptsLimit *= 8

	return node, bitcoinChain, signers[0].wallet.publicKey
}

func testMigrationExtraData(revealer string) string {
	return "0x" + hex.EncodeToString([]byte("AC_MIGRATEV1")) + strings.TrimPrefix(strings.ToLower(revealer), "0x")
}

func testDepositScriptHash(t *testing.T, depositScript bitcoin.Script) string {
	t.Helper()

	sum := sha256.Sum256(depositScript)
	return "0x" + hex.EncodeToString(sum[:])
}

type testDestinationCommitmentPayload struct {
	Reserve            string `json:"reserve"`
	Epoch              uint64 `json:"epoch"`
	Route              string `json:"route"`
	Revealer           string `json:"revealer"`
	Vault              string `json:"vault"`
	Network            string `json:"network"`
	DepositScriptHash  string `json:"depositScriptHash"`
	MigrationExtraData string `json:"migrationExtraData"`
}

func testDestinationCommitmentHash(
	t *testing.T,
	reservation *covenantsigner.MigrationDestinationReservation,
) string {
	t.Helper()

	payload, err := json.Marshal(testDestinationCommitmentPayload{
		Reserve:            strings.ToLower(reservation.Reserve),
		Epoch:              reservation.Epoch,
		Route:              string(reservation.Route),
		Revealer:           strings.ToLower(reservation.Revealer),
		Vault:              strings.ToLower(reservation.Vault),
		Network:            strings.TrimSpace(reservation.Network),
		DepositScriptHash:  strings.ToLower(reservation.DepositScriptHash),
		MigrationExtraData: strings.ToLower(reservation.MigrationExtraData),
	})
	if err != nil {
		t.Fatal(err)
	}

	sum := sha256.Sum256(payload)
	return "0x" + hex.EncodeToString(sum[:])
}

type testMigrationTransactionPlanCommitmentPayload struct {
	PlanVersion               uint32 `json:"planVersion"`
	Reserve                   string `json:"reserve"`
	Epoch                     uint64 `json:"epoch"`
	ActiveOutpointTxID        string `json:"activeOutpointTxid"`
	ActiveOutpointVout        uint32 `json:"activeOutpointVout"`
	DestinationCommitmentHash string `json:"destinationCommitmentHash"`
	InputValueSats            uint64 `json:"inputValueSats"`
	DestinationValueSats      uint64 `json:"destinationValueSats"`
	AnchorValueSats           uint64 `json:"anchorValueSats"`
	FeeSats                   uint64 `json:"feeSats"`
	InputSequence             uint32 `json:"inputSequence"`
	LockTime                  uint32 `json:"lockTime"`
}

func testMigrationTransactionPlanCommitmentHash(
	t *testing.T,
	request covenantsigner.RouteSubmitRequest,
	plan *covenantsigner.MigrationTransactionPlan,
) string {
	t.Helper()

	payload, err := json.Marshal(testMigrationTransactionPlanCommitmentPayload{
		PlanVersion:               plan.PlanVersion,
		Reserve:                   strings.ToLower(request.Reserve),
		Epoch:                     request.Epoch,
		ActiveOutpointTxID:        strings.ToLower(request.ActiveOutpoint.TxID),
		ActiveOutpointVout:        request.ActiveOutpoint.Vout,
		DestinationCommitmentHash: strings.ToLower(request.DestinationCommitmentHash),
		InputValueSats:            plan.InputValueSats,
		DestinationValueSats:      plan.DestinationValueSats,
		AnchorValueSats:           plan.AnchorValueSats,
		FeeSats:                   plan.FeeSats,
		InputSequence:             plan.InputSequence,
		LockTime:                  plan.LockTime,
	})
	if err != nil {
		t.Fatal(err)
	}

	sum := sha256.Sum256(payload)
	return "0x" + hex.EncodeToString(sum[:])
}

var testArtifactApprovalTypeHash = crypto.Keccak256Hash([]byte(
	"ArtifactApproval(" +
		"uint8 approvalVersion," +
		"bytes32 route," +
		"bytes32 scriptTemplateId," +
		"bytes32 destinationCommitmentHash," +
		"bytes32 planCommitmentHash)",
))

func testArtifactApprovalDigest(
	t *testing.T,
	payload covenantsigner.ArtifactApprovalPayload,
) []byte {
	t.Helper()

	decodeBytes32 := func(name string, value string) [32]byte {
		t.Helper()

		rawValue, err := hex.DecodeString(strings.TrimPrefix(value, "0x"))
		if err != nil {
			t.Fatalf("cannot decode %s: %v", name, err)
		}
		if len(rawValue) != 32 {
			t.Fatalf("expected %s to be 32 bytes, got %d", name, len(rawValue))
		}

		var decoded [32]byte
		copy(decoded[:], rawValue)
		return decoded
	}

	encodeUint32 := func(value uint32) [32]byte {
		var encoded [32]byte
		binary.BigEndian.PutUint32(encoded[28:], value)
		return encoded
	}

	keccakTemplateIdentifier := func(value covenantsigner.TemplateID) [32]byte {
		hash := crypto.Keccak256Hash([]byte(value))
		var encoded [32]byte
		copy(encoded[:], hash.Bytes())
		return encoded
	}

	destinationCommitmentHash := decodeBytes32(
		"destinationCommitmentHash",
		payload.DestinationCommitmentHash,
	)
	planCommitmentHash := decodeBytes32(
		"planCommitmentHash",
		payload.PlanCommitmentHash,
	)
	approvalVersionWord := encodeUint32(payload.ApprovalVersion)
	routeIdentifier := keccakTemplateIdentifier(payload.Route)
	scriptTemplateIdentifier := keccakTemplateIdentifier(payload.ScriptTemplateID)

	encoded := make([]byte, 32*6)
	copy(encoded[0:32], testArtifactApprovalTypeHash.Bytes())
	copy(encoded[32:64], approvalVersionWord[:])
	copy(encoded[64:96], routeIdentifier[:])
	copy(encoded[96:128], scriptTemplateIdentifier[:])
	copy(encoded[128:160], destinationCommitmentHash[:])
	copy(encoded[160:192], planCommitmentHash[:])

	digest := crypto.Keccak256Hash(encoded)
	return digest.Bytes()
}

func testSignArtifactApproval(
	t *testing.T,
	privateKey *btcec.PrivateKey,
	payload covenantsigner.ArtifactApprovalPayload,
) string {
	t.Helper()

	signature, err := privateKey.Sign(testArtifactApprovalDigest(t, payload))
	if err != nil {
		t.Fatal(err)
	}

	return "0x" + hex.EncodeToString(signature.Serialize())
}

func applyTestArtifactApprovals(
	t *testing.T,
	request *covenantsigner.RouteSubmitRequest,
	depositorPrivateKey *btcec.PrivateKey,
	custodianPrivateKey *btcec.PrivateKey,
) {
	t.Helper()

	payload := covenantsigner.ArtifactApprovalPayload{
		ApprovalVersion:           1,
		Route:                     request.Route,
		ScriptTemplateID:          request.Route,
		DestinationCommitmentHash: request.DestinationCommitmentHash,
		PlanCommitmentHash:        request.MigrationTransactionPlan.PlanCommitmentHash,
	}

	approvals := []covenantsigner.ArtifactRoleApproval{
		{
			Role:      covenantsigner.ArtifactApprovalRoleDepositor,
			Signature: testSignArtifactApproval(t, depositorPrivateKey, payload),
		},
		{
			Role:      covenantsigner.ArtifactApprovalRoleSigner,
			Signature: "0x5151",
		},
	}

	if request.Route == covenantsigner.TemplateQcV1 {
		approvals = []covenantsigner.ArtifactRoleApproval{
			{
				Role:      covenantsigner.ArtifactApprovalRoleDepositor,
				Signature: testSignArtifactApproval(t, depositorPrivateKey, payload),
			},
			{
				Role:      covenantsigner.ArtifactApprovalRoleCustodian,
				Signature: testSignArtifactApproval(t, custodianPrivateKey, payload),
			},
			{
				Role:      covenantsigner.ArtifactApprovalRoleSigner,
				Signature: "0x5151",
			},
		}
	}

	request.ArtifactApprovals = &covenantsigner.ArtifactApprovalEnvelope{
		Payload:   payload,
		Approvals: approvals,
	}
	request.ArtifactSignatures = make([]string, len(approvals))
	for i, approval := range approvals {
		request.ArtifactSignatures[i] = approval.Signature
	}
}

func applyTestMigrationTransactionPlanCommitment(
	t *testing.T,
	request *covenantsigner.RouteSubmitRequest,
) {
	t.Helper()

	if request.MigrationTransactionPlan == nil {
		return
	}

	request.MigrationTransactionPlan.PlanVersion = 1
	request.MigrationTransactionPlan.PlanCommitmentHash = testMigrationTransactionPlanCommitmentHash(
		t,
		*request,
		request.MigrationTransactionPlan,
	)
}
