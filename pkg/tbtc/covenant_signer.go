package tbtc

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/txscript"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/covenantsigner"
	"github.com/keep-network/keep-core/pkg/internal/canonicaljson"
	"github.com/keep-network/keep-core/pkg/tecdsa"
)

type covenantSignerEngine struct {
	node                               *node
	minimumActiveOutpointConfirmations uint
}

// defaultMinActiveOutpointConfirmations is the confirmation threshold applied
// when the operator config does not specify a custom value. It aligns with
// DepositSweepRequiredFundingTxConfirmations to ensure consistent reorg safety
// across the tBTC subsystem.
const defaultMinActiveOutpointConfirmations uint = 6

const qcV1SignerHandoffKind = "qc_v1_signer_handoff_v1"

// qcV1SignerHandoff is the downstream handoff artifact produced for qc_v1
// signing jobs. Its field names and the qcV1SignerHandoffKind constant form
// part of the external handoff API contract consumed by the handoff
// processor. Do not rename fields or change the Kind value without a
// corresponding schema version bump.
type qcV1SignerHandoff struct {
	Kind                      string
	SignerRequestID           string
	BundleID                  string
	DestinationCommitmentHash string
	PayloadHash               string
	UnsignedTransactionHex    string
	WitnessScript             string
	SignerSignature           string
	SelectorWitnessItems      []string
	RequiresDummy             bool
	SighashType               uint32
}

// newCovenantSignerEngine creates a covenant signer engine bound to the given
// node. When minConfirmations is zero (the Go zero-value produced by an unset
// config field), defaultMinActiveOutpointConfirmations is used.
func newCovenantSignerEngine(node *node, minConfirmations uint) covenantsigner.Engine {
	if minConfirmations == 0 {
		minConfirmations = defaultMinActiveOutpointConfirmations
	}

	return &covenantSignerEngine{
		node:                               node,
		minimumActiveOutpointConfirmations: minConfirmations,
	}
}

func (cse *covenantSignerEngine) VerifySignerApproval(
	request covenantsigner.RouteSubmitRequest,
) error {
	if request.SignerApproval == nil {
		return covenantsigner.NewInputError(
			"request.signerApproval is required for signer approval verification",
		)
	}
	if request.ArtifactApprovals == nil {
		return covenantsigner.NewInputError(
			"request.artifactApprovals is required for signer approval verification",
		)
	}

	expectedApprovalDigest, err := covenantsigner.ComputeArtifactApprovalDigest(
		request.ArtifactApprovals.Payload,
	)
	if err != nil {
		return covenantsigner.NewInputError(
			fmt.Sprintf(
				"request.artifactApprovals.payload is invalid for signer approval verification: %v",
				err,
			),
		)
	}
	if !strings.EqualFold(
		request.SignerApproval.ApprovalDigest,
		"0x"+hex.EncodeToString(expectedApprovalDigest),
	) {
		return covenantsigner.NewInputError(
			"request.signerApproval.approvalDigest must match request.artifactApprovals.payload",
		)
	}

	signerPublicKey, err := cse.resolveSignerApprovalTemplatePublicKey(request)
	if err != nil {
		return covenantsigner.NewInputError(err.Error())
	}

	expectedWalletPublicKeyBytes, err := marshalPublicKey(signerPublicKey)
	if err != nil {
		return fmt.Errorf(
			"cannot marshal signer public key for signer approval verification: %w",
			err,
		)
	}

	expectedWalletPublicKey := "0x" + hex.EncodeToString(expectedWalletPublicKeyBytes)
	if !strings.EqualFold(
		request.SignerApproval.WalletPublicKey,
		expectedWalletPublicKey,
	) {
		return covenantsigner.NewInputError(
			"request.signerApproval.walletPublicKey must match request.scriptTemplate.signerPublicKey",
		)
	}

	walletChainData, err := cse.node.chain.GetWallet(
		bitcoin.PublicKeyHash(signerPublicKey),
	)
	if err != nil {
		if errors.Is(err, ErrWalletNotFound) {
			return covenantsigner.NewInputError(
				"request.signerApproval.walletPublicKey must resolve to a registered on-chain wallet",
			)
		}

		return fmt.Errorf(
			"cannot resolve on-chain wallet for signer approval verification: %w",
			err,
		)
	}
	if err := ensureWalletRegistryDataAvailable(
		walletChainData,
		"verify signer approval",
	); err != nil {
		return err
	}

	expectedSignerSetHash, err := computeSignerApprovalCertificateSignerSetHash(
		signerPublicKey,
		walletChainData,
		cse.node.groupParameters,
	)
	if err != nil {
		if errors.Is(err, ErrMissingWalletID) || errors.Is(err, ErrMissingMembersIDsHash) {
			return fmt.Errorf(
				"wallet registry unavailable; signer approval verification requires registry data: %w",
				err,
			)
		}
		return fmt.Errorf(
			"cannot compute signer approval signer set hash: %w",
			err,
		)
	}

	if err := verifySignerApprovalCertificate(
		request.SignerApproval,
		expectedSignerSetHash,
	); err != nil {
		return covenantsigner.NewInputError(
			fmt.Sprintf("request.signerApproval is invalid: %v", err),
		)
	}

	return nil
}

