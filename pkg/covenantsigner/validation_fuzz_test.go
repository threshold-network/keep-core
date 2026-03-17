package covenantsigner

import "testing"

func FuzzParseMigrationPlanQuoteTrustRoot_NoPanic(f *testing.F) {
	f.Add("trustRoot", "not a pem")
	f.Add("trustRoot", "-----BEGIN PUBLIC KEY-----\nZm9v\n-----END PUBLIC KEY-----")

	f.Fuzz(func(t *testing.T, name string, publicKeyPEM string) {
		_, _ = parseMigrationPlanQuoteTrustRoot(
			name,
			MigrationPlanQuoteTrustRoot{
				KeyID:        "fuzz",
				PublicKeyPEM: publicKeyPEM,
			},
		)
	})
}
