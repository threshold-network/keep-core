package tbtc

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/keep-network/keep-common/pkg/chain/ethereum"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/clientinfo"

	"go.uber.org/zap"

	"github.com/keep-network/keep-common/pkg/persistence"
	"github.com/keep-network/keep-core/pkg/frost/signing"
	"github.com/keep-network/keep-core/pkg/generator"
	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/protocol/announcer"
	"github.com/keep-network/keep-core/pkg/protocol/group"
	"github.com/keep-network/keep-core/pkg/protocol/inactivity"
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
				expectedTrustCertificateHead,
				expectedTrustHead,
				priorTrustCertificateHead,
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

// getSigningExecutor gets the signing executor responsible for executing
// signing related to a specific wallet whose part is controlled by this node.
// The second boolean return value indicates whether the node controls at least
// one signer for the given wallet.
func (n *node) getSigningExecutor(
	walletPublicKey *ecdsa.PublicKey,
) (*signingExecutor, bool, error) {
	n.signingExecutorsMutex.Lock()
	defer n.signingExecutorsMutex.Unlock()

	walletPublicKeyBytes, err := marshalPublicKey(walletPublicKey)
	if err != nil {
		return nil, false, fmt.Errorf("cannot marshal wallet public key: [%v]", err)
	}

	executorKey := hex.EncodeToString(walletPublicKeyBytes)

	if executor, exists := n.signingExecutors[executorKey]; exists {
		return executor, true, nil
	}

	executorLogger := logger.With(
		zap.String("wallet", fmt.Sprintf("0x%x", walletPublicKeyBytes)),
	)

	signers := n.walletRegistry.getSigners(walletPublicKey)
	if len(signers) == 0 {
		// This is not an error because the node simply does not control
		// the given wallet.
		return nil, false, nil
	}

	// All signers belong to one wallet. Take that wallet from the
	// first signer.
	wallet := signers[0].wallet

	channelName := fmt.Sprintf(
		"%s-%s",
		ProtocolName,
		hex.EncodeToString(walletPublicKeyBytes),
	)

	broadcastChannel, err := n.netProvider.BroadcastChannelFor(channelName)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get broadcast channel: [%v]", err)
	}

	signing.RegisterUnmarshallers(broadcastChannel)
	announcer.RegisterUnmarshaller(broadcastChannel)
	broadcastChannel.SetUnmarshaler(func() net.TaggedUnmarshaler {
		return &signingDoneMessage{}
	})

	membershipValidator := group.NewMembershipValidator(
		executorLogger,
		wallet.signingGroupOperators,
		n.chain.Signing(),
	)

	err = broadcastChannel.SetFilter(membershipValidator.IsInGroup)
	if err != nil {
		return nil, false, fmt.Errorf(
			"could not set filter for channel [%v]: [%v]",
			broadcastChannel.Name(),
			err,
		)
	}

	executorLogger.Infof(
		"signing executor created; controlling [%v] signers",
		len(signers),
	)

	// Register a ROAST-retry coordinator for each local seat of this wallet so
	// the interactive FROST signing path can run for it. This must happen before
	// the executor is used: the executor entry's BeginOrchestrationForSession
	// looks the coordinator up per member, and an absent coordinator makes the
	// interactive drive fall through (fail-closed under interactive-only mode).
	// No-op unless built with frost_native && frost_roast_retry.
	registerRoastRetryCoordinatorForSeats(n, signers)

	blockCounter, err := n.chain.BlockCounter()
	if err != nil {
		return nil, false, fmt.Errorf(
			"could not get block counter: [%v]",
			err,
		)
	}

	walletGroupParameters, err := n.groupParametersForSigners(signers)
	if err != nil {
		return nil, false, err
	}

	executor := newSigningExecutor(
		signers,
		broadcastChannel,
		membershipValidator,
		walletGroupParameters,
		n.protocolLatch,
		blockCounter.CurrentBlock,
		n.waitForBlockHeight,
		signingAttemptsLimit,
	)
	if n.frostPreSignAuthorizationBackend != nil {
		localMemberIndexes := make([]group.MemberIndex, 0, len(signers))
		for _, signer := range signers {
			localMemberIndexes = append(
				localMemberIndexes,
				signer.signingGroupMemberIndex,
			)
		}
		gate, err := newThresholdFrostPreSignAuthorizationGate(
			n.frostPreSignAuthorizationBackend,
			n.frostPreSignActivationProfile,
			n.frostDurableSessionStoreBinding,
			n.frostProductionSignerReadiness,
			n.frostNativeSignerAnchorAdmission,
			n.chain.Signing(),
			broadcastChannel,
			membershipValidator,
			wallet,
			localMemberIndexes,
		)
		if err != nil {
			return nil, false, fmt.Errorf(
				"cannot create FROST pre-sign authorization gate: [%w]",
				err,
			)
		}
		executor.preSignAuthorizationGate = gate
		executor.broadcastOutbox = n.bitcoinBroadcastOutbox

		// The seat ceiling is a property of this node's seat count in this
		// wallet and of protocol constants, so it is decided the moment the
		// wallet is formed and never changes for the wallet's whole life. The
		// gate is the only thing that reports it otherwise, and only inside the
		// error of an authorization it has already refused, so an operator over
		// the ceiling would learn about it the first time a deposit sweep is
		// proposed - which may be weeks after the seats were awarded, and is
		// the worst possible moment. Say it once, here, at the same place the
		// executor announces how many signers it controls.
		//
		// No production node is expected to trip this now that admission
		// reserves one input at a time: a wallet's whole hundred-seat set fits
		// the certified windows for one input. It stays because the numbers it
		// reads are protocol constants, and a change to any of them must
		// announce itself rather than quietly excluding an operator again.
		//
		// The gate's own deduplicated, validated seat set is used rather than
		// localMemberIndexes because that is exactly what reservePreSign
		// charges for; a duplicated index in the registry would otherwise make
		// this warn about a seat count admission never sees.
		//
		// Gated on Schnorr material because getSigningExecutor builds a gate
		// for every wallet on a FROST-enabled node, including the legacy ECDSA
		// wallets still draining, and walletTransactionExecutor only routes
		// through the gate when the wallet signs with Schnorr. Warning about a
		// seat ceiling that cannot apply to an ECDSA wallet would be noise.
		if executor.usesSchnorrSignatures() {
			if warning, exceeded := frostPreSignLocalSeatCeilingWarning(
				uint64(len(gate.localMemberIndexes)),
				gate.maximumAttempts,
			); exceeded {
				executorLogger.Warnf("%s", warning)
			}
		}
	}

	// Wire metrics recorder if available
	if n.performanceMetrics != nil {
		executor.setMetricsRecorder(n.performanceMetrics)
	}

	n.signingExecutors[executorKey] = executor

	return executor, true, nil
}

