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
/*
 * Returns the exact descriptor-bound durable session-store identity using the
 * tbtc-signer-durable-session-store-identity/v2 JSON schema. The call opens and
 * exclusively locks the store before reading any signer state and fails closed
 * if a live path, lock, store-ID, or state entry no longer matches its held
 * no-follow descriptor. This stable v2 identity does not attest pre-start
 * state freshness or the installed key-package inventory.
 */
TbtcSignerResult frost_tbtc_durable_store_identity(void);
/*
 * Returns the validated public-only retained FROST key-package inventory and
 * the exact dynamic state-witness tip using
 * tbtc-signer-retained-key-package-inventory/v1.
 */
TbtcSignerResult frost_tbtc_retained_key_package_inventory(void);
/*
 * Returns up to maximumEntries contiguous witness transitions from a known
 * ancestor to an exact historical target. Callers must persist accepted tips
 * independently of this signer store to detect a coordinated local rollback.
 */
TbtcSignerResult frost_tbtc_state_witness_proof(const uint8_t* request_ptr, size_t request_len);
/*
 * Returns tbtc-signer-state-witness-tip/v1 JSON. Decimal counters are strings.
 * The mandatory anchor fields are anchorBindingHash, anchorServiceEpoch,
 * anchorRevision, anchorEventRoot, and anchorAcknowledgementDigest; all five
 * are zero before an acknowledgement is durably accepted.
 */
TbtcSignerResult frost_tbtc_state_witness_tip(void);
/*
 * Accepts strict tbtc-signer-state-witness-checkpoint-ack/v1 camelCase JSON,
 * verifies the pinned Ed25519 service response, expiry and monotonic CAS, and
 * returns tbtc-signer-state-witness-checkpoint-ack-result/v1. The request's
 * operation identifier is spelled exactly `operationID`.
 */
TbtcSignerResult frost_tbtc_acknowledge_state_witness_checkpoint(
    const uint8_t* request_ptr,
    size_t request_len
);

/*
 * Recovers a remotely committed checkpoint from an unexpired
 * tbtc-frost-native-signer-state-anchor-read-response/v1 wrapper. The wrapper
 * must bind the exact raw nested historical acknowledgement.
 */
TbtcSignerResult frost_tbtc_recover_state_witness_checkpoint(
    const uint8_t* request_ptr,
    size_t request_len
);
/*
 * Verifies and durably applies a strict
 * tbtc-signer-state-anchor-trust-transition/v1 request. This operation is
 * startup-only: it must complete before ordinary signer engine/store access.
 * Certificate-chain and Read bytes are retained in a durable intent until the
 * transition completes. The full verified certificate chain and each
 * certificate's raw embedded target acknowledgement remain in the durable
 * audit journal.
 * Callers MUST invoke frost_tbtc_state_anchor_trust_head first on every
 * startup. If it reports state_anchor_trust_recovery_required, use its bounded
 * selector to choose the exact configured certificate chain, obtain a newly
 * signed target Read wrapper, and resubmit this request. Local intent bytes
 * never waive external freshness.
 * Returns tbtc-signer-state-anchor-trust-transition-result/v1.
 */
TbtcSignerResult frost_tbtc_transition_state_witness_anchor(
    const uint8_t* request_ptr,
    size_t request_len
);
/*
 * Required startup preflight returning the committed
 * tbtc-signer-state-anchor-trust-head/v1 record. Before ordinary store
 * initialization this performs an ephemeral descriptor-bound inspection, so a
 * preflight read does not consume the startup-only transition window. It
 * reports any durable in-progress transition without mutation as
 * state_anchor_trust_recovery_required. An unbootstrapped store returns
 * state_anchor_trust_head_absent.
 */
TbtcSignerResult frost_tbtc_state_anchor_trust_head(void);
/*
 * Provisioning-only startup preflight returning
 * tbtc-signer-state-anchor-bootstrap-facts/v1: the stable store fingerprint
 * and exact pristine genesis checkpoint needed for the first offline trust
 * certificate. Requires a production init config whose purpose is
 * state_anchor_bootstrap_provisioning and whose only other populated fields
 * are state_path and state_witness_max_records=4. Every signer, key, session,
 * policy, network, and anchor/trust field is forbidden.
 * The call is ephemeral, does not consume the normal signer startup window,
 * and rejects any store containing state, anchor/trust data, a segmented
 * witness, or witness history beyond the exact genesis image.
 */
TbtcSignerResult frost_tbtc_state_anchor_bootstrap_facts(void);
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
TbtcSignerResult frost_tbtc_retire_distributed_dkg_key_packages(const uint8_t* request_ptr, size_t request_len);

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
