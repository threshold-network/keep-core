package signing

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/ipfs/go-log/v2"
	"github.com/keep-network/keep-core/pkg/net"
)

// Tests in this file mutate process-global registration state and the fatal
// seam; per the package convention they must not use t.Parallel.

type capturedFatalCalls struct {
	messages []string
}

// captureFatalNativeRegistrationExit swaps the fatal seam for a recorder so
// the demand enforcement can be observed without killing the test binary.
func captureFatalNativeRegistrationExit(t *testing.T) *capturedFatalCalls {
	t.Helper()

	capture := &capturedFatalCalls{}
	previous := fatalNativeRegistrationExit
	fatalNativeRegistrationExit = func(format string, args ...interface{}) {
		capture.messages = append(capture.messages, fmt.Sprintf(format, args...))
	}
	t.Cleanup(func() {
		fatalNativeRegistrationExit = previous
	})

	return capture
}

// resetNativeRegistrationStateForDemandTest snapshots the process-global
// registration state, clears it for the test, and restores it on cleanup.
func resetNativeRegistrationStateForDemandTest(t *testing.T) {
	t.Helper()

	executionBackendMutex.Lock()
	previousAdapter := nativeExecutionAdapter
	previousExecutor := nativeExecutionFFIExecutor
	nativeExecutionAdapter = nil
	nativeExecutionFFIExecutor = nil
	executionBackendMutex.Unlock()

	registrationErrorMu.Lock()
	previousError := lastRegistrationError
	lastRegistrationError = nil
	registrationErrorMu.Unlock()

	t.Cleanup(func() {
		executionBackendMutex.Lock()
		nativeExecutionAdapter = previousAdapter
		nativeExecutionFFIExecutor = previousExecutor
		executionBackendMutex.Unlock()

		setLastRegistrationError(previousError)
	})
}

type demandTestExecutionStub struct{}

func (dtes *demandTestExecutionStub) Execute(
	_ context.Context,
	_ log.StandardLogger,
	_ *Request,
) (*Result, error) {
	return nil, errors.New("demand test stub is not executable")
}

func (dtes *demandTestExecutionStub) RegisterUnmarshallers(
	_ net.BroadcastChannel,
) {
}

func registerDemandTestNativeState(t *testing.T) {
	t.Helper()

	if err := RegisterNativeExecutionAdapter(&demandTestExecutionStub{}); err != nil {
		t.Fatalf("failed to register stub native adapter: [%v]", err)
	}
	if err := RegisterNativeExecutionFFIExecutor(&demandTestExecutionStub{}); err != nil {
		t.Fatalf("failed to register stub FFI executor: [%v]", err)
	}
}

func TestEnforceNativeInitConfigDemand_PathUnset_KeepsDegradePosture(t *testing.T) {
	resetNativeRegistrationStateForDemandTest(t)
	capture := captureFatalNativeRegistrationExit(t)
	t.Setenv(TBTCSignerInitConfigPathEnv, "")

	// Even with a recorded registration failure and nothing registered, an
	// unset path means env-fallback mode: registration failures degrade to
	// the legacy bridge and never abort the process.
	setLastRegistrationError(errors.New("simulated registration failure"))

	enforceNativeInitConfigDemand()

	if len(capture.messages) != 0 {
		t.Fatalf(
			"expected no fatal exit with the path unset, got: %q",
			capture.messages,
		)
	}
}

func TestEnforceNativeInitConfigDemand_PathWhitespace_KeepsDegradePosture(t *testing.T) {
	resetNativeRegistrationStateForDemandTest(t)
	capture := captureFatalNativeRegistrationExit(t)
	t.Setenv(TBTCSignerInitConfigPathEnv, "   ")

	setLastRegistrationError(errors.New("simulated registration failure"))

	enforceNativeInitConfigDemand()

	if len(capture.messages) != 0 {
		t.Fatalf(
			"expected no fatal exit with a whitespace-only path, got: %q",
			capture.messages,
		)
	}
}

