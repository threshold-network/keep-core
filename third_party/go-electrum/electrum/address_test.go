package electrum

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAddressToElectrumScriptHash(t *testing.T) {
	tests := []struct {
		address        string
		wantScriptHash string
	}{
		{
			address:        "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
			wantScriptHash: "8b01df4e368ea28f8dc0423bcf7a4923e3a12d307c875e47a0cfbf90b5c39161",
		},
		{
			address:        "34xp4vRoCGJym3xR7yCVPFHoCNxv4Twseo",
			wantScriptHash: "2375f2bbf7815e3cdc835074b052d65c9b2f101bab28d37250cc96b2ed9a6809",
		},
	}

	for _, tc := range tests {
		scriptHash, err := AddressToElectrumScriptHash(tc.address)
		require.NoError(t, err)
		assert.Equal(t, scriptHash, tc.wantScriptHash)
	}
}
