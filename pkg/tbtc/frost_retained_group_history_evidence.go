package tbtc

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	ethabi "github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	frostabi "github.com/keep-network/keep-core/pkg/chain/ethereum/frost/gen/abi"
	bridgeabi "github.com/keep-network/keep-core/pkg/chain/ethereum/tbtc/gen/abi"
)

// FrostRetainedGroupActivationEvidenceBinder binds a retained-group history
// source to the exact deployment and descriptor set authenticated by the
// signed activation manifest. Production activation requires this binding
// before the source can authenticate semantic history or operator IDs.
type FrostRetainedGroupActivationEvidenceBinder interface {
	BindFrostRetainedGroupActivationEvidence(
		FrostPreSignActivationProfile,
		[32]byte,
		[32]byte,
	) error
}

type frostRetainedGroupContractPin struct {
	address  common.Address
	codeHash common.Hash
}

type frostRetainedGroupEvidenceProfile struct {
	manifestHash      [32]byte
	descriptorSetHash [32]byte
	bridge            frostRetainedGroupContractPin
	registry          frostRetainedGroupContractPin
	sortitionPool     frostRetainedGroupContractPin
	bridgeABI         ethabi.ABI
	registryABI       ethabi.ABI
	membersArguments  ethabi.Arguments
}

type frostRetainedGroupReceiptCache map[common.Hash]*types.Receipt

type frostRetainedGroupCodeCacheKey struct {
	address     common.Address
	blockNumber uint64
}

type frostRetainedGroupCodeCache map[frostRetainedGroupCodeCacheKey]struct{}

var _ FrostRetainedGroupActivationEvidenceBinder = (*signedFrostRetainedGroupHistorySource)(nil)

// BindFrostRetainedGroupActivationEvidence is deliberately one-shot. The
// profile and descriptor are supplied only after the activation envelope has
// been signature-checked and converted to its immutable runtime manifest.
func (source *signedFrostRetainedGroupHistorySource) BindFrostRetainedGroupActivationEvidence(
	profile FrostPreSignActivationProfile,
	manifestHash [32]byte,
	descriptorSetHash [32]byte,
) error {
	if source == nil {
		return fmt.Errorf("retained-group history source is nil")
	}
	if err := profile.ValidateForProduction(); err != nil {
		return fmt.Errorf("retained-group activation profile is invalid: [%w]", err)
	}
	if manifestHash == [32]byte{} || descriptorSetHash == [32]byte{} ||
		profile.ActivationManifestHash != manifestHash {
		return fmt.Errorf("retained-group evidence does not match the signed activation manifest")
	}
	parsedBridgeABI, err := ethabi.JSON(strings.NewReader(bridgeabi.BridgeMetaData.ABI))
	if err != nil {
		return fmt.Errorf("cannot parse pinned Bridge ABI: [%w]", err)
	}
	parsedRegistryABI, err := ethabi.JSON(strings.NewReader(frostabi.FrostWalletRegistryMetaData.ABI))
	if err != nil {
		return fmt.Errorf("cannot parse pinned FROST registry ABI: [%w]", err)
	}
	membersType, err := ethabi.NewType("uint32[]", "", nil)
	if err != nil {
		return fmt.Errorf("cannot construct pinned DKG members descriptor: [%w]", err)
	}
	evidence := &frostRetainedGroupEvidenceProfile{
		manifestHash:      manifestHash,
		descriptorSetHash: descriptorSetHash,
		bridge: frostRetainedGroupContractPin{
			address:  common.Address(profile.BridgeAddress),
			codeHash: common.Hash(profile.BridgeCodeHash),
		},
		registry: frostRetainedGroupContractPin{
			address:  common.Address(profile.FrostRegistry),
			codeHash: common.Hash(profile.FrostRegistryCodeHash),
		},
		sortitionPool: frostRetainedGroupContractPin{
			address:  common.Address(profile.SortitionPool),
			codeHash: common.Hash(profile.SortitionPoolCodeHash),
		},
		bridgeABI:        parsedBridgeABI,
		registryABI:      parsedRegistryABI,
		membersArguments: ethabi.Arguments{{Type: membersType}},
	}
	for name, event := range map[string]struct {
		contract *ethabi.ABI
		topic    common.Hash
	}{
		"DkgResultSubmitted":    {&evidence.registryABI, common.HexToHash("0xbfc6cd6291b6741d3ac1631ba81a0288d08265bea4d59d452e8c953e11ec11c6")},
		"DkgResultApproved":     {&evidence.registryABI, common.HexToHash("0xe6e9d5eba171e82025efb3f3d44fd35905e7283d104284cb9f3bbc5bf1e4276f")},
		"WalletCreated":         {&evidence.registryABI, common.HexToHash("0xbe8f27cef1f3d94120c9c547c3614f5b992fdb0c0a497cc920fde06546291ab4")},
		"WalletClosed":          {&evidence.registryABI, common.HexToHash("0xa6ae4af610b8ada39d3675190ead27a5552631a8e33f53e4e37dbb082f11a73e")},
		"NewWalletRegisteredV2": {&evidence.bridgeABI, common.HexToHash("0x6a501a1d441e1c8b5490e52589d0d27d35504cf1063a8c848fef40f326710d4b")},
		"WalletMovingFunds":     {&evidence.bridgeABI, common.HexToHash("0xbdc9ce990a067e5fd3a5d8dfc68e27e9f221aaa3fe55265e0b7e93c460b3efe2")},
		"WalletClosing":         {&evidence.bridgeABI, common.HexToHash("0x68cb496f5e64383745876664ef119840f154a729c03ba866b8aecb5c9f53d516")},
		"BridgeWalletClosed":    {&evidence.bridgeABI, common.HexToHash("0x47b159947c3066cb253f60e8f046cfd747411788a545cb189679e3fa1467b28d")},
		"WalletTerminated":      {&evidence.bridgeABI, common.HexToHash("0x9272a280b0f32f70b00ad0b546499c68e3ecc6f7bb7ef43491ec5d7b99bf69ef")},
	} {
		eventName := name
		if name == "BridgeWalletClosed" {
			eventName = "WalletClosed"
		}
		parsedEvent, ok := event.contract.Events[eventName]
		if !ok || parsedEvent.ID != event.topic {
			return fmt.Errorf("pinned retained-group event descriptor [%s] is unavailable or changed", name)
		}
	}

	source.evidenceMutex.Lock()
	defer source.evidenceMutex.Unlock()
	if source.evidence != nil {
		return fmt.Errorf("retained-group activation evidence is already bound")
	}
	source.evidence = evidence
	return nil
}