func (cse *covenantSignerEngine) resolveSignerApprovalTemplatePublicKey(
	request covenantsigner.RouteSubmitRequest,
) (*ecdsa.PublicKey, error) {
	switch request.Route {
	case covenantsigner.TemplateSelfV1:
		template, err := decodeSelfV1Template(request.ScriptTemplate)
		if err != nil {
			return nil, err
		}
		return parseCompressedPublicKey(template.SignerPublicKey)
	case covenantsigner.TemplateQcV1:
		template, err := decodeQcV1Template(request.ScriptTemplate)
		if err != nil {
			return nil, err
		}
		return parseCompressedPublicKey(template.SignerPublicKey)
	default:
		return nil, fmt.Errorf("unsupported covenant route")
	}
}

func (cse *covenantSignerEngine) OnSubmit(
	ctx context.Context,
	job *covenantsigner.Job,
) (*covenantsigner.Transition, error) {
	switch job.Route {
	case covenantsigner.TemplateSelfV1:
		return cse.submitSelfV1(ctx, job), nil
	case covenantsigner.TemplateQcV1:
		return cse.submitQcV1(ctx, job), nil
	default:
		return &covenantsigner.Transition{
			State:  covenantsigner.JobStateFailed,
			Reason: covenantsigner.ReasonInvalidInput,
			Detail: "unsupported covenant route",
		}, nil
	}
}

func (cse *covenantSignerEngine) OnPoll(
	context.Context,
	*covenantsigner.Job,
) (*covenantsigner.Transition, error) {
	return nil, nil
}

func (cse *covenantSignerEngine) submitSelfV1(
	ctx context.Context,
	job *covenantsigner.Job,
) *covenantsigner.Transition {
	template, err := decodeSelfV1Template(job.Request.ScriptTemplate)
	if err != nil {
		return failedTransition(covenantsigner.ReasonInvalidInput, err.Error())
	}

	walletPublicKey, err := parseCompressedPublicKey(template.SignerPublicKey)
	if err != nil {
		return failedTransition(covenantsigner.ReasonInvalidInput, "invalid self_v1 signer public key")
	}

	signingExecutor, ok, err := cse.node.getSigningExecutor(walletPublicKey)
	if err != nil {
		return failedTransition(covenantsigner.ReasonProviderFailed, fmt.Sprintf("cannot resolve signing executor: %v", err))
	}
	if !ok {
		return failedTransition(covenantsigner.ReasonPolicyRejected, "wallet is not controlled by this node")
	}

	witnessScript, err := buildSelfV1WitnessScript(template, job.Request.MaturityHeight)
	if err != nil {
		return failedTransition(covenantsigner.ReasonInvalidInput, err.Error())
	}

	activeUtxo, err := cse.resolveSelfV1ActiveUtxo(job.Request, witnessScript)
	if err != nil {
		return failedTransition(covenantsigner.ReasonInvalidInput, err.Error())
	}
	if err := validateMigrationOutputValues(job.Request); err != nil {
		return failedTransition(covenantsigner.ReasonInvalidInput, err.Error())
	}

	transaction, err := cse.buildAndSignSelfV1Transaction(
		ctx,
		signingExecutor,
		job.Request,
		activeUtxo,
		witnessScript,
	)
	if err != nil {
		return failedTransition(covenantsigner.ReasonProviderFailed, err.Error())
	}

	transactionHex := "0x" + hex.EncodeToString(transaction.Serialize(bitcoin.Witness))

	// Until the wider stack standardizes a PSBT-native artifact hash,
	// return a deterministic 32-byte artifact identifier derived from the
	// final witness transaction serialization.
	psbtHash := "0x" + transaction.WitnessHash().Hex(bitcoin.InternalByteOrder)

	return &covenantsigner.Transition{
		State: covenantsigner.JobStateArtifactReady,
		Detail: func() string {
			if job.Request.RequestType == covenantsigner.RequestTypePresignSelfV1 {
				return "self_v1 presign artifact ready"
			}
			return "self_v1 artifact ready"
		}(),
		PSBTHash:       psbtHash,
		TransactionHex: transactionHex,
	}
}

