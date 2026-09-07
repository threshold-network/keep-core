package spv

import (
	"time"

	"github.com/keep-network/keep-core/pkg/clientinfo"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

func (sm *spvMaintainer) setHealthGauge(name string, value float64) {
	if sm.metricsRecorder != nil {
		sm.metricsRecorder.SetGauge(name, value)
	}
}

func (sm *spvMaintainer) recordActivity() {
	sm.setHealthGauge(clientinfo.MetricSpvMaintainerLastActivityTimestamp, float64(time.Now().Unix()))
}

// runProofTask includes discovery and proof-info failures that occur before a
// proof submitter can increment its submission counters. Skipped proofs are
// counted separately by proveTransactions and are not task failures.
func (sm *spvMaintainer) runProofTask(
	action tbtc.WalletActionType,
	getter unprovenTransactionsGetter,
	submitter transactionProofSubmitter,
) error {
	sm.recordActivity()
	err := sm.proveTransactions(getter, submitter)
	sm.recordActivity()
	if err != nil && sm.metricsRecorder != nil {
		sm.metricsRecorder.IncrementCounter(clientinfo.MetricSpvProofTaskFailuresTotal, 1)
		if action == tbtc.ActionRedemption {
			sm.metricsRecorder.IncrementCounter(clientinfo.MetricRedemptionProofTaskFailuresTotal, 1)
		}
	}
	return err
}
