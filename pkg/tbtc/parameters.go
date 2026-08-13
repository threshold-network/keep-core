package tbtc

import "math/big"

// MovingFundsParameters holds the current value of the on-chain parameters
// relevant to the moving funds and moved funds sweep processes. It replaces a
// wide positional return so callers access fields by name.
type MovingFundsParameters struct {
	TxMaxTotalFee                        uint64
	DustThreshold                        uint64
	TimeoutResetDelay                    uint32
	Timeout                              uint32
	TimeoutSlashingAmount                *big.Int
	TimeoutNotifierRewardMultiplier      uint32
	CommitmentGasOffset                  uint16
	SweepTxMaxTotalFee                   uint64
	SweepTimeout                         uint32
	SweepTimeoutSlashingAmount           *big.Int
	SweepTimeoutNotifierRewardMultiplier uint32
}

// WalletParameters holds the current value of the on-chain parameters relevant
// to the wallet lifecycle.
type WalletParameters struct {
	CreationPeriod        uint32
	CreationMinBtcBalance uint64
	CreationMaxBtcBalance uint64
	ClosureMinBtcBalance  uint64
	MaxAge                uint32
	MaxBtcTransfer        uint64
	ClosingPeriod         uint32
}

// DepositParameters holds the current value of the on-chain parameters relevant
// to deposits.
type DepositParameters struct {
	DustThreshold      uint64
	TreasuryFeeDivisor uint64
	TxMaxFee           uint64
	RevealAheadPeriod  uint32
}

// RedemptionParameters holds the current value of the on-chain parameters
// relevant to redemptions.
type RedemptionParameters struct {
	DustThreshold                   uint64
	TreasuryFeeDivisor              uint64
	TxMaxFee                        uint64
	TxMaxTotalFee                   uint64
	Timeout                         uint32
	TimeoutSlashingAmount           *big.Int
	TimeoutNotifierRewardMultiplier uint32
}
