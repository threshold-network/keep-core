package tbtc

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/exp/slices"

	"github.com/ipfs/go-log/v2"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/clientinfo"
	"github.com/keep-network/keep-core/pkg/crypto/secp256k1"
	"github.com/keep-network/keep-core/pkg/frost"
	"github.com/keep-network/keep-core/pkg/protocol/group"
	"github.com/keep-network/keep-core/pkg/tecdsa"
	"go.uber.org/zap"
)

type unsignedTransactionInputReference struct {
	TxIDHex string
	Vout    uint32
}

// WalletActionType represents actions types that can be performed by a wallet.
type WalletActionType uint8

const (
	ActionNoop WalletActionType = iota
	ActionHeartbeat
	ActionDepositSweep
	ActionRedemption
	ActionMovingFunds
	ActionMovedFundsSweep
)

// ParseWalletActionType parses the given value into a WalletActionType.
func ParseWalletActionType(value uint8) (WalletActionType, error) {
	switch value {
	case 0:
		return ActionNoop, nil
	case 1:
		return ActionHeartbeat, nil
	case 2:
		return ActionDepositSweep, nil
	case 3:
		return ActionRedemption, nil
	case 4:
		return ActionMovingFunds, nil
	case 5:
		return ActionMovedFundsSweep, nil
	default:
		return 0, fmt.Errorf("unknown wallet action type [%v]", value)
	}
}

func (wat WalletActionType) String() string {
	switch wat {
	case ActionNoop:
		return "Noop"
	case ActionHeartbeat:
		return "Heartbeat"
	case ActionDepositSweep:
		return "DepositSweep"
	case ActionRedemption:
		return "Redemption"
	case ActionMovingFunds:
		return "MovingFunds"
	case ActionMovedFundsSweep:
		return "MovedFundsSweep"
	default:
		panic("unknown wallet action type")
	}
}

// MetricName returns the metric name format for this action type (lowercase with underscores).
// This is used for generating per-action metric names.
func (wat WalletActionType) MetricName() string {
	switch wat {
	case ActionNoop:
		return "noop"
	case ActionHeartbeat:
		return "heartbeat"
	case ActionDepositSweep:
		return "deposit_sweep"
	case ActionRedemption:
		return "redemption"
	case ActionMovingFunds:
		return "moving_funds"
	case ActionMovedFundsSweep:
		return "moved_funds_sweep"
	default:
		panic("unknown wallet action type")
	}
}

// walletAction represents an action that can be performed by the wallet.
type walletAction interface {
	// execute carries out the walletAction until completion.
	execute() error

	// wallet returns the wallet the walletAction is bound to.
	wallet() wallet

	// actionType returns the specific type of the walletAction.
	actionType() WalletActionType
}

// WalletState represents the state of a wallet.
type WalletState uint8

const (
	StateUnknown WalletState = iota
	StateLive
	StateMovingFunds
	StateClosing
	StateClosed
	StateTerminated
)

func (ws WalletState) String() string {
	switch ws {
	case StateUnknown:
		return "Unknown"
	case StateLive:
		return "Live"
	case StateMovingFunds:
		return "MovingFunds"
	case StateClosing:
		return "Closing"
	case StateClosed:
		return "Closed"
	case StateTerminated:
		return "Terminated"
	default:
		panic("unknown wallet state")
	}
}

// errWalletBusy is an error returned when the waller cannot execute the
// requested walletAction due to an ongoing work.
var errWalletBusy = fmt.Errorf("wallet is busy")

// walletDispatcher is a component responsible for dispatching wallet actions
// to specific wallets.
type walletDispatcher struct {
	actionsMutex sync.Mutex
	// actions is the mapping holding the currently executed action of the
	// given wallet. The mapping key is the uncompressed public key
	// (with 04 prefix) of the wallet.
	actions map[string]WalletActionType
	// metricsRecorderMutex protects concurrent access to metricsRecorder
	metricsRecorderMutex sync.RWMutex
	// metricsRecorder is optional and used for recording performance metrics
	metricsRecorder interface {
		IncrementCounter(name string, value float64)
		SetGauge(name string, value float64)
		RecordDuration(name string, duration time.Duration)
	}
}

func newWalletDispatcher() *walletDispatcher {
	return &walletDispatcher{
		actions: make(map[string]WalletActionType),
	}
}

// setMetricsRecorder sets the metrics recorder for the wallet dispatcher.
func (wd *walletDispatcher) setMetricsRecorder(recorder interface {
	IncrementCounter(name string, value float64)
	SetGauge(name string, value float64)
	RecordDuration(name string, duration time.Duration)
}) {
	wd.metricsRecorderMutex.Lock()
	defer wd.metricsRecorderMutex.Unlock()
	wd.metricsRecorder = recorder
}

// dispatch sends the given walletAction for execution. If the wallet is
// already busy, an errWalletBusy error is returned and the action is ignored.
func (wd *walletDispatcher) dispatch(action walletAction) error {
	wd.actionsMutex.Lock()
	defer wd.actionsMutex.Unlock()

	walletPublicKeyBytes, err := marshalPublicKey(action.wallet().publicKey)
	if err != nil {
		return fmt.Errorf("cannot marshal wallet public key: [%v]", err)
	}

	walletActionLogger := logger.With(
		zap.String("wallet", fmt.Sprintf("0x%x", walletPublicKeyBytes)),
		zap.String("action", action.actionType().String()),
	)

	key := hex.EncodeToString(walletPublicKeyBytes)

	if _, ok := wd.actions[key]; ok {
		wd.metricsRecorderMutex.RLock()
		if wd.metricsRecorder != nil {
			wd.metricsRecorder.IncrementCounter(clientinfo.MetricWalletDispatcherRejectedTotal, 1)
		}
		wd.metricsRecorderMutex.RUnlock()
		return errWalletBusy
	}

	actionType := action.actionType()
	wd.actions[key] = actionType
	actionMetricName := actionType.MetricName()

	// Update metrics
	wd.metricsRecorderMutex.RLock()
	if wd.metricsRecorder != nil {
		activeCount := float64(len(wd.actions))
		wd.metricsRecorder.SetGauge(clientinfo.MetricWalletDispatcherActiveActions, activeCount)
		// Aggregate metrics (for backward compatibility)
		wd.metricsRecorder.IncrementCounter(clientinfo.MetricWalletActionsTotal, 1)
		// Per-action metrics
		wd.metricsRecorder.IncrementCounter(clientinfo.WalletActionMetricName(actionMetricName, "total"), 1)
	}
	wd.metricsRecorderMutex.RUnlock()

	go func() {
		startTime := time.Now()
		defer func() {
			wd.actionsMutex.Lock()
			delete(wd.actions, key)
			activeCount := float64(len(wd.actions))
			wd.actionsMutex.Unlock()

			// Update metrics
			wd.metricsRecorderMutex.RLock()
			if wd.metricsRecorder != nil {
				wd.metricsRecorder.SetGauge(clientinfo.MetricWalletDispatcherActiveActions, activeCount)
				duration := time.Since(startTime)
				// Aggregate metrics (for backward compatibility)
				wd.metricsRecorder.RecordDuration(clientinfo.MetricWalletActionDurationSeconds, duration)
				// Per-action metrics
				wd.metricsRecorder.RecordDuration(clientinfo.WalletActionMetricName(actionMetricName, "duration_seconds"), duration)
			}
			wd.metricsRecorderMutex.RUnlock()
		}()

		walletActionLogger.Infof("starting action execution")

		err := action.execute()
		if err != nil {
			walletActionLogger.Errorf(
				"action execution terminated with error: [%v]",
				err,
			)
			wd.metricsRecorderMutex.RLock()
			if wd.metricsRecorder != nil {
				// Aggregate metrics (for backward compatibility)
				wd.metricsRecorder.IncrementCounter(clientinfo.MetricWalletActionFailedTotal, 1)
				// Per-action metrics
				wd.metricsRecorder.IncrementCounter(clientinfo.WalletActionMetricName(actionMetricName, "failed_total"), 1)
			}
			wd.metricsRecorderMutex.RUnlock()
			return
		}

		wd.metricsRecorderMutex.RLock()
		if wd.metricsRecorder != nil {
			// Aggregate metrics (for backward compatibility)
			wd.metricsRecorder.IncrementCounter(clientinfo.MetricWalletActionSuccessTotal, 1)
			// Per-action metrics
			wd.metricsRecorder.IncrementCounter(clientinfo.WalletActionMetricName(actionMetricName, "success_total"), 1)
		}
		wd.metricsRecorderMutex.RUnlock()

		walletActionLogger.Infof("action execution terminated with success")
	}()

	return nil
}

