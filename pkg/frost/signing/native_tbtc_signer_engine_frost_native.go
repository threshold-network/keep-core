//go:build frost_native

package signing

import "fmt"

const (
	// NativeSignerMaterialFormatFrostTBTCSignerV1 carries signer material for
	// tbtc-signer coarse session APIs.
	NativeSignerMaterialFormatFrostTBTCSignerV1 = "frost-tbtc-signer-v1"
)

// NativeTBTCSignerMaterialPayload is the signer-material payload schema for
// `frost-tbtc-signer-v1`.
type NativeTBTCSignerMaterialPayload struct {
	KeyGroup                 string `json:"keyGroup"`
	KeyGroupSource           string `json:"keyGroupSource,omitempty"`
	LegacyPrivateKeyShareHex string `json:"legacyPrivateKeyShareHex,omitempty"`
}

// NativeTBTCSignerRoundContribution is a participant contribution consumed by
// tbtc-signer during signature finalization.
type NativeTBTCSignerRoundContribution struct {
	Identifier uint16 `json:"identifier"`
	Data       []byte `json:"data"`
}

// NativeTBTCSignerRoundState captures coarse session round metadata returned by
// StartSignRound.
type NativeTBTCSignerRoundState struct {
	SessionID             string `json:"sessionID"`
	RoundID               string `json:"roundID"`
	RequiredContributions uint16 `json:"requiredContributions"`
	MessageDigestHex      string `json:"messageDigestHex"`
}

// NativeTBTCSignerEngine executes coarse, session-keyed tbtc-signer
// operations.
type NativeTBTCSignerEngine interface {
	StartSignRound(
		sessionID string,
		message []byte,
		keyGroup string,
	) (*NativeTBTCSignerRoundState, error)
	FinalizeSignRound(
		sessionID string,
		roundContributions []NativeTBTCSignerRoundContribution,
	) ([]byte, error)
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

func currentNativeTBTCSignerEngine() NativeTBTCSignerEngine {
	executionBackendMutex.RLock()
	defer executionBackendMutex.RUnlock()

	return nativeTBTCSignerEngine
}
