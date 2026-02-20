package retry

import (
	"fmt"
	"math/rand"
	"sort"

	"github.com/keep-network/keep-core/pkg/chain"
)

type byAddress []chain.Address

func (ba byAddress) Len() int           { return len(ba) }
func (ba byAddress) Swap(i, j int)      { ba[i], ba[j] = ba[j], ba[i] }
func (ba byAddress) Less(i, j int) bool { return ba[i] < ba[j] }

func calculateSeatCount(groupMembers []chain.Address) map[chain.Address]uint {
	operatorToSeatCount := make(map[chain.Address]uint)
	for _, operator := range groupMembers {
		operatorToSeatCount[operator]++
	}
	return operatorToSeatCount
}

// EvaluateRetryParticipantsForSigning takes in a slice of `groupMembers` and
// returns a subslice of those same members of length >=
// `retryParticipantsCount` randomly according to the provided `seed` and
// `retryCount`.
//
// This function is intended to be called during a signing protocol after a
// signing event has failed but *not* due to inactivity. Assuming that some of
// the `groupMembers` are sending corrupted information, either on purpose or
// accidentally, we keep trying to find a subset of `groupMembers` that is as
// small as possible, yet still larger than `retryParticipantsCount`.
//
// The `seed` param needs to vary on a per-message basis but must be the same
// seed between all operators for each invocation. This can be the hash of the
// message since cryptographically secure randomness isn't important.
//
// The `retryCount` denotes the number of the given retry, so that should be
// incremented after each attempt while the `seed` stays consistent on a
// per-message basis.
func EvaluateRetryParticipantsForSigning(
	groupMembers []chain.Address,
	seed int64,
	retryCount uint,
	retryParticipantsCount uint,
) ([]chain.Address, error) {
	if int(retryParticipantsCount) > len(groupMembers) {
		return nil, fmt.Errorf(
			"asked for too many seats; [%d] seats were requested, but there are only [%d] available",
			retryParticipantsCount,
			len(groupMembers),
		)
	}
	operatorToSeatCount := calculateSeatCount(groupMembers)

	// #nosec G404 (insecure random number source (rand))
	// Shuffling operators for retries does not require secure randomness.
	rng := rand.New(rand.NewSource(seed + int64(retryCount)))

	operators := make([]chain.Address, len(operatorToSeatCount))
	i := 0
	for operator := range operatorToSeatCount {
		operators[i] = operator
		i++
	}
	sort.Sort(byAddress(operators))
	rng.Shuffle(len(operators), func(i, j int) {
		operators[i], operators[j] = operators[j], operators[i]
	})

	seatCount := uint(0)
	acceptedOperators := make(map[chain.Address]bool)
	for j := 0; seatCount < retryParticipantsCount; j++ {
		operator := operators[j]
		seatCount += operatorToSeatCount[operator]
		acceptedOperators[operator] = true
	}

	var seats []chain.Address
	for _, operator := range groupMembers {
		if acceptedOperators[operator] {
			seats = append(seats, operator)
		}
	}
	return seats, nil
}