func (source *signedFrostRetainedGroupHistorySource) activationEvidence() (
	*frostRetainedGroupEvidenceProfile,
	error,
) {
	if source == nil {
		return nil, fmt.Errorf("retained-group history source is nil")
	}
	source.evidenceMutex.RLock()
	defer source.evidenceMutex.RUnlock()
	if source.evidence == nil {
		return nil, fmt.Errorf("retained-group history source is not bound to the signed activation manifest")
	}
	return source.evidence, nil
}

func (source *signedFrostRetainedGroupHistorySource) verifyHistoryEvidence(
	ctx context.Context,
	mutations []FrostRetainedGroupMutation,
	evidence *frostRetainedGroupEvidenceProfile,
) error {
	if evidence == nil {
		return fmt.Errorf("retained-group activation evidence is nil")
	}
	receipts := make(frostRetainedGroupReceiptCache)
	code := make(frostRetainedGroupCodeCache)
	for index, mutation := range mutations {
		if err := source.verifyMutationEvidence(ctx, mutation, evidence, receipts, code); err != nil {
			return fmt.Errorf("mutation [%d] [%s]: [%w]", index, mutation.Kind, err)
		}
	}
	return nil
}

func (source *signedFrostRetainedGroupHistorySource) verifyMutationEvidence(
	ctx context.Context,
	mutation FrostRetainedGroupMutation,
	evidence *frostRetainedGroupEvidenceProfile,
	receipts frostRetainedGroupReceiptCache,
	code frostRetainedGroupCodeCache,
) error {
	// Quarantine, recovery-required, and lift records are signed operational
	// records, not Ethereum events. Their Point is only a canonical finalized
	// ordering anchor and is deliberately never presented as receipt evidence.
	if isFrostRetainedGroupQuarantineMutation(mutation.Kind) {
		return nil
	}

	switch mutation.Kind {
	case FrostRetainedGroupAdmissionMutation:
		return source.verifyAdmissionEvidence(ctx, mutation, evidence, receipts, code)
	case FrostRetainedGroupMovingFundsMutation,
		FrostRetainedGroupClosingMutation,
		FrostRetainedGroupClosedMutation,
		FrostRetainedGroupTerminatedMutation:
		eventName := map[FrostRetainedGroupMutationKind]string{
			FrostRetainedGroupMovingFundsMutation: "WalletMovingFunds",
			FrostRetainedGroupClosingMutation:     "WalletClosing",
			FrostRetainedGroupClosedMutation:      "WalletClosed",
			FrostRetainedGroupTerminatedMutation:  "WalletTerminated",
		}[mutation.Kind]
		log, err := source.authenticatedEventLog(
			ctx,
			mutation.Point,
			evidence.bridge,
			evidence.bridgeABI.Events[eventName].ID,
			receipts,
			code,
		)
		if err != nil {
			return err
		}
		if len(log.Topics) != 3 || len(log.Data) != 0 || log.Topics[1] != (common.Hash{}) ||
			log.Topics[2] != frostRetainedGroupBytes20Topic(mutation.WalletPublicKeyHash) {
			return fmt.Errorf("Bridge lifecycle log does not encode the exported FROST wallet")
		}
		return nil
	case FrostRetainedGroupRegistryClosureMutation:
		log, err := source.authenticatedEventLog(
			ctx,
			mutation.Point,
			evidence.registry,
			evidence.registryABI.Events["WalletClosed"].ID,
			receipts,
			code,
		)
		if err != nil {
			return err
		}
		if len(log.Topics) != 2 || len(log.Data) != 0 || log.Topics[1] != common.Hash(mutation.WalletID) {
			return fmt.Errorf("FROST registry closure log does not encode the exported wallet")
		}
		return nil
	default:
		return fmt.Errorf("unsupported retained-group mutation kind [%s]", mutation.Kind)
	}
}