func (cse *covenantSignerEngine) submitQcV1(
	ctx context.Context,
	job *covenantsigner.Job,
) *covenantsigner.Transition {
	template, err := decodeQcV1Template(job.Request.ScriptTemplate)
	if err != nil {
		return failedTransition(covenantsigner.ReasonInvalidInput, err.Error())
	}

	walletPublicKey, err := parseCompressedPublicKey(template.SignerPublicKey)
	if err != nil {
		return failedTransition(covenantsigner.ReasonInvalidInput, "invalid qc_v1 signer public key")
	}

	signingExecutor, ok, err := cse.node.getSigningExecutor(walletPublicKey)
	if err != nil {
		return failedTransition(covenantsigner.ReasonProviderFailed, fmt.Sprintf("cannot resolve signing executor: %v", err))
	}
	if !ok {
		return failedTransition(covenantsigner.ReasonPolicyRejected, "wallet is not controlled by this node")
	}

	witnessScript, err := buildQcV1WitnessScript(template, job.Request.MaturityHeight)
	if err != nil {
		return failedTransition(covenantsigner.ReasonInvalidInput, err.Error())
	}

	activeUtxo, err := cse.resolveQcV1ActiveUtxo(job.Request, witnessScript)
	if err != nil {
		return failedTransition(covenantsigner.ReasonInvalidInput, err.Error())
	}
	if err := validateMigrationOutputValues(job.Request); err != nil {
		return failedTransition(covenantsigner.ReasonInvalidInput, err.Error())
	}

	handoff, err := cse.buildQcV1SignerHandoff(
		ctx,
		job.RequestID,
		signingExecutor,
		job.Request,
		activeUtxo,
		witnessScript,
	)
	if err != nil {
		return failedTransition(covenantsigner.ReasonProviderFailed, err.Error())
	}

	return &covenantsigner.Transition{
		State:   covenantsigner.JobStateHandoffReady,
		Detail:  "qc_v1 signer handoff ready for custodian coordination",
		Handoff: handoff.toMap(),
	}
}

func decodeSelfV1Template(raw json.RawMessage) (*covenantsigner.SelfV1Template, error) {
	template := &covenantsigner.SelfV1Template{}
	if err := json.Unmarshal(raw, template); err != nil {
		return nil, fmt.Errorf("cannot decode self_v1 template: %v", err)
	}
	if template.Template != covenantsigner.TemplateSelfV1 {
		return nil, fmt.Errorf("request template must be self_v1")
	}
	return template, nil
}

func decodeQcV1Template(raw json.RawMessage) (*covenantsigner.QcV1Template, error) {
	template := &covenantsigner.QcV1Template{}
	if err := json.Unmarshal(raw, template); err != nil {
		return nil, fmt.Errorf("cannot decode qc_v1 template: %v", err)
	}
	if template.Template != covenantsigner.TemplateQcV1 {
		return nil, fmt.Errorf("request template must be qc_v1")
	}
	return template, nil
}

