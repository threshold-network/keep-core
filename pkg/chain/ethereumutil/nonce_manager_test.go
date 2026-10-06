package ethereum

import (
	"context"
	"testing"
	"time"
)

func TestResolveAndIncrement(t *testing.T) {
	tests := map[string]struct {
		pendingNonce      uint64
		localNonce        uint64
		expirationDate    time.Time
		expectedNonce     uint64
		expectedNextNonce uint64
	}{
		"pending and local the same": {
			pendingNonce:      10,
			localNonce:        10,
			expirationDate:    time.Now().Add(time.Second),
			expectedNonce:     10,
			expectedNextNonce: 11,
		},
		"pending nonce higher": {
			pendingNonce:      121,
			localNonce:        120,
			expirationDate:    time.Now().Add(time.Second),
			expectedNonce:     121,
			expectedNextNonce: 122,
		},
		"pending nonce lower": {
			pendingNonce:      110,
			localNonce:        111,
			expirationDate:    time.Now().Add(time.Second),
			expectedNonce:     111,
			expectedNextNonce: 112,
		},
		"pending nonce lower and local one expired": {
			pendingNonce:      110,
			localNonce:        111,
			expirationDate:    time.Now().Add(-1 * time.Second),
			expectedNonce:     110,
			expectedNextNonce: 111,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			transactor := &mockContractTransactor{test.pendingNonce}
			manager := &NonceManager{
				transactor:     transactor,
				localNonce:     test.localNonce,
				expirationDate: test.expirationDate,
			}

			nonce, err := manager.CurrentNonce()
			if err != nil {
				t.Fatal(err)
			}

			if nonce != test.expectedNonce {
				t.Errorf(
					"unexpected nonce\nexpected: [%v]\nactual:  [%v]",
					test.expectedNonce,
					nonce,
				)
			}

			nextNonce := manager.IncrementNonce()

			if nextNonce != test.expectedNextNonce {
				t.Errorf(
					"unexpected nonce\nexpected: [%v]\nactual:  [%v]",
					test.expectedNextNonce,
					nextNonce,
				)
			}
		})
	}
}

type mockContractTransactor struct {
	nextNonce uint64
}

func (mct *mockContractTransactor) PendingNonceAt(
	ctx context.Context,
	account Address,
) (uint64, error) {
	return mct.nextNonce, nil
}