// walletSigningExecutor is an interface meant to decouple the specific
// implementation of the signing executor from the wallet transaction executor.
type walletSigningExecutor interface {
	signBatch(
		ctx context.Context,
		messages []*big.Int,
		startBlock uint64,
	) ([]*frost.Signature, error)
}

type schnorrWalletSigningExecutor interface {
	usesSchnorrSignatures() bool
}

type taprootTweakedWalletSigningExecutor interface {
	signBatchWithTaprootMerkleRoots(
		ctx context.Context,
		messages []*big.Int,
		taprootMerkleRoots []*[32]byte,
		startBlock uint64,
	) ([]*frost.Signature, error)
}

type taprootPolicyBoundWalletSigningExecutor interface {
	signBatchWithTaprootTransaction(
		ctx context.Context,
		messages []*big.Int,
		taprootMerkleRoots []*[32]byte,
		startBlock uint64,
		unsignedTx *bitcoin.TransactionBuilder,
	) ([]*frost.Signature, error)
}

type authorizedTaprootPolicyBoundWalletSigningExecutor interface {
	signBatchWithAuthorizedTaprootTransaction(
		ctx context.Context,
		messages []*big.Int,
		taprootMerkleRoots []*[32]byte,
		startBlock uint64,
		unsignedTx *bitcoin.TransactionBuilder,
		authorizationID [32]byte,
		authorizationGuard func(context.Context) error,
	) ([]*frost.Signature, error)
}

type frostTransactionSafetyProvider interface {
	frostPreSignGate() frostPreSignAuthorizationGate
	bitcoinOutbox() *bitcoinBroadcastOutbox
}

// walletTransactionExecutor is a component allowing to sign and broadcast
// wallet Bitcoin transactions.
type walletTransactionExecutor struct {
	btcChain bitcoin.Chain

	executingWallet wallet
	signingExecutor walletSigningExecutor

	waitForBlockFn waitForBlockFn

	action                            WalletActionType
	frostPreSignActionContext         *FrostPreSignActionContext
	preSignAuthorizationGate          frostPreSignAuthorizationGate
	broadcastOutbox                   *bitcoinBroadcastOutbox
	frostAuthorizationMonitorInterval time.Duration
}

const defaultFrostAuthorizationMonitorInterval = time.Second

// frostPreSignAuthorizationMonitor keeps the pinned Ethereum authorization
// live for the entire native signing window. All explicit nonce/share guards
// and the periodic monitor serialize through validationMutex so a backend never
// observes overlapping reads for one authorization. Production revalidation
// polls the current finalized point on every pass but reuses the authorization's
// cached signer-readiness reconciliation while that exact point is unchanged.
type frostPreSignAuthorizationMonitor struct {
	gate          frostPreSignAuthorizationGate
	authorization *frostPreSignAuthorization
	ctx           context.Context
	cancel        context.CancelFunc
	interval      time.Duration

	validationMutex  sync.Mutex
	errorMutex       sync.Mutex
	authorizationErr error
	done             chan struct{}
}

func newFrostPreSignAuthorizationMonitor(
	parent context.Context,
	gate frostPreSignAuthorizationGate,
	authorization *frostPreSignAuthorization,
	interval time.Duration,
) *frostPreSignAuthorizationMonitor {
	if interval <= 0 {
		interval = defaultFrostAuthorizationMonitorInterval
	}
	ctx, cancel := context.WithCancel(parent)
	monitor := &frostPreSignAuthorizationMonitor{
		gate:          gate,
		authorization: authorization,
		ctx:           ctx,
		cancel:        cancel,
		interval:      interval,
		done:          make(chan struct{}),
	}
	go monitor.run()
	return monitor
}

func (fpsam *frostPreSignAuthorizationMonitor) run() {
	defer close(fpsam.done)
	ticker := time.NewTicker(fpsam.interval)
	defer ticker.Stop()
	for {
		select {
		case <-fpsam.ctx.Done():
			return
		case <-ticker.C:
			_ = fpsam.revalidate(fpsam.ctx)
		}
	}
}

func (fpsam *frostPreSignAuthorizationMonitor) revalidate(
	ctx context.Context,
) error {
	if err := fpsam.err(); err != nil {
		return err
	}
	fpsam.validationMutex.Lock()
	defer fpsam.validationMutex.Unlock()
	if err := fpsam.err(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := fpsam.gate.revalidate(ctx, fpsam.authorization); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		authorizationErr := fmt.Errorf(
			"FROST finalized authorization changed during native signing: [%w]",
			err,
		)
		fpsam.errorMutex.Lock()
		if fpsam.authorizationErr == nil {
			fpsam.authorizationErr = authorizationErr
		}
		authorizationErr = fpsam.authorizationErr
		fpsam.errorMutex.Unlock()
		fpsam.cancel()
		return authorizationErr
	}
	return nil
}

func (fpsam *frostPreSignAuthorizationMonitor) validateNow() error {
	return fpsam.revalidate(fpsam.ctx)
}

func (fpsam *frostPreSignAuthorizationMonitor) guard(
	ctx context.Context,
) error {
	return fpsam.revalidate(ctx)
}

func (fpsam *frostPreSignAuthorizationMonitor) err() error {
	fpsam.errorMutex.Lock()
	defer fpsam.errorMutex.Unlock()
	return fpsam.authorizationErr
}

func (fpsam *frostPreSignAuthorizationMonitor) stop() {
	fpsam.cancel()
	<-fpsam.done
}

var buildTaprootTxViaNativeSignerFn = buildTaprootTxViaNativeSigner
var nativeBuildTaprootTxSigningSubstitutionEnabledFn = nativeBuildTaprootTxSigningSubstitutionEnabled

const nativeBuildTaprootTxSigningSubstitutionEnvVar = "KEEP_CORE_NATIVE_BUILDTX_SIGNING_SUBSTITUTION"

