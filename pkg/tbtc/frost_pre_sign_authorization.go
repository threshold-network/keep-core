package tbtc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"
	"sort"
	"sync"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	ethereumCrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

const (
	frostPreSignAuthorizationMessageTypePrefix = "tbtc/frost_pre_sign_authorization/"
	frostPreSignAuthorizationThreshold         = 51
	frostPreSignAuthorizationMaximumSeats      = 100
	frostPreSignAuthorizationMaximumInputs     = 21

	frostPreSignReservationProtocolDomain = "tbtc/p2tr-pre-signing-reservation/threshold-v1"
	frostPreSignSigningPolicyDomain       = "tbtc/p2tr-pre-signing-policy/default-no-annex-51-seats-v1"
	frostPreSignChallengeIdentityDomain   = "tbtc-p2tr-signature-fraud-authorization-v3"
	frostCompleteEvidenceProtocolDomain   = "tbtc/p2tr-signature-fraud/evidence/complete-v2"
)

// FrostPreSignAction is the compact action identity committed by the on-chain
// reservation registry. It intentionally excludes heartbeat/arbitrary-message
// signing.
type FrostPreSignAction uint8

const (
	FrostPreSignActionDepositSweep FrostPreSignAction = iota + 1
	FrostPreSignActionRedemption
	FrostPreSignActionMovingFunds
	FrostPreSignActionMovedFundsSweep
)

func frostPreSignAction(action WalletActionType) (FrostPreSignAction, error) {
	switch action {
	case ActionDepositSweep:
		return FrostPreSignActionDepositSweep, nil
	case ActionRedemption:
		return FrostPreSignActionRedemption, nil
	case ActionMovingFunds:
		return FrostPreSignActionMovingFunds, nil
	case ActionMovedFundsSweep:
		return FrostPreSignActionMovedFundsSweep, nil
	default:
		return 0, fmt.Errorf(
			"wallet action [%s] is not eligible for FROST pre-sign authorization",
			action,
		)
	}
}

// FrostPreSignTransaction is the exact, stripped Bitcoin transaction and
// BIP-341 signing batch proposed by a wallet. TransactionHash uses keep-core's
// raw SHA256d digest byte order; it is never display-byte-reversed. Every
// SignatureHashes item is exactly 32 bytes, preserving leading zeroes lost by
// big.Int.Bytes().
type FrostPreSignTransaction struct {
	Action              FrostPreSignAction
	WalletPublicKeyHash [20]byte
	Version             [4]byte
	InputVector         []byte
	OutputVector        []byte
	Locktime            [4]byte
	RawTransaction      []byte
	TransactionHash     bitcoin.Hash
	InputValues         []uint64
	SigningKeys         [][32]byte
	SignatureHashes     [][32]byte
	SighashTypes        []uint8
	SpendTypes          []uint8
	ActionContext       *FrostPreSignActionContext
}

// FrostPreSignActionContext carries only data that has already passed the
// ordinary action validation path. The Ethereum COMPLETE_V2 adapter encodes
// exactly one branch into P2TRPreSigning actionData; nil, multiple, or
// action-mismatched branches fail closed before preview.
type FrostPreSignActionContext struct {
	DepositSweep    *FrostPreSignDepositSweepActionContext
	Redemption      *FrostPreSignRedemptionActionContext
	MovingFunds     *FrostPreSignMovingFundsActionContext
	MovedFundsSweep *FrostPreSignMovedFundsSweepActionContext
}

type FrostPreSignDepositSweepActionContext struct {
	Proposal *DepositSweepProposal
	Deposits []*Deposit
	MainUtxo *bitcoin.UnspentTransactionOutput
}

type FrostPreSignRedemptionActionContext struct {
	Proposal *RedemptionProposal
	MainUtxo *bitcoin.UnspentTransactionOutput
}

type FrostPreSignMovingFundsActionContext struct {
	Proposal *MovingFundsProposal
	MainUtxo *bitcoin.UnspentTransactionOutput
}

type FrostPreSignMovedFundsSweepActionContext struct {
	Proposal *MovedFundsSweepProposal
	MainUtxo *bitcoin.UnspentTransactionOutput
}

func newFrostPreSignTransaction(
	action WalletActionType,
	walletPublicKeyHash [20]byte,
	unsignedTx *bitcoin.TransactionBuilder,
	signatureHashes []*big.Int,
) (*FrostPreSignTransaction, error) {
	preSignAction, err := frostPreSignAction(action)
	if err != nil {
		return nil, err
	}
	if unsignedTx == nil {
		return nil, fmt.Errorf("unsigned transaction builder is nil")
	}
	if !unsignedTx.HasOnlyTaprootKeyPathInputs() {
		return nil, fmt.Errorf(
			"FROST pre-sign authorization requires only P2TR key-path inputs",
		)
	}

	transaction := unsignedTx.UnsignedTransaction()
	if transaction == nil || len(transaction.Inputs) == 0 {
		return nil, fmt.Errorf("unsigned transaction has no inputs")
	}
	if len(transaction.Inputs) > frostPreSignAuthorizationMaximumInputs {
		return nil, fmt.Errorf(
			"FROST pre-sign authorization input count [%d] exceeds maximum [%d]",
			len(transaction.Inputs),
			frostPreSignAuthorizationMaximumInputs,
		)
	}
	if len(transaction.Outputs) == 0 {
		return nil, fmt.Errorf("unsigned transaction has no outputs")
	}
	if len(signatureHashes) != len(transaction.Inputs) {
		return nil, fmt.Errorf(
			"signature hash count [%d] does not match input count [%d]",
			len(signatureHashes),
			len(transaction.Inputs),
		)
	}
	for i, input := range transaction.Inputs {
		if input == nil || input.Outpoint == nil {
			return nil, fmt.Errorf("unsigned transaction input [%d] has no outpoint", i)
		}
		if len(input.SignatureScript) != 0 || len(input.Witness) != 0 {
			return nil, fmt.Errorf(
				"unsigned P2TR input [%d] contains pre-signing witness data",
				i,
			)
		}
	}

	inputs, _, err := unsignedTx.UnsignedTransactionIO()
	if err != nil {
		return nil, fmt.Errorf("cannot extract unsigned transaction metadata: [%w]", err)
	}
	if len(inputs) != len(transaction.Inputs) {
		return nil, fmt.Errorf("unsigned transaction metadata count mismatch")
	}

	inputValues := make([]uint64, len(inputs))
	signingKeys := make([][32]byte, len(inputs))
	for i, input := range inputs {
		scriptBytes, err := hex.DecodeString(input.ScriptPubKeyHex)
		if err != nil {
			return nil, fmt.Errorf("cannot decode input [%d] script: [%w]", i, err)
		}
		signingKey, err := bitcoin.ExtractTaprootKey(bitcoin.Script(scriptBytes))
		if err != nil {
			return nil, fmt.Errorf("cannot extract input [%d] P2TR signing key: [%w]", i, err)
		}
		if signingKey == [32]byte{} {
			return nil, fmt.Errorf("input [%d] P2TR signing key is zero", i)
		}

		inputValues[i] = input.ValueSats
		signingKeys[i] = signingKey
	}

	canonicalSignatureHashes, err := unsignedTx.ComputeSignatureHashes()
	if err != nil {
		return nil, fmt.Errorf(
			"cannot independently compute FROST signature hashes: [%w]",
			err,
		)
	}
	if len(canonicalSignatureHashes) != len(signatureHashes) {
		return nil, fmt.Errorf("canonical signature hash count mismatch")
	}
	fixedSignatureHashes := make([][32]byte, len(signatureHashes))
	for i, signatureHash := range signatureHashes {
		supplied, err := fixedFrostPreSignSignatureHash(signatureHash)
		if err != nil {
			return nil, fmt.Errorf("signature hash [%d] is invalid: [%w]", i, err)
		}
		canonical, err := fixedFrostPreSignSignatureHash(
			canonicalSignatureHashes[i],
		)
		if err != nil {
			return nil, fmt.Errorf(
				"canonical signature hash [%d] is invalid: [%w]",
				i,
				err,
			)
		}
		if supplied != canonical {
			return nil, fmt.Errorf(
				"signature hash [%d] differs from canonical BIP-341 digest",
				i,
			)
		}
		fixedSignatureHashes[i] = canonical
	}

	rawTransaction := transaction.Serialize(bitcoin.Standard)
	if len(rawTransaction) == 0 {
		return nil, fmt.Errorf("cannot serialize stripped unsigned transaction")
	}
	version := transaction.SerializeVersion()
	inputVector := transaction.SerializeInputs()
	outputVector := transaction.SerializeOutputs()
	locktime := transaction.SerializeLocktime()
	expectedRaw := make([]byte, 0, len(rawTransaction))
	expectedRaw = append(expectedRaw, version[:]...)
	expectedRaw = append(expectedRaw, inputVector...)
	expectedRaw = append(expectedRaw, outputVector...)
	expectedRaw = append(expectedRaw, locktime[:]...)
	if !bytes.Equal(rawTransaction, expectedRaw) {
		return nil, fmt.Errorf("stripped transaction serialization is inconsistent")
	}

	result := &FrostPreSignTransaction{
		Action:              preSignAction,
		WalletPublicKeyHash: walletPublicKeyHash,
		Version:             version,
		InputVector:         append([]byte{}, inputVector...),
		OutputVector:        append([]byte{}, outputVector...),
		Locktime:            locktime,
		RawTransaction:      append([]byte{}, rawTransaction...),
		TransactionHash:     bitcoin.ComputeHash(rawTransaction),
		InputValues:         inputValues,
		SigningKeys:         signingKeys,
		SignatureHashes:     fixedSignatureHashes,
		// TransactionBuilder's P2TR path is deliberately frozen to BIP-341
		// SIGHASH_DEFAULT (0) and key-path/no-annex spend_type (0).
		SighashTypes: make([]uint8, len(inputs)),
		SpendTypes:   make([]uint8, len(inputs)),
	}
	if err := result.validate(); err != nil {
		return nil, err
	}

	return result, nil
}

