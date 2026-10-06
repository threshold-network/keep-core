package electrum

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
)

// AddressToElectrumScriptHash converts valid bitcoin address to electrum scriptHash sha256 encoded, reversed and encoded in hex
// https://electrumx.readthedocs.io/en/latest/protocol-basics.html#script-hashes
func AddressToElectrumScriptHash(addressStr string) (string, error) {
	address, err := btcutil.DecodeAddress(addressStr, &chaincfg.MainNetParams)
	if err != nil {
		return "", err
	}
	script, err := txscript.PayToAddrScript(address)
	if err != nil {
		return "", err
	}

	hashSum := sha256.Sum256(script)

	for i, j := 0, len(hashSum)-1; i < j; i, j = i+1, j-1 {
		hashSum[i], hashSum[j] = hashSum[j], hashSum[i]
	}

	return hex.EncodeToString(hashSum[:]), nil
}
