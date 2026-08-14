package tbtc

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"strings"
	"testing"

	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
)

type frostNativeSignerAnchorTrustStartupFixture struct {
	runtime     FrostPreSignActivationRuntimeManifest
	installed   frostsigning.NativeTBTCSignerInstalledStateAnchorConfig
	certificate FrostNativeSignerAnchorTrustCertificate
}

func TestValidateFrostNativeSignerAnchorTrustExpectedHead(t *testing.T) {
	fixture := newFrostNativeSignerAnchorTrustStartupFixture()

	protocolHead, nativeHead, err :=
		validateFrostNativeSignerAnchorTrustExpectedHead(
			fixture.runtime,
			&fixture.installed,
			&fixture.certificate,
		)
	if err != nil {
		t.Fatal(err)
	}
	if protocolHead.CertificateSequence !=
		fixture.certificate.CertificateSequence ||
		protocolHead.CertificateDigest != fixture.certificate.CertificateDigest ||
		protocolHead.ProtocolID != fixture.certificate.ProtocolID ||
		protocolHead.StreamID != fixture.certificate.StreamID ||
		protocolHead.SignerStoreFingerprint !=
			fixture.certificate.SignerStoreFingerprint ||
		protocolHead.Endpoint != fixture.certificate.To {
		t.Fatal("unexpected protocol trust head")
	}
	expectedNativeHead := frostNativeSignerAnchorNativeTrustHead(protocolHead)
	if *nativeHead != expectedNativeHead {
		t.Fatal("unexpected native trust head")
	}

	reference := frostNativeSignerAnchorReferenceFromTrust(
		fixture.certificate.To.Reference,
	)
	if reference.ServiceEpoch != fixture.certificate.To.Reference.ServiceEpoch ||
		reference.Revision != fixture.certificate.To.Reference.Revision ||
		reference.EventRoot != fixture.certificate.To.Reference.EventRoot ||
		reference.AcknowledgementDigest !=
			fixture.certificate.To.Reference.AcknowledgementDigest ||
		reference.Checkpoint != fixture.certificate.To.Reference.Checkpoint {
		t.Fatal("unexpected ordinary anchor reference")
	}
}

func TestValidateFrostNativeSignerAnchorTrustExpectedHeadRejectsPinMismatch(
	t *testing.T,
) {
	tests := map[string]func(*frostNativeSignerAnchorTrustStartupFixture){
		"protocol": func(fixture *frostNativeSignerAnchorTrustStartupFixture) {
			fixture.installed.ProtocolID[0] ^= 1
		},
		"stream": func(fixture *frostNativeSignerAnchorTrustStartupFixture) {
			fixture.installed.StreamID[0] ^= 1
		},
		"signer store": func(
			fixture *frostNativeSignerAnchorTrustStartupFixture,
		) {
			fixture.certificate.SignerStoreFingerprint[0] ^= 1
		},
		"runtime manifest hash": func(
			fixture *frostNativeSignerAnchorTrustStartupFixture,
		) {
			fixture.runtime.ManifestHash[0] ^= 1
		},
		"manifest sequence": func(
			fixture *frostNativeSignerAnchorTrustStartupFixture,
		) {
			fixture.installed.ActivationManifestSequence++
		},
		"binding": func(fixture *frostNativeSignerAnchorTrustStartupFixture) {
			fixture.installed.BindingHash[0] ^= 1
		},
		"online raw key": func(
			fixture *frostNativeSignerAnchorTrustStartupFixture,
		) {
			fixture.certificate.To.ResponsePublicKey[0] ^= 1
		},
		"online SPKI": func(
			fixture *frostNativeSignerAnchorTrustStartupFixture,
		) {
			fixture.installed.ResponsePublicKeySPKISHA256[0] ^= 1
		},
		"offline raw key": func(
			fixture *frostNativeSignerAnchorTrustStartupFixture,
		) {
			fixture.runtime.ActivationAuthorityPublicKey[0] ^= 1
		},
		"offline SPKI": func(
			fixture *frostNativeSignerAnchorTrustStartupFixture,
		) {
			fixture.runtime.NativeSignerAnchor.Identity.
				OfflineAuthorityHash[0] ^= 1
		},
		"certificate sequence": func(
			fixture *frostNativeSignerAnchorTrustStartupFixture,
		) {
			fixture.certificate.CertificateSequence++
		},
		"certificate digest": func(
			fixture *frostNativeSignerAnchorTrustStartupFixture,
		) {
			fixture.certificate.CertificateDigest[0] ^= 1
		},
		"maximum records": func(
			fixture *frostNativeSignerAnchorTrustStartupFixture,
		) {
			fixture.certificate.To.WitnessMaximumRecords++
		},
		"rotation threshold": func(
			fixture *frostNativeSignerAnchorTrustStartupFixture,
		) {
			fixture.installed.WitnessRotationThresholdRecords--
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			fixture := newFrostNativeSignerAnchorTrustStartupFixture()
			mutate(&fixture)
			if _, _, err :=
				validateFrostNativeSignerAnchorTrustExpectedHead(
					fixture.runtime,
					&fixture.installed,
					&fixture.certificate,
				); err == nil {
				t.Fatal("expected pin mismatch")
			}
		})
	}

}

func TestValidateFrostNativeSignerAnchorTrustExpectedHeadRejectsRoleAliasing(
	t *testing.T,
) {
	fixture := newFrostNativeSignerAnchorTrustStartupFixture()
	authority := fixture.installed.OfflineAuthorityPublicKey
	authorityHash := fixture.installed.OfflineAuthoritySPKISHA256
	fixture.installed.ResponsePublicKey = authority
	fixture.installed.ResponsePublicKeySPKISHA256 = authorityHash
	fixture.runtime.NativeSignerAnchor.Identity.OnlineKeyHash = authorityHash
	fixture.installed.BindingHash = ComputeFrostNativeSignerAnchorBindingHash(
		fixture.runtime.NativeSignerAnchor.Identity,
	)
	fixture.certificate.To.ResponsePublicKey = authority
	fixture.certificate.To.ResponsePublicKeySPKISHA256 = authorityHash
	fixture.certificate.To.BindingHash = fixture.installed.BindingHash

	_, _, err := validateFrostNativeSignerAnchorTrustExpectedHead(
		fixture.runtime,
		&fixture.installed,
		&fixture.certificate,
	)
	if err == nil || !strings.Contains(err.Error(), "aliases") {
		t.Fatalf("expected online/offline role-alias rejection, got [%v]", err)
	}
}

func TestReconstructFrostNativeSignerAnchorTrustPriorHead(t *testing.T) {
	fixture, expectedPrior, readback :=
		newFrostNativeSignerAnchorTrustPriorStartupFixture()

	actual, err := reconstructFrostNativeSignerAnchorTrustPriorHead(
		&readback,
		fixture.runtime,
		&fixture.installed,
		&fixture.certificate,
	)
	if err != nil {
		t.Fatal(err)
	}
	if *actual != expectedPrior {
		t.Fatal("reconstructed prior head differs from authenticated readback")
	}
}

