//go:build integration

package electrum_test

import (
	"errors"
	"fmt"
	"testing"

	goelectrum "github.com/checksum0/go-electrum/electrum"

	"github.com/keep-network/keep-core/pkg/bitcoin/electrum"
	"github.com/keep-network/keep-core/pkg/wrappers"
)

// TestShouldSkipElectrumIntegrationError pins the transient-failure classifier
// used by the integration suite to decide whether a failure is the public
// Electrum service misbehaving (skip) or a real defect (fail).
//
// The classifier previously matched substrings of err.Error(), so rewording an
// upstream message silently widened or narrowed what counts as transient. It
// now matches sentinels, and these cases fail if the wrapping chain loses %w
// anywhere, or if the classifier starts swallowing unrelated errors.
func TestShouldSkipElectrumIntegrationError(t *testing.T) {
	tests := map[string]struct {
		err          error
		expectedSkip bool
	}{
		"nil error": {
			err:          nil,
			expectedSkip: false,
		},
		"bare request timeout": {
			err:          goelectrum.ErrTimeout,
			expectedSkip: true,
		},
		"wrapped request timeout": {
			err: fmt.Errorf(
				"failed to get raw transaction with ID [abc]: [%w]",
				goelectrum.ErrTimeout,
			),
			expectedSkip: true,
		},
		"bare retry timeout": {
			err:          wrappers.ErrRetryTimeout,
			expectedSkip: true,
		},
		"wrapped retry timeout": {
			err: fmt.Errorf(
				"failed to get the latest block height: [%w]",
				wrappers.ErrRetryTimeout,
			),
			expectedSkip: true,
		},
		"bare missing fee estimate": {
			err:          electrum.ErrFeeEstimateUnavailable,
			expectedSkip: true,
		},
		"wrapped missing fee estimate": {
			err: fmt.Errorf(
				"failed to get fee: [%w]",
				electrum.ErrFeeEstimateUnavailable,
			),
			expectedSkip: true,
		},
		"doubly wrapped sentinel": {
			err: fmt.Errorf(
				"outer: [%w]",
				fmt.Errorf("inner: [%w]", wrappers.ErrRetryTimeout),
			),
			expectedSkip: true,
		},
		"unrelated error": {
			err:          errors.New("transaction not found"),
			expectedSkip: false,
		},
		"error whose text merely mentions a timeout": {
			err:          errors.New("server said: request timeout is 30s"),
			expectedSkip: false,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			actualSkip := shouldSkipElectrumIntegrationError(test.err)
			if actualSkip != test.expectedSkip {
				t.Errorf(
					"unexpected skip decision for [%v]\n"+
						"expected: [%v]\n"+
						"actual:   [%v]",
					test.err,
					test.expectedSkip,
					actualSkip,
				)
			}
		})
	}
}