// EvaluateRetryParticipantsForKeyGeneration takes in a slice of `groupMembers`
// and returns a subslice of those same members of length >=
// `retryParticipantsCount` randomly according to the provided `seed` and
// `retryCount`.
//
// This function is intended to be called during key generation after a failure
// *not* due to inactivity. Assuming that some of the `groupMembers` are
// sending corrupted information, either on purpose or accidentally, we keep
// trying to find a subset of `groupMembers` that is as large as possible by
// first excluding single operators, then pairs of operators, then triplets of
// operators. We use the `seed` param to generate randomness to shuffle the
// singles/pairs/triplets of operators to exclude and then use the `retryCount`
// param to select which single/pair/triplet to exclude.
//
// The `seed` param needs to vary on a per-message basis but must be the same
// seed between all operators for each invocation. This can be the hash of the
// message since cryptographically secure randomness isn't important.
//
// The `retryCount` denotes the number of the given retry, so that should be
// incremented after each attempt while the `seed` stays consistent on a
// per-message basis.
func EvaluateRetryParticipantsForKeyGeneration(
	groupMembers []chain.Address,
	seed int64,
	retryCount uint,
	retryParticipantsCount uint,
) ([]chain.Address, error) {
	remainingTries := retryCount
	if int(retryParticipantsCount) > len(groupMembers) {
		return nil, fmt.Errorf(
			"asked for too many seats; [%d] seats were requested, "+
				"but there are only [%d] available",
			retryParticipantsCount,
			len(groupMembers),
		)
	}
	operatorToSeatCount := calculateSeatCount(groupMembers)
	// #nosec G404 (insecure random number source (rand))
	// Shuffling operators for retries does not require secure randomness. Unlike
	// EvaluateRetryParticipantsForSigning above, we only want to use the seed as
	// a source of randomness. The `retryCount` is used to select which operators
	// to exclude after we shuffle them.
	rng := rand.New(rand.NewSource(seed))

	operators := make([]chain.Address, 0, len(operatorToSeatCount))
	for operator := range operatorToSeatCount {
		// Only include the operators that have few enough seats such that if they
		// were excluded we still have at least `retryParticipantsCount` seats.
		if len(groupMembers)-int(operatorToSeatCount[operator]) >= int(retryParticipantsCount) {
			operators = append(operators, operator)
		}
	}
	sort.Sort(byAddress(operators))

	usedOperators, tries, ok := excludeSingleOperator(
		rng,
		groupMembers,
		int(remainingTries),
		operatorToSeatCount,
		operators,
	)
	if ok {
		return usedOperators, nil
	} else {
		remainingTries -= uint(tries)
	}

	usedOperators, tries, ok = excludeOperatorPairs(
		rng,
		groupMembers,
		int(remainingTries),
		operatorToSeatCount,
		operators,
		int(retryParticipantsCount),
	)
	if ok {
		return usedOperators, nil
	} else {
		remainingTries -= uint(tries)
	}

	usedOperators, tries, ok = excludeOperatorTriplets(
		rng,
		groupMembers,
		int(remainingTries),
		operatorToSeatCount,
		operators,
		int(retryParticipantsCount),
	)
	if ok {
		return usedOperators, nil
	} else {
		remainingTries -= uint(tries)
		return nil, fmt.Errorf(
			"the retry count [%d] was too large to handle; "+
				"tried every single, pair, and triplet, but still needed [%d] more retries",
			retryCount,
			remainingTries,
		)
	}
}

// excludeSingleOperator randomly excludes all of an operator's seats from a
// given `groupMembers`. It needs a pre-seeded random generator `rng`, and an
// `index`, which is expected to be inferred from a `retryCount`.
//
// It does this by shuffling a list of eligible-for-exclusion operators
// according to `rng`, selecting the operator according to `index`, and then
// filtering that operator out of `groupMembers`.
//
// In the case that `index` is larger than the number of eligible operators, it
// skips shuffling and returns the number of eligible operators, which is
// useful for determining the index of the operator pair to ignore.
func excludeSingleOperator(
	rng *rand.Rand,
	groupMembers []chain.Address,
	index int,
	operatorToSeatCount map[chain.Address]uint,
	operators []chain.Address,
) ([]chain.Address, int, bool) {
	if index < len(operators) {
		rng.Shuffle(len(operators), func(i, j int) {
			operators[i], operators[j] = operators[j], operators[i]
		})
		removedOperator := operators[index]
		usedOperators := make([]chain.Address, 0, len(groupMembers))
		for _, operator := range groupMembers {
			if operator != removedOperator {
				usedOperators = append(usedOperators, operator)
			}
		}
		return usedOperators, 0, true
	} else {
		return nil, len(operators), false
	}
}

