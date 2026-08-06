package tbtc

import (
	"context"
	"errors"
	"fmt"

	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
)

type frostNativeSignerAnchorAuthenticatedRecoveryArtifact struct {
	certificates []FrostNativeSignerAnchorTrustCertificate
	trustFloor   *frostNativeSignerAnchorVerifiedTrustFloor
}

type frostNativeSignerAnchorTrustTransitionTargetReader func(
	context.Context,
	bool,
) (*FrostNativeSignerAnchorTrustTransitionTarget, error)

type frostNativeSignerAnchorTrustTransitionInvoker func(
	[]byte,
) (*frostsigning.NativeTBTCSignerStateAnchorTrustTransitionResult, error)

// authenticateFrostNativeSignerAnchorTrustRecoveryArtifact establishes the
// authority recovery metadata deliberately lacks. Only a complete sequence-one
// artifact, independently authenticated through the pinned final head, can be
// used to select and replay a crash-recovery suffix.
func authenticateFrostNativeSignerAnchorTrustRecoveryArtifact(
	configured []FrostNativeSignerAnchorTrustCertificate,
	options FrostNativeSignerAnchorTrustChainValidationOptions,
) (*frostNativeSignerAnchorAuthenticatedRecoveryArtifact, error) {
	if len(configured) == 0 ||
		configured[0].CertificateSequence != 1 ||
		configured[0].PreviousCertificateDigest != [32]byte{} {
		return nil, fmt.Errorf(
			"native signer anchor recovery requires a complete sequence-one certificate artifact",
		)
	}
	options.PriorHead = nil
	trustFloor, err :=
		authenticateFrostNativeSignerAnchorTrustCertificateChain(
			configured,
			options,
		)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot independently authenticate the complete native signer anchor recovery artifact: %w",
			err,
		)
	}
	certificates := make(
		[]FrostNativeSignerAnchorTrustCertificate,
		len(configured),
	)
	for index := range configured {
		certificates[index] =
			frostNativeSignerAnchorTrustCloneCertificate(&configured[index])
	}
	return &frostNativeSignerAnchorAuthenticatedRecoveryArtifact{
		certificates: certificates,
		trustFloor:   trustFloor,
	}, nil
}

// selectFrostNativeSignerAnchorTrustRecoveryChain treats Rust's durable intent
// metadata strictly as a selector. It must identify one exact contiguous suffix
// of the independently authenticated artifact, end at the independently pinned
// final certificate, and describe that final certificate's embedded target.
func selectFrostNativeSignerAnchorTrustRecoveryChain(
	artifact *frostNativeSignerAnchorAuthenticatedRecoveryArtifact,
	recovery *frostsigning.NativeTBTCSignerStateAnchorTrustRecoveryRequired,
) ([]FrostNativeSignerAnchorTrustCertificate, error) {
	if artifact == nil || artifact.trustFloor == nil ||
		len(artifact.certificates) == 0 || recovery == nil ||
		recovery.CertificateCount == 0 ||
		uint64(len(recovery.OrderedCertificateDigests)) !=
			recovery.CertificateCount {
		return nil, fmt.Errorf(
			"native signer anchor trust-recovery selector inputs are incomplete",
		)
	}
	configured := artifact.certificates
	final := &configured[len(configured)-1]
	if recovery.StoreFingerprint != final.SignerStoreFingerprint ||
		recovery.FinalCertificateSequence != final.CertificateSequence ||
		recovery.FinalCertificateDigest != final.CertificateDigest ||
		recovery.TargetBindingHash != final.To.BindingHash ||
		recovery.TargetServiceEpoch != final.To.Reference.ServiceEpoch ||
		recovery.TargetRevision != final.To.Reference.Revision ||
		frostNativeSignerAnchorTrustCheckpointFromNative(
			recovery.TargetCheckpoint,
		) != final.To.Reference.Checkpoint {
		return nil, fmt.Errorf(
			"native signer anchor trust-recovery selector differs from the independently authenticated final certificate",
		)
	}
	if recovery.CertificateCount > uint64(len(configured)) {
		return nil, fmt.Errorf(
			"native signer anchor trust-recovery selector exceeds the configured artifact",
		)
	}

	match := -1
	for start := range configured {
		if uint64(len(configured)-start) < recovery.CertificateCount {
			break
		}
		matches := true
		for offset := uint64(0); offset < recovery.CertificateCount; offset++ {
			certificate := &configured[start+int(offset)]
			if certificate.CertificateSequence !=
				recovery.FirstCertificateSequence+offset ||
				certificate.CertificateDigest !=
					recovery.OrderedCertificateDigests[offset] {
				matches = false
				break
			}
		}
		if matches {
			if match >= 0 {
				return nil, fmt.Errorf(
					"native signer anchor trust-recovery selector matches the configured artifact more than once",
				)
			}
			match = start
		}
	}
	if match < 0 ||
		match+int(recovery.CertificateCount) != len(configured) {
		return nil, fmt.Errorf(
			"native signer anchor trust-recovery selector does not identify an exact configured suffix",
		)
	}

	result := make(
		[]FrostNativeSignerAnchorTrustCertificate,
		recovery.CertificateCount,
	)
	for index := range result {
		result[index] = frostNativeSignerAnchorTrustCloneCertificate(
			&configured[match+index],
		)
	}
	return result, nil
}

