package test

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/keep-network/keep-core/pkg/tbtcpg"
	"math/big"
	"time"

	"github.com/keep-network/keep-core/internal/hexutils"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// UnmarshalJSON implements a custom JSON unmarshaling logic to produce a
// proper FindDepositsToSweepTestScenario.
func (dsts *FindDepositsToSweepTestScenario) UnmarshalJSON(data []byte) error {
	type findDepositsToSweepTestScenario struct {
		Title           string
		ChainParameters struct {
			AverageBlockTime int64
			CurrentBlock     uint64
			DepositMinAge    uint32
		}
		MaxNumberOfDeposits uint16
		WalletPublicKeyHash string
		Deposits            []struct {
			FundingTxHash          string
			FundingOutputIndex     uint32
			FundingTxConfirmations uint
			FundingTxHex           string
			WalletPublicKeyHash    string
			Age                    int64
			SweptAt                int64
		}
		ExpectedUnsweptDeposits []struct {
			FundingTxHash      string
			FundingOutputIndex uint32
			RevealBlockNumber  uint64
		}
	}

	bytesFromHex := func(str string) []byte {
		value, err := hex.DecodeString(str)
		if err != nil {
			panic(err)
		}

		return value
	}

	txFromHex := func(str string) *bitcoin.Transaction {
		transaction := new(bitcoin.Transaction)
		err := transaction.Deserialize(bytesFromHex(str))
		if err != nil {
			panic(err)
		}

		return transaction
	}

	var unmarshaled findDepositsToSweepTestScenario

	err := json.Unmarshal(data, &unmarshaled)
	if err != nil {
		return err
	}

	// Unmarshal title.
	dsts.Title = unmarshaled.Title

	dsts.ChainParameters.AverageBlockTime =
		time.Duration(unmarshaled.ChainParameters.AverageBlockTime) * time.Second
	dsts.ChainParameters.CurrentBlock = unmarshaled.ChainParameters.CurrentBlock
	dsts.ChainParameters.DepositMinAge = unmarshaled.ChainParameters.DepositMinAge

	dsts.MaxNumberOfDeposits = unmarshaled.MaxNumberOfDeposits

	// Unmarshal wallet PKH.
	if len(unmarshaled.WalletPublicKeyHash) > 0 {
		copy(dsts.WalletPublicKeyHash[:], hexToSlice(unmarshaled.WalletPublicKeyHash))
	}

	now := time.Now()
	currentBlock := dsts.ChainParameters.CurrentBlock
	averageBlockTime := dsts.ChainParameters.AverageBlockTime

	// Unmarshal deposits.
	for i, deposit := range unmarshaled.Deposits {
		d := new(Deposit)

		age := time.Duration(deposit.Age) * time.Second
		ageBlocks := uint64(age.Milliseconds() / averageBlockTime.Milliseconds())

		revealedAt := now.Add(-age)
		revealBlockNumber := currentBlock - ageBlocks

		fundingTxHash, err := bitcoin.NewHashFromString(deposit.FundingTxHash, bitcoin.ReversedByteOrder)
		if err != nil {
			return fmt.Errorf(
				"failed to unmarshal funding transaction hash for deposit [%d/%d]: [%w]",
				i,
				len(unmarshaled.Deposits),
				err,
			)
		}

		copy(d.WalletPublicKeyHash[:], hexToSlice(deposit.WalletPublicKeyHash))

		d.FundingTxHash = fundingTxHash
		d.FundingOutputIndex = deposit.FundingOutputIndex
		d.FundingTxConfirmations = deposit.FundingTxConfirmations
		d.FundingTx = txFromHex(deposit.FundingTxHex)
		d.RevealBlockNumber = revealBlockNumber
		d.RevealedAt = revealedAt
		d.SweptAt = time.Unix(deposit.SweptAt, 0)

		dsts.Deposits = append(dsts.Deposits, d)
	}

	// Unmarshal expected unswept deposits.
	for i, deposit := range unmarshaled.ExpectedUnsweptDeposits {
		ud := new(tbtcpg.DepositReference)

		fundingTxHash, err := bitcoin.NewHashFromString(deposit.FundingTxHash, bitcoin.ReversedByteOrder)
		if err != nil {
			return fmt.Errorf(
				"failed to unmarshal funding transaction hash for expected unswept deposit [%d/%d]: [%w]",
				i,
				len(unmarshaled.Deposits),
				err,
			)
		}

		ud.FundingTxHash = fundingTxHash
		ud.FundingOutputIndex = deposit.FundingOutputIndex
		ud.RevealBlock = deposit.RevealBlockNumber

		dsts.ExpectedUnsweptDeposits = append(dsts.ExpectedUnsweptDeposits, ud)
	}

	return nil
}

