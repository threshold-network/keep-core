package network

import (
	"github.com/keep-network/keep-common/pkg/chain/ethereum"
	"github.com/keep-network/keep-core/pkg/bitcoin"
)

// Type is a type used for networks enumeration.
type Type int

// Network types enumeration.
const (
	Unknown Type = iota
	Mainnet
	Testnet
	Developer
)

func (n Type) String() string {
	return []string{
		"unknown",
		"mainnet",
		"testnet",
		"developer",
	}[n]
}

// Ethereum returns Ethereum network corresponding to the given client network.
func (n Type) Ethereum() ethereum.Network {
	return []ethereum.Network{
		ethereum.Unknown,
		ethereum.Mainnet,
		ethereum.Sepolia,
		ethereum.Developer,
	}[n]
}

// Bitcoin returns Bitcoin network corresponding to the given client network.
func (n Type) Bitcoin() bitcoin.Network {
	return []bitcoin.Network{
		bitcoin.Unknown,
		bitcoin.Mainnet,
		bitcoin.Testnet4, // Sepolia uses Bitcoin testnet4
		bitcoin.Regtest,
	}[n]
}