func (source *signedFrostRetainedGroupHistorySource) verifyAdmissionEvidence(
	ctx context.Context,
	mutation FrostRetainedGroupMutation,
	evidence *frostRetainedGroupEvidenceProfile,
	receipts frostRetainedGroupReceiptCache,
	code frostRetainedGroupCodeCache,
) error {
	if mutation.Point != mutation.BridgeRegistrationPoint ||
		compareFrostRetainedGroupEventPoints(mutation.DkgSubmissionPoint, mutation.DkgApprovalPoint) >= 0 ||
		!sameFrostRetainedGroupTransaction(mutation.DkgApprovalPoint, mutation.CreationPoint) ||
		!sameFrostRetainedGroupTransaction(mutation.CreationPoint, mutation.BridgeRegistrationPoint) ||
		mutation.DkgApprovalPoint.LogIndex >= mutation.CreationPoint.LogIndex ||
		mutation.CreationPoint.LogIndex >= mutation.BridgeRegistrationPoint.LogIndex {
		return fmt.Errorf("admission evidence points are not the required DKG/registration sequence")
	}

	submissionLog, err := source.authenticatedEventLog(
		ctx,
		mutation.DkgSubmissionPoint,
		evidence.registry,
		evidence.registryABI.Events["DkgResultSubmitted"].ID,
		receipts,
		code,
	)
	if err != nil {
		return err
	}
	result, resultHash, err := frostRetainedGroupDecodeDkgSubmission(
		submissionLog,
		evidence,
	)
	if err != nil {
		return err
	}
	if resultHash != mutation.DkgResultHash || result.XOnlyOutputKey != mutation.WalletID ||
		result.MembersHash != mutation.RetainedGroupHash ||
		!frostRetainedGroupEqualOperatorIDs(result.Members, mutation.OperatorIDs) {
		return fmt.Errorf("DKG submission does not encode the exported admission")
	}
	encodedMembers, err := evidence.membersArguments.Pack(result.Members)
	if err != nil || crypto.Keccak256Hash(encodedMembers) != common.Hash(result.MembersHash) {
		return fmt.Errorf("DKG members hash does not commit to the exact ordered member IDs")
	}

	approvalLog, err := source.authenticatedEventLog(
		ctx,
		mutation.DkgApprovalPoint,
		evidence.registry,
		evidence.registryABI.Events["DkgResultApproved"].ID,
		receipts,
		code,
	)
	if err != nil {
		return err
	}
	if len(approvalLog.Topics) != 3 || len(approvalLog.Data) != 0 ||
		approvalLog.Topics[1] != common.Hash(mutation.DkgResultHash) ||
		approvalLog.Topics[2] == (common.Hash{}) ||
		!frostRetainedGroupCanonicalAddressTopic(approvalLog.Topics[2]) {
		return fmt.Errorf("DKG approval log does not approve the exported result")
	}

	creationLog, err := source.authenticatedEventLog(
		ctx,
		mutation.CreationPoint,
		evidence.registry,
		evidence.registryABI.Events["WalletCreated"].ID,
		receipts,
		code,
	)
	if err != nil {
		return err
	}
	if len(creationLog.Topics) != 3 || len(creationLog.Data) != 0 ||
		creationLog.Topics[1] != common.Hash(mutation.WalletID) ||
		creationLog.Topics[2] != common.Hash(mutation.DkgResultHash) {
		return fmt.Errorf("FROST wallet-creation log does not encode the exported admission")
	}

	registrationLog, err := source.authenticatedEventLog(
		ctx,
		mutation.BridgeRegistrationPoint,
		evidence.bridge,
		evidence.bridgeABI.Events["NewWalletRegisteredV2"].ID,
		receipts,
		code,
	)
	if err != nil {
		return err
	}
	if len(registrationLog.Topics) != 4 || len(registrationLog.Data) != 0 ||
		registrationLog.Topics[1] != common.Hash(mutation.WalletID) ||
		registrationLog.Topics[2] != (common.Hash{}) ||
		registrationLog.Topics[3] != frostRetainedGroupBytes20Topic(mutation.WalletPublicKeyHash) {
		return fmt.Errorf("Bridge registration log does not encode the exported FROST wallet")
	}
	return nil
}

