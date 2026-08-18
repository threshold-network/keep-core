/// State-encryption key providers: `StateKeyProvider` trait with
/// `EnvKeyProvider` / `CommandKeyProvider` adapters and a process-lifetime
/// `CachedStateKeyProvider` decorator. Also owns the subprocess machinery
/// that runs the command provider's KMS/HSM subprocess. Moved from
/// `persistence.rs` as part of the C2 persistence-deepening refactor.

use super::*;
use std::sync::{LazyLock, Mutex};

#[derive(Clone)]
pub(crate) struct StateEncryptionKeyMaterial {
    pub(crate) key: Zeroizing<[u8; 32]>,
    pub(crate) key_provider: &'static str,
    pub(crate) key_id: String,
}

#[cfg(test)]
pub(crate) const TEST_STATE_ENCRYPTION_KEY_HEX: &str =
    "1111111111111111111111111111111111111111111111111111111111111111";

pub(crate) fn state_key_command_timeout_secs() -> u64 {
    signer_env_var(TBTC_SIGNER_STATE_KEY_COMMAND_TIMEOUT_SECS_ENV)
        .and_then(|value| value.trim().parse::<u64>().ok())
        .filter(|value| {
            *value >= TBTC_SIGNER_MIN_STATE_KEY_COMMAND_TIMEOUT_SECS
                && *value <= TBTC_SIGNER_MAX_STATE_KEY_COMMAND_TIMEOUT_SECS
        })
        .unwrap_or(TBTC_SIGNER_DEFAULT_STATE_KEY_COMMAND_TIMEOUT_SECS)
}

pub(crate) fn decode_state_encryption_key_hex(
    mut raw_key_hex: String,
    source_label: &str,
) -> Result<Zeroizing<[u8; 32]>, EngineError> {
    let key_len = raw_key_hex.trim().len();
    if key_len != 64 {
        raw_key_hex.zeroize();
        return Err(EngineError::Internal(format!(
            "state encryption key from [{}] must be exactly 64 hex chars (32 bytes)",
            source_label
        )));
    }
    let trimmed_key_hex = raw_key_hex.trim().to_string();
    raw_key_hex.zeroize();

    let decode_result = hex::decode(&trimmed_key_hex);
    let mut trimmed_key_hex = trimmed_key_hex;
    trimmed_key_hex.zeroize();
    let mut key_bytes = decode_result.map_err(|_| {
        EngineError::Internal(format!(
            "state encryption key from [{}] must be valid hex",
            source_label
        ))
    })?;

    if key_bytes.len() != 32 {
        key_bytes.zeroize();
        return Err(EngineError::Internal(format!(
            "state encryption key from [{}] must decode to exactly 32 bytes",
            source_label
        )));
    }

    let mut key = [0u8; 32];
    key.copy_from_slice(&key_bytes);
    key_bytes.zeroize();
    Ok(Zeroizing::new(key))
}

pub(crate) fn state_key_identifier(key: &[u8; 32]) -> String {
    format!("sha256:{}", hex::encode(hash_bytes(key)))
}

pub(crate) fn drain_command_pipe<R>(mut pipe: R) -> mpsc::Receiver<std::io::Result<Vec<u8>>>
where
    R: Read + Send + 'static,
{
    let (sender, receiver) = mpsc::channel();
    std::thread::spawn(move || {
        let mut bytes = Vec::new();
        let result = match pipe.read_to_end(&mut bytes) {
            Ok(_) => Ok(bytes),
            Err(err) => {
                bytes.zeroize();
                Err(err)
            }
        };
        if let Err(mpsc::SendError(Ok(mut bytes))) = sender.send(result) {
            bytes.zeroize();
        }
    });
    receiver
}