func parseCompressedPublicKey(encoded string) (*ecdsa.PublicKey, error) {
	bytes, err := canonicalCompressedPublicKeyBytes(encoded)
	if err != nil {
		return nil, err
	}

	parsed, err := btcec.ParsePubKey(bytes, btcec.S256())
	if err != nil {
		return nil, err
	}

	return &ecdsa.PublicKey{
		Curve: tecdsa.Curve,
		X:     parsed.X,
		Y:     parsed.Y,
	}, nil
}

func buildSelfV1WitnessScript(
	template *covenantsigner.SelfV1Template,
	maturityHeight uint64,
) (bitcoin.Script, error) {
	if maturityHeight == 0 {
		return nil, fmt.Errorf("maturity height must be greater than zero")
	}
	if maturityHeight > math.MaxUint32 {
		return nil, fmt.Errorf("maturity height exceeds bitcoin locktime range")
	}
	if template.Delta2 > math.MaxUint32 || maturityHeight > math.MaxUint32-template.Delta2 {
		return nil, fmt.Errorf("self_v1 delta2 overflows bitcoin locktime range")
	}

	depositorPublicKey, err := canonicalCompressedPublicKeyBytes(template.DepositorPublicKey)
	if err != nil {
		return nil, fmt.Errorf("invalid self_v1 depositor public key")
	}
	signerPublicKey, err := canonicalCompressedPublicKeyBytes(template.SignerPublicKey)
	if err != nil {
		return nil, fmt.Errorf("invalid self_v1 signer public key")
	}

	maturityScriptNumber, err := encodeScriptNumber(uint32(maturityHeight))
	if err != nil {
		return nil, err
	}
	lastResortScriptNumber, err := encodeScriptNumber(uint32(maturityHeight + template.Delta2))
	if err != nil {
		return nil, err
	}

	return txscript.NewScriptBuilder().
		AddOp(txscript.OP_IF).
		AddOp(txscript.OP_2).
		AddData(depositorPublicKey).
		AddData(signerPublicKey).
		AddOp(txscript.OP_2).
		AddOp(txscript.OP_CHECKMULTISIG).
		AddOp(txscript.OP_ELSE).
		AddOp(txscript.OP_IF).
		AddData(maturityScriptNumber).
		AddOp(txscript.OP_CHECKLOCKTIMEVERIFY).
		AddOp(txscript.OP_DROP).
		AddData(signerPublicKey).
		AddOp(txscript.OP_CHECKSIG).
		AddOp(txscript.OP_ELSE).
		AddData(lastResortScriptNumber).
		AddOp(txscript.OP_CHECKLOCKTIMEVERIFY).
		AddOp(txscript.OP_DROP).
		AddData(depositorPublicKey).
		AddOp(txscript.OP_CHECKSIG).
		AddOp(txscript.OP_ENDIF).
		AddOp(txscript.OP_ENDIF).
		Script()
}