func frostRetainedGroupDecodeDkgSubmission(
	log *types.Log,
	evidence *frostRetainedGroupEvidenceProfile,
) (result frostabi.FrostDkgResult, resultHash [32]byte, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			result = frostabi.FrostDkgResult{}
			resultHash = [32]byte{}
			err = fmt.Errorf("cannot decode DKG submission result")
		}
	}()
	if log == nil || evidence == nil || len(log.Topics) != 3 || len(log.Data) == 0 {
		return result, resultHash, fmt.Errorf("DKG submission log is malformed")
	}
	resultHash = [32]byte(log.Topics[1])
	if resultHash == [32]byte{} || crypto.Keccak256Hash(log.Data) != common.Hash(resultHash) {
		return result, resultHash, fmt.Errorf("DKG result hash does not commit to the submitted result")
	}
	values, unpackErr := evidence.registryABI.Events["DkgResultSubmitted"].Inputs.NonIndexed().Unpack(log.Data)
	if unpackErr != nil || len(values) != 1 {
		return result, resultHash, fmt.Errorf("cannot decode DKG submitted result: [%w]", unpackErr)
	}
	converted := ethabi.ConvertType(values[0], new(frostabi.FrostDkgResult))
	decoded, ok := converted.(*frostabi.FrostDkgResult)
	if !ok || decoded == nil {
		return result, resultHash, fmt.Errorf("cannot convert DKG submitted result")
	}
	return *decoded, resultHash, nil
}

