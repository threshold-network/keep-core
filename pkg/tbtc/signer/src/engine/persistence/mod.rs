//! Persistence layer for the FROST signer engine.
//!
//! Split from a single 2,300-line `persistence.rs` (August 2026) into four
//! `pub(crate)` submodules, each owning one concern:
//!
//! - [`schema_codec`] — pure: encode/decode/`TryFrom`, schema-version validation.
//! - [`envelope_io`] — file I/O, lock, atomic rename, corrupt recovery, backup retention.
//! - [`key_provider`] — `StateKeyProvider` trait + 3 adapters + subprocess machinery.
//! - [`pending_ops`] — marker registry + snapshot covering + durable retry.
//!
//! Glob re-exports keep call sites (`crate::engine::persistence::*`,
//! `crate::engine::tests::*`) unchanged from the pre-split module.
//! See `docs/specs/frost-signer-persistence-deepening.md` for context.

use super::*;

mod envelope_io;
mod key_provider;
mod pending_ops;
mod schema_codec;

pub(crate) use envelope_io::*;
pub(crate) use key_provider::*;
pub(crate) use pending_ops::*;
pub(crate) use schema_codec::*;
