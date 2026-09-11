package ethereum

import (
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"

	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/chain/ethereumutil/ethutil"
	"github.com/keep-network/keep-core/pkg/operator"
)

// TODO: Consider moving the `EthereumSigner` out of `ethutil` to this file.
type signer struct {
	*ethutil.EthereumSigner
}

func newSigner(chainKey *keystore.Key) *signer {
	return &signer{
		ethutil.NewSigner(chainKey.PrivateKey),
	}
}

// Address returns operator's address.
func (s *signer) Address() chain.Address {
	return s.PublicKeyBytesToAddress(s.PublicKey())
}

func (s *signer) PublicKeyToAddress(
	publicKey *operator.PublicKey,
) (chain.Address, error) {
	chainPublicKey, err := operatorPublicKeyToChainPublicKey(publicKey)
	if err != nil {
		return "", fmt.Errorf(
			"cannot convert operator key to chain key: [%v]",
			err,
		)
	}

	addressBytes := s.EthereumSigner.PublicKeyToAddress(*chainPublicKey)

	return chain.Address(common.BytesToAddress(addressBytes).String()), nil
}

func (s *signer) PublicKeyBytesToAddress(publicKey []byte) chain.Address {
	addressBytes := s.EthereumSigner.PublicKeyBytesToAddress(publicKey)

	return chain.Address(common.BytesToAddress(addressBytes).String())
}

func (bc *baseChain) Signing() chain.Signing {
	return newSigner(bc.key)
}