// executeFrostNativeSignerAnchorTrustTransition obtains a fresh signed Read for
// every transition attempt. A typed recovery failure is retried at most once,
// using only an exact suffix selected from the independently authenticated
// artifact; restored local intent bytes never supply authority or a target.
func executeFrostNativeSignerAnchorTrustTransition(
	ctx context.Context,
	artifact *frostNativeSignerAnchorAuthenticatedRecoveryArtifact,
	initialChain []FrostNativeSignerAnchorTrustCertificate,
	initialRecovery *frostsigning.NativeTBTCSignerStateAnchorTrustRecoveryRequired,
	readTarget frostNativeSignerAnchorTrustTransitionTargetReader,
	invoke frostNativeSignerAnchorTrustTransitionInvoker,
) (
	*frostsigning.NativeTBTCSignerStateAnchorTrustTransitionResult,
	*FrostNativeSignerAnchorTrustTransitionTarget,
	[]FrostNativeSignerAnchorTrustCertificate,
	bool,
	error,
) {
	if ctx == nil || readTarget == nil || invoke == nil {
		return nil, nil, nil, false, fmt.Errorf(
			"native signer anchor trust-transition executor dependencies are incomplete",
		)
	}

	chain := initialChain
	recoveryReplay := initialRecovery != nil
	if initialRecovery != nil {
		var err error
		chain, err = selectFrostNativeSignerAnchorTrustRecoveryChain(
			artifact,
			initialRecovery,
		)
		if err != nil {
			return nil, nil, nil, false, err
		}
	}
	if len(chain) == 0 {
		return nil, nil, nil, false, fmt.Errorf(
			"native signer anchor trust-transition certificate chain is empty",
		)
	}

	execute := func(
		chain []FrostNativeSignerAnchorTrustCertificate,
	) (
		*frostsigning.NativeTBTCSignerStateAnchorTrustTransitionResult,
		*FrostNativeSignerAnchorTrustTransitionTarget,
		error,
	) {
		target, err := readTarget(ctx, false)
		if err != nil {
			return nil, nil, fmt.Errorf(
				"cannot obtain a fresh native signer anchor trust-transition target: %w",
				err,
			)
		}
		final := &chain[len(chain)-1]
		if target == nil ||
			len(target.ExactReadResponse) == 0 ||
			target.Reference != final.To.Reference {
			return nil, nil, fmt.Errorf(
				"fresh native signer anchor trust-transition target differs from the selected certificate",
			)
		}
		request, err := EncodeFrostNativeSignerAnchorTrustTransitionRequest(
			&FrostNativeSignerAnchorTrustTransitionRequest{
				CertificateChain:   chain,
				TargetReadResponse: target.ExactReadResponse,
			},
		)
		if err != nil {
			return nil, nil, fmt.Errorf(
				"cannot encode native signer anchor trust transition: %w",
				err,
			)
		}
		result, err := invoke(request)
		return result, target, err
	}

	result, target, err := execute(chain)
	if err == nil {
		return result, target, chain, recoveryReplay, nil
	}
	if initialRecovery != nil {
		return nil, nil, nil, false, fmt.Errorf(
			"native signer anchor trust recovery retry failed: %w",
			err,
		)
	}
	var recoveryError *frostsigning.NativeTBTCSignerStateAnchorTrustRecoveryRequiredError
	if !errors.As(err, &recoveryError) {
		return nil, nil, nil, false, err
	}
	recoveryChain, selectErr :=
		selectFrostNativeSignerAnchorTrustRecoveryChain(
			artifact,
			&recoveryError.Recovery,
		)
	if selectErr != nil {
		return nil, nil, nil, false, selectErr
	}
	result, target, err = execute(recoveryChain)
	if err != nil {
		return nil, nil, nil, false, fmt.Errorf(
			"native signer anchor trust recovery retry failed: %w",
			err,
		)
	}
	return result, target, recoveryChain, true, nil
}

