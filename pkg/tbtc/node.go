package tbtc

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/clientinfo"

	"github.com/keep-network/keep-common/pkg/persistence"
	"github.com/keep-network/keep-core/pkg/frost/signing"
	"github.com/keep-network/keep-core/pkg/generator"
	"github.com/keep-network/keep-core/pkg/net"
)

const (
	// signingAttemptsLimit determines the maximum number of signing attempts
	// that can be performed for the given message being subject of signing.
	//
	// The value of `5` should be enough to produce the signature even with
	// `2` malicious members in a signing group of `100` members. To produce
	// the signature, `51` members must be selected out of the honest `98`.
	// The probability of successful signing in that case is:
	// `P = (98 choose 51) / (100 choose 51) = ~0.24` which means we need
	// `5` attempts on the worst case.
	//
	// A greater limit does not necessarily make sense. Presence of more than
	// `2` malicious members in the signing group has a very small probability.
	// Moreover, the signature must be produced in the reasonable time.
	// That being said, the value `5` seems to be reasonable trade-off.
	signingAttemptsLimit = 5

	// walletClosureConfirmationBlocks determines the period used when waiting
	// for the wallet closure confirmation. This period ensures the wallet has
	// been definitely closed and the closing transaction will not be removed by
	// a chain reorganization.
	walletClosureConfirmationBlocks = 32
)

// TODO: Unit tests for `node.go`.

// node represents the current state of an ECDSA node.
type node struct {
	// groupParameters contains the legacy ECDSA wallet parameters.
	groupParameters *GroupParameters
	// frostGroupParameters contains the independently configured FROST wallet
	// parameters. FROST DKG must not use the legacy validator's constants: the
	// two registries can deliberately use different group sizes and thresholds.
	frostGroupParameters *GroupParameters

	chain          Chain
	btcChain       bitcoin.Chain
	netProvider    net.Provider
	walletRegistry *walletRegistry

	frostPreSignAuthorizationBackend FrostPreSignAuthorizationBackend
	frostPreSignActivationProfile    FrostPreSignActivationProfile
	frostDurableSessionStoreBinding  *frostDurableSessionStoreBinding
	frostProductionSignerReadiness   frostProductionSignerReadinessVerifier
	frostNativeSignerAnchorAdmission *frostNativeSignerAnchorAdmissionController
	bitcoinBroadcastOutbox           *bitcoinBroadcastOutbox
	frostRetainedGroupJournal        *frostRetainedGroupJournal
	frostActivationHandshakeExporter *frostActivationHandshakeExporter

	// walletDispatcher ensures only one action is executed by a wallet at
	// a time. All possible activities of a created wallet must be represented
	// by appropriate actions dispatched through this component.
	walletDispatcher *walletDispatcher

	// protocolLatch makes sure no expensive number generator operations are
	// running when signing or generating a wallet key are executed. The
	// protocolLatch is used by dkgExecutor and signingExecutor.
	protocolLatch *generator.ProtocolLatch

	// dkgExecutor encapsulates the logic of distributed key generation.
	//
	// dkgExecutor MUST NOT be used outside this struct.
	dkgExecutor *dkgExecutor

	// heartbeatFailureCounter stores the counters of consecutive heartbeat
	// failures for each wallet.
	heartbeatFailureCounter *heartbeatFailureCounter

	inactivityClaimExecutorMutex sync.Mutex
	// inactivityClaimExecutors is the cache holding inactivity claim executors
	// for specific wallets. The cache key is the uncompressed public key
	// (with 04 prefix) of the wallet.
	// inactivityClaimExecutor encapsulates the logic of handling inactivity
	// claim signing and submitting.
	//
	// inactivityClaimExecutors MUST NOT be used outside this struct. Please use
	// wallet actions and walletDispatcher to execute an action on an existing
	// wallet.
	inactivityClaimExecutors map[string]*inactivityClaimExecutor

	signingExecutorsMutex sync.Mutex
	// signingExecutors is the cache holding signing executors for specific wallets.
	// The cache key is the uncompressed public key (with 04 prefix) of the wallet.
	// signingExecutor encapsulates the generic logic of signing messages.
	//
	// signingExecutors MUST NOT be used outside this struct. Please use
	// wallet actions and walletDispatcher to execute an action on an existing
	// wallet.
	signingExecutors map[string]*signingExecutor

	coordinationExecutorsMutex sync.Mutex
	// coordinationExecutors is the cache holding coordination executors for
	// specific wallets. The cache key is the uncompressed public key
	// (with 04 prefix) of the wallet. The coordinationExecutor encapsulates the
	// logic of the wallet coordination procedure.
	//
	// coordinationExecutors MUST NOT be used outside this struct.
	coordinationExecutors map[string]*coordinationExecutor

	// proposalGenerator is the implementation of the coordination proposal
	// generator used by the node.
	proposalGenerator CoordinationProposalGenerator

	// performanceMetrics is optional and used for recording performance metrics
	performanceMetrics interface {
		IncrementCounter(name string, value float64)
		SetGauge(name string, value float64)
		RecordDuration(name string, duration time.Duration)
	}

	// windowMetricsTracker tracks detailed metrics for individual coordination windows
	windowMetricsTracker *coordinationWindowMetrics
}

