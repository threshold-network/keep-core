//go:build frost_native && frost_tbtc_signer && cgo

package signing

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Worker env vars for the subprocess that runs the success scenario in
// isolation. The other scenarios run inline against a barrier installed over
// test-double state, so they do not need their own subprocess role.
const (
	rekeyWorkerRoleEnv     = "KEEP_CORE_FROST_TBTC_REKEY_WORKER_ROLE"
	rekeyWorkerStateDirEnv = "TBTC_SIGNER_STATE_PATH"
	rekeyWorkerProfileEnv  = "TBTC_SIGNER_PROFILE"
	rekeyWorkerEnforceEnv  = "TBTC_SIGNER_ENFORCE_PROVENANCE_GATE"
	rekeyWorkerStateKeyEnv = "TBTC_SIGNER_STATE_ENCRYPTION_KEY_HEX"

	rekeySkipPrefix = "FROST_REKEY_SKIP="
	rekeyErrPrefix  = "FROST_REKEY_ERROR="

	rekeyRoleSuccess = "success"
)

// TestTriggerNativeTBTCSignerEmergencyRekey_CgoWiringAndBarrierGating proves
// the cgo-backed TriggerNativeTBTCSignerEmergencyRekey path is wired through
// the state anchor barrier: a successful call advances the durable anchor tip
// exactly once, while calls with a poisoned barrier or an exhausted certified
// anchor window are refused.
//
// The success scenario mutates durable signer state (arming the wallet-level
// kill switch is wallet-lifetime and single-flight - the engine has no export
// that clears it), so it runs in a dedicated subprocess with its own
// ephemeral state directory. The poisoned and exhausted scenarios are refused
// before any state-touching cgo call reaches the engine: they run inline
// against an installed barrier over test-double state, and the inline call to
// TriggerNativeTBTCSignerEmergencyRekey only reaches the ABI preflight,
// which is the part that fails closed if the linked libfrost_tbtc is missing
// the FFI contract version symbol - in which case the scenario SKIPs rather
// than failing the suite.
func TestTriggerNativeTBTCSignerEmergencyRekey_CgoWiringAndBarrierGating(t *testing.T) {
	role := os.Getenv(rekeyWorkerRoleEnv)
	if role != "" {
		runRekeyWorker(t, role)
		return
	}

	t.Run("SuccessAdvancesAnchorTipOnce", func(t *testing.T) {
		runRekeySuccessSubprocess(t)
	})
	t.Run("PoisonedBarrierRefusesCall", func(t *testing.T) {
		runRekeyPoisonedScenario(t)
	})
	t.Run("ExhaustedAnchorWindowRefusesCall", func(t *testing.T) {
		runRekeyExhaustedScenario(t)
	})
}

func runRekeySuccessSubprocess(t *testing.T) {
	stateDir := filepath.Join(
		os.TempDir(),
		fmt.Sprintf(
			"keep-frost-tbtc-rekey-%s-%d-%d",
			rekeyRoleSuccess,
			os.Getpid(),
			time.Now().UnixNano(),
		),
	)
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		t.Fatalf("create subprocess state dir: %v", err)
	}

	stateKey := make([]byte, 32)
	for i := range stateKey {
		stateKey[i] = byte(i + 1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, os.Args[0],
		"-test.run", "^TestTriggerNativeTBTCSignerEmergencyRekey_CgoWiringAndBarrierGating$",
		"-test.v",
	)
	cmd.Env = withEnvOverrides(os.Environ(), map[string]string{
		rekeyWorkerRoleEnv:           rekeyRoleSuccess,
		rekeyWorkerStateDirEnv:       filepath.Join(stateDir, "signer-state"),
		rekeyWorkerProfileEnv:        "development",
		rekeyWorkerEnforceEnv:        "false",
		rekeyWorkerStateKeyEnv:       hex.EncodeToString(stateKey),
		frostSubprocessSkipPrefixEnv: rekeySkipPrefix,
	})
	out, err := cmd.CombinedOutput()
	output := string(out)
	if skipMsg := extractPrefixed(output, rekeySkipPrefix); skipMsg != "" {
		t.Skipf("subprocess skipped: %s", skipMsg)
	}
	if errMsg := extractPrefixed(output, rekeyErrPrefix); errMsg != "" {
		t.Fatalf("subprocess reported error: %s\n%s", errMsg, indentTail(output, 40))
	}
	if err != nil {
		t.Fatalf("subprocess failed (err=%v):\n%s", err, indentTail(output, 40))
	}
}