type depositSweepProposal struct {
	WalletPublicKeyHash string
	DepositsKeys        []struct {
		FundingTxHash      string
		FundingOutputIndex uint32
	}
	SweepTxFee           int64
	DepositsRevealBlocks []int64
}

func (dsp *depositSweepProposal) convert() (
	[20]byte,
	*tbtc.DepositSweepProposal,
	error,
) {
	if dsp == nil {
		return [20]byte{}, nil, nil
	}

	result := &tbtc.DepositSweepProposal{}

	var walletPublicKeyHash [20]byte
	if len(dsp.WalletPublicKeyHash) > 0 {
		copy(walletPublicKeyHash[:], hexToSlice(dsp.WalletPublicKeyHash))
	}

	result.DepositsKeys = make([]struct {
		FundingTxHash      bitcoin.Hash
		FundingOutputIndex uint32
	}, len(dsp.DepositsKeys))
	for i, depositKey := range dsp.DepositsKeys {
		fundingTxHash, err := bitcoin.NewHashFromString(depositKey.FundingTxHash, bitcoin.ReversedByteOrder)
		if err != nil {
			return [20]byte{}, nil, fmt.Errorf(
				"failed to unmarshal funding transaction hash for deposit [%d/%d]: [%w]",
				i,
				len(dsp.DepositsKeys),
				err,
			)
		}
		result.DepositsKeys[i].FundingTxHash = fundingTxHash
		result.DepositsKeys[i].FundingOutputIndex = depositKey.FundingOutputIndex
	}

	result.DepositsRevealBlocks = make([]*big.Int, len(dsp.DepositsRevealBlocks))
	for i, depositRevealBlock := range dsp.DepositsRevealBlocks {
		result.DepositsRevealBlocks[i] = big.NewInt(depositRevealBlock)
	}

	result.SweepTxFee = big.NewInt(dsp.SweepTxFee)

	return walletPublicKeyHash, result, nil
}

