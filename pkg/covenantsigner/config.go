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
	// Non-loopback binds must set this.
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
}
