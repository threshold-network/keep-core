package tbtc

import (
	"crypto/ecdsa"
	"fmt"
	"math"
	"math/big"

	"github.com/keep-network/keep-core/pkg/bitcoin"

	"google.golang.org/protobuf/proto"

	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/crypto/secp256k1"
	"github.com/keep-network/keep-core/pkg/protocol/group"
	"github.com/keep-network/keep-core/pkg/tbtc/gen/pb"
	"github.com/keep-network/keep-core/pkg/tecdsa"
)

var errIncompatiblePublicKey = fmt.Errorf(
	"public key is not tECDSA compatible and will cause unmarshaling error",
)

// Marshal converts the signer to a byte array.
func (s *signer) Marshal() ([]byte, error) {
	walletPublicKey, err := marshalPublicKey(s.wallet.publicKey)
	if err != nil {
		return nil, err
	}

	walletSigningGroupOperators := make(
		[]string,
		len(s.wallet.signingGroupOperators),
	)
	for i := range walletSigningGroupOperators {
		walletSigningGroupOperators[i] =
			s.wallet.signingGroupOperators[i].String()
	}

	pbWallet := &pb.Wallet{
		PublicKey:             walletPublicKey,
		SigningGroupOperators: walletSigningGroupOperators,
	}

	privateKeyShare, err := s.privateKeyShare.Marshal()
	if err != nil {
		return nil, fmt.Errorf("cannot marshal private key share: [%w]", err)
	}

	return proto.Marshal(&pb.Signer{
		Wallet:                  pbWallet,
		SigningGroupMemberIndex: uint32(s.signingGroupMemberIndex),
		PrivateKeyShare:         privateKeyShare,
	})
}

// Unmarshal converts a byte array back to the signer.
func (s *signer) Unmarshal(bytes []byte) error {
	pbSigner := pb.Signer{}
	if err := proto.Unmarshal(bytes, &pbSigner); err != nil {
		return fmt.Errorf("cannot unmarshal signer: [%w]", err)
	}

	walletPublicKey, err := unmarshalPublicKey(pbSigner.Wallet.PublicKey)
	if err != nil {
		return fmt.Errorf("cannot unmarshal wallet public key: [%w]", err)
	}

	walletSigningGroupOperators := make(
		[]chain.Address,
		len(pbSigner.Wallet.SigningGroupOperators),
	)
	for i := range walletSigningGroupOperators {
		walletSigningGroupOperators[i] =
			chain.Address(pbSigner.Wallet.SigningGroupOperators[i])
	}

	privateKeyShare := &tecdsa.PrivateKeyShare{}
	if err := privateKeyShare.Unmarshal(pbSigner.PrivateKeyShare); err != nil {
		return fmt.Errorf("cannot unmarshal private key share: [%w]", err)
	}

	s.wallet = wallet{
		publicKey:             walletPublicKey,
		signingGroupOperators: walletSigningGroupOperators,
	}
	s.signingGroupMemberIndex = group.MemberIndex(pbSigner.SigningGroupMemberIndex)
	s.privateKeyShare = privateKeyShare

	return nil
}

// Marshal converts the signingDoneMessage to a byte array.
func (sdm *signingDoneMessage) Marshal() ([]byte, error) {
	signatureBytes, err := sdm.signature.Marshal()
	if err != nil {
		return nil, err
	}

	return proto.Marshal(&pb.SigningDoneMessage{
		SenderID:      uint32(sdm.senderID),
		Message:       sdm.message.Bytes(),
		AttemptNumber: sdm.attemptNumber,
		Signature:     signatureBytes,
		EndBlock:      sdm.endBlock,
	})
}

// Unmarshal converts a byte array back to the signingDoneMessage.
func (sdm *signingDoneMessage) Unmarshal(bytes []byte) error {
	pbMsg := pb.SigningDoneMessage{}
	if err := proto.Unmarshal(bytes, &pbMsg); err != nil {
		return fmt.Errorf("failed to unmarshal SigningDoneMessage: [%v]", err)
	}

	if err := validateMemberIndex(pbMsg.SenderID); err != nil {
		return err
	}

	signature := &tecdsa.Signature{}
	if err := signature.Unmarshal(pbMsg.Signature); err != nil {
		return fmt.Errorf("cannot unmarshal signature: [%v]", err)
	}

	sdm.senderID = group.MemberIndex(pbMsg.SenderID)
	sdm.message = new(big.Int).SetBytes(pbMsg.Message)
	sdm.attemptNumber = pbMsg.AttemptNumber
	sdm.signature = signature
	sdm.endBlock = pbMsg.EndBlock

	return nil
}