pub(crate) fn read_command_pipe(
    receiver: mpsc::Receiver<std::io::Result<Vec<u8>>>,
    stream_name: &str,
    timeout: Duration,
) -> Result<Vec<u8>, EngineError> {
    match receiver.recv_timeout(timeout) {
        Ok(Ok(bytes)) => Ok(bytes),
        Ok(Err(e)) => Err(EngineError::Internal(format!(
            "failed to read state key command {stream_name}: {e}"
        ))),
        Err(mpsc::RecvTimeoutError::Timeout) => Err(EngineError::Internal(format!(
            "state key command {stream_name} pipe timed out waiting for EOF"
        ))),
        Err(mpsc::RecvTimeoutError::Disconnected) => Err(EngineError::Internal(format!(
            "state key command {stream_name} reader exited without a result"
        ))),
    }
}

pub(crate) fn zeroize_command_pipe_if_ready(receiver: mpsc::Receiver<std::io::Result<Vec<u8>>>) {
    if let Ok(Ok(mut bytes)) = receiver.try_recv() {
        bytes.zeroize();
    }
}

#[cfg(unix)]
pub(crate) fn configure_state_key_command_process_group(command: &mut std::process::Command) {
    unsafe {
        command.pre_exec(|| {
            if libc::setpgid(0, 0) == 0 {
                Ok(())
            } else {
                Err(std::io::Error::last_os_error())
            }
        });
    }
}

#[cfg(not(unix))]
pub(crate) fn configure_state_key_command_process_group(_command: &mut std::process::Command) {}

#[cfg(unix)]
pub(crate) fn kill_state_key_command_process_group(child_id: u32) {
    let pgid = -(child_id as i32);
    unsafe {
        let _ = libc::kill(pgid, libc::SIGKILL);
    }
}

#[cfg(not(unix))]
pub(crate) fn kill_state_key_command_process_group(_child_id: u32) {}

pub(crate) fn terminate_state_key_command(child: &mut std::process::Child, child_id: u32) {
    kill_state_key_command_process_group(child_id);
    let _ = child.kill();
    let _ = child.wait();
}

pub(crate) fn remaining_timeout(deadline: Instant) -> Duration {
    deadline
        .checked_duration_since(Instant::now())
        .unwrap_or(Duration::ZERO)
}

pub(crate) fn execute_state_key_command(command_spec: &str) -> Result<Output, EngineError> {
    let timeout_secs = state_key_command_timeout_secs();
    let timeout = Duration::from_secs(timeout_secs);
    let deadline = Instant::now() + timeout;
    let mut command = std::process::Command::new("/bin/sh");
    command
        .arg("-c")
        .arg(command_spec)
        .stdout(Stdio::piped())
        .stderr(Stdio::piped());
    configure_state_key_command_process_group(&mut command);

    let mut child = command.spawn().map_err(|e| {
        EngineError::Internal(format!(
            "failed to execute state key command from [{}]: {e}",
            TBTC_SIGNER_STATE_KEY_COMMAND_ENV
        ))
    })?;
    let child_id = child.id();
    let stdout = child.stdout.take().ok_or_else(|| {
        EngineError::Internal("state key command stdout pipe unavailable".to_string())
    })?;
    let stderr = child.stderr.take().ok_or_else(|| {
        EngineError::Internal("state key command stderr pipe unavailable".to_string())
    })?;
    let stdout_receiver = drain_command_pipe(stdout);
    let stderr_receiver = drain_command_pipe(stderr);
    let started_at = Instant::now();

    loop {
        match child.try_wait().map_err(|e| {
            EngineError::Internal(format!(
                "failed while waiting for state key command from [{}]: {e}",
                TBTC_SIGNER_STATE_KEY_COMMAND_ENV
            ))
        })? {
            Some(status) => {
                let stdout_result =
                    read_command_pipe(stdout_receiver, "stdout", remaining_timeout(deadline));
                let stdout = match stdout_result {
                    Ok(stdout) => stdout,
                    Err(err) => {
                        terminate_state_key_command(&mut child, child_id);
                        zeroize_command_pipe_if_ready(stderr_receiver);
                        return Err(err);
                    }
                };
                let stderr_result =
                    read_command_pipe(stderr_receiver, "stderr", remaining_timeout(deadline));
                let stderr = match stderr_result {
                    Ok(stderr) => stderr,
                    Err(err) => {
                        let mut stdout = stdout;
                        stdout.zeroize();
                        terminate_state_key_command(&mut child, child_id);
                        return Err(err);
                    }
                };
                return Ok(Output {
                    status,
                    stdout,
                    stderr,
                });
            }
            None => {
                if started_at.elapsed() >= Duration::from_secs(timeout_secs) {
                    terminate_state_key_command(&mut child, child_id);
                    zeroize_command_pipe_if_ready(stdout_receiver);
                    zeroize_command_pipe_if_ready(stderr_receiver);
                    return Err(EngineError::Internal(format!(
                        "state key command from [{}] timed out after [{}] seconds",
                        TBTC_SIGNER_STATE_KEY_COMMAND_ENV, timeout_secs
                    )));
                }
                std::thread::sleep(Duration::from_millis(25));
            }
        }
    }
}

