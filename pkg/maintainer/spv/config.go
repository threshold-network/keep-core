package spv

import (
	"time"
)

const (
	// DefaultHistoryDepth is the default value for history depth which is the
	// number of blocks to look back from the current block when searching for
	// past wallet-related events. The value is the approximate number of
	// Ethereum blocks in a week, assuming one block is 12s.
	DefaultHistoryDepth = 50400

	// DefaultTransactionLimit is the default value for the limit of
	// transactions returned for a given wallet public key hash. The value is
	// based on the frequency of how often wallet transactions will happen.
	// For example, deposit sweep transactions are assumed to happen every 48h.
	// Redemption transactions are assumed to happen every 3h. The wallet should
	// refuse any proposals from the coordinator if the previously executed
	// Bitcoin transaction was not proved to the Bridge yet so in theory, the
	// value of 1 should be enough. We make it a bit higher - better to be
	// safe than sorry.
	DefaultTransactionLimit = 20

	// DefaultRestartBackoffTime is the default value for restart back-off time.
	DefaultRestartBackoffTime = 30 * time.Minute

	// DefaultIdleBackOffTime is the default value for idle back-off time.
	DefaultIdleBackOffTime = 10 * time.Minute
)

// Config holds configurable properties.
type Config struct {
	// Enabled indicates whether the SPV maintainer should be started.
	Enabled bool

	// HistoryDepth is the number of blocks to look back from the current block
	// when searching for past wallet-related events. To find Bitcoin transactions
	// for which the SPV proof should be submitted, the maintainer first inspects
	// the appropriate type of wallet-related events. This depth determines how
	// far into the past the system will consider events for processing. This
	// value must not be too high so that the event lookup is efficient. At the
	// same time, this value can not be too low to make sure all performed and
	// not yet proven transactions can be found.
	HistoryDepth uint64

	// TransactionLimit sets the maximum number of confirmed transactions
	// returned when getting transactions for a public key hash. Once the
	// maintainer establishes the list of wallets, it needs to check Bitcoin
	// transactions executed by each wallet. Then, it tries to find the
	// transactions matching the given proposal type. For example, if set
	// to `20`, only the latest twenty transactions will be returned. This
	// value must not be too high so that the transaction lookup is efficient.
	// At the same time, this value can not be too low to make sure the
	// performed proposal's transaction can be found in case the wallet decided
	// to execute some other Bitcoin transaction after the yet-not-proven
	// transaction.
	TransactionLimit int

	// RestartBackoffTime is a restart backoff which should be applied when the
	// SPV maintainer is restarted. It helps to avoid being flooded with error
	// logs in case of a permanent error in the SPV maintainer.
	RestartBackoffTime time.Duration

	// IdleBackoffTime is a wait time which should be applied when there are no
	// more transaction proofs to submit.
	IdleBackoffTime time.Duration

	// Reservations gates the m1 reservation feature within the SPV maintainer:
	// reservation acceptance / re-anchor proof tasks and the stranding /
	// stale-deposit / action-timeout watchers. When disabled the SPV maintainer
	// constructs without any reservation plumbing.
	Reservations ReservationsConfig
}

// ReservationsConfig holds the reservation-related spv.Config fields.
//
// This flag controls only SPV PROOF SUBMISSION for reservation acceptance /
// re-anchor action generations (the `maintainer` command, config category
// Maintainer). The `start` command runs as a separate process reading a
// disjoint config category (see config.StartCmdCategories) and has its own
// independent gate, tbtc.ReservationsConfig.Enabled, that controls
// reservation proposal GENERATION. Neither command's config loading sees
// the other's category, so this flag cannot be derived from or validated
// against tbtc.ReservationsConfig.Enabled in code. An operator running both
// `start` and `maintainer` for the reservation feature to work end-to-end
// MUST enable both flags - normally the same [Maintainer.Spv.Reservations]
// / [Tbtc.Reservations] TOML sections in one shared config file.
type ReservationsConfig struct {
	// Enabled toggles reservation plumbing in the SPV maintainer.
	Enabled bool
}