func TestEnforceNativeInitConfigDemand_RegistrationError_IsFatal(t *testing.T) {
	resetNativeRegistrationStateForDemandTest(t)
	capture := captureFatalNativeRegistrationExit(t)
	t.Setenv(TBTCSignerInitConfigPathEnv, "/etc/keep/tbtc-signer-config.json")

	setLastRegistrationError(errors.New("simulated install rejection"))

	enforceNativeInitConfigDemand()

	if len(capture.messages) != 1 {
		t.Fatalf(
			"expected exactly one fatal exit, got %d: %q",
			len(capture.messages), capture.messages,
		)
	}

	message := capture.messages[0]
	for _, want := range []string{
		TBTCSignerInitConfigPathEnv,
		"/etc/keep/tbtc-signer-config.json",
		"simulated install rejection",
	} {
		if !strings.Contains(message, want) {
			t.Errorf("fatal message missing %q: %q", want, message)
		}
	}
}

func TestEnforceNativeInitConfigDemand_NothingRegistered_IsFatal(t *testing.T) {
	resetNativeRegistrationStateForDemandTest(t)
	capture := captureFatalNativeRegistrationExit(t)
	t.Setenv(TBTCSignerInitConfigPathEnv, "/etc/keep/tbtc-signer-config.json")

	enforceNativeInitConfigDemand()

	if len(capture.messages) != 1 {
		t.Fatalf(
			"expected exactly one fatal exit, got %d: %q",
			len(capture.messages), capture.messages,
		)
	}

	message := capture.messages[0]
	if !strings.Contains(message, TBTCSignerInitConfigPathEnv) {
		t.Errorf(
			"fatal message missing %q: %q",
			TBTCSignerInitConfigPathEnv, message,
		)
	}

	// The message names the precise cause per build flavor: a binary without
	// frost_native can never honor the demand; a frost_native binary reports
	// which registration leg is missing.
	if buildHasNativeFROSTRegistration {
		if !strings.Contains(message, "did not complete") {
			t.Errorf(
				"fatal message missing registration-incomplete cause: %q",
				message,
			)
		}
	} else {
		if !strings.Contains(message, "without the frost_native build tag") {
			t.Errorf(
				"fatal message missing wrong-binary cause: %q",
				message,
			)
		}
	}
}

func TestEnforceNativeInitConfigDemand_PartialRegistration_IsFatal(t *testing.T) {
	resetNativeRegistrationStateForDemandTest(t)
	capture := captureFatalNativeRegistrationExit(t)
	t.Setenv(TBTCSignerInitConfigPathEnv, "/etc/keep/tbtc-signer-config.json")

	// Only the FFI executor comes up; the native adapter leg is missing.
	// The demand requires the complete bring-up, so this is still fatal.
	if err := RegisterNativeExecutionFFIExecutor(&demandTestExecutionStub{}); err != nil {
		t.Fatalf("failed to register stub FFI executor: [%v]", err)
	}

	enforceNativeInitConfigDemand()

	if len(capture.messages) != 1 {
		t.Fatalf(
			"expected exactly one fatal exit, got %d: %q",
			len(capture.messages), capture.messages,
		)
	}
}

func TestEnforceNativeInitConfigDemand_FullyRegistered_NoFatal(t *testing.T) {
	resetNativeRegistrationStateForDemandTest(t)
	capture := captureFatalNativeRegistrationExit(t)
	t.Setenv(TBTCSignerInitConfigPathEnv, "/etc/keep/tbtc-signer-config.json")

	registerDemandTestNativeState(t)

	enforceNativeInitConfigDemand()

	if len(capture.messages) != 0 {
		t.Fatalf(
			"expected no fatal exit with the native engine fully registered, "+
				"got: %q",
			capture.messages,
		)
	}
}

func TestRegisterNativeExecutionAdapterForBuild_EnforcesInitConfigDemand(t *testing.T) {
	resetNativeRegistrationStateForDemandTest(t)
	capture := captureFatalNativeRegistrationExit(t)

	// A nonexistent config file: on frost_native builds the install leg
	// fails and records a registration error; on default builds no native
	// registration runs at all. Both states must be fatal under a set path,
	// pinning that the enforcement is wired into the registration entry
	// point for every build flavor.
	t.Setenv(
		TBTCSignerInitConfigPathEnv,
		t.TempDir()+"/nonexistent-tbtc-signer-config.json",
	)

	RegisterNativeExecutionAdapterForBuild()

	if len(capture.messages) != 1 {
		t.Fatalf(
			"expected exactly one fatal exit, got %d: %q",
			len(capture.messages), capture.messages,
		)
	}

	if !strings.Contains(capture.messages[0], TBTCSignerInitConfigPathEnv) {
		t.Errorf(
			"fatal message missing %q: %q",
			TBTCSignerInitConfigPathEnv, capture.messages[0],
		)
	}
}