// frostPreSignLocalSeatCeilingWarning states, at wallet-executor construction
// time, that this node's seat count in a FROST wallet is above the count anchor
// admission can serve for a single pre-sign transaction input. It returns the
// warning text and whether the ceiling is exceeded at all.
//
// The exclusion this reports is whole-node, not per-seat.
// thresholdFrostPreSignAuthorizationGate charges reservePreSign for its
// complete local seat set in one reservation, so a set one seat over the
// ceiling fails the reservation and the node contributes nothing at all - every
// one of its seats is lost to the wallet's signing threshold, not just the
// surplus one. That is worth stating explicitly because the natural reading of
// "a ceiling of N" is that N seats still sign, and they do not.
//
// It no longer takes a batch size, and there is no longer a second lever to
// offer. Anchor admission reserves one input at a time and a batch signs its
// inputs sequentially, so one input costs the same in a one-input batch as in a
// full twenty-one input sweep: an operator over this ceiling cannot get under
// it by proposing smaller sweeps, only by shedding seats. Under the current
// constants nothing is over it - a wallet's entire hundred-seat set fits inside
// the certified windows for one input - so this is a guard against a constant
// change rather than a warning any production node is expected to see.
//
// Kept as a pure function of the two numbers so it can be exercised without
// standing up a wallet, a gate, or a native build.
func frostPreSignLocalSeatCeilingWarning(
	localSeatCount uint64,
	maximumSigningAttempts uint64,
) (string, bool) {
	// Mirrors the gate: an unset limit means the package default, and charging
	// the ceiling scan a different attempt count than admission uses would make
	// this warn about a ceiling that does not exist.
	if maximumSigningAttempts == 0 {
		maximumSigningAttempts = signingAttemptsLimit
	}

	admissibleSeats := frostPreSignMaximumAdmissibleLocalSeatCount(
		maximumSigningAttempts,
	)
	if admissibleSeats > 0 && localSeatCount <= admissibleSeats {
		return "", false
	}
	if admissibleSeats == 0 {
		// Not this node's problem to fix: no seat count at all can serve even a
		// single input under the current windows, so shedding seats would not
		// help and only a protocol-level change would.
		return fmt.Sprintf(
			"this node holds [%d] of this FROST wallet's seats, and no local "+
				"seat count can sign a single pre-sign transaction input within "+
				"the certified anchor restart windows; every deposit sweep on this "+
				"wallet will be refused for every member, and only enlarging those "+
				"windows or lowering the signing-attempt limit [%d] changes that",
			localSeatCount,
			maximumSigningAttempts,
		), true
	}

	return fmt.Sprintf(
		"this node holds [%d] of this FROST wallet's seats, above the [%d]-seat "+
			"ceiling for a single pre-sign transaction input; anchor admission "+
			"refuses the node as a whole rather than the surplus seats, so all "+
			"[%d] of its seats are excluded from every deposit sweep for this "+
			"wallet's whole life and are lost to the wallet's signing threshold. "+
			"Batch size is not a lever - admission reserves one input at a time "+
			"and a sweep signs its inputs sequentially - so shed seats down to "+
			"[%d]",
		localSeatCount,
		admissibleSeats,
		localSeatCount,
		admissibleSeats,
	), true
}

// getCoordinationExecutor gets the coordination executor responsible for
// executing coordination related to a specific wallet whose part is controlled
// by this node. The second boolean return value indicates whether the node
// controls at least one signer for the given wallet.
func (n *node) getCoordinationExecutor(
	walletPublicKey *ecdsa.PublicKey,
) (*coordinationExecutor, bool, error) {
	n.coordinationExecutorsMutex.Lock()
	defer n.coordinationExecutorsMutex.Unlock()

	walletPublicKeyBytes, err := marshalPublicKey(walletPublicKey)
	if err != nil {
		return nil, false, fmt.Errorf("cannot marshal wallet public key: [%v]", err)
	}

	executorKey := hex.EncodeToString(walletPublicKeyBytes)

	if executor, exists := n.coordinationExecutors[executorKey]; exists {
		// Ensure metrics recorder is set if metrics are available
		// (executor may have been created before metrics were initialized)
		if n.performanceMetrics != nil {
			executor.setMetricsRecorder(n.performanceMetrics)
		}
		return executor, true, nil
	}

	executorLogger := logger.With(
		zap.String("wallet", fmt.Sprintf("0x%x", walletPublicKeyBytes)),
	)

	signers := n.walletRegistry.getSigners(walletPublicKey)
	if len(signers) == 0 {
		// This is not an error because the node simply does not control
		// the given wallet.
		return nil, false, nil
	}

	// All signers belong to one wallet. Take that wallet from the
	// first signer.
	wallet := signers[0].wallet

	channelName := fmt.Sprintf(
		"%s-%s-coordination",
		ProtocolName,
		hex.EncodeToString(walletPublicKeyBytes),
	)

	broadcastChannel, err := n.netProvider.BroadcastChannelFor(channelName)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get broadcast channel: [%v]", err)
	}

	broadcastChannel.SetUnmarshaler(func() net.TaggedUnmarshaler {
		return &coordinationMessage{}
	})

	membershipValidator := group.NewMembershipValidator(
		executorLogger,
		wallet.signingGroupOperators,
		n.chain.Signing(),
	)

	err = broadcastChannel.SetFilter(membershipValidator.IsInGroup)
	if err != nil {
		return nil, false, fmt.Errorf(
			"could not set filter for channel [%v]: [%v]",
			broadcastChannel.Name(),
			err,
		)
	}

	// The coordination executor does not need access to signers' key material.
	// It is enough to pass only their member indexes.
	membersIndexes := make([]group.MemberIndex, len(signers))
	for i, s := range signers {
		membersIndexes[i] = s.signingGroupMemberIndex
	}

	operatorAddress, err := n.operatorAddress()
	if err != nil {
		return nil, false, fmt.Errorf("failed to get operator address: [%v]", err)
	}

	executor := newCoordinationExecutor(
		n.chain,
		wallet,
		membersIndexes,
		operatorAddress,
		n.proposalGenerator,
		broadcastChannel,
		membershipValidator,
		n.protocolLatch,
		n.waitForBlockHeight,
	)
	executor.suppressHeartbeat = signingMaterialUsesSchnorrSignatures(
		signers[0].signingMaterial(),
	)

	// Wire metrics recorder if available
	if n.performanceMetrics != nil {
		executor.setMetricsRecorder(n.performanceMetrics)
	}

	n.coordinationExecutors[executorKey] = executor

	executorLogger.Infof(
		"coordination executor created; controlling [%v] signers",
		len(signers),
	)

	return executor, true, nil
}

