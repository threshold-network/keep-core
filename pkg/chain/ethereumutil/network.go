package ethereum

// Network is a type used for ethereum networks enumeration.
type Network int

// Ethereum networks enumeration.
const (
	Unknown Network = iota
	Mainnet
	Sepolia
	Developer
)

func (n Network) String() string {
	return []string{"unknown", "mainnet", "sepolia", "developer"}[n]
}

// ChainID returns chain id associated with the network.
func (n Network) ChainID() int64 {
	switch n {
	case Mainnet:
		return 1
	case Sepolia:
		return 11155111
	}
	return 0
}
