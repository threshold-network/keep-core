//go:build frost_native && frost_tbtc_signer && cgo

package tbtc

import (
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
)

func TestNativeBuildUnavailableIsOptionalForParityButFatalForSigningBinding(
	t *testing.T,
) {
	original := buildTaprootTxForSessionFn
	defer func() { buildTaprootTxForSessionFn = original }()

	unavailable := fmt.Errorf(
		"%w: injected unavailable signer",
		frostsigning.ErrNativeCryptographyUnavailable,
	)
	buildTaprootTxForSessionFn = func(
		string,
		*bitcoin.TransactionBuilder,
	) (*frostsigning.NativeTBTCSignerTxResult, error) {
		return nil, unavailable
	}

	unsignedTx := bitcoin.NewTransactionBuilder(nil)
	parityTxHex, err := buildTaprootTxViaNativeSigner(unsignedTx)
	if err != nil {
		t.Fatalf("observational parity build must preserve unavailable fallback: %v", err)
	}
	if parityTxHex != "" {
		t.Fatalf("unavailable parity build returned transaction %q", parityTxHex)
	}

	err = bindTaprootTxViaNativeSigner(
		"roast-policy-session",
		unsignedTx,
		0,
		big.NewInt(1),
	)
	if !errors.Is(err, frostsigning.ErrNativeCryptographyUnavailable) {
		t.Fatalf("mandatory policy binding must propagate unavailable signer: %v", err)
	}
}