// getInactivityClaimExecutor gets the inactivity claim executor responsible for
// executing inactivity claim signing and submission related to a specific
// wallet whose part is controlled by this node. The second boolean return value
// indicates whether the node controls at least one signer for the given wallet.
func (n *node) getInactivityClaimExecutor(
	walletPublicKey *ecdsa.PublicKey,
) (*inactivityClaimExecutor, bool, error) {
	n.inactivityClaimExecutorMutex.Lock()
	defer n.inactivityClaimExecutorMutex.Unlock()

	walletPublicKeyBytes, err := marshalPublicKey(walletPublicKey)
	if err != nil {
		return nil, false, fmt.Errorf("cannot marshal wallet public key: [%v]", err)
	}

	executorKey := hex.EncodeToString(walletPublicKeyBytes)

	if executor, exists := n.inactivityClaimExecutors[executorKey]; exists {
		return executor, true, nil
	}

	executorLogger := logger.With(
		zap.String("wallet", fmt.Sprintf("0x%x", walletPublicKeyBytes)),
	)

	signers := n.walletRegistry.getSigners(walletPublicKey)
	if len(signers) == 0 {
		// This is not an error because the node simply does not control
		// the given wallet.
		return nil, false, nil
	}

	// All signers belong to one wallet. Take that wallet from the first signer.
	wallet := signers[0].wallet

	channelName := fmt.Sprintf(
		"%s-%s-inactivity",
		ProtocolName,
		hex.EncodeToString(walletPublicKeyBytes),
	)

	broadcastChannel, err := n.netProvider.BroadcastChannelFor(channelName)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get broadcast channel: [%v]", err)
	}

	inactivity.RegisterUnmarshallers(broadcastChannel)

	membershipValidator := group.NewMembershipValidator(
		executorLogger,
		wallet.signingGroupOperators,
		n.chain.Signing(),
	)

	err = broadcastChannel.SetFilter(membershipValidator.IsInGroup)
	if err != nil {
		return nil, false, fmt.Errorf(
			"could not set filter for channel [%v]: [%v]",
			broadcastChannel.Name(),
			err,
		)
	}

	executorLogger.Infof(
		"inactivity executor created; controlling [%v] signers",
		len(signers),
	)

	walletGroupParameters, err := n.groupParametersForSigners(signers)
	if err != nil {
		return nil, false, err
	}

	executor := newInactivityClaimExecutor(
		n.chain,
		signers,
		broadcastChannel,
		membershipValidator,
		walletGroupParameters,
		n.protocolLatch,
		n.waitForBlockHeight,
	)

	n.inactivityClaimExecutors[executorKey] = executor

	return executor, true, nil
}

// groupParametersForSigners selects parameters from the registry that created
// the wallet. Native Schnorr signer material denotes a FROST wallet; legacy
// material denotes an ECDSA wallet. All signers in this slice belong to the
// same wallet, so the first signer is sufficient to identify the scheme.
func (n *node) groupParametersForSigners(
	signers []*signer,
) (*GroupParameters, error) {
	if len(signers) == 0 {
		return nil, fmt.Errorf("cannot select group parameters without signers")
	}

	if signingMaterialUsesSchnorrSignatures(signers[0].signingMaterial()) {
		if n.frostGroupParameters == nil {
			return nil, fmt.Errorf("FROST group parameters are not configured")
		}

		return n.frostGroupParameters, nil
	}

	if n.groupParameters == nil {
		return nil, fmt.Errorf("ECDSA group parameters are not configured")
	}

	return n.groupParameters, nil
}

// heartbeatMinimumActiveMembers selects the activity threshold for the wallet
// scheme handled by the signing executor. Legacy ECDSA heartbeats retain their
// historical fixed threshold while FROST heartbeats use the independent
// activeThreshold configured on FrostDkgValidator. The validator configuration
// may change after a wallet is created, making the current active threshold
// larger than the persisted wallet's signing group, so cap the threshold at
// that wallet's actual member count.
func heartbeatMinimumActiveMembers(
	signingExecutor *signingExecutor,
	signingGroupMembersCount int,
) int {
	if signingExecutor.usesSchnorrSignatures() {
		minimumActiveMembers := signingExecutor.groupParameters.GroupQuorum
		if signingGroupMembersCount < minimumActiveMembers {
			return signingGroupMembersCount
		}

		return minimumActiveMembers
	}

	return heartbeatSigningMinimumActiveMembers
}

