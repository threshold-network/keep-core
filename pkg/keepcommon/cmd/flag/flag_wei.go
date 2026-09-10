package flag

import (
	"github.com/keep-network/keep-common/pkg/chain/ethereum"
	"github.com/spf13/pflag"
)

// WeiVarFlag is a custom flag to handle `ethereum.Wei` type, that is not supported
// by `pflag.FlagSet`.
func WeiVarFlag(f *pflag.FlagSet, p *ethereum.Wei, name string, value ethereum.Wei, usage string) {
	WeiVarPFlag(f, p, name, "", value, usage)
}

// WeiVarPFlag is a custom flag to handle `ethereum.Wei` type, that is not supported
// by `pflag.FlagSet`.
func WeiVarPFlag(f *pflag.FlagSet, p *ethereum.Wei, name string, short string, value ethereum.Wei, usage string) {
	f.VarP(newWeiValue(value, p), name, short, usage)
}

type weiValue ethereum.Wei

func newWeiValue(val ethereum.Wei, p *ethereum.Wei) *weiValue {
	*p = val
	return (*weiValue)(p)
}

func (w *weiValue) Set(s string) error {
	v := ethereum.Wei{}
	err := v.UnmarshalText([]byte(s))
	*w = weiValue(v)
	return err
}

func (w *weiValue) Type() string {
	return "wei"
}

func (w *weiValue) String() string { return (*ethereum.Wei)(w).String() }
