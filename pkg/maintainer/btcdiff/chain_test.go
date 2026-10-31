package btcdiff

import (
	"math/big"
	"sync"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/chain/local_v1"
	"github.com/keep-network/keep-core/pkg/operator"
)

// RetargetEvent represents an invocation of the Retarget method.
type RetargetEvent struct {
	oldDifficulty, newDifficulty uint32
}

// localBitcoinChain represents a local Bitcoin difficulty chain.
type localBitcoinDifficultyChain struct {
	mutex sync.Mutex

	operatorPrivateKey *operator.PrivateKey

	currentEpoch uint64
	proofLength  uint64

	ready                        bool
	authorizedOperators          map[chain.Address]bool
	authorizedForRefundOperators map[chain.Address]bool

	retargetEvents           []*RetargetEvent
	retargetWithRefundEvents []*RetargetEvent
}

// Ready checks whether the relay is active (i.e. genesis has been performed).
func (lbdc *localBitcoinDifficultyChain) Ready() (bool, error) {
	lbdc.mutex.Lock()
	defer lbdc.mutex.Unlock()

	return lbdc.ready, nil
}

// IsAuthorized checks whether the given address has been authorized to
// submit a retarget directly to LightRelay. This function should be used
// when retargetting via LightRelayMaintainerProxy is disabled.
func (lbdc *localBitcoinDifficultyChain) IsAuthorized(
	address chain.Address,
) (bool, error) {
	lbdc.mutex.Lock()
	defer lbdc.mutex.Unlock()

	return lbdc.authorizedOperators[address], nil
}

// IsAuthorizedForRefund checks whether the given address has been
// authorized to submit a retarget via LightRelayMaintainerProxy. This
// function should be used when retargetting via LightRelayMaintainerProxy
// is not disabled.
func (lbdc *localBitcoinDifficultyChain) IsAuthorizedForRefund(
	address chain.Address,
) (bool, error) {
	lbdc.mutex.Lock()
	defer lbdc.mutex.Unlock()

	return lbdc.authorizedForRefundOperators[address], nil
}

// Signing returns the signing associated with the chain.
func (lbdc *localBitcoinDifficultyChain) Signing() chain.Signing {
	return local_v1.NewSigner(lbdc.operatorPrivateKey)
}

// Retarget adds a new epoch to the relay by providing a proof
// of the difficulty before and after the retarget.
func (lbdc *localBitcoinDifficultyChain) Retarget(
	headers []*bitcoin.BlockHeader,
) error {
	// For simplicity, store block header bits instead of their difficulty
	// targets.
	retargetEvent := &RetargetEvent{
		oldDifficulty: headers[len(headers)/2-1].Bits,
		newDifficulty: headers[len(headers)/2].Bits,
	}
	lbdc.mutex.Lock()
	defer lbdc.mutex.Unlock()

	lbdc.retargetEvents = append(lbdc.retargetEvents, retargetEvent)

	lbdc.currentEpoch++

	return nil
}

// RetargetWithRefund adds a new epoch to the relay by providing a proof of
// the difficulty before and after the retarget. The cost of calling this
// function is refunded to the caller.
func (lbdc *localBitcoinDifficultyChain) RetargetWithRefund(
	headers []*bitcoin.BlockHeader,
) error {
	// For simplicity, store block header bits instead of their difficulty
	// targets.
	retargetEvent := &RetargetEvent{
		oldDifficulty: headers[len(headers)/2-1].Bits,
		newDifficulty: headers[len(headers)/2].Bits,
	}
	lbdc.mutex.Lock()
	defer lbdc.mutex.Unlock()

	lbdc.retargetWithRefundEvents = append(lbdc.retargetWithRefundEvents, retargetEvent)

	lbdc.currentEpoch++

	return nil
}