// Marshal converts the coordinationMessage to a byte array.
func (cm *coordinationMessage) Marshal() ([]byte, error) {
	proposalBytes, err := cm.proposal.Marshal()
	if err != nil {
		return nil, err
	}

	pbProposal := &pb.CoordinationProposal{
		ActionType: uint32(cm.proposal.ActionType()),
		Payload:    proposalBytes,
	}

	return proto.Marshal(
		&pb.CoordinationMessage{
			SenderID:            uint32(cm.senderID),
			CoordinationBlock:   cm.coordinationBlock,
			WalletPublicKeyHash: append([]byte{}, cm.walletPublicKeyHash[:]...),
			Proposal:            pbProposal,
		},
	)
}

// Unmarshal converts a byte array back to the coordinationMessage.
func (cm *coordinationMessage) Unmarshal(bytes []byte) error {
	pbMsg := pb.CoordinationMessage{}
	if err := proto.Unmarshal(bytes, &pbMsg); err != nil {
		return fmt.Errorf("failed to unmarshal CoordinationMessage: [%v]", err)
	}

	if err := validateMemberIndex(pbMsg.SenderID); err != nil {
		return err
	}

	walletPublicKeyHash, err := unmarshalWalletPublicKeyHash(pbMsg.WalletPublicKeyHash)
	if err != nil {
		return fmt.Errorf(
			"failed to unmarshal wallet public key hash: [%v]",
			err,
		)
	}

	if pbMsg.Proposal == nil {
		return fmt.Errorf("missing proposal")
	}
	proposal, err := unmarshalCoordinationProposal(
		pbMsg.Proposal.ActionType,
		pbMsg.Proposal.Payload,
	)
	if err != nil {
		return fmt.Errorf("failed to unmarshal proposal: [%v]", err)
	}

	cm.senderID = group.MemberIndex(pbMsg.SenderID)
	cm.coordinationBlock = pbMsg.CoordinationBlock
	cm.walletPublicKeyHash = walletPublicKeyHash
	cm.proposal = proposal

	return nil
}

// unmarshalWalletPublicKeyHash converts a byte array to a wallet public key
// hash.
func unmarshalWalletPublicKeyHash(bytes []byte) ([20]byte, error) {
	if len(bytes) != 20 {
		return [20]byte{}, fmt.Errorf(
			"invalid wallet public key hash length: [%v]",
			len(bytes),
		)
	}

	var walletPublicKeyHash [20]byte
	copy(walletPublicKeyHash[:], bytes)

	return walletPublicKeyHash, nil
}

// unmarshalCoordinationProposal converts a byte array back to the coordination
// proposal.
func unmarshalCoordinationProposal(actionType uint32, payload []byte) (
	CoordinationProposal,
	error,
) {
	if actionType > math.MaxUint8 {
		return nil, fmt.Errorf(
			"invalid proposal action type value: [%v]",
			actionType,
		)
	}

	parsedActionType, err := ParseWalletActionType(uint8(actionType))
	if err != nil {
		return nil, fmt.Errorf(
			"failed to parse proposal action type: [%v]",
			err,
		)
	}

	proposal, ok := map[WalletActionType]CoordinationProposal{
		ActionNoop:                   &NoopProposal{},
		ActionHeartbeat:              &HeartbeatProposal{},
		ActionDepositSweep:           &DepositSweepProposal{},
		ActionRedemption:             &RedemptionProposal{},
		ActionMovingFunds:            &MovingFundsProposal{},
		ActionMovedFundsSweep:        &MovedFundsSweepProposal{},
		ActionReservationAnchor:      &ReservationAnchorProposal{},
		ActionReservedRedemption:     &ReservedRedemptionProposal{},
		ActionReservationReanchor:    &ReservationReanchorProposal{},
		ActionReservationDissolution: &ReservationDissolutionProposal{},
	}[parsedActionType]
	if !ok {
		return nil, fmt.Errorf(
			"no unmarshaler for proposal action type: [%v]",
			parsedActionType,
		)
	}

	if err := proposal.Unmarshal(payload); err != nil {
		return nil, fmt.Errorf("cannot unmarshal proposal payload: [%v]", err)
	}

	return proposal, nil
}

// Marshal converts the noopProposal to a byte array.
func (np *NoopProposal) Marshal() ([]byte, error) {
	return []byte{}, nil
}

