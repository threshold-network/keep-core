//go:build frost_native

package tbtc

import (
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
)

func TestArchiveClosedWalletsDefersUnregisteredFrostMaterial(t *testing.T) {
	node, signer, chain := setupNodeWithChain(t)
	walletPublicKeyHash := bitcoin.PublicKeyHash(signer.wallet.publicKey)

	const xOnlyOutputKey = "79be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798"
	payload, err := json.Marshal(frostsigning.NativeTBTCSignerMaterialPayload{
		KeyGroup:         "03" + xOnlyOutputKey,
		TaprootOutputKey: xOnlyOutputKey,
		KeyGroupSource:   frostsigning.NativeTBTCSignerKeyGroupSourceDKGPersisted,
	})
	if err != nil {
		t.Fatal(err)
	}
	nativeMaterial := &frostsigning.NativeSignerMaterial{
		Format:  frostsigning.NativeSignerMaterialFormatFrostTBTCSignerV1,
		Payload: payload,
	}

	decodedWalletID, err := hex.DecodeString(xOnlyOutputKey)
	if err != nil {
		t.Fatal(err)
	}
	var walletID [32]byte
	copy(walletID[:], decodedWalletID)

	node.walletRegistry.mutex.Lock()
	for _, value := range node.walletRegistry.walletCache {
		if value.walletPublicKeyHash == walletPublicKeyHash {
			value.walletID = walletID
			value.signers[0].signerMaterial = nativeMaterial
		}
	}
	node.walletRegistry.mutex.Unlock()

	chain.walletsMutex.Lock()
	delete(chain.wallets, walletPublicKeyHash)
	chain.frostWalletRegistryAvailable = true
	chain.walletsMutex.Unlock()

	if err := node.archiveClosedWallets(); err != nil {
		t.Fatal(err)
	}
	if _, ok := node.walletRegistry.getWalletByPublicKeyHash(
		walletPublicKeyHash,
	); !ok {
		t.Fatal("unregistered FROST material was archived before DKG reconciliation")
	}
}