pub(crate) trait StateKeyProvider: Send + Sync {
    fn material(&self) -> Result<StateEncryptionKeyMaterial, EngineError>;
    fn key_id(&self) -> &str;
}

pub(crate) struct EnvKeyProvider;

impl EnvKeyProvider {
    fn new() -> Result<Self, EngineError> {
        if signer_profile_is_production() {
            return Err(EngineError::Internal(format!(
                "state key provider [{}] is not allowed in profile [{}]; configure [{}]={} with [{}] returning a 32-byte hex key sourced from KMS/HSM",
                TBTC_SIGNER_STATE_KEY_PROVIDER_ENV_DEFAULT,
                TBTC_SIGNER_PROFILE_PRODUCTION,
                TBTC_SIGNER_STATE_KEY_PROVIDER_ENV,
                TBTC_SIGNER_STATE_KEY_PROVIDER_COMMAND,
                TBTC_SIGNER_STATE_KEY_COMMAND_ENV
            )));
        }
        Ok(Self)
    }
}

impl StateKeyProvider for EnvKeyProvider {
    fn material(&self) -> Result<StateEncryptionKeyMaterial, EngineError> {
        // Deliberately read from the real environment even when an init-time
        // config is installed: the state-encryption key is a secret and the
        // config FFI carries operational knobs only (see signer_env_var).
        let raw_key_hex =
            std::env::var(TBTC_SIGNER_STATE_ENCRYPTION_KEY_HEX_ENV).map_err(|_| {
                EngineError::Internal(format!(
                    "missing required state encryption key env [{}]",
                    TBTC_SIGNER_STATE_ENCRYPTION_KEY_HEX_ENV
                ))
            })?;
        let key = decode_state_encryption_key_hex(
            raw_key_hex,
            TBTC_SIGNER_STATE_ENCRYPTION_KEY_HEX_ENV,
        )?;
        let key_id = state_key_identifier(&key);
        Ok(StateEncryptionKeyMaterial {
            key,
            key_provider: TBTC_SIGNER_STATE_KEY_PROVIDER_ENV_DEFAULT,
            key_id,
        })
    }

    fn key_id(&self) -> &str {
        TBTC_SIGNER_STATE_ENCRYPTION_KEY_HEX_ENV
    }
}

pub(crate) struct CommandKeyProvider {
    command_spec: String,
}

impl CommandKeyProvider {
    fn new(command_spec: String) -> Result<Self, EngineError> {
        if command_spec.trim().is_empty() {
            return Err(EngineError::Internal(format!(
                "state key command env [{}] must be non-empty",
                TBTC_SIGNER_STATE_KEY_COMMAND_ENV
            )));
        }
        Ok(Self { command_spec })
    }
}

