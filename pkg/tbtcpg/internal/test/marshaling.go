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
		// fmt.Errorf requires a constant format string; ExpectedErr is a
		// plain string so use errors.New to avoid formatting interpretation.
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
