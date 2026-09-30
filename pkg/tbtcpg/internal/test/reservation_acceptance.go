package test

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/keep-network/keep-core/internal/hexutils"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// reservationAcceptanceTestDataFilePrefix is the prefix shared by every
// reservation acceptance scenario file under testdata/. The loader walks
// the directory and matches files whose name starts with this prefix.
const reservationAcceptanceTestDataFilePrefix = "reservation_acceptance"

// ReservedDepositScenario holds a single reserved deposit's data in a
// reservation acceptance test scenario, including unexported parsed copies
// populated by UnmarshalJSON for use by Materialize.
type ReservedDepositScenario struct {
	FundingTxHash          string
	FundingOutputIndex     uint32
	FundingTxConfirmations uint
	FundingTxHex           string
	WalletPublicKeyHash    string
	Depositor              string
	BlindingFactor         string
	RefundPublicKeyHash    string
	RefundLocktime         string
	Amount                 uint64
	RevealBlock            uint64
	Age                    int64
	SweptAt                int64
	Vault                  string

	// PendingAcceptanceAction, when non-nil, instructs the test driver to
	// seed the deposit's current generation as a Pending Acceptance action
	// record targeting TargetWalletPublicKeyHash, together with the
	// ReservationAcceptanceRequested event the depositor's request emits.
	// The acceptance task only discovers deposits through that event and
	// only proposes against deposits whose current generation is exactly
	// that record (mirroring WalletProposalValidator.sol's
	// validateReservationAnchorProposal precondition), so every scenario
	// that should reach a signer-time rule must seed one here, with the
	// parameter snapshots as they stood at the depositor's request time.
	PendingAcceptanceAction *PendingAcceptanceActionScenario

	parsedFundingTxHash bitcoin.Hash
	parsedFundingTx     *bitcoin.Transaction
	parsedWalletPKH     [20]byte
	parsedPendingAction *tbtc.ReservationAction
}

// PendingAcceptanceAction returns the *tbtc.ReservationAction the driver
// should seed for the depositor's Pending Acceptance generation. It is
// built once on Materialize and cached on the scenario so the JSON
// fields are not re-parsed on every driver call.
func (rds *ReservedDepositScenario) PendingAcceptanceActionValue() *tbtc.ReservationAction {
	return rds.parsedPendingAction
}

// PendingAcceptanceActionScenario captures the snapshotted fields a
// depositor's requestReservationAcceptance call writes onto the action
// record the acceptance task consumes. The driver seeds the deposit's
// current reservation and the corresponding action record from this
// description.
type PendingAcceptanceActionScenario struct {
	RequestNonce              uint64
	TxMaxFee                  uint64
	MinAmount                 uint64
	TermSeconds               uint32
	TargetWalletPublicKeyHash string
	// TimeoutAt, when non-zero, sets the action record's snapshotted
	// timeout. Zero is treated as "unset" and the driver defaults to a
	// far-future value (now + 24 hours) so scenarios that do not intend
	// to exercise the timeout safety margin gate do not have to pin a
	// concrete epoch.
	TimeoutAt uint32
}

// ReservationAcceptanceTestScenario represents one test scenario for the
// reservation acceptance proposal builder. It captures the on-chain state
// (chain parameters, reserved deposits, cap snapshot, wallet state) and the
// expected outcome (no proposal or a specific anchor proposal).
type ReservationAcceptanceTestScenario struct {
	Title string

	ChainParameters struct {
		AverageBlockTime time.Duration
		CurrentBlock     uint64
		DepositMinAge    uint32
	}

	WalletPublicKeyHash [20]byte

	ReservationVault string

	WalletState string

	ReservationParameters struct {
		ReservationMinAmount      uint64
		ReservationTxMaxFee       uint64
		ReservationMaxTotalAmount uint64
		ReservationTotalAmount    uint64
		MaxReservationsPerWallet  uint32
	}

	Caps struct {
		MaxReservationsAmountPerWallet uint64
		ReservationMaxSingleAmount     uint64
	}

	WalletCustody struct {
		Count  uint32
		Amount uint64
	}

	Global struct {
		ActiveCount uint32
		MaxActive   uint32
	}

	PendingReservedDeposits uint64

	ReservedDeposits []*ReservedDepositScenario

	ExpectedAnchorProposal *tbtc.ReservationAnchorProposal
	ExpectedErr            error
}