// validateFrostNativeSignerAnchorTrustExpectedHead derives the two exact head
// representations consumed during startup. It does so only after proving that
// the final certificate endpoint agrees with the independently authenticated
// runtime manifest and the exact configuration bytes already accepted by the
// native signer.
func validateFrostNativeSignerAnchorTrustExpectedHead(
	runtimeManifest FrostPreSignActivationRuntimeManifest,
	installed *frostsigning.NativeTBTCSignerInstalledStateAnchorConfig,
	finalCertificate *FrostNativeSignerAnchorTrustCertificate,
) (
	*FrostNativeSignerAnchorTrustCertificateHead,
	*frostsigning.NativeTBTCSignerStateAnchorTrustHead,
	error,
) {
	if err := validateFrostNativeSignerAnchorTrustRuntimePins(
		runtimeManifest,
		installed,
	); err != nil {
		return nil, nil, err
	}
	if finalCertificate == nil {
		return nil, nil, fmt.Errorf(
			"final native signer anchor trust certificate is nil",
		)
	}

	identity := runtimeManifest.NativeSignerAnchor.Identity
	anchorManifest := runtimeManifest.NativeSignerAnchor
	expectedBindingHash := ComputeFrostNativeSignerAnchorBindingHash(identity)
	if finalCertificate.CertificateSequence !=
		installed.TrustCertificateSequence ||
		finalCertificate.CertificateDigest !=
			installed.TrustCertificateDigest ||
		finalCertificate.ProtocolID != identity.ProtocolID ||
		finalCertificate.StreamID != identity.StreamID ||
		finalCertificate.SignerStoreFingerprint !=
			identity.SignerStoreFingerprint {
		return nil, nil, fmt.Errorf(
			"final native signer anchor trust certificate head pins differ from the installed configuration",
		)
	}

	expectedEndpoint := FrostNativeSignerAnchorTrustEndpoint{
		ActivationManifestHash:          runtimeManifest.ManifestHash,
		ActivationManifestSequence:      identity.ActivationManifestSequence,
		BindingHash:                     expectedBindingHash,
		ResponsePublicKey:               installed.ResponsePublicKey,
		ResponsePublicKeySPKISHA256:     installed.ResponsePublicKeySPKISHA256,
		OfflineAuthorityPublicKey:       runtimeManifest.ActivationAuthorityPublicKey,
		OfflineAuthoritySPKISHA256:      identity.OfflineAuthorityHash,
		WitnessMaximumRecords:           anchorManifest.WitnessMaximumRecords,
		WitnessRotationThresholdRecords: anchorManifest.WitnessRotationThresholdRecords,
		Reference:                       finalCertificate.To.Reference,
	}
	if finalCertificate.To != expectedEndpoint {
		return nil, nil, fmt.Errorf(
			"final native signer anchor trust certificate endpoint differs from the runtime and installed pins",
		)
	}
	if err := frostNativeSignerAnchorTrustValidateEndpoint(
		&expectedEndpoint,
		identity.SignerStoreFingerprint,
		"expected",
	); err != nil {
		return nil, nil, err
	}

	protocolHead := &FrostNativeSignerAnchorTrustCertificateHead{
		CertificateSequence:    finalCertificate.CertificateSequence,
		CertificateDigest:      finalCertificate.CertificateDigest,
		ProtocolID:             identity.ProtocolID,
		StreamID:               identity.StreamID,
		SignerStoreFingerprint: identity.SignerStoreFingerprint,
		Endpoint:               expectedEndpoint,
	}
	nativeHead := frostNativeSignerAnchorNativeTrustHead(protocolHead)
	return protocolHead, &nativeHead, nil
}