func buildQcV1WitnessScript(
	template *covenantsigner.QcV1Template,
	maturityHeight uint64,
) (bitcoin.Script, error) {
	if maturityHeight == 0 {
		return nil, fmt.Errorf("maturity height must be greater than zero")
	}
	if maturityHeight > math.MaxUint32 {
		return nil, fmt.Errorf("maturity height exceeds bitcoin locktime range")
	}
	if template.Beta > math.MaxUint32 || template.Beta >= maturityHeight {
		return nil, fmt.Errorf("qc_v1 beta must be below maturity height")
	}
	if template.Delta2 > math.MaxUint32 || maturityHeight > math.MaxUint32-template.Delta2 {
		return nil, fmt.Errorf("qc_v1 delta2 overflows bitcoin locktime range")
	}

	depositorPublicKey, err := canonicalCompressedPublicKeyBytes(template.DepositorPublicKey)
	if err != nil {
		return nil, fmt.Errorf("invalid qc_v1 depositor public key")
	}
	custodianPublicKey, err := canonicalCompressedPublicKeyBytes(template.CustodianPublicKey)
	if err != nil {
		return nil, fmt.Errorf("invalid qc_v1 custodian public key")
	}
	signerPublicKey, err := canonicalCompressedPublicKeyBytes(template.SignerPublicKey)
	if err != nil {
		return nil, fmt.Errorf("invalid qc_v1 signer public key")
	}

	maturityScriptNumber, err := encodeScriptNumber(uint32(maturityHeight))
	if err != nil {
		return nil, err
	}
	earlyExitScriptNumber, err := encodeScriptNumber(uint32(maturityHeight - template.Beta))
	if err != nil {
		return nil, err
	}
	lastResortScriptNumber, err := encodeScriptNumber(uint32(maturityHeight + template.Delta2))
	if err != nil {
		return nil, err
	}

	return txscript.NewScriptBuilder().
		AddOp(txscript.OP_IF).
		AddOp(txscript.OP_3).
		AddData(depositorPublicKey).
		AddData(custodianPublicKey).
		AddData(signerPublicKey).
		AddOp(txscript.OP_3).
		AddOp(txscript.OP_CHECKMULTISIG).
		AddOp(txscript.OP_ELSE).
		AddOp(txscript.OP_IF).
		AddData(maturityScriptNumber).
		AddOp(txscript.OP_CHECKLOCKTIMEVERIFY).
		AddOp(txscript.OP_DROP).
		AddOp(txscript.OP_2).
		AddData(signerPublicKey).
		AddData(custodianPublicKey).
		AddOp(txscript.OP_2).
		AddOp(txscript.OP_CHECKMULTISIG).
		AddOp(txscript.OP_ELSE).
		AddOp(txscript.OP_IF).
		AddData(earlyExitScriptNumber).
		AddOp(txscript.OP_CHECKLOCKTIMEVERIFY).
		AddOp(txscript.OP_DROP).
		AddOp(txscript.OP_2).
		AddData(depositorPublicKey).
		AddData(custodianPublicKey).
		AddOp(txscript.OP_2).
		AddOp(txscript.OP_CHECKMULTISIG).
		AddOp(txscript.OP_ELSE).
		AddData(lastResortScriptNumber).
		AddOp(txscript.OP_CHECKLOCKTIMEVERIFY).
		AddOp(txscript.OP_DROP).
		AddData(depositorPublicKey).
		AddOp(txscript.OP_CHECKSIG).
		AddOp(txscript.OP_ENDIF).
		AddOp(txscript.OP_ENDIF).
		AddOp(txscript.OP_ENDIF).
		Script()
}

// resolveActiveUtxo fetches and validates the active covenant UTXO against
// the given witness script and request. templateName is used in error messages
// to identify which template path triggered the validation failure.
func (cse *covenantSignerEngine) resolveActiveUtxo(
	request covenantsigner.RouteSubmitRequest,
	witnessScript bitcoin.Script,
	templateName string,
) (*bitcoin.UnspentTransactionOutput, error) {
	activeTxHash, err := bitcoin.NewHashFromString(
		strings.TrimPrefix(request.ActiveOutpoint.TxID, "0x"),
		bitcoin.ReversedByteOrder,
	)
	if err != nil {
		return nil, fmt.Errorf("active outpoint txid is invalid")
	}

	transaction, err := cse.node.btcChain.GetTransaction(activeTxHash)
	if err != nil {
		return nil, fmt.Errorf("active outpoint transaction not found")
	}
	if err := cse.ensureActiveOutpointFinality(activeTxHash); err != nil {
		return nil, err
	}
	if int(request.ActiveOutpoint.Vout) >= len(transaction.Outputs) {
		return nil, fmt.Errorf("active outpoint output index is out of range")
	}

	expectedWitnessScriptHash := bitcoin.WitnessScriptHash(witnessScript)
	expectedScriptPubKey, err := bitcoin.PayToWitnessScriptHash(expectedWitnessScriptHash)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot build expected %s locking script: %v",
			templateName,
			err,
		)
	}

	actualOutput := transaction.Outputs[request.ActiveOutpoint.Vout]
	if !bytes.Equal(actualOutput.PublicKeyScript, expectedScriptPubKey) {
		return nil, fmt.Errorf(
			"active outpoint script does not match %s template",
			templateName,
		)
	}
	if actualOutput.Value <= 0 {
		return nil, fmt.Errorf("active outpoint value must be greater than zero")
	}
	if uint64(actualOutput.Value) != request.MigrationTransactionPlan.InputValueSats {
		return nil, fmt.Errorf("active outpoint value does not match migration transaction plan")
	}

	if request.ActiveOutpoint.ScriptHash != "" {
		// The optional scriptHash convention follows the tBTC-side request
		// contract: sha256(scriptPubKey) for the active covenant output.
		scriptHash := sha256.Sum256(expectedScriptPubKey)
		expectedScriptHash := "0x" + hex.EncodeToString(scriptHash[:])
		if strings.ToLower(request.ActiveOutpoint.ScriptHash) != expectedScriptHash {
			return nil, fmt.Errorf(
				"active outpoint script hash does not match %s template",
				templateName,
			)
		}
	}

	return &bitcoin.UnspentTransactionOutput{
		Outpoint: &bitcoin.TransactionOutpoint{
			TransactionHash: activeTxHash,
			OutputIndex:     request.ActiveOutpoint.Vout,
		},
		Value: actualOutput.Value,
	}, nil
}

