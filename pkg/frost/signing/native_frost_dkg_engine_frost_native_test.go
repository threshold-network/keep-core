//go:build frost_native

package signing

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/chain/local_v1"
	keepnet "github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/net/local"
	"github.com/keep-network/keep-core/pkg/operator"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

type mockNativeFROSTDKGEngine struct{}

type countingBroadcastChannel struct {
	keepnet.BroadcastChannel
	onSend func(message keepnet.TaggedMarshaler)
}

func (cbc *countingBroadcastChannel) Send(
	ctx context.Context,
	message keepnet.TaggedMarshaler,
	retransmissionStrategy ...keepnet.RetransmissionStrategy,
) error {
	if cbc.onSend != nil {
		cbc.onSend(message)
	}

	return cbc.BroadcastChannel.Send(ctx, message, retransmissionStrategy...)
}

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

func TestExecuteNativeFROSTDKG(t *testing.T) {
	const channelName = "native-frost-dkg-test"
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	engine := &deterministicNativeFROSTDKGEngine{}
	includedMembers := []group.MemberIndex{1, 2, 3}
	operatorPublicKeys, membershipValidator := nativeFROSTDKGTestMembership(
		t,
		includedMembers,
	)

	var wg sync.WaitGroup
	errChan := make(chan error, len(includedMembers))
	for _, memberIndex := range includedMembers {
		memberIndex := memberIndex
		wg.Add(1)

		go func() {
			defer wg.Done()

			provider := local.ConnectWithKey(operatorPublicKeys[memberIndex])
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
					MembershipValidator:    membershipValidator,
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

func TestExecuteNativeFROSTDKGBroadcastsBundledRoundTwoPackages(t *testing.T) {
	const channelName = "native-frost-dkg-bundled-round-two-test"
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	engine := &deterministicNativeFROSTDKGEngine{}
	includedMembers := []group.MemberIndex{1, 2, 3, 4}
	operatorPublicKeys, membershipValidator := nativeFROSTDKGTestMembership(
		t,
		includedMembers,
	)

	var roundTwoMessagesMutex sync.Mutex
	roundTwoPackageCounts := make([]int, 0, len(includedMembers))

	var wg sync.WaitGroup
	errChan := make(chan error, len(includedMembers))
	for _, memberIndex := range includedMembers {
		memberIndex := memberIndex
		wg.Add(1)

		go func() {
			defer wg.Done()

			provider := local.ConnectWithKey(operatorPublicKeys[memberIndex])
			channel, err := provider.BroadcastChannelFor(channelName)
			if err != nil {
				errChan <- err
				return
			}
			RegisterNativeFROSTDKGUnmarshallers(channel)

			channel = &countingBroadcastChannel{
				BroadcastChannel: channel,
				onSend: func(message keepnet.TaggedMarshaler) {
					roundTwoMessage, ok :=
						message.(*nativeFROSTDKGRoundTwoPackageMessage)
					if !ok {
						return
					}

					roundTwoMessagesMutex.Lock()
					defer roundTwoMessagesMutex.Unlock()

					roundTwoPackageCounts = append(
						roundTwoPackageCounts,
						len(roundTwoMessage.Packages),
					)
				},
			}

			result, err := ExecuteNativeFROSTDKG(
				ctx,
				nil,
				&NativeFROSTDKGRequest{
					MemberIndex:            memberIndex,
					GroupSize:              len(includedMembers),
					Threshold:              3,
					SessionID:              "session-bundled-round-two",
					IncludedMembersIndexes: includedMembers,
					Channel:                channel,
					MembershipValidator:    membershipValidator,
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

	if len(roundTwoPackageCounts) != len(includedMembers) {
		t.Fatalf(
			"unexpected round-two message count\nexpected: [%d]\nactual:   [%d]",
			len(includedMembers),
			len(roundTwoPackageCounts),
		)
	}
	for _, packagesCount := range roundTwoPackageCounts {
		expectedPackages := len(includedMembers) - 1
		if packagesCount != expectedPackages {
			t.Fatalf(
				"unexpected package count in bundled round-two message\nexpected: [%d]\nactual:   [%d]",
				expectedPackages,
				packagesCount,
			)
		}
	}
}

func TestNativeFROSTDKGRoundTwoPackageForReceiver(t *testing.T) {
	message := &nativeFROSTDKGRoundTwoPackageMessage{
		Packages: []*nativeFROSTDKGRoundTwoPackage{
			{
				ReceiverIDValue:              2,
				PackageParticipantIdentifier: "participant-2",
				PackageData:                  []byte{0x22},
			},
			{
				ReceiverIDValue:              3,
				PackageParticipantIdentifier: "participant-3",
				PackageData:                  []byte{0x33},
			},
		},
	}

	pkg, err := nativeFROSTDKGRoundTwoPackageForReceiver(message, 3)
	if err != nil {
		t.Fatalf("unexpected receiver package error: [%v]", err)
	}
	if pkg.PackageParticipantIdentifier != "participant-3" {
		t.Fatalf(
			"unexpected receiver package identifier\nexpected: [participant-3]\nactual:   [%s]",
			pkg.PackageParticipantIdentifier,
		)
	}

	_, err = nativeFROSTDKGRoundTwoPackageForReceiver(message, 4)
	if err == nil {
		t.Fatal("expected missing receiver package rejection")
	}
	if !strings.Contains(err.Error(), "no round-two package for receiver [4]") {
		t.Fatalf("unexpected missing receiver package error: [%v]", err)
	}

	message.Packages = append(message.Packages, &nativeFROSTDKGRoundTwoPackage{
		ReceiverIDValue:              3,
		PackageParticipantIdentifier: "participant-3-duplicate",
		PackageData:                  []byte{0x44},
	})

	_, err = nativeFROSTDKGRoundTwoPackageForReceiver(message, 3)
	if err == nil {
		t.Fatal("expected duplicate receiver package rejection")
	}
	if !strings.Contains(err.Error(), "multiple round-two packages for receiver [3]") {
		t.Fatalf("unexpected duplicate receiver package error: [%v]", err)
	}
}

func TestExecuteNativeFROSTDKGRejectsNilMembershipValidator(t *testing.T) {
	channel, err := local.Connect().BroadcastChannelFor(
		"native-frost-dkg-nil-membership-validator-test",
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = ExecuteNativeFROSTDKG(
		context.Background(),
		nil,
		&NativeFROSTDKGRequest{
			MemberIndex:            1,
			GroupSize:              2,
			Threshold:              2,
			SessionID:              "session-nil-membership-validator",
			IncludedMembersIndexes: []group.MemberIndex{1, 2},
			Channel:                channel,
		},
		&deterministicNativeFROSTDKGEngine{},
	)
	if err == nil {
		t.Fatal("expected nil membership validator rejection")
	}
	if !strings.Contains(err.Error(), "membership validator is nil") {
		t.Fatalf("unexpected error: [%v]", err)
	}
}

func TestValidateNativeFROSTDKGParticipantIdentifier(t *testing.T) {
	identifiersByMemberIndex, _, err := nativeFROSTDKGParticipantIdentifiers(
		[]group.MemberIndex{1, 2},
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := validateNativeFROSTDKGParticipantIdentifier(
		identifiersByMemberIndex,
		2,
		identifiersByMemberIndex[2],
	); err != nil {
		t.Fatalf("expected participant identifier to match: [%v]", err)
	}

	err = validateNativeFROSTDKGParticipantIdentifier(
		identifiersByMemberIndex,
		2,
		identifiersByMemberIndex[1],
	)
	if err == nil {
		t.Fatal("expected mismatched participant identifier rejection")
	}
	if !strings.Contains(err.Error(), "participant identifier mismatch") {
		t.Fatalf("unexpected mismatch error: [%v]", err)
	}
}

func nativeFROSTDKGTestMembership(
	t *testing.T,
	memberIndexes []group.MemberIndex,
) (map[group.MemberIndex]*operator.PublicKey, *group.MembershipValidator) {
	t.Helper()

	localChain := local_v1.Connect(3, 3)
	signing := localChain.Signing()
	operatorPublicKeys := make(map[group.MemberIndex]*operator.PublicKey, len(memberIndexes))
	operatorAddresses := make([]chain.Address, len(memberIndexes))

	for i, memberIndex := range memberIndexes {
		_, operatorPublicKey, err := operator.GenerateKeyPair(local.DefaultCurve)
		if err != nil {
			t.Fatal(err)
		}
		operatorPublicKeys[memberIndex] = operatorPublicKey

		operatorAddress, err := signing.PublicKeyToAddress(operatorPublicKey)
		if err != nil {
			t.Fatal(err)
		}
		operatorAddresses[i] = operatorAddress
	}

	return operatorPublicKeys, group.NewMembershipValidator(
		&testutils.MockLogger{},
		operatorAddresses,
		signing,
	)
}

func TestNativeFROSTDKGResultSignerMaterialRejectsUnsupportedFormat(t *testing.T) {
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

	_, err := dkgResult.SignerMaterial()
	if err == nil {
		t.Fatal("expected unsupported signer material error")
	}
	if !strings.Contains(err.Error(), NativeSignerMaterialFormatFrostUniFFIV2) {
		t.Fatalf("error should mention unsupported format: [%v]", err)
	}
}
