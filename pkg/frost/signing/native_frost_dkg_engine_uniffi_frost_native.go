//go:build frost_native

package signing

import "fmt"

type uniFFINativeFROSTDKGBridge interface {
	Part1(
		participantIdentifier string,
		maxSigners uint16,
		minSigners uint16,
	) (*NativeFROSTDKGPart1Result, error)
	Part2(
		secretPackage *NativeFROSTDKGRound1SecretPackage,
		round1Packages []*NativeFROSTDKGRound1Package,
	) (*NativeFROSTDKGPart2Result, error)
	Part3(
		secretPackage *NativeFROSTDKGRound2SecretPackage,
		round1Packages []*NativeFROSTDKGRound1Package,
		round2Packages []*NativeFROSTDKGRound2Package,
	) (*NativeFROSTDKGResult, error)
}

type uniFFINativeFROSTDKGEngine struct {
	bridge uniFFINativeFROSTDKGBridge
}

func newUniFFINativeFROSTDKGEngine(
	bridge uniFFINativeFROSTDKGBridge,
) (NativeFROSTDKGEngine, error) {
	if bridge == nil {
		return nil, fmt.Errorf("uniffi native FROST DKG bridge is nil")
	}

	return &uniFFINativeFROSTDKGEngine{
		bridge: bridge,
	}, nil
}

func (unfdkg *uniFFINativeFROSTDKGEngine) Part1(
	participantIdentifier string,
	maxSigners uint16,
	minSigners uint16,
) (*NativeFROSTDKGPart1Result, error) {
	if participantIdentifier == "" {
		return nil, fmt.Errorf("participant identifier is empty")
	}
	if maxSigners == 0 {
		return nil, fmt.Errorf("max signers is zero")
	}
	if minSigners == 0 {
		return nil, fmt.Errorf("min signers is zero")
	}
	if minSigners > maxSigners {
		return nil, fmt.Errorf("min signers exceeds max signers")
	}

	result, err := unfdkg.bridge.Part1(
		participantIdentifier,
		maxSigners,
		minSigners,
	)
	if err != nil {
		return nil, err
	}

	if err := validateNativeFROSTDKGPart1Result(result); err != nil {
		return nil, err
	}

	return result, nil
}

func (unfdkg *uniFFINativeFROSTDKGEngine) Part2(
	secretPackage *NativeFROSTDKGRound1SecretPackage,
	round1Packages []*NativeFROSTDKGRound1Package,
) (*NativeFROSTDKGPart2Result, error) {
	if secretPackage == nil {
		return nil, fmt.Errorf("round-one secret package is nil")
	}
	if len(secretPackage.Data) == 0 {
		return nil, fmt.Errorf("round-one secret package data is empty")
	}
	if len(round1Packages) == 0 {
		return nil, fmt.Errorf("round-one packages are empty")
	}
	for i, pkg := range round1Packages {
		if pkg == nil {
			return nil, fmt.Errorf("round-one package [%d] is nil", i)
		}
		if pkg.Identifier == "" {
			return nil, fmt.Errorf("round-one package [%d] identifier is empty", i)
		}
		if len(pkg.Data) == 0 {
			return nil, fmt.Errorf("round-one package [%d] data is empty", i)
		}
	}

	result, err := unfdkg.bridge.Part2(secretPackage, round1Packages)
	if err != nil {
		return nil, err
	}

	if err := validateNativeFROSTDKGPart2Result(result); err != nil {
		return nil, err
	}

	return result, nil
}

func (unfdkg *uniFFINativeFROSTDKGEngine) Part3(
	secretPackage *NativeFROSTDKGRound2SecretPackage,
	round1Packages []*NativeFROSTDKGRound1Package,
	round2Packages []*NativeFROSTDKGRound2Package,
) (*NativeFROSTDKGResult, error) {
	if secretPackage == nil {
		return nil, fmt.Errorf("round-two secret package is nil")
	}
	if len(secretPackage.Data) == 0 {
		return nil, fmt.Errorf("round-two secret package data is empty")
	}
	if len(round1Packages) == 0 {
		return nil, fmt.Errorf("round-one packages are empty")
	}
	if len(round2Packages) == 0 {
		return nil, fmt.Errorf("round-two packages are empty")
	}
	for i, pkg := range round2Packages {
		if pkg == nil {
			return nil, fmt.Errorf("round-two package [%d] is nil", i)
		}
		if pkg.Identifier == "" {
			return nil, fmt.Errorf("round-two package [%d] identifier is empty", i)
		}
		if pkg.SenderIdentifier == "" {
			return nil, fmt.Errorf("round-two package [%d] sender identifier is empty", i)
		}
		if len(pkg.Data) == 0 {
			return nil, fmt.Errorf("round-two package [%d] data is empty", i)
		}
	}

	result, err := unfdkg.bridge.Part3(
		secretPackage,
		round1Packages,
		round2Packages,
	)
	if err != nil {
		return nil, err
	}

	if err := validateNativeFROSTDKGResult(result); err != nil {
		return nil, err
	}

	return result, nil
}