func newWalletTransactionExecutor(
	btcChain bitcoin.Chain,
	executingWallet wallet,
	signingExecutor walletSigningExecutor,
	waitForBlockFn waitForBlockFn,
) *walletTransactionExecutor {
	executor := &walletTransactionExecutor{
		btcChain:        btcChain,
		executingWallet: executingWallet,
		signingExecutor: signingExecutor,
		waitForBlockFn:  waitForBlockFn,
	}
	if provider, ok := signingExecutor.(frostTransactionSafetyProvider); ok {
		executor.preSignAuthorizationGate = provider.frostPreSignGate()
		executor.broadcastOutbox = provider.bitcoinOutbox()
	}
	return executor
}

// signTransaction performs signing of an unsigned Bitcoin transaction
// and returns a signed transaction ready to be broadcasted over the
// Bitcoin network.
func (wte *walletTransactionExecutor) signTransaction(
	signTxLogger log.StandardLogger,
	unsignedTx *bitcoin.TransactionBuilder,
	signingStartBlock uint64,
	signingTimeoutBlock uint64,
) (*bitcoin.Transaction, error) {
	usesSchnorrSignatures := wte.usesSchnorrSignatures()

	// The native tbtc-signer BuildTaprootTx parity/substitution path applies
	// only to Taproot transactions that do not use the mandatory policy-bound
	// Schnorr path below. That path calls BuildTaprootTx under the exact ROAST
	// session before every signature; running this preflight first under a
	// buildtx-* session would consume one extra token from the signer's global
	// policy rate limiter, and its artifact cannot authorize the ROAST session.
	// The native builder is meaningless for legacy transactions, so those skip
	// this path as before.
	if unsignedTx.HasOnlyTaprootKeyPathInputs() && !usesSchnorrSignatures {
		if err := wte.maybeSubstituteNativeBuildTaprootTx(
			signTxLogger,
			unsignedTx,
		); err != nil {
			return nil, err
		}
	}

	signTxLogger.Infof("computing transaction's sig hashes")

	sigHashes, err := unsignedTx.ComputeSignatureHashes()
	if err != nil {
		return nil, fmt.Errorf(
			"error while computing transaction's sig hashes: [%v]",
			err,
		)
	}

	if unsignedTx.HasTaprootKeyPathInputs() &&
		!unsignedTx.HasOnlyTaprootKeyPathInputs() {
		return nil, fmt.Errorf(
			"cannot apply FROST signatures to mixed taproot and legacy inputs",
		)
	}

	if usesSchnorrSignatures &&
		!unsignedTx.HasOnlyTaprootKeyPathInputs() {
		return nil, fmt.Errorf(
			"cannot apply FROST signatures to non-taproot transaction inputs",
		)
	}

	var signatures []*frost.Signature
	var authorization *frostPreSignAuthorization
	var authorizationMonitor *frostPreSignAuthorizationMonitor
	effectiveSigningStartBlock := signingStartBlock
	taprootMerkleRoots := unsignedTx.TaprootKeyPathInputMerkleRoots()

	if usesSchnorrSignatures {
		authorizedSigningExecutor, ok :=
			wte.signingExecutor.(authorizedTaprootPolicyBoundWalletSigningExecutor)
		if !ok {
			return nil, fmt.Errorf(
				"Schnorr signing executor does not support finalized transaction authorization binding",
			)
		}
		if wte.preSignAuthorizationGate == nil {
			return nil, fmt.Errorf(
				"FROST signing is disabled: pre-sign authorization gate is unavailable",
			)
		}
		if wte.broadcastOutbox == nil {
			return nil, fmt.Errorf(
				"FROST signing is disabled: durable Bitcoin broadcast outbox is unavailable",
			)
		}

		preSignTransaction, buildErr := newFrostPreSignTransaction(
			wte.action,
			bitcoin.PublicKeyHash(wte.executingWallet.publicKey),
			unsignedTx,
			sigHashes,
		)
		if buildErr != nil {
			return nil, fmt.Errorf("cannot build FROST pre-sign proposal: [%w]", buildErr)
		}
		preSignTransaction.ActionContext = cloneFrostPreSignActionContext(
			wte.frostPreSignActionContext,
		)

		authorizationCtx, cancelAuthorizationCtx := withCancelOnBlock(
			context.Background(),
			signingTimeoutBlock,
			wte.waitForBlockFn,
		)
		authorization, err = wte.preSignAuthorizationGate.authorize(
			authorizationCtx,
			preSignTransaction,
		)
		if authorization != nil {
			defer authorization.releaseAnchorReservation()
		}
		if err == nil {
			err = wte.preSignAuthorizationGate.revalidate(
				authorizationCtx,
				authorization,
			)
		}
		if err == nil {
			err = authorizationCtx.Err()
		}
		cancelAuthorizationCtx()
		if err != nil {
			return nil, fmt.Errorf(
				"FROST pre-sign authorization failed before nonce generation: [%w]",
				err,
			)
		}

		// The relay finality point is part of the finalized authorization all
		// signers revalidate. Use it as the common protocol epoch for both ROAST
		// session derivation and retry scheduling. A local head observed after
		// authorization may differ between honest operators and would partition
		// them into permanently offset signing sessions. The retry loop already
		// skips elapsed windows when finality confirmation puts this epoch in the
		// past.
		effectiveSigningStartBlock = authorization.Finality.BlockNumber
		if effectiveSigningStartBlock >= signingTimeoutBlock {
			return nil, fmt.Errorf(
				"FROST authorization finalized at/after signing timeout block [%d]",
				signingTimeoutBlock,
			)
		}

		// Create a fresh absolute-timeout context after finality. Reusing the
		// pre-authorization session start can make ROAST windows stale before the
		// first native bind/nonce operation.
		signingCtx, cancelSigningCtx := withCancelOnBlock(
			context.Background(),
			signingTimeoutBlock,
			wte.waitForBlockFn,
		)
		defer cancelSigningCtx()
		if err := wte.preSignAuthorizationGate.revalidate(
			signingCtx,
			authorization,
		); err != nil {
			return nil, fmt.Errorf(
				"FROST finalized authorization changed before native signing: [%w]",
				err,
			)
		}
		authorizationMonitor = newFrostPreSignAuthorizationMonitor(
			signingCtx,
			wte.preSignAuthorizationGate,
			authorization,
			wte.frostAuthorizationMonitorInterval,
		)
		defer authorizationMonitor.stop()

		signTxLogger.Infof("signing transaction's sig hashes")
		signatures, err = authorizedSigningExecutor.signBatchWithAuthorizedTaprootTransaction(
			authorizationMonitor.ctx,
			sigHashes,
			taprootMerkleRoots,
			effectiveSigningStartBlock,
			unsignedTx,
			authorization.AuthorizationID,
			authorizationMonitor.guard,
		)
		if authorizationErr := authorizationMonitor.err(); authorizationErr != nil {
			err = authorizationErr
		} else if err == nil {
			err = authorizationMonitor.validateNow()
		}
	} else {
		signTxLogger.Infof("signing transaction's sig hashes")
		signingCtx, cancelSigningCtx := withCancelOnBlock(
			context.Background(),
			signingTimeoutBlock,
			wte.waitForBlockFn,
		)
		defer cancelSigningCtx()

		if hasTaprootMerkleRoots(taprootMerkleRoots) {
			tweakedSigningExecutor, ok := wte.signingExecutor.(taprootTweakedWalletSigningExecutor)
			if !ok {
				return nil, fmt.Errorf(
					"taproot tweaked signing requires signer support",
				)
			}

			signatures, err = tweakedSigningExecutor.signBatchWithTaprootMerkleRoots(
				signingCtx,
				sigHashes,
				taprootMerkleRoots,
				effectiveSigningStartBlock,
			)
		} else {
			signatures, err = wte.signingExecutor.signBatch(
				signingCtx,
				sigHashes,
				effectiveSigningStartBlock,
			)
		}
	}
	if err != nil {
		return nil, fmt.Errorf(
			"error while signing transaction's sig hashes: [%v]",
			err,
		)
	}

	signTxLogger.Infof("applying transaction's signatures")

	if unsignedTx.HasTaprootKeyPathInputs() {
		containers := make(
			[]*bitcoin.SchnorrSignatureContainer,
			len(signatures),
		)
		for i, signature := range signatures {
			containers[i] = &bitcoin.SchnorrSignatureContainer{
				Signature: signature.Serialize(),
			}
		}

		tx, err := unsignedTx.AddTaprootKeyPathSignatures(containers)
		if err != nil {
			return nil, fmt.Errorf(
				"error while applying transaction's taproot key-path "+
					"signatures: [%v]",
				err,
			)
		}

		if usesSchnorrSignatures {
			if err := authorizationMonitor.validateNow(); err != nil {
				return nil, err
			}
			proposal := authorization.proposal
			if err := wte.broadcastOutbox.enqueue(
				tx,
				proposal.Transaction.WalletPublicKeyHash,
				proposal.WalletID,
				proposal.Transaction.Action,
				proposal.Transaction.TransactionHash,
				bitcoinBroadcastAuthorization{
					ActivationProfileHash:     authorization.ActivationProfileHash,
					AuthorizationID:           authorization.AuthorizationID,
					ReservationID:             authorization.ReservationID,
					AuthorizationRoot:         authorization.VariantRoot,
					SnapshotHash:              proposal.SnapshotHash,
					ResourceHash:              proposal.ResourceHash,
					OrderedInputRoot:          proposal.OrderedInputRoot,
					LockedPlanHash:            proposal.computeLockedPlanHash(),
					VariantApplyPlanHash:      proposal.ApplyPlanHash,
					FeeLimitSnapshot:          proposal.FeeLimitSnapshot,
					FinalizedBlock:            authorization.Finality.BlockNumber,
					FinalizedBlockHash:        authorization.Finality.BlockHash,
					FinalizedTransactionIndex: authorization.Finality.TransactionIndex,
					FinalizedLogIndex:         authorization.Finality.LogIndex,
					VariantSequence:           authorization.VariantSequence,
				},
			); err != nil {
				return nil, fmt.Errorf(
					"cannot durably enqueue signed FROST transaction: [%w]",
					err,
				)
			}
		}

		signTxLogger.Infof("transaction created successfully")
		return tx, nil
	}

	containers := make([]*bitcoin.SignatureContainer, len(signatures))
	for i, signature := range signatures {
		containers[i] = &bitcoin.SignatureContainer{
			R:         new(big.Int).SetBytes(signature.R[:]),
			S:         new(big.Int).SetBytes(signature.S[:]),
			PublicKey: wte.executingWallet.publicKey,
		}
	}

	tx, err := unsignedTx.AddSignatures(containers)
	if err != nil {
		return nil, fmt.Errorf(
			"error while applying transaction's signatures: [%v]",
			err,
		)
	}

	signTxLogger.Infof("transaction created successfully")

	return tx, nil
}

