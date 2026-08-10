#ifndef FROST_TBTC_H
#define FROST_TBTC_H

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef struct {
  uint8_t* ptr;
  size_t len;
} TbtcBuffer;

typedef struct {
  int32_t status_code;
  TbtcBuffer buffer;
} TbtcSignerResult;

TbtcSignerResult frost_tbtc_version(void);
TbtcSignerResult frost_tbtc_abi_version(void);
TbtcSignerResult frost_tbtc_init_signer_config(const uint8_t* request_ptr, size_t request_len);
TbtcSignerResult frost_tbtc_roast_liveness_policy(void);
TbtcSignerResult frost_tbtc_hardening_metrics(void);
TbtcSignerResult frost_tbtc_roast_transcript_audit(const uint8_t* request_ptr, size_t request_len);
TbtcSignerResult frost_tbtc_verify_blame_proof(const uint8_t* request_ptr, size_t request_len);
TbtcSignerResult frost_tbtc_quarantine_status(const uint8_t* request_ptr, size_t request_len);
TbtcSignerResult frost_tbtc_refresh_cadence_status(const uint8_t* request_ptr, size_t request_len);
TbtcSignerResult frost_tbtc_trigger_emergency_rekey(const uint8_t* request_ptr, size_t request_len);
TbtcSignerResult frost_tbtc_run_differential_fuzzing(const uint8_t* request_ptr, size_t request_len);
TbtcSignerResult frost_tbtc_canary_rollout_status(void);
TbtcSignerResult frost_tbtc_promote_canary(const uint8_t* request_ptr, size_t request_len);
TbtcSignerResult frost_tbtc_rollback_canary(const uint8_t* request_ptr, size_t request_len);
void frost_tbtc_free_buffer(uint8_t* ptr, size_t len);

TbtcSignerResult frost_tbtc_dkg_part1(const uint8_t* request_ptr, size_t request_len);
TbtcSignerResult frost_tbtc_dkg_part2(const uint8_t* request_ptr, size_t request_len);
TbtcSignerResult frost_tbtc_dkg_part3(const uint8_t* request_ptr, size_t request_len);
TbtcSignerResult frost_tbtc_persist_distributed_dkg_key_package(const uint8_t* request_ptr, size_t request_len);

TbtcSignerResult frost_tbtc_new_signing_package(const uint8_t* request_ptr, size_t request_len);
TbtcSignerResult frost_tbtc_build_taproot_tx(const uint8_t* request_ptr, size_t request_len);
/*
 * Reserved ABI: fails closed with terminal error code
 * `cryptographic_refresh_not_supported` until a multi-round, zero-constant
 * FROST refresh protocol is implemented.
 */
TbtcSignerResult frost_tbtc_refresh_shares(const uint8_t* request_ptr, size_t request_len);

/*
 * Phase 7.1 hardened interactive signing session.
 *
 * Secret nonces NEVER cross this boundary in either direction: the engine
 * generates, holds, consumes, and zeroizes them internally, keyed by
 * (session_id, attempt_id). The caller
 * exchanges only public commitments, signing packages, and signature shares.
 * frost_tbtc_interactive_round2 verifies the coordinator's signing package in
 * full and consumes the attempt's nonces exactly once; a repeat call for a
 * consumed attempt fails closed with the `consumed_nonce_replay` error code.
 */
TbtcSignerResult frost_tbtc_interactive_session_open(const uint8_t* request_ptr, size_t request_len);
TbtcSignerResult frost_tbtc_interactive_round1(const uint8_t* request_ptr, size_t request_len);
TbtcSignerResult frost_tbtc_interactive_round2(const uint8_t* request_ptr, size_t request_len);
TbtcSignerResult frost_tbtc_interactive_session_abort(const uint8_t* request_ptr, size_t request_len);
/*
 * Coordinator-side aggregation: verifies each collected signature share
 * against its verifying share (resolved from the session's DKG state) and
 * returns the aggregated BIP-340 signature. Operates on public material
 * only - no secret crosses here. On any verification failure it fails
 * closed with the generic `validation_error` code and returns no
 * signature; per-member attributable blame (a structured culprit list)
 * is intentionally NOT emitted yet - it requires the signed-package
 * envelope binding added in Phase 7.2b, without which the attribution
 * would be forgeable by a coordinator using a mismatched package/root.
 */
TbtcSignerResult frost_tbtc_interactive_aggregate(const uint8_t* request_ptr, size_t request_len);
/*
 * Phase 7.2b-4 single round-2 signature-share verification: backs the Go
 * host's Round2ShareVerifier (member-blame classifier). Verifies ONE retained
 * share against the attempt's signing package using the group's own
 * (taproot-tweaked) verifying material - public material only, no secret and
 * no operator-signed-envelope inspection (the latter is the Go layer's job).
 * Returns an explicit tri-state verdict (valid/invalid/indeterminate) so the
 * caller never infers member-fault from an FFI error code. Like aggregate's
 * culprit list, an `invalid` verdict is framable by a coordinator that
 * supplies a mismatched package/root, so it is an INPUT to the Go host's f+1
 * envelope-bound adjudication (Phase 7.2b spec, section 6), NOT authoritative
 * blame on its own.
 */
TbtcSignerResult frost_tbtc_verify_signature_share(const uint8_t* request_ptr, size_t request_len);
/*
 * Phase 7.3 interactive attempt-context derivation. Stateless and secret-free:
 * derives the canonical attempt context (coordinator, included-participants
 * fingerprint, attempt id) and the per-participant FROST identifiers the host
 * feeds into frost_tbtc_interactive_session_open, so the host never
 * re-implements the engine's domain-separated derivations. The returned context
 * is re-validated against the same strict-mode check session_open runs, so the
 * engine is guaranteed to accept it. No DKG/nonce/session state is touched.
 */
TbtcSignerResult frost_tbtc_derive_interactive_attempt_context(const uint8_t* request_ptr, size_t request_len);

#ifdef __cplusplus
}
#endif

#endif
