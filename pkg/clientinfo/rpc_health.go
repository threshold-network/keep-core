package clientinfo

import (
	"context"
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/ipfs/go-log"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
)

var rpcHealthLogger = log.Logger("keep-rpc-health")

// benignBtcHeaderErrorPattern matches the JSON-RPC error code -32602
// ("invalid params") returned by some Electrum servers that reject the
// `cp_height` parameter of `blockchain.block.header`. The condition is
// known-benign: the endpoint is reachable and serving data
// (GetLatestBlockHeight already succeeded in the same check), the server
// just does not support that parameter. Such errors are counted and logged
// at debug level instead of warning to avoid flooding logs on every
// health-check tick.
//
// The underlying JSON-RPC error type is not exported by the Electrum client
// library, so the code is matched against the library's serialized error
// form (`errNo: %d, errMsg: %s`). The match is anchored to the `errNo`
// field with token boundaries so adjacent codes (e.g. -326020) or the
// digits appearing elsewhere in an unrelated message do not match.
var benignBtcHeaderErrorPattern = regexp.MustCompile(`\berrNo: -32602\b`)

// isKnownBenignBtcHeaderError returns true if the GetBlockHeader health-check
// error is the known-benign JSON-RPC -32602 ("invalid params") response.
func isKnownBenignBtcHeaderError(err error) bool {
	return err != nil && benignBtcHeaderErrorPattern.MatchString(err.Error())
}

// RPCHealthChecker performs periodic health checks on Ethereum and Bitcoin RPC endpoints
// by making actual RPC calls (not just ICMP ping) to verify the services are working.
type RPCHealthChecker struct {
	registry *Registry

	// Ethereum health check
	ethBlockCounter chain.BlockCounter
	ethLastCheck    time.Time
	ethLastSuccess  time.Time
	ethLastError    error
	ethLastDuration time.Duration // Last successful RPC call duration
	ethMutex        sync.RWMutex

	// Bitcoin health check
	btcChain        bitcoin.Chain
	btcLastCheck    time.Time
	btcLastSuccess  time.Time
	btcLastError    error
	btcLastDuration time.Duration // Last successful RPC call duration
	// Count of known-benign -32602 GetBlockHeader errors observed. Kept as
	// a counter so the condition remains observable through metrics even
	// though it is no longer logged at warning level.
	btcBenignHeaderErrors float64
	btcMutex              sync.RWMutex

	// Configuration
	checkInterval time.Duration

	// Concurrency control
	startOnce sync.Once
}

// NewRPCHealthChecker creates a new RPC health checker instance.
func NewRPCHealthChecker(
	registry *Registry,
	ethBlockCounter chain.BlockCounter,
	btcChain bitcoin.Chain,
	checkInterval time.Duration,
) *RPCHealthChecker {
	if checkInterval == 0 {
		checkInterval = 30 * time.Second // Default: check every 30 seconds
	}

	return &RPCHealthChecker{
		registry:        registry,
		ethBlockCounter: ethBlockCounter,
		btcChain:        btcChain,
		checkInterval:   checkInterval,
	}
}

// Start begins periodic health checks for both Ethereum and Bitcoin RPC endpoints.
// Safe to call multiple times - only the first call will execute.
func (r *RPCHealthChecker) Start(ctx context.Context) {
	r.startOnce.Do(func() {
		r.start(ctx)
	})
}

// start is the internal implementation of Start. Use Start() for public API.
func (r *RPCHealthChecker) start(ctx context.Context) {
	// Perform initial health checks immediately
	r.checkEthereumHealth(ctx)
	r.checkBitcoinHealth(ctx)

	// Start periodic health checks
	go r.runEthereumHealthChecks(ctx)
	go r.runBitcoinHealthChecks(ctx)

	// Register metrics observers
	r.registerMetrics()
}

// runEthereumHealthChecks runs periodic Ethereum RPC health checks.
func (r *RPCHealthChecker) runEthereumHealthChecks(ctx context.Context) {
	ticker := time.NewTicker(r.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.checkEthereumHealth(ctx)
		case <-ctx.Done():
			return
		}
	}
}