// reservationAnchorProposalScenario is the JSON-friendly representation of
// the expected anchor proposal.
type reservationAnchorProposalScenario struct {
	DepositFundingTxHash      string
	DepositFundingOutputIndex uint32
	RequestNonce              uint64
	AnchorTxFee               int64
}

// convert builds a *tbtc.ReservationAnchorProposal from the scenario's
// JSON-friendly form. It returns nil when the scenario is nil.
func (ras *reservationAnchorProposalScenario) convert() *tbtc.ReservationAnchorProposal {
	if ras == nil {
		return nil
	}

	fundingTxHash, err := bitcoin.NewHashFromString(
		ras.DepositFundingTxHash,
		bitcoin.ReversedByteOrder,
	)
	if err != nil {
		panic(fmt.Errorf(
			"failed to parse anchor deposit funding tx hash: [%w]",
			err,
		))
	}

	return &tbtc.ReservationAnchorProposal{
		DepositFundingTxHash:      fundingTxHash,
		DepositFundingOutputIndex: ras.DepositFundingOutputIndex,
		RequestNonce:              ras.RequestNonce,
		AnchorTxFee:               big.NewInt(ras.AnchorTxFee),
	}
}

// LoadReservationAcceptanceTestScenario loads all scenarios related with
// reservation acceptance. The scenarios live in
// internal/test/testdata/reservation_acceptance_scenario_*.json.
func LoadReservationAcceptanceTestScenario() (
	[]*ReservationAcceptanceTestScenario,
	error,
) {
	return loadTestScenarios[*ReservationAcceptanceTestScenario](
		reservationAcceptanceTestDataFilePrefix,
	)
}