func newNode(
	groupParameters *GroupParameters,
	chain Chain,
	btcChain bitcoin.Chain,
	netProvider net.Provider,
	keyStorePersistance persistence.ProtectedHandle,
	workPersistence persistence.BasicHandle,
	scheduler *generator.Scheduler,
	proposalGenerator CoordinationProposalGenerator,
	config Config,
) (*node, error) {
	if err := RegisterSignerMaterialResolverForBuild(); err != nil {
		return nil, fmt.Errorf(
			"cannot register signer material resolver for build: %w",
			err,
		)
	}

	if err := configureFrostSigningBackend(config); err != nil {
		return nil, fmt.Errorf("cannot configure FROST signing backend: %w", err)
	}

	// Fail fast on an invalid FROST signing backend here, right after it is
	// configured and before any further node construction - in particular before
	// newDkgExecutor below starts the legacy ECDSA pre-params pool, which
	// schedules CPU-heavy generation/persistence on a background context. This
	// keeps the fail-closed path side-effect free. verifyFrostSigningBackend is a
	// no-op unless FROST is enabled (a FROST wallet registry is configured); a
	// FROST-enabled node on the legacy backend, or without a usable/linked native
	// signer engine, cannot sign native FROST wallets.
	if frostChain, ok := chain.(FrostDKGChain); ok {
		if err := verifyFrostSigningBackend(
			frostChain.FrostWalletRegistryAvailable(),
		); err != nil {
			return nil, err
		}
	}

	walletRegistry, err := newWalletRegistry(
		keyStorePersistance,
		chain.CalculateWalletID,
	)
	if err != nil {
		return nil, fmt.Errorf("cannot create wallet registry: [%v]", err)
	}

	latch := generator.NewProtocolLatch()
	scheduler.RegisterProtocol(latch)

	node := &node{
		groupParameters: groupParameters,
		// Initialize this field for chains without FROST support and for tests that
		// construct a node directly. Initialize replaces it with the parameters
		// loaded from FrostDkgValidator whenever FROST is enabled.
		frostGroupParameters:     groupParameters,
		chain:                    chain,
		btcChain:                 btcChain,
		netProvider:              netProvider,
		walletRegistry:           walletRegistry,
		walletDispatcher:         newWalletDispatcher(),
		protocolLatch:            latch,
		heartbeatFailureCounter:  newHeartbeatFailureCounter(),
		signingExecutors:         make(map[string]*signingExecutor),
		inactivityClaimExecutors: make(map[string]*inactivityClaimExecutor),
		coordinationExecutors:    make(map[string]*coordinationExecutor),
		proposalGenerator:        proposalGenerator,
	}

	// Archive any wallets that might have been closed or terminated while the
	// client was turned off.
	err = node.archiveClosedWallets()
	if err != nil {
		return nil, fmt.Errorf("cannot archive closed wallets: [%v]", err)
	}

	// Only the operator address is known at this point and can be pre-fetched.
	// The operator ID must be determined later as the operator may not be in
	// the sortition pool yet.
	operatorAddress, err := node.operatorAddress()
	if err != nil {
		return nil, fmt.Errorf("cannot get node's operator address: [%v]", err)
	}

	if shouldRunLegacyECDSA(config) {
		// TODO: This chicken and egg problem should be solved when
		// waitForBlockHeight becomes a part of BlockHeightWaiter interface.
		node.dkgExecutor = newDkgExecutor(
			node.groupParameters,
			node.operatorID,
			operatorAddress,
			chain,
			netProvider,
			walletRegistry,
			latch,
			config,
			workPersistence,
			scheduler,
			node.waitForBlockHeight,
		)
	}

	if config.EnableFrostPreSignAuthorization {
		if !currentFrostInteractiveSigningReadiness() {
			return nil, fmt.Errorf(
				"cannot enable FROST pre-sign authorization: interactive native signer is not ready",
			)
		}
		activationProfile := config.FrostPreSignActivationProfile
		if configurator, ok := chain.(FrostPreSignAuthorizationConfigurator); ok {
			if config.FrostPreSignActivationManifestPath == "" {
				return nil, fmt.Errorf(
					"cannot enable production FROST pre-sign authorization without an activation manifest",
				)
			}
			verifierSource, ok := config.FrostRetainedGroupHistorySource.(FrostPreSignEthereumEvidenceVerifierSource)
			if !ok {
				return nil, fmt.Errorf(
					"cannot enable production FROST pre-sign authorization without an independent Ethereum evidence verifier",
				)
			}
			ethereumEvidenceVerifier, err :=
				verifierSource.FrostPreSignEthereumEvidenceVerifier(
					context.Background(),
				)
			if err != nil {
				return nil, fmt.Errorf(
					"cannot obtain independent FROST Ethereum evidence verifier: [%w]",
					err,
				)
			}
			configuredProfile, err := configurator.ConfigureFrostPreSignAuthorization(
				context.Background(),
				config.FrostPreSignActivationManifestPath,
				config.FrostPreSignActivationEnvelopeSignerKeyHash,
				config.FrostPreSignLinkedLibraryDescriptorSetHash,
				ethereumEvidenceVerifier,
			)
			if err != nil {
				return nil, fmt.Errorf(
					"cannot configure production FROST pre-sign authorization: [%w]",
					err,
				)
			}
			if activationProfile != nil &&
				*activationProfile != *configuredProfile {
				return nil, fmt.Errorf(
					"configured FROST activation profile differs from the verified manifest",
				)
			}
			activationProfile = configuredProfile
		}
		if activationProfile == nil {
			return nil, fmt.Errorf(
				"cannot enable FROST pre-sign authorization without a local activation profile",
			)
		}
		verifiedActivationProfile := *activationProfile
		if err := verifiedActivationProfile.validate(); err != nil {
			return nil, fmt.Errorf(
				"cannot enable FROST pre-sign authorization with invalid activation profile: [%w]",
				err,
			)
		}
		frostChain, ok := chain.(FrostDKGChain)
		if !ok || !frostChain.FrostWalletRegistryAvailable() {
			return nil, fmt.Errorf(
				"cannot enable FROST pre-sign authorization without a configured FROST wallet registry",
			)
		}
		backend, ok := chain.(FrostPreSignAuthorizationBackend)
		if !ok {
			return nil, fmt.Errorf(
				"cannot enable FROST pre-sign authorization: anchoring chain does not implement the authorization backend",
			)
		}
		canonicalBitcoinChain, ok := btcChain.(canonicalBitcoinBroadcastChain)
		if !ok {
			return nil, fmt.Errorf(
				"cannot enable FROST pre-sign authorization: Bitcoin backend is not an authenticated canonical transaction index",
			)
		}
		authorizationStatusSource, ok := chain.(FrostBitcoinBroadcastAuthorizationStatusSource)
		if !ok {
			return nil, fmt.Errorf(
				"cannot enable FROST pre-sign authorization: anchoring chain does not implement canonical broadcast-authorization revalidation",
			)
		}
		outbox, err := newBitcoinBroadcastOutbox(
			config.BitcoinBroadcastOutboxDirectory,
			canonicalBitcoinChain,
			authorizationStatusSource,
			verifiedActivationProfile.ProfileHash,
		)
		if err != nil {
			return nil, fmt.Errorf("cannot initialize durable Bitcoin broadcast outbox: [%w]", err)
		}
		manifestSource, ok := chain.(FrostPreSignActivationRuntimeManifestSource)
		if !ok {
			_ = outbox.close()
			return nil, fmt.Errorf(
				"cannot enable FROST activation handshake: chain does not expose the authenticated runtime manifest",
			)
		}
		pointVerifier, ok := chain.(FrostPreSignActivationPointVerifier)
		if !ok {
			_ = outbox.close()
			return nil, fmt.Errorf(
				"cannot enable FROST activation handshake: chain cannot verify exact finalized deployment points",
			)
		}
		runtimeManifest, err := manifestSource.FrostPreSignActivationRuntimeManifest()
		if err != nil {
			_ = outbox.close()
			return nil, fmt.Errorf("cannot read authenticated FROST runtime manifest: [%w]", err)
		}
		anchorManifest := runtimeManifest.NativeSignerAnchor
		clientPrivateKey, err := loadFrostNativeSignerAnchorClientPrivateKey(
			config.FrostNativeSignerAnchorClientPrivateKeyPath,
		)
		if err != nil {
			_ = outbox.close()
			return nil, fmt.Errorf(
				"cannot load FROST native signer anchor client key: [%w]",
				err,
			)
		}
		defer zeroFrostNativeSignerKeyBytes(clientPrivateKey)
		onlineKeySPKI, onlinePublicKey, err :=
			loadFrostNativeSignerAnchorOnlinePublicKeySPKI(
				config.FrostNativeSignerAnchorOnlinePublicKeyPath,
			)
		if err != nil {
			_ = outbox.close()
			return nil, fmt.Errorf(
				"cannot load FROST native signer anchor online key: [%w]",
				err,
			)
		}
		installedAnchorConfig, err :=
			signing.ReadInstalledNativeTBTCSignerStateAnchorConfig()
		if err != nil {
			_ = outbox.close()
			return nil, fmt.Errorf(
				"cannot verify installed native signer anchor configuration: [%w]",
				err,
			)
		}
		onlineRawKey := [32]byte{}
		copy(onlineRawKey[:], onlinePublicKey)
		trustCertificateJSON, err :=
			loadFrostNativeSignerAnchorTrustCertificateChain(
				config.FrostNativeSignerAnchorTrustCertificatePath,
			)
		if err != nil {
			_ = outbox.close()
			return nil, fmt.Errorf(
				"cannot load FROST native signer anchor trust certificates: [%w]",
				err,
			)
		}
		trustCertificateChain, err :=
			DecodeFrostNativeSignerAnchorTrustCertificateChain(
				trustCertificateJSON,
			)
		if err != nil {
			_ = outbox.close()
			return nil, fmt.Errorf(
				"cannot decode FROST native signer anchor trust certificates: [%w]",
				err,
			)
		}
		finalTrustCertificate :=
			&trustCertificateChain[len(trustCertificateChain)-1]
		expectedTrustCertificateHead, expectedTrustHead, err :=
			validateFrostNativeSignerAnchorTrustExpectedHead(
				runtimeManifest,
				installedAnchorConfig,
				finalTrustCertificate,
			)
		if err != nil {
			_ = outbox.close()
			return nil, fmt.Errorf(
				"cannot bind the expected FROST native signer anchor trust head: [%w]",
				err,
			)
		}
		if installedAnchorConfig.ResponsePublicKey != onlineRawKey ||
			finalTrustCertificate.To.ResponsePublicKey != onlineRawKey {
			_ = outbox.close()
			return nil, fmt.Errorf(
				"native signer anchor online key file differs from the installed and certified key",
			)
		}

		var priorTrustCertificateHead *FrostNativeSignerAnchorTrustCertificateHead
		var trustRecoverySelector *signing.NativeTBTCSignerStateAnchorTrustRecoveryRequired
		priorTrustHead, priorTrustHeadErr :=
			signing.ReadNativeTBTCSignerStateAnchorTrustHead()
		transitionCertificateChain := trustCertificateChain
		if priorTrustHeadErr == nil {
			transitionCertificateChain, err =
				selectFrostNativeSignerAnchorTrustTransitionChain(
					trustCertificateChain,
					priorTrustHead,
				)
			if err != nil {
				_ = outbox.close()
				return nil, fmt.Errorf(
					"cannot select the missing FROST native signer anchor trust-certificate suffix: [%w]",
					err,
				)
			}
			priorTrustCertificateHead, err =
				reconstructFrostNativeSignerAnchorTrustPriorHead(
					priorTrustHead,
					runtimeManifest,
					installedAnchorConfig,
					&transitionCertificateChain[0],
				)
			if err != nil {
				_ = outbox.close()
				return nil, fmt.Errorf(
					"cannot authenticate the installed FROST native signer anchor trust head: [%w]",
					err,
				)
			}
		} else {
			var recoveryError *signing.NativeTBTCSignerStateAnchorTrustRecoveryRequiredError
			switch {
			case errors.As(priorTrustHeadErr, &recoveryError):
				recovery := recoveryError.Recovery
				recovery.OrderedCertificateDigests = append(
					[][32]byte{},
					recovery.OrderedCertificateDigests...,
				)
				trustRecoverySelector = &recovery
			case errors.Is(
				priorTrustHeadErr,
				signing.ErrNativeTBTCSignerStateAnchorTrustHeadAbsent,
			):
				if transitionCertificateChain[0].CertificateSequence != 1 ||
					transitionCertificateChain[0].PreviousCertificateDigest !=
						[32]byte{} {
					_ = outbox.close()
					return nil, fmt.Errorf(
						"cannot read the prior FROST native signer anchor trust head for a certificate suffix: [%w]",
						priorTrustHeadErr,
					)
				}
			default:
				_ = outbox.close()
				return nil, fmt.Errorf(
					"cannot read the prior FROST native signer anchor trust head: [%w]",
					priorTrustHeadErr,
				)
			}
		}
		trustChainOptions := FrostNativeSignerAnchorTrustChainValidationOptions{
			// A sequence-one rotation is an explicit, offline-authorized
			// adoption of a pre-journal anchor. The configured owner-only
			// certificate artifact and exact installed final digest are the
			// local authorization to perform it.
			AllowLegacyAdoption: true,
			ExpectedProtocolID:  anchorManifest.Identity.ProtocolID,
			ExpectedStreamID:    anchorManifest.Identity.StreamID,
			ExpectedSignerStoreFingerprint: anchorManifest.Identity.
				SignerStoreFingerprint,
			ExpectedOfflineAuthorityPublicKey: runtimeManifest.
				ActivationAuthorityPublicKey,
			ExpectedOfflineAuthoritySPKISHA256: anchorManifest.Identity.
				OfflineAuthorityHash,
			PriorHead:    priorTrustCertificateHead,
			ExpectedHead: expectedTrustCertificateHead,
			ValidateTargetAcknowledgement: func(
				certificate *FrostNativeSignerAnchorTrustCertificate,
				rawAcknowledgement []byte,
			) error {
				return ValidateFrostNativeSignerAnchorTrustTargetAcknowledgement(
					certificate,
					rawAcknowledgement,
				)
			},
		}
		verifiedTrustFloor, err :=
			authenticateFrostNativeSignerAnchorTrustCertificateChain(
				transitionCertificateChain,
				trustChainOptions,
			)
		if err != nil {
			_ = outbox.close()
			return nil, fmt.Errorf(
				"cannot authenticate FROST native signer anchor trust-certificate chain: [%w]",
				err,
			)
		}
		recoveryArtifact, err :=
			authenticateFrostNativeSignerAnchorTrustRecoveryArtifact(
				trustCertificateChain,
				trustChainOptions,
			)
		if err != nil {
			_ = outbox.close()
			return nil, fmt.Errorf(
				"cannot authenticate the complete FROST native signer anchor recovery artifact: [%w]",
				err,
			)
		}
		anchorClient, err :=
			newFrostNativeSignerAnchorClientWithTrustFloor(
				FrostNativeSignerAnchorClientConfig{
					Endpoint: config.FrostNativeSignerAnchorURL,
					RequestTimeout: config.
						FrostNativeSignerAnchorRequestTimeout,
					ClientPrivateKey:    clientPrivateKey,
					OnlinePublicKeySPKI: onlineKeySPKI,
					Identity:            anchorManifest.Identity,
				},
				verifiedTrustFloor,
			)
		if err != nil {
			_ = outbox.close()
			return nil, fmt.Errorf(
				"cannot initialize authenticated native signer anchor client: [%w]",
				err,
			)
		}
		exactTrustHeadReplay :=
			isFrostNativeSignerAnchorTrustExactHeadReplay(
				priorTrustCertificateHead,
				expectedTrustCertificateHead,
			)
		var trustTransitionTarget *FrostNativeSignerAnchorTrustTransitionTarget
		if !exactTrustHeadReplay {
			var trustTransitionResult *signing.NativeTBTCSignerStateAnchorTrustTransitionResult
			var trustTransitionRecoveryReplay bool
			trustTransitionResult,
				trustTransitionTarget,
				transitionCertificateChain,
				trustTransitionRecoveryReplay,
				err = executeFrostNativeSignerAnchorTrustTransition(
				context.Background(),
				recoveryArtifact,
				transitionCertificateChain,
				trustRecoverySelector,
				anchorClient.
					readFrostNativeSignerAnchorTrustTransitionTarget,
				signing.TransitionNativeTBTCSignerStateWitnessAnchor,
			)
			if err != nil {
				_ = outbox.close()
				return nil, fmt.Errorf(
					"cannot install FROST native signer anchor trust transition: [%w]",
					err,
				)
			}
			if err := validateFrostNativeSignerAnchorTrustTransitionResult(
				trustTransitionResult,
				trustTransitionTarget,
				finalTrustCertificate,
				expectedTrustHead,
				transitionCertificateChain,
				trustTransitionRecoveryReplay,
			); err != nil {
				_ = outbox.close()
				return nil, fmt.Errorf(
					"cannot validate native signer anchor trust-transition result: [%w]",
					err,
				)
			}
		}
		installedTrustHead, err :=
			signing.ReadNativeTBTCSignerStateAnchorTrustHead()
		if err != nil {
			_ = outbox.close()
			return nil, fmt.Errorf(
				"cannot read back the exact installed native signer anchor trust head: [%w]",
				err,
			)
		}
		if installedTrustHead == nil ||
			*installedTrustHead != *expectedTrustHead {
			_ = outbox.close()
			return nil, fmt.Errorf(
				"installed native signer anchor trust head differs from the independently pinned expected head",
			)
		}

		// A missing suffix must transition before any durable signer-store
		// access. An exact authenticated head deliberately skips the strict
		// replay transition so ordinary reconciliation can repair either crash
		// window where the durable tip or remote CAS is ahead of the persisted
		// local acknowledgement.
		storeBinding, err := newFrostDurableSessionStoreBinding(
			runtimeManifest.DurableSessionStoreFingerprint,
			signing.ReadNativeTBTCSignerDurableStoreIdentity,
		)
		if err != nil {
			_ = outbox.close()
			return nil, fmt.Errorf(
				"cannot bind the active FROST durable session store to the signed manifest: [%w]",
				err,
			)
		}
		certifiedFloor := frostNativeSignerAnchorReferenceFromTrust(
			finalTrustCertificate.To.Reference,
		)
		expectedAnchorBindingHash := finalTrustCertificate.To.BindingHash
		anchorBinding, err := newFrostNativeSignerAnchorBinding(
			anchorClient,
			anchorManifest,
			certifiedFloor,
			finalTrustCertificate.To.Reference.PreviousEventRoot,
			signing.ReadNativeTBTCSignerStateWitnessTip,
			signing.ReadNativeTBTCSignerStateWitnessProof,
			signing.AcknowledgeNativeTBTCSignerStateWitnessCheckpoint,
			signing.RecoverNativeTBTCSignerStateWitnessCheckpoint,
		)
		if err != nil {
			_ = outbox.close()
			return nil, fmt.Errorf(
				"cannot bind authenticated native signer anchor: [%w]",
				err,
			)
		}
		anchorAdmission, err :=
			newFrostNativeSignerAnchorAdmissionController(anchorBinding)
		if err != nil {
			_ = outbox.close()
			return nil, fmt.Errorf(
				"cannot initialize native signer anchor admission: [%w]",
				err,
			)
		}
		startupSignerTip, err := anchorBinding.reconcileStartup(
			context.Background(),
		)
		if err != nil {
			_ = outbox.close()
			return nil, fmt.Errorf(
				"cannot reconcile startup native signer anchor: [%w]",
				err,
			)
		}
		if err := validateFrostNativeSignerAnchorReconciledTransitionTarget(
			startupSignerTip,
			trustTransitionTarget,
		); err != nil {
			_ = outbox.close()
			return nil, err
		}
		if err := signing.InstallNativeTBTCSignerStateAnchorBarrier(
			signing.NativeTBTCSignerStateAnchorBarrierConfig{
				InitialTip:                                startupSignerTip,
				ExpectedAnchorBindingHash:                 expectedAnchorBindingHash,
				MinimumAnchorServiceEpoch:                 certifiedFloor.ServiceEpoch,
				MaximumAnchorRevisionDistance:             FrostNativeSignerAnchorMaximumHistoryEvents,
				MaximumStateGenerationDistance:            FrostNativeSignerAnchorMaximumHistoryProofEntries,
				MaximumStateGenerationAdvancePerOperation: frostNativeSignerMaximumGenerationAdvancesPerAnchoredCall,
				ExpectedTrustHead:                         expectedTrustHead,
				ReadTip:                                   signing.ReadNativeTBTCSignerStateWitnessTip,
				ReadTrustHead: signing.
					ReadNativeTBTCSignerStateAnchorTrustHead,
				Committer: anchorBinding,
				Timeout:   config.FrostNativeSignerAnchorRequestTimeout,
			},
		); err != nil {
			_ = outbox.close()
			return nil, fmt.Errorf(
				"cannot install native signer protocol-output barrier: [%w]",
				err,
			)
		}
		inventoryBinding, err := newFrostNativeSignerInventoryBinding(
			storeBinding,
			anchorBinding,
			signing.ReadNativeTBTCSignerRetainedKeyPackageInventory,
			signing.ReadNativeTBTCSignerStateAnchorTrustHead,
			expectedTrustHead,
		)
		if err != nil {
			_ = outbox.close()
			return nil, fmt.Errorf(
				"cannot bind the native FROST key-package inventory and rollback witness: [%w]",
				err,
			)
		}
		if config.FrostRetainedGroupHistorySource == nil {
			_ = outbox.close()
			return nil, fmt.Errorf(
				"cannot enable FROST activation handshake without an independent retained-group history source",
			)
		}
		evidenceBinder, ok := config.FrostRetainedGroupHistorySource.(FrostRetainedGroupActivationEvidenceBinder)
		if !ok {
			_ = outbox.close()
			return nil, fmt.Errorf(
				"cannot enable FROST activation handshake without a manifest-bound retained-group evidence source",
			)
		}
		if err := evidenceBinder.BindFrostRetainedGroupActivationEvidence(
			verifiedActivationProfile,
			runtimeManifest,
		); err != nil {
			_ = outbox.close()
			return nil, fmt.Errorf(
				"cannot bind retained-group evidence to the authenticated activation manifest: [%w]",
				err,
			)
		}
		bindingSource, ok :=
			config.FrostRetainedGroupHistorySource.(FrostRetainedGroupProtocolBindingSource)
		if !ok {
			_ = outbox.close()
			return nil, fmt.Errorf(
				"cannot enable FROST activation handshake without a protocol-bound retained-group source",
			)
		}
		retainedGroupBindingHash, err :=
			bindingSource.FrostRetainedGroupProtocolBindingHash()
		if err != nil {
			_ = outbox.close()
			return nil, fmt.Errorf(
				"cannot read retained-group protocol binding: [%w]",
				err,
			)
		}
		journal, err := newFrostRetainedGroupJournal(
			config.FrostRetainedGroupJournalDirectory,
			retainedGroupBindingHash,
			runtimeManifest,
			config.FrostRetainedGroupHistorySource,
			walletRegistry,
			operatorAddress,
		)
		if err != nil {
			_ = outbox.close()
			return nil, fmt.Errorf("cannot initialize canonical FROST retained-group journal: [%w]", err)
		}
		node.frostRetainedGroupJournal = journal
		orphanedDKGReconciler, err := newFrostOrphanedDKGReconciler(
			chain,
			walletRegistry,
			anchorAdmission,
		)
		if err != nil {
			_ = journal.close()
			_ = outbox.close()
			return nil, fmt.Errorf(
				"cannot initialize orphaned FROST DKG reconciliation: [%w]",
				err,
			)
		}
		journal.orphanedDKGReconciler = orphanedDKGReconciler
		readiness, err := newFrostProductionSignerReadiness(
			currentFrostInteractiveSigningReadiness,
			journal,
			inventoryBinding,
		)
		if err != nil {
			_ = journal.close()
			_ = outbox.close()
			return nil, fmt.Errorf("cannot initialize FROST signer readiness: [%w]", err)
		}
		startupFinality, err := backend.CurrentFrostPreSignFinality(context.Background())
		if err != nil {
			_ = journal.close()
			_ = outbox.close()
			return nil, fmt.Errorf(
				"cannot obtain the startup FROST signer-readiness checkpoint: [%w]",
				err,
			)
		}
		if startupFinality == nil || startupFinality.BlockNumber == 0 ||
			startupFinality.BlockHash == [32]byte{} {
			_ = journal.close()
			_ = outbox.close()
			return nil, fmt.Errorf(
				"cannot obtain a valid startup FROST signer-readiness checkpoint",
			)
		}
		// Every authenticated readiness reconciliation already carries the
		// restartable revision and generation headroom, so the scrapeable
		// mirror is fed from the path that computes them instead of being
		// recomputed on the scrape. Recomputing costs the anchor binding
		// mutex - which CommitNativeTBTCSignerStateTransition holds across the
		// remote CAS for the whole of a signing commit - plus an authenticated
		// read of the remote anchor service, and neither belongs on a metrics
		// timer: the scrape would stall behind an in-flight signing operation
		// and would put a network call on a fixed tick. Wrapping here covers
		// both the startup verification immediately below and every later
		// pre-sign authorization, which is the only recurring caller of the
		// verifier this node stores.
		observedReadiness :=
			newFrostNativeSignerAnchorHeadroomObserver(readiness)
		if _, err := observedReadiness.verifyFrostProductionSignerReadiness(
			context.Background(),
			*startupFinality,
		); err != nil {
			_ = journal.close()
			_ = outbox.close()
			return nil, fmt.Errorf(
				"cannot enable FROST pre-sign authorization with an unready signer: [%w]",
				err,
			)
		}
		exporter, err := newFrostActivationHandshakeExporter(
			config.FrostActivationHandshakeURL,
			config.FrostActivationHandshakePrivateKeyPath,
			runtimeManifest,
			pointVerifier,
			storeBinding,
			outbox,
			journal,
			readiness,
		)
		if err != nil {
			_ = journal.close()
			_ = outbox.close()
			return nil, fmt.Errorf("cannot initialize FROST activation handshake: [%w]", err)
		}
		node.frostPreSignAuthorizationBackend = backend
		node.frostPreSignActivationProfile = verifiedActivationProfile
		node.frostDurableSessionStoreBinding = storeBinding
		node.frostProductionSignerReadiness = observedReadiness
		node.frostNativeSignerAnchorAdmission = anchorAdmission
		node.bitcoinBroadcastOutbox = outbox
		node.frostActivationHandshakeExporter = exporter
	}

	return node, nil
}

