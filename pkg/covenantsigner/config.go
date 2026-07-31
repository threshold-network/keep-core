package covenantsigner

const DefaultListenAddress = "127.0.0.1"

// Config configures the covenant signer HTTP service.
type Config struct {
	// Port enables the covenant signer provider HTTP surface when non-zero.
	Port int
	// ListenAddress controls which interface the covenant signer HTTP service
	// binds to. Empty defaults to loopback-only.
	ListenAddress string
	// AuthToken enables static Bearer authentication for signer endpoints.
	// Non-loopback binds must set this. Prefer environment variables or
	// config files over CLI flags to avoid exposing the token in
	// /proc/PID/cmdline.
	AuthToken string
	// EnableSelfV1 exposes the self_v1 signer HTTP routes. Keep this disabled
	// for a qc_v1-first launch unless self_v1 has cleared its own go-live gate.
	EnableSelfV1 bool
	// RequireApprovalTrustRoots turns missing route-level approval trust roots
	// from startup warnings into startup errors. This does not prove every
	// reserve/network launch scope is provisioned; request-time validation still
	// enforces exact route/reserve/network matches for configured entries.
	RequireApprovalTrustRoots bool `mapstructure:"requireApprovalTrustRoots"`
	// MigrationPlanQuoteTrustRoots configures the destination-service plan-quote
	// trust roots used to verify migration plan quotes when the quote authority
	// path is enabled.
	MigrationPlanQuoteTrustRoots []MigrationPlanQuoteTrustRoot `mapstructure:"migrationPlanQuoteTrustRoots"`
	// DepositorTrustRoots configures independently pinned depositor public keys
	// by route/reserve/network for self_v1 approval verification.
	DepositorTrustRoots []DepositorTrustRoot `mapstructure:"depositorTrustRoots"`
	// CustodianTrustRoots configures independently pinned custodian public keys
	// by route/reserve/network for qc_v1 approval verification.
	CustodianTrustRoots []CustodianTrustRoot `mapstructure:"custodianTrustRoots"`
	// MinActiveOutpointConfirmations sets the minimum number of Bitcoin
	// confirmations required for an active outpoint transaction before the
	// covenant signer accepts it. When zero (unset), the system defaults to 6
	// to align with the deposit sweep finality threshold.
	MinActiveOutpointConfirmations uint `mapstructure:"minActiveOutpointConfirmations"`
	// BridgeCovenantFraudDefenseConfirmed must be set only when the operator has
	// confirmed that the tBTC Bridge recognizes covenant active UTXO spends as
	// honest spends in Fraud.defeatFraudChallenge (the covenant fraud-defense
	// path is deployed). Until then the covenant signer fails closed and refuses
	// to produce signatures, because a covenant signature over a covenant active
	// UTXO would otherwise be a valid, undefeatable tBTC fraud proof that could
	// slash the signing wallet.
	BridgeCovenantFraudDefenseConfirmed bool `mapstructure:"bridgeCovenantFraudDefenseConfirmed"`
	// DataDir is the base directory path used by the disk persistence handle.
	// When set, the store acquires an exclusive file lock to prevent concurrent
	// process corruption. When empty, file locking is skipped.
	DataDir string `mapstructure:"dataDir"`
	// EIP712ChainID pins the chainId of the EIP-712 domain used to wrap the v2
	// artifact approval digest. It must equal the chain the depositor's wallet
	// signs on (eth_signTypedData_v4 enforces the active chain matches). Set
	// this to the covenant's Ethereum chain (e.g. 1 mainnet, 11155111 Sepolia).
	EIP712ChainID uint64 `mapstructure:"eip712ChainId"`
	// EIP712Salt optionally overrides the EIP-712 domain salt (32-byte hex).
	// When empty, a fixed program-namespace salt is used. It must match the
	// client (wallet / covenant-manager / dashboard) domain construction.
	EIP712Salt string `mapstructure:"eip712Salt"`
}