func fixedFrostPreSignSignatureHash(
	signatureHash *big.Int,
) ([32]byte, error) {
	result := [32]byte{}
	if signatureHash == nil {
		return result, fmt.Errorf("value is nil")
	}
	if signatureHash.Sign() < 0 || signatureHash.BitLen() > 256 {
		return result, fmt.Errorf("value does not fit 32 bytes")
	}
	signatureHash.FillBytes(result[:])
	return result, nil
}

func (fpst *FrostPreSignTransaction) validate() error {
	if fpst == nil {
		return fmt.Errorf("FROST pre-sign transaction is nil")
	}
	if fpst.Action < FrostPreSignActionDepositSweep ||
		fpst.Action > FrostPreSignActionMovedFundsSweep {
		return fmt.Errorf("unknown FROST pre-sign action [%d]", fpst.Action)
	}
	inputsCount := len(fpst.InputValues)
	if inputsCount == 0 || inputsCount > frostPreSignAuthorizationMaximumInputs {
		return fmt.Errorf("invalid FROST pre-sign input count [%d]", inputsCount)
	}
	if len(fpst.SigningKeys) != inputsCount ||
		len(fpst.SignatureHashes) != inputsCount ||
		len(fpst.SighashTypes) != inputsCount ||
		len(fpst.SpendTypes) != inputsCount {
		return fmt.Errorf("FROST pre-sign batch vectors are not aligned")
	}
	if len(fpst.InputVector) == 0 || len(fpst.OutputVector) == 0 {
		return fmt.Errorf("FROST pre-sign transaction vectors are empty")
	}
	raw := make([]byte, 0, 8+len(fpst.InputVector)+len(fpst.OutputVector))
	raw = append(raw, fpst.Version[:]...)
	raw = append(raw, fpst.InputVector...)
	raw = append(raw, fpst.OutputVector...)
	raw = append(raw, fpst.Locktime[:]...)
	if !bytes.Equal(raw, fpst.RawTransaction) {
		return fmt.Errorf("FROST pre-sign raw transaction bytes mismatch")
	}
	if bitcoin.ComputeHash(raw) != fpst.TransactionHash {
		return fmt.Errorf("FROST pre-sign transaction SHA256d mismatch")
	}
	for i, signingKey := range fpst.SigningKeys {
		if signingKey == [32]byte{} {
			return fmt.Errorf("FROST pre-sign signing key [%d] is zero", i)
		}
		if fpst.SighashTypes[i] != 0 {
			return fmt.Errorf(
				"FROST pre-sign input [%d] is not SIGHASH_DEFAULT",
				i,
			)
		}
		if fpst.SpendTypes[i] != 0 {
			return fmt.Errorf(
				"FROST pre-sign input [%d] is not key-path/no-annex",
				i,
			)
		}
	}
	canonicalSignatureHashes, err := fpst.computeDefaultSignatureHashes()
	if err != nil {
		return fmt.Errorf(
			"cannot independently reconstruct FROST pre-sign signature hashes: [%w]",
			err,
		)
	}
	for i := range canonicalSignatureHashes {
		if canonicalSignatureHashes[i] != fpst.SignatureHashes[i] {
			return fmt.Errorf(
				"FROST pre-sign signature hash [%d] differs from the stripped transaction",
				i,
			)
		}
	}

	return nil
}

// computeDefaultSignatureHashes reconstructs the exact BIP-341
// SIGHASH_DEFAULT/key-path/no-annex digest batch from the immutable stripped
// transaction and the UTXO values/output keys. This is intentionally separate
// from the builder computation used at construction: a backend cannot mutate a
// cached digest, value, or signing key and still pass authorization validation.
func (fpst *FrostPreSignTransaction) computeDefaultSignatureHashes() (
	[][32]byte,
	error,
) {
	if fpst == nil {
		return nil, fmt.Errorf("FROST pre-sign transaction is nil")
	}

	msgTx := wire.NewMsgTx(wire.TxVersion)
	reader := bytes.NewReader(fpst.RawTransaction)
	if err := msgTx.Deserialize(reader); err != nil {
		return nil, fmt.Errorf("cannot decode stripped transaction: [%w]", err)
	}
	if reader.Len() != 0 {
		return nil, fmt.Errorf(
			"stripped transaction has [%d] trailing bytes",
			reader.Len(),
		)
	}
	var canonicalRaw bytes.Buffer
	if err := msgTx.SerializeNoWitness(&canonicalRaw); err != nil {
		return nil, fmt.Errorf("cannot re-encode stripped transaction: [%w]", err)
	}
	if !bytes.Equal(canonicalRaw.Bytes(), fpst.RawTransaction) {
		return nil, fmt.Errorf("stripped transaction is not canonical no-witness encoding")
	}
	if len(msgTx.TxIn) != len(fpst.InputValues) {
		return nil, fmt.Errorf(
			"decoded input count [%d] differs from metadata [%d]",
			len(msgTx.TxIn),
			len(fpst.InputValues),
		)
	}
	if len(msgTx.TxOut) == 0 {
		return nil, fmt.Errorf("decoded transaction has no outputs")
	}

	var prevouts bytes.Buffer
	var amounts bytes.Buffer
	var scriptPubKeys bytes.Buffer
	var sequences bytes.Buffer
	var outputs bytes.Buffer
	for i, input := range msgTx.TxIn {
		if _, err := prevouts.Write(input.PreviousOutPoint.Hash[:]); err != nil {
			return nil, err
		}
		if err := binary.Write(
			&prevouts,
			binary.LittleEndian,
			input.PreviousOutPoint.Index,
		); err != nil {
			return nil, err
		}
		if err := binary.Write(
			&amounts,
			binary.LittleEndian,
			fpst.InputValues[i],
		); err != nil {
			return nil, err
		}
		p2trScript := make([]byte, 34)
		p2trScript[0] = 0x51 // OP_1.
		p2trScript[1] = 0x20 // 32-byte witness program.
		copy(p2trScript[2:], fpst.SigningKeys[i][:])
		if err := wire.WriteVarBytes(&scriptPubKeys, 0, p2trScript); err != nil {
			return nil, fmt.Errorf(
				"cannot encode input [%d] P2TR script: [%w]",
				i,
				err,
			)
		}
		if err := binary.Write(
			&sequences,
			binary.LittleEndian,
			input.Sequence,
		); err != nil {
			return nil, err
		}
	}
	for i, output := range msgTx.TxOut {
		if err := wire.WriteTxOut(&outputs, 0, 0, output); err != nil {
			return nil, fmt.Errorf(
				"cannot encode transaction output [%d]: [%w]",
				i,
				err,
			)
		}
	}

	hashPrevouts := sha256.Sum256(prevouts.Bytes())
	hashAmounts := sha256.Sum256(amounts.Bytes())
	hashScriptPubKeys := sha256.Sum256(scriptPubKeys.Bytes())
	hashSequences := sha256.Sum256(sequences.Bytes())
	hashOutputs := sha256.Sum256(outputs.Bytes())

	result := make([][32]byte, len(msgTx.TxIn))
	for i := range msgTx.TxIn {
		var sigMsg bytes.Buffer
		sigMsg.WriteByte(0x00) // Epoch.
		sigMsg.WriteByte(0x00) // SIGHASH_DEFAULT.
		if err := binary.Write(&sigMsg, binary.LittleEndian, msgTx.Version); err != nil {
			return nil, err
		}
		if err := binary.Write(&sigMsg, binary.LittleEndian, msgTx.LockTime); err != nil {
			return nil, err
		}
		sigMsg.Write(hashPrevouts[:])
		sigMsg.Write(hashAmounts[:])
		sigMsg.Write(hashScriptPubKeys[:])
		sigMsg.Write(hashSequences[:])
		sigMsg.Write(hashOutputs[:])
		sigMsg.WriteByte(0x00) // Key path, no annex.
		if err := binary.Write(
			&sigMsg,
			binary.LittleEndian,
			uint32(i),
		); err != nil {
			return nil, err
		}

		digest := chainhash.TaggedHash([]byte("TapSighash"), sigMsg.Bytes())
		copy(result[i][:], digest[:])
	}

	return result, nil
}

func cloneFrostPreSignTransaction(
	transaction *FrostPreSignTransaction,
) *FrostPreSignTransaction {
	if transaction == nil {
		return nil
	}
	result := *transaction
	result.InputVector = append([]byte{}, transaction.InputVector...)
	result.OutputVector = append([]byte{}, transaction.OutputVector...)
	result.RawTransaction = append([]byte{}, transaction.RawTransaction...)
	result.InputValues = append([]uint64{}, transaction.InputValues...)
	result.SigningKeys = append([][32]byte{}, transaction.SigningKeys...)
	result.SignatureHashes = append([][32]byte{}, transaction.SignatureHashes...)
	result.SighashTypes = append([]uint8{}, transaction.SighashTypes...)
	result.SpendTypes = append([]uint8{}, transaction.SpendTypes...)
	result.ActionContext = cloneFrostPreSignActionContext(transaction.ActionContext)
	return &result
}