// UnmarshalJSON implements a custom JSON unmarshaling logic to produce a
// proper ReservationAcceptanceTestScenario.
func (rats *ReservationAcceptanceTestScenario) UnmarshalJSON(
	data []byte,
) error {
	type pendingAcceptanceActionScenarioJSON struct {
		RequestNonce              uint64
		TxMaxFee                  uint64
		MinAmount                 uint64
		TermSeconds               uint32
		TargetWalletPublicKeyHash string
		TimeoutAt                 uint32
	}

	type reservedDepositScenarioJSON struct {
		FundingTxHash          string
		FundingOutputIndex     uint32
		FundingTxConfirmations uint
		FundingTxHex           string
		WalletPublicKeyHash    string
		Depositor              string
		BlindingFactor         string
		RefundPublicKeyHash    string
		RefundLocktime         string
		Amount                 uint64
		RevealBlock            uint64
		Age                    int64
		SweptAt                int64
		Vault                  string

		PendingAcceptanceAction *pendingAcceptanceActionScenarioJSON
	}

	type scenario struct {
		Title           string
		ChainParameters struct {
			AverageBlockTime int64
			CurrentBlock     uint64
			DepositMinAge    uint32
		}
		WalletPublicKeyHash string
		ReservationVault    string
		Wallet              struct {
			State string
		}
		ReservationParameters struct {
			ReservationMinAmount      uint64
			ReservationTxMaxFee       uint64
			ReservationMaxTotalAmount uint64
			ReservationTotalAmount    uint64
			MaxReservationsPerWallet  uint32
		}
		Caps struct {
			MaxReservationsAmountPerWallet uint64
			ReservationMaxSingleAmount     uint64
		}
		WalletCustody struct {
			Count  uint32
			Amount uint64
		}
		Global struct {
			ActiveCount uint32
			MaxActive   uint32
		}
		PendingReservedDeposits uint64
		ReservedDeposits        []reservedDepositScenarioJSON
		ExpectedAnchorProposal  *reservationAnchorProposalScenario
		ExpectedErr             string
	}

	bytesFromHex := func(str string) []byte {
		value, err := hexutils.Decode(str)
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

	var unmarshaled scenario
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		return err
	}

	rats.Title = unmarshaled.Title

	rats.ChainParameters.AverageBlockTime =
		time.Duration(unmarshaled.ChainParameters.AverageBlockTime) * time.Second
	rats.ChainParameters.CurrentBlock = unmarshaled.ChainParameters.CurrentBlock
	rats.ChainParameters.DepositMinAge = unmarshaled.ChainParameters.DepositMinAge

	if len(unmarshaled.WalletPublicKeyHash) > 0 {
		walletBytes := hexToSlice(unmarshaled.WalletPublicKeyHash)
		if len(walletBytes) != 20 {
			return fmt.Errorf(
				"wallet public key hash must be 20 bytes, got [%d]",
				len(walletBytes),
			)
		}
		copy(rats.WalletPublicKeyHash[:], walletBytes)
	}

	rats.ReservationVault = unmarshaled.ReservationVault
	rats.WalletState = unmarshaled.Wallet.State

	rats.ReservationParameters.ReservationMinAmount =
		unmarshaled.ReservationParameters.ReservationMinAmount
	rats.ReservationParameters.ReservationTxMaxFee =
		unmarshaled.ReservationParameters.ReservationTxMaxFee
	rats.ReservationParameters.ReservationMaxTotalAmount =
		unmarshaled.ReservationParameters.ReservationMaxTotalAmount
	rats.ReservationParameters.ReservationTotalAmount =
		unmarshaled.ReservationParameters.ReservationTotalAmount
	rats.ReservationParameters.MaxReservationsPerWallet =
		unmarshaled.ReservationParameters.MaxReservationsPerWallet

	rats.Caps.MaxReservationsAmountPerWallet =
		unmarshaled.Caps.MaxReservationsAmountPerWallet
	rats.Caps.ReservationMaxSingleAmount =
		unmarshaled.Caps.ReservationMaxSingleAmount

	rats.WalletCustody.Count = unmarshaled.WalletCustody.Count
	rats.WalletCustody.Amount = unmarshaled.WalletCustody.Amount

	rats.Global.ActiveCount = unmarshaled.Global.ActiveCount
	rats.Global.MaxActive = unmarshaled.Global.MaxActive

	rats.PendingReservedDeposits = unmarshaled.PendingReservedDeposits

	rats.ReservedDeposits = make([]*ReservedDepositScenario, 0)
	for _, rd := range unmarshaled.ReservedDeposits {
		fundingTxHash, err := bitcoin.NewHashFromString(
			rd.FundingTxHash,
			bitcoin.ReversedByteOrder,
		)
		if err != nil {
			return fmt.Errorf(
				"failed to parse reserved deposit funding tx hash: [%w]",
				err,
			)
		}

		var fundingTx *bitcoin.Transaction
		if len(rd.FundingTxHex) > 0 {
			fundingTx = txFromHex(rd.FundingTxHex)
		}

		var pendingAction *PendingAcceptanceActionScenario
		if rd.PendingAcceptanceAction != nil {
			pendingAction = &PendingAcceptanceActionScenario{
				RequestNonce:              rd.PendingAcceptanceAction.RequestNonce,
				TxMaxFee:                  rd.PendingAcceptanceAction.TxMaxFee,
				MinAmount:                 rd.PendingAcceptanceAction.MinAmount,
				TermSeconds:               rd.PendingAcceptanceAction.TermSeconds,
				TargetWalletPublicKeyHash: rd.PendingAcceptanceAction.TargetWalletPublicKeyHash,
				TimeoutAt:                 rd.PendingAcceptanceAction.TimeoutAt,
			}
		}

		rats.ReservedDeposits = append(
			rats.ReservedDeposits,
			&ReservedDepositScenario{
				FundingTxHash:           rd.FundingTxHash,
				FundingOutputIndex:      rd.FundingOutputIndex,
				FundingTxConfirmations:  rd.FundingTxConfirmations,
				FundingTxHex:            rd.FundingTxHex,
				WalletPublicKeyHash:     rd.WalletPublicKeyHash,
				Depositor:               rd.Depositor,
				BlindingFactor:          rd.BlindingFactor,
				RefundPublicKeyHash:     rd.RefundPublicKeyHash,
				RefundLocktime:          rd.RefundLocktime,
				Amount:                  rd.Amount,
				RevealBlock:             rd.RevealBlock,
				Age:                     rd.Age,
				SweptAt:                 rd.SweptAt,
				Vault:                   rd.Vault,
				PendingAcceptanceAction: pendingAction,
				parsedFundingTxHash:     fundingTxHash,
				parsedFundingTx:         fundingTx,
			},
		)
	}

	rats.ExpectedAnchorProposal = unmarshaled.ExpectedAnchorProposal.convert()

	if len(unmarshaled.ExpectedErr) > 0 {
		rats.ExpectedErr = errors.New(unmarshaled.ExpectedErr)
	}

	return nil
}

