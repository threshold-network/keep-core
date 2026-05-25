//go:build frost_native

package signing

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/net/local"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

type mockNativeFROSTDKGEngine struct{}

func (mnfdkg *mockNativeFROSTDKGEngine) Part1(
	participantIdentifier string,
	maxSigners uint16,
	minSigners uint16,
) (*NativeFROSTDKGPart1Result, error) {
	return nil, nil
}

type deterministicNativeFROSTDKGEngine struct{}

func (dnfdkg *deterministicNativeFROSTDKGEngine) Part1(
	participantIdentifier string,
	maxSigners uint16,
	minSigners uint16,
) (*NativeFROSTDKGPart1Result, error) {
	return &NativeFROSTDKGPart1Result{
		SecretPackage: &NativeFROSTDKGRound1SecretPackage{
			Data: []byte("round1-secret-" + participantIdentifier),
		},
		Package: &NativeFROSTDKGRound1Package{
			Identifier: participantIdentifier,
			Data: []byte(fmt.Sprintf(
				"round1-package-%s-%d-%d",
				participantIdentifier,
				maxSigners,
				minSigners,
			)),
		},
	}, nil
}

func (dnfdkg *deterministicNativeFROSTDKGEngine) Part2(
	secretPackage *NativeFROSTDKGRound1SecretPackage,
	round1Packages []*NativeFROSTDKGRound1Package,
) (*NativeFROSTDKGPart2Result, error) {
	packages := make([]*NativeFROSTDKGRound2Package, 0, len(round1Packages))
	for _, round1Package := range round1Packages {
		packages = append(packages, &NativeFROSTDKGRound2Package{
			Identifier: round1Package.Identifier,
			Data: append(
				[]byte("round2-package-for-"+round1Package.Identifier+":"),
				secretPackage.Data...,
			),
		})
	}

	return &NativeFROSTDKGPart2Result{
		SecretPackage: &NativeFROSTDKGRound2SecretPackage{
			Data: []byte("round2-secret"),
		},
		Packages: packages,
	}, nil
}

func (dnfdkg *deterministicNativeFROSTDKGEngine) Part3(
	secretPackage *NativeFROSTDKGRound2SecretPackage,
	round1Packages []*NativeFROSTDKGRound1Package,
	round2Packages []*NativeFROSTDKGRound2Package,
) (*NativeFROSTDKGResult, error) {
	if len(round1Packages) != len(round2Packages) {
		return nil, fmt.Errorf("unexpected package counts")
	}

	return &NativeFROSTDKGResult{
		KeyPackage: &NativeFROSTKeyPackage{
			Identifier: round2Packages[0].Identifier,
			Data:       append([]byte{}, secretPackage.Data...),
		},
		PublicKeyPackage: &NativeFROSTPublicKeyPackage{
			VerifyingShares: map[string]string{
				round1Packages[0].Identifier: "share",
			},
			VerifyingKey: "1111111111111111111111111111111111111111111111111111111111111111",
		},
	}, nil
}

func (mnfdkg *mockNativeFROSTDKGEngine) Part2(
	secretPackage *NativeFROSTDKGRound1SecretPackage,
	round1Packages []*NativeFROSTDKGRound1Package,
) (*NativeFROSTDKGPart2Result, error) {
	return nil, nil
}

func (mnfdkg *mockNativeFROSTDKGEngine) Part3(
	secretPackage *NativeFROSTDKGRound2SecretPackage,
	round1Packages []*NativeFROSTDKGRound1Package,
	round2Packages []*NativeFROSTDKGRound2Package,
) (*NativeFROSTDKGResult, error) {
	return nil, nil
}

type mockUniFFINativeFROSTDKGBridge struct {
	part1Called bool
	part2Called bool
	part3Called bool
}

func (munfdkgb *mockUniFFINativeFROSTDKGBridge) Part1(
	participantIdentifier string,
	maxSigners uint16,
	minSigners uint16,
) (*NativeFROSTDKGPart1Result, error) {
	munfdkgb.part1Called = true

	return &NativeFROSTDKGPart1Result{
		SecretPackage: &NativeFROSTDKGRound1SecretPackage{Data: []byte{0x01}},
		Package: &NativeFROSTDKGRound1Package{
			Identifier: participantIdentifier,
			Data:       []byte{byte(maxSigners), byte(minSigners)},
		},
	}, nil
}

func (munfdkgb *mockUniFFINativeFROSTDKGBridge) Part2(
	secretPackage *NativeFROSTDKGRound1SecretPackage,
	round1Packages []*NativeFROSTDKGRound1Package,
) (*NativeFROSTDKGPart2Result, error) {
	munfdkgb.part2Called = true

	return &NativeFROSTDKGPart2Result{
		SecretPackage: &NativeFROSTDKGRound2SecretPackage{Data: []byte{0x02}},
		Packages: []*NativeFROSTDKGRound2Package{
			{
				Identifier: round1Packages[0].Identifier,
				Data:       append([]byte{}, secretPackage.Data...),
			},
		},
	}, nil
}

func (munfdkgb *mockUniFFINativeFROSTDKGBridge) Part3(
	secretPackage *NativeFROSTDKGRound2SecretPackage,
	round1Packages []*NativeFROSTDKGRound1Package,
	round2Packages []*NativeFROSTDKGRound2Package,
) (*NativeFROSTDKGResult, error) {
	munfdkgb.part3Called = true

	return &NativeFROSTDKGResult{
		KeyPackage: &NativeFROSTKeyPackage{
			Identifier: round2Packages[0].SenderIdentifier,
			Data:       append([]byte{}, secretPackage.Data...),
		},
		PublicKeyPackage: &NativeFROSTPublicKeyPackage{
			VerifyingShares: map[string]string{
				round1Packages[0].Identifier: "share",
			},
			VerifyingKey: "1111111111111111111111111111111111111111111111111111111111111111",
		},
	}, nil
}