func runRekeyWorker(t *testing.T, role string) {
	switch role {
	case rekeyRoleSuccess:
		runRekeySuccessScenario(t)
	default:
		fmt.Printf("%sunknown rekey worker role: %s\n", rekeyErrPrefix, role)
		t.Fatalf("unknown rekey worker role: %s", role)
	}
}

func runRekeySuccessScenario(t *testing.T) {
	setupRealCgoSignerStateAnchor(t)

	initialTip, err := ReadNativeTBTCSignerStateWitnessTip()
	skipFrostUnavailable(t, "state-witness tip", err)
	if err != nil {
		t.Fatalf("cannot read pre-trigger state tip: %v", err)
	}

	const (
		sessionID = "wallet-session"
		reason    = "compromise"
	)
	rekey, err := TriggerNativeTBTCSignerEmergencyRekey(sessionID, reason)
	skipFrostUnavailable(t, "trigger emergency rekey", err)
	if err != nil {
		t.Fatalf("unexpected trigger error: %v", err)
	}
	if rekey == nil {
		t.Fatal("trigger returned a nil rekey record on success")
	}
	if rekey.SessionID == "" {
		t.Fatalf("trigger returned an empty session id: %+v", rekey)
	}
	if rekey.Reason != reason {
		t.Fatalf(
			"trigger returned a different reason than requested: got %q want %q",
			rekey.Reason, reason,
		)
	}
	if rekey.TriggeredAtUnix == 0 {
		t.Fatalf("trigger returned a zero trigger timestamp: %+v", rekey)
	}
	if rekey.RecommendedNewSessionID == "" {
		t.Fatalf("trigger returned an empty recommended new session id: %+v", rekey)
	}

	finalTip, err := ReadNativeTBTCSignerStateWitnessTip()
	skipFrostUnavailable(t, "state-witness tip after", err)
	if err != nil {
		t.Fatalf("cannot read post-trigger state tip: %v", err)
	}

	if finalTip.Generation != initialTip.Generation+1 {
		t.Fatalf(
			"expected anchor generation to advance by exactly one, got %d -> %d",
			initialTip.Generation, finalTip.Generation,
		)
	}
	if finalTip.AnchorRevision != initialTip.AnchorRevision+1 {
		t.Fatalf(
			"expected anchor revision to advance by exactly one, got %d -> %d",
			initialTip.AnchorRevision, finalTip.AnchorRevision,
		)
	}
	if finalTip.StateCommitment == initialTip.StateCommitment {
		t.Fatal(
			"anchor state commitment did not advance after a successful emergency rekey",
		)
	}
	if finalTip.AnchorAcknowledgementDigest == initialTip.AnchorAcknowledgementDigest {
		t.Fatal(
			"anchor acknowledgement digest did not advance after a successful emergency rekey",
		)
	}
}

// skipRekeyCgoUnavailable turns an ErrNativeCryptographyUnavailable-classed
// failure on TriggerNativeTBTCSignerEmergencyRekey into a SKIP for the
// inline barrier-only scenarios. The barrier refuses pre-call, so any
// ErrNativeCryptographyUnavailable here is purely the ABI preflight failing
// because the linked libfrost_tbtc predates FFI contract versioning - the
// wiring we are pinning is unreachable in that build.
func skipRekeyCgoUnavailable(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		return
	}
	if errors.Is(err, ErrNativeCryptographyUnavailable) {
		if prefix := os.Getenv(frostSubprocessSkipPrefixEnv); prefix != "" {
			fmt.Printf("%s%s\n", prefix, frostUnavailableSkipMessage(
				"trigger emergency rekey", err,
			))
		}
		t.Skip(frostUnavailableSkipMessage("trigger emergency rekey", err))
	}
}