// excludeOperatorPairs randomly excludes all of a pair of operator's seats from a
// given `groupMembers`. It needs a pre-seeded random generator `rng`, and an
// `index`, which is expected to be inferred from a `retryCount`.
//
// It does this by shuffling a list of eligable-for-exclusion operators
// according to `rng`, selecting the operator according to `index`, and then
// filtering that operator pair out of `groupMembers`.
//
// In the case that `index` is larger than the number of eligible operator
// pairs, it skips shuffling and returns the number of eligible operators
// pairs, which is useful for determining the index of the operator triplet to
// ignore.
func excludeOperatorPairs(
	rng *rand.Rand,
	groupMembers []chain.Address,
	index int,
	operatorToSeatCount map[chain.Address]uint,
	operators []chain.Address,
	retryParticipantsCount int,
) ([]chain.Address, int, bool) {
	pairIndexes := make([][2]int, 0, len(operators)*len(operators))
	for i := 0; i < len(operators)-1; i++ {
		for j := i + 1; j < len(operators); j++ {
			leftOperator := operators[i]
			rightOperator := operators[j]

			// Only include the operators pairs that have few enough seats such that
			// if they were excluded we still have at least `retryParticipantsCount`
			// seats.
			count := len(groupMembers) -
				int(operatorToSeatCount[leftOperator]) -
				int(operatorToSeatCount[rightOperator])
			if count >= int(retryParticipantsCount) {
				pairIndexes = append(pairIndexes, [2]int{i, j})
			}
		}
	}
	if index < len(pairIndexes) {
		rng.Shuffle(len(pairIndexes), func(i, j int) {
			pairIndexes[i], pairIndexes[j] = pairIndexes[j], pairIndexes[i]
		})
		pair := pairIndexes[index]
		leftOperator := operators[pair[0]]
		rightOperator := operators[pair[1]]
		usedOperators := make([]chain.Address, 0, len(groupMembers))
		for _, operator := range groupMembers {
			if operator != leftOperator && operator != rightOperator {
				usedOperators = append(usedOperators, operator)
			}
		}
		return usedOperators, 0, true
	} else {
		return nil, len(pairIndexes), false
	}
}

// excludeOperatorTriplets randomly excludes all of a triplet of operator's seats from a
// given `groupMembers`. It needs a pre-seeded random generator `rng`, and an
// `index`, which is expected to be inferred from a `retryCount`.
//
// It does this by shuffling a list of eligable-for-exclusion operators
// according to `rng`, selecting the operator according to `index`, and then
// filtering that operator triplet out of `groupMembers`.
//
// In the case that `index` is larger than the number of eligible operator
// triplets, it skips shuffling and returns the number of eligible operators
// triplets, which is useful for logging errors.
func excludeOperatorTriplets(
	rng *rand.Rand,
	groupMembers []chain.Address,
	index int,
	operatorToSeatCount map[chain.Address]uint,
	operators []chain.Address,
	retryParticipantsCount int,
) ([]chain.Address, int, bool) {
	tripletIndexes := make([][3]int, 0, len(operators)*len(operators)*len(operators))
	for i := 0; i < len(operators)-2; i++ {
		for j := i + 1; j < len(operators)-1; j++ {
			for k := j + 1; k < len(operators); k++ {
				leftOperator := operators[i]
				middleOperator := operators[j]
				rightOperator := operators[j]

				// Only include the operators triples that have few enough seats such
				// that if they were excluded we still have at least
				// `retryParticipantsCount` seats.
				count := len(groupMembers) -
					int(operatorToSeatCount[leftOperator]) -
					int(operatorToSeatCount[middleOperator]) -
					int(operatorToSeatCount[rightOperator])
				if count >= int(retryParticipantsCount) {
					tripletIndexes = append(tripletIndexes, [3]int{i, j, k})
				}
			}
		}
	}
	if index < len(tripletIndexes) {
		rng.Shuffle(len(tripletIndexes), func(i, j int) {
			tripletIndexes[i], tripletIndexes[j] = tripletIndexes[j], tripletIndexes[i]
		})
		triplet := tripletIndexes[index]
		leftOperator := operators[triplet[0]]
		middleOperator := operators[triplet[1]]
		rightOperator := operators[triplet[2]]
		usedOperators := make([]chain.Address, 0, len(groupMembers))
		for _, operator := range groupMembers {
			if operator != leftOperator && operator != middleOperator && operator != rightOperator {
				usedOperators = append(usedOperators, operator)
			}
		}
		return usedOperators, 0, true
	} else {
		return nil, len(tripletIndexes), false
	}
}
