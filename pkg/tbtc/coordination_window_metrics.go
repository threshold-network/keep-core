package tbtc

import (
	"fmt"
	"sync"
	"time"

	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/clientinfo"
)

// coordinationWindowMetrics tracks detailed metrics for individual coordination windows.
type coordinationWindowMetrics struct {
	mu sync.RWMutex

	// windows stores metrics for each coordination window by window index
	windows map[uint64]*windowMetrics

	// performanceMetrics is used to record aggregate metrics
	performanceMetrics clientinfo.PerformanceMetricsRecorder

	// maxWindowsToTrack limits the number of windows to keep in memory
	// to prevent unbounded memory growth
	maxWindowsToTrack uint64
}

// windowMetrics contains all metrics for a single coordination window.
type windowMetrics struct {
	// Window identification
	WindowIndex       uint64 `json:"window_index"`
	CoordinationBlock uint64 `json:"coordination_block"`

	// Window timing
	StartTime           time.Time     `json:"start_time"`
	EndTime             time.Time     `json:"end_time"`
	Duration            time.Duration `json:"duration_ns"`
	ActivePhaseEndBlock uint64        `json:"active_phase_end_block"`
	EndBlock            uint64        `json:"end_block"`

	// Coordination statistics
	WalletsCoordinated       uint64 `json:"wallets_coordinated"`
	WalletsSuccessful        uint64 `json:"wallets_successful"`
	WalletsFailed            uint64 `json:"wallets_failed"`
	TotalProceduresStarted   uint64 `json:"total_procedures_started"`
	TotalProceduresCompleted uint64 `json:"total_procedures_completed"`

	// Leader information
	Leaders map[string]uint64 `json:"leaders"` // leader address -> count of wallets they led

	// Action type statistics
	ActionTypes map[string]uint64 `json:"action_types"` // action type -> count

	// Fault statistics
	TotalFaults     uint64            `json:"total_faults"`
	FaultsByType    map[string]uint64 `json:"faults_by_type"`    // fault type -> count
	FaultsByCulprit map[string]uint64 `json:"faults_by_culprit"` // culprit address -> count

	// Per-wallet coordination details
	WalletCoordinationDetails []walletCoordinationDetail `json:"wallet_coordination_details"`
}

// walletCoordinationDetail contains metrics for a single wallet's coordination
// in a window.
type walletCoordinationDetail struct {
	WalletPublicKeyHash string        `json:"wallet_public_key_hash"`
	Leader              string        `json:"leader"`
	ActionType          string        `json:"action_type"`
	Success             bool          `json:"success"`
	Duration            time.Duration `json:"duration_ns"`
	ErrorMessage        string        `json:"error_message,omitempty"` // error message if failed
	Faults              []faultDetail `json:"faults"`                  // detailed fault information
}

// faultDetail contains detailed information about a coordination fault.
type faultDetail struct {
	Type    string `json:"type"`    // fault type (e.g., LeaderIdleness, LeaderMistake)
	Culprit string `json:"culprit"` // address of the operator responsible
	Message string `json:"message"` // human-readable description
}

// faultMessage generates a human-readable message for a coordination fault.
func faultMessage(faultType CoordinationFaultType, culprit string) string {
	switch faultType {
	case FaultLeaderIdleness:
		return fmt.Sprintf("Leader %s was idle and missed their turn to propose a wallet action", culprit)
	case FaultLeaderMistake:
		return fmt.Sprintf("Leader %s proposed an invalid action", culprit)
	case FaultLeaderImpersonation:
		return fmt.Sprintf("Operator %s impersonated the leader", culprit)
	case FaultUnknown:
		return fmt.Sprintf("Unknown fault from operator %s", culprit)
	default:
		return fmt.Sprintf("Fault type %s from operator %s", faultType.String(), culprit)
	}
}

// newCoordinationWindowMetrics creates a new coordination window metrics tracker.
func newCoordinationWindowMetrics(
	performanceMetrics clientinfo.PerformanceMetricsRecorder,
	maxWindowsToTrack uint64,
) *coordinationWindowMetrics {
	return &coordinationWindowMetrics{
		windows:            make(map[uint64]*windowMetrics),
		performanceMetrics: performanceMetrics,
		maxWindowsToTrack:  maxWindowsToTrack,
	}
}

// recordWindowStart records the start of a coordination window.
func (cwm *coordinationWindowMetrics) recordWindowStart(window *coordinationWindow) {
	cwm.mu.Lock()
	defer cwm.mu.Unlock()

	cwm.initializeWindowIfNeeded(window)
}

// initializeWindowIfNeeded initializes window metrics if they don't exist.
// This function assumes the caller already holds cwm.mu.Lock().
func (cwm *coordinationWindowMetrics) initializeWindowIfNeeded(window *coordinationWindow) {
	windowIndex := window.index()
	if windowIndex == 0 {
		// Invalid window, skip
		return
	}

	// Initialize window metrics if not exists
	if _, exists := cwm.windows[windowIndex]; !exists {
		cwm.windows[windowIndex] = &windowMetrics{
			WindowIndex:               windowIndex,
			CoordinationBlock:         window.coordinationBlock,
			StartTime:                 time.Now(),
			ActivePhaseEndBlock:       window.activePhaseEndBlock(),
			EndBlock:                  window.endBlock(),
			Leaders:                   make(map[string]uint64),
			ActionTypes:               make(map[string]uint64),
			FaultsByType:              make(map[string]uint64),
			FaultsByCulprit:           make(map[string]uint64),
			WalletCoordinationDetails: make([]walletCoordinationDetail, 0),
		}
	}

	// Clean up old windows if we exceed the limit
	cwm.cleanupOldWindows()
}