func cloneFrostPreSignActionContext(
	context *FrostPreSignActionContext,
) *FrostPreSignActionContext {
	if context == nil {
		return nil
	}
	result := &FrostPreSignActionContext{}
	if source := context.DepositSweep; source != nil {
		deposits := make([]*Deposit, len(source.Deposits))
		for i, deposit := range source.Deposits {
			deposits[i] = cloneFrostPreSignDeposit(deposit)
		}
		result.DepositSweep = &FrostPreSignDepositSweepActionContext{
			Proposal: cloneFrostPreSignDepositSweepProposal(source.Proposal),
			Deposits: deposits,
			MainUtxo: cloneFrostPreSignUtxo(source.MainUtxo),
		}
	}
	if source := context.Redemption; source != nil {
		result.Redemption = &FrostPreSignRedemptionActionContext{
			Proposal: cloneFrostPreSignRedemptionProposal(source.Proposal),
			MainUtxo: cloneFrostPreSignUtxo(source.MainUtxo),
		}
	}
	if source := context.MovingFunds; source != nil {
		result.MovingFunds = &FrostPreSignMovingFundsActionContext{
			Proposal: cloneFrostPreSignMovingFundsProposal(source.Proposal),
			MainUtxo: cloneFrostPreSignUtxo(source.MainUtxo),
		}
	}
	if source := context.MovedFundsSweep; source != nil {
		result.MovedFundsSweep = &FrostPreSignMovedFundsSweepActionContext{
			Proposal: cloneFrostPreSignMovedFundsSweepProposal(source.Proposal),
			MainUtxo: cloneFrostPreSignUtxo(source.MainUtxo),
		}
	}
	return result
}

func cloneFrostPreSignUtxo(
	utxo *bitcoin.UnspentTransactionOutput,
) *bitcoin.UnspentTransactionOutput {
	if utxo == nil {
		return nil
	}
	result := *utxo
	if utxo.Outpoint != nil {
		outpoint := *utxo.Outpoint
		result.Outpoint = &outpoint
	}
	return &result
}

func cloneFrostPreSignBitcoinTransaction(
	transaction *bitcoin.Transaction,
) *bitcoin.Transaction {
	if transaction == nil {
		return nil
	}
	result := &bitcoin.Transaction{}
	if err := result.Deserialize(transaction.Serialize(bitcoin.Standard)); err != nil {
		return nil
	}
	return result
}

func cloneFrostPreSignDeposit(deposit *Deposit) *Deposit {
	if deposit == nil {
		return nil
	}
	result := *deposit
	result.Utxo = cloneFrostPreSignUtxo(deposit.Utxo)
	result.FundingTx = cloneFrostPreSignBitcoinTransaction(deposit.FundingTx)
	if deposit.WalletXOnlyPublicKey != nil {
		value := *deposit.WalletXOnlyPublicKey
		result.WalletXOnlyPublicKey = &value
	}
	if deposit.RefundXOnlyPublicKey != nil {
		value := *deposit.RefundXOnlyPublicKey
		result.RefundXOnlyPublicKey = &value
	}
	if deposit.Vault != nil {
		value := *deposit.Vault
		result.Vault = &value
	}
	if deposit.ExtraData != nil {
		value := *deposit.ExtraData
		result.ExtraData = &value
	}
	return &result
}

func cloneFrostPreSignDepositSweepProposal(
	proposal *DepositSweepProposal,
) *DepositSweepProposal {
	if proposal == nil {
		return nil
	}
	result := *proposal
	result.DepositsKeys = append(result.DepositsKeys[:0:0], proposal.DepositsKeys...)
	result.DepositsRevealBlocks = make([]*big.Int, len(proposal.DepositsRevealBlocks))
	for i, block := range proposal.DepositsRevealBlocks {
		if block != nil {
			result.DepositsRevealBlocks[i] = new(big.Int).Set(block)
		}
	}
	if proposal.SweepTxFee != nil {
		result.SweepTxFee = new(big.Int).Set(proposal.SweepTxFee)
	}
	return &result
}

func cloneFrostPreSignRedemptionProposal(
	proposal *RedemptionProposal,
) *RedemptionProposal {
	if proposal == nil {
		return nil
	}
	result := *proposal
	result.RedeemersOutputScripts = make([]bitcoin.Script, len(proposal.RedeemersOutputScripts))
	for i, script := range proposal.RedeemersOutputScripts {
		result.RedeemersOutputScripts[i] = append(bitcoin.Script{}, script...)
	}
	if proposal.RedemptionTxFee != nil {
		result.RedemptionTxFee = new(big.Int).Set(proposal.RedemptionTxFee)
	}
	return &result
}

func cloneFrostPreSignMovingFundsProposal(
	proposal *MovingFundsProposal,
) *MovingFundsProposal {
	if proposal == nil {
		return nil
	}
	result := *proposal
	result.TargetWallets = append([][20]byte{}, proposal.TargetWallets...)
	if proposal.MovingFundsTxFee != nil {
		result.MovingFundsTxFee = new(big.Int).Set(proposal.MovingFundsTxFee)
	}
	return &result
}

func cloneFrostPreSignMovedFundsSweepProposal(
	proposal *MovedFundsSweepProposal,
) *MovedFundsSweepProposal {
	if proposal == nil {
		return nil
	}
	result := *proposal
	if proposal.SweepTxFee != nil {
		result.SweepTxFee = new(big.Int).Set(proposal.SweepTxFee)
	}
	return &result
}

// FrostPreSignFinality pins all post-relay reads to one finalized block. A
// backend must reject a block-number/hash mismatch instead of silently reading
// the canonical block at that height after a reorganization.
type FrostPreSignFinality struct {
	RelayTransactionHash [32]byte
	BlockNumber          uint64
	BlockHash            [32]byte
	TransactionIndex     uint32
	LogIndex             uint32
	// AuthorizationSequence is the registry's uint256 monotonic sequence from
	// P2TRAuthorizedVariantAdvanced, encoded as a canonical big-endian word.
	// It is zero only for a generic current-finalized checkpoint.
	AuthorizationSequence [32]byte
}

// FrostPreSignVariantSequence is the registry's canonical global order of
// authorization variants. It is independent of block/log ordering and remains
// monotonic even when several RBF variants finalize in one block.
type FrostPreSignVariantSequence struct {
	AuthorizationSequence [32]byte
}

func frostPreSignVariantSequence(
	finality FrostPreSignFinality,
) FrostPreSignVariantSequence {
	return FrostPreSignVariantSequence{
		AuthorizationSequence: finality.AuthorizationSequence,
	}
}

// FrostPreSignActivationProfile is the node operator's immutable, local trust
// anchor for one reviewed deployment. The backend cannot choose these values:
// its prepared proposal must match this profile before any seat attestation is
// signed. ProfileHash pins the canonical field serialization and is intended
// to be copied from the signed deployment manifest.
type FrostPreSignActivationProfile struct {
	DomainChainID             [32]byte
	ActivationManifestHash    [32]byte
	ImplementationSetHash     [32]byte
	BridgeAddress             [20]byte
	RegistryAddress           [20]byte
	CompleteRouter            [20]byte
	FrostRegistry             [20]byte
	ProposalValidator         [20]byte
	SortitionPool             [20]byte
	BridgeCodeHash            [32]byte
	RegistryCodeHash          [32]byte
	CompleteRouterCodeHash    [32]byte
	FrostRegistryCodeHash     [32]byte
	ProposalValidatorCodeHash [32]byte
	SortitionPoolCodeHash     [32]byte
	ReservationProtocolID     [32]byte
	EvidenceProtocolID        [32]byte
	SigningPolicyHash         [32]byte
	ProfileHash               [32]byte
}

// ComputeHash returns the canonical activation-profile commitment.
func (fpsap FrostPreSignActivationProfile) ComputeHash() [32]byte {
	hasher := sha256.New()
	hasher.Write([]byte("tbtc-frost-pre-sign-activation-profile-v4"))
	hasher.Write(fpsap.DomainChainID[:])
	hasher.Write(fpsap.ActivationManifestHash[:])
	hasher.Write(fpsap.ImplementationSetHash[:])
	hasher.Write(fpsap.BridgeAddress[:])
	hasher.Write(fpsap.RegistryAddress[:])
	hasher.Write(fpsap.CompleteRouter[:])
	hasher.Write(fpsap.FrostRegistry[:])
	hasher.Write(fpsap.ProposalValidator[:])
	hasher.Write(fpsap.SortitionPool[:])
	hasher.Write(fpsap.BridgeCodeHash[:])
	hasher.Write(fpsap.RegistryCodeHash[:])
	hasher.Write(fpsap.CompleteRouterCodeHash[:])
	hasher.Write(fpsap.FrostRegistryCodeHash[:])
	hasher.Write(fpsap.ProposalValidatorCodeHash[:])
	hasher.Write(fpsap.SortitionPoolCodeHash[:])
	hasher.Write(fpsap.ReservationProtocolID[:])
	hasher.Write(fpsap.EvidenceProtocolID[:])
	hasher.Write(fpsap.SigningPolicyHash[:])
	var result [32]byte
	copy(result[:], hasher.Sum(nil))
	return result
}