func (cse *covenantSignerEngine) resolveSelfV1ActiveUtxo(
	request covenantsigner.RouteSubmitRequest,
	witnessScript bitcoin.Script,
) (*bitcoin.UnspentTransactionOutput, error) {
	return cse.resolveActiveUtxo(request, witnessScript, "self_v1")
}

func (cse *covenantSignerEngine) resolveQcV1ActiveUtxo(
	request covenantsigner.RouteSubmitRequest,
	witnessScript bitcoin.Script,
) (*bitcoin.UnspentTransactionOutput, error) {
	return cse.resolveActiveUtxo(request, witnessScript, "qc_v1")
}

func (cse *covenantSignerEngine) ensureActiveOutpointFinality(
	activeTxHash bitcoin.Hash,
) error {
	confirmations, err := cse.node.btcChain.GetTransactionConfirmations(activeTxHash)
	if err != nil {
		return fmt.Errorf("cannot determine active outpoint transaction confirmations: %v", err)
	}
	if confirmations < cse.minimumActiveOutpointConfirmations {
		return fmt.Errorf(
			"active outpoint transaction must have at least %d confirmations",
			cse.minimumActiveOutpointConfirmations,
		)
	}

	return nil
}

func validateMigrationOutputValues(request covenantsigner.RouteSubmitRequest) error {
	_, err := toBitcoinOutputValue(
		request.MigrationTransactionPlan.DestinationValueSats,
		"migration destination value",
	)
	if err != nil {
		return err
	}

	_, err = toBitcoinOutputValue(
		request.MigrationTransactionPlan.AnchorValueSats,
		"migration anchor value",
	)
	return err
}

func (cse *covenantSignerEngine) buildAndSignSelfV1Transaction(
	ctx context.Context,
	signingExecutor *signingExecutor,
	request covenantsigner.RouteSubmitRequest,
	activeUtxo *bitcoin.UnspentTransactionOutput,
	witnessScript bitcoin.Script,
) (*bitcoin.Transaction, error) {
	builder, err := cse.buildCovenantTransactionBuilder(
		request,
		activeUtxo,
		witnessScript,
	)
	if err != nil {
		return nil, err
	}
	signature, err := signCovenantTransactionInput(ctx, signingExecutor, builder)
	if err != nil {
		return nil, err
	}

	witness, err := buildSelfV1MigrationWitness(signature, witnessScript)
	if err != nil {
		return nil, err
	}
	if err := builder.SetInputWitness(0, witness); err != nil {
		return nil, fmt.Errorf("cannot set covenant witness: %v", err)
	}

	transaction := builder.Build()
	if len(transaction.Inputs) != 1 {
		return nil, fmt.Errorf("unexpected covenant input count")
	}
	if len(transaction.Inputs[0].Witness) == 0 {
		return nil, fmt.Errorf("unexpected empty covenant witness stack")
	}
	if !bytes.Equal(transaction.Inputs[0].Witness[len(transaction.Inputs[0].Witness)-1], witnessScript) {
		// This can never happen with the current builder path, but keeping the
		// explicit comparison helps catch future witness-shape regressions.
		return nil, fmt.Errorf("unexpected covenant witness stack")
	}

	return transaction, nil
}