// frostNativeSignerAnchorHeadroomObserver decorates the production signer
// readiness verifier so that the restartable revision and generation headroom
// each successful reconciliation already computed reaches the scrapeable
// mirror in pkg/frost/signing.
//
// It is a decorator rather than a call inside the verifier because those two
// numbers are only trustworthy once the reconciliation that produced them has
// succeeded: the snapshot is assembled after the local tip has been
// authenticated against the remote anchor, and a failed verification can
// return a partially built or nil snapshot whose headroom means nothing. So
// only the success path publishes, and a failed reconciliation deliberately
// leaves the previous value standing rather than zeroing it - zero is the
// value that means "the certified windows are exhausted", and a transport blip
// must not be reported as that.
//
// Publishing costs one atomic store on a path that has just performed remote
// I/O, so it is not measurable there. Nothing reads the mirror back to make a
// decision; it exists solely so an operator can see the windows drain in time
// to schedule the offline rotation ceremony.
type frostNativeSignerAnchorHeadroomObserver struct {
	inner frostProductionSignerReadinessVerifier
}

func newFrostNativeSignerAnchorHeadroomObserver(
	inner frostProductionSignerReadinessVerifier,
) frostProductionSignerReadinessVerifier {
	if inner == nil {
		return nil
	}
	return &frostNativeSignerAnchorHeadroomObserver{inner: inner}
}

