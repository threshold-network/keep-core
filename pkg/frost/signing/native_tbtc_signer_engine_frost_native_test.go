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
	memberIdentifier uint16,
	message []byte,
	keyGroup string,
	signingParticipants []uint16,
	taprootMerkleRoot *[32]byte,
) (*NativeTBTCSignerRoundState, error) {
	_ = memberIdentifier
	_ = signingParticipants
	_ = taprootMerkleRoot
	return nil, fmt.Errorf("not implemented")
}

func (mntse *mockNativeTBTCSignerEngine) FinalizeSignRound(
	sessionID string,
	roundContributions []NativeTBTCSignerRoundContribution,
	taprootMerkleRoot *[32]byte,
) ([]byte, error) {
	_ = taprootMerkleRoot
	return nil, fmt.Errorf("not implemented")
}

func (mntse *mockNativeTBTCSignerEngine) BuildTaprootTx(
	sessionID string,
	inputs []NativeTBTCSignerTxInput,
	outputs []NativeTBTCSignerTxOutput,
	scriptTreeHex *string,
) (*NativeTBTCSignerTxResult, error) {
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