// UnmarshalJSON implements a custom JSON unmarshaling logic to produce a
// proper ProposeSweepTestScenario.
func (psts *ProposeSweepTestScenario) UnmarshalJSON(data []byte) error {
	type proposeSweepTestScenario struct {
		Title               string
		WalletPublicKeyHash string
		DepositTxMaxFee     uint64
		Deposits            []struct {
			FundingTxHash          string
			FundingOutputIndex     uint32
			RevealBlock            uint64
			FundingTxConfirmations uint
		}
		SweepTxFee                   int64
		EstimateSatPerVByteFee       int64
		ExpectedDepositSweepProposal *depositSweepProposal
		ExpectedErr                  string
	}

	var unmarshaled proposeSweepTestScenario

	err := json.Unmarshal(data, &unmarshaled)
	if err != nil {
		return err
	}

	// Unmarshal title.
	psts.Title = unmarshaled.Title

	// Unmarshal wallet public key hash.
	if len(unmarshaled.WalletPublicKeyHash) > 0 {
		copy(psts.WalletPublicKeyHash[:], hexToSlice(unmarshaled.WalletPublicKeyHash))
	}

	// Unmarshal deposit transaction max fee.
	psts.DepositTxMaxFee = unmarshaled.DepositTxMaxFee

	// Unmarshal deposits.
	for i, deposit := range unmarshaled.Deposits {
		d := new(ProposeSweepDepositsData)

		fundingTxHash, err := bitcoin.NewHashFromString(deposit.FundingTxHash, bitcoin.ReversedByteOrder)
		if err != nil {
			return fmt.Errorf(
				"failed to unmarshal funding transaction hash for deposit [%d/%d]: [%w]",
				i,
				len(unmarshaled.Deposits),
				err,
			)
		}

		d.FundingTxHash = fundingTxHash
		d.FundingOutputIndex = deposit.FundingOutputIndex
		d.RevealBlock = deposit.RevealBlock
		d.FundingTxConfirmations = deposit.FundingTxConfirmations

		psts.Deposits = append(psts.Deposits, d)
	}

	// Unmarshal sweep transaction fee.
	psts.SweepTxFee = unmarshaled.SweepTxFee

	// Unmarshal estimate sat per vbyte fee.
	psts.EstimateSatPerVByteFee = unmarshaled.EstimateSatPerVByteFee

	// Unmarshal deposit sweep proposal
	_, psts.ExpectedDepositSweepProposal, err = unmarshaled.ExpectedDepositSweepProposal.convert()
	if err != nil {
		return fmt.Errorf(
			"failed to unmarshal expected deposit sweep proposal: [%w]",
			err,
		)
	}

	// Unmarshal expected error
	if len(unmarshaled.ExpectedErr) > 0 {
		psts.ExpectedErr = errors.New(unmarshaled.ExpectedErr)
	}

	return nil
}

// UnmarshalJSON implements a custom JSON unmarshaling logic to produce a
// proper FindPendingRedemptionsTestScenario.
func (fprts *FindPendingRedemptionsTestScenario) UnmarshalJSON(data []byte) error {
	type findPendingRedemptionsTestScenario struct {
		Title           string
		ChainParameters struct {
			AverageBlockTime int64
			CurrentBlock     uint64
			RequestTimeout   uint32
			RequestMinAge    uint32
		}
		WalletPublicKeyHash string
		MaxNumberOfRequests uint16
		PendingRedemptions  []struct {
			WalletPublicKeyHash  string
			RedeemerOutputScript string
			RequestedAmount      uint64
			Age                  int64
			Delay                int64
		}
		ExpectedRedeemersOutputScripts []string
	}

	var unmarshaled findPendingRedemptionsTestScenario

	err := json.Unmarshal(data, &unmarshaled)
	if err != nil {
		return err
	}

	fprts.Title = unmarshaled.Title

	fprts.ChainParameters.AverageBlockTime =
		time.Duration(unmarshaled.ChainParameters.AverageBlockTime) * time.Second
	fprts.ChainParameters.CurrentBlock = unmarshaled.ChainParameters.CurrentBlock
	fprts.ChainParameters.RequestTimeout = unmarshaled.ChainParameters.RequestTimeout
	fprts.ChainParameters.RequestMinAge = unmarshaled.ChainParameters.RequestMinAge

	// Unmarshal wallet PKH.
	if len(unmarshaled.WalletPublicKeyHash) > 0 {
		copy(fprts.WalletPublicKeyHash[:], hexToSlice(unmarshaled.WalletPublicKeyHash))
	}

	fprts.MaxNumberOfRequests = unmarshaled.MaxNumberOfRequests

	now := time.Now()
	currentBlock := fprts.ChainParameters.CurrentBlock
	averageBlockTime := fprts.ChainParameters.AverageBlockTime

	for _, pr := range unmarshaled.PendingRedemptions {
		var wpkh [20]byte
		copy(wpkh[:], hexToSlice(pr.WalletPublicKeyHash))

		age := time.Duration(pr.Age) * time.Second
		ageBlocks := uint64(age.Milliseconds() / averageBlockTime.Milliseconds())

		requestedAt := now.Add(-age)
		requestBlock := currentBlock - ageBlocks

		fprts.PendingRedemptions = append(
			fprts.PendingRedemptions,
			&RedemptionRequest{
				WalletPublicKeyHash:  wpkh,
				RedeemerOutputScript: hexToSlice(pr.RedeemerOutputScript),
				RequestedAmount:      pr.RequestedAmount,
				RequestedAt:          requestedAt,
				RequestBlock:         requestBlock,
				Delay:                time.Duration(pr.Delay) * time.Second,
			},
		)
	}

	fprts.ExpectedRedeemersOutputScripts = make([]bitcoin.Script, 0)
	for _, s := range unmarshaled.ExpectedRedeemersOutputScripts {
		fprts.ExpectedRedeemersOutputScripts = append(
			fprts.ExpectedRedeemersOutputScripts,
			hexToSlice(s),
		)
	}

	return nil
}