func (fnsaho *frostNativeSignerAnchorHeadroomObserver) verifyFrostProductionSignerReadiness(
	ctx context.Context,
	finality FrostPreSignFinality,
) (*frostProductionSignerReadinessSnapshot, error) {
	snapshot, err := fnsaho.inner.verifyFrostProductionSignerReadiness(
		ctx,
		finality,
	)
	if err == nil && snapshot != nil && snapshot.Inventory != nil {
		signing.RecordNativeTBTCSignerStateAnchorRestartableHeadroom(
			snapshot.Inventory.RestartableRevisionHeadroom,
			snapshot.Inventory.RestartableGenerationHeadroom,
		)
	}
	return snapshot, err
}

// verifyFrostProductionSignerReadinessUnchanged publishes nothing. It proves a
// previously reconciled snapshot has not changed and returns no fresh headroom
// of its own, so republishing the caller's snapshot here would only restate a
// value the mirror already holds while bumping the observation counter that
// tells an operator how fresh that value is.
func (fnsaho *frostNativeSignerAnchorHeadroomObserver) verifyFrostProductionSignerReadinessUnchanged(
	ctx context.Context,
	snapshot *frostProductionSignerReadinessSnapshot,
) error {
	return fnsaho.inner.verifyFrostProductionSignerReadinessUnchanged(
		ctx,
		snapshot,
	)
}