func TestRegisterNativeFROSTDKGEngineRejectsNil(t *testing.T) {
	UnregisterNativeFROSTDKGEngine()
	t.Cleanup(UnregisterNativeFROSTDKGEngine)

	err := RegisterNativeFROSTDKGEngine(nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRegisterNativeFROSTDKGEngine(t *testing.T) {
	UnregisterNativeFROSTDKGEngine()
	t.Cleanup(UnregisterNativeFROSTDKGEngine)

	engine := &mockNativeFROSTDKGEngine{}
	if err := RegisterNativeFROSTDKGEngine(engine); err != nil {
		t.Fatalf("unexpected register error: [%v]", err)
	}

	if currentNativeFROSTDKGEngine() != engine {
		t.Fatal("unexpected current native FROST DKG engine")
	}
}

func TestNewUniFFINativeFROSTDKGEngine_NilBridge(t *testing.T) {
	_, err := newUniFFINativeFROSTDKGEngine(nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUniFFINativeFROSTDKGEngine(t *testing.T) {
	bridge := &mockUniFFINativeFROSTDKGBridge{}
	engine, err := newUniFFINativeFROSTDKGEngine(bridge)
	if err != nil {
		t.Fatalf("unexpected constructor error: [%v]", err)
	}

	part1, err := engine.Part1("participant-1", 3, 2)
	if err != nil {
		t.Fatalf("unexpected part1 error: [%v]", err)
	}

	part2, err := engine.Part2(
		part1.SecretPackage,
		[]*NativeFROSTDKGRound1Package{
			{Identifier: "participant-2", Data: []byte{0x22}},
		},
	)
	if err != nil {
		t.Fatalf("unexpected part2 error: [%v]", err)
	}

	_, err = engine.Part3(
		part2.SecretPackage,
		[]*NativeFROSTDKGRound1Package{
			{Identifier: "participant-2", Data: []byte{0x22}},
		},
		[]*NativeFROSTDKGRound2Package{
			{
				Identifier:       "participant-1",
				SenderIdentifier: "participant-2",
				Data:             []byte{0x33},
			},
		},
	)
	if err != nil {
		t.Fatalf("unexpected part3 error: [%v]", err)
	}

	if !bridge.part1Called || !bridge.part2Called || !bridge.part3Called {
		t.Fatal("expected all bridge parts to be called")
	}
}

func TestExecuteNativeFROSTDKG(t *testing.T) {
	const channelName = "native-frost-dkg-test"
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	engine := &deterministicNativeFROSTDKGEngine{}
	includedMembers := []group.MemberIndex{1, 2, 3}

	var wg sync.WaitGroup
	errChan := make(chan error, len(includedMembers))
	for _, memberIndex := range includedMembers {
		memberIndex := memberIndex
		wg.Add(1)

		go func() {
			defer wg.Done()

			provider := local.Connect()
			channel, err := provider.BroadcastChannelFor(channelName)
			if err != nil {
				errChan <- err
				return
			}
			RegisterNativeFROSTDKGUnmarshallers(channel)

			result, err := ExecuteNativeFROSTDKG(
				ctx,
				nil,
				&NativeFROSTDKGRequest{
					MemberIndex:            memberIndex,
					GroupSize:              3,
					Threshold:              2,
					SessionID:              "session-1",
					IncludedMembersIndexes: includedMembers,
					Channel:                channel,
				},
				engine,
			)
			if err != nil {
				errChan <- err
				return
			}
			if result == nil {
				errChan <- fmt.Errorf("nil DKG result")
			}
		}()
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestNativeFROSTDKGResultSignerMaterial(t *testing.T) {
	dkgResult := &NativeFROSTDKGResult{
		KeyPackage: &NativeFROSTKeyPackage{
			Identifier: "0000000000000000000000000000000000000000000000000000000000000001",
			Data:       []byte{0x01, 0x02, 0x03},
		},
		PublicKeyPackage: &NativeFROSTPublicKeyPackage{
			VerifyingShares: map[string]string{
				"0000000000000000000000000000000000000000000000000000000000000001": "share",
			},
			VerifyingKey: "1111111111111111111111111111111111111111111111111111111111111111",
		},
	}

	signerMaterial, err := dkgResult.SignerMaterial()
	if err != nil {
		t.Fatalf("unexpected signer material error: [%v]", err)
	}

	if signerMaterial.Format != NativeSignerMaterialFormatFrostUniFFIV2 {
		t.Fatalf(
			"unexpected signer material format\nexpected: [%s]\nactual:   [%s]",
			NativeSignerMaterialFormatFrostUniFFIV2,
			signerMaterial.Format,
		)
	}

	extracted, err := ExtractDkgGroupPublicKeyFromMaterial(signerMaterial)
	if err != nil {
		t.Fatalf("unexpected DKG public-key extraction error: [%v]", err)
	}

	expected := "1111111111111111111111111111111111111111111111111111111111111111"
	if actual := stringHex(extracted); actual != expected {
		t.Fatalf(
			"unexpected extracted DKG output key\nexpected: [%s]\nactual:   [%s]",
			expected,
			actual,
		)
	}
}

func stringHex(data []byte) string {
	const hexChars = "0123456789abcdef"
	result := make([]byte, len(data)*2)
	for i, b := range data {
		result[i*2] = hexChars[b>>4]
		result[i*2+1] = hexChars[b&0x0f]
	}
	return string(result)
}