// reconstructFrostNativeSignerAnchorTrustPriorHead combines only authenticated
// native readback fields with stable local pins. The native ABI intentionally
// exposes only the online key's SPKI hash, so a missing-suffix transition may
// recover the raw prior key solely from the first certificate's From endpoint
// after checking its canonical SPKI hash. An exact completed replay instead
// recovers the final raw key from the independently installed configuration,
// which also permits replaying a bootstrap certificate with from:null.
func reconstructFrostNativeSignerAnchorTrustPriorHead(
	readback *frostsigning.NativeTBTCSignerStateAnchorTrustHead,
	runtimeManifest FrostPreSignActivationRuntimeManifest,
	installed *frostsigning.NativeTBTCSignerInstalledStateAnchorConfig,
	firstCertificate *FrostNativeSignerAnchorTrustCertificate,
) (*FrostNativeSignerAnchorTrustCertificateHead, error) {
	if err := validateFrostNativeSignerAnchorTrustRuntimePins(
		runtimeManifest,
		installed,
	); err != nil {
		return nil, err
	}
	if readback == nil || firstCertificate == nil {
		return nil, fmt.Errorf(
			"native signer anchor prior trust-head inputs are incomplete",
		)
	}
	if readback.Schema !=
		frostsigning.NativeTBTCSignerStateAnchorTrustHeadSchema ||
		readback.CertificateSequence == 0 ||
		readback.CertificateDigest == [32]byte{} ||
		readback.ActivationManifestSequence == 0 ||
		readback.ActivationManifestHash == [32]byte{} ||
		readback.BindingHash == [32]byte{} ||
		readback.ResponsePublicKeySPKISHA256 == [32]byte{} ||
		readback.ServiceEpoch == 0 {
		return nil, fmt.Errorf(
			"native signer anchor prior trust-head readback is incomplete",
		)
	}

	// A durable trust head ahead of the configured pin means the artifacts on
	// disk are older than this signer store - the configuration was rolled back
	// while the store was not. That already fails closed a few lines below,
	// because every rotation changes the activation manifest hash and so the
	// endpoint identities cannot match; naming the condition here only turns an
	// oblique identity mismatch into a diagnosable one.
	//
	// This is diagnosability, not a new guarantee, and in particular it is NOT a
	// bound on operator-side rollback: reverting the artifact bundle *and* the
	// signer store together leaves readback equal to installed and fires nothing
	// here or anywhere else. See "What the floor does not bound" in
	// docs/development/frost-anchor-rotation.adoc.
	if readback.CertificateSequence > installed.TrustCertificateSequence {
		return nil, fmt.Errorf(
			"durable native signer anchor trust head sequence [%v] is ahead of "+
				"the installed certificate pin [%v]; the configured anchor "+
				"artifacts are older than this signer store",
			readback.CertificateSequence,
			installed.TrustCertificateSequence,
		)
	}

	identity := runtimeManifest.NativeSignerAnchor.Identity
	anchorManifest := runtimeManifest.NativeSignerAnchor
	if readback.OfflineAuthoritySPKISHA256 !=
		installed.OfflineAuthoritySPKISHA256 ||
		readback.WitnessMaximumRecords !=
			anchorManifest.WitnessMaximumRecords ||
		readback.WitnessRotationThresholdRecords !=
			anchorManifest.WitnessRotationThresholdRecords ||
		readback.ServiceEpoch != readback.CertifiedFloor.ServiceEpoch ||
		readback.CertifiedFloor.Revision != 1 ||
		readback.CertifiedFloor.Checkpoint.StoreFingerprint !=
			identity.SignerStoreFingerprint {
		return nil, fmt.Errorf(
			"native signer anchor prior trust-head readback differs from stable local pins",
		)
	}
	if firstCertificate.ProtocolID != identity.ProtocolID ||
		firstCertificate.StreamID != identity.StreamID ||
		firstCertificate.SignerStoreFingerprint !=
			identity.SignerStoreFingerprint {
		return nil, fmt.Errorf(
			"first native signer anchor trust certificate differs from stable local pins",
		)
	}

	exactReplay := readback.CertificateSequence ==
		installed.TrustCertificateSequence &&
		readback.CertificateDigest == installed.TrustCertificateDigest &&
		firstCertificate.CertificateSequence == readback.CertificateSequence &&
		firstCertificate.CertificateDigest == readback.CertificateDigest

	var responsePublicKey [32]byte
	var certificateEndpoint *FrostNativeSignerAnchorTrustEndpoint
	if exactReplay {
		responsePublicKey = installed.ResponsePublicKey
		certificateEndpoint = &firstCertificate.To
	} else {
		if firstCertificate.From == nil {
			return nil, fmt.Errorf(
				"first missing native signer anchor trust certificate has no prior endpoint",
			)
		}
		responsePublicKey = firstCertificate.From.ResponsePublicKey
		certificateEndpoint = firstCertificate.From
	}
	if ComputeFrostNativeSignerAnchorTrustEd25519SPKISHA256(
		responsePublicKey,
	) != readback.ResponsePublicKeySPKISHA256 ||
		responsePublicKey == installed.OfflineAuthorityPublicKey {
		return nil, fmt.Errorf(
			"native signer anchor prior response key differs from authenticated readback or aliases the offline authority",
		)
	}

	endpoint := FrostNativeSignerAnchorTrustEndpoint{
		ActivationManifestHash:          readback.ActivationManifestHash,
		ActivationManifestSequence:      readback.ActivationManifestSequence,
		BindingHash:                     readback.BindingHash,
		ResponsePublicKey:               responsePublicKey,
		ResponsePublicKeySPKISHA256:     readback.ResponsePublicKeySPKISHA256,
		OfflineAuthorityPublicKey:       installed.OfflineAuthorityPublicKey,
		OfflineAuthoritySPKISHA256:      installed.OfflineAuthoritySPKISHA256,
		WitnessMaximumRecords:           readback.WitnessMaximumRecords,
		WitnessRotationThresholdRecords: readback.WitnessRotationThresholdRecords,
		Reference: frostNativeSignerAnchorTrustReferenceFromNative(
			readback.CertifiedFloor,
		),
	}
	if exactReplay {
		if *certificateEndpoint != endpoint {
			return nil, fmt.Errorf(
				"exact replay endpoint differs from authenticated native trust-head readback",
			)
		}
	} else {
		if !frostNativeSignerAnchorTrustStaticEndpointEqual(
			*certificateEndpoint,
			endpoint,
		) {
			return nil, fmt.Errorf(
				"certificate prior endpoint identity differs from authenticated native trust-head readback",
			)
		}
		if err := frostNativeSignerAnchorTrustValidateReferenceDescendant(
			endpoint.Reference,
			certificateEndpoint.Reference,
		); err != nil {
			return nil, fmt.Errorf(
				"certificate prior reference is not a descendant of the authenticated native trust-head floor: %w",
				err,
			)
		}
	}
	if err := frostNativeSignerAnchorTrustValidateEndpoint(
		&endpoint,
		identity.SignerStoreFingerprint,
		"prior",
	); err != nil {
		return nil, err
	}

	protocolHead := &FrostNativeSignerAnchorTrustCertificateHead{
		CertificateSequence:    readback.CertificateSequence,
		CertificateDigest:      readback.CertificateDigest,
		ProtocolID:             identity.ProtocolID,
		StreamID:               identity.StreamID,
		SignerStoreFingerprint: identity.SignerStoreFingerprint,
		Endpoint:               endpoint,
	}
	reconstructedReadback := frostNativeSignerAnchorNativeTrustHead(protocolHead)
	if reconstructedReadback != *readback {
		return nil, fmt.Errorf(
			"reconstructed native signer anchor prior head differs from authenticated readback",
		)
	}
	return protocolHead, nil
}

