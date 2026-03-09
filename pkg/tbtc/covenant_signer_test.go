package tbtc

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
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
			LockTime:             maturityHeight,
		},
		ArtifactSignatures: []string{"0x0708"},
		Artifacts:          map[covenantsigner.RecoveryPathID]covenantsigner.ArtifactRecord{},
		ScriptTemplate:     templateJSON,
		Signing: covenantsigner.SigningRequirements{
			SignerRequired:    true,
			CustodianRequired: false,
		},
	}

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

func TestValidateSelfV1OutputValues_RejectsValuesExceedingInt64(t *testing.T) {
	err := validateSelfV1OutputValues(covenantsigner.RouteSubmitRequest{
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

	localChain.setWallet(
		walletPublicKeyHash,
		&WalletChainData{
			EcdsaWalletID: walletID,
			State:         StateLive,
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