// runBitcoinHealthChecks runs periodic Bitcoin RPC health checks.
func (r *RPCHealthChecker) runBitcoinHealthChecks(ctx context.Context) {
	ticker := time.NewTicker(r.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.checkBitcoinHealth(ctx)
		case <-ctx.Done():
			return
		}
	}
}

// checkEthereumHealth performs a comprehensive health check on the Ethereum RPC endpoint
// by making actual RPC calls to verify the service is working properly.
// It checks:
// 1. Current block number retrieval
// 2. Block number is reasonable (not stuck at 0 or extremely old)
func (r *RPCHealthChecker) checkEthereumHealth(ctx context.Context) {
	if r.ethBlockCounter == nil {
		return
	}

	startTime := time.Now()

	// First check: Get current block number
	currentBlock, err := r.ethBlockCounter.CurrentBlock()
	if err != nil {
		r.ethMutex.Lock()
		r.ethLastCheck = startTime
		r.ethLastError = err
		r.ethMutex.Unlock()
		rpcHealthLogger.Warnf(
			"Ethereum RPC health check failed (CurrentBlock): [%v] (duration: %v)",
			err,
			time.Since(startTime),
		)
		return
	}

	// Second check: Verify block number is reasonable
	// Block number should be > 0 (unless on a very new testnet)
	// For mainnet/testnet, block numbers should be in thousands/millions
	if currentBlock == 0 {
		blockErr := fmt.Errorf("block number is 0, node may not be synced")
		r.ethMutex.Lock()
		r.ethLastCheck = startTime
		r.ethLastError = blockErr
		r.ethMutex.Unlock()
		rpcHealthLogger.Warnf(
			"Ethereum RPC health check failed (block number is 0): [%v] (duration: %v)",
			blockErr,
			time.Since(startTime),
		)
		return
	}

	duration := time.Since(startTime)

	r.ethMutex.Lock()
	r.ethLastCheck = startTime
	r.ethLastSuccess = time.Now()
	r.ethLastError = nil
	r.ethLastDuration = duration
	r.ethMutex.Unlock()

	rpcHealthLogger.Debugf(
		"Ethereum RPC health check succeeded (block: %d, duration: %v)",
		currentBlock,
		duration,
	)
}

// checkBitcoinHealth performs a comprehensive health check on the Bitcoin RPC endpoint
// by making actual RPC calls to verify the service is working properly.
// It checks:
// 1. Latest block height retrieval
// 2. Block header retrieval for the latest block (verifies RPC can retrieve block data)
// 3. Block height is reasonable (not 0)
func (r *RPCHealthChecker) checkBitcoinHealth(ctx context.Context) {
	if r.btcChain == nil {
		return
	}

	startTime := time.Now()

	// First check: Get latest block height
	latestHeight, err := r.btcChain.GetLatestBlockHeight()
	if err != nil {
		r.btcMutex.Lock()
		r.btcLastCheck = startTime
		r.btcLastError = err
		r.btcMutex.Unlock()
		rpcHealthLogger.Warnf(
			"Bitcoin RPC health check failed (GetLatestBlockHeight): [%v] (duration: %v)",
			err,
			time.Since(startTime),
		)
		return
	}

	// Second check: Verify block height is reasonable
	if latestHeight == 0 {
		heightErr := fmt.Errorf("block height is 0, node may not be synced")
		r.btcMutex.Lock()
		r.btcLastCheck = startTime
		r.btcLastError = heightErr
		r.btcMutex.Unlock()
		rpcHealthLogger.Warnf(
			"Bitcoin RPC health check failed (block height is 0): [%v] (duration: %v)",
			heightErr,
			time.Since(startTime),
		)
		return
	}

	// Third check: Try to get block header for the latest block
	// This verifies the RPC can actually retrieve block data, not just return a number
	_, err = r.btcChain.GetBlockHeader(latestHeight)
	if err != nil {
		headerErr := fmt.Errorf("failed to get block header for height %d: %w", latestHeight, err)
		benign := isKnownBenignBtcHeaderError(err)
		r.btcMutex.Lock()
		r.btcLastCheck = startTime
		r.btcLastError = headerErr
		if benign {
			r.btcBenignHeaderErrors++
		}
		r.btcMutex.Unlock()
		if benign {
			rpcHealthLogger.Debugf(
				"Bitcoin RPC health check failed (GetBlockHeader) with "+
					"known-benign -32602 response (server does not support "+
					"the cp_height parameter): [%v] (duration: %v)",
				headerErr,
				time.Since(startTime),
			)
		} else {
			rpcHealthLogger.Warnf(
				"Bitcoin RPC health check failed (GetBlockHeader): [%v] (duration: %v)",
				headerErr,
				time.Since(startTime),
			)
		}
		return
	}

	duration := time.Since(startTime)

	r.btcMutex.Lock()
	r.btcLastCheck = startTime
	r.btcLastSuccess = time.Now()
	r.btcLastError = nil
	r.btcLastDuration = duration
	r.btcMutex.Unlock()

	rpcHealthLogger.Debugf(
		"Bitcoin RPC health check succeeded (height: %d, duration: %v)",
		latestHeight,
		duration,
	)
}

