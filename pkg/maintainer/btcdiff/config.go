package btcdiff

import "time"

// Config contains maintainer configuration.
type Config struct {
	// Enabled indicates whether the Bitcoin difficulty maintainer
	// should be started.
	Enabled bool

	// DisableProxy indicates whether the Bitcoin difficulty
	// maintainer proxy should be disabled. By default, the Bitcoin difficulty
	// maintainer submits the epoch headers to the relay via the proxy to be
	// reimbursed for the transaction. If this option is set to true, the epoch
	// headers will be submitted directly to the relay.
	DisableProxy bool

	// IdleOnPreflightFailure, when true, treats a relay preflight failure due to
	// pre-retarget headers not matching LightRelay rules (ErrUniformPreRetargetDifficulty) as a
	// non-fatal condition: skip submitting and idle until the next tick instead
	// of surfacing an error that restarts the maintainer loop. Use on Bitcoin
	// testnets where minimum-difficulty blocks make LightRelay proofs impossible.
	IdleOnPreflightFailure bool

	// IdleBackOffTime is a wait time which should be applied when there are no
	// more Bitcoin epochs to be proven because the difficulty maintainer is
	// up-to-date with the Bitcoin blockchain or there are not enough blocks yet
	// to prove the new epoch.
	IdleBackOffTime time.Duration

	// RestartBackOffTime is a restart backoff which should be applied when the Bitcoin
	// difficulty maintainer is restarted. It helps to avoid being flooded with
	// error logs in case of a permanent error in the Bitcoin difficulty
	// maintainer.
	RestartBackOffTime time.Duration
}