func (source *signedFrostRetainedGroupHistorySource) authenticatedEventLog(
	ctx context.Context,
	point FrostRetainedGroupEventPoint,
	pin frostRetainedGroupContractPin,
	topic common.Hash,
	receipts frostRetainedGroupReceiptCache,
	code frostRetainedGroupCodeCache,
) (*types.Log, error) {
	if !point.valid() || pin.address == (common.Address{}) || pin.codeHash == (common.Hash{}) ||
		topic == (common.Hash{}) {
		return nil, fmt.Errorf("event evidence descriptor is incomplete")
	}
	if err := source.authenticateContractCode(ctx, pin, point.BlockNumber, code); err != nil {
		return nil, err
	}
	transactionHash := common.Hash(point.TransactionHash)
	receipt, ok := receipts[transactionHash]
	if !ok {
		var err error
		receipt, err = source.verifier.TransactionReceipt(ctx, transactionHash)
		if err != nil {
			return nil, fmt.Errorf("cannot read transaction receipt [%s]: [%w]", transactionHash.Hex(), err)
		}
		if receipt == nil {
			return nil, fmt.Errorf("transaction receipt [%s] is missing", transactionHash.Hex())
		}
		receipts[transactionHash] = receipt
	}
	if receipt.Status != types.ReceiptStatusSuccessful || receipt.BlockNumber == nil ||
		!receipt.BlockNumber.IsUint64() || receipt.BlockNumber.Uint64() != point.BlockNumber ||
		receipt.BlockHash != common.Hash(point.BlockHash) || receipt.TxHash != transactionHash ||
		receipt.TransactionIndex != uint(point.TransactionIndex) {
		return nil, fmt.Errorf("transaction receipt does not match the exported event point")
	}
	var matched *types.Log
	for _, candidate := range receipt.Logs {
		if candidate == nil || candidate.Index != uint(point.LogIndex) {
			continue
		}
		if matched != nil {
			return nil, fmt.Errorf("transaction receipt contains duplicate global log index")
		}
		matched = candidate
	}
	if matched == nil || matched.Removed || matched.Address != pin.address ||
		matched.BlockNumber != point.BlockNumber || matched.BlockHash != common.Hash(point.BlockHash) ||
		matched.TxHash != transactionHash || matched.TxIndex != uint(point.TransactionIndex) ||
		len(matched.Topics) == 0 || matched.Topics[0] != topic {
		return nil, fmt.Errorf("receipt log does not match the exact contract/event point")
	}
	return matched, nil
}

func (source *signedFrostRetainedGroupHistorySource) authenticateContractCode(
	ctx context.Context,
	pin frostRetainedGroupContractPin,
	blockNumber uint64,
	cache frostRetainedGroupCodeCache,
) error {
	key := frostRetainedGroupCodeCacheKey{address: pin.address, blockNumber: blockNumber}
	if _, ok := cache[key]; ok {
		return nil
	}
	code, err := source.verifier.CodeAt(ctx, pin.address, new(big.Int).SetUint64(blockNumber))
	if err != nil {
		return fmt.Errorf("cannot read pinned contract code at block [%d]: [%w]", blockNumber, err)
	}
	if len(code) == 0 || crypto.Keccak256Hash(code) != pin.codeHash {
		return fmt.Errorf("contract code at block [%d] differs from the signed activation manifest", blockNumber)
	}
	cache[key] = struct{}{}
	return nil
}

func (source *signedFrostRetainedGroupHistorySource) resolveOperatorIDAt(
	ctx context.Context,
	operator common.Address,
	at FrostPreSignFinality,
	evidence *frostRetainedGroupEvidenceProfile,
) (uint32, error) {
	if evidence == nil || operator == (common.Address{}) {
		return 0, fmt.Errorf("operator-resolution evidence is incomplete")
	}
	if err := source.authenticateContractCode(
		ctx,
		evidence.sortitionPool,
		at.BlockNumber,
		make(frostRetainedGroupCodeCache),
	); err != nil {
		return 0, err
	}
	// getOperatorID(address), pinned explicitly rather than learned from an
	// exporter or a mutable ABI service.
	callData := make([]byte, 4+32)
	copy(callData[:4], []byte{0x5a, 0x48, 0xb4, 0x6b})
	copy(callData[4+12:], operator[:])
	to := evidence.sortitionPool.address
	output, err := source.verifier.CallContract(
		ctx,
		ethereum.CallMsg{To: &to, Data: callData},
		new(big.Int).SetUint64(at.BlockNumber),
	)
	if err != nil {
		return 0, err
	}
	if len(output) != 32 || !bytes.Equal(output[:28], make([]byte, 28)) {
		return 0, fmt.Errorf("sortition-pool getOperatorID returned noncanonical data")
	}
	operatorID := binary.BigEndian.Uint32(output[28:])
	if operatorID == 0 {
		return 0, fmt.Errorf("operator is not registered in the pinned sortition pool at the requested block")
	}
	return operatorID, nil
}

func frostRetainedGroupEqualOperatorIDs(left []uint32, right []uint32) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func frostRetainedGroupBytes20Topic(value [20]byte) common.Hash {
	var result common.Hash
	copy(result[:20], value[:])
	return result
}

func frostRetainedGroupCanonicalAddressTopic(topic common.Hash) bool {
	return bytes.Equal(topic[:12], make([]byte, 12))
}
