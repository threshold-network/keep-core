package config

type Category int

const (
	General Category = iota
	Ethereum
	BitcoinElectrum
	Network
	Storage
	ClientInfo
	CovenantSigner
	Tbtc
	Maintainer
	Developer
)

// StartCmdCategories are categories needed for the start command.
var StartCmdCategories = []Category{
	General,
	Ethereum,
	BitcoinElectrum,
	Network,
	Storage,
	ClientInfo,
	CovenantSigner,
	Tbtc,
	Developer,
}

// MaintainerCategories are categories needed for the maintainer command.
var MaintainerCategories = []Category{
	Ethereum,
	BitcoinElectrum,
	Maintainer,
}

// AllCategories are all available categories.
var AllCategories = []Category{
	General,
	Ethereum,
	BitcoinElectrum,
	Network,
	Storage,
	ClientInfo,
	CovenantSigner,
	Tbtc,
	Maintainer,
	Developer,
}
