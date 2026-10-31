package tbtcpg

// SetMetricsRecorderForTest exposes setMetricsRecorder to the external
// tbtcpg_test package, whose acceptance fixtures build full candidates.
func (rat *ReservationAcceptanceTask) SetMetricsRecorderForTest(
	recorder interface {
		SetGauge(name string, value float64)
	},
) {
	rat.setMetricsRecorder(recorder)
}