// selectFrostNativeSignerAnchorTrustTransitionChain converts the configured
// static artifact into the exact non-empty suffix Rust should apply. A
// completed restart replays only the final certificate, as required by the
// frozen ABI. A partially applied restart resumes strictly after the exact
// authenticated journal head. If the head immediately precedes the artifact,
// no item matches and the full configured suffix is retained for ordinary
// extension validation.
func selectFrostNativeSignerAnchorTrustTransitionChain(
	configured []FrostNativeSignerAnchorTrustCertificate,
	readback *frostsigning.NativeTBTCSignerStateAnchorTrustHead,
) ([]FrostNativeSignerAnchorTrustCertificate, error) {
	if len(configured) == 0 ||
		len(configured) >
			FrostNativeSignerAnchorTrustMaximumCertificateChainLength {
		return nil, fmt.Errorf(
			"configured native signer anchor trust certificate chain length is invalid",
		)
	}
	start := 0
	if readback != nil {
		match := -1
		for index := range configured {
			if configured[index].CertificateSequence ==
				readback.CertificateSequence &&
				configured[index].CertificateDigest ==
					readback.CertificateDigest {
				if match >= 0 {
					return nil, fmt.Errorf(
						"configured native signer anchor trust chain contains the authenticated head more than once",
					)
				}
				match = index
			}
		}
		switch {
		case match == len(configured)-1:
			// The transition request may not be empty. Replay the exact final
			// item so Rust can return its explicit idempotent result.
			start = match
		case match >= 0:
			start = match + 1
		}
	}
	result := append(
		[]FrostNativeSignerAnchorTrustCertificate{},
		configured[start:]...,
	)
	if len(result) == 0 {
		return nil, fmt.Errorf(
			"native signer anchor trust transition suffix is empty",
		)
	}
	return result, nil
}

func isFrostNativeSignerAnchorTrustExactHeadReplay(
	prior *FrostNativeSignerAnchorTrustCertificateHead,
	expected *FrostNativeSignerAnchorTrustCertificateHead,
) bool {
	return prior != nil && expected != nil && *prior == *expected
}