// Unmarshal converts a byte array back to the noopProposal.
func (np *NoopProposal) Unmarshal([]byte) error {
	return nil
}

// Marshal converts the heartbeatProposal to a byte array.
func (hp *HeartbeatProposal) Marshal() ([]byte, error) {
	return proto.Marshal(
		&pb.HeartbeatProposal{
			Message: hp.Message[:],
		},
	)
}

// Unmarshal converts a byte array back to the heartbeatProposal.
func (hp *HeartbeatProposal) Unmarshal(bytes []byte) error {
	pbMsg := pb.HeartbeatProposal{}
	if err := proto.Unmarshal(bytes, &pbMsg); err != nil {
		return fmt.Errorf("failed to unmarshal HeartbeatProposal: [%v]", err)
	}

	if len(pbMsg.Message) != 16 {
		return fmt.Errorf(
			"invalid heartbeat message length: [%v]",
			len(pbMsg.Message),
		)
	}

	var message [16]byte
	copy(message[:], pbMsg.Message)

	hp.Message = message

	return nil
}

// Marshal converts the depositSweepProposal to a byte array.
func (dsp *DepositSweepProposal) Marshal() ([]byte, error) {
	depositsKeys := make(
		[]*pb.DepositSweepProposal_DepositKey,
		len(dsp.DepositsKeys),
	)
	for i, depositKey := range dsp.DepositsKeys {
		depositsKeys[i] = &pb.DepositSweepProposal_DepositKey{
			FundingTxHash:      append([]byte{}, depositKey.FundingTxHash[:]...),
			FundingOutputIndex: depositKey.FundingOutputIndex,
		}
	}

	depositsRevealBlocks := make([]uint64, len(dsp.DepositsRevealBlocks))
	for i, block := range dsp.DepositsRevealBlocks {
		depositsRevealBlocks[i] = block.Uint64()
	}

	return proto.Marshal(
		&pb.DepositSweepProposal{
			DepositsKeys:         depositsKeys,
			SweepTxFee:           dsp.SweepTxFee.Bytes(),
			DepositsRevealBlocks: depositsRevealBlocks,
		},
	)
}

// Unmarshal converts a byte array back to the depositSweepProposal.
func (dsp *DepositSweepProposal) Unmarshal(bytes []byte) error {
	pbMsg := pb.DepositSweepProposal{}
	if err := proto.Unmarshal(bytes, &pbMsg); err != nil {
		return fmt.Errorf("failed to unmarshal DepositSweepProposal: [%v]", err)
	}

	depositsKeys := make(
		[]struct {
			FundingTxHash      bitcoin.Hash
			FundingOutputIndex uint32
		},
		len(pbMsg.DepositsKeys),
	)
	for i, depositKey := range pbMsg.DepositsKeys {
		hash, err := bitcoin.NewHash(
			depositKey.FundingTxHash,
			bitcoin.InternalByteOrder,
		)
		if err != nil {
			return fmt.Errorf(
				"failed to unmarshal funding tx hash: [%v]",
				err,
			)
		}

		depositsKeys[i] = struct {
			FundingTxHash      bitcoin.Hash
			FundingOutputIndex uint32
		}{
			FundingTxHash:      hash,
			FundingOutputIndex: depositKey.FundingOutputIndex,
		}
	}

	depositsRevealBlocks := make([]*big.Int, len(pbMsg.DepositsRevealBlocks))
	for i, block := range pbMsg.DepositsRevealBlocks {
		depositsRevealBlocks[i] = big.NewInt(int64(block))
	}

	dsp.DepositsKeys = depositsKeys
	dsp.SweepTxFee = new(big.Int).SetBytes(pbMsg.SweepTxFee)
	dsp.DepositsRevealBlocks = depositsRevealBlocks

	return nil
}

// Marshal converts the redemptionProposal to a byte array.
func (rp *RedemptionProposal) Marshal() ([]byte, error) {
	redeemersOutputScripts := make([][]byte, len(rp.RedeemersOutputScripts))
	for i, script := range rp.RedeemersOutputScripts {
		redeemersOutputScripts[i] = script
	}

	return proto.Marshal(
		&pb.RedemptionProposal{
			RedeemersOutputScripts: redeemersOutputScripts,
			RedemptionTxFee:        rp.RedemptionTxFee.Bytes(),
		},
	)
}

