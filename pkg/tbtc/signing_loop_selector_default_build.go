//go:build !frost_roast_retry

package tbtc

// defaultSigningParticipantSelector in the default build always
// returns the legacy retry shuffle. The ROAST-driven selector is
// only compiled into the frost_roast_retry build (see
// signing_loop_selector_frost_roast_retry.go) so the default
// production binary contains no ROAST-retry code paths at all.
func defaultSigningParticipantSelector() signingParticipantSelector {
	return legacySigningParticipantSelector{}
}
