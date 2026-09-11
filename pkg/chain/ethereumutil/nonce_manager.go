package ethereum

import (
	"context"
	"time"
)

// The inactivity time after which the local nonce is refreshed with the value
// from the chain. The local value is invalidated after the certain duration to
// let the nonce recover in case the mempool crashed before propagating the last
// transaction sent.
const localNonceTrustDuration = 30 * time.Second

// NonceManager tracks the nonce for the account and allows to update it after
// each successfully submitted transaction. Tracking the nonce locally is
// required when transactions are submitted from multiple goroutines or when
// multiple Ethereum clients are deployed behind a load balancer,
// there are no sticky sessions and mempool synchronization between them
// takes some time.
//
// NonceManager provides no synchronization and is NOT safe for concurrent use.
// It is up to the client code to implement the required synchronization.
//
// An example execution might work as follows:
// 1. Obtain transaction lock,
// 2. Calculate CurrentNonce(),
// 3. Submit transaction with the calculated nonce,
// 4. Call IncrementNonce(),
// 5. Release transaction lock.
type NonceManager struct {
	account        Address
	transactor     ContractTransactor
	localNonce     uint64
	expirationDate time.Time
}

// NewNonceManager creates NonceManager instance for the provided account using
// the provided contract transactor. Contract transactor is used for every
// CurrentNonce execution to check the pending nonce value as seen by the
// Ethereum client.
func NewNonceManager(
	transactor ContractTransactor,
	account Address,
) *NonceManager {
	return &NonceManager{
		account:    account,
		transactor: transactor,
		localNonce: 0,
	}
}

// CurrentNonce returns the nonce value that should be used for the next
// transaction. The nonce is evaluated as the higher value from the local
// nonce and pending nonce fetched from the Ethereum client. The local nonce
// is cached for the specific duration. If the local nonce expired, the pending
// nonce returned from the chain is used.
//
// CurrentNonce is NOT safe for concurrent use. It is up to the code using this
// function to provide the required synchronization, optionally including
// IncrementNonce call as well.
func (nm *NonceManager) CurrentNonce() (uint64, error) {
	pendingNonce, err := nm.transactor.PendingNonceAt(
		context.TODO(),
		nm.account,
	)
	if err != nil {
		return 0, err
	}

	now := time.Now()

	if pendingNonce < nm.localNonce {
		if now.Before(nm.expirationDate) {
			logger.Infof(
				"local nonce [%v] is higher than pending [%v]; using the local one",
				nm.localNonce,
				pendingNonce,
			)
		} else {
			logger.Infof(
				"local nonce [%v] is higher than pending [%v] but local "+
					"nonce expired; updating local nonce",
				nm.localNonce,
				pendingNonce,
			)

			nm.localNonce = pendingNonce
		}
	}

	// After localNonceTrustDuration of inactivity (no CurrentNonce() calls),
	// the local copy is considered as no longer up-to-date and it's always
	// reset to the pending nonce value as seen by the chain.
	//
	// We do it to recover from potential mempool crashes.
	//
	// Keep in mind, the local copy is considered valid as long as transactions
	// are submitted one after another.
	nm.expirationDate = now.Add(localNonceTrustDuration)

	if pendingNonce > nm.localNonce {
		logger.Infof(
			"local nonce [%v] is lower than pending [%v]; updating local nonce",
			nm.localNonce,
			pendingNonce,
		)

		nm.localNonce = pendingNonce
	}

	return nm.localNonce, nil
}

// IncrementNonce increments the value of the nonce kept locally by one.
// This function is NOT safe for concurrent use. It is up to the client code
// using this function to provide the required synchronization.
func (nm *NonceManager) IncrementNonce() uint64 {
	nm.localNonce++
	return nm.localNonce
}