func configureFrostSigningBackend(config Config) error {
	return signing.SetExecutionBackendByName(config.FrostSigningBackend)
}

// verifyFrostSigningBackend fails fast when FROST DKG is enabled while the
// configured signing backend is the transitional legacy backend. Native FROST
// wallets carry native signer material that the legacy backend cannot process,
// so a node left on the legacy backend produces valid wallets via DKG but then
// fails every signing attempt (heartbeat, deposit sweep, redemption, ...). The
// native backend handles both native FROST and legacy-ECDSA material, so it is
// always the correct choice once FROST is enabled.
//
// The guard is a no-op when FROST is not enabled (frostEnabled == false): the
// normal Ethereum TbtcChain satisfies FrostDKGChain even when no FROST wallet
// registry is configured, and in that case the node has no FROST wallets to
// sign for, so the default legacy backend is fine.
func verifyFrostSigningBackend(frostEnabled bool) error {
	if !frostEnabled {
		return nil
	}

	backend := signing.CurrentExecutionBackendName()
	if backend == signing.LegacyExecutionBackendName {
		return fmt.Errorf(
			"FROST DKG is enabled but the FROST signing backend is [%s]; set "+
				"tbtc.frostSigningBackend to \"native\" or \"ffi\" - the legacy "+
				"backend cannot sign native FROST wallets and would fail every "+
				"signature",
			backend,
		)
	}

	// A non-legacy backend name is not sufficient: the fallback-allowed "native"
	// mode is selected without verifying that native execution is actually
	// available, so an unavailable native engine would fall back to the legacy
	// bridge and fail on native FROST signer material at signing time. Require
	// usable native execution up front.
	if !signing.NativeExecutionAvailable() {
		return fmt.Errorf(
			"FROST DKG is enabled with signing backend [%s] but native FROST "+
				"execution is unavailable in this build/runtime; use "+
				"tbtc.frostSigningBackend=\"ffi\" with the native tbtc-signer "+
				"linked in, otherwise signing falls back to the legacy bridge "+
				"and fails on native FROST wallets",
			backend,
		)
	}
	return nil
}

