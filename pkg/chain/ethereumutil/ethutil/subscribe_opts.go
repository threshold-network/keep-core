package ethutil

import "time"

const (
	// DefaultSubscribeOptsTick is the default duration with which
	// past events are pulled from the chain by the subscription monitoring
	// mechanism if no other value is provided in SubscribeOpts when creating
	// the subscription.
	DefaultSubscribeOptsTick = 15 * time.Minute

	// DefaultSubscribeOptsPastBlocks is the default number of past blocks
	// pulled from the chain by the subscription monitoring mechanism if no
	// other value is provided in SubscribeOpts when creating the subscription.
	DefaultSubscribeOptsPastBlocks = 100

	// SubscriptionBackoffMax is the maximum backoff time between event
	// resubscription attempts.
	SubscriptionBackoffMax = 2 * time.Minute

	// SubscriptionAlertThreshold is time threshold below which event
	// resubscription emits an error to the logs.
	// WS connection can be dropped at any moment and event resubscription will
	// follow. However, if WS connection for event subscription is getting
	// dropped too often, it may indicate something is wrong with Ethereum
	// client. This constant defines the minimum lifetime of an event
	// subscription required before the subscription failure happens and
	// resubscription follows so that the resubscription does not emit an error
	// to the logs alerting about potential problems with Ethereum client
	// connection.
	SubscriptionAlertThreshold = 15 * time.Minute
)