// recordWindowEnd records the end of a coordination window.
func (cwm *coordinationWindowMetrics) recordWindowEnd(window *coordinationWindow) {
	cwm.mu.Lock()
	defer cwm.mu.Unlock()

	windowIndex := window.index()
	if windowIndex == 0 {
		return
	}

	wm, exists := cwm.windows[windowIndex]
	if !exists {
		return
	}

	// This guard prevents double-recording when recordWindowEnd is called
	// both during normal operation (when a new window is detected) and during
	// shutdown cleanup (to ensure the last active window is properly closed).
	// Without this check, the shutdown cleanup goroutine could overwrite the
	// EndTime that was already set by the normal window transition flow.
	if !wm.EndTime.IsZero() {
		return
	}

	wm.EndTime = time.Now()
	wm.Duration = wm.EndTime.Sub(wm.StartTime)

	// Record aggregate metrics
	if cwm.performanceMetrics != nil {
		// Record window duration
		cwm.performanceMetrics.RecordDuration(
			clientinfo.MetricCoordinationWindowDurationSeconds,
			wm.Duration,
		)
	}
}

// recordWalletCoordination records metrics for a single wallet's coordination
// in a window.
func (cwm *coordinationWindowMetrics) recordWalletCoordination(
	window *coordinationWindow,
	walletPublicKeyHash [20]byte,
	leader chain.Address,
	actionType string,
	success bool,
	duration time.Duration,
	faults []*coordinationFault,
	coordinationErr error,
) {
	cwm.mu.Lock()
	defer cwm.mu.Unlock()

	windowIndex := window.index()
	if windowIndex == 0 {
		return
	}

	wm, exists := cwm.windows[windowIndex]
	if !exists {
		// Window not initialized, initialize it now
		// Note: we already hold the lock, so use the lock-free helper
		cwm.initializeWindowIfNeeded(window)
		wm = cwm.windows[windowIndex]
	}

	// Update window-level statistics
	wm.WalletsCoordinated++
	wm.TotalProceduresStarted++
	if success {
		wm.WalletsSuccessful++
		wm.TotalProceduresCompleted++
	} else {
		wm.WalletsFailed++
	}

	// Track leader
	leaderStr := leader.String()
	wm.Leaders[leaderStr]++

	// Track action type
	if actionType != "" {
		wm.ActionTypes[actionType]++
	}

	// Track faults
	faultDetails := make([]faultDetail, 0, len(faults))
	for _, fault := range faults {
		faultTypeStr := fault.faultType.String()
		culpritStr := fault.culprit.String()

		wm.FaultsByType[faultTypeStr]++
		wm.TotalFaults++
		wm.FaultsByCulprit[culpritStr]++

		faultDetails = append(faultDetails, faultDetail{
			Type:    faultTypeStr,
			Culprit: culpritStr,
			Message: faultMessage(fault.faultType, culpritStr),
		})
	}

	// Record per-wallet detail
	detail := walletCoordinationDetail{
		WalletPublicKeyHash: fmt.Sprintf("0x%x", walletPublicKeyHash),
		Leader:              leaderStr,
		ActionType:          actionType,
		Success:             success,
		Duration:            duration,
		Faults:              faultDetails,
	}
	if coordinationErr != nil {
		detail.ErrorMessage = coordinationErr.Error()
	}
	wm.WalletCoordinationDetails = append(wm.WalletCoordinationDetails, detail)
}

// GetWindowMetrics returns metrics for a specific window.
func (cwm *coordinationWindowMetrics) GetWindowMetrics(windowIndex uint64) (*windowMetrics, bool) {
	cwm.mu.RLock()
	defer cwm.mu.RUnlock()

	wm, exists := cwm.windows[windowIndex]
	if !exists {
		return nil, false
	}

	// Return a deep copy to avoid race conditions
	return wm.deepCopy(), true
}

// GetRecentWindows returns metrics for the most recent N windows.
func (cwm *coordinationWindowMetrics) GetRecentWindows(limit int) []*windowMetrics {
	cwm.mu.RLock()
	defer cwm.mu.RUnlock()

	// Collect all window indices and sort them
	indices := make([]uint64, 0, len(cwm.windows))
	for idx := range cwm.windows {
		indices = append(indices, idx)
	}

	// Sort in descending order (most recent first)
	for i := 0; i < len(indices)-1; i++ {
		for j := i + 1; j < len(indices); j++ {
			if indices[i] < indices[j] {
				indices[i], indices[j] = indices[j], indices[i]
			}
		}
	}

	// Limit results
	if limit > 0 && limit < len(indices) {
		indices = indices[:limit]
	}

	// Return deep copies
	result := make([]*windowMetrics, 0, len(indices))
	for _, idx := range indices {
		wm := cwm.windows[idx]
		result = append(result, wm.deepCopy())
	}

	return result
}

