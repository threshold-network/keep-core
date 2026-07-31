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

	// DefaultTransactionLimit is the default value for the limit of matching
	// (unproven protocol) transactions returned for a given wallet public key
	// hash. The value is based on the frequency of how often wallet
	// transactions will happen. For example, deposit sweep transactions are
	// assumed to happen every 48h. Redemption transactions are assumed to
	// happen every 3h. The wallet should refuse any proposals from the
	// coordinator if the previously executed Bitcoin transaction was not
	// proved to the Bridge yet so in theory, the value of 1 should be enough.
	// We make it a bit higher - better to be safe than sorry.
	DefaultTransactionLimit = 20

	// DefaultTransactionScanLimit is the default value for the maximum number
	// of not-yet-classified confirmed transactions examined per wallet, per
	// scan, when searching for matching (unproven protocol) transactions. It
	// bounds the per-tick cost of the scan; see TransactionScanLimit.
	DefaultTransactionScanLimit = 100

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

	// TransactionLimit sets the maximum number of matching (unproven
	// protocol) transactions returned for a given wallet public key hash.
	// Once the maintainer establishes the list of wallets, it needs to check
	// Bitcoin transactions executed by each wallet. Then, it tries to find
	// the transactions matching the given proposal type. This value must not
	// be too high so that the transaction lookup is efficient. At the same
	// time, this value can not be too low to make sure the performed
	// proposal's transaction can be found in case the wallet decided to
	// execute some other Bitcoin transaction after the yet-not-proven
	// transaction. See TransactionScanLimit for the separate bound on how
	// many raw transactions are examined to find these matches.
	TransactionLimit int

	// TransactionScanLimit sets the maximum number of not-yet-classified
	// confirmed transactions examined, per wallet, each time the maintainer
	// searches for matching (unproven protocol) transactions. Unlike
	// TransactionLimit, which bounds the number of matches returned, this
	// bounds the number of raw address transactions inspected to find them,
	// keeping a single scan's cost bounded even when a wallet address has
	// accumulated a large amount of unrelated (e.g. spam) transaction
	// history. Search progress is remembered across calls, so the scan
	// eventually covers the wallet's whole history over enough calls rather
	// than starting over each time.
	TransactionScanLimit int

	// RestartBackoffTime is a restart backoff which should be applied when the
	// SPV maintainer is restarted. It helps to avoid being flooded with error
	// logs in case of a permanent error in the SPV maintainer.
	RestartBackoffTime time.Duration

	// IdleBackoffTime is a wait time which should be applied when there are no
	// more transaction proofs to submit.
	IdleBackoffTime time.Duration
}