// handleHeartbeatProposal handles an incoming heartbeat proposal by
// orchestrating and dispatching an appropriate wallet action.
func (n *node) handleHeartbeatProposal(
	wallet wallet,
	proposal *HeartbeatProposal,
	startBlock uint64,
	expiryBlock uint64,
) {
	walletPublicKeyBytes, err := marshalPublicKey(wallet.publicKey)
	if err != nil {
		logger.Errorf("cannot marshal wallet public key: [%v]", err)
		return
	}
	signers := n.walletRegistry.getSigners(wallet.publicKey)
	if len(signers) > 0 &&
		signingMaterialUsesSchnorrSignatures(signers[0].signingMaterial()) {
		logger.Infof(
			"ignoring heartbeat request for transaction-only FROST wallet [0x%x]",
			walletPublicKeyBytes,
		)
		return
	}

	signingExecutor, ok, err := n.getSigningExecutor(wallet.publicKey)
	if err != nil {
		logger.Errorf("cannot get signing executor: [%v]", err)
		return
	}
	// This check is actually redundant. We know the node controls some
	// wallet signers as we just got the wallet from the registry using their
	// public key hash. However, we are doing it just in case. The API
	// contract of getSigningExecutor may change one day.
	if !ok {
		logger.Infof(
			"node does not control signers of wallet [0x%x]; "+
				"ignoring the received heartbeat request",
			walletPublicKeyBytes,
		)
		return
	}

	inactivityClaimExecutor, ok, err := n.getInactivityClaimExecutor(wallet.publicKey)
	if err != nil {
		logger.Errorf("cannot get inactivity claim executor: [%v]", err)
		return
	}
	// This check is actually redundant. We know the node controls some
	// wallet signers as we just got the wallet from the registry using their
	// public key hash. However, we are doing it just in case. The API
	// contract of getInactivityClaimExecutor may change one day.
	if !ok {
		logger.Infof(
			"node does not control signers of wallet [0x%x]; "+
				"ignoring the received heartbeat request",
			walletPublicKeyBytes,
		)
		return
	}

	logger.Infof(
		"starting orchestration of the heartbeat action for wallet [0x%x]; "+
			"20-byte public key hash of that wallet is [0x%x]",
		walletPublicKeyBytes,
		bitcoin.PublicKeyHash(wallet.publicKey),
	)

	walletActionLogger := logger.With(
		zap.String("wallet", fmt.Sprintf("0x%x", walletPublicKeyBytes)),
		zap.String("action", ActionHeartbeat.String()),
		zap.Uint64("startBlock", startBlock),
		zap.Uint64("expiryBlock", expiryBlock),
	)
	walletActionLogger.Infof("dispatching wallet action")

	action := newHeartbeatAction(
		walletActionLogger,
		n.chain,
		wallet,
		signingExecutor,
		heartbeatMinimumActiveMembers(
			signingExecutor,
			len(wallet.signingGroupOperators),
		),
		proposal,
		n.heartbeatFailureCounter,
		inactivityClaimExecutor,
		startBlock,
		expiryBlock,
		n.waitForBlockHeight,
	)

	err = n.walletDispatcher.dispatch(action)
	if err != nil {
		walletActionLogger.Errorf("cannot dispatch wallet action: [%v]", err)
		return
	}

	walletActionLogger.Infof("wallet action dispatched successfully")
}

// handleDepositSweepProposal handles an incoming deposit sweep proposal by
// orchestrating and dispatching an appropriate wallet action.
func (n *node) handleDepositSweepProposal(
	wallet wallet,
	proposal *DepositSweepProposal,
	startBlock uint64,
	expiryBlock uint64,
) {
	walletPublicKeyBytes, err := marshalPublicKey(wallet.publicKey)
	if err != nil {
		logger.Errorf("cannot marshal wallet public key: [%v]", err)
		return
	}

	signingExecutor, ok, err := n.getSigningExecutor(wallet.publicKey)
	if err != nil {
		logger.Errorf("cannot get signing executor: [%v]", err)
		return
	}
	// This check is actually redundant. We know the node controls some
	// wallet signers as we just got the wallet from the registry using their
	// public key hash. However, we are doing it just in case. The API
	// contract of getSigningExecutor may change one day.
	if !ok {
		logger.Infof(
			"node does not control signers of wallet [0x%x]; "+
				"ignoring the received deposit sweep proposal",
			walletPublicKeyBytes,
		)
		return
	}

	logger.Infof(
		"starting orchestration of the deposit sweep action for wallet [0x%x]; "+
			"20-byte public key hash of that wallet is [0x%x]",
		walletPublicKeyBytes,
		bitcoin.PublicKeyHash(wallet.publicKey),
	)

	walletActionLogger := logger.With(
		zap.String("wallet", fmt.Sprintf("0x%x", walletPublicKeyBytes)),
		zap.String("action", ActionDepositSweep.String()),
		zap.Uint64("startBlock", startBlock),
		zap.Uint64("expiryBlock", expiryBlock),
	)
	walletActionLogger.Infof("dispatching wallet action")

	action := newDepositSweepAction(
		walletActionLogger,
		n.chain,
		n.btcChain,
		wallet,
		signingExecutor,
		proposal,
		startBlock,
		expiryBlock,
		n.waitForBlockHeight,
	)

	// Wire metrics recorder if available
	if n.performanceMetrics != nil {
		action.setMetricsRecorder(n.performanceMetrics)
	}

	err = n.walletDispatcher.dispatch(action)
	if err != nil {
		walletActionLogger.Errorf("cannot dispatch wallet action: [%v]", err)
		return
	}

	walletActionLogger.Infof("wallet action dispatched successfully")
}