// The honest pending-rotation path must keep working: the durable head sits at
// or below the configured pin, which is exactly what a node about to install a
// new certificate looks like.
func TestReconstructFrostNativeSignerAnchorTrustPriorHeadAcceptsConfiguredPin(
	t *testing.T,
) {
	fixture, _, readback := newFrostNativeSignerAnchorTrustPriorStartupFixture()

	if readback.CertificateSequence > fixture.installed.TrustCertificateSequence {
		t.Fatalf(
			"fixture is not representative: readback sequence [%v] already "+
				"exceeds the installed pin [%v]",
			readback.CertificateSequence,
			fixture.installed.TrustCertificateSequence,
		)
	}
	if _, err := reconstructFrostNativeSignerAnchorTrustPriorHead(
		&readback,
		fixture.runtime,
		&fixture.installed,
		&fixture.certificate,
	); err != nil {
		t.Fatal(err)
	}
}

// Configuration rolled back while the signer store was not. This is a
// diagnosability assertion over a state that already failed closed; it is not a
// bound on operator rollback, which reverts both and leaves the two equal.
func TestReconstructFrostNativeSignerAnchorTrustPriorHeadRejectsAheadDurableHead(
	t *testing.T,
) {
	fixture, _, readback := newFrostNativeSignerAnchorTrustPriorStartupFixture()
	readback.CertificateSequence = fixture.installed.TrustCertificateSequence + 1

	_, err := reconstructFrostNativeSignerAnchorTrustPriorHead(
		&readback,
		fixture.runtime,
		&fixture.installed,
		&fixture.certificate,
	)
	if err == nil {
		t.Fatal("expected a durable trust head ahead of the pin to be rejected")
	}
	if !strings.Contains(err.Error(), "ahead of the installed certificate pin") {
		t.Fatalf("unexpected error: [%v]", err)
	}
}

func TestReconstructFrostNativeSignerAnchorTrustPriorHeadUsesCertifiedFloor(
	t *testing.T,
) {
	fixture, expectedPrior, readback :=
		newFrostNativeSignerAnchorTrustPriorStartupFixture()
	from := *fixture.certificate.From
	from.Reference.Revision = 7
	from.Reference.PreviousEventRoot =
		frostNativeSignerAnchorTrustStartupBytes32(0x76)
	from.Reference.EventRoot =
		frostNativeSignerAnchorTrustStartupBytes32(0x77)
	from.Reference.AcknowledgementDigest =
		frostNativeSignerAnchorTrustStartupBytes32(0x78)
	from.Reference.Checkpoint =
		FrostNativeSignerStateWitnessCheckpoint{
			StoreFingerprint: expectedPrior.Endpoint.Reference.Checkpoint.
				StoreFingerprint,
			Generation: expectedPrior.Endpoint.Reference.Checkpoint.
				Generation + 2,
			PreviousStateCommitment: frostNativeSignerAnchorTrustStartupBytes32(
				0x79,
			),
			StateImageDigest: frostNativeSignerAnchorTrustStartupBytes32(
				0x7a,
			),
		}
	from.Reference.Checkpoint.StateCommitment =
		frostsigning.ComputeNativeTBTCSignerStateWitnessCommitment(
			from.Reference.Checkpoint.StoreFingerprint,
			from.Reference.Checkpoint.Generation,
			from.Reference.Checkpoint.PreviousStateCommitment,
			from.Reference.Checkpoint.StateImageDigest,
		)
	fixture.certificate.From = &from

	actual, err := reconstructFrostNativeSignerAnchorTrustPriorHead(
		&readback,
		fixture.runtime,
		&fixture.installed,
		&fixture.certificate,
	)
	if err != nil {
		t.Fatal(err)
	}
	if *actual != expectedPrior {
		t.Fatal("prior reconstruction replaced the authenticated floor with certificate From")
	}
}

func TestReconstructFrostNativeSignerAnchorTrustPriorHeadRejectsUntrustedFrom(
	t *testing.T,
) {
	tests := map[string]func(
		*frostNativeSignerAnchorTrustStartupFixture,
		*frostsigning.NativeTBTCSignerStateAnchorTrustHead,
	){
		"missing from": func(
			fixture *frostNativeSignerAnchorTrustStartupFixture,
			_ *frostsigning.NativeTBTCSignerStateAnchorTrustHead,
		) {
			fixture.certificate.From = nil
		},
		"raw response key hash": func(
			fixture *frostNativeSignerAnchorTrustStartupFixture,
			_ *frostsigning.NativeTBTCSignerStateAnchorTrustHead,
		) {
			from := *fixture.certificate.From
			from.ResponsePublicKey[0] ^= 1
			fixture.certificate.From = &from
		},
		"from manifest hash": func(
			fixture *frostNativeSignerAnchorTrustStartupFixture,
			_ *frostsigning.NativeTBTCSignerStateAnchorTrustHead,
		) {
			from := *fixture.certificate.From
			from.ActivationManifestHash[0] ^= 1
			fixture.certificate.From = &from
		},
		"from binding": func(
			fixture *frostNativeSignerAnchorTrustStartupFixture,
			_ *frostsigning.NativeTBTCSignerStateAnchorTrustHead,
		) {
			from := *fixture.certificate.From
			from.BindingHash[0] ^= 1
			fixture.certificate.From = &from
		},
		"readback authority": func(
			_ *frostNativeSignerAnchorTrustStartupFixture,
			readback *frostsigning.NativeTBTCSignerStateAnchorTrustHead,
		) {
			readback.OfflineAuthoritySPKISHA256[0] ^= 1
		},
		"readback geometry": func(
			_ *frostNativeSignerAnchorTrustStartupFixture,
			readback *frostsigning.NativeTBTCSignerStateAnchorTrustHead,
		) {
			readback.WitnessMaximumRecords++
		},
		"readback store": func(
			_ *frostNativeSignerAnchorTrustStartupFixture,
			readback *frostsigning.NativeTBTCSignerStateAnchorTrustHead,
		) {
			readback.CertifiedFloor.Checkpoint.StoreFingerprint[0] ^= 1
		},
		"readback certified revision": func(
			_ *frostNativeSignerAnchorTrustStartupFixture,
			readback *frostsigning.NativeTBTCSignerStateAnchorTrustHead,
		) {
			readback.CertifiedFloor.Revision = 2
		},
		"readback schema": func(
			_ *frostNativeSignerAnchorTrustStartupFixture,
			readback *frostsigning.NativeTBTCSignerStateAnchorTrustHead,
		) {
			readback.Schema += "-unknown"
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			fixture, _, readback :=
				newFrostNativeSignerAnchorTrustPriorStartupFixture()
			mutate(&fixture, &readback)
			if _, err := reconstructFrostNativeSignerAnchorTrustPriorHead(
				&readback,
				fixture.runtime,
				&fixture.installed,
				&fixture.certificate,
			); err == nil {
				t.Fatal("expected prior-head reconstruction rejection")
			}
		})
	}

}