// CurrentEpoch returns the number of the latest difficulty epoch which is
// proven to the relay. If the genesis epoch's number is set correctly, and
// retargets along the way have been legitimate, this equals the height of
// the block starting the most recent epoch, divided by 2016.
func (lbdc *localBitcoinDifficultyChain) CurrentEpoch() (uint64, error) {
	lbdc.mutex.Lock()
	defer lbdc.mutex.Unlock()

	return lbdc.currentEpoch, nil
}

// ProofLength returns the number of blocks required for each side of a
// retarget proof.
func (lbdc *localBitcoinDifficultyChain) ProofLength() (uint64, error) {
	lbdc.mutex.Lock()
	defer lbdc.mutex.Unlock()

	return lbdc.proofLength, nil
}

// GetCurrentAndPrevEpochDifficulty returns the difficulties of the current
// and previous Bitcoin epochs.
func (lbdc *localBitcoinDifficultyChain) GetCurrentAndPrevEpochDifficulty() (
	*big.Int, *big.Int, error,
) {
	panic("unimplemented")
}

// SetReady sets chain's status as either ready or not.
func (lbdc *localBitcoinDifficultyChain) SetReady(ready bool) {
	lbdc.mutex.Lock()
	defer lbdc.mutex.Unlock()

	lbdc.ready = ready
}

// SetAuthorizedOperator sets the given operator address as either authorized or
// unauthorized.
func (lbdc *localBitcoinDifficultyChain) SetAuthorizedOperator(
	operatorAddress chain.Address,
	authorized bool,
) {
	lbdc.mutex.Lock()
	defer lbdc.mutex.Unlock()

	lbdc.authorizedOperators[operatorAddress] = authorized
}

// SetAuthorizedForRefundOperator sets the given operator address as either
// authorized or unauthorized for refund.
func (lbdc *localBitcoinDifficultyChain) SetAuthorizedForRefundOperator(
	operatorAddress chain.Address,
	authorized bool,
) {
	lbdc.mutex.Lock()
	defer lbdc.mutex.Unlock()

	lbdc.authorizedForRefundOperators[operatorAddress] = authorized
}

// SetCurrentEpoch sets the current proven epoch in the chain.
func (lbdc *localBitcoinDifficultyChain) SetCurrentEpoch(currentEpoch uint64) {
	lbdc.mutex.Lock()
	defer lbdc.mutex.Unlock()

	lbdc.currentEpoch = currentEpoch
}

// SetProofLength sets the proof length needed for a retarget.
func (lbdc *localBitcoinDifficultyChain) SetProofLength(proofLength uint64) {
	lbdc.mutex.Lock()
	defer lbdc.mutex.Unlock()

	lbdc.proofLength = proofLength
}

// RetargetEvents returns all invocations of the Retarget method.
func (lbdc *localBitcoinDifficultyChain) RetargetEvents() []*RetargetEvent {
	lbdc.mutex.Lock()
	defer lbdc.mutex.Unlock()

	return append([]*RetargetEvent(nil), lbdc.retargetEvents...)
}

// RetargetWithRefundEvents returns all invocations of the Retarget method.
func (lbdc *localBitcoinDifficultyChain) RetargetWithRefundEvents() []*RetargetEvent {
	lbdc.mutex.Lock()
	defer lbdc.mutex.Unlock()

	return append([]*RetargetEvent(nil), lbdc.retargetWithRefundEvents...)
}

// connectLocalBitcoinDifficultyChain connects to the local Bitcoin difficulty
// chain and returns a chain handle.
func connectLocalBitcoinDifficultyChain() *localBitcoinDifficultyChain {
	operatorPrivateKey, _, err := operator.GenerateKeyPair(local_v1.DefaultCurve)
	if err != nil {
		panic(err)
	}

	return &localBitcoinDifficultyChain{
		operatorPrivateKey:           operatorPrivateKey,
		authorizedOperators:          make(map[chain.Address]bool),
		authorizedForRefundOperators: make(map[chain.Address]bool),
	}
}
