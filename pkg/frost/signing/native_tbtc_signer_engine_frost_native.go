//go:build frost_native

package signing

import "fmt"

// NativeTBTCSignerDKGResult captures DKG result metadata returned by RunDKG.
type NativeTBTCSignerDKGResult struct {
	SessionID        string `json:"sessionID"`
	KeyGroup         string `json:"keyGroup"`
	ParticipantCount uint16 `json:"participantCount"`
	Threshold        uint16 `json:"threshold"`
	CreatedAtUnix    uint64 `json:"createdAtUnix"`
}

// NativeTBTCSignerRoundContribution is a participant contribution consumed by
// tbtc-signer during signature finalization.
type NativeTBTCSignerRoundContribution struct {
	Identifier uint16 `json:"identifier"`
	Data       []byte `json:"data"`
}

// NativeTBTCSignerTxInput describes an unsigned transaction input consumed by
// BuildTaprootTx.
type NativeTBTCSignerTxInput struct {
	TxIDHex   string `json:"txIDHex"`
	Vout      uint32 `json:"vout"`
	ValueSats uint64 `json:"valueSats"`
}

// NativeTBTCSignerTxOutput describes an unsigned transaction output consumed
// by BuildTaprootTx.
type NativeTBTCSignerTxOutput struct {
	ScriptPubKeyHex string `json:"scriptPubKeyHex"`
	ValueSats       uint64 `json:"valueSats"`
}

// NativeTBTCSignerTxResult captures unsigned transaction metadata returned by
// BuildTaprootTx.
type NativeTBTCSignerTxResult struct {
	SessionID string `json:"sessionID"`
	TxHex     string `json:"txHex"`
}

// NativeTBTCSignerRoundState captures coarse session round metadata returned by
// StartSignRound.
type NativeTBTCSignerRoundState struct {
	SessionID             string                             `json:"sessionID"`
	RoundID               string                             `json:"roundID"`
	RequiredContributions uint16                             `json:"requiredContributions"`
	MessageDigestHex      string                             `json:"messageDigestHex"`
	SigningParticipants   []uint16                           `json:"signingParticipants"`
	OwnContribution       *NativeTBTCSignerRoundContribution `json:"ownContribution"`
}

// NativeTBTCSignerEngine executes coarse, session-keyed tbtc-signer
// operations.
type NativeTBTCSignerEngine interface {
	RunDKG(
		sessionID string,
		participants []NativeTBTCSignerDKGParticipant,
		threshold uint16,
	) (*NativeTBTCSignerDKGResult, error)
	StartSignRound(
		sessionID string,
		memberIdentifier uint16,
		message []byte,
		keyGroup string,
		signingParticipants []uint16,
		taprootMerkleRoot *[32]byte,
	) (*NativeTBTCSignerRoundState, error)
	FinalizeSignRound(
		sessionID string,
		roundContributions []NativeTBTCSignerRoundContribution,
		taprootMerkleRoot *[32]byte,
	) ([]byte, error)
	BuildTaprootTx(
		sessionID string,
		inputs []NativeTBTCSignerTxInput,
		outputs []NativeTBTCSignerTxOutput,
		scriptTreeHex *string,
	) (*NativeTBTCSignerTxResult, error)
}

// NativeTBTCSignerSeededDKGEngine is implemented by tbtc-signer engines that
// can pin development dealer DKG to an externally supplied seed. Production
// distributed DKG does not rely on this helper.
type NativeTBTCSignerSeededDKGEngine interface {
	RunDKGWithSeed(
		sessionID string,
		participants []NativeTBTCSignerDKGParticipant,
		threshold uint16,
		dkgSeedHex string,
	) (*NativeTBTCSignerDKGResult, error)
}

var nativeTBTCSignerEngine NativeTBTCSignerEngine

// RegisterNativeTBTCSignerEngine registers the coarse tbtc-signer engine used
// by frost_tbtc_signer builds.
func RegisterNativeTBTCSignerEngine(engine NativeTBTCSignerEngine) error {
	if engine == nil {
		return fmt.Errorf("native tbtc-signer engine is nil")
	}

	executionBackendMutex.Lock()
	defer executionBackendMutex.Unlock()

	nativeTBTCSignerEngine = engine

	return nil
}

// UnregisterNativeTBTCSignerEngine clears coarse tbtc-signer engine
// registration.
func UnregisterNativeTBTCSignerEngine() {
	executionBackendMutex.Lock()
	defer executionBackendMutex.Unlock()

	nativeTBTCSignerEngine = nil
}

// CurrentNativeTBTCSignerEngine returns the registered coarse tbtc-signer
// engine.
func CurrentNativeTBTCSignerEngine() NativeTBTCSignerEngine {
	return currentNativeTBTCSignerEngine()
}

func currentNativeTBTCSignerEngine() NativeTBTCSignerEngine {
	executionBackendMutex.RLock()
	defer executionBackendMutex.RUnlock()

	return nativeTBTCSignerEngine
}