func TestReconstructFrostNativeSignerAnchorTrustPriorHeadRejectsRoleAliasing(
	t *testing.T,
) {
	fixture, _, readback :=
		newFrostNativeSignerAnchorTrustPriorStartupFixture()
	from := *fixture.certificate.From
	from.ResponsePublicKey = fixture.installed.OfflineAuthorityPublicKey
	from.ResponsePublicKeySPKISHA256 =
		fixture.installed.OfflineAuthoritySPKISHA256
	fixture.certificate.From = &from
	readback.ResponsePublicKeySPKISHA256 =
		fixture.installed.OfflineAuthoritySPKISHA256

	_, err := reconstructFrostNativeSignerAnchorTrustPriorHead(
		&readback,
		fixture.runtime,
		&fixture.installed,
		&fixture.certificate,
	)
	if err == nil || !strings.Contains(err.Error(), "aliases") {
		t.Fatalf("expected prior online/offline role-alias rejection, got [%v]", err)
	}
}

func TestReconstructFrostNativeSignerAnchorTrustPriorHeadExactBootstrapReplay(
	t *testing.T,
) {
	fixture := newFrostNativeSignerAnchorTrustStartupFixture()
	fixture.certificate.Kind =
		FrostNativeSignerAnchorTrustCertificateBootstrap
	fixture.certificate.CertificateSequence = 1
	fixture.installed.TrustCertificateSequence = 1
	fixture.certificate.From = nil

	expected, readback, err :=
		validateFrostNativeSignerAnchorTrustExpectedHead(
			fixture.runtime,
			&fixture.installed,
			&fixture.certificate,
		)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := reconstructFrostNativeSignerAnchorTrustPriorHead(
		readback,
		fixture.runtime,
		&fixture.installed,
		&fixture.certificate,
	)
	if err != nil {
		t.Fatal(err)
	}
	if *actual != *expected {
		t.Fatal("exact bootstrap replay did not reconstruct the final head")
	}

	fixture.certificate.To.BindingHash[0] ^= 1
	if _, err := reconstructFrostNativeSignerAnchorTrustPriorHead(
		readback,
		fixture.runtime,
		&fixture.installed,
		&fixture.certificate,
	); err == nil {
		t.Fatal("expected tampered exact-replay endpoint rejection")
	}
}