// ReservedDeposit is the materialized form of a reserved deposit scenario,
// populated by the test driver once the chain state is set up.
type ReservedDeposit struct {
	// FundingTxHash is the hash of FundingTx, the deposit's real funding
	// transaction. LabelFundingTxHash is the scenario's FundingTxHash,
	// which only labels the deposit when the scenario gives no
	// FundingTxHex; ExpectedAnchorProposal refers to deposits by label.
	FundingTxHash       bitcoin.Hash
	LabelFundingTxHash  bitcoin.Hash
	FundingOutputIndex  uint32
	FundingTx           *bitcoin.Transaction
	WalletPublicKeyHash [20]byte
	Depositor           chain.Address
	BlindingFactor      [8]byte
	RefundPublicKeyHash [20]byte
	RefundLocktime      [4]byte
	RevealBlock         uint64
	RevealedAt          time.Time
	SweptAt             time.Time
	Amount              uint64
	Vault               *chain.Address
}

// Materialize converts a scenario row into a fully-typed ReservedDeposit
// the test driver can wire into the local chain. When the scenario gives
// no FundingTxHex, a funding transaction whose output at
// FundingOutputIndex locks Amount with the deposit's P2WSH script is
// built from the row's reveal fields, so the deposit passes the strict
// validator's extra-info checks.
func (rds *ReservedDepositScenario) Materialize() (*ReservedDeposit, error) {
	if rds == nil {
		return nil, fmt.Errorf("nil scenario deposit")
	}

	if rds.parsedFundingTxHash == (bitcoin.Hash{}) {
		return nil, fmt.Errorf("scenario not yet unmarshaled")
	}

	var walletHash [20]byte
	if len(rds.WalletPublicKeyHash) > 0 {
		copy(walletHash[:], hexToSlice(rds.WalletPublicKeyHash))
		rds.parsedWalletPKH = walletHash
	}

	var vault *chain.Address
	if len(rds.Vault) > 0 {
		addr := chain.Address(rds.Vault)
		vault = &addr
	}

	var blindingFactor [8]byte
	copy(blindingFactor[:], hexToSlice(rds.BlindingFactor))
	var refundPublicKeyHash [20]byte
	copy(refundPublicKeyHash[:], hexToSlice(rds.RefundPublicKeyHash))
	var refundLocktime [4]byte
	copy(refundLocktime[:], hexToSlice(rds.RefundLocktime))

	fundingTx := rds.parsedFundingTx
	if fundingTx == nil {
		depositScript, err := (&tbtc.Deposit{
			Depositor:           chain.Address(rds.Depositor),
			BlindingFactor:      blindingFactor,
			WalletPublicKeyHash: walletHash,
			RefundPublicKeyHash: refundPublicKeyHash,
			RefundLocktime:      refundLocktime,
		}).Script()
		if err != nil {
			return nil, fmt.Errorf("cannot build deposit script: [%w]", err)
		}
		depositLockingScript, err := bitcoin.PayToWitnessScriptHash(
			bitcoin.WitnessScriptHash(depositScript),
		)
		if err != nil {
			return nil, err
		}

		outputs := make([]*bitcoin.TransactionOutput, rds.FundingOutputIndex+1)
		for i := range outputs {
			outputs[i] = &bitcoin.TransactionOutput{
				PublicKeyScript: append([]byte{0x00, 0x14}, make([]byte, 20)...),
			}
		}
		outputs[rds.FundingOutputIndex] = &bitcoin.TransactionOutput{
			Value:           int64(rds.Amount),
			PublicKeyScript: depositLockingScript,
		}

		fundingTx = &bitcoin.Transaction{
			Version: 1,
			Inputs: []*bitcoin.TransactionInput{{
				Outpoint: &bitcoin.TransactionOutpoint{
					// The label makes every scenario deposit's funding
					// transaction, and so its hash, distinct.
					TransactionHash: rds.parsedFundingTxHash,
					OutputIndex:     0,
				},
				Sequence: 0xffffffff,
			}},
			Outputs: outputs,
		}
	}

	age := time.Duration(rds.Age) * time.Second
	revealedAt := time.Now().Add(-age)

	if rds.PendingAcceptanceAction != nil {
		var targetHash [20]byte
		if len(rds.PendingAcceptanceAction.TargetWalletPublicKeyHash) > 0 {
			copy(
				targetHash[:],
				hexToSlice(rds.PendingAcceptanceAction.TargetWalletPublicKeyHash),
			)
		}
		// Default the generation's timeout to a far-future value so the
		// signer-time timeout safety margin gate (REQUEST_TIMEOUT_SAFETY_MARGIN,
		// 2 hours in WalletProposalValidatorConstants.sol) does not skip
		// scenarios that intentionally exercise the gate separately.
		timeoutAt := rds.PendingAcceptanceAction.TimeoutAt
		if timeoutAt == 0 {
			timeoutAt = uint32(time.Now().Add(24 * time.Hour).Unix())
		}
		rds.parsedPendingAction = &tbtc.ReservationAction{
			ActionType:                tbtc.ReservationActionTypeAcceptance,
			State:                     tbtc.ReservationActionStatePending,
			TargetWalletPublicKeyHash: targetHash,
			TxMaxFee:                  rds.PendingAcceptanceAction.TxMaxFee,
			MinAmount:                 rds.PendingAcceptanceAction.MinAmount,
			TermSeconds:               rds.PendingAcceptanceAction.TermSeconds,
			TimeoutAt:                 timeoutAt,
			Amount:                    rds.Amount,
		}
	}

	return &ReservedDeposit{
		FundingTxHash:       fundingTx.Hash(),
		LabelFundingTxHash:  rds.parsedFundingTxHash,
		FundingOutputIndex:  rds.FundingOutputIndex,
		FundingTx:           fundingTx,
		WalletPublicKeyHash: walletHash,
		Depositor:           chain.Address(rds.Depositor),
		BlindingFactor:      blindingFactor,
		RefundPublicKeyHash: refundPublicKeyHash,
		RefundLocktime:      refundLocktime,
		RevealBlock:         rds.RevealBlock,
		RevealedAt:          revealedAt,
		SweptAt:             time.Unix(rds.SweptAt, 0),
		Amount:              rds.Amount,
		Vault:               vault,
	}, nil
}
