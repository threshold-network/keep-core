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
	// bridgeFraudDefenseConfirmed records the operator's explicit confirmation
	// that the tBTC Bridge recognizes covenant active UTXO spends as honest
	// spends, i.e. the covenant fraud-defense path is deployed. Until it is set,
	// the engine refuses to produce covenant signatures because a covenant
	// SIGHASH_ALL signature over a covenant active UTXO is otherwise a valid,
	// undefeatable tBTC fraud proof against the signing wallet.
	bridgeFraudDefenseConfirmed bool
	// eip712ChainID and eip712Salt define the EIP-712 domain used to recompute
	// the v2 artifact approval digest during signer approval verification. They
	// must match the covenant signer service's configured domain.
	eip712ChainID uint64
	eip712Salt    [32]byte
}

// Compile-time assertions that covenantSignerEngine satisfies the full
// covenant signer contract, including CurrentBlockHeightProvider. Signer
// approval certificate expiry enforcement depends on every verifier-capable
// engine also providing a current block height; losing this interface
// silently would make certificates never expire.
var (
	_ covenantsigner.Engine                          = (*covenantSignerEngine)(nil)
	_ covenantsigner.SignerApprovalVerifier          = (*covenantSignerEngine)(nil)
	_ covenantsigner.CurrentBlockHeightProvider      = (*covenantSignerEngine)(nil)
	_ covenantsigner.SignerApprovalCertificateIssuer = (*covenantSignerEngine)(nil)
)

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
//
// bridgeFraudDefenseConfirmed must be set only when the operator has confirmed
// that the tBTC Bridge covenant fraud-defense path is deployed. When false (the
// default), the engine fails closed and refuses to produce covenant signatures.
func newCovenantSignerEngine(
	node *node,
	minConfirmations uint,
	bridgeFraudDefenseConfirmed bool,
	eip712ChainID uint64,
	eip712Salt [32]byte,
) covenantsigner.Engine {
	if minConfirmations == 0 {
		minConfirmations = defaultMinActiveOutpointConfirmations
	}

	return &covenantSignerEngine{
		node:                               node,
		minimumActiveOutpointConfirmations: minConfirmations,
		bridgeFraudDefenseConfirmed:        bridgeFraudDefenseConfirmed,
		eip712ChainID:                      eip712ChainID,
		eip712Salt:                         eip712Salt,
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
		cse.eip712ChainID,
		cse.eip712Salt,
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

	// Fail closed for wallets that are not in a state eligible for covenant
	// signing. The signer set hash embedded in a certificate binds only the
	// wallet identity, members hash, and threshold, none of which change when a
	// wallet is closed or terminated, so a certificate issued while the wallet
	// was live would otherwise keep verifying after closure. Rejecting
	// non-eligible states here ensures a wallet the closure path intended to
	// deauthorize cannot be made to sign a covenant transaction.
	if !isCovenantSigningEligibleState(walletChainData.State) {
		return covenantsigner.NewInputError(
			fmt.Sprintf(
				"request.signerApproval.walletPublicKey resolves to a wallet in "+
					"state [%v] that is not eligible for covenant signing",
				walletChainData.State,
			),
		)
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

// isCovenantSigningEligibleState reports whether a wallet in the given state is
// eligible to receive covenant signatures. The rule is uniform across every
// covenant action - migration, redeem, and renew: all three are only expected
// for live wallets, so covenant signing fails closed for every other state
// (including closed and terminated wallets that the closure path intends to
// deauthorize).
//
// Redeem and renew are deliberately held to the same live-only rule rather than
// being widened on the intuition that a cooperative payout should still be
// possible while a wallet winds down. Nothing in a signer approval certificate
// binds the wallet's state - the signer set hash covers only wallet identity,
// members hash, and threshold - so a certificate issued while the wallet was
// live stays verifiable after closure, and widening any action here makes that
// certificate replayable against a wallet the protocol already deauthorized.
// Allowing another action/state pair therefore requires an explicit protocol
// decision about why certificate reuse past that point is safe; add it here
// with that justification and matching tests.
func isCovenantSigningEligibleState(state WalletState) bool {
	return state == StateLive
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
	// Fail closed unless the operator has confirmed the tBTC Bridge covenant
	// fraud-defense path is deployed. Producing a covenant SIGHASH_ALL
	// signature over a covenant active UTXO exposes the signing wallet to a
	// tBTC fraud challenge it cannot defeat, because the Bridge does not
	// recognize a covenant active UTXO spend as an honest spend in
	// Fraud.defeatFraudChallenge; only a swept deposit, a spent main UTXO, or a
	// processed moved-funds sweep can defeat the challenge. The complete
	// remediation is the bridge-side covenant fraud-defense path. Until it is
	// deployed and confirmed here, refuse to sign so a valid migration
	// signature cannot become a slashable wallet signature.
	if !cse.bridgeFraudDefenseConfirmed {
		return failedTransition(
			covenantsigner.ReasonPolicyRejected,
			"covenant signing is disabled until the tBTC Bridge covenant "+
				"fraud-defense path is confirmed deployed; a covenant signature "+
				"would otherwise expose the wallet to an undefeatable fraud "+
				"challenge",
		), nil
	}

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

// CurrentBlockHeight returns the current height of the host chain (e.g.
// Ethereum), obtained through the same node chain connection the signing
// executors use. It is deliberately the host chain, not cse.node.btcChain:
// signer approval certificate EndBlock values are defined in host-chain block
// units so expiry can be enforced independently of Bitcoin's slower,
// reorg-prone confirmation times.
func (cse *covenantSignerEngine) CurrentBlockHeight(context.Context) (uint64, error) {
	blockCounter, err := cse.node.chain.BlockCounter()
	if err != nil {
		return 0, fmt.Errorf("cannot get host chain block counter: %w", err)
	}

	return blockCounter.CurrentBlock()
}

// IssueSignerApprovalCertificate threshold-signs a v2 signer approval
// certificate for a wallet this node controls. approvalDigest must match the
// artifact-approval payload digest of the Submit request that will carry the
// certificate. endBlock is the host-chain expiry height.
func (cse *covenantSignerEngine) IssueSignerApprovalCertificate(
	ctx context.Context,
	walletPublicKeyHash [20]byte,
	approvalDigest []byte,
	endBlock uint64,
) (*covenantsigner.SignerApprovalCertificate, error) {
	if len(approvalDigest) != sha256.Size {
		return nil, covenantsigner.NewInputError(
			fmt.Sprintf(
				"approvalDigest must be exactly %d bytes",
				sha256.Size,
			),
		)
	}

	wallet, ok := cse.node.walletRegistry.getWalletByPublicKeyHash(
		walletPublicKeyHash,
	)
	if !ok {
		return nil, covenantsigner.NewInputError(
			"walletPublicKeyHash does not resolve to a wallet controlled by this node",
		)
	}

	walletChainData, err := cse.node.chain.GetWallet(walletPublicKeyHash)
	if err != nil {
		if errors.Is(err, ErrWalletNotFound) {
			return nil, covenantsigner.NewInputError(
				"walletPublicKeyHash must resolve to a registered on-chain wallet",
			)
		}
		return nil, fmt.Errorf(
			"cannot resolve on-chain wallet for signer approval certificate: %w",
			err,
		)
	}
	if err := ensureWalletRegistryDataAvailable(
		walletChainData,
		"issue signer approval certificate",
	); err != nil {
		return nil, err
	}
	if !isCovenantSigningEligibleState(walletChainData.State) {
		return nil, covenantsigner.NewInputError(
			fmt.Sprintf(
				"walletPublicKeyHash resolves to a wallet in state [%v] that is "+
					"not eligible for covenant signing",
				walletChainData.State,
			),
		)
	}

	signingExecutor, ok, err := cse.node.getSigningExecutor(wallet.publicKey)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve signing executor: %w", err)
	}
	if !ok {
		return nil, covenantsigner.NewInputError(
			"wallet is not controlled by this node",
		)
	}

	startBlock, err := signingExecutor.getCurrentBlockFn()
	if err != nil {
		return nil, fmt.Errorf(
			"cannot get current host-chain block for signer approval certificate: %w",
			err,
		)
	}
	if endBlock <= startBlock {
		return nil, covenantsigner.NewInputError(
			fmt.Sprintf(
				"endBlock [%d] must be greater than the current host-chain block [%d]",
				endBlock,
				startBlock,
			),
		)
	}

	certificate, err := signingExecutor.issueSignerApprovalCertificate(
		ctx,
		approvalDigest,
		startBlock,
		endBlock,
	)
	if err != nil {
		if errors.Is(err, errSigningExecutorBusy) {
			return nil, covenantsigner.ErrSignerApprovalCertificateIssuerBusy
		}
		return nil, err
	}

	return certificate, nil
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

	expectedScriptPubKey, err := payToWitnessScriptHash(witnessScript)
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

// covenantDestinationOutputScript returns the destination output scriptPubKey
// for the request's covenant action. The redeem payout script and the renew
// next-covenant script are already output scripts and are paid directly, but a
// migration's deposit script is not: it is wrapped into its P2WSH scriptPubKey
// first. Validation has already recompute-and-compared each of these against the
// action's destination commitment, so the built output is exactly what the
// depositor's artifact approval authorized.
func covenantDestinationOutputScript(
	request covenantsigner.RouteSubmitRequest,
) (bitcoin.Script, error) {
	switch request.ResolvedAction() {
	case covenantsigner.CovenantActionMigration:
		if request.MigrationDestination == nil {
			return nil, fmt.Errorf("migration destination is required")
		}
		script, err := decodePrefixedHex(request.MigrationDestination.DepositScript)
		if err != nil {
			return nil, fmt.Errorf("migration destination deposit script is invalid")
		}
		if len(script) == 0 {
			return nil, fmt.Errorf("migration destination deposit script must not be empty")
		}
		// MigrationDestination.DepositScript is the plain tBTC deposit script, not
		// a ready-made output script. The Bitcoin funding output must pay to its
		// P2WSH script hash (OP_0 <sha256(depositScript)>), which is how the tBTC
		// Bridge rebuilds and verifies the funding output in
		// revealDepositWithExtraData. Using the plain deposit script directly as
		// the output script would make the migration deposit unrevealable to the
		// Bridge.
		scriptPubKey, err := payToWitnessScriptHash(script)
		if err != nil {
			return nil, fmt.Errorf("cannot build migration destination locking script: %v", err)
		}
		return scriptPubKey, nil
	case covenantsigner.CovenantActionRedeem:
		if request.RedeemDestination == nil {
			return nil, fmt.Errorf("redeem destination is required")
		}
		script, err := decodePrefixedHex(request.RedeemDestination.OutputScript)
		if err != nil {
			return nil, fmt.Errorf("redeem destination output script is invalid")
		}
		return script, nil
	case covenantsigner.CovenantActionRenew:
		if request.RenewDestination == nil {
			return nil, fmt.Errorf("renew destination is required")
		}
		script, err := decodePrefixedHex(request.RenewDestination.NextCovenantScript)
		if err != nil {
			return nil, fmt.Errorf("renew destination next covenant script is invalid")
		}
		return script, nil
	default:
		return nil, fmt.Errorf("unsupported covenant action %q", request.ResolvedAction())
	}
}

func (cse *covenantSignerEngine) buildCovenantTransactionBuilder(
	request covenantsigner.RouteSubmitRequest,
	activeUtxo *bitcoin.UnspentTransactionOutput,
	witnessScript bitcoin.Script,
) (*bitcoin.TransactionBuilder, error) {
	destinationScriptPubKey, err := covenantDestinationOutputScript(request)
	if err != nil {
		return nil, err
	}
	destinationValue, err := toBitcoinOutputValue(
		request.MigrationTransactionPlan.DestinationValueSats,
		"covenant destination value",
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
		PublicKeyScript: destinationScriptPubKey,
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

// signCovenantTransactionInput produces the wallet's tECDSA signature over the
// single covenant input of the migration transaction.
//
// This signature is a normal Bitcoin SIGHASH_ALL signature over a covenant
// active UTXO. A covenant active UTXO is neither a swept deposit, a spent main
// UTXO, nor a processed moved-funds sweep, so the tBTC Bridge cannot defeat a
// fraud challenge that replays this signature. Callers must therefore only
// reach this path once the bridge-side covenant fraud-defense has been
// confirmed deployed; OnSubmit enforces that fail-closed gate.
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
	return payToWitnessScriptHash(bitcoin.Script{txscript.OP_TRUE})
}

// payToWitnessScriptHash derives the P2WSH locking script
// (OP_0 <sha256(script)>) that pays to the given witness script. This is how
// the tBTC Bridge matches an output against a script it independently
// recomputes; callers wrap the returned error with their own context.
func payToWitnessScriptHash(script bitcoin.Script) (bitcoin.Script, error) {
	return bitcoin.PayToWitnessScriptHash(bitcoin.WitnessScriptHash(script))
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