func validateFrostNativeSignerAnchorReconciledTransitionTarget(
	tip *frostsigning.NativeTBTCSignerStateWitnessTip,
	target *FrostNativeSignerAnchorTrustTransitionTarget,
) error {
	// A nil target is intentional for an authenticated exact-head restart.
	// Ordinary history reconciliation, not a stale pre-transition Read, is the
	// authority for repairing either crash window on that path.
	if target == nil {
		return nil
	}
	if tip == nil ||
		frostNativeSignerCheckpointFromTip(*tip) !=
			target.Reference.Checkpoint ||
		tip.AnchorServiceEpoch != target.Reference.ServiceEpoch ||
		tip.AnchorRevision != target.Reference.Revision ||
		tip.AnchorEventRoot != target.Reference.EventRoot ||
		tip.AnchorAcknowledgementDigest !=
			target.Reference.AcknowledgementDigest {
		return fmt.Errorf(
			"reconciled native signer tip differs from the fresh trust-transition target",
		)
	}
	return nil
}

func validateFrostNativeSignerAnchorTrustTransitionResult(
	result *frostsigning.NativeTBTCSignerStateAnchorTrustTransitionResult,
	target *FrostNativeSignerAnchorTrustTransitionTarget,
	finalCertificate *FrostNativeSignerAnchorTrustCertificate,
	expectedProtocolHead *FrostNativeSignerAnchorTrustCertificateHead,
	expectedNativeHead *frostsigning.NativeTBTCSignerStateAnchorTrustHead,
	priorHead *FrostNativeSignerAnchorTrustCertificateHead,
	appliedChain []FrostNativeSignerAnchorTrustCertificate,
	recoveryReplay bool,
) error {
	if result == nil || target == nil || finalCertificate == nil ||
		expectedProtocolHead == nil || expectedNativeHead == nil ||
		len(target.ExactReadResponse) == 0 ||
		len(appliedChain) == 0 ||
		len(appliedChain) >
			FrostNativeSignerAnchorTrustMaximumCertificateChainLength {
		return fmt.Errorf(
			"native signer anchor trust-transition validation inputs are incomplete",
		)
	}
	if !result.Installed || result.TrustHead != *expectedNativeHead {
		return fmt.Errorf(
			"native signer anchor trust-transition result differs from the independently pinned head",
		)
	}
	currentReference := frostNativeSignerAnchorTrustReferenceFromNative(
		result.CurrentAnchorReference,
	)
	currentCheckpoint := frostNativeSignerAnchorTrustCheckpointFromNative(
		result.CurrentCheckpoint,
	)
	witnessBase := frostNativeSignerAnchorTrustCheckpointFromNative(
		result.WitnessBaseCheckpoint,
	)
	exactReplay := priorHead != nil &&
		*priorHead == *expectedProtocolHead
	if !exactReplay && target.Reference != finalCertificate.To.Reference {
		return fmt.Errorf(
			"new native signer anchor trust transition does not use the exact certified target",
		)
	}
	if err := frostNativeSignerAnchorTrustValidateReferenceDescendant(
		finalCertificate.To.Reference,
		target.Reference,
	); err != nil {
		return fmt.Errorf(
			"fresh native signer anchor trust-transition target is not a certified-floor descendant: %w",
			err,
		)
	}
	if currentReference != target.Reference ||
		currentCheckpoint != target.Reference.Checkpoint {
		return fmt.Errorf(
			"native signer anchor trust-transition state differs from the fresh certified target",
		)
	}

	if exactReplay {
		if recoveryReplay {
			return fmt.Errorf(
				"native signer anchor trust-transition result is ambiguously both an exact-head and recovery replay",
			)
		}
		head := &appliedChain[len(appliedChain)-1]
		if len(appliedChain) != 1 ||
			head.CertificateSequence !=
				expectedProtocolHead.CertificateSequence ||
			head.CertificateDigest !=
				expectedProtocolHead.CertificateDigest ||
			head.To != expectedProtocolHead.Endpoint ||
			!result.Idempotent ||
			result.AppliedCertificateCount != 0 {
			return fmt.Errorf(
				"native signer anchor completed-restart result is not an exact idempotent replay",
			)
		}
		floorCheckpoint := finalCertificate.To.Reference.Checkpoint
		if witnessBase.StoreFingerprint != floorCheckpoint.StoreFingerprint ||
			witnessBase.Generation < floorCheckpoint.Generation ||
			witnessBase.Generation > currentCheckpoint.Generation ||
			(witnessBase.Generation == floorCheckpoint.Generation &&
				witnessBase != floorCheckpoint) ||
			(witnessBase.Generation == currentCheckpoint.Generation &&
				witnessBase != currentCheckpoint) {
			return fmt.Errorf(
				"native signer anchor completed-restart witness base is outside the retained certified segment",
			)
		}
		return nil
	}
	if recoveryReplay {
		if !result.Idempotent ||
			result.AppliedCertificateCount != 0 {
			return fmt.Errorf(
				"native signer anchor recovered transition is not an exact idempotent replay",
			)
		}
		if witnessBase != finalCertificate.To.Reference.Checkpoint {
			return fmt.Errorf(
				"native signer anchor recovered transition witness base differs from the recovered certified floor",
			)
		}
		return nil
	}
	if result.Idempotent ||
		result.AppliedCertificateCount != uint64(len(appliedChain)) {
		return fmt.Errorf(
			"native signer anchor trust-transition applied count differs from the authenticated missing suffix",
		)
	}
	if witnessBase != finalCertificate.To.Reference.Checkpoint {
		return fmt.Errorf(
			"native signer anchor trust-transition witness base differs from the new certified floor",
		)
	}
	return nil
}