func TestSelectFrostNativeSignerAnchorTrustTransitionChainResumesMissingSuffix(
	t *testing.T,
) {
	bootstrap, authority := trustTestBootstrapCertificate(t)
	second := trustTestRotationCertificate(t, bootstrap, authority, 0x81)
	final := trustTestRotationCertificate(t, second, authority, 0x82)
	configured := []FrostNativeSignerAnchorTrustCertificate{
		*bootstrap,
		*second,
		*final,
	}

	priorHead := trustTestCertificateHead(second)
	priorReadback := frostNativeSignerAnchorNativeTrustHead(priorHead)
	suffix, err := selectFrostNativeSignerAnchorTrustTransitionChain(
		configured,
		&priorReadback,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(suffix) != 1 ||
		suffix[0].CertificateDigest != final.CertificateDigest {
		t.Fatal("partially installed chain did not resume at its missing suffix")
	}
	options := trustTestChainOptions(final)
	options.PriorHead = priorHead
	if err := ValidateFrostNativeSignerAnchorTrustCertificateChain(
		suffix,
		options,
	); err != nil {
		t.Fatalf("selected partial-resume suffix is invalid: %v", err)
	}

	// A configured artifact may itself already be a suffix whose first item
	// immediately follows the authenticated head.
	suffix, err = selectFrostNativeSignerAnchorTrustTransitionChain(
		[]FrostNativeSignerAnchorTrustCertificate{*final},
		&priorReadback,
	)
	if err != nil || len(suffix) != 1 ||
		suffix[0].CertificateDigest != final.CertificateDigest {
		t.Fatalf("pre-sliced missing suffix was not retained: %v", err)
	}
}

func TestSelectFrostNativeSignerAnchorTrustTransitionChainCompletedRestart(
	t *testing.T,
) {
	bootstrap, authority := trustTestBootstrapCertificate(t)
	second := trustTestRotationCertificate(t, bootstrap, authority, 0x83)
	final := trustTestRotationCertificate(t, second, authority, 0x84)
	configured := []FrostNativeSignerAnchorTrustCertificate{
		*bootstrap,
		*second,
		*final,
	}
	finalHead := trustTestCertificateHead(final)
	finalReadback := frostNativeSignerAnchorNativeTrustHead(finalHead)
	replay, err := selectFrostNativeSignerAnchorTrustTransitionChain(
		configured,
		&finalReadback,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(replay) != 1 ||
		replay[0].CertificateDigest != final.CertificateDigest {
		t.Fatal("completed restart did not select the exact one-item replay")
	}
	options := trustTestChainOptions(final)
	options.PriorHead = finalHead
	if err := ValidateFrostNativeSignerAnchorTrustCertificateChain(
		replay,
		options,
	); err != nil {
		t.Fatalf("selected completed-restart replay is invalid: %v", err)
	}
	if !isFrostNativeSignerAnchorTrustExactHeadReplay(
		finalHead,
		finalHead,
	) {
		t.Fatal("authenticated completed head did not select transition bypass")
	}
	different := *finalHead
	different.CertificateDigest[0] ^= 1
	if isFrostNativeSignerAnchorTrustExactHeadReplay(
		&different,
		finalHead,
	) || isFrostNativeSignerAnchorTrustExactHeadReplay(nil, finalHead) {
		t.Fatal("missing suffix selected completed-head transition bypass")
	}
}

func TestSelectFrostNativeSignerAnchorTrustTransitionChainRejectsAmbiguity(
	t *testing.T,
) {
	bootstrap, _ := trustTestBootstrapCertificate(t)
	readback := frostNativeSignerAnchorNativeTrustHead(
		trustTestCertificateHead(bootstrap),
	)
	if _, err := selectFrostNativeSignerAnchorTrustTransitionChain(
		[]FrostNativeSignerAnchorTrustCertificate{
			*bootstrap,
			*bootstrap,
		},
		&readback,
	); err == nil {
		t.Fatal("ambiguous authenticated head in configured chain was accepted")
	}
	if _, err := selectFrostNativeSignerAnchorTrustTransitionChain(
		nil,
		nil,
	); err == nil {
		t.Fatal("empty configured trust chain was accepted")
	}
}

func TestSelectFrostNativeSignerAnchorTrustRecoveryChainRequiresExactFinalSuffix(
	t *testing.T,
) {
	bootstrap, authority := trustTestBootstrapCertificate(t)
	second := trustTestRotationCertificate(t, bootstrap, authority, 0x91)
	final := trustTestRotationCertificate(t, second, authority, 0x92)
	configured := []FrostNativeSignerAnchorTrustCertificate{
		*bootstrap,
		*second,
		*final,
	}
	artifact, err :=
		authenticateFrostNativeSignerAnchorTrustRecoveryArtifact(
			configured,
			trustTestChainOptions(final),
		)
	if err != nil {
		t.Fatal(err)
	}
	recovery := testFrostNativeSignerAnchorTrustRecoverySelector(
		configured,
		1,
	)
	selected, err := selectFrostNativeSignerAnchorTrustRecoveryChain(
		artifact,
		&recovery,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 2 ||
		selected[0].CertificateDigest != second.CertificateDigest ||
		selected[1].CertificateDigest != final.CertificateDigest {
		t.Fatal("recovery selector did not select the exact final suffix")
	}

	for name, mutate := range map[string]func(
		*frostsigning.NativeTBTCSignerStateAnchorTrustRecoveryRequired,
	){
		"store": func(
			value *frostsigning.NativeTBTCSignerStateAnchorTrustRecoveryRequired,
		) {
			value.StoreFingerprint[0] ^= 1
		},
		"ordered digest": func(
			value *frostsigning.NativeTBTCSignerStateAnchorTrustRecoveryRequired,
		) {
			value.OrderedCertificateDigests[0][0] ^= 1
		},
		"final digest": func(
			value *frostsigning.NativeTBTCSignerStateAnchorTrustRecoveryRequired,
		) {
			value.FinalCertificateDigest[0] ^= 1
		},
		"target binding": func(
			value *frostsigning.NativeTBTCSignerStateAnchorTrustRecoveryRequired,
		) {
			value.TargetBindingHash[0] ^= 1
		},
		"target checkpoint": func(
			value *frostsigning.NativeTBTCSignerStateAnchorTrustRecoveryRequired,
		) {
			value.TargetCheckpoint.Generation++
		},
		"stale prior final": func(
			value *frostsigning.NativeTBTCSignerStateAnchorTrustRecoveryRequired,
		) {
			value.CertificateCount = 1
			value.FirstCertificateSequence = second.CertificateSequence
			value.OrderedCertificateDigests = [][32]byte{
				second.CertificateDigest,
			}
			value.FinalCertificateSequence = second.CertificateSequence
			value.FinalCertificateDigest = second.CertificateDigest
			value.TargetBindingHash = second.To.BindingHash
			value.TargetServiceEpoch = second.To.Reference.ServiceEpoch
			value.TargetRevision = second.To.Reference.Revision
			value.TargetCheckpoint =
				frostNativeSignerAnchorNativeTrustCheckpoint(
					second.To.Reference.Checkpoint,
				)
		},
	} {
		t.Run(name, func(t *testing.T) {
			candidate := testFrostNativeSignerAnchorTrustRecoverySelector(
				configured,
				1,
			)
			mutate(&candidate)
			if _, err :=
				selectFrostNativeSignerAnchorTrustRecoveryChain(
					artifact,
					&candidate,
				); err == nil {
				t.Fatal("tampered or stale recovery selector was accepted")
			}
		})
	}
}

func TestExecuteFrostNativeSignerAnchorTrustTransitionRecoversMultiCertificateIntent(
	t *testing.T,
) {
	bootstrap, authority := trustTestBootstrapCertificate(t)
	second := trustTestRotationCertificate(t, bootstrap, authority, 0x93)
	final := trustTestRotationCertificate(t, second, authority, 0x94)
	configured := []FrostNativeSignerAnchorTrustCertificate{
		*bootstrap,
		*second,
		*final,
	}
	artifact, err :=
		authenticateFrostNativeSignerAnchorTrustRecoveryArtifact(
			configured,
			trustTestChainOptions(final),
		)
	if err != nil {
		t.Fatal(err)
	}
	recovery := testFrostNativeSignerAnchorTrustRecoverySelector(
		configured,
		0,
	)
	fresh := []byte(`{"fresh":"multi-certificate-recovery"}`)
	readCalls := 0
	invokeCalls := 0
	expectedResult :=
		&frostsigning.NativeTBTCSignerStateAnchorTrustTransitionResult{
			Installed: true,
		}
	result, target, applied, recoveryReplay, err :=
		executeFrostNativeSignerAnchorTrustTransition(
			context.Background(),
			artifact,
			nil,
			&recovery,
			func(
				context.Context,
				bool,
			) (*FrostNativeSignerAnchorTrustTransitionTarget, error) {
				readCalls++
				return &FrostNativeSignerAnchorTrustTransitionTarget{
					Reference:         final.To.Reference,
					ExactReadResponse: fresh,
				}, nil
			},
			func(
				request []byte,
			) (*frostsigning.NativeTBTCSignerStateAnchorTrustTransitionResult, error) {
				invokeCalls++
				decoded, err :=
					DecodeFrostNativeSignerAnchorTrustTransitionRequest(
						request,
					)
				if err != nil {
					t.Fatal(err)
				}
				if len(decoded.CertificateChain) != 3 ||
					!bytes.Equal(decoded.TargetReadResponse, fresh) {
					t.Fatal("recovery did not replay the exact intent chain with a fresh Read")
				}
				return expectedResult, nil
			},
		)
	if err != nil {
		t.Fatal(err)
	}
	if result != expectedResult || target == nil ||
		!bytes.Equal(target.ExactReadResponse, fresh) ||
		len(applied) != 3 || !recoveryReplay ||
		readCalls != 1 || invokeCalls != 1 {
		t.Fatalf(
			"unexpected multi-certificate recovery result [applied %d reads %d invokes %d]",
			len(applied),
			readCalls,
			invokeCalls,
		)
	}
}

func TestExecuteFrostNativeSignerAnchorTrustTransitionRetriesWithFreshRead(
	t *testing.T,
) {
	bootstrap, authority := trustTestBootstrapCertificate(t)
	second := trustTestRotationCertificate(t, bootstrap, authority, 0x95)
	final := trustTestRotationCertificate(t, second, authority, 0x96)
	configured := []FrostNativeSignerAnchorTrustCertificate{
		*bootstrap,
		*second,
		*final,
	}
	artifact, err :=
		authenticateFrostNativeSignerAnchorTrustRecoveryArtifact(
			configured,
			trustTestChainOptions(final),
		)
	if err != nil {
		t.Fatal(err)
	}
	recovery := testFrostNativeSignerAnchorTrustRecoverySelector(
		configured,
		0,
	)
	freshReads := [][]byte{
		[]byte(`{"fresh":"before-recovery-signal"}`),
		[]byte(`{"fresh":"after-recovery-signal"}`),
	}
	readCalls := 0
	invokeCalls := 0
	expectedResult :=
		&frostsigning.NativeTBTCSignerStateAnchorTrustTransitionResult{
			Installed: true,
		}
	result, target, applied, recoveryReplay, err :=
		executeFrostNativeSignerAnchorTrustTransition(
			context.Background(),
			artifact,
			[]FrostNativeSignerAnchorTrustCertificate{*final},
			nil,
			func(
				context.Context,
				bool,
			) (*FrostNativeSignerAnchorTrustTransitionTarget, error) {
				read := freshReads[readCalls]
				readCalls++
				return &FrostNativeSignerAnchorTrustTransitionTarget{
					Reference:         final.To.Reference,
					ExactReadResponse: read,
				}, nil
			},
			func(
				request []byte,
			) (*frostsigning.NativeTBTCSignerStateAnchorTrustTransitionResult, error) {
				decoded, err :=
					DecodeFrostNativeSignerAnchorTrustTransitionRequest(
						request,
					)
				if err != nil {
					t.Fatal(err)
				}
				invokeCalls++
				if invokeCalls == 1 {
					if len(decoded.CertificateChain) != 1 ||
						!bytes.Equal(
							decoded.TargetReadResponse,
							freshReads[0],
						) {
						t.Fatal("initial transition request is unexpected")
					}
					return nil,
						&frostsigning.NativeTBTCSignerStateAnchorTrustRecoveryRequiredError{
							Recovery: recovery,
						}
				}
				if len(decoded.CertificateChain) != 3 ||
					!bytes.Equal(
						decoded.TargetReadResponse,
						freshReads[1],
					) {
					t.Fatal("recovery retry reused stale Read or wrong chain")
				}
				return expectedResult, nil
			},
		)
	if err != nil {
		t.Fatal(err)
	}
	if result != expectedResult || target == nil ||
		!bytes.Equal(target.ExactReadResponse, freshReads[1]) ||
		len(applied) != 3 || !recoveryReplay ||
		readCalls != 2 || invokeCalls != 2 {
		t.Fatalf(
			"unexpected recovery retry [applied %d reads %d invokes %d]",
			len(applied),
			readCalls,
			invokeCalls,
		)
	}
}

func TestExecuteFrostNativeSignerAnchorTrustTransitionRejectsStaleRecoveryTarget(
	t *testing.T,
) {
	bootstrap, authority := trustTestBootstrapCertificate(t)
	final := trustTestRotationCertificate(t, bootstrap, authority, 0x97)
	configured := []FrostNativeSignerAnchorTrustCertificate{
		*bootstrap,
		*final,
	}
	artifact, err :=
		authenticateFrostNativeSignerAnchorTrustRecoveryArtifact(
			configured,
			trustTestChainOptions(final),
		)
	if err != nil {
		t.Fatal(err)
	}
	recovery := testFrostNativeSignerAnchorTrustRecoverySelector(
		configured,
		0,
	)
	staleReference := final.To.Reference
	staleReference.Revision++
	staleReference.PreviousEventRoot = staleReference.EventRoot
	staleReference.EventRoot[0] ^= 1
	readCalls := 0
	invokeCalls := 0
	_, _, _, _, err = executeFrostNativeSignerAnchorTrustTransition(
		context.Background(),
		artifact,
		nil,
		&recovery,
		func(
			context.Context,
			bool,
		) (*FrostNativeSignerAnchorTrustTransitionTarget, error) {
			readCalls++
			return &FrostNativeSignerAnchorTrustTransitionTarget{
				Reference:         staleReference,
				ExactReadResponse: []byte(`{"fresh":"but-target-is-stale"}`),
			}, nil
		},
		func(
			[]byte,
		) (*frostsigning.NativeTBTCSignerStateAnchorTrustTransitionResult, error) {
			invokeCalls++
			return nil, nil
		},
	)
	if err == nil || readCalls != 1 || invokeCalls != 0 {
		t.Fatalf(
			"stale restored target was not rejected before mutation [err %v reads %d invokes %d]",
			err,
			readCalls,
			invokeCalls,
		)
	}
}

func TestValidateFrostNativeSignerAnchorTrustTransitionResult(t *testing.T) {
	fixture := newFrostNativeSignerAnchorTrustStartupFixture()
	protocolHead, nativeHead, err :=
		validateFrostNativeSignerAnchorTrustExpectedHead(
			fixture.runtime,
			&fixture.installed,
			&fixture.certificate,
		)
	if err != nil {
		t.Fatal(err)
	}
	target := &FrostNativeSignerAnchorTrustTransitionTarget{
		ExactReadResponse: []byte(`{"fresh":"read"}`),
		Reference:         fixture.certificate.To.Reference,
	}
	result := frostNativeSignerAnchorTrustStartupResult(
		*nativeHead,
		target.Reference,
		fixture.certificate.To.Reference.Checkpoint,
		false,
		1,
	)
	chain := []FrostNativeSignerAnchorTrustCertificate{
		fixture.certificate,
	}
	if err := validateFrostNativeSignerAnchorTrustTransitionResult(
		&result,
		target,
		&fixture.certificate,
		protocolHead,
		nativeHead,
		nil,
		chain,
		false,
	); err != nil {
		t.Fatalf("valid trust transition result was rejected: %v", err)
	}

	tests := map[string]func(
		*frostsigning.NativeTBTCSignerStateAnchorTrustTransitionResult,
	){
		"not installed": func(
			result *frostsigning.NativeTBTCSignerStateAnchorTrustTransitionResult,
		) {
			result.Installed = false
		},
		"wrong head": func(
			result *frostsigning.NativeTBTCSignerStateAnchorTrustTransitionResult,
		) {
			result.TrustHead.CertificateDigest[0] ^= 1
		},
		"idempotent": func(
			result *frostsigning.NativeTBTCSignerStateAnchorTrustTransitionResult,
		) {
			result.Idempotent = true
		},
		"applied count": func(
			result *frostsigning.NativeTBTCSignerStateAnchorTrustTransitionResult,
		) {
			result.AppliedCertificateCount = 0
		},
		"current reference": func(
			result *frostsigning.NativeTBTCSignerStateAnchorTrustTransitionResult,
		) {
			result.CurrentAnchorReference.EventRoot[0] ^= 1
		},
		"current checkpoint": func(
			result *frostsigning.NativeTBTCSignerStateAnchorTrustTransitionResult,
		) {
			result.CurrentCheckpoint.Generation++
		},
		"witness base": func(
			result *frostsigning.NativeTBTCSignerStateAnchorTrustTransitionResult,
		) {
			result.WitnessBaseCheckpoint.Generation++
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			candidate := result
			mutate(&candidate)
			if err := validateFrostNativeSignerAnchorTrustTransitionResult(
				&candidate,
				target,
				&fixture.certificate,
				protocolHead,
				nativeHead,
				nil,
				chain,
				false,
			); err == nil {
				t.Fatal("invalid trust-transition result was accepted")
			}
		})
	}

	descendantTarget := *target
	descendantTarget.Reference.Revision++
	descendantTarget.Reference.PreviousEventRoot =
		target.Reference.EventRoot
	descendantTarget.Reference.EventRoot =
		frostNativeSignerAnchorTrustStartupBytes32(0xaf)
	descendantTarget.Reference.AcknowledgementDigest =
		frostNativeSignerAnchorTrustStartupBytes32(0xb0)
	descendantResult := frostNativeSignerAnchorTrustStartupResult(
		*nativeHead,
		descendantTarget.Reference,
		fixture.certificate.To.Reference.Checkpoint,
		false,
		1,
	)
	if err := validateFrostNativeSignerAnchorTrustTransitionResult(
		&descendantResult,
		&descendantTarget,
		&fixture.certificate,
		protocolHead,
		nativeHead,
		nil,
		chain,
		false,
	); err == nil {
		t.Fatal(
			"new transition accepted a descendant instead of the exact certified target",
		)
	}
}

func TestValidateFrostNativeSignerAnchorReconciledTransitionTargetScopesExactHeadBypass(
	t *testing.T,
) {
	fixture := newFrostNativeSignerAnchorTrustStartupFixture()
	reference := fixture.certificate.To.Reference
	tip := &frostsigning.NativeTBTCSignerStateWitnessTip{
		Schema:                      frostsigning.NativeTBTCSignerStateWitnessTipSchema,
		StoreFingerprint:            reference.Checkpoint.StoreFingerprint,
		Generation:                  reference.Checkpoint.Generation,
		PreviousStateCommitment:     reference.Checkpoint.PreviousStateCommitment,
		StateImageDigest:            reference.Checkpoint.StateImageDigest,
		StateCommitment:             reference.Checkpoint.StateCommitment,
		WitnessBaseGeneration:       reference.Checkpoint.Generation,
		WitnessBaseCommitment:       reference.Checkpoint.StateCommitment,
		AnchorBindingHash:           fixture.certificate.To.BindingHash,
		AnchorServiceEpoch:          reference.ServiceEpoch,
		AnchorRevision:              reference.Revision,
		AnchorEventRoot:             reference.EventRoot,
		AnchorAcknowledgementDigest: reference.AcknowledgementDigest,
	}
	target := &FrostNativeSignerAnchorTrustTransitionTarget{
		ExactReadResponse: []byte(`{"fresh":"transition"}`),
		Reference:         reference,
	}
	if err := validateFrostNativeSignerAnchorReconciledTransitionTarget(
		tip,
		target,
	); err != nil {
		t.Fatalf("exact missing-suffix target was rejected: %v", err)
	}

	repaired := *tip
	repaired.Generation++
	repaired.AnchorRevision++
	repaired.AnchorEventRoot[0] ^= 1
	repaired.AnchorAcknowledgementDigest[0] ^= 1
	if err := validateFrostNativeSignerAnchorReconciledTransitionTarget(
		&repaired,
		nil,
	); err != nil {
		t.Fatalf(
			"exact-head crash recovery was constrained by a stale target: %v",
			err,
		)
	}
	if err := validateFrostNativeSignerAnchorReconciledTransitionTarget(
		&repaired,
		target,
	); err == nil {
		t.Fatal("genuine missing-suffix transition accepted a different target")
	}
}

func TestValidateFrostNativeSignerAnchorTrustTransitionResultExactReplay(
	t *testing.T,
) {
	fixture := newFrostNativeSignerAnchorTrustStartupFixture()
	protocolHead, nativeHead, err :=
		validateFrostNativeSignerAnchorTrustExpectedHead(
			fixture.runtime,
			&fixture.installed,
			&fixture.certificate,
		)
	if err != nil {
		t.Fatal(err)
	}
	descendant := fixture.certificate.To.Reference
	descendant.Revision = 4
	descendant.PreviousEventRoot = descendant.EventRoot
	descendant.EventRoot =
		frostNativeSignerAnchorTrustStartupBytes32(0xb1)
	descendant.AcknowledgementDigest =
		frostNativeSignerAnchorTrustStartupBytes32(0xb2)
	descendant.Checkpoint.Generation += 3
	descendant.Checkpoint.PreviousStateCommitment =
		frostNativeSignerAnchorTrustStartupBytes32(0xb3)
	descendant.Checkpoint.StateImageDigest =
		frostNativeSignerAnchorTrustStartupBytes32(0xb4)
	descendant.Checkpoint.StateCommitment =
		frostsigning.ComputeNativeTBTCSignerStateWitnessCommitment(
			descendant.Checkpoint.StoreFingerprint,
			descendant.Checkpoint.Generation,
			descendant.Checkpoint.PreviousStateCommitment,
			descendant.Checkpoint.StateImageDigest,
		)
	target := &FrostNativeSignerAnchorTrustTransitionTarget{
		ExactReadResponse: []byte(`{"fresh":"descendant"}`),
		Reference:         descendant,
	}
	result := frostNativeSignerAnchorTrustStartupResult(
		*nativeHead,
		descendant,
		fixture.certificate.To.Reference.Checkpoint,
		true,
		0,
	)
	chain := []FrostNativeSignerAnchorTrustCertificate{
		fixture.certificate,
	}
	if err := validateFrostNativeSignerAnchorTrustTransitionResult(
		&result,
		target,
		&fixture.certificate,
		protocolHead,
		nativeHead,
		protocolHead,
		chain,
		false,
	); err != nil {
		t.Fatalf("valid completed-restart result was rejected: %v", err)
	}

	result.WitnessBaseCheckpoint.Generation =
		descendant.Checkpoint.Generation
	if err := validateFrostNativeSignerAnchorTrustTransitionResult(
		&result,
		target,
		&fixture.certificate,
		protocolHead,
		nativeHead,
		protocolHead,
		chain,
		false,
	); err == nil {
		t.Fatal("equal-generation forked replay witness base was accepted")
	}
}

func TestValidateFrostNativeSignerAnchorTrustTransitionResultRecoveryReplay(
	t *testing.T,
) {
	fixture := newFrostNativeSignerAnchorTrustStartupFixture()
	protocolHead, nativeHead, err :=
		validateFrostNativeSignerAnchorTrustExpectedHead(
			fixture.runtime,
			&fixture.installed,
			&fixture.certificate,
		)
	if err != nil {
		t.Fatal(err)
	}
	target := &FrostNativeSignerAnchorTrustTransitionTarget{
		ExactReadResponse: []byte(`{"fresh":"recovery"}`),
		Reference:         fixture.certificate.To.Reference,
	}
	chain := []FrostNativeSignerAnchorTrustCertificate{
		fixture.certificate,
	}
	floorCheckpoint := fixture.certificate.To.Reference.Checkpoint

	recovered := frostNativeSignerAnchorTrustStartupResult(
		*nativeHead,
		target.Reference,
		floorCheckpoint,
		true,
		0,
	)
	if err := validateFrostNativeSignerAnchorTrustTransitionResult(
		&recovered,
		target,
		&fixture.certificate,
		protocolHead,
		nativeHead,
		nil,
		chain,
		true,
	); err != nil {
		t.Fatalf("valid recovered replay at the certified floor was rejected: %v", err)
	}

	// The identical engine result without the recovery marker must fail: a
	// fresh transition may never report an idempotent zero-applied outcome.
	if err := validateFrostNativeSignerAnchorTrustTransitionResult(
		&recovered,
		target,
		&fixture.certificate,
		protocolHead,
		nativeHead,
		nil,
		chain,
		false,
	); err == nil {
		t.Fatal("idempotent zero-applied result was accepted as a fresh transition")
	}

	partiallyApplied := frostNativeSignerAnchorTrustStartupResult(
		*nativeHead,
		target.Reference,
		floorCheckpoint,
		true,
		1,
	)
	if err := validateFrostNativeSignerAnchorTrustTransitionResult(
		&partiallyApplied,
		target,
		&fixture.certificate,
		protocolHead,
		nativeHead,
		nil,
		chain,
		true,
	); err == nil {
		t.Fatal("recovery replay reporting fresh application was accepted")
	}

	nonIdempotent := frostNativeSignerAnchorTrustStartupResult(
		*nativeHead,
		target.Reference,
		floorCheckpoint,
		false,
		0,
	)
	if err := validateFrostNativeSignerAnchorTrustTransitionResult(
		&nonIdempotent,
		target,
		&fixture.certificate,
		protocolHead,
		nativeHead,
		nil,
		chain,
		true,
	); err == nil {
		t.Fatal("non-idempotent recovery replay was accepted")
	}

	divergedBase := floorCheckpoint
	divergedBase.Generation++
	divergedBase.StateCommitment =
		frostsigning.ComputeNativeTBTCSignerStateWitnessCommitment(
			divergedBase.StoreFingerprint,
			divergedBase.Generation,
			divergedBase.PreviousStateCommitment,
			divergedBase.StateImageDigest,
		)
	diverged := frostNativeSignerAnchorTrustStartupResult(
		*nativeHead,
		target.Reference,
		divergedBase,
		true,
		0,
	)
	if err := validateFrostNativeSignerAnchorTrustTransitionResult(
		&diverged,
		target,
		&fixture.certificate,
		protocolHead,
		nativeHead,
		nil,
		chain,
		true,
	); err == nil {
		t.Fatal("recovery replay witness base off the certified floor was accepted")
	}

	// A result cannot be both an exact-head replay and a recovery replay.
	if err := validateFrostNativeSignerAnchorTrustTransitionResult(
		&recovered,
		target,
		&fixture.certificate,
		protocolHead,
		nativeHead,
		protocolHead,
		chain,
		true,
	); err == nil {
		t.Fatal("ambiguous exact-head and recovery replay was accepted")
	}
}

func frostNativeSignerAnchorTrustStartupResult(
	head frostsigning.NativeTBTCSignerStateAnchorTrustHead,
	current FrostNativeSignerAnchorTrustReference,
	witnessBase FrostNativeSignerStateWitnessCheckpoint,
	idempotent bool,
	applied uint64,
) frostsigning.NativeTBTCSignerStateAnchorTrustTransitionResult {
	return frostsigning.NativeTBTCSignerStateAnchorTrustTransitionResult{
		Schema: frostsigning.
			NativeTBTCSignerStateAnchorTrustTransitionResultSchema,
		Installed:               true,
		Idempotent:              idempotent,
		AppliedCertificateCount: applied,
		TrustHead:               head,
		CurrentCheckpoint: frostNativeSignerAnchorNativeTrustCheckpoint(
			current.Checkpoint,
		),
		WitnessBaseCheckpoint: frostNativeSignerAnchorNativeTrustCheckpoint(
			witnessBase,
		),
		CurrentAnchorReference: frostNativeSignerAnchorNativeTrustReference(
			current,
		),
	}
}

func testFrostNativeSignerAnchorTrustRecoverySelector(
	configured []FrostNativeSignerAnchorTrustCertificate,
	start int,
) frostsigning.NativeTBTCSignerStateAnchorTrustRecoveryRequired {
	final := configured[len(configured)-1]
	digests := make([][32]byte, len(configured)-start)
	for index := range digests {
		digests[index] = configured[start+index].CertificateDigest
	}
	return frostsigning.NativeTBTCSignerStateAnchorTrustRecoveryRequired{
		Schema: frostsigning.
			NativeTBTCSignerStateAnchorTrustRecoveryRequiredSchema,
		StoreFingerprint:          final.SignerStoreFingerprint,
		CertificateCount:          uint64(len(digests)),
		FirstCertificateSequence:  configured[start].CertificateSequence,
		OrderedCertificateDigests: digests,
		FinalCertificateSequence:  final.CertificateSequence,
		FinalCertificateDigest:    final.CertificateDigest,
		TargetBindingHash:         final.To.BindingHash,
		TargetServiceEpoch:        final.To.Reference.ServiceEpoch,
		TargetRevision:            final.To.Reference.Revision,
		TargetCheckpoint: frostNativeSignerAnchorNativeTrustCheckpoint(
			final.To.Reference.Checkpoint,
		),
	}
}

func newFrostNativeSignerAnchorTrustStartupFixture() (
	fixture frostNativeSignerAnchorTrustStartupFixture,
) {
	protocolID := frostNativeSignerAnchorTrustStartupBytes32(0x11)
	streamID := frostNativeSignerAnchorTrustStartupBytes32(0x12)
	manifestHash := frostNativeSignerAnchorTrustStartupBytes32(0x13)
	storeFingerprint := frostNativeSignerAnchorTrustStartupBytes32(0x14)
	responsePublicKey :=
		frostNativeSignerAnchorTrustStartupPublicKey(0x21)
	authorityPublicKey :=
		frostNativeSignerAnchorTrustStartupPublicKey(0x31)
	responseSPKIHash :=
		ComputeFrostNativeSignerAnchorTrustEd25519SPKISHA256(
			responsePublicKey,
		)
	authoritySPKIHash :=
		ComputeFrostNativeSignerAnchorTrustEd25519SPKISHA256(
			authorityPublicKey,
		)
	identity := FrostNativeSignerAnchorIdentity{
		ProtocolID:                      protocolID,
		StreamID:                        streamID,
		ActivationManifestHash:          manifestHash,
		ActivationManifestSequence:      4,
		TrustDomainID:                   "trust.example",
		EndpointLeafSPKIHash:            frostNativeSignerAnchorTrustStartupBytes32(0x41),
		OnlineKeyHash:                   responseSPKIHash,
		OperatorFingerprint:             frostNativeSignerAnchorTrustStartupBytes32(0x42),
		HistoryStoreID:                  "history-1",
		HistoryStoreFingerprint:         frostNativeSignerAnchorTrustStartupBytes32(0x43),
		HistoryClusterFingerprint:       frostNativeSignerAnchorTrustStartupBytes32(0x44),
		OfflineAuthorityHash:            authoritySPKIHash,
		ClientSPKIHash:                  frostNativeSignerAnchorTrustStartupBytes32(0x45),
		SignerStoreFingerprint:          storeFingerprint,
		TransportBinding:                frostNativeSignerAnchorTrustStartupBytes32(0x46),
		WitnessMaximumRecords:           64,
		WitnessRotationThresholdRecords: 48,
	}
	identity.StreamID = ComputeFrostNativeSignerAnchorStreamID(identity)
	streamID = identity.StreamID
	bindingHash := ComputeFrostNativeSignerAnchorBindingHash(identity)
	checkpoint := FrostNativeSignerStateWitnessCheckpoint{
		StoreFingerprint:        storeFingerprint,
		Generation:              7,
		PreviousStateCommitment: frostNativeSignerAnchorTrustStartupBytes32(0x51),
		StateImageDigest:        frostNativeSignerAnchorTrustStartupBytes32(0x52),
	}
	checkpoint.StateCommitment =
		frostsigning.ComputeNativeTBTCSignerStateWitnessCommitment(
			checkpoint.StoreFingerprint,
			checkpoint.Generation,
			checkpoint.PreviousStateCommitment,
			checkpoint.StateImageDigest,
		)
	reference := FrostNativeSignerAnchorTrustReference{
		ServiceEpoch:          3,
		Revision:              1,
		PreviousEventRoot:     frostNativeSignerAnchorTrustStartupBytes32(0x53),
		EventRoot:             frostNativeSignerAnchorTrustStartupBytes32(0x54),
		AcknowledgementDigest: frostNativeSignerAnchorTrustStartupBytes32(0x55),
		Checkpoint:            checkpoint,
	}
	endpoint := FrostNativeSignerAnchorTrustEndpoint{
		ActivationManifestHash:          manifestHash,
		ActivationManifestSequence:      identity.ActivationManifestSequence,
		BindingHash:                     bindingHash,
		ResponsePublicKey:               responsePublicKey,
		ResponsePublicKeySPKISHA256:     responseSPKIHash,
		OfflineAuthorityPublicKey:       authorityPublicKey,
		OfflineAuthoritySPKISHA256:      authoritySPKIHash,
		WitnessMaximumRecords:           identity.WitnessMaximumRecords,
		WitnessRotationThresholdRecords: identity.WitnessRotationThresholdRecords,
		Reference:                       reference,
	}
	certificateDigest :=
		frostNativeSignerAnchorTrustStartupBytes32(0x61)
	fixture.runtime = FrostPreSignActivationRuntimeManifest{
		ManifestHash: manifestHash,
		NativeSignerAnchor: FrostNativeSignerAnchorManifest{
			Identity:                        identity,
			WitnessMaximumRecords:           identity.WitnessMaximumRecords,
			WitnessRotationThresholdRecords: identity.WitnessRotationThresholdRecords,
		},
		ActivationAuthorityPublicKey: authorityPublicKey,
	}
	fixture.installed =
		frostsigning.NativeTBTCSignerInstalledStateAnchorConfig{
			ProtocolID:                      protocolID,
			StreamID:                        streamID,
			ActivationManifestHash:          manifestHash,
			ActivationManifestSequence:      identity.ActivationManifestSequence,
			BindingHash:                     bindingHash,
			ResponsePublicKey:               responsePublicKey,
			ResponsePublicKeySPKISHA256:     responseSPKIHash,
			OfflineAuthorityPublicKey:       authorityPublicKey,
			OfflineAuthoritySPKISHA256:      authoritySPKIHash,
			TrustCertificateSequence:        3,
			TrustCertificateDigest:          certificateDigest,
			WitnessMaximumRecords:           identity.WitnessMaximumRecords,
			WitnessRotationThresholdRecords: identity.WitnessRotationThresholdRecords,
			ConfigFingerprint:               "installed-config",
		}
	fixture.certificate = FrostNativeSignerAnchorTrustCertificate{
		Kind:                      FrostNativeSignerAnchorTrustCertificateRotation,
		CertificateSequence:       fixture.installed.TrustCertificateSequence,
		CertificateDigest:         certificateDigest,
		ProtocolID:                protocolID,
		StreamID:                  streamID,
		SignerStoreFingerprint:    storeFingerprint,
		To:                        endpoint,
		PreviousCertificateDigest: frostNativeSignerAnchorTrustStartupBytes32(0x62),
	}
	return fixture
}

func newFrostNativeSignerAnchorTrustPriorStartupFixture() (
	frostNativeSignerAnchorTrustStartupFixture,
	FrostNativeSignerAnchorTrustCertificateHead,
	frostsigning.NativeTBTCSignerStateAnchorTrustHead,
) {
	fixture := newFrostNativeSignerAnchorTrustStartupFixture()
	priorResponsePublicKey :=
		frostNativeSignerAnchorTrustStartupPublicKey(0x71)
	priorResponseSPKIHash :=
		ComputeFrostNativeSignerAnchorTrustEd25519SPKISHA256(
			priorResponsePublicKey,
		)
	priorEndpoint := fixture.certificate.To
	priorEndpoint.ActivationManifestHash =
		frostNativeSignerAnchorTrustStartupBytes32(0x72)
	priorEndpoint.ActivationManifestSequence--
	priorEndpoint.BindingHash =
		frostNativeSignerAnchorTrustStartupBytes32(0x73)
	priorEndpoint.ResponsePublicKey = priorResponsePublicKey
	priorEndpoint.ResponsePublicKeySPKISHA256 = priorResponseSPKIHash
	priorEndpoint.Reference.ServiceEpoch--
	priorEndpoint.Reference.EventRoot =
		frostNativeSignerAnchorTrustStartupBytes32(0x74)
	priorEndpoint.Reference.AcknowledgementDigest =
		frostNativeSignerAnchorTrustStartupBytes32(0x75)
	fixture.certificate.From = &priorEndpoint
	priorHead := FrostNativeSignerAnchorTrustCertificateHead{
		CertificateSequence:    fixture.certificate.CertificateSequence - 1,
		CertificateDigest:      fixture.certificate.PreviousCertificateDigest,
		ProtocolID:             fixture.installed.ProtocolID,
		StreamID:               fixture.installed.StreamID,
		SignerStoreFingerprint: fixture.certificate.SignerStoreFingerprint,
		Endpoint:               priorEndpoint,
	}
	readback := frostNativeSignerAnchorNativeTrustHead(&priorHead)
	return fixture, priorHead, readback
}

func frostNativeSignerAnchorTrustStartupBytes32(value byte) [32]byte {
	result := [32]byte{}
	for i := range result {
		result[i] = value
	}
	return result
}

func frostNativeSignerAnchorTrustStartupPublicKey(seed byte) [32]byte {
	privateKey := ed25519.NewKeyFromSeed(
		bytes.Repeat([]byte{seed}, ed25519.SeedSize),
	)
	var result [32]byte
	copy(result[:], privateKey.Public().(ed25519.PublicKey))
	return result
}
