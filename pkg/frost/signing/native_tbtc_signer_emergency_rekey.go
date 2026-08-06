package signing

// maximumTBTCSignerEmergencyRekeySessionIDLength bounds the operator-supplied
// session ID before it reaches the engine. The engine validates the ID too;
// this only stops an obviously malformed input from consuming a barrier lease
// and an anchor round trip on its way to a guaranteed rejection.
const maximumTBTCSignerEmergencyRekeySessionIDLength = 128

// NativeTBTCSignerEmergencyRekey is the engine's record of an armed wallet-level
// emergency rekey.
//
// SessionID is the session the engine actually armed, which is not always the
// one requested: arming a per-signing session retargets to the wallet session
// it serves, so the write lands where the interactive gates read it.
//
// The event is immutable once armed - no engine export clears it - so the
// wallet named here cannot sign again, and RecommendedNewSessionID is the
// engine's suggested identity for the replacement DKG.
type NativeTBTCSignerEmergencyRekey struct {
	SessionID               string
	Reason                  string
	TriggeredAtUnix         uint64
	RecommendedNewSessionID string
}