// handleRedemptionProposal handles an incoming redemption proposal by
// orchestrating and dispatching an appropriate wallet action.
func (n *node) handleRedemptionProposal(
	wallet wallet,
	proposal *RedemptionProposal,
	startBlock uint64,
	expiryBlock uint64,
) {
	walletPublicKeyBytes, err := marshalPublicKey(wallet.publicKey)
	if err != nil {
		logger.Errorf("cannot marshal wallet public key: [%v]", err)
		return
	}

	signingExecutor, ok, err := n.getSigningExecutor(wallet.publicKey)
	if err != nil {
		logger.Errorf("cannot get signing executor: [%v]", err)
		return
	}
	// This check is actually redundant. We know the node controls some
	// wallet signers as we just got the wallet from the registry using their
	// public key hash. However, we are doing it just in case. The API
	// contract of getSigningExecutor may change one day.
	if !ok {
		logger.Infof(
			"node does not control signers of wallet [0x%x]; "+
				"ignoring the received redemption proposal",
			walletPublicKeyBytes,
		)
		return
	}

	logger.Infof(
		"starting orchestration of the redemption action for wallet [0x%x]; "+
			"20-byte public key hash of that wallet is [0x%x]",
		walletPublicKeyBytes,
		bitcoin.PublicKeyHash(wallet.publicKey),
	)

	walletActionLogger := logger.With(
		zap.String("wallet", fmt.Sprintf("0x%x", walletPublicKeyBytes)),
		zap.String("action", ActionRedemption.String()),
		zap.Uint64("startBlock", startBlock),
		zap.Uint64("expiryBlock", expiryBlock),
	)
	walletActionLogger.Infof("dispatching wallet action")

	action := newRedemptionAction(
		walletActionLogger,
		n.chain,
		n.btcChain,
		wallet,
		signingExecutor,
		proposal,
		startBlock,
		expiryBlock,
		n.waitForBlockHeight,
	)

	// Wire metrics recorder if available
	if n.performanceMetrics != nil {
		action.setMetricsRecorder(n.performanceMetrics)
	}

	err = n.walletDispatcher.dispatch(action)
	if err != nil {
		walletActionLogger.Errorf("cannot dispatch wallet action: [%v]", err)
		return
	}

	walletActionLogger.Infof("wallet action dispatched successfully")
}

// handleMovingFundsProposal handles an incoming moving funds proposal by
// orchestrating and dispatching an appropriate wallet action.
func (n *node) handleMovingFundsProposal(
	wallet wallet,
	proposal *MovingFundsProposal,
	startBlock uint64,
	expiryBlock uint64,
) {
	walletPublicKeyBytes, err := marshalPublicKey(wallet.publicKey)
	if err != nil {
		logger.Errorf("cannot marshal wallet public key: [%v]", err)
		return
	}

	signingExecutor, ok, err := n.getSigningExecutor(wallet.publicKey)
	if err != nil {
		logger.Errorf("cannot get signing executor: [%v]", err)
		return
	}
	// This check is actually redundant. We know the node controls some
	// wallet signers as we just got the wallet from the registry using their
	// public key hash. However, we are doing it just in case. The API
	// contract of getSigningExecutor may change one day.
	if !ok {
		logger.Infof(
			"node does not control signers of wallet PKH [0x%x]; "+
				"ignoring the received moving funds proposal",
			walletPublicKeyBytes,
		)
		return
	}

	logger.Infof(
		"starting orchestration of the moving funds action for wallet [0x%x]; "+
			"20-byte public key hash of that wallet is [0x%x]",
		walletPublicKeyBytes,
		bitcoin.PublicKeyHash(wallet.publicKey),
	)

	walletActionLogger := logger.With(
		zap.String("wallet", fmt.Sprintf("0x%x", walletPublicKeyBytes)),
		zap.String("action", ActionMovingFunds.String()),
		zap.Uint64("startBlock", startBlock),
		zap.Uint64("expiryBlock", expiryBlock),
	)
	walletActionLogger.Infof("dispatching wallet action")

	action := newMovingFundsAction(
		walletActionLogger,
		n.chain,
		n.btcChain,
		wallet,
		signingExecutor,
		proposal,
		startBlock,
		expiryBlock,
		n.waitForBlockHeight,
	)

	err = n.walletDispatcher.dispatch(action)
	if err != nil {
		walletActionLogger.Errorf("cannot dispatch wallet action: [%v]", err)
		return
	}

	walletActionLogger.Infof("wallet action dispatched successfully")
}

// handleMovedFundsSweepProposal handles an incoming moved funds sweep proposal
// by orchestrating and dispatching an appropriate wallet action.
func (n *node) handleMovedFundsSweepProposal(
	wallet wallet,
	proposal *MovedFundsSweepProposal,
	startBlock uint64,
	expiryBlock uint64,
) {
	walletPublicKeyBytes, err := marshalPublicKey(wallet.publicKey)
	if err != nil {
		logger.Errorf("cannot marshal wallet public key: [%v]", err)
		return
	}

	signingExecutor, ok, err := n.getSigningExecutor(wallet.publicKey)
	if err != nil {
		logger.Errorf("cannot get signing executor: [%v]", err)
		return
	}
	// This check is actually redundant. We know the node controls some
	// wallet signers as we just got the wallet from the registry using their
	// public key hash. However, we are doing it just in case. The API
	// contract of getSigningExecutor may change one day.
	if !ok {
		logger.Infof(
			"node does not control signers of wallet PKH [0x%x]; "+
				"ignoring the received moved funds sweep proposal",
			walletPublicKeyBytes,
		)
		return
	}

	logger.Infof(
		"starting orchestration of the moved funds sweep action for wallet "+
			"[0x%x]; 20-byte public key hash of that wallet is [0x%x]",
		walletPublicKeyBytes,
		bitcoin.PublicKeyHash(wallet.publicKey),
	)

	walletActionLogger := logger.With(
		zap.String("wallet", fmt.Sprintf("0x%x", walletPublicKeyBytes)),
		zap.String("action", ActionMovedFundsSweep.String()),
		zap.Uint64("startBlock", startBlock),
		zap.Uint64("expiryBlock", expiryBlock),
	)
	walletActionLogger.Infof("dispatching wallet action")

	action := newMovedFundsSweepAction(
		walletActionLogger,
		n.chain,
		n.btcChain,
		wallet,
		signingExecutor,
		proposal,
		startBlock,
		expiryBlock,
		n.waitForBlockHeight,
	)

	err = n.walletDispatcher.dispatch(action)
	if err != nil {
		walletActionLogger.Errorf("cannot dispatch wallet action: [%v]", err)
		return
	}

	walletActionLogger.Infof("wallet action dispatched successfully")
}