func (fpsap FrostPreSignActivationProfile) validate() error {
	for name, value := range map[string][32]byte{
		"domain chain ID":              fpsap.DomainChainID,
		"activation manifest hash":     fpsap.ActivationManifestHash,
		"implementation set hash":      fpsap.ImplementationSetHash,
		"Bridge code hash":             fpsap.BridgeCodeHash,
		"authorization registry hash":  fpsap.RegistryCodeHash,
		"COMPLETE router code hash":    fpsap.CompleteRouterCodeHash,
		"FROST registry code hash":     fpsap.FrostRegistryCodeHash,
		"proposal validator code hash": fpsap.ProposalValidatorCodeHash,
		"sortition pool code hash":     fpsap.SortitionPoolCodeHash,
		"reservation protocol ID":      fpsap.ReservationProtocolID,
		"evidence protocol ID":         fpsap.EvidenceProtocolID,
		"signing policy hash":          fpsap.SigningPolicyHash,
		"profile hash":                 fpsap.ProfileHash,
	} {
		if value == [32]byte{} {
			return fmt.Errorf("FROST pre-sign activation %s is zero", name)
		}
	}
	for name, value := range map[string][20]byte{
		"Bridge address":                 fpsap.BridgeAddress,
		"authorization registry address": fpsap.RegistryAddress,
		"COMPLETE router address":        fpsap.CompleteRouter,
		"FROST registry address":         fpsap.FrostRegistry,
		"proposal validator address":     fpsap.ProposalValidator,
		"sortition pool address":         fpsap.SortitionPool,
	} {
		if value == [20]byte{} {
			return fmt.Errorf("FROST pre-sign activation %s is zero", name)
		}
	}
	if fpsap.ReservationProtocolID != frostPreSignReservationProtocolID() {
		return fmt.Errorf("FROST pre-sign reservation protocol ID is not COMPLETE_V2")
	}
	if fpsap.SigningPolicyHash != frostPreSignSigningPolicyHash() {
		return fmt.Errorf("FROST pre-sign signing policy is not COMPLETE_V2")
	}
	if fpsap.EvidenceProtocolID != frostCompleteEvidenceProtocolID() {
		return fmt.Errorf("FROST fraud evidence protocol is not COMPLETE_V2")
	}
	if fpsap.ComputeHash() != fpsap.ProfileHash {
		return fmt.Errorf("FROST pre-sign activation profile hash mismatch")
	}
	return nil
}

// ValidateForProduction validates the immutable activation trust anchor. It is
// exported for deployment-specific chain adapters that must reject malformed
// signed manifests before exposing themselves as authorization backends.
func (fpsap FrostPreSignActivationProfile) ValidateForProduction() error {
	return fpsap.validate()
}

func (fpsap FrostPreSignActivationProfile) validateProposal(
	proposal *FrostPreSignAuthorizationProposal,
) error {
	if err := fpsap.validate(); err != nil {
		return err
	}
	if proposal == nil ||
		proposal.DomainChainID != fpsap.DomainChainID ||
		proposal.ActivationManifestHash != fpsap.ActivationManifestHash ||
		proposal.ImplementationSetHash != fpsap.ImplementationSetHash ||
		proposal.BridgeAddress != fpsap.BridgeAddress ||
		proposal.RegistryAddress != fpsap.RegistryAddress ||
		proposal.CompleteRouter != fpsap.CompleteRouter ||
		proposal.FrostRegistry != fpsap.FrostRegistry ||
		proposal.ProposalValidator != fpsap.ProposalValidator ||
		proposal.SortitionPool != fpsap.SortitionPool ||
		proposal.BridgeCodeHash != fpsap.BridgeCodeHash ||
		proposal.RegistryCodeHash != fpsap.RegistryCodeHash ||
		proposal.CompleteRouterCodeHash != fpsap.CompleteRouterCodeHash ||
		proposal.FrostRegistryCodeHash != fpsap.FrostRegistryCodeHash ||
		proposal.ProposalValidatorCodeHash != fpsap.ProposalValidatorCodeHash ||
		proposal.SortitionPoolCodeHash != fpsap.SortitionPoolCodeHash ||
		proposal.ReservationProtocolID != fpsap.ReservationProtocolID ||
		proposal.EvidenceProtocolID != fpsap.EvidenceProtocolID ||
		proposal.SigningPolicyHash != fpsap.SigningPolicyHash {
		return fmt.Errorf(
			"FROST pre-sign proposal differs from the local activation profile",
		)
	}
	return nil
}

// FrostPreSignAuthorizationProposal adds the exact state-derived reservation
// plan and immutable deployment expectations to a Bitcoin signing batch. The
// backend obtains these values from the canonical Bitcoin indexer and pinned
// Ethereum views before seat attestations are collected.
type FrostPreSignAuthorizationProposal struct {
	Transaction *FrostPreSignTransaction

	WalletID             [32]byte
	SnapshotHash         [32]byte
	ResourceHash         [32]byte
	OrderedInputRoot     [32]byte
	ApplyPlanHash        [32]byte
	ApplyPlanData1       [32]byte
	ApplyPlanData2       [32]byte
	FeeLimitSnapshot     uint64
	ResourceIDs          [][32]byte
	WalletMembersIDs     []uint32
	WalletMembersIDsHash [32]byte

	ReservationID     [32]byte
	AuthorizationRoot [32]byte
	Digest            [32]byte

	DomainChainID             [32]byte
	ActivationManifestHash    [32]byte
	ImplementationSetHash     [32]byte
	BridgeAddress             [20]byte
	RegistryAddress           [20]byte
	CompleteRouter            [20]byte
	FrostRegistry             [20]byte
	ProposalValidator         [20]byte
	SortitionPool             [20]byte
	BridgeCodeHash            [32]byte
	RegistryCodeHash          [32]byte
	CompleteRouterCodeHash    [32]byte
	FrostRegistryCodeHash     [32]byte
	ProposalValidatorCodeHash [32]byte
	SortitionPoolCodeHash     [32]byte
	ReservationProtocolID     [32]byte
	EvidenceProtocolID        [32]byte
	SigningPolicyHash         [32]byte
	PreparationFinality       FrostPreSignFinality
}

func (fpsap *FrostPreSignAuthorizationProposal) validate() error {
	if fpsap == nil {
		return fmt.Errorf("FROST pre-sign authorization proposal is nil")
	}
	if err := fpsap.Transaction.validate(); err != nil {
		return err
	}
	for name, value := range map[string][32]byte{
		"wallet ID":                        fpsap.WalletID,
		"snapshot hash":                    fpsap.SnapshotHash,
		"resource hash":                    fpsap.ResourceHash,
		"ordered input root":               fpsap.OrderedInputRoot,
		"apply plan hash":                  fpsap.ApplyPlanHash,
		"reservation ID":                   fpsap.ReservationID,
		"authorization root":               fpsap.AuthorizationRoot,
		"authorization digest":             fpsap.Digest,
		"wallet members IDs hash":          fpsap.WalletMembersIDsHash,
		"domain chain ID":                  fpsap.DomainChainID,
		"activation manifest hash":         fpsap.ActivationManifestHash,
		"implementation set hash":          fpsap.ImplementationSetHash,
		"Bridge code hash":                 fpsap.BridgeCodeHash,
		"authorization registry code hash": fpsap.RegistryCodeHash,
		"COMPLETE router code hash":        fpsap.CompleteRouterCodeHash,
		"FROST registry code hash":         fpsap.FrostRegistryCodeHash,
		"proposal validator code hash":     fpsap.ProposalValidatorCodeHash,
		"sortition pool code hash":         fpsap.SortitionPoolCodeHash,
		"reservation protocol ID":          fpsap.ReservationProtocolID,
		"evidence protocol ID":             fpsap.EvidenceProtocolID,
		"signing policy hash":              fpsap.SigningPolicyHash,
	} {
		if value == [32]byte{} {
			return fmt.Errorf("FROST pre-sign %s is zero", name)
		}
	}
	for name, value := range map[string][20]byte{
		"Bridge address":                 fpsap.BridgeAddress,
		"authorization registry address": fpsap.RegistryAddress,
		"COMPLETE router address":        fpsap.CompleteRouter,
		"FROST registry address":         fpsap.FrostRegistry,
		"proposal validator address":     fpsap.ProposalValidator,
		"sortition pool address":         fpsap.SortitionPool,
	} {
		if value == [20]byte{} {
			return fmt.Errorf("FROST pre-sign %s is zero", name)
		}
	}
	if len(fpsap.ResourceIDs) == 0 || len(fpsap.ResourceIDs) > 64 {
		return fmt.Errorf("invalid FROST pre-sign resource count [%d]", len(fpsap.ResourceIDs))
	}
	for i, resourceID := range fpsap.ResourceIDs {
		if resourceID == [32]byte{} {
			return fmt.Errorf("FROST pre-sign resource ID [%d] is zero", i)
		}
		if i > 0 && bytes.Compare(fpsap.ResourceIDs[i-1][:], resourceID[:]) >= 0 {
			return fmt.Errorf("FROST pre-sign resource IDs are not sorted and unique")
		}
	}
	if len(fpsap.WalletMembersIDs) < frostPreSignAuthorizationThreshold ||
		len(fpsap.WalletMembersIDs) > frostPreSignAuthorizationMaximumSeats {
		return fmt.Errorf(
			"invalid FROST wallet member count [%d]",
			len(fpsap.WalletMembersIDs),
		)
	}
	for i, memberID := range fpsap.WalletMembersIDs {
		if memberID == 0 {
			return fmt.Errorf("FROST wallet member ID [%d] is zero", i)
		}
	}
	if fpsap.PreparationFinality.BlockNumber == 0 ||
		fpsap.PreparationFinality.BlockHash == [32]byte{} {
		return fmt.Errorf("FROST pre-sign preparation is not pinned to finality")
	}
	if fpsap.ReservationProtocolID != frostPreSignReservationProtocolID() {
		return fmt.Errorf("FROST pre-sign reservation protocol ID is not COMPLETE_V2")
	}
	if fpsap.SigningPolicyHash != frostPreSignSigningPolicyHash() {
		return fmt.Errorf("FROST pre-sign signing policy is not COMPLETE_V2")
	}
	if fpsap.EvidenceProtocolID != frostCompleteEvidenceProtocolID() {
		return fmt.Errorf("FROST fraud evidence protocol is not COMPLETE_V2")
	}
	if err := fpsap.validateLocalCommitments(); err != nil {
		return err
	}

	return nil
}