func (cse *covenantSignerEngine) buildQcV1SignerHandoff(
	ctx context.Context,
	requestID string,
	signingExecutor *signingExecutor,
	request covenantsigner.RouteSubmitRequest,
	activeUtxo *bitcoin.UnspentTransactionOutput,
	witnessScript bitcoin.Script,
) (*qcV1SignerHandoff, error) {
	builder, err := cse.buildCovenantTransactionBuilder(
		request,
		activeUtxo,
		witnessScript,
	)
	if err != nil {
		return nil, err
	}
	signature, err := signCovenantTransactionInput(ctx, signingExecutor, builder)
	if err != nil {
		return nil, err
	}
	signatureBytes, err := buildWitnessSignatureBytes(signature)
	if err != nil {
		return nil, err
	}

	unsignedTransaction := builder.Build()
	unsignedTransactionHex := "0x" + hex.EncodeToString(unsignedTransaction.Serialize(bitcoin.Standard))
	witnessScriptHex := "0x" + hex.EncodeToString(witnessScript)
	signatureHex := "0x" + hex.EncodeToString(signatureBytes)
	selectorWitnessItems := []string{"0x01", "0x"}

	payloadHash, err := computeQcV1SignerHandoffPayloadHash(map[string]any{
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
		return nil, err
	}

	return &qcV1SignerHandoff{
		Kind:                      qcV1SignerHandoffKind,
		SignerRequestID:           requestID,
		BundleID:                  payloadHash,
		DestinationCommitmentHash: request.DestinationCommitmentHash,
		PayloadHash:               payloadHash,
		UnsignedTransactionHex:    unsignedTransactionHex,
		WitnessScript:             witnessScriptHex,
		SignerSignature:           signatureHex,
		SelectorWitnessItems:      selectorWitnessItems,
		RequiresDummy:             true,
		SighashType:               uint32(txscript.SigHashAll),
	}, nil
}

func (cse *covenantSignerEngine) buildCovenantTransactionBuilder(
	request covenantsigner.RouteSubmitRequest,
	activeUtxo *bitcoin.UnspentTransactionOutput,
	witnessScript bitcoin.Script,
) (*bitcoin.TransactionBuilder, error) {
	destinationScript, err := decodePrefixedHex(request.MigrationDestination.DepositScript)
	if err != nil {
		return nil, fmt.Errorf("migration destination deposit script is invalid")
	}
	destinationValue, err := toBitcoinOutputValue(
		request.MigrationTransactionPlan.DestinationValueSats,
		"migration destination value",
	)
	if err != nil {
		return nil, err
	}
	anchorValue, err := toBitcoinOutputValue(
		request.MigrationTransactionPlan.AnchorValueSats,
		"migration anchor value",
	)
	if err != nil {
		return nil, err
	}

	builder := bitcoin.NewTransactionBuilder(cse.node.btcChain)
	if err := builder.AddScriptHashInput(activeUtxo, witnessScript); err != nil {
		return nil, fmt.Errorf("cannot add covenant input: %v", err)
	}
	if err := builder.SetInputSequence(0, request.MigrationTransactionPlan.InputSequence); err != nil {
		return nil, fmt.Errorf("cannot set covenant input sequence: %v", err)
	}
	builder.SetLocktime(request.MigrationTransactionPlan.LockTime)
	builder.AddOutput(&bitcoin.TransactionOutput{
		Value:           destinationValue,
		PublicKeyScript: destinationScript,
	})

	anchorScript, err := canonicalAnchorScriptPubKey()
	if err != nil {
		return nil, err
	}
	builder.AddOutput(&bitcoin.TransactionOutput{
		Value:           anchorValue,
		PublicKeyScript: anchorScript,
	})

	return builder, nil
}

func signCovenantTransactionInput(
	ctx context.Context,
	signingExecutor *signingExecutor,
	builder *bitcoin.TransactionBuilder,
) (*tecdsa.Signature, error) {
	sigHashes, err := builder.ComputeSignatureHashes()
	if err != nil {
		return nil, fmt.Errorf("cannot compute covenant sighash: %v", err)
	}
	if len(sigHashes) != 1 {
		return nil, fmt.Errorf("unexpected covenant sighash count")
	}

	startBlock, err := signingExecutor.getCurrentBlockFn()
	if err != nil {
		return nil, fmt.Errorf("cannot determine signing start block: %v", err)
	}

	signatures, err := signingExecutor.signBatch(ctx, sigHashes, startBlock)
	if err != nil {
		return nil, fmt.Errorf("cannot sign covenant transaction: %v", err)
	}
	if len(signatures) != 1 {
		return nil, fmt.Errorf("unexpected covenant signature count")
	}
	return signatures[0], nil
}

func buildSelfV1MigrationWitness(
	signature *tecdsa.Signature,
	witnessScript bitcoin.Script,
) ([][]byte, error) {
	signatureBytes, err := buildWitnessSignatureBytes(signature)
	if err != nil {
		return nil, err
	}

	return [][]byte{
		signatureBytes,
		{0x01},
		{},
		witnessScript,
	}, nil
}

func buildWitnessSignatureBytes(signature *tecdsa.Signature) ([]byte, error) {
	if signature == nil || signature.R == nil || signature.S == nil {
		return nil, fmt.Errorf("missing covenant signature")
	}

	return append(
		(&btcec.Signature{R: signature.R, S: signature.S}).Serialize(),
		byte(txscript.SigHashAll),
	), nil
}

func computeQcV1SignerHandoffPayloadHash(payload map[string]any) (string, error) {
	// The handoff bundle ID is content-addressed using canonical JSON
	// (alphabetical key ordering, no HTML escaping, no trailing newline).
	// Go's encoding/json.Marshal already sorts map keys alphabetically
	// (since Go 1.12), so using canonicaljson.Marshal produces identical
	// output for non-HTML content while also disabling HTML escaping for
	// safety. Non-Go custodian consumers that recompute this hash must
	// use the same canonical serialization rules.
	rawPayload, err := canonicaljson.Marshal(payload)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(rawPayload)
	return "0x" + hex.EncodeToString(sum[:]), nil
}

func (handoff *qcV1SignerHandoff) toMap() map[string]any {
	return map[string]any{
		"kind":                      handoff.Kind,
		"signerRequestId":           handoff.SignerRequestID,
		"bundleId":                  handoff.BundleID,
		"destinationCommitmentHash": handoff.DestinationCommitmentHash,
		"payloadHash":               handoff.PayloadHash,
		"unsignedTransactionHex":    handoff.UnsignedTransactionHex,
		"witnessScript":             handoff.WitnessScript,
		"signerSignature":           handoff.SignerSignature,
		"selectorWitnessItems":      handoff.SelectorWitnessItems,
		"requiresDummy":             handoff.RequiresDummy,
		"sighashType":               handoff.SighashType,
	}
}

func canonicalAnchorScriptPubKey() (bitcoin.Script, error) {
	witnessScriptHash := bitcoin.WitnessScriptHash(bitcoin.Script{txscript.OP_TRUE})
	return bitcoin.PayToWitnessScriptHash(witnessScriptHash)
}

func decodePrefixedHex(value string) ([]byte, error) {
	return hex.DecodeString(strings.TrimPrefix(value, "0x"))
}

func canonicalCompressedPublicKeyBytes(encoded string) ([]byte, error) {
	bytes, err := decodePrefixedHex(encoded)
	if err != nil {
		return nil, err
	}

	parsed, err := btcec.ParsePubKey(bytes, btcec.S256())
	if err != nil {
		return nil, err
	}

	return parsed.SerializeCompressed(), nil
}

func toBitcoinOutputValue(value uint64, field string) (int64, error) {
	if value > math.MaxInt64 {
		return 0, fmt.Errorf("%s exceeds bitcoin output value range", field)
	}

	return int64(value), nil
}

func encodeScriptNumber(value uint32) ([]byte, error) {
	if value == 0 {
		return []byte{}, nil
	}

	result := make([]byte, 0, 5)
	absolute := value
	for absolute > 0 {
		result = append(result, byte(absolute&0xff))
		absolute >>= 8
	}

	if result[len(result)-1]&0x80 != 0 {
		result = append(result, 0x00)
	}

	return result, nil
}

func failedTransition(reason covenantsigner.FailureReason, detail string) *covenantsigner.Transition {
	return &covenantsigner.Transition{
		State:  covenantsigner.JobStateFailed,
		Reason: reason,
		Detail: detail,
	}
}
