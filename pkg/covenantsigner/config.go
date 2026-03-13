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
	// MigrationPlanQuoteTrustRoots configures the destination-service plan-quote
	// trust roots used to verify migration plan quotes when the quote authority
	// path is enabled.
	MigrationPlanQuoteTrustRoots []MigrationPlanQuoteTrustRoot `mapstructure:"migrationPlanQuoteTrustRoots"`
}
