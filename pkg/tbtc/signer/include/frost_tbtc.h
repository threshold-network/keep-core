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

TbtcSignerResult frost_tbtc_run_dkg(const uint8_t* request_ptr, size_t request_len);
TbtcSignerResult frost_tbtc_start_sign_round(const uint8_t* request_ptr, size_t request_len);
TbtcSignerResult frost_tbtc_finalize_sign_round(const uint8_t* request_ptr, size_t request_len);
TbtcSignerResult frost_tbtc_build_taproot_tx(const uint8_t* request_ptr, size_t request_len);
TbtcSignerResult frost_tbtc_refresh_shares(const uint8_t* request_ptr, size_t request_len);

#ifdef __cplusplus
}
#endif

#endif