func hexToSlice(hexString string) []byte {
	if len(hexString) == 0 {
		return []byte{}
	}

	bytes, err := hexutils.Decode(hexString)
	if err != nil {
		panic(err)
	}

	return bytes
}

// UnmarshalJSON implements a custom JSON unmarshaling logic to produce a
// proper ReservationReanchorTestScenario.
func (rrts *ReservationReanchorTestScenario) UnmarshalJSON(data []byte) error {
	type reservationDataJSON struct {
		ReservationKey      string
		WalletPublicKeyHash string
		AnchorTxHash        string
		AnchorTxOutputIndex uint32
		AnchorValue         int64
		State               string
		RequestNonce        uint64
		HasPendingAction    bool
		PendingActionState  string
	}
	type reservationReanchorTestScenarioJSON struct {
		Title string

		SourceWalletPublicKeyHash   string
		SourceWalletState           string
		SourceWalletMainUtxoHash    string
		SourceWalletMainUtxoValue   int64
		SourceWalletMainUtxoTxHash  string
		SourceWalletMainUtxoTxIndex uint32

		TargetWalletPublicKeyHash string

		LiveWalletsCount uint32

		MovingFundsDustThreshold uint64
		ReservationTxMaxFee      uint64
		EstimateSatPerVByteFee   int64
		ReanchorTxFee            int64

		Reservations []reservationDataJSON

		ExpectedProposal *reservationReanchorProposalJSON
		ExpectedErr      string
	}

	var unmarshaled reservationReanchorTestScenarioJSON

	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		return err
	}

	rrts.Title = unmarshaled.Title

	if len(unmarshaled.SourceWalletPublicKeyHash) > 0 {
		copy(rrts.SourceWalletPublicKeyHash[:], hexToSlice(unmarshaled.SourceWalletPublicKeyHash))
	}
	rrts.SourceWalletState = parseWalletState(unmarshaled.SourceWalletState)
	if len(unmarshaled.SourceWalletMainUtxoHash) > 0 {
		copy(rrts.SourceWalletMainUtxoHashBytes[:], hexToSlice(unmarshaled.SourceWalletMainUtxoHash))
	} else {
		rrts.SourceWalletMainUtxoHashBytes = [32]byte{}
	}
	rrts.SourceWalletMainUtxoValue = unmarshaled.SourceWalletMainUtxoValue
	rrts.SourceWalletMainUtxoTxHash = unmarshaled.SourceWalletMainUtxoTxHash
	rrts.SourceWalletMainUtxoTxIndex = unmarshaled.SourceWalletMainUtxoTxIndex

	if len(unmarshaled.TargetWalletPublicKeyHash) > 0 {
		copy(rrts.TargetWalletPublicKeyHash[:], hexToSlice(unmarshaled.TargetWalletPublicKeyHash))
	}

	rrts.LiveWalletsCount = unmarshaled.LiveWalletsCount

	rrts.MovingFundsDustThreshold = unmarshaled.MovingFundsDustThreshold
	rrts.ReservationTxMaxFee = unmarshaled.ReservationTxMaxFee
	rrts.EstimateSatPerVByteFee = unmarshaled.EstimateSatPerVByteFee
	rrts.ReanchorTxFee = unmarshaled.ReanchorTxFee

	rrts.Reservations = make([]*ReservationReanchorData, 0, len(unmarshaled.Reservations))
	for _, r := range unmarshaled.Reservations {
		d := &ReservationReanchorData{}

		if len(r.ReservationKey) > 0 {
			keyBytes := hexToSlice(r.ReservationKey)
			d.ReservationKey = new(big.Int).SetBytes(keyBytes)
		}
		if len(r.WalletPublicKeyHash) > 0 {
			copy(d.WalletPublicKeyHash[:], hexToSlice(r.WalletPublicKeyHash))
		}
		d.AnchorTxHash = r.AnchorTxHash
		d.AnchorTxOutputIndex = r.AnchorTxOutputIndex
		d.AnchorValue = r.AnchorValue
		d.State = parseReservationState(r.State)
		d.RequestNonce = r.RequestNonce
		d.HasPendingAction = r.HasPendingAction
		d.PendingActionState = parseReservationActionState(r.PendingActionState)

		rrts.Reservations = append(rrts.Reservations, d)
	}

	if unmarshaled.ExpectedProposal != nil {
		prop, err := unmarshaled.ExpectedProposal.convert()
		if err != nil {
			return fmt.Errorf(
				"failed to convert expected reservation re-anchor proposal: [%w]",
				err,
			)
		}
		rrts.ExpectedProposal = prop
	}

	if len(unmarshaled.ExpectedErr) > 0 {
		rrts.ExpectedErr = errors.New(unmarshaled.ExpectedErr)
	}

	return nil
}