impl StateKeyProvider for CommandKeyProvider {
    fn material(&self) -> Result<StateEncryptionKeyMaterial, EngineError> {
        let mut output = execute_state_key_command(&self.command_spec)?;

        if !output.status.success() {
            output.stdout.zeroize();
            output.stderr.zeroize();
            return Err(EngineError::Internal(format!(
                "state key command from [{}] exited with non-zero status [{}]",
                TBTC_SIGNER_STATE_KEY_COMMAND_ENV, output.status
            )));
        }

        let command_stdout_bytes = std::mem::take(&mut output.stdout);
        output.stderr.zeroize();
        let mut command_stdout = String::from_utf8(command_stdout_bytes).map_err(|error| {
            let mut command_stdout_raw = error.into_bytes();
            command_stdout_raw.zeroize();
            EngineError::Internal(format!(
                "state key command from [{}] must output UTF-8 hex key bytes",
                TBTC_SIGNER_STATE_KEY_COMMAND_ENV
            ))
        })?;
        let key = decode_state_encryption_key_hex(
            std::mem::take(&mut command_stdout),
            TBTC_SIGNER_STATE_KEY_COMMAND_ENV,
        )?;
        command_stdout.zeroize();
        let key_id = state_key_identifier(&key);
        Ok(StateEncryptionKeyMaterial {
            key,
            key_provider: TBTC_SIGNER_STATE_KEY_PROVIDER_COMMAND,
            key_id,
        })
    }

    fn key_id(&self) -> &str {
        &self.command_spec
    }
}

pub(crate) struct CachedStateKeyProvider {
    inner: Box<dyn StateKeyProvider>,
}

static STATE_KEY_CACHE: LazyLock<Mutex<Option<(String, StateEncryptionKeyMaterial)>>> =
    LazyLock::new(|| Mutex::new(None));


impl StateKeyProvider for CachedStateKeyProvider {
    fn material(&self) -> Result<StateEncryptionKeyMaterial, EngineError> {
        let cache_key = self.inner.key_id().to_string();
        let mut guard = STATE_KEY_CACHE
            .lock()
            .unwrap_or_else(std::sync::PoisonError::into_inner);
        if let Some((cached_key, cached_material)) = guard.as_ref() {
            if cached_key == &cache_key {
                return Ok(cached_material.clone());
            }
        }
        let material = self.inner.material()?;
        *guard = Some((cache_key, material.clone()));
        Ok(material)
    }

    fn key_id(&self) -> &str {
        self.inner.key_id()
    }
}

pub(crate) fn resolve_state_key_provider() -> Result<Box<dyn StateKeyProvider>, EngineError> {
    let provider = signer_env_var(TBTC_SIGNER_STATE_KEY_PROVIDER_ENV)
        .map(|value| value.trim().to_ascii_lowercase())
        .unwrap_or_else(|| TBTC_SIGNER_STATE_KEY_PROVIDER_ENV_DEFAULT.to_string());

    let inner: Box<dyn StateKeyProvider> = match provider.as_str() {
        TBTC_SIGNER_STATE_KEY_PROVIDER_ENV_DEFAULT => Box::new(EnvKeyProvider::new()?),
        TBTC_SIGNER_STATE_KEY_PROVIDER_COMMAND => {
            let command_spec =
                signer_env_var(TBTC_SIGNER_STATE_KEY_COMMAND_ENV).ok_or_else(|| {
                    EngineError::Internal(format!(
                        "missing required state key command env [{}]",
                        TBTC_SIGNER_STATE_KEY_COMMAND_ENV
                    ))
                })?;
            Box::new(CommandKeyProvider::new(command_spec)?)
        }
        _ => {
            return Err(EngineError::Internal(format!(
                "unsupported state key provider [{}]; expected [{}] or [{}]",
                provider,
                TBTC_SIGNER_STATE_KEY_PROVIDER_ENV_DEFAULT,
                TBTC_SIGNER_STATE_KEY_PROVIDER_COMMAND
            )));
        }
    };

    Ok(Box::new(CachedStateKeyProvider { inner }))
}

pub(crate) fn state_encryption_key_material() -> Result<StateEncryptionKeyMaterial, EngineError> {
    resolve_state_key_provider()?.material()
}
