package bitcoin

import (
	"bytes"
	"fmt"

	btcec2 "github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
)

const taprootBaseLeafVersion = 0xc0

// TaprootLeafHash computes the BIP-342 TapLeaf hash for a base-version script.
func TaprootLeafHash(script Script) ([32]byte, error) {
	var buffer bytes.Buffer

	if err := buffer.WriteByte(taprootBaseLeafVersion); err != nil {
		return [32]byte{}, err
	}

	scriptLength, err := writeCompactSizeUint(CompactSizeUint(len(script)))
	if err != nil {
		return [32]byte{}, fmt.Errorf(
			"cannot encode taproot script length: [%v]",
			err,
		)
	}

	if _, err := buffer.Write(scriptLength); err != nil {
		return [32]byte{}, err
	}
	if _, err := buffer.Write(script); err != nil {
		return [32]byte{}, err
	}

	return taggedHashToArray(
		chainhash.TaggedHash(chainhash.TagTapLeaf, buffer.Bytes()),
	), nil
}

// TaprootTweak computes the BIP-341 TapTweak hash for an x-only internal key
// and optional script merkle root.
func TaprootTweak(
	internalKey [32]byte,
	merkleRoot *[32]byte,
) ([32]byte, error) {
	_, _, tweak, err := taprootTweakScalar(internalKey, merkleRoot)
	return tweak, err
}

// TaprootOutputKey derives the BIP-341 tweaked x-only Taproot output key from
// an x-only internal key and optional script merkle root.
func TaprootOutputKey(
	internalKey [32]byte,
	merkleRoot *[32]byte,
) ([32]byte, error) {
	internalPublicKey, tweakScalar, _, err := taprootTweakScalar(
		internalKey,
		merkleRoot,
	)
	if err != nil {
		return [32]byte{}, err
	}

	var internalPoint btcec2.JacobianPoint
	internalPublicKey.AsJacobian(&internalPoint)

	var tweakPoint btcec2.JacobianPoint
	btcec2.ScalarBaseMultNonConst(&tweakScalar, &tweakPoint)

	var outputPoint btcec2.JacobianPoint
	btcec2.AddNonConst(&internalPoint, &tweakPoint, &outputPoint)

	if outputPoint.Z.IsZero() {
		return [32]byte{}, fmt.Errorf("taproot output key is infinity")
	}

	outputPoint.ToAffine()
	outputPublicKey := btcec2.NewPublicKey(&outputPoint.X, &outputPoint.Y)

	var outputKey [32]byte
	copy(outputKey[:], schnorr.SerializePubKey(outputPublicKey))

	return outputKey, nil
}

// PayToTaprootWithScriptTree constructs a P2TR script from an internal key and
// a script merkle root by applying the BIP-341 TapTweak.
func PayToTaprootWithScriptTree(
	internalKey [32]byte,
	merkleRoot [32]byte,
) (Script, error) {
	outputKey, err := TaprootOutputKey(internalKey, &merkleRoot)
	if err != nil {
		return nil, fmt.Errorf("cannot derive taproot output key: [%v]", err)
	}

	return PayToTaproot(outputKey)
}

func taprootTweakScalar(
	internalKey [32]byte,
	merkleRoot *[32]byte,
) (*btcec2.PublicKey, btcec2.ModNScalar, [32]byte, error) {
	internalPublicKey, err := schnorr.ParsePubKey(internalKey[:])
	if err != nil {
		return nil, btcec2.ModNScalar{}, [32]byte{}, fmt.Errorf(
			"cannot parse taproot internal key: [%v]",
			err,
		)
	}

	tweakMessages := [][]byte{internalKey[:]}
	if merkleRoot != nil {
		tweakMessages = append(tweakMessages, merkleRoot[:])
	}

	tweak := taggedHashToArray(
		chainhash.TaggedHash(chainhash.TagTapTweak, tweakMessages...),
	)

	var tweakScalar btcec2.ModNScalar
	if overflow := tweakScalar.SetBytes(&tweak); overflow != 0 {
		return nil, btcec2.ModNScalar{}, [32]byte{}, fmt.Errorf(
			"taproot tweak is greater than or equal to curve order",
		)
	}

	return internalPublicKey, tweakScalar, tweak, nil
}

func taggedHashToArray(hash *chainhash.Hash) [32]byte {
	var result [32]byte
	copy(result[:], hash[:])

	return result
}