func (wte *walletTransactionExecutor) usesSchnorrSignatures() bool {
	executor, ok := wte.signingExecutor.(schnorrWalletSigningExecutor)
	if !ok {
		return false
	}

	return executor.usesSchnorrSignatures()
}

// maybeSubstituteNativeBuildTaprootTx runs the native tbtc-signer BuildTaprootTx
// parity/substitution path: it builds the unsigned transaction via the native
// signer and, when KEEP_CORE_NATIVE_BUILDTX_SIGNING_SUBSTITUTION is set,
// substitutes the Go-built transaction with the native one. In the default build
// the native builder is a no-op (returns an empty hex), so this is observational
// only. It must only be called for Taproot transactions (see signTransaction).
func (wte *walletTransactionExecutor) maybeSubstituteNativeBuildTaprootTx(
	signTxLogger log.StandardLogger,
	unsignedTx *bitcoin.TransactionBuilder,
) error {
	substitutionEnabled := nativeBuildTaprootTxSigningSubstitutionEnabledFn()

	nativeUnsignedTxHex, err := buildTaprootTxViaNativeSignerFn(unsignedTx)
	if err != nil {
		return fmt.Errorf(
			"error while building unsigned transaction with native tbtc-signer: [%w]",
			err,
		)
	}

	if nativeUnsignedTxHex == "" {
		return nil
	}

	signTxLogger.Debugf(
		"received unsigned transaction from native tbtc-signer BuildTaprootTx [txHexLen:%d]",
		len(nativeUnsignedTxHex),
	)

	nativeUnsignedTx, err := evaluateNativeUnsignedTransactionForSigning(
		signTxLogger,
		nativeUnsignedTxHex,
		unsignedTx.UnsignedTransaction(),
		substitutionEnabled,
	)
	if err != nil {
		return fmt.Errorf(
			"cannot process native BuildTaprootTx unsigned transaction for signing: [%v]",
			err,
		)
	}

	if nativeUnsignedTx != nil {
		if err := unsignedTx.ReplaceUnsignedTransaction(nativeUnsignedTx); err != nil {
			return fmt.Errorf(
				"cannot substitute Go unsigned transaction with native BuildTaprootTx output: [%v]",
				err,
			)
		}

		signTxLogger.Infof(
			"substituted Go unsigned transaction with native tbtc-signer BuildTaprootTx output",
		)
	}

	return nil
}

func hasTaprootMerkleRoots(taprootMerkleRoots []*[32]byte) bool {
	for _, merkleRoot := range taprootMerkleRoots {
		if merkleRoot != nil {
			return true
		}
	}

	return false
}