func frostPreSignReservationProtocolID() [32]byte {
	return frostPreSignKeccak256([]byte(frostPreSignReservationProtocolDomain))
}

func frostPreSignSigningPolicyHash() [32]byte {
	return frostPreSignKeccak256([]byte(frostPreSignSigningPolicyDomain))
}

func frostCompleteEvidenceProtocolID() [32]byte {
	return frostPreSignKeccak256([]byte(frostCompleteEvidenceProtocolDomain))
}

func frostPreSignKeccak256(data []byte) [32]byte {
	digest := ethereumCrypto.Keccak256(data)
	result := [32]byte{}
	copy(result[:], digest)
	return result
}

func frostPreSignKeccak256Words(words ...[32]byte) [32]byte {
	encoded := make([]byte, 0, len(words)*32)
	for _, word := range words {
		encoded = append(encoded, word[:]...)
	}
	return frostPreSignKeccak256(encoded)
}

func frostPreSignABIUint8(value uint8) [32]byte {
	result := [32]byte{}
	result[31] = value
	return result
}

func frostPreSignABIUint32(value uint32) [32]byte {
	result := [32]byte{}
	binary.BigEndian.PutUint32(result[28:], value)
	return result
}

func frostPreSignABIUint64(value uint64) [32]byte {
	result := [32]byte{}
	binary.BigEndian.PutUint64(result[24:], value)
	return result
}

// Solidity ABI encodes fixed bytesN left-aligned, unlike address and integer
// values, which are right-aligned in a 32-byte word.
func frostPreSignABIBytes20(value [20]byte) [32]byte {
	result := [32]byte{}
	copy(result[:20], value[:])
	return result
}

func frostPreSignABIAddress(value [20]byte) [32]byte {
	result := [32]byte{}
	copy(result[12:], value[:])
	return result
}

func frostPreSignABIBytes32Array(values [][32]byte) []byte {
	// abi.encode(bytes32[]) = head offset || array length || elements.
	encoded := make([]byte, 0, 64+len(values)*32)
	offset := frostPreSignABIUint32(32)
	length := frostPreSignABIUint64(uint64(len(values)))
	encoded = append(encoded, offset[:]...)
	encoded = append(encoded, length[:]...)
	for _, value := range values {
		encoded = append(encoded, value[:]...)
	}
	return encoded
}

func frostPreSignABIUint32Array(values []uint32) []byte {
	// abi.encode(uint32[]) = head offset || array length || padded elements.
	encoded := make([]byte, 0, 64+len(values)*32)
	offset := frostPreSignABIUint32(32)
	length := frostPreSignABIUint64(uint64(len(values)))
	encoded = append(encoded, offset[:]...)
	encoded = append(encoded, length[:]...)
	for _, value := range values {
		word := frostPreSignABIUint32(value)
		encoded = append(encoded, word[:]...)
	}
	return encoded
}

func (fpsap *FrostPreSignAuthorizationProposal) computeAuthorizationRoot() (
	[32]byte,
	error,
) {
	if fpsap == nil || fpsap.Transaction == nil {
		return [32]byte{}, fmt.Errorf("FROST pre-sign authorization proposal is nil")
	}
	if len(fpsap.Transaction.SigningKeys) != len(fpsap.Transaction.SignatureHashes) {
		return [32]byte{}, fmt.Errorf("FROST pre-sign signing-key/digest vectors are not aligned")
	}

	identities := make([][32]byte, len(fpsap.Transaction.SigningKeys))
	for i := range identities {
		preimage := make(
			[]byte,
			0,
			len(frostPreSignChallengeIdentityDomain)+32+20+32+32+32,
		)
		preimage = append(preimage, []byte(frostPreSignChallengeIdentityDomain)...)
		preimage = append(preimage, fpsap.DomainChainID[:]...)
		preimage = append(preimage, fpsap.BridgeAddress[:]...)
		preimage = append(preimage, fpsap.WalletID[:]...)
		preimage = append(preimage, fpsap.Transaction.SigningKeys[i][:]...)
		preimage = append(preimage, fpsap.Transaction.SignatureHashes[i][:]...)
		identities[i] = sha256.Sum256(preimage)
	}

	return frostPreSignKeccak256(frostPreSignABIBytes32Array(identities)), nil
}

func (fpsap *FrostPreSignAuthorizationProposal) computeLockedPlanHash() [32]byte {
	return frostPreSignKeccak256Words(
		fpsap.ResourceHash,
		fpsap.OrderedInputRoot,
		fpsap.ApplyPlanData1,
		fpsap.ApplyPlanData2,
		frostPreSignABIUint64(fpsap.FeeLimitSnapshot),
	)
}

func (fpsap *FrostPreSignAuthorizationProposal) computeReservationID() [32]byte {
	walletScopeHash := frostPreSignKeccak256Words(
		frostPreSignABIUint8(uint8(fpsap.Transaction.Action)),
		frostPreSignABIBytes20(fpsap.Transaction.WalletPublicKeyHash),
		fpsap.WalletID,
		fpsap.WalletMembersIDsHash,
		fpsap.SnapshotHash,
	)
	return frostPreSignKeccak256Words(
		fpsap.ReservationProtocolID,
		fpsap.DomainChainID,
		frostPreSignABIAddress(fpsap.BridgeAddress),
		frostPreSignABIAddress(fpsap.RegistryAddress),
		frostPreSignABIAddress(fpsap.FrostRegistry),
		frostPreSignABIAddress(fpsap.ProposalValidator),
		walletScopeHash,
		fpsap.computeLockedPlanHash(),
	)
}

func (fpsap *FrostPreSignAuthorizationProposal) computeDigest(
	authorizationRoot [32]byte,
) [32]byte {
	return frostPreSignKeccak256Words(
		fpsap.ReservationProtocolID,
		fpsap.SigningPolicyHash,
		fpsap.DomainChainID,
		frostPreSignABIAddress(fpsap.BridgeAddress),
		frostPreSignABIAddress(fpsap.RegistryAddress),
		frostPreSignABIAddress(fpsap.FrostRegistry),
		frostPreSignABIAddress(fpsap.ProposalValidator),
		fpsap.computeReservationID(),
		[32]byte(fpsap.Transaction.TransactionHash),
		fpsap.ApplyPlanHash,
		authorizationRoot,
	)
}

func (fpsap *FrostPreSignAuthorizationProposal) validateLocalCommitments() error {
	expectedMembersHash := frostPreSignKeccak256(
		frostPreSignABIUint32Array(fpsap.WalletMembersIDs),
	)
	if fpsap.WalletMembersIDsHash != expectedMembersHash {
		return fmt.Errorf("FROST pre-sign wallet members IDs hash differs from local ABI encoding")
	}
	expectedResourceHash := frostPreSignKeccak256(
		frostPreSignABIBytes32Array(fpsap.ResourceIDs),
	)
	if fpsap.ResourceHash != expectedResourceHash {
		return fmt.Errorf("FROST pre-sign resource hash differs from local ABI encoding")
	}
	expectedAuthorizationRoot, err := fpsap.computeAuthorizationRoot()
	if err != nil {
		return err
	}
	if fpsap.AuthorizationRoot != expectedAuthorizationRoot {
		return fmt.Errorf("FROST pre-sign authorization root differs from local COMPLETE_V2 computation")
	}
	if fpsap.ReservationID != fpsap.computeReservationID() {
		return fmt.Errorf("FROST pre-sign reservation ID differs from local COMPLETE_V2 computation")
	}
	if fpsap.Digest != fpsap.computeDigest(expectedAuthorizationRoot) {
		return fmt.Errorf("FROST pre-sign authorization digest differs from local COMPLETE_V2 computation")
	}
	return nil
}

func cloneFrostPreSignAuthorizationProposal(
	proposal *FrostPreSignAuthorizationProposal,
) *FrostPreSignAuthorizationProposal {
	if proposal == nil {
		return nil
	}
	result := *proposal
	result.Transaction = cloneFrostPreSignTransaction(proposal.Transaction)
	result.ResourceIDs = append([][32]byte{}, proposal.ResourceIDs...)
	result.WalletMembersIDs = append([]uint32{}, proposal.WalletMembersIDs...)
	return &result
}

// FrostPreSignSeatAttestation is ABI-shaped: indices are one-based seat
// positions, strictly increasing, and signatures are packed in matching order.
// Operators are intentionally not deduplicated because one ordinary Ethereum
// operator key may occupy multiple distinct wallet seats.
type FrostPreSignSeatAttestation struct {
	WalletMembersIDs     []uint32
	SigningMemberIndices []uint8
	Signatures           []byte
}