// setPerformanceMetrics sets the performance metrics recorder for the node
// and wires it into components that support metrics.
func (n *node) setPerformanceMetrics(metrics interface {
	IncrementCounter(name string, value float64)
	SetGauge(name string, value float64)
	RecordDuration(name string, duration time.Duration)
}) {
	n.performanceMetrics = metrics

	if metrics == nil {
		signing.UnregisterNativeTBTCSignerFallbackObserver()
	} else {
		err := signing.RegisterNativeTBTCSignerFallbackObserver(
			func(event signing.NativeTBTCSignerFallbackEvent) {
				metrics.IncrementCounter(
					clientinfo.MetricSigningNativeTBTCSignerFallbackTotal,
					1,
				)
			},
		)
		if err != nil {
			logger.Warnf(
				"cannot register native tbtc-signer fallback observer: [%v]",
				err,
			)
		}
	}

	// Initialize window metrics tracker with performance metrics
	// Keep metrics for the last 100 windows (approximately 25 hours at 900 blocks per window)
	if perfMetrics, ok := metrics.(clientinfo.PerformanceMetricsRecorder); ok {
		n.windowMetricsTracker = newCoordinationWindowMetrics(perfMetrics, 100)
	}

	if n.walletDispatcher != nil {
		n.walletDispatcher.setMetricsRecorder(metrics)
	}
	if n.dkgExecutor != nil {
		n.dkgExecutor.setMetricsRecorder(metrics)
	}

	// Wire redemption metrics to proposal generator if it supports it
	// This uses a type assertion to check if proposalGenerator is a *ProposalGenerator
	// from the tbtcpg package. We can't import tbtcpg here to avoid circular dependencies,
	// so we use an interface check instead.
	if pg, ok := n.proposalGenerator.(interface {
		SetRedemptionMetricsRecorder(recorder interface {
			SetGauge(name string, value float64)
		})
	}); ok {
		pg.SetRedemptionMetricsRecorder(metrics)
	}

	// Update metrics recorder for all cached coordination executors
	// This is important because executors may be created before metrics are set
	n.coordinationExecutorsMutex.Lock()
	for _, executor := range n.coordinationExecutors {
		executor.setMetricsRecorder(metrics)
	}
	n.coordinationExecutorsMutex.Unlock()
}