func nativeBuildTaprootTxSigningSubstitutionEnabled() bool {
	switch strings.ToLower(
		strings.TrimSpace(
			os.Getenv(nativeBuildTaprootTxSigningSubstitutionEnvVar),
		),
	) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func evaluateNativeUnsignedTransactionForSigning(
	signTxLogger log.StandardLogger,
	nativeUnsignedTxHex string,
	expectedTransaction *bitcoin.Transaction,
	substitutionEnabled bool,
) (*bitcoin.Transaction, error) {
	nativeUnsignedTx, err := decodeNativeUnsignedTransactionHex(nativeUnsignedTxHex)
	if err != nil {
		if substitutionEnabled {
			return nil, err
		}

		signTxLogger.Warnf(
			"cannot compare native BuildTaprootTx unsigned transaction with Go builder state: [%v]",
			err,
		)
		return nil, nil
	}

	diverges, divergenceReason, err := nativeUnsignedTransactionDivergesFromTransaction(
		nativeUnsignedTx,
		expectedTransaction,
	)
	if err != nil {
		if substitutionEnabled {
			return nil, err
		}

		signTxLogger.Warnf(
			"cannot compare native BuildTaprootTx unsigned transaction with Go builder state: [%v]",
			err,
		)
		return nil, nil
	}

	if diverges {
		divergenceMessage := "native BuildTaprootTx unsigned transaction diverges from Go builder state"
		if divergenceReason != "" {
			divergenceMessage = fmt.Sprintf(
				"%s: %s",
				divergenceMessage,
				divergenceReason,
			)
		}

		if substitutionEnabled {
			return nil, fmt.Errorf("%s", divergenceMessage)
		}

		signTxLogger.Warnf(divergenceMessage)
	}

	if substitutionEnabled {
		return nativeUnsignedTx, nil
	}

	return nil, nil
}

func decodeNativeUnsignedTransactionHex(
	nativeUnsignedTxHex string,
) (*bitcoin.Transaction, error) {
	nativeUnsignedTxBytes, err := hex.DecodeString(nativeUnsignedTxHex)
	if err != nil {
		return nil, fmt.Errorf("cannot decode native tx hex: [%w]", err)
	}

	nativeUnsignedTx := &bitcoin.Transaction{}
	if err := nativeUnsignedTx.Deserialize(nativeUnsignedTxBytes); err != nil {
		return nil, fmt.Errorf("cannot deserialize native tx bytes: [%w]", err)
	}

	return nativeUnsignedTx, nil
}

func nativeUnsignedTransactionDivergesFromTransaction(
	nativeUnsignedTx *bitcoin.Transaction,
	expectedTransaction *bitcoin.Transaction,
) (bool, string, error) {
	actualShape, err := extractUnsignedTransactionShapeFromTransaction(nativeUnsignedTx)
	if err != nil {
		return false, "", err
	}

	expectedShape, err := extractUnsignedTransactionShapeFromTransaction(expectedTransaction)
	if err != nil {
		return false, "", err
	}

	if actualShape.Version != expectedShape.Version {
		return true, fmt.Sprintf(
			"version mismatch: expected [%d], got [%d]",
			expectedShape.Version,
			actualShape.Version,
		), nil
	}

	if actualShape.Locktime != expectedShape.Locktime {
		return true, fmt.Sprintf(
			"locktime mismatch: expected [%d], got [%d]",
			expectedShape.Locktime,
			actualShape.Locktime,
		), nil
	}

	if reason, diverges := unsignedTransactionInputReferencesDivergenceReason(
		actualShape.InputReferences,
		expectedShape.InputReferences,
	); diverges {
		return true, reason, nil
	}

	if reason, diverges := unsignedTransactionInputSequencesDivergenceReason(
		actualShape.InputSequences,
		expectedShape.InputSequences,
	); diverges {
		return true, reason, nil
	}

	if reason, diverges := unsignedTransactionOutputsDivergenceReason(
		actualShape.Outputs,
		expectedShape.Outputs,
	); diverges {
		return true, reason, nil
	}

	return false, "", nil
}

func unsignedTransactionInputReferencesDivergenceReason(
	actual []unsignedTransactionInputReference,
	expected []unsignedTransactionInputReference,
) (string, bool) {
	if len(actual) != len(expected) {
		return fmt.Sprintf(
			"input reference count mismatch: expected [%d], got [%d]",
			len(expected),
			len(actual),
		), true
	}

	for i := range actual {
		if actual[i] != expected[i] {
			return fmt.Sprintf(
				"input reference mismatch at index [%d]: expected [%s:%d], got [%s:%d]",
				i,
				expected[i].TxIDHex,
				expected[i].Vout,
				actual[i].TxIDHex,
				actual[i].Vout,
			), true
		}
	}

	return "", false
}

type unsignedTransactionShape struct {
	Version         int32
	Locktime        uint32
	InputReferences []unsignedTransactionInputReference
	InputSequences  []uint32
	Outputs         []bitcoin.UnsignedTransactionOutput
}

func extractUnsignedTransactionShapeFromTransaction(
	transaction *bitcoin.Transaction,
) (*unsignedTransactionShape, error) {
	if transaction == nil {
		return nil, fmt.Errorf("transaction is nil")
	}

	inputReferences := make(
		[]unsignedTransactionInputReference,
		0,
		len(transaction.Inputs),
	)
	inputSequences := make([]uint32, 0, len(transaction.Inputs))
	for i, input := range transaction.Inputs {
		if input == nil {
			return nil, fmt.Errorf("transaction input [%d] is nil", i)
		}

		if input.Outpoint == nil {
			return nil, fmt.Errorf("transaction input [%d] outpoint is nil", i)
		}

		inputReferences = append(
			inputReferences,
			unsignedTransactionInputReference{
				TxIDHex: input.Outpoint.TransactionHash.Hex(bitcoin.ReversedByteOrder),
				Vout:    input.Outpoint.OutputIndex,
			},
		)
		inputSequences = append(inputSequences, input.Sequence)
	}

	outputs := make([]bitcoin.UnsignedTransactionOutput, 0, len(transaction.Outputs))
	for i, output := range transaction.Outputs {
		if output == nil {
			return nil, fmt.Errorf("transaction output [%d] is nil", i)
		}

		if output.Value < 0 {
			return nil, fmt.Errorf("transaction output [%d] value is negative", i)
		}

		outputs = append(
			outputs,
			bitcoin.UnsignedTransactionOutput{
				ScriptPubKeyHex: hex.EncodeToString(output.PublicKeyScript),
				ValueSats:       uint64(output.Value),
			},
		)
	}

	return &unsignedTransactionShape{
		Version:         transaction.Version,
		Locktime:        transaction.Locktime,
		InputReferences: inputReferences,
		InputSequences:  inputSequences,
		Outputs:         outputs,
	}, nil
}

func unsignedTransactionOutputsDivergenceReason(
	actual []bitcoin.UnsignedTransactionOutput,
	expected []bitcoin.UnsignedTransactionOutput,
) (string, bool) {
	if len(actual) != len(expected) {
		return fmt.Sprintf(
			"output count mismatch: expected [%d], got [%d]",
			len(expected),
			len(actual),
		), true
	}

	for i := range actual {
		if actual[i].ValueSats != expected[i].ValueSats {
			return fmt.Sprintf(
				"output value mismatch at index [%d]: expected [%d], got [%d]",
				i,
				expected[i].ValueSats,
				actual[i].ValueSats,
			), true
		}

		if actual[i].ScriptPubKeyHex != expected[i].ScriptPubKeyHex {
			return fmt.Sprintf(
				"output script mismatch at index [%d]: expected [%s], got [%s]",
				i,
				expected[i].ScriptPubKeyHex,
				actual[i].ScriptPubKeyHex,
			), true
		}
	}

	return "", false
}

func unsignedTransactionInputSequencesDivergenceReason(
	actual []uint32,
	expected []uint32,
) (string, bool) {
	if len(actual) != len(expected) {
		return fmt.Sprintf(
			"input sequence count mismatch: expected [%d], got [%d]",
			len(expected),
			len(actual),
		), true
	}

	for i := range actual {
		if actual[i] != expected[i] {
			return fmt.Sprintf(
				"input sequence mismatch at index [%d]: expected [%d], got [%d]",
				i,
				expected[i],
				actual[i],
			), true
		}
	}

	return "", false
}

// broadcastTransaction broadcasts a signed Bitcoin transaction until
// the transaction lands in the Bitcoin mempool or the provided timeout
// is hit, whichever comes first.
func (wte *walletTransactionExecutor) broadcastTransaction(
	broadcastTxLogger log.StandardLogger,
	tx *bitcoin.Transaction,
	timeout time.Duration,
	checkDelay time.Duration,
) error {
	txHash := tx.Hash()

	broadcastCtx, cancelBroadcastCtx := context.WithTimeout(
		context.Background(),
		timeout,
	)
	defer cancelBroadcastCtx()

	broadcastAttempt := 0

	for {
		select {
		case <-broadcastCtx.Done():
			return fmt.Errorf("broadcast timeout exceeded")
		default:
			broadcastAttempt++

			broadcastTxLogger.Infof(
				"broadcasting transaction on the Bitcoin chain - attempt [%v]",
				broadcastAttempt,
			)

			var err error
			if wte.usesSchnorrSignatures() {
				if wte.broadcastOutbox == nil {
					return fmt.Errorf(
						"FROST Bitcoin broadcast requires the durable authorized outbox",
					)
				}
				err = wte.broadcastOutbox.broadcastTransaction(
					broadcastCtx,
					txHash,
				)
			} else {
				err = wte.btcChain.BroadcastTransaction(tx)
			}
			if err != nil {
				broadcastTxLogger.Warnf(
					"broadcasting failed: [%v]; transaction could be "+
						"broadcasted by another wallet operators though",
					err,
				)
			} else {
				broadcastTxLogger.Infof("broadcasting completed")
			}

			broadcastTxLogger.Infof(
				"waiting [%v] before checking whether the "+
					"transaction is known on Bitcoin chain",
				checkDelay,
			)

			select {
			case <-time.After(checkDelay):
			case <-broadcastCtx.Done():
				return fmt.Errorf("broadcast timeout exceeded")
			}

			broadcastTxLogger.Infof(
				"checking whether the transaction is known on Bitcoin chain",
			)

			_, err = wte.btcChain.GetTransactionConfirmations(txHash)
			if err != nil {
				broadcastTxLogger.Warnf(
					"cannot say whether the transaction is known "+
						"on Bitcoin chain; check returned an error: [%v]",
					err,
				)
				continue
			}

			broadcastTxLogger.Infof("transaction is known on Bitcoin chain")
			return nil
		}
	}
}

// wallet represents a tBTC wallet. A wallet is one of the basic building
// blocks of the system that takes BTC under custody during the deposit
// process and gives that BTC back during redemptions.
type wallet struct {
	// publicKey is the unique ECDSA public key that identifies the
	// given wallet. This public key is also used to derive contract-specific
	// wallet identifiers (e.g. the Bridge contract identifies the wallet using
	// the SHA-256+RIPEMD-160 hash computed over the compressed ECDSA public key)
	publicKey *ecdsa.PublicKey
	// signingGroupOperators is the list holding operators' addresses that
	// form the whole wallet's signing group. This list may differ from the
	// original list outputted by the sortition protocol as it contains only
	// those signing group members who behaved properly during the DKG
	// protocol so all misbehaved members are not included here.
	// This list's size is always in the range [GroupQuorum, GroupSize].
	//
	// Each item in this list represents the given signing group member (seat)
	// and has a group.MemberIndex that is just the element's list index
	// incremented by one (e.g. element with index 0 has the group.MemberIndex
	// equal to 1 and so on).
	signingGroupOperators []chain.Address
}

// groupSize returns the actual size of the wallet's signing group. This
// value may be different from the GroupParameters.GroupSize parameter as some
// candidates may be excluded during distributed key generation.
func (w *wallet) groupSize() int {
	return len(w.signingGroupOperators)
}

// groupDishonestThreshold returns the dishonest threshold for the wallet's
// signing group. The returned value is computed using the wallet's actual
// signing group size for the given honest threshold provided as argument.
func (w *wallet) groupDishonestThreshold(honestThreshold int) int {
	return w.groupSize() - honestThreshold
}

// membersByOperator returns the list of group members' indexes that are
// associated with the given operator address. The returned list is sorted
// in ascending order.
func (w *wallet) membersByOperator(operator chain.Address) []group.MemberIndex {
	members := make([]group.MemberIndex, 0)

	for i, signingGroupOperator := range w.signingGroupOperators {
		if signingGroupOperator == operator {
			members = append(members, group.MemberIndex(i+1))
		}
	}

	slices.Sort(members)

	return members
}

func (w *wallet) String() string {
	publicKey := secp256k1.Marshal(w.publicKey)

	return fmt.Sprintf("public key [0x%x]", publicKey)
}

// DetermineWalletMainUtxo determines the plain-text wallet main UTXO
// currently registered in the Bridge on-chain contract. The returned
// main UTXO can be nil if the wallet does not have a main UTXO registered
// in the Bridge at the moment.
func DetermineWalletMainUtxo(
	walletPublicKeyHash [20]byte,
	bridgeChain BridgeChain,
	btcChain bitcoin.Chain,
) (*bitcoin.UnspentTransactionOutput, error) {
	walletScripts, err := legacyWalletPublicKeyScripts(walletPublicKeyHash)
	if err != nil {
		return nil, err
	}

	return determineWalletMainUtxo(
		walletPublicKeyHash,
		walletScripts,
		bridgeChain,
		btcChain,
	)
}

// DetermineWalletMainUtxoForPublicKey determines the plain-text wallet main
// UTXO currently registered in the Bridge on-chain contract. Unlike
// DetermineWalletMainUtxo, this variant can discover Taproot wallet outputs.
func DetermineWalletMainUtxoForPublicKey(
	walletPublicKey *ecdsa.PublicKey,
	bridgeChain BridgeChain,
	btcChain bitcoin.Chain,
) (*bitcoin.UnspentTransactionOutput, error) {
	walletPublicKeyHash := bitcoin.PublicKeyHash(walletPublicKey)

	walletScripts, err := walletPublicKeyScripts(walletPublicKey)
	if err != nil {
		return nil, err
	}

	return determineWalletMainUtxo(
		walletPublicKeyHash,
		walletScripts,
		bridgeChain,
		btcChain,
	)
}

func determineWalletMainUtxo(
	walletPublicKeyHash [20]byte,
	walletScripts []bitcoin.Script,
	bridgeChain BridgeChain,
	btcChain bitcoin.Chain,
) (*bitcoin.UnspentTransactionOutput, error) {
	walletChainData, err := bridgeChain.GetWallet(walletPublicKeyHash)
	if err != nil {
		return nil, fmt.Errorf("cannot get on-chain data for wallet: [%v]", err)
	}

	// Valid case when the wallet doesn't have a main UTXO registered into
	// the Bridge.
	if walletChainData.MainUtxoHash == [32]byte{} {
		return nil, nil
	}

	// The wallet main UTXO registered in the Bridge almost always comes
	// from the latest BTC transaction made by the wallet. However, there may
	// be cases where the BTC transaction was made but their SPV proof is
	// not yet submitted to the Bridge thus the registered main UTXO points
	// to the second last BTC transaction. In theory, such a gap between
	// the actual latest BTC transaction and the registered main UTXO in
	// the Bridge may be even wider. To cover the worst possible cases, we
	// must rely on the full transaction history. Due to performance reasons,
	// we are first taking just the transactions hashes (fast call) and then
	// fetch full transaction data (time-consuming calls) starting from
	// the most recent transactions as there is a high chance the main UTXO
	// comes from there.
	txHashes, err := getTxHashesForWalletScripts(
		btcChain,
		walletPublicKeyHash,
		walletScripts,
	)
	if err != nil {
		return nil, fmt.Errorf("cannot get transactions history for wallet: [%v]", err)
	}

	// Start iterating from the latest transaction as the chance it matches
	// the wallet main UTXO is the highest.
	for i := len(txHashes) - 1; i >= 0; i-- {
		txHash := txHashes[i]

		transaction, err := btcChain.GetTransaction(txHash)
		if err != nil {
			return nil, fmt.Errorf(
				"cannot get transaction with hash [%s]: [%v]",
				txHash.String(),
				err,
			)
		}

		// Iterate over transaction's outputs and find the one that targets
		// the wallet public key hash.
		for outputIndex, output := range transaction.Outputs {
			script := output.PublicKeyScript
			matchesWallet := scriptMatchesAny(script, walletScripts)

			// Once the right output is found, check whether their hash
			// matches the main UTXO hash stored on-chain. If so, this
			// UTXO is the one we are looking for.
			if matchesWallet {
				utxo := &bitcoin.UnspentTransactionOutput{
					Outpoint: &bitcoin.TransactionOutpoint{
						TransactionHash: transaction.Hash(),
						OutputIndex:     uint32(outputIndex),
					},
					Value: output.Value,
				}

				if bridgeChain.ComputeMainUtxoHash(utxo) ==
					walletChainData.MainUtxoHash {
					return utxo, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("main UTXO not found")
}

// EnsureWalletSyncedBetweenChains makes sure all actions taken by the wallet
// on the Bitcoin chain are reflected in the host chain Bridge.
func EnsureWalletSyncedBetweenChains(
	walletPublicKeyHash [20]byte,
	walletMainUtxo *bitcoin.UnspentTransactionOutput,
	bridgeChain BridgeChain,
	btcChain bitcoin.Chain,
) error {
	walletScripts, err := legacyWalletPublicKeyScripts(walletPublicKeyHash)
	if err != nil {
		return err
	}

	return ensureWalletSyncedBetweenChains(
		walletPublicKeyHash,
		walletScripts,
		walletMainUtxo,
		bridgeChain,
		btcChain,
	)
}

// EnsureWalletSyncedBetweenChainsForPublicKey makes sure all actions taken by
// the wallet on the Bitcoin chain are reflected in the host chain Bridge.
// Unlike EnsureWalletSyncedBetweenChains, this variant can discover Taproot
// wallet outputs.
func EnsureWalletSyncedBetweenChainsForPublicKey(
	walletPublicKey *ecdsa.PublicKey,
	walletMainUtxo *bitcoin.UnspentTransactionOutput,
	bridgeChain BridgeChain,
	btcChain bitcoin.Chain,
) error {
	walletPublicKeyHash := bitcoin.PublicKeyHash(walletPublicKey)

	walletScripts, err := walletPublicKeyScripts(walletPublicKey)
	if err != nil {
		return err
	}

	return ensureWalletSyncedBetweenChains(
		walletPublicKeyHash,
		walletScripts,
		walletMainUtxo,
		bridgeChain,
		btcChain,
	)
}

func ensureWalletSyncedBetweenChains(
	walletPublicKeyHash [20]byte,
	walletScripts []bitcoin.Script,
	walletMainUtxo *bitcoin.UnspentTransactionOutput,
	bridgeChain BridgeChain,
	btcChain bitcoin.Chain,
) error {
	// Take UTXOs controlled by the wallet on Bitcoin chain. Those are outputs
	// coming from confirmed transactions, ready to be spent right now, and
	// not used as inputs of other (either confirmed or mempool) transactions.
	confirmedUtxos, err := getUtxosForWalletScripts(
		btcChain,
		walletPublicKeyHash,
		walletScripts,
		true,
	)
	if err != nil {
		return fmt.Errorf("cannot get confirmed UTXOs: [%v]", err)
	}

	if walletMainUtxo != nil {
		// If the wallet main UTXO exists, the UTXOs set must
		// contain at least one item. If it is empty, something went
		// really wrong. This should never happen but check this scenario
		// just in case.
		if len(confirmedUtxos) == 0 {
			return fmt.Errorf(
				"wallet main UTXO exists but there are no " +
					"UTXOs controlled by the wallet on Bitcoin chain",
			)
		}

		// Start iterating from the latest UTXO as the chance it matches
		// the wallet main UTXO is the highest.
		for i := len(confirmedUtxos) - 1; i >= 0; i-- {
			utxo := confirmedUtxos[i]

			// If the wallet main UTXO is among the UTXOs returned by Bitcoin
			// client, that means the wallet has not spent it by creating
			// a Bitcoin transaction. That implies the wallet is not doing
			// any action on Bitcoin right now and their state here is synced
			// with the host chain Bridge.
			if walletMainUtxo.Outpoint.TransactionHash == utxo.Outpoint.TransactionHash &&
				walletMainUtxo.Outpoint.OutputIndex == utxo.Outpoint.OutputIndex &&
				walletMainUtxo.Value == utxo.Value {
				return nil
			}
		}

		return fmt.Errorf("wallet main UTXO registered in the " +
			"host chain Bridge is actually spent on Bitcoin; " +
			"Bridge is probably awaiting some SPV proofs",
		)
	} else {
		// Otherwise, the wallet is a fresh one and requires special
		// treatment. We need to minimize the chance the wallet is
		// currently doing their first Bitcoin transaction but, in the same
		// time, we cannot just assume their transaction history must be
		// empty as there can be spam transactions which arbitrarily send BTC
		// to the wallet address. We need to look at the confirmed and mempool
		// UTXOs and make sure there are no transactions produced by the wallet
		// there.
		mempoolUtxos, err := getUtxosForWalletScripts(
			btcChain,
			walletPublicKeyHash,
			walletScripts,
			false,
		)
		if err != nil {
			return fmt.Errorf("cannot get mempool UTXOs: [%v]", err)
		}

		allUtxos := append(confirmedUtxos, mempoolUtxos...)
		if len(allUtxos) == 0 {
			// Wallet have not produced any transactions - we are good.
			return nil
		}

		for _, utxo := range allUtxos {
			// The first valid transaction of a wallet is most likely a deposit
			// sweep, but there is a small chance it could be a moved funds sweep.
			// It could happen if the wallet was selected as a target wallet and
			// some funds were moved to it by the source wallet. In that case the
			// wallet could create a moved fund sweep transaction even before
			// sweeping any deposits.

			// In any case, we know that valid first transaction of the wallet
			// always have just one output. Any utxos with output index other
			// than 0 are certainly not produced by the wallet and, we should
			// not take them into account.
			if utxo.Outpoint.OutputIndex != 0 {
				continue
			}

			transaction, err := btcChain.GetTransaction(utxo.Outpoint.TransactionHash)
			if err != nil {
				return fmt.Errorf(
					"cannot get transaction with hash [%s]: [%v]",
					utxo.Outpoint.TransactionHash.String(),
					err,
				)
			}

			// The transaction could be a deposit sweep. In that case all the
			// transaction's inputs must refer to revealed deposits. We can
			// check one input. If it points to a revealed deposit, that means
			// the given transaction is produced by our wallet.
			if len(transaction.Inputs) == 0 {
				return fmt.Errorf(
					"transaction with hash [%s] has no inputs",
					utxo.Outpoint.TransactionHash.String(),
				)
			}
			input := transaction.Inputs[0]
			_, isDeposit, err := bridgeChain.GetDepositRequest(
				input.Outpoint.TransactionHash,
				input.Outpoint.OutputIndex,
			)
			if err != nil {
				return fmt.Errorf(
					"cannot get deposit request for hash [%s] "+
						"and output index [%v]: [%v]",
					input.Outpoint.TransactionHash.String(),
					input.Outpoint.OutputIndex,
					err,
				)
			}

			if isDeposit {
				// If that's the case, the wallet has already created a deposit
				// sweep as their first Bitcoin transaction and the Bridge is
				// awaiting the SPV proof.
				return fmt.Errorf("wallet already produced their first " +
					"Bitcoin transaction (deposit sweep); Bridge is probably " +
					"awaiting the SPV proof",
				)
			}

			// The transaction could be a moved funds sweep request. In that
			// case the transaction's input must refer to a moved funds sweep
			// request. If the input points to a moved funds sweep request, that
			// means the given transaction is produced by our wallet.
			_, isRequest, err := bridgeChain.GetMovedFundsSweepRequest(
				input.Outpoint.TransactionHash,
				input.Outpoint.OutputIndex,
			)
			if err != nil {
				return fmt.Errorf(
					"cannot get moved funds sweep request for hash [%s] "+
						"and output index [%v]: [%v]",
					input.Outpoint.TransactionHash.String(),
					input.Outpoint.OutputIndex,
					err,
				)
			}

			if isRequest {
				// If that's the case, the wallet has already created a moved
				// funds sweep as their first Bitcoin transaction and the Bridge
				// is awaiting the SPV proof.
				return fmt.Errorf("wallet already produced their first " +
					"Bitcoin transaction (moved funds sweep); Bridge is " +
					"probably awaiting the SPV proof",
				)
			}

			// If the transaction does not refer revealed deposits, it is
			// a spam, and we go to the next one.
		}

		return nil
	}
}

type walletPublicKeyScriptsChain interface {
	GetTxHashesForPublicKeyScripts(
		publicKeyScripts []bitcoin.Script,
	) ([]bitcoin.Hash, error)
	GetUtxosForPublicKeyScripts(
		publicKeyScripts []bitcoin.Script,
	) ([]*bitcoin.UnspentTransactionOutput, error)
	GetMempoolUtxosForPublicKeyScripts(
		publicKeyScripts []bitcoin.Script,
	) ([]*bitcoin.UnspentTransactionOutput, error)
}

func legacyWalletPublicKeyScripts(
	walletPublicKeyHash [20]byte,
) ([]bitcoin.Script, error) {
	walletP2PKH, err := bitcoin.PayToPublicKeyHash(walletPublicKeyHash)
	if err != nil {
		return nil, fmt.Errorf("cannot construct P2PKH for wallet: [%v]", err)
	}

	walletP2WPKH, err := bitcoin.PayToWitnessPublicKeyHash(walletPublicKeyHash)
	if err != nil {
		return nil, fmt.Errorf("cannot construct P2WPKH for wallet: [%v]", err)
	}

	return []bitcoin.Script{walletP2PKH, walletP2WPKH}, nil
}

func walletPublicKeyScripts(
	walletPublicKey *ecdsa.PublicKey,
) ([]bitcoin.Script, error) {
	walletPublicKeyHash := bitcoin.PublicKeyHash(walletPublicKey)

	walletScripts, err := legacyWalletPublicKeyScripts(walletPublicKeyHash)
	if err != nil {
		return nil, err
	}

	xOnlyPublicKey, err := walletXOnlyPublicKey(walletPublicKey)
	if err != nil {
		return nil, err
	}

	walletP2TR, err := bitcoin.PayToTaproot(xOnlyPublicKey)
	if err != nil {
		return nil, fmt.Errorf("cannot construct P2TR for wallet: [%v]", err)
	}

	return append(walletScripts, walletP2TR), nil
}

func getTxHashesForWalletScripts(
	btcChain bitcoin.Chain,
	walletPublicKeyHash [20]byte,
	walletScripts []bitcoin.Script,
) ([]bitcoin.Hash, error) {
	if scriptChain, ok := btcChain.(walletPublicKeyScriptsChain); ok {
		return scriptChain.GetTxHashesForPublicKeyScripts(walletScripts)
	}

	return btcChain.GetTxHashesForPublicKeyHash(walletPublicKeyHash)
}

func getUtxosForWalletScripts(
	btcChain bitcoin.Chain,
	walletPublicKeyHash [20]byte,
	walletScripts []bitcoin.Script,
	confirmed bool,
) ([]*bitcoin.UnspentTransactionOutput, error) {
	if scriptChain, ok := btcChain.(walletPublicKeyScriptsChain); ok {
		if confirmed {
			return scriptChain.GetUtxosForPublicKeyScripts(walletScripts)
		}

		return scriptChain.GetMempoolUtxosForPublicKeyScripts(walletScripts)
	}

	if confirmed {
		return btcChain.GetUtxosForPublicKeyHash(walletPublicKeyHash)
	}

	return btcChain.GetMempoolUtxosForPublicKeyHash(walletPublicKeyHash)
}

func scriptMatchesAny(script bitcoin.Script, scripts []bitcoin.Script) bool {
	for _, candidate := range scripts {
		if bytes.Equal(script, candidate) {
			return true
		}
	}

	return false
}

// signer represents a threshold signer of a tBTC wallet. A signer holds
// a wallet tECDSA private key share and is able to participate in the
// signing process.
type signer struct {
	// wallet points to the tBTC wallet this signer belongs to.
	wallet wallet

	// signingGroupMemberIndex indicates the signer position (seat) in the
	// wallet signing group. Since the final wallet signing group may differ
	// from the original group outputted by the sortition protocol
	// (see wallet.signingGroupOperators documentation for reference), the
	// signingGroupMemberIndex may differ from the member index using
	// during the DKG protocol as well. The value of this index is in the
	// [1, len(wallet.signingGroupOperators)] range.
	signingGroupMemberIndex group.MemberIndex

	// privateKeyShare is the tECDSA private key share required to participate
	// in the signing process.
	privateKeyShare *tecdsa.PrivateKeyShare

	// signerMaterial carries backend-specific signer material used by the
	// FROST signing runtime. Legacy path falls back to privateKeyShare.
	signerMaterial any
}

// newSigner constructs a new instance of the wallet's signer.
func newSigner(
	walletPublicKey *ecdsa.PublicKey,
	walletSigningGroupOperators []chain.Address,
	signingGroupMemberIndex group.MemberIndex,
	privateKeyShare *tecdsa.PrivateKeyShare,
	signerMaterial any,
) *signer {
	wallet := wallet{
		publicKey:             walletPublicKey,
		signingGroupOperators: walletSigningGroupOperators,
	}

	if signerMaterial == nil {
		signerMaterial = privateKeyShare
	}

	return &signer{
		wallet:                  wallet,
		signingGroupMemberIndex: signingGroupMemberIndex,
		privateKeyShare:         privateKeyShare,
		signerMaterial:          signerMaterial,
	}
}

func (s *signer) signingMaterial() any {
	if s.signerMaterial != nil {
		return s.signerMaterial
	}

	return s.privateKeyShare
}

func (s *signer) String() string {
	return fmt.Sprintf(
		"signer with index [%v] of wallet [%s]",
		s.signingGroupMemberIndex,
		&s.wallet,
	)
}