func validateFrostNativeSignerAnchorTrustRuntimePins(
	runtimeManifest FrostPreSignActivationRuntimeManifest,
	installed *frostsigning.NativeTBTCSignerInstalledStateAnchorConfig,
) error {
	if installed == nil {
		return fmt.Errorf(
			"installed native signer anchor configuration is nil",
		)
	}
	identity := runtimeManifest.NativeSignerAnchor.Identity
	anchorManifest := runtimeManifest.NativeSignerAnchor
	expectedBindingHash := ComputeFrostNativeSignerAnchorBindingHash(identity)
	authoritySPKIHash :=
		ComputeFrostNativeSignerAnchorTrustEd25519SPKISHA256(
			runtimeManifest.ActivationAuthorityPublicKey,
		)
	responseSPKIHash :=
		ComputeFrostNativeSignerAnchorTrustEd25519SPKISHA256(
			installed.ResponsePublicKey,
		)
	installedAuthoritySPKIHash :=
		ComputeFrostNativeSignerAnchorTrustEd25519SPKISHA256(
			installed.OfflineAuthorityPublicKey,
		)

	if runtimeManifest.ManifestHash == [32]byte{} ||
		identity.ProtocolID == [32]byte{} ||
		identity.StreamID == [32]byte{} ||
		identity.SignerStoreFingerprint == [32]byte{} ||
		identity.ActivationManifestSequence == 0 ||
		installed.TrustCertificateSequence == 0 ||
		installed.TrustCertificateDigest == [32]byte{} ||
		runtimeManifest.ActivationAuthorityPublicKey == [32]byte{} ||
		installed.ResponsePublicKey == [32]byte{} ||
		installed.OfflineAuthorityPublicKey == [32]byte{} {
		return fmt.Errorf(
			"native signer anchor runtime or installed trust pins are incomplete",
		)
	}
	if identity.ActivationManifestHash != runtimeManifest.ManifestHash ||
		identity.StreamID != ComputeFrostNativeSignerAnchorStreamID(identity) ||
		installed.ProtocolID != identity.ProtocolID ||
		installed.StreamID != identity.StreamID ||
		installed.ActivationManifestHash != runtimeManifest.ManifestHash ||
		installed.ActivationManifestSequence !=
			identity.ActivationManifestSequence ||
		installed.BindingHash != expectedBindingHash ||
		installed.ResponsePublicKeySPKISHA256 != responseSPKIHash ||
		identity.OnlineKeyHash != responseSPKIHash ||
		installed.OfflineAuthorityPublicKey !=
			runtimeManifest.ActivationAuthorityPublicKey ||
		installed.OfflineAuthoritySPKISHA256 !=
			installedAuthoritySPKIHash ||
		authoritySPKIHash != identity.OfflineAuthorityHash ||
		installed.OfflineAuthoritySPKISHA256 != authoritySPKIHash ||
		anchorManifest.WitnessMaximumRecords !=
			identity.WitnessMaximumRecords ||
		anchorManifest.WitnessRotationThresholdRecords !=
			identity.WitnessRotationThresholdRecords ||
		installed.WitnessMaximumRecords !=
			anchorManifest.WitnessMaximumRecords ||
		installed.WitnessRotationThresholdRecords !=
			anchorManifest.WitnessRotationThresholdRecords {
		return fmt.Errorf(
			"installed native signer anchor configuration differs from the authenticated runtime manifest",
		)
	}
	if identity.ClientSPKIHash == identity.OnlineKeyHash ||
		identity.ClientSPKIHash == identity.OfflineAuthorityHash ||
		identity.OnlineKeyHash == identity.OfflineAuthorityHash {
		return fmt.Errorf(
			"native signer anchor client, online response, and offline authority key roles have aliases",
		)
	}
	if installed.ResponsePublicKey == installed.OfflineAuthorityPublicKey {
		return fmt.Errorf(
			"native signer anchor online response key aliases the offline authority",
		)
	}
	if err := frostsigning.ValidateNativeTBTCSignerStateWitnessGeometry(
		anchorManifest.WitnessMaximumRecords,
		anchorManifest.WitnessRotationThresholdRecords,
	); err != nil {
		return fmt.Errorf(
			"native signer anchor witness geometry is invalid: %w",
			err,
		)
	}
	return nil
}