// coordinationLayerSettings represents settings for the coordination layer.
type coordinationLayerSettings struct {
	// executeCoordinationProcedureFn is a function executing the coordination
	// procedure for the given wallet and coordination window.
	executeCoordinationProcedureFn func(
		node *node,
		window *coordinationWindow,
		walletPublicKey *ecdsa.PublicKey,
	) (*coordinationResult, bool)

	// processCoordinationResultFn is a function processing the given
	// coordination result.
	processCoordinationResultFn func(
		node *node,
		result *coordinationResult,
	)
}

// runCoordinationLayer starts the coordination layer of the node. It is
// responsible for detecting new coordination windows, running coordination
// procedures for all wallets controlled by the node, and processing
// coordination results.
func (n *node) runCoordinationLayer(
	ctx context.Context,
	settings ...*coordinationLayerSettings,
) error {
	// Resolve settings for the coordination layer.
	var cls *coordinationLayerSettings
	switch len(settings) {
	case 1:
		cls = settings[0]
	default:
		cls = &coordinationLayerSettings{
			executeCoordinationProcedureFn: executeCoordinationProcedure,
			processCoordinationResultFn:    processCoordinationResult,
		}
	}

	blockCounter, err := n.chain.BlockCounter()
	if err != nil {
		return fmt.Errorf("cannot get block counter: [%w]", err)
	}

	coordinationResultChan := make(chan *coordinationResult)

	// Track the previous window to record its end when a new one starts
	// Use a mutex to safely access from multiple goroutines
	var previousWindowMu sync.Mutex
	var previousWindow *coordinationWindow

	// Prepare a callback function that will be called every time a new
	// coordination window is detected.
	onWindowFn := func(window *coordinationWindow) {
		previousWindowMu.Lock()
		// Record end of previous window if it exists
		if previousWindow != nil && n.windowMetricsTracker != nil {
			n.windowMetricsTracker.recordWindowEnd(previousWindow)
		}
		previousWindowMu.Unlock()

		// Track coordination window detection
		if n.performanceMetrics != nil {
			n.performanceMetrics.IncrementCounter(clientinfo.MetricCoordinationWindowsDetectedTotal, 1)
		}

		// Record window start in detailed metrics tracker
		if n.windowMetricsTracker != nil {
			n.windowMetricsTracker.recordWindowStart(window)
		}

		previousWindowMu.Lock()
		previousWindow = window
		previousWindowMu.Unlock()

		// Fetch all wallets controlled by the node. It is important to
		// get the wallets every time the window is triggered as the
		// node may have started controlling a new wallet in the meantime.
		walletsPublicKeys := n.walletRegistry.getWalletsPublicKeys()

		for _, currentWalletPublicKey := range walletsPublicKeys {
			// Run an independent coordination procedure for the given wallet
			// in a separate goroutine. The coordination result will be sent
			// to the coordination result channel.
			go func(walletPublicKey *ecdsa.PublicKey) {
				result, ok := cls.executeCoordinationProcedureFn(
					n,
					window,
					walletPublicKey,
				)
				if ok {
					coordinationResultChan <- result
				}
			}(currentWalletPublicKey)
		}
	}

	// Start the coordination windows watcher.
	go watchCoordinationWindows(
		ctx,
		blockCounter.WatchBlocks,
		onWindowFn,
	)

	// Start the coordination result processor.
	go func() {
		for {
			select {
			case result := <-coordinationResultChan:
				go cls.processCoordinationResultFn(n, result)
			case <-ctx.Done():
				return
			}
		}
	}()

	// Start a cleanup goroutine to record the end time of the last window on shutdown
	go func() {
		<-ctx.Done()
		// Record end time for the active window if it exists and hasn't been ended yet
		previousWindowMu.Lock()
		if previousWindow != nil && n.windowMetricsTracker != nil {
			n.windowMetricsTracker.recordWindowEnd(previousWindow)
		}
		previousWindowMu.Unlock()
	}()

	return nil
}

// executeCoordinationProcedure executes the coordination procedure for the
// given wallet and coordination window.
func executeCoordinationProcedure(
	node *node,
	window *coordinationWindow,
	walletPublicKey *ecdsa.PublicKey,
) (*coordinationResult, bool) {
	walletPublicKeyBytes, err := marshalPublicKey(walletPublicKey)
	if err != nil {
		logger.Errorf("cannot marshal wallet public key: [%v]", err)
		return nil, false
	}

	procedureLogger := logger.With(
		zap.Uint64("coordinationBlock", window.coordinationBlock),
		zap.String("wallet", fmt.Sprintf("0x%x", walletPublicKeyBytes)),
	)

	procedureLogger.Infof("starting coordination procedure")

	executor, ok, err := node.getCoordinationExecutor(walletPublicKey)
	if err != nil {
		procedureLogger.Errorf("cannot get coordination executor: [%v]", err)
		return nil, false
	}
	// This check is actually redundant. We know the node controls some
	// wallet signers as we just got the wallet from the registry.
	// However, we are doing it just in case. The API contract of
	// getWalletsPublicKeys and/or getCoordinationExecutor may change one day.
	if !ok {
		procedureLogger.Infof("node does not control signers of this wallet")
		return nil, false
	}

	startTime := time.Now()
	result, err := executor.coordinate(window)
	duration := time.Since(startTime)

	if err != nil {
		procedureLogger.Errorf("coordination procedure failed: [%v]", err)
		// Metrics are already recorded in executor.coordinate() for failures

		// Record window metrics for failed coordination
		if node.windowMetricsTracker != nil {
			walletPublicKeyHash := bitcoin.PublicKeyHash(walletPublicKey)
			// Extract leader and faults from partial result if available
			// (e.g., when follower routine fails, we know who the leader was)
			leader := chain.Address("")
			var faults []*coordinationFault
			if result != nil {
				leader = result.leader
				faults = result.faults
			}
			node.windowMetricsTracker.recordWalletCoordination(
				window,
				walletPublicKeyHash,
				leader,
				"",
				false,
				duration,
				faults,
				err, // capture the error message
			)
		}
		return nil, false
	}

	procedureLogger.Infof(
		"coordination procedure finished successfully with result [%s]",
		result,
	)

	// Metrics are already recorded in executor.coordinate() for successful executions

	// Record window metrics for successful coordination
	if node.windowMetricsTracker != nil {
		walletPublicKeyHash := bitcoin.PublicKeyHash(walletPublicKey)
		actionType := ""
		if result.proposal != nil {
			actionType = result.proposal.ActionType().String()
		}
		node.windowMetricsTracker.recordWalletCoordination(
			window,
			walletPublicKeyHash,
			result.leader,
			actionType,
			true,
			duration,
			result.faults,
			nil, // no error on success
		)
	}

	return result, true
}