// FrostPreSignAuthorizationState is the complete state re-read at one pinned
// finalized block. It contains enough information to reject proxy/crosslink or
// code changes, archived wallets, altered reservations, and mismatched RBF
// variants before native signing starts.
type FrostPreSignAuthorizationState struct {
	Finality FrostPreSignFinality

	DomainChainID             [32]byte
	ActivationManifestHash    [32]byte
	ImplementationSetHash     [32]byte
	BridgeAddress             [20]byte
	RegistryAddress           [20]byte
	CompleteRouter            [20]byte
	FrostRegistry             [20]byte
	ProposalValidator         [20]byte
	SortitionPool             [20]byte
	BridgeCodeHash            [32]byte
	RegistryCodeHash          [32]byte
	CompleteRouterCodeHash    [32]byte
	FrostRegistryCodeHash     [32]byte
	ProposalValidatorCodeHash [32]byte
	SortitionPoolCodeHash     [32]byte
	ReservationProtocolID     [32]byte
	EvidenceProtocolID        [32]byte
	SigningPolicyHash         [32]byte

	WalletActive         bool
	WalletID             [32]byte
	WalletPublicKeyHash  [20]byte
	WalletMembersIDsHash [32]byte
	WalletXOnlyOutputKey [32]byte

	ActiveReservationID            [32]byte
	ReservationWalletID            [32]byte
	ReservationWalletPublicKeyHash [20]byte
	ReservationSnapshotHash        [32]byte
	ReservationResourceHash        [32]byte
	ReservationOrderedInputRoot    [32]byte
	ReservationApplyPlanData1      [32]byte
	ReservationApplyPlanData2      [32]byte
	ReservationFeeLimitSnapshot    uint64
	ReservationAction              FrostPreSignAction
	ReservationActive              bool

	VariantTransactionHash        bitcoin.Hash
	VariantReservationID          [32]byte
	VariantAuthorizationRoot      [32]byte
	VariantApplyPlanHash          [32]byte
	VariantAuthorizationSequence  [32]byte
	VariantFraudDefenseAuthorized bool
	VariantSigningAllowed         bool

	LatestVariantTransactionHash       bitcoin.Hash
	LatestVariantAuthorizationSequence [32]byte
	LatestVariantSigningAllowed        bool
}

// FrostPreSignAuthorizationBackend is the ABI-independent anchoring boundary.
// The Ethereum implementation owns exact ABI packing, canonical-indexer plan
// derivation, relay submission, receipt finality, and block-hash-pinned calls.
// There is deliberately no permissive/default implementation: a network must
// supply a backend compiled against its reviewed COMPLETE ABI before node
// activation can pass the fail-closed startup checks.
type FrostPreSignAuthorizationBackend interface {
	PrepareFrostPreSignAuthorization(
		context.Context,
		*FrostPreSignTransaction,
		[]chain.Address,
	) (*FrostPreSignAuthorizationProposal, error)
	RelayFrostPreSignAuthorization(
		context.Context,
		*FrostPreSignAuthorizationProposal,
		*FrostPreSignSeatAttestation,
	) ([32]byte, error)
	WaitForFrostPreSignAuthorizationFinality(
		context.Context,
		[32]byte,
		*FrostPreSignAuthorizationProposal,
	) (*FrostPreSignFinality, error)
	// CurrentFrostPreSignFinality returns the latest canonical finalized block
	// checkpoint. RelayTransactionHash/transaction/log positions are ignored for
	// this checkpoint; BlockNumber and BlockHash are mandatory. Release guards
	// use it to detect a reservation settled or conflicted after its relay block.
	CurrentFrostPreSignFinality(
		context.Context,
	) (*FrostPreSignFinality, error)
	ReadFrostPreSignAuthorizationState(
		context.Context,
		*FrostPreSignAuthorizationProposal,
		FrostPreSignFinality,
	) (*FrostPreSignAuthorizationState, error)
}

// FrostPreSignAuthorizationConfigurator is implemented by production chain
// adapters that load and independently verify a deployment manifest before
// exposing the authorization backend.
type FrostPreSignAuthorizationConfigurator interface {
	ConfigureFrostPreSignAuthorization(
		context.Context,
		string,
		string,
		string,
	) (*FrostPreSignActivationProfile, error)
}

// FrostPreSignActivationPointVerifier authenticates one exact finalized
// Ethereum point and all deployment/proxy/library bindings there. Runtime
// readiness attestations use it instead of assuming the latest finalized head
// stayed unchanged while an activation audit was in flight.
type FrostPreSignActivationPointVerifier interface {
	VerifyFrostPreSignActivationPoint(
		context.Context,
		FrostPreSignFinality,
	) error
}

// FrostPreSignActivationRuntimeManifest is the signer-facing subset of the
// authenticated production activation envelope. It is immutable for the
// process lifetime and feeds the nonce-bound runtime status exporter.
type FrostPreSignActivationRuntimeManifest struct {
	ManifestHash                     [32]byte
	SignerProtocolID                 [32]byte
	ReservationProtocolID            [32]byte
	BitcoinOutboxProtocolID          [32]byte
	SigningPolicyHash                [32]byte
	DurableSessionStoreFingerprint   string
	CompleteRouterAddress            [20]byte
	AuthorizationRegistryAddress     [20]byte
	AttestationSignerKeyHash         [32]byte
	Threshold                        uint64
	MaximumGroupSize                 uint64
	RetainedGroupInventoryProtocolID [32]byte
}

type FrostPreSignActivationRuntimeManifestSource interface {
	FrostPreSignActivationRuntimeManifest() (
		FrostPreSignActivationRuntimeManifest,
		error,
	)
}

type frostPreSignAuthorization struct {
	ActivationProfileHash [32]byte
	AuthorizationID       [32]byte
	ReservationID         [32]byte
	VariantRoot           [32]byte
	TransactionHash       bitcoin.Hash
	Finality              FrostPreSignFinality
	VariantSequence       FrostPreSignVariantSequence
	proposal              *FrostPreSignAuthorizationProposal
}

type frostPreSignAuthorizationGate interface {
	authorize(
		context.Context,
		*FrostPreSignTransaction,
	) (*frostPreSignAuthorization, error)
	revalidate(
		context.Context,
		*frostPreSignAuthorization,
	) error
}

type thresholdFrostPreSignAuthorizationGate struct {
	backend             FrostPreSignAuthorizationBackend
	activationProfile   FrostPreSignActivationProfile
	signing             chain.Signing
	broadcastChannel    net.BroadcastChannel
	membershipValidator *group.MembershipValidator
	wallet              wallet
	localMemberIndexes  []group.MemberIndex
	threshold           int
}

func newThresholdFrostPreSignAuthorizationGate(
	backend FrostPreSignAuthorizationBackend,
	activationProfile FrostPreSignActivationProfile,
	signing chain.Signing,
	broadcastChannel net.BroadcastChannel,
	membershipValidator *group.MembershipValidator,
	wallet wallet,
	localMemberIndexes []group.MemberIndex,
) (*thresholdFrostPreSignAuthorizationGate, error) {
	if backend == nil {
		return nil, fmt.Errorf("FROST pre-sign authorization backend is nil")
	}
	if err := activationProfile.validate(); err != nil {
		return nil, err
	}
	if signing == nil {
		return nil, fmt.Errorf("FROST pre-sign Ethereum signer is nil")
	}
	if broadcastChannel == nil {
		return nil, fmt.Errorf("FROST pre-sign broadcast channel is nil")
	}
	if membershipValidator == nil {
		return nil, fmt.Errorf("FROST pre-sign membership validator is nil")
	}
	if len(localMemberIndexes) == 0 {
		return nil, fmt.Errorf("FROST pre-sign gate controls no wallet seats")
	}

	seen := make(map[group.MemberIndex]struct{})
	indexes := make([]group.MemberIndex, 0, len(localMemberIndexes))
	for _, memberIndex := range localMemberIndexes {
		if memberIndex == 0 || int(memberIndex) > wallet.groupSize() {
			return nil, fmt.Errorf("invalid local FROST wallet seat [%d]", memberIndex)
		}
		if _, ok := seen[memberIndex]; ok {
			continue
		}
		seen[memberIndex] = struct{}{}
		indexes = append(indexes, memberIndex)
	}
	sort.Slice(indexes, func(i, j int) bool { return indexes[i] < indexes[j] })

	registerFrostPreSignAuthorizationUnmarshaller(broadcastChannel)
	return &thresholdFrostPreSignAuthorizationGate{
		backend:             backend,
		activationProfile:   activationProfile,
		signing:             signing,
		broadcastChannel:    broadcastChannel,
		membershipValidator: membershipValidator,
		wallet:              wallet,
		localMemberIndexes:  indexes,
		threshold:           frostPreSignAuthorizationThreshold,
	}, nil
}