// Unmarshal converts a byte array back to the redemptionProposal.
func (rp *RedemptionProposal) Unmarshal(bytes []byte) error {
	pbMsg := pb.RedemptionProposal{}
	if err := proto.Unmarshal(bytes, &pbMsg); err != nil {
		return fmt.Errorf("failed to unmarshal RedemptionProposal: [%v]", err)
	}

	redeemersOutputScripts := make([]bitcoin.Script, len(pbMsg.RedeemersOutputScripts))
	for i, script := range pbMsg.RedeemersOutputScripts {
		redeemersOutputScripts[i] = script
	}

	rp.RedeemersOutputScripts = redeemersOutputScripts
	rp.RedemptionTxFee = new(big.Int).SetBytes(pbMsg.RedemptionTxFee)

	return nil
}

// Marshal converts the movingFundsProposal to a byte array.
func (mfp *MovingFundsProposal) Marshal() ([]byte, error) {
	targetWallets := make([][]byte, len(mfp.TargetWallets))

	for i, wallet := range mfp.TargetWallets {
		targetWallet := make([]byte, len(wallet))
		copy(targetWallet, wallet[:])

		targetWallets[i] = targetWallet
	}

	return proto.Marshal(
		&pb.MovingFundsProposal{
			TargetWallets:    targetWallets,
			MovingFundsTxFee: mfp.MovingFundsTxFee.Bytes(),
		})
}

// Unmarshal converts a byte array back to the movingFundsProposal.
func (mfp *MovingFundsProposal) Unmarshal(data []byte) error {
	pbMsg := pb.MovingFundsProposal{}
	if err := proto.Unmarshal(data, &pbMsg); err != nil {
		return fmt.Errorf("failed to unmarshal MovingFundsProposal: [%v]", err)
	}

	mfp.TargetWallets = make([][20]byte, len(pbMsg.TargetWallets))
	for i, wallet := range pbMsg.TargetWallets {
		if len(wallet) != 20 {
			return fmt.Errorf("invalid target wallet length: [%v]", len(wallet))
		}
		copy(mfp.TargetWallets[i][:], wallet)
	}

	mfp.MovingFundsTxFee = new(big.Int).SetBytes(pbMsg.MovingFundsTxFee)
	return nil
}

// Marshal converts the movingFundsProposal to a byte array.
func (mfsp *MovedFundsSweepProposal) Marshal() ([]byte, error) {
	return proto.Marshal(
		&pb.MovedFundsSweepProposal{
			MovingFundsTxHash:        mfsp.MovingFundsTxHash[:],
			MovingFundsTxOutputIndex: mfsp.MovingFundsTxOutputIndex,
			SweepTxFee:               mfsp.SweepTxFee.Bytes(),
		})
}

// Unmarshal converts a byte array back to the movingFundsProposal.
func (mfsp *MovedFundsSweepProposal) Unmarshal(data []byte) error {
	pbMsg := pb.MovedFundsSweepProposal{}
	if err := proto.Unmarshal(data, &pbMsg); err != nil {
		return fmt.Errorf("failed to unmarshal MovedFundsSweepProposal: [%v]", err)
	}

	if len(pbMsg.MovingFundsTxHash) != 32 {
		return fmt.Errorf(
			"invalid moving funds tx hash length: [%v]",
			len(pbMsg.MovingFundsTxHash),
		)
	}

	copy(mfsp.MovingFundsTxHash[:], pbMsg.MovingFundsTxHash)
	mfsp.MovingFundsTxOutputIndex = pbMsg.MovingFundsTxOutputIndex
	mfsp.SweepTxFee = new(big.Int).SetBytes(pbMsg.SweepTxFee)

	return nil
}

// marshalPublicKey converts an ECDSA public key to a byte
// array (uncompressed).
func marshalPublicKey(publicKey *ecdsa.PublicKey) ([]byte, error) {
	if publicKey.Curve.Params().Name != tecdsa.Curve.Params().Name {
		return nil, errIncompatiblePublicKey
	}

	return secp256k1.Marshal(publicKey), nil
}

// unmarshalPublicKey converts a byte array (uncompressed) to an ECDSA
// public key.
func unmarshalPublicKey(bytes []byte) (*ecdsa.PublicKey, error) {
	return secp256k1.Unmarshal(bytes)
}

func validateMemberIndex(protoIndex uint32) error {
	// Protobuf does not have uint8 type, so we are using uint32. When
	// unmarshalling message, we need to make sure we do not overflow.
	if protoIndex > group.MaxMemberIndex {
		return fmt.Errorf("invalid member index value: [%v]", protoIndex)
	}
	return nil
}