// GetCoordinationWindowsSummary returns a summary of coordination window metrics.
// Returns nil if the window metrics tracker is not initialized.
func (n *node) GetCoordinationWindowsSummary() *WindowMetricsSummary {
	if n.windowMetricsTracker == nil {
		return nil
	}
	summary := n.windowMetricsTracker.GetSummary()
	return &summary
}

// operatorAddress returns the node's operator address.
func (n *node) operatorAddress() (chain.Address, error) {
	_, operatorPublicKey, err := n.chain.OperatorKeyPair()
	if err != nil {
		return "", fmt.Errorf("failed to get operator public key: [%v]", err)
	}

	operatorAddress, err := n.chain.Signing().PublicKeyToAddress(operatorPublicKey)
	if err != nil {
		return "", fmt.Errorf(
			"failed to convert operator public key to address: [%v]",
			err,
		)
	}

	return operatorAddress, nil
}

// operatorAddress returns the node's operator ID.
func (n *node) operatorID() (chain.OperatorID, error) {
	operatorAddress, err := n.operatorAddress()
	if err != nil {
		return 0, fmt.Errorf("failed to get operator address: [%v]", err)
	}

	operatorID, err := n.chain.GetOperatorID(operatorAddress)
	if err != nil {
		return 0, fmt.Errorf("failed to get operator ID: [%v]", err)
	}

	return operatorID, nil
}