// GetEthereumHealthStatus returns the current Ethereum RPC health status.
func (r *RPCHealthChecker) GetEthereumHealthStatus() (isHealthy bool, lastCheck time.Time, lastSuccess time.Time, lastError error, lastDuration time.Duration) {
	r.ethMutex.RLock()
	defer r.ethMutex.RUnlock()

	isHealthy = r.ethLastError == nil && !r.ethLastCheck.IsZero()
	return isHealthy, r.ethLastCheck, r.ethLastSuccess, r.ethLastError, r.ethLastDuration
}

// GetBitcoinHealthStatus returns the current Bitcoin RPC health status.
func (r *RPCHealthChecker) GetBitcoinHealthStatus() (isHealthy bool, lastCheck time.Time, lastSuccess time.Time, lastError error, lastDuration time.Duration) {
	r.btcMutex.RLock()
	defer r.btcMutex.RUnlock()

	isHealthy = r.btcLastError == nil && !r.btcLastCheck.IsZero()
	return isHealthy, r.btcLastCheck, r.btcLastSuccess, r.btcLastError, r.btcLastDuration
}

// GetBitcoinBenignHeaderErrorCount returns the number of known-benign -32602
// GetBlockHeader errors observed by the Bitcoin health check.
func (r *RPCHealthChecker) GetBitcoinBenignHeaderErrorCount() float64 {
	r.btcMutex.RLock()
	defer r.btcMutex.RUnlock()

	return r.btcBenignHeaderErrors
}

// registerMetrics registers metrics observers for RPC health status.
func (r *RPCHealthChecker) registerMetrics() {
	// Ethereum RPC response time
	r.registry.ObserveApplicationSource(
		"performance",
		map[string]Source{
			"rpc_eth_response_time_seconds": func() float64 {
				_, _, _, _, lastDuration := r.GetEthereumHealthStatus()
				return lastDuration.Seconds()
			},
		},
	)

	// Bitcoin RPC response time
	r.registry.ObserveApplicationSource(
		"performance",
		map[string]Source{
			"rpc_btc_response_time_seconds": func() float64 {
				_, _, _, _, lastDuration := r.GetBitcoinHealthStatus()
				return lastDuration.Seconds()
			},
		},
	)

	// Known-benign -32602 GetBlockHeader errors. Kept observable as a
	// counter since they are no longer logged at warning level.
	r.registry.ObserveApplicationSource(
		"performance",
		map[string]Source{
			"rpc_btc_health_check_benign_errors_total": func() float64 {
				return r.GetBitcoinBenignHeaderErrorCount()
			},
		},
	)
}