func (tfpsag *thresholdFrostPreSignAuthorizationGate) authorize(
	ctx context.Context,
	transaction *FrostPreSignTransaction,
) (*frostPreSignAuthorization, error) {
	if ctx == nil {
		return nil, fmt.Errorf("FROST pre-sign authorization context is nil")
	}
	if err := transaction.validate(); err != nil {
		return nil, err
	}
	// Freeze the locally constructed signing batch before crossing the backend
	// boundary. Passing the caller's pointer would let an adapter mutate both the
	// proposal and the comparison target in place, making pointer-equal data pass
	// reflect.DeepEqual after its canonical BIP-341 digests changed.
	frozenTransaction := cloneFrostPreSignTransaction(transaction)

	preparedProposal, err := tfpsag.backend.PrepareFrostPreSignAuthorization(
		ctx,
		cloneFrostPreSignTransaction(frozenTransaction),
		append([]chain.Address{}, tfpsag.wallet.signingGroupOperators...),
	)
	if err != nil {
		return nil, fmt.Errorf("cannot prepare FROST pre-sign authorization: [%w]", err)
	}
	proposal := cloneFrostPreSignAuthorizationProposal(preparedProposal)
	if err := proposal.validate(); err != nil {
		return nil, fmt.Errorf("invalid FROST pre-sign authorization proposal: [%w]", err)
	}
	if !reflect.DeepEqual(proposal.Transaction, frozenTransaction) {
		return nil, fmt.Errorf("authorization backend changed the proposed Bitcoin signing batch")
	}
	if len(proposal.WalletMembersIDs) != tfpsag.wallet.groupSize() {
		return nil, fmt.Errorf(
			"authorization wallet member count [%d] differs from local wallet [%d]",
			len(proposal.WalletMembersIDs),
			tfpsag.wallet.groupSize(),
		)
	}
	if err := tfpsag.activationProfile.validateProposal(proposal); err != nil {
		return nil, err
	}

	attestation, err := tfpsag.collectSeatAttestations(ctx, proposal)
	if err != nil {
		return nil, err
	}
	relayTransactionHash, err := tfpsag.backend.RelayFrostPreSignAuthorization(
		ctx,
		proposal,
		attestation,
	)
	if err != nil {
		return nil, fmt.Errorf("cannot relay FROST pre-sign authorization: [%w]", err)
	}
	if relayTransactionHash == [32]byte{} {
		return nil, fmt.Errorf("FROST pre-sign relay transaction hash is zero")
	}

	finality, err := tfpsag.backend.WaitForFrostPreSignAuthorizationFinality(
		ctx,
		relayTransactionHash,
		proposal,
	)
	if err != nil {
		return nil, fmt.Errorf("FROST pre-sign authorization did not finalize: [%w]", err)
	}
	if finality == nil || finality.RelayTransactionHash != relayTransactionHash ||
		finality.BlockNumber == 0 || finality.BlockHash == [32]byte{} ||
		finality.AuthorizationSequence == [32]byte{} {
		return nil, fmt.Errorf("invalid FROST pre-sign authorization finality proof")
	}

	authorization := &frostPreSignAuthorization{
		ActivationProfileHash: tfpsag.activationProfile.ProfileHash,
		AuthorizationID:       proposal.Digest,
		ReservationID:         proposal.ReservationID,
		VariantRoot:           proposal.AuthorizationRoot,
		TransactionHash:       frozenTransaction.TransactionHash,
		Finality:              *finality,
		VariantSequence:       frostPreSignVariantSequence(*finality),
		proposal:              proposal,
	}
	if err := tfpsag.revalidate(ctx, authorization); err != nil {
		return nil, err
	}

	return authorization, nil
}

func (tfpsag *thresholdFrostPreSignAuthorizationGate) revalidate(
	ctx context.Context,
	authorization *frostPreSignAuthorization,
) error {
	if authorization == nil || authorization.proposal == nil {
		return fmt.Errorf("FROST pre-sign authorization is nil")
	}
	if err := authorization.proposal.validate(); err != nil {
		return fmt.Errorf("FROST pre-sign authorization proposal changed: [%w]", err)
	}
	if authorization.ActivationProfileHash != tfpsag.activationProfile.ProfileHash ||
		authorization.AuthorizationID != authorization.proposal.Digest ||
		authorization.ReservationID != authorization.proposal.ReservationID ||
		authorization.VariantRoot != authorization.proposal.AuthorizationRoot ||
		authorization.TransactionHash != authorization.proposal.Transaction.TransactionHash ||
		authorization.VariantSequence != frostPreSignVariantSequence(authorization.Finality) {
		return fmt.Errorf("FROST pre-sign authorization identity changed")
	}

	if err := tfpsag.validatePinnedAuthorizationStateTwice(
		ctx,
		authorization.proposal,
		authorization.Finality,
		authorization.VariantSequence,
	); err != nil {
		return err
	}

	// The relay block remains a necessary canonicality witness, but it is not a
	// current authorization oracle. A reservation may be settled or conflicted in
	// any later finalized block. Pin the latest finalized head and require the
	// exact reservation/variant to remain active there before every nonce, share,
	// signature, enqueue, or replay release boundary.
	currentFinality, err := tfpsag.backend.CurrentFrostPreSignFinality(ctx)
	if err != nil {
		return fmt.Errorf("cannot obtain current FROST pre-sign finality: [%w]", err)
	}
	if currentFinality == nil ||
		currentFinality.BlockNumber < authorization.Finality.BlockNumber ||
		currentFinality.BlockHash == [32]byte{} {
		return fmt.Errorf("invalid current FROST pre-sign finalized checkpoint")
	}
	if currentFinality.BlockNumber == authorization.Finality.BlockNumber &&
		currentFinality.BlockHash != authorization.Finality.BlockHash {
		return fmt.Errorf("current FROST pre-sign checkpoint conflicts with relay finality")
	}
	if err := tfpsag.validatePinnedAuthorizationStateTwice(
		ctx,
		authorization.proposal,
		*currentFinality,
		authorization.VariantSequence,
	); err != nil {
		return err
	}

	return nil
}

func (tfpsag *thresholdFrostPreSignAuthorizationGate) validatePinnedAuthorizationStateTwice(
	ctx context.Context,
	proposal *FrostPreSignAuthorizationProposal,
	finality FrostPreSignFinality,
	variantSequence FrostPreSignVariantSequence,
) error {
	first, err := tfpsag.backend.ReadFrostPreSignAuthorizationState(
		ctx,
		proposal,
		finality,
	)
	if err != nil {
		return fmt.Errorf("cannot read finalized FROST pre-sign authorization: [%w]", err)
	}
	if err := validateFrostPreSignAuthorizationState(
		proposal,
		finality,
		variantSequence,
		first,
	); err != nil {
		return err
	}
	second, err := tfpsag.backend.ReadFrostPreSignAuthorizationState(
		ctx,
		proposal,
		finality,
	)
	if err != nil {
		return fmt.Errorf("cannot re-read finalized FROST pre-sign authorization: [%w]", err)
	}
	if err := validateFrostPreSignAuthorizationState(
		proposal,
		finality,
		variantSequence,
		second,
	); err != nil {
		return err
	}
	if !reflect.DeepEqual(first, second) {
		return fmt.Errorf("finalized FROST pre-sign authorization changed between pinned reads")
	}
	return nil
}

func validateFrostPreSignAuthorizationState(
	proposal *FrostPreSignAuthorizationProposal,
	finality FrostPreSignFinality,
	variantSequence FrostPreSignVariantSequence,
	state *FrostPreSignAuthorizationState,
) error {
	if state == nil {
		return fmt.Errorf("finalized FROST pre-sign authorization state is nil")
	}
	if state.Finality != finality {
		return fmt.Errorf("finalized FROST pre-sign block number/hash changed")
	}
	if state.DomainChainID != proposal.DomainChainID ||
		state.ActivationManifestHash != proposal.ActivationManifestHash ||
		state.ImplementationSetHash != proposal.ImplementationSetHash ||
		state.BridgeAddress != proposal.BridgeAddress ||
		state.RegistryAddress != proposal.RegistryAddress ||
		state.CompleteRouter != proposal.CompleteRouter ||
		state.FrostRegistry != proposal.FrostRegistry ||
		state.ProposalValidator != proposal.ProposalValidator ||
		state.SortitionPool != proposal.SortitionPool ||
		state.BridgeCodeHash != proposal.BridgeCodeHash ||
		state.RegistryCodeHash != proposal.RegistryCodeHash ||
		state.CompleteRouterCodeHash != proposal.CompleteRouterCodeHash ||
		state.FrostRegistryCodeHash != proposal.FrostRegistryCodeHash ||
		state.ProposalValidatorCodeHash != proposal.ProposalValidatorCodeHash ||
		state.SortitionPoolCodeHash != proposal.SortitionPoolCodeHash ||
		state.ReservationProtocolID != proposal.ReservationProtocolID ||
		state.EvidenceProtocolID != proposal.EvidenceProtocolID ||
		state.SigningPolicyHash != proposal.SigningPolicyHash {
		return fmt.Errorf("finalized FROST deployment domain/crosslink/code mismatch")
	}
	if !state.WalletActive ||
		state.WalletID != proposal.WalletID ||
		state.WalletPublicKeyHash != proposal.Transaction.WalletPublicKeyHash ||
		state.WalletMembersIDsHash != proposal.WalletMembersIDsHash ||
		state.WalletXOnlyOutputKey != proposal.WalletID {
		return fmt.Errorf("FROST wallet is not active under the finalized registry crosslink")
	}
	if state.ActiveReservationID != proposal.ReservationID ||
		state.ReservationWalletID != proposal.WalletID ||
		state.ReservationWalletPublicKeyHash != proposal.Transaction.WalletPublicKeyHash ||
		state.ReservationSnapshotHash != proposal.SnapshotHash ||
		state.ReservationResourceHash != proposal.ResourceHash ||
		state.ReservationOrderedInputRoot != proposal.OrderedInputRoot ||
		state.ReservationApplyPlanData1 != proposal.ApplyPlanData1 ||
		state.ReservationApplyPlanData2 != proposal.ApplyPlanData2 ||
		state.ReservationFeeLimitSnapshot != proposal.FeeLimitSnapshot ||
		state.ReservationAction != proposal.Transaction.Action ||
		!state.ReservationActive {
		return fmt.Errorf("finalized FROST reservation differs from the attested proposal")
	}
	if state.VariantTransactionHash != proposal.Transaction.TransactionHash ||
		state.VariantReservationID != proposal.ReservationID ||
		state.VariantAuthorizationRoot != proposal.AuthorizationRoot ||
		state.VariantApplyPlanHash != proposal.ApplyPlanHash ||
		state.VariantAuthorizationSequence != variantSequence.AuthorizationSequence ||
		!state.VariantFraudDefenseAuthorized ||
		!state.VariantSigningAllowed {
		return fmt.Errorf("finalized FROST transaction variant differs from the attested proposal")
	}
	if state.LatestVariantTransactionHash != proposal.Transaction.TransactionHash ||
		state.LatestVariantAuthorizationSequence != variantSequence.AuthorizationSequence ||
		!state.LatestVariantSigningAllowed {
		return fmt.Errorf("finalized FROST transaction variant has been superseded")
	}

	return nil
}

