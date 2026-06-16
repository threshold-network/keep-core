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

// NativeShareVerificationVerdict is the typed result of a single-share FROST
// re-verification (frost_tbtc_verify_signature_share). It mirrors the engine's
// tri-state verdict.
//
// Indeterminate is deliberately the ZERO value: the boundary between blame
// (Invalid) and don't-blame is security-critical, so an unset value, a decode
// failure, or an FFI-transport error all fail closed against false blame. A
// verdict is only meaningful when the accompanying error is nil.
type NativeShareVerificationVerdict int

const (
	// NativeShareVerdictIndeterminate: verification could not be completed for a
	// reason that is not the member's fault (or could not be obtained at all).
	// Fail closed against blame. Zero value.
	NativeShareVerdictIndeterminate NativeShareVerificationVerdict = iota
	// NativeShareVerdictValid: the share is a valid FROST signature share for the
	// (tweaked) package. Not blamable.
	NativeShareVerdictValid
	// NativeShareVerdictInvalid: the share is member-attributable garbage -
	// mathematically invalid, or undecodable member-signed bytes. Blamable.
	NativeShareVerdictInvalid
)

// NativeInteractiveAttemptContext is the RFC-21 attempt context an interactive
// signing session is bound to. It mirrors the engine's AttemptContext: the
// orchestrator derives it from the wallet/session state (never from a peer
// message) and passes it on InteractiveSessionOpen.
type NativeInteractiveAttemptContext struct {
	AttemptNumber                   uint32
	CoordinatorIdentifier           uint16
	IncludedParticipants            []uint16
	IncludedParticipantsFingerprint string
	AttemptID                       string
}

// NativeInteractiveSessionOpenResult is the result of InteractiveSessionOpen:
// the engine's canonical attempt id for the opened (or idempotently re-opened)
// attempt.
type NativeInteractiveSessionOpenResult struct {
	SessionID  string
	AttemptID  string
	Idempotent bool
}

// NativeInteractiveSessionAbortResult is the result of InteractiveSessionAbort.
// Aborted is false when there was no live attempt to abort.
type NativeInteractiveSessionAbortResult struct {
	SessionID string
	Aborted   bool
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
	VerifySignatureShare(
		sessionID string,
		signingPackage []byte,
		signatureShare []byte,
		memberIdentifier uint16,
		taprootMerkleRoot *[32]byte,
	) (NativeShareVerificationVerdict, error)
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
