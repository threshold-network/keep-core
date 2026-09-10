package ethereum

import (
	"math/big"
)

// Units defines denominations of the Ether token.
var Units = map[string]int64{
	"wei":   1, // default unit
	"gwei":  1e9,
	"ether": 1e18,
}

// Wei is a custom type to handle Ether value parsing in configuration files
// using BurntSushi/toml package. It supports wei, Gwei and ether units. The
// Ether value is kept as `wei` and `wei` is the default unit.
// The value can be provided in the text file as e.g.: `1 wei`, `200 Gwei` or
// `0.5 ether`.
type Wei struct {
	Token
}

// WrapWei wraps the given integer value in order to represent it as Wei value.
func WrapWei(value *big.Int) *Wei {
	return &Wei{Token{value}}
}

// UnmarshalText is a function used to parse a value of Ethers.
func (w *Wei) UnmarshalText(text []byte) error {
	return w.UnmarshalToken(text, Units)
}

func (w *Wei) String() string {
	return w.Token.MarshalToken(Units)
}