// processCoordinationResult processes the given coordination result.
func processCoordinationResult(node *node, result *coordinationResult) {
	logger.Infof("processing coordination result [%s]", result)

	// TODO: In the future, create coordination faults cache and
	//       record faults from the processed results there.

	proposedAction := result.proposal.ActionType()

	if proposedAction == ActionNoop {
		// No-op proposal cannot be processed so return early to avoid
		// panicking on the ValidityBlocks call.
		return
	}

	startBlock := result.window.endBlock()
	expiryBlock := startBlock + result.proposal.ValidityBlocks()

	switch proposedAction {
	case ActionHeartbeat:
		if proposal, ok := result.proposal.(*HeartbeatProposal); ok {
			node.handleHeartbeatProposal(
				result.wallet,
				proposal,
				startBlock,
				expiryBlock,
			)
		}
	case ActionDepositSweep:
		if proposal, ok := result.proposal.(*DepositSweepProposal); ok {
			node.handleDepositSweepProposal(
				result.wallet,
				proposal,
				startBlock,
				expiryBlock,
			)
		}
	case ActionRedemption:
		if proposal, ok := result.proposal.(*RedemptionProposal); ok {
			node.handleRedemptionProposal(
				result.wallet,
				proposal,
				startBlock,
				expiryBlock,
			)
		}
	case ActionMovingFunds:
		if proposal, ok := result.proposal.(*MovingFundsProposal); ok {
			node.handleMovingFundsProposal(
				result.wallet,
				proposal,
				startBlock,
				expiryBlock,
			)
		}
	case ActionMovedFundsSweep:
		if proposal, ok := result.proposal.(*MovedFundsSweepProposal); ok {
			node.handleMovedFundsSweepProposal(
				result.wallet,
				proposal,
				startBlock,
				expiryBlock,
			)
		}
	default:
		logger.Errorf("no handler for coordination result [%s]", result)
	}
}

// archiveClosedWallets archives closed or terminated wallets.
func (n *node) archiveClosedWallets() error {
	// Get all the wallets controlled by the node.
	walletPublicKeys := n.walletRegistry.getWalletsPublicKeys()

	for _, walletPublicKey := range walletPublicKeys {
		walletPublicKeyHash := bitcoin.PublicKeyHash(walletPublicKey)

		var walletID [32]byte
		var archiveWallet bool

		walletChainData, err := n.chain.GetWallet(walletPublicKeyHash)
		if err != nil {
			var found bool
			walletID, found = n.walletRegistry.
				getWalletIDByPublicKeyHash(walletPublicKeyHash)
			if !found {
				return fmt.Errorf(
					"could not resolve local wallet ID for public key hash [0x%x]",
					walletPublicKeyHash,
				)
			}

			// Legacy fallback for deployments where Bridge wallet state is
			// unavailable. FROST wallets are registered in the Bridge but not
			// in the legacy ECDSA wallet registry.
			isRegistered, err := n.chain.IsWalletRegistered(walletID)
			if err != nil {
				return fmt.Errorf(
					"could not check if wallet is registered for wallet with ECDSA ID "+
						"[0x%x]: [%v]",
					walletID,
					err,
				)
			}

			if !isRegistered && n.frostWalletRegistryAvailable() {
				isFrostWallet, err := n.walletRegistry.
					isFrostWalletByPublicKeyHash(walletPublicKeyHash)
				if err != nil {
					return fmt.Errorf(
						"could not classify local wallet with public key hash "+
							"[0x%x]: [%v]",
						walletPublicKeyHash,
						err,
					)
				}
				if isFrostWallet {
					frostChain, ok := n.chain.(frostWalletRegistrationChain)
					if !ok {
						return fmt.Errorf(
							"FROST wallet registration is available but the chain " +
								"does not expose registration checks",
						)
					}
					isFrostRegistered, err :=
						frostChain.IsFrostWalletRegistered(walletID)
					if err != nil {
						return fmt.Errorf(
							"could not check FROST registration for wallet with ID "+
								"[0x%x]: [%v]",
							walletID,
							err,
						)
					}
					if isFrostRegistered {
						continue
					}
					logger.Infof(
						"FROST wallet with ID [0x%x] and public key hash [0x%x] "+
							"was not found in Bridge or the FROST registry; "+
							"deferring its material to anchored retained-history "+
							"reconciliation so a valid pending DKG can be "+
							"distinguished from an orphan",
						walletID,
						walletPublicKeyHash,
					)
					continue
				}
			}

			archiveWallet = !isRegistered
		} else {
			walletID = walletChainData.WalletID
			if walletID == [32]byte{} {
				walletID = DeriveLegacyWalletID(walletPublicKeyHash)
			}

			archiveWallet = walletChainData.State == StateClosed ||
				walletChainData.State == StateTerminated
		}

		if archiveWallet {
			// If the wallet is no longer registered it means the wallet has
			// been closed or terminated.
			err := n.walletRegistry.archiveWallet(walletPublicKeyHash)
			if err != nil {
				return fmt.Errorf(
					"could not archive wallet with public key hash [0x%x]: [%v]",
					walletPublicKeyHash,
					err,
				)
			}

			logger.Infof(
				"successfully archived wallet with ID [0x%x] and public key "+
					"hash [0x%x]",
				walletID,
				walletPublicKeyHash,
			)
		}
	}

	return nil
}

