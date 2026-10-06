package ethereum

import "time"

// SubscribeOpts specifies optional configuration options that can be passed
// when creating Ethereum event subscription.
type SubscribeOpts struct {

	// Tick is the duration with which subscription monitoring mechanism
	// pulls events from the chain. This mechanism is an additional process
	// next to a regular watchLogs subscription making sure no events are lost
	// even in case the regular subscription missed them because of, for
	// example, connectivity problems.
	Tick time.Duration

	// PastBlocks is the number of past blocks subscription monitoring mechanism
	// takes into consideration when pulling past events from the chain.
	// This event pull mechanism is an additional process next to a regular
	// watchLogs subscription making sure no events are lost even in case the
	// regular subscription missed them because of, for example, connectivity
	// problems.
	PastBlocks uint64
}