func frostNativeSignerAnchorNativeTrustHead(
	head *FrostNativeSignerAnchorTrustCertificateHead,
) frostsigning.NativeTBTCSignerStateAnchorTrustHead {
	return frostsigning.NativeTBTCSignerStateAnchorTrustHead{
		Schema:                     frostsigning.NativeTBTCSignerStateAnchorTrustHeadSchema,
		CertificateSequence:        head.CertificateSequence,
		CertificateDigest:          head.CertificateDigest,
		ActivationManifestSequence: head.Endpoint.ActivationManifestSequence,
		ActivationManifestHash:     head.Endpoint.ActivationManifestHash,
		BindingHash:                head.Endpoint.BindingHash,
		ResponsePublicKeySPKISHA256: head.Endpoint.
			ResponsePublicKeySPKISHA256,
		OfflineAuthoritySPKISHA256: head.Endpoint.
			OfflineAuthoritySPKISHA256,
		ServiceEpoch: head.Endpoint.Reference.ServiceEpoch,
		CertifiedFloor: frostNativeSignerAnchorNativeTrustReference(
			head.Endpoint.Reference,
		),
		WitnessMaximumRecords: head.Endpoint.WitnessMaximumRecords,
		WitnessRotationThresholdRecords: head.Endpoint.
			WitnessRotationThresholdRecords,
	}
}

func frostNativeSignerAnchorNativeTrustReference(
	reference FrostNativeSignerAnchorTrustReference,
) frostsigning.NativeTBTCSignerStateAnchorTrustReference {
	return frostsigning.NativeTBTCSignerStateAnchorTrustReference{
		ServiceEpoch:          reference.ServiceEpoch,
		Revision:              reference.Revision,
		PreviousEventRoot:     reference.PreviousEventRoot,
		EventRoot:             reference.EventRoot,
		AcknowledgementDigest: reference.AcknowledgementDigest,
		Checkpoint: frostNativeSignerAnchorNativeTrustCheckpoint(
			reference.Checkpoint,
		),
	}
}

func frostNativeSignerAnchorNativeTrustCheckpoint(
	checkpoint FrostNativeSignerStateWitnessCheckpoint,
) frostsigning.NativeTBTCSignerStateAnchorCheckpoint {
	return frostsigning.NativeTBTCSignerStateAnchorCheckpoint{
		StoreFingerprint:        checkpoint.StoreFingerprint,
		Generation:              checkpoint.Generation,
		PreviousStateCommitment: checkpoint.PreviousStateCommitment,
		StateImageDigest:        checkpoint.StateImageDigest,
		StateCommitment:         checkpoint.StateCommitment,
	}
}

func frostNativeSignerAnchorTrustReferenceFromNative(
	reference frostsigning.NativeTBTCSignerStateAnchorTrustReference,
) FrostNativeSignerAnchorTrustReference {
	return FrostNativeSignerAnchorTrustReference{
		ServiceEpoch:          reference.ServiceEpoch,
		Revision:              reference.Revision,
		PreviousEventRoot:     reference.PreviousEventRoot,
		EventRoot:             reference.EventRoot,
		AcknowledgementDigest: reference.AcknowledgementDigest,
		Checkpoint: frostNativeSignerAnchorTrustCheckpointFromNative(
			reference.Checkpoint,
		),
	}
}

func frostNativeSignerAnchorTrustCheckpointFromNative(
	checkpoint frostsigning.NativeTBTCSignerStateAnchorCheckpoint,
) FrostNativeSignerStateWitnessCheckpoint {
	return FrostNativeSignerStateWitnessCheckpoint{
		StoreFingerprint:        checkpoint.StoreFingerprint,
		Generation:              checkpoint.Generation,
		PreviousStateCommitment: checkpoint.PreviousStateCommitment,
		StateImageDigest:        checkpoint.StateImageDigest,
		StateCommitment:         checkpoint.StateCommitment,
	}
}

func frostNativeSignerAnchorReferenceFromTrust(
	reference FrostNativeSignerAnchorTrustReference,
) FrostNativeSignerStateWitnessAnchorReference {
	return FrostNativeSignerStateWitnessAnchorReference{
		ServiceEpoch:          reference.ServiceEpoch,
		Revision:              reference.Revision,
		EventRoot:             reference.EventRoot,
		AcknowledgementDigest: reference.AcknowledgementDigest,
		Checkpoint:            reference.Checkpoint,
	}
}