// cleanupOldWindows removes old windows to prevent unbounded memory growth.
func (cwm *coordinationWindowMetrics) cleanupOldWindows() {
	if uint64(len(cwm.windows)) <= cwm.maxWindowsToTrack {
		return
	}

	// Find the oldest window indices
	indices := make([]uint64, 0, len(cwm.windows))
	for idx := range cwm.windows {
		indices = append(indices, idx)
	}

	// Sort in ascending order (oldest first)
	for i := 0; i < len(indices)-1; i++ {
		for j := i + 1; j < len(indices); j++ {
			if indices[i] > indices[j] {
				indices[i], indices[j] = indices[j], indices[i]
			}
		}
	}

	// Remove oldest windows
	windowsToRemove := len(cwm.windows) - int(cwm.maxWindowsToTrack)
	for i := 0; i < windowsToRemove; i++ {
		delete(cwm.windows, indices[i])
	}
}

// GetSummary returns a summary of all tracked windows.
func (cwm *coordinationWindowMetrics) GetSummary() WindowMetricsSummary {
	cwm.mu.RLock()
	defer cwm.mu.RUnlock()

	summary := WindowMetricsSummary{
		TotalWindows:            uint64(len(cwm.windows)),
		TotalWalletsCoordinated: 0,
		TotalWalletsSuccessful:  0,
		TotalWalletsFailed:      0,
		TotalFaults:             0,
		Windows:                 make([]*windowMetrics, 0, len(cwm.windows)),
	}

	for _, wm := range cwm.windows {
		summary.TotalWalletsCoordinated += wm.WalletsCoordinated
		summary.TotalWalletsSuccessful += wm.WalletsSuccessful
		summary.TotalWalletsFailed += wm.WalletsFailed
		summary.TotalFaults += wm.TotalFaults

		summary.Windows = append(summary.Windows, wm.deepCopy())
	}

	return summary
}

// WindowMetricsSummary provides a summary of coordination window metrics.
type WindowMetricsSummary struct {
	TotalWindows            uint64           `json:"total_windows"`
	TotalWalletsCoordinated uint64           `json:"total_wallets_coordinated"`
	TotalWalletsSuccessful  uint64           `json:"total_wallets_successful"`
	TotalWalletsFailed      uint64           `json:"total_wallets_failed"`
	TotalFaults             uint64           `json:"total_faults"`
	Windows                 []*windowMetrics `json:"windows"`
}

// deepCopy creates a deep copy of windowMetrics, properly copying all maps and slices.
func (wm *windowMetrics) deepCopy() *windowMetrics {
	if wm == nil {
		return nil
	}

	wmCopy := &windowMetrics{
		WindowIndex:               wm.WindowIndex,
		CoordinationBlock:         wm.CoordinationBlock,
		StartTime:                 wm.StartTime,
		EndTime:                   wm.EndTime,
		Duration:                  wm.Duration,
		ActivePhaseEndBlock:       wm.ActivePhaseEndBlock,
		EndBlock:                  wm.EndBlock,
		WalletsCoordinated:        wm.WalletsCoordinated,
		WalletsSuccessful:         wm.WalletsSuccessful,
		WalletsFailed:             wm.WalletsFailed,
		TotalProceduresStarted:    wm.TotalProceduresStarted,
		TotalProceduresCompleted:  wm.TotalProceduresCompleted,
		TotalFaults:               wm.TotalFaults,
		Leaders:                   make(map[string]uint64, len(wm.Leaders)),
		ActionTypes:               make(map[string]uint64, len(wm.ActionTypes)),
		FaultsByType:              make(map[string]uint64, len(wm.FaultsByType)),
		FaultsByCulprit:           make(map[string]uint64, len(wm.FaultsByCulprit)),
		WalletCoordinationDetails: make([]walletCoordinationDetail, len(wm.WalletCoordinationDetails)),
	}

	// Deep copy maps
	for k, v := range wm.Leaders {
		wmCopy.Leaders[k] = v
	}
	for k, v := range wm.ActionTypes {
		wmCopy.ActionTypes[k] = v
	}
	for k, v := range wm.FaultsByType {
		wmCopy.FaultsByType[k] = v
	}
	for k, v := range wm.FaultsByCulprit {
		wmCopy.FaultsByCulprit[k] = v
	}

	// Deep copy slice
	copy(wmCopy.WalletCoordinationDetails, wm.WalletCoordinationDetails)

	return wmCopy
}

// String returns a string representation of window metrics for logging.
func (wm *windowMetrics) String() string {
	return fmt.Sprintf(
		"window[%d] block[%d] wallets[%d/%d/%d] faults[%d] actions[%v]",
		wm.WindowIndex,
		wm.CoordinationBlock,
		wm.WalletsSuccessful,
		wm.WalletsFailed,
		wm.WalletsCoordinated,
		wm.TotalFaults,
		wm.ActionTypes,
	)
}