func runRekeyPoisonedScenario(t *testing.T) {
	resetNativeTBTCSignerStateAnchorBarrierForTest()
	t.Cleanup(resetNativeTBTCSignerStateAnchorBarrierForTest)

	initial := testNativeTBTCSignerStateWitnessTip(1, [32]byte{2})
	current := initial
	// Install the barrier with a healthy committer first: the installer
	// calls VerifyNativeTBTCSignerStateTip itself, so a pre-set verifyErr
	// would refuse installation rather than poisoning on a request-taking
	// call.
	committer := &testNativeTBTCSignerStateAnchorCommitter{current: &current}
	installTestNativeTBTCSignerStateAnchorBarrier(
		t, &initial, &current, committer,
	)

	// Force the committer to disagree with the anchor: the next
	// request-taking call now classifies the disagreement as a fact about
	// the anchor and poisons the barrier, while returning
	// ErrNativeTBTCSignerStateAnchorTerminal to the caller. The state-touching
	// arming FFI never fires, so the immutable kill switch is untouched.
	committer.verifyErr = errors.New(
		"test: forced anchor disagreement to poison the barrier",
	)

	rekey, err := TriggerNativeTBTCSignerEmergencyRekey(
		"wallet-session",
		"compromise",
	)
	skipRekeyCgoUnavailable(t, err)
	if !errors.Is(err, ErrNativeTBTCSignerStateAnchorTerminal) {
		t.Fatalf(
			"a verify-failing committer did not poison the barrier: rekey=%+v err=%v",
			rekey, err,
		)
	}
	if rekey != nil {
		t.Fatalf("a verify-failing committer returned a rekey record: %+v", rekey)
	}

	// With the committer now honest, a second call MUST still be refused:
	// the barrier is latched terminal, and any request-taking call routed
	// through it must surface ErrNativeTBTCSignerStateAnchorTerminal.
	committer.verifyErr = nil
	rekey, err = TriggerNativeTBTCSignerEmergencyRekey(
		"wallet-session",
		"compromise",
	)
	skipRekeyCgoUnavailable(t, err)
	if !errors.Is(err, ErrNativeTBTCSignerStateAnchorTerminal) {
		t.Fatalf(
			"a poisoned barrier accepted the rekey trigger: rekey=%+v err=%v",
			rekey, err,
		)
	}
	if rekey != nil {
		t.Fatalf("a poisoned barrier returned a rekey record: %+v", rekey)
	}
	if poisoned := NativeTBTCSignerStateAnchorPoisoned(); !errors.Is(
		poisoned, ErrNativeTBTCSignerStateAnchorTerminal,
	) {
		t.Fatalf("poisoned barrier was not reported by its accessor: %v", poisoned)
	}
}

func runRekeyExhaustedScenario(t *testing.T) {
	resetNativeTBTCSignerStateAnchorBarrierForTest()
	t.Cleanup(resetNativeTBTCSignerStateAnchorBarrierForTest)

	initial := testNativeTBTCSignerStateWitnessTip(1, [32]byte{2})
	// Push the initial tip to the certified revision window's edge: the
	// barrier's revision-distance check fires when
	// readback.AnchorRevision - trustHead.CertifiedFloor.Revision >=
	// MaximumAnchorRevisionDistance. Setting the tip at exactly that
	// distance exhausts the window before any request-taking call runs.
	initial.AnchorRevision =
		testNativeTBTCSignerStateAnchorTrustHead().CertifiedFloor.Revision +
			NativeTBTCSignerStateAnchorMaximumRevisionDistance
	current := initial
	committer := &testNativeTBTCSignerStateAnchorCommitter{current: &current}
	installTestNativeTBTCSignerStateAnchorBarrier(
		t, &initial, &current, committer,
	)

	rekey, err := TriggerNativeTBTCSignerEmergencyRekey(
		"wallet-session",
		"compromise",
	)
	skipRekeyCgoUnavailable(t, err)
	if !errors.Is(err, ErrNativeTBTCSignerStateAnchorUnavailable) {
		t.Fatalf(
			"an exhausted certified window accepted the rekey trigger: rekey=%+v err=%v",
			rekey, err,
		)
	}
	if rekey != nil {
		t.Fatalf("an exhausted window returned a rekey record: %+v", rekey)
	}
	if !strings.Contains(err.Error(), "certified anchor revision window is exhausted") {
		t.Fatalf("exhaustion reason was not surfaced: %v", err)
	}
}
