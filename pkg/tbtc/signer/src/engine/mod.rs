//! tBTC FROST/ROAST signer engine.
//!
//! Split from a single 18k-line `engine.rs` (June 2026) as a pure code
//! move: behavior, the `engine::*` API consumed by lib.rs, and the
//! `engine::tests::*` test paths are unchanged. Submodule items are
//! `pub(crate)` and glob re-exported here; `mod engine` itself remains
//! private to the crate, so the crate-external surface is identical.
//!
//! - [`audit`] — Forensics: transcript audit, blame-proof verification, differential fuzzing references.
//! - [`codec`] — Hex/struct codecs and Go<->frost identifier conversions.
//! - [`config`] — TBTC_SIGNER_* environment surface: constant names, defaults, and parsers.
//! - [`dkg`] — distributed-DKG key-package persistence (`persist_distributed_dkg_key_package`).
//! - [`frost_ops`] — Stateless FROST primitives: dkg_part1..3 and signing-package assembly.
//! - [`interactive`] — Phase 7.1 hardened interactive signing session: engine-held nonce custody, Round1/Round2, consumption markers.
//! - [`lifecycle`] — Operational lifecycle: canary rollout, refresh cadence/shares, emergency rekey, quarantine status.
//! - [`persistence`] — Encrypted state-file persistence: envelope codec, key providers, corruption recovery, persisted<->live conversions.
//! - [`policy`] — Admission, signing-policy firewall, rate limiting, and auto-quarantine enforcement.
//! - [`provenance`] — Runtime provenance attestation gate.
//! - [`roast`] — ROAST/RFC-21 attempt machinery: request fingerprints, round/attempt ids, attempt-context and transition-evidence validation.
//! - [`state`] — In-memory engine/session state, the state-file lock, and registry capacity guards.
//! - [`telemetry`] — Hardening telemetry: latency trackers and metrics reporting.
//! - [`transaction`] — Taproot transaction building.
//! - [`testsupport`] — Cross-module test helpers (cfg(test)): state lock, reset, restart simulation.
//! - [`tests`] — the full engine test suite (single module, stable paths).

use bitcoin::{
    absolute::LockTime,
    consensus::encode::{deserialize, serialize_hex},
    hashes::Hash as BitcoinHash,
    secp256k1::{
        schnorr::Signature as SchnorrSignature, Message as SecpMessage, Secp256k1, XOnlyPublicKey,
    },
    sighash::{Prevouts, SighashCache, TapSighashType},
    transaction::Version,
    Amount, OutPoint, ScriptBuf, Sequence, Transaction, TxIn, TxOut, Txid, Witness,
};
use chacha20poly1305::aead::{Aead, KeyInit, OsRng, Payload};
use chacha20poly1305::{XChaCha20Poly1305, XNonce};
#[cfg(unix)]
use libc::{flock, EAGAIN, EWOULDBLOCK, LOCK_EX, LOCK_NB};
use std::collections::{BTreeMap, BTreeSet, HashMap, HashSet, VecDeque};
use std::fs;
use std::io::{Read, Write};
#[cfg(unix)]
use std::os::unix::fs::OpenOptionsExt;
#[cfg(unix)]
use std::os::unix::process::CommandExt;
use std::path::{Path, PathBuf};
use std::process::{Output, Stdio};
use std::str::FromStr;
use std::sync::{mpsc, Mutex, OnceLock};
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};

use frost_secp256k1_tr::{
    self as frost,
    keys::{EvenY, Tweak},
};
use rand_chacha::rand_core::{CryptoRng, Error as RandCoreError, RngCore, SeedableRng};
use rand_chacha::ChaCha20Rng;
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use zeroize::{Zeroize, Zeroizing};

use crate::api::{
    AttemptContext, BlameProofVerificationResult, BuildTaprootTxRequest, CanaryRolloutStatusResult,
    DeriveInteractiveAttemptContextRequest, DeriveInteractiveAttemptContextResult,
    DifferentialDivergence, DifferentialFuzzRequest, DifferentialFuzzResult, DkgPart1Request,
    DkgPart1Result, DkgPart2Request, DkgPart2Result, DkgPart3Request, DkgPart3Result, DkgResult,
    DkgRound1Package, DkgRound2Package, InitSignerConfigRequest, InitSignerConfigResult,
    InteractiveAggregateRequest, InteractiveAggregateResult, InteractiveRound1Request,
    InteractiveRound1Result, InteractiveRound2Request, InteractiveRound2Result,
    InteractiveSessionAbortRequest, InteractiveSessionAbortResult, InteractiveSessionOpenRequest,
    InteractiveSessionOpenResult, InteractiveSigningIntent, NativeFrostCommitment,
    NativeFrostKeyPackage, NativeFrostPublicKeyPackage, NativeFrostSignatureShare,
    NewSigningPackageRequest, NewSigningPackageResult, ParticipantFrostIdentifier,
    PersistDistributedDkgKeyPackageRequest, PromoteCanaryRequest, PromoteCanaryResult,
    QuarantineStatusRequest, QuarantineStatusResult, RefreshCadenceStatusRequest,
    RefreshCadenceStatusResult, RefreshSharesRequest, RefreshSharesResult,
    RoastLivenessPolicyResult, RollbackCanaryRequest, RollbackCanaryResult, RoundState,
    SignatureResult, SignerHardeningMetricsResult, TransactionResult, TranscriptAuditRecord,
    TranscriptAuditRequest, TranscriptAuditResult, TriggerEmergencyRekeyRequest,
    TriggerEmergencyRekeyResult, VerifyBlameProofRequest,
};
use crate::errors::EngineError;
use crate::go_math_rand::select_coordinator_identifier;

mod audit;
mod codec;
mod config;
mod dkg;
mod frost_ops;
mod init_config;
mod interactive;
mod lifecycle;
mod persistence;
mod policy;
mod provenance;
mod roast;
mod state;
mod telemetry;
#[cfg(test)]
mod tests;
#[cfg(test)]
mod testsupport;
mod transaction;
mod verify_share;

pub(crate) use audit::*;
pub(crate) use codec::*;
pub(crate) use config::*;
pub(crate) use dkg::*;
pub(crate) use frost_ops::*;
pub(crate) use init_config::*;
pub(crate) use interactive::*;
pub(crate) use lifecycle::*;
pub(crate) use persistence::*;
pub(crate) use policy::*;
pub(crate) use provenance::*;
pub(crate) use roast::*;
pub(crate) use state::*;
pub(crate) use telemetry::*;
#[cfg(test)]
pub(crate) use testsupport::*;
pub(crate) use transaction::*;
pub(crate) use verify_share::*;