type reservationReanchorProposalJSON struct {
	ReservationKey            string
	RequestNonce              uint64
	TargetWalletPublicKeyHash string
	ReanchorTxFee             int64
}

func (rj *reservationReanchorProposalJSON) convert() (*tbtc.ReservationReanchorProposal, error) {
	if rj == nil {
		return nil, nil
	}

	result := &tbtc.ReservationReanchorProposal{
		RequestNonce:  rj.RequestNonce,
		ReanchorTxFee: big.NewInt(rj.ReanchorTxFee),
	}
	if len(rj.ReservationKey) > 0 {
		result.ReservationKey = new(big.Int).SetBytes(hexToSlice(rj.ReservationKey))
	}
	if len(rj.TargetWalletPublicKeyHash) > 0 {
		copy(result.TargetWalletPublicKeyHash[:], hexToSlice(rj.TargetWalletPublicKeyHash))
	}
	return result, nil
}

func parseWalletState(s string) tbtc.WalletState {
	switch s {
	case "Live":
		return tbtc.StateLive
	case "MovingFunds":
		return tbtc.StateMovingFunds
	case "Closing":
		return tbtc.StateClosing
	case "Closed":
		return tbtc.StateClosed
	case "Terminated":
		return tbtc.StateTerminated
	default:
		return tbtc.StateUnknown
	}
}

func parseReservationState(s string) tbtc.ReservationState {
	switch s {
	case "Active":
		return tbtc.ReservationStateActive
	case "ActionPending":
		return tbtc.ReservationStateActionPending
	case "Closed":
		return tbtc.ReservationStateClosed
	case "Stranded":
		return tbtc.ReservationStateStranded
	default:
		return tbtc.ReservationStateUnknown
	}
}

func parseReservationActionState(s string) tbtc.ReservationActionState {
	switch s {
	case "Pending":
		return tbtc.ReservationActionStatePending
	case "Settled":
		return tbtc.ReservationActionStateSettled
	case "TimedOut":
		return tbtc.ReservationActionStateTimedOut
	case "Vetoed":
		return tbtc.ReservationActionStateVetoed
	case "Superseded":
		return tbtc.ReservationActionStateSuperseded
	default:
		return tbtc.ReservationActionStateUnknown
	}
}