type frostPreSignAuthorizationMessage struct {
	SenderIDValue uint32 `json:"senderID"`
	Digest        []byte `json:"digest"`
	PublicKey     []byte `json:"publicKey"`
	Signature     []byte `json:"signature"`
}

func (fpsam *frostPreSignAuthorizationMessage) SenderID() group.MemberIndex {
	return group.MemberIndex(fpsam.SenderIDValue)
}

func (fpsam *frostPreSignAuthorizationMessage) Type() string {
	return frostPreSignAuthorizationMessageTypePrefix + "seat_attestation"
}

func (fpsam *frostPreSignAuthorizationMessage) Marshal() ([]byte, error) {
	return json.Marshal(fpsam)
}

func (fpsam *frostPreSignAuthorizationMessage) Unmarshal(data []byte) error {
	if err := json.Unmarshal(data, fpsam); err != nil {
		return err
	}
	if fpsam.SenderID() == 0 {
		return fmt.Errorf("sender seat is zero")
	}
	if len(fpsam.Digest) != 32 {
		return fmt.Errorf("authorization digest length [%d] is not 32", len(fpsam.Digest))
	}
	if len(fpsam.PublicKey) == 0 {
		return fmt.Errorf("operator public key is empty")
	}
	if len(fpsam.Signature) != 65 {
		return fmt.Errorf("operator signature length [%d] is not 65", len(fpsam.Signature))
	}
	return nil
}

func registerFrostPreSignAuthorizationUnmarshaller(channel net.BroadcastChannel) {
	channel.SetUnmarshaler(func() net.TaggedUnmarshaler {
		return &frostPreSignAuthorizationMessage{}
	})
}

func (tfpsag *thresholdFrostPreSignAuthorizationGate) collectSeatAttestations(
	ctx context.Context,
	proposal *FrostPreSignAuthorizationProposal,
) (*FrostPreSignSeatAttestation, error) {
	signature, err := tfpsag.signing.Sign(proposal.Digest[:])
	if err != nil {
		return nil, fmt.Errorf("cannot sign FROST pre-sign authorization digest: [%w]", err)
	}
	if len(signature) != 65 {
		return nil, fmt.Errorf("FROST pre-sign Ethereum signature length [%d] is not 65", len(signature))
	}

	localSeats := make(map[group.MemberIndex]struct{})
	signatures := make(map[group.MemberIndex][]byte)
	for _, memberIndex := range tfpsag.localMemberIndexes {
		localSeats[memberIndex] = struct{}{}
		signatures[memberIndex] = append([]byte{}, signature...)
		message := &frostPreSignAuthorizationMessage{
			SenderIDValue: uint32(memberIndex),
			Digest:        append([]byte{}, proposal.Digest[:]...),
			PublicKey:     append([]byte{}, tfpsag.signing.PublicKey()...),
			Signature:     append([]byte{}, signature...),
		}
		if err := tfpsag.broadcastChannel.Send(
			ctx,
			message,
			net.BackoffRetransmissionStrategy,
		); err != nil {
			return nil, fmt.Errorf("cannot broadcast FROST pre-sign seat attestation: [%w]", err)
		}
	}

	// Admit at most one message per authenticated remote seat. A Byzantine seat
	// can always withhold its own attestation, but it must not be able to enqueue
	// unlimited invalid retransmissions, keep a finite buffer full, and make the
	// other honest seats' messages drop. With one slot per remote seat, groupSize+1
	// is a strict upper bound even when callbacks run concurrently.
	messageChannel := make(chan *frostPreSignAuthorizationMessage, len(proposal.WalletMembersIDs)+1)
	seenRemoteSeats := make(map[group.MemberIndex]struct{})
	var seenRemoteSeatsMutex sync.Mutex
	receiveCtx, cancelReceive := context.WithCancel(ctx)
	defer cancelReceive()
	tfpsag.broadcastChannel.Recv(receiveCtx, func(message net.Message) {
		payload, ok := message.Payload().(*frostPreSignAuthorizationMessage)
		if !ok || payload == nil {
			return
		}
		if !frostPreSignTransportPublicKeyMatches(
			payload.PublicKey,
			message.SenderPublicKey(),
		) {
			return
		}
		seat := payload.SenderID()
		if _, isLocal := localSeats[seat]; isLocal || seat == 0 || int(seat) > len(proposal.WalletMembersIDs) {
			return
		}
		if !tfpsag.membershipValidator.IsValidMembership(seat, message.SenderPublicKey()) {
			return
		}
		if !claimFrostPreSignRemoteSeat(
			seat,
			len(proposal.WalletMembersIDs),
			localSeats,
			seenRemoteSeats,
			&seenRemoteSeatsMutex,
		) {
			return
		}
		select {
		case messageChannel <- payload:
		default:
			logger.Warnf("dropping FROST pre-sign seat attestation [%d]; collector buffer full", seat)
		}
	})

	for len(signatures) < tfpsag.threshold {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("FROST pre-sign seat attestation collection interrupted: [%w]", ctx.Err())
		case message := <-messageChannel:
			seat := message.SenderID()
			if !bytes.Equal(message.Digest, proposal.Digest[:]) {
				continue
			}
			valid, err := tfpsag.signing.VerifyWithPublicKey(
				proposal.Digest[:],
				message.Signature,
				message.PublicKey,
			)
			if err != nil || !valid {
				continue
			}
			expectedOperator := tfpsag.wallet.signingGroupOperators[seat-1]
			actualOperator := tfpsag.signing.PublicKeyBytesToAddress(message.PublicKey)
			if actualOperator != expectedOperator {
				continue
			}
			if existing, ok := signatures[seat]; ok {
				if !bytes.Equal(existing, message.Signature) {
					logger.Warnf("dropping conflicting FROST pre-sign signature for seat [%d]", seat)
				}
				continue
			}
			signatures[seat] = append([]byte{}, message.Signature...)
		}
	}

	return buildFrostPreSignSeatAttestation(
		proposal.WalletMembersIDs,
		signatures,
		tfpsag.threshold,
	)
}

func claimFrostPreSignRemoteSeat(
	seat group.MemberIndex,
	walletSize int,
	localSeats map[group.MemberIndex]struct{},
	seenRemoteSeats map[group.MemberIndex]struct{},
	mutex *sync.Mutex,
) bool {
	if seat == 0 || int(seat) > walletSize ||
		seenRemoteSeats == nil || mutex == nil {
		return false
	}
	if _, isLocal := localSeats[seat]; isLocal {
		return false
	}
	mutex.Lock()
	defer mutex.Unlock()
	if _, seen := seenRemoteSeats[seat]; seen {
		return false
	}
	seenRemoteSeats[seat] = struct{}{}
	return true
}

func frostPreSignTransportPublicKeyMatches(
	payloadPublicKey []byte,
	transportPublicKey []byte,
) bool {
	return len(payloadPublicKey) > 0 &&
		bytes.Equal(payloadPublicKey, transportPublicKey)
}

func buildFrostPreSignSeatAttestation(
	walletMembersIDs []uint32,
	signatures map[group.MemberIndex][]byte,
	threshold int,
) (*FrostPreSignSeatAttestation, error) {
	if threshold <= 0 || len(signatures) < threshold {
		return nil, fmt.Errorf(
			"insufficient FROST pre-sign seat attestations [%d/%d]",
			len(signatures),
			threshold,
		)
	}
	seats := make([]int, 0, len(signatures))
	for seat := range signatures {
		seats = append(seats, int(seat))
	}
	sort.Ints(seats)
	if len(seats) > threshold {
		seats = seats[:threshold]
	}
	indices := make([]uint8, 0, len(seats))
	packedSignatures := make([]byte, 0, len(seats)*65)
	for _, seat := range seats {
		if seat > 255 {
			return nil, fmt.Errorf("FROST pre-sign seat [%d] does not fit uint8", seat)
		}
		if seat <= 0 || seat > len(walletMembersIDs) {
			return nil, fmt.Errorf("FROST pre-sign seat [%d] is outside the wallet", seat)
		}
		if len(signatures[group.MemberIndex(seat)]) != 65 {
			return nil, fmt.Errorf("FROST pre-sign seat [%d] signature is not 65 bytes", seat)
		}
		indices = append(indices, uint8(seat))
		packedSignatures = append(packedSignatures, signatures[group.MemberIndex(seat)]...)
	}

	return &FrostPreSignSeatAttestation{
		WalletMembersIDs:     append([]uint32{}, walletMembersIDs...),
		SigningMemberIndices: indices,
		Signatures:           packedSignatures,
	}, nil
}

func frostPreSignTransactionIdentity(transaction *FrostPreSignTransaction) [32]byte {
	if transaction == nil {
		return [32]byte{}
	}
	hasher := sha256.New()
	hasher.Write([]byte("tbtc-frost-pre-sign-transaction-v1"))
	hasher.Write([]byte{byte(transaction.Action)})
	hasher.Write(transaction.WalletPublicKeyHash[:])
	hasher.Write(transaction.TransactionHash[:])
	for _, signatureHash := range transaction.SignatureHashes {
		hasher.Write(signatureHash[:])
	}
	for i := range transaction.SighashTypes {
		hasher.Write([]byte{transaction.SighashTypes[i], transaction.SpendTypes[i]})
	}
	for _, value := range transaction.InputValues {
		var valueBytes [8]byte
		binary.BigEndian.PutUint64(valueBytes[:], value)
		hasher.Write(valueBytes[:])
	}
	for _, signingKey := range transaction.SigningKeys {
		hasher.Write(signingKey[:])
	}
	var result [32]byte
	copy(result[:], hasher.Sum(nil))
	return result
}