// joinDKGIfEligible takes a seed value and undergoes the process of the
// distributed key generation if this node's operator proves to be eligible for
// the group generated by that seed. This is an interactive on-chain process,
// and joinDKGIfEligible can block for an extended period of time while it
// completes the on-chain operation. The execution can be delayed by an
// arbitrary number of blocks using the delayBlocks argument. This allows
// confirming the state on-chain - e.g. wait for the required number of
// confirming blocks - before executing the off-chain action.
func (n *node) joinDKGIfEligible(
	seed *big.Int,
	startBlock uint64,
	delayBlocks uint64,
) {
	if n.dkgExecutor == nil {
		logger.Warnf("legacy ECDSA DKG is disabled; ignoring DKG started event")
		return
	}

	n.dkgExecutor.executeDkgIfEligible(seed, startBlock, delayBlocks)
}

// validateDKG performs the submitted DKG result validation process.
// If the result is not valid, this function submits an on-chain result
// challenge. If the result is valid and the given node was involved in the DKG,
// this function schedules an on-chain approve that is submitted once the
// challenge period elapses.
func (n *node) validateDKG(
	seed *big.Int,
	submissionBlock uint64,
	result *DKGChainResult,
	resultHash [32]byte,
) {
	if n.dkgExecutor == nil {
		logger.Warnf("legacy ECDSA DKG is disabled; ignoring DKG result")
		return
	}

	n.dkgExecutor.executeDkgValidation(seed, submissionBlock, result, resultHash)
}
