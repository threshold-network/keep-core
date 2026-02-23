//go:build frost_native

package signing

import (
	"fmt"
	"testing"
)

type mockNativeTBTCSignerEngine struct{}

func (mntse *mockNativeTBTCSignerEngine) RunDKG(
	sessionID string,
	participants []NativeTBTCSignerDKGParticipant,
	threshold uint16,
) (*NativeTBTCSignerDKGResult, error) {
	return nil, fmt.Errorf("not implemented")
}

func (mntse *mockNativeTBTCSignerEngine) StartSignRound(
	sessionID string,
	message []byte,
	keyGroup string,
) (*NativeTBTCSignerRoundState, error) {
	return nil, fmt.Errorf("not implemented")
}

func (mntse *mockNativeTBTCSignerEngine) FinalizeSignRound(
	sessionID string,
	roundContributions []NativeTBTCSignerRoundContribution,
) ([]byte, error) {
	return nil, fmt.Errorf("not implemented")
}

func TestRegisterNativeTBTCSignerEngineRejectsNil(t *testing.T) {
	UnregisterNativeTBTCSignerEngine()
	t.Cleanup(UnregisterNativeTBTCSignerEngine)

	err := RegisterNativeTBTCSignerEngine(nil)
	if err == nil {
		t.Fatal("expected registration error")
	}
}

func TestRegisterNativeTBTCSignerEngine(t *testing.T) {
	UnregisterNativeTBTCSignerEngine()
	t.Cleanup(UnregisterNativeTBTCSignerEngine)

	engine := &mockNativeTBTCSignerEngine{}

	err := RegisterNativeTBTCSignerEngine(engine)
	if err != nil {
		t.Fatalf("unexpected registration error: [%v]", err)
	}

	if currentNativeTBTCSignerEngine() != engine {
		t.Fatal("expected current native tbtc-signer engine to match registered engine")
	}
}

func TestUnregisterNativeTBTCSignerEngine(t *testing.T) {
	UnregisterNativeTBTCSignerEngine()

	err := RegisterNativeTBTCSignerEngine(&mockNativeTBTCSignerEngine{})
	if err != nil {
		t.Fatalf("unexpected registration error: [%v]", err)
	}

	UnregisterNativeTBTCSignerEngine()

	if currentNativeTBTCSignerEngine() != nil {
		t.Fatal("expected native tbtc-signer engine to be nil after unregister")
	}
}