type frostWalletRegistryAvailability interface {
	FrostWalletRegistryAvailable() bool
}

type frostWalletRegistrationChain interface {
	IsFrostWalletRegistered(walletID [32]byte) (bool, error)
}

func (n *node) frostWalletRegistryAvailable() bool {
	frostChain, ok := n.chain.(frostWalletRegistryAvailability)

	return ok && frostChain.FrostWalletRegistryAvailable()
}

// handleWalletClosure handles the wallet termination or closing process.
func (n *node) handleWalletClosure(
	walletID [32]byte,
	walletScheme WalletScheme,
) error {
	walletScheme = normalizeWalletScheme(walletScheme)

	blockCounter, err := n.chain.BlockCounter()
	if err != nil {
		return fmt.Errorf("error getting block counter [%w]", err)
	}

	currentBlock, err := blockCounter.CurrentBlock()
	if err != nil {
		return fmt.Errorf("error getting current block [%w]", err)
	}

	// To verify there was no chain reorg and the wallet is really closed, check
	// registration in the registry that emitted the closure event.
	stateCheck := func() (bool, error) {
		var isRegistered bool
		var err error

		switch walletScheme {
		case WalletSchemeECDSA:
			isRegistered, err = n.chain.IsWalletRegistered(walletID)
		case WalletSchemeFROST:
			frostChain, ok := n.chain.(frostWalletRegistrationChain)
			if !ok {
				return false, fmt.Errorf(
					"chain does not support FROST wallet registration checks",
				)
			}

			isRegistered, err = frostChain.IsFrostWalletRegistered(walletID)
		default:
			return false, fmt.Errorf("unsupported wallet scheme [%v]", walletScheme)
		}
		if err != nil {
			return false, err
		}

		return !isRegistered, nil
	}

	// Wait a significant number of blocks to make sure the transaction has not
	// been reverted for some reason, e.g. due to a chain reorganization.
	result, err := ethereum.WaitForBlockConfirmations(
		blockCounter,
		currentBlock,
		walletClosureConfirmationBlocks,
		stateCheck,
	)
	if err != nil {
		return fmt.Errorf(
			"error while waiting for wallet closure confirmation [%w]",
			err,
		)
	}

	if !result {
		return fmt.Errorf("wallet closure not confirmed")
	}

	walletPublicKeyHash, err := n.chain.WalletPublicKeyHashForWalletID(walletID)
	if err != nil {
		// Legacy WalletClosed events carry ECDSA registry IDs rather than
		// canonical Bridge IDs, and canonical resolution can therefore miss.
		// The local registry fallback also protects custom chain adapters that
		// cannot resolve a FROST ID through the Bridge.
		logger.Debugf(
			"cannot resolve wallet public key hash for wallet ID [0x%x]: [%v]; "+
				"falling back to local wallet ID matching",
			walletID,
			err,
		)

		wallet, ok := n.walletRegistry.getWalletByID(walletID)
		if !ok {
			// Wallet was not found in the registry. The wallet is not controlled
			// by this node.
			logger.Infof(
				"node does not control wallet with ID [0x%x]; quitting wallet "+
					"archiving",
				walletID,
			)
			return nil
		}

		walletPublicKeyHash = bitcoin.PublicKeyHash(wallet.publicKey)
	}

	_, ok := n.walletRegistry.getWalletByPublicKeyHash(walletPublicKeyHash)
	if !ok {
		// Wallet was not found in the registry. The wallet is not controlled by
		// this node.
		logger.Infof(
			"node does not control wallet with ID [0x%x] and public key hash "+
				"[0x%x]; quitting wallet archiving",
			walletID,
			walletPublicKeyHash,
		)
		return nil
	}

	err = n.walletRegistry.archiveWallet(walletPublicKeyHash)
	if err != nil {
		return fmt.Errorf("failed to archive the wallet: [%v]", err)
	}

	logger.Infof(
		"successfully archived wallet with wallet ID [0x%x] and public key "+
			"hash [0x%x]",
		walletID,
		walletPublicKeyHash,
	)

	return nil
}

// waitForBlockFn represents a function blocking the execution until the given
// block height.
type waitForBlockFn func(context.Context, uint64) error

// getCurrentBlockFn represents a function returning the current block height.
type getCurrentBlockFn func() (uint64, error)

// TODO: this should become a part of BlockHeightWaiter interface.
func (n *node) waitForBlockHeight(ctx context.Context, blockHeight uint64) error {
	blockCounter, err := n.chain.BlockCounter()
	if err != nil {
		return err
	}

	wait, err := blockCounter.BlockHeightWaiter(blockHeight)
	if err != nil {
		return err
	}

	select {
	case <-wait:
	case <-ctx.Done():
	}

	return nil
}

// withCancelOnBlock returns a copy of the given ctx that is automatically
// cancelled on the given block or when the parent ctx is done. Note that the
// context can be cancelled earlier if the waitForBlockFn returns an error.
func withCancelOnBlock(
	ctx context.Context,
	block uint64,
	waitForBlockFn waitForBlockFn,
) (context.Context, context.CancelFunc) {
	// #nosec G118 -- The returned cancel function is intentionally propagated
	// to the caller and also invoked by the helper goroutine below.
	blockCtx, cancelBlockCtx := context.WithCancel(ctx)

	go func() {
		defer cancelBlockCtx()

		err := waitForBlockFn(blockCtx, block)
		if err != nil && blockCtx.Err() == nil {
			logger.Errorf(
				"failed to wait for block [%v]; "+
					"context cancelled earlier than expected",
				err,
			)
		}
	}()

	return blockCtx, cancelBlockCtx
}
