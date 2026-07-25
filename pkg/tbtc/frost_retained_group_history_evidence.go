package tbtc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math/big"
	"sort"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	ethabi "github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	frostabi "github.com/keep-network/keep-core/pkg/chain/ethereum/frost/gen/abi"
	bridgeabi "github.com/keep-network/keep-core/pkg/chain/ethereum/tbtc/gen/abi"
	frostregistry "github.com/keep-network/keep-core/pkg/frost/registry"
)

// FrostRetainedGroupActivationEvidenceBinder binds a retained-group history
// source to the exact deployment and descriptor set authenticated by the
// signed activation manifest. Production activation requires this binding
// before the source can authenticate semantic history or operator IDs.
type FrostRetainedGroupActivationEvidenceBinder interface {
	BindFrostRetainedGroupActivationEvidence(
		FrostPreSignActivationProfile,
		FrostPreSignActivationRuntimeManifest,
	) error
}

type FrostRetainedGroupProtocolBindingSource interface {
	FrostRetainedGroupProtocolBindingHash() ([32]byte, error)
}

type frostRetainedGroupEvidenceProfile struct {
	manifestHash                   [32]byte
	profileHash                    [32]byte
	implementationSetHash          [32]byte
	descriptorSetHash              [32]byte
	linkedLibraryDescriptorSetHash [32]byte
	inventoryProtocolID            [32]byte
	quarantineProtocolID           [32]byte
	domainChainID                  [32]byte
	genesisBlockHash               [32]byte
	bindingHash                    [32]byte
	deployments                    map[string]FrostPreSignDeploymentEvidence
	bridgeABI                      ethabi.ABI
	registryABI                    ethabi.ABI
}

type frostRetainedGroupReceiptCache map[common.Hash]*types.Receipt

type frostRetainedGroupCodeCacheKey struct {
	address        common.Address
	blockHash      common.Hash
	codeHash       common.Hash
	descriptorHash common.Hash
	verified       bool
}

type frostRetainedGroupCodeCache map[frostRetainedGroupCodeCacheKey][]byte

var _ FrostRetainedGroupActivationEvidenceBinder = (*signedFrostRetainedGroupHistorySource)(nil)
var _ FrostRetainedGroupProtocolBindingSource = (*signedFrostRetainedGroupHistorySource)(nil)

// BindFrostRetainedGroupActivationEvidence is deliberately one-shot. The
// profile and descriptor are supplied only after the activation envelope has
// been signature-checked and converted to its immutable runtime manifest.
func (source *signedFrostRetainedGroupHistorySource) BindFrostRetainedGroupActivationEvidence(
	profile FrostPreSignActivationProfile,
	runtimeManifest FrostPreSignActivationRuntimeManifest,
) error {
	if source == nil {
		return fmt.Errorf("retained-group history source is nil")
	}
	if err := profile.ValidateForProduction(); err != nil {
		return fmt.Errorf("retained-group activation profile is invalid: [%w]", err)
	}
	if runtimeManifest.ManifestHash == [32]byte{} ||
		runtimeManifest.ProfileHash == [32]byte{} ||
		runtimeManifest.GenesisBlockHash == [32]byte{} ||
		runtimeManifest.ImplementationSetHash == [32]byte{} ||
		runtimeManifest.LinkedLibraryDescriptorSetHash == [32]byte{} ||
		runtimeManifest.EndpointIdentitySetHash == [32]byte{} ||
		runtimeManifest.CanonicalJournal.DescriptorSetHash == [32]byte{} ||
		runtimeManifest.RetainedGroupInventoryProtocolID == [32]byte{} ||
		runtimeManifest.QuarantineJournal.ProtocolID == [32]byte{} ||
		profile.ActivationManifestHash != runtimeManifest.ManifestHash ||
		profile.ProfileHash != runtimeManifest.ProfileHash ||
		profile.ImplementationSetHash != runtimeManifest.ImplementationSetHash ||
		profile.DomainChainID != runtimeManifest.DomainChainID ||
		profile.ReservationProtocolID != runtimeManifest.ReservationProtocolID ||
		profile.SigningPolicyHash != runtimeManifest.SigningPolicyHash ||
		runtimeManifest.SignerProtocolID == [32]byte{} ||
		runtimeManifest.BitcoinOutboxProtocolID == [32]byte{} ||
		runtimeManifest.CanonicalJournal.Checkpoint.BlockNumber == 0 ||
		runtimeManifest.CanonicalJournal.Checkpoint.BlockHash == [32]byte{} ||
		strings.TrimSpace(runtimeManifest.CanonicalJournal.StoreID) == "" ||
		runtimeManifest.CanonicalJournal.StoreFingerprint == [32]byte{} ||
		runtimeManifest.CanonicalJournal.ClusterFingerprint == [32]byte{} ||
		strings.TrimSpace(runtimeManifest.QuarantineJournal.StoreID) == "" ||
		runtimeManifest.QuarantineJournal.StoreFingerprint == [32]byte{} ||
		runtimeManifest.QuarantineJournal.ClusterFingerprint == [32]byte{} ||
		runtimeManifest.CanonicalJournal.SourceTrustDomainID !=
			source.identity.TrustDomainID ||
		runtimeManifest.CanonicalJournal.SourceEndpointFingerprint !=
			source.identity.EndpointFingerprint ||
		runtimeManifest.CanonicalJournal.SourceOperatorFingerprint !=
			source.identity.OperatorFingerprint ||
		new(big.Int).SetBytes(runtimeManifest.DomainChainID[:]).BitLen() > 64 ||
		new(big.Int).SetBytes(runtimeManifest.DomainChainID[:]).Uint64() !=
			source.chainID ||
		ComputeFrostPreSignDeploymentEvidenceHash(runtimeManifest.Deployments) !=
			profile.ImplementationSetHash {
		return fmt.Errorf("retained-group evidence does not match the signed activation manifest")
	}
	deployments, err := validateFrostRetainedGroupDeploymentEvidence(
		runtimeManifest.Deployments,
	)
	if err != nil {
		return err
	}
	for role, expected := range map[string]struct {
		address  [20]byte
		codeHash [32]byte
	}{
		"bridge": {
			address:  profile.BridgeAddress,
			codeHash: profile.BridgeCodeHash,
		},
		"completeRouter": {
			address:  profile.CompleteRouter,
			codeHash: profile.CompleteRouterCodeHash,
		},
		"authorizationRegistry": {
			address:  profile.RegistryAddress,
			codeHash: profile.RegistryCodeHash,
		},
		"frostWalletRegistry": {
			address:  profile.FrostRegistry,
			codeHash: profile.FrostRegistryCodeHash,
		},
		"frostProposalValidator": {
			address:  profile.ProposalValidator,
			codeHash: profile.ProposalValidatorCodeHash,
		},
		"frostSortitionPool": {
			address:  profile.SortitionPool,
			codeHash: profile.SortitionPoolCodeHash,
		},
	} {
		deployment := deployments[role]
		if deployment.Current.Address != expected.address ||
			deployment.Current.RuntimeCodeHash != expected.codeHash {
			return fmt.Errorf(
				"retained-group deployment [%s] differs from the activation profile",
				role,
			)
		}
	}
	linkedLibraryDescriptorSetHash, err :=
		frostRetainedGroupLinkedLibraryDescriptorSetHash(
			runtimeManifest.Deployments,
		)
	if err != nil ||
		linkedLibraryDescriptorSetHash !=
			runtimeManifest.LinkedLibraryDescriptorSetHash {
		return fmt.Errorf(
			"retained-group linked-library descriptor set differs from the signed activation manifest",
		)
	}
	bindingHash, err := source.computeProtocolBinding(
		profile,
		runtimeManifest,
	)
	if err != nil {
		return err
	}
	parsedBridgeABI, err := ethabi.JSON(strings.NewReader(bridgeabi.BridgeMetaData.ABI))
	if err != nil {
		return fmt.Errorf("cannot parse pinned Bridge ABI: [%w]", err)
	}
	parsedRegistryABI, err := ethabi.JSON(strings.NewReader(frostabi.FrostWalletRegistryMetaData.ABI))
	if err != nil {
		return fmt.Errorf("cannot parse pinned FROST registry ABI: [%w]", err)
	}
	evidence := &frostRetainedGroupEvidenceProfile{
		manifestHash:                   runtimeManifest.ManifestHash,
		profileHash:                    runtimeManifest.ProfileHash,
		implementationSetHash:          runtimeManifest.ImplementationSetHash,
		descriptorSetHash:              runtimeManifest.CanonicalJournal.DescriptorSetHash,
		linkedLibraryDescriptorSetHash: runtimeManifest.LinkedLibraryDescriptorSetHash,
		inventoryProtocolID:            runtimeManifest.RetainedGroupInventoryProtocolID,
		quarantineProtocolID:           runtimeManifest.QuarantineJournal.ProtocolID,
		domainChainID:                  runtimeManifest.DomainChainID,
		genesisBlockHash:               runtimeManifest.GenesisBlockHash,
		bindingHash:                    bindingHash,
		deployments:                    deployments,
		bridgeABI:                      parsedBridgeABI,
		registryABI:                    parsedRegistryABI,
	}
	for name, event := range map[string]struct {
		contract *ethabi.ABI
		topic    common.Hash
	}{
		"DkgResultSubmitted":    {&evidence.registryABI, common.HexToHash("0xbfc6cd6291b6741d3ac1631ba81a0288d08265bea4d59d452e8c953e11ec11c6")},
		"DkgResultApproved":     {&evidence.registryABI, common.HexToHash("0xe6e9d5eba171e82025efb3f3d44fd35905e7283d104284cb9f3bbc5bf1e4276f")},
		"WalletCreated":         {&evidence.registryABI, common.HexToHash("0xbe8f27cef1f3d94120c9c547c3614f5b992fdb0c0a497cc920fde06546291ab4")},
		"WalletClosed":          {&evidence.registryABI, common.HexToHash("0xa6ae4af610b8ada39d3675190ead27a5552631a8e33f53e4e37dbb082f11a73e")},
		"NewWalletRegisteredV2": {&evidence.bridgeABI, common.HexToHash("0x6a501a1d441e1c8b5490e52589d0d27d35504cf1063a8c848fef40f326710d4b")},
		"WalletMovingFunds":     {&evidence.bridgeABI, common.HexToHash("0xbdc9ce990a067e5fd3a5d8dfc68e27e9f221aaa3fe55265e0b7e93c460b3efe2")},
		"WalletClosing":         {&evidence.bridgeABI, common.HexToHash("0x68cb496f5e64383745876664ef119840f154a729c03ba866b8aecb5c9f53d516")},
		"BridgeWalletClosed":    {&evidence.bridgeABI, common.HexToHash("0x47b159947c3066cb253f60e8f046cfd747411788a545cb189679e3fa1467b28d")},
		"WalletTerminated":      {&evidence.bridgeABI, common.HexToHash("0x9272a280b0f32f70b00ad0b546499c68e3ecc6f7bb7ef43491ec5d7b99bf69ef")},
	} {
		eventName := name
		if name == "BridgeWalletClosed" {
			eventName = "WalletClosed"
		}
		parsedEvent, ok := event.contract.Events[eventName]
		if !ok || parsedEvent.ID != event.topic {
			return fmt.Errorf("pinned retained-group event descriptor [%s] is unavailable or changed", name)
		}
	}

	source.evidenceMutex.Lock()
	defer source.evidenceMutex.Unlock()
	if source.evidence != nil {
		return fmt.Errorf("retained-group activation evidence is already bound")
	}
	source.evidence = evidence
	return nil
}

func (source *signedFrostRetainedGroupHistorySource) computeProtocolBinding(
	profile FrostPreSignActivationProfile,
	runtimeManifest FrostPreSignActivationRuntimeManifest,
) ([32]byte, error) {
	binding := frostRetainedGroupProtocolBinding{
		Schema:           "tbtc-frost-retained-group-protocol-binding/v2",
		ChainID:          source.chainID,
		DomainChainID:    frostActivationHex32(runtimeManifest.DomainChainID),
		GenesisBlockHash: frostActivationHex32(runtimeManifest.GenesisBlockHash),
		Checkpoint: frostRetainedGroupWireBlockPoint{
			BlockNumber: runtimeManifest.CanonicalJournal.Checkpoint.BlockNumber,
			BlockHash: frostActivationHex32(
				runtimeManifest.CanonicalJournal.Checkpoint.BlockHash,
			),
		},
		ManifestHash:                   frostActivationHex32(runtimeManifest.ManifestHash),
		ProfileHash:                    frostActivationHex32(runtimeManifest.ProfileHash),
		ImplementationSetHash:          frostActivationHex32(runtimeManifest.ImplementationSetHash),
		DescriptorSetHash:              frostActivationHex32(runtimeManifest.CanonicalJournal.DescriptorSetHash),
		LinkedLibraryDescriptorSetHash: frostActivationHex32(runtimeManifest.LinkedLibraryDescriptorSetHash),
		EndpointIdentitySetHash:        frostActivationHex32(runtimeManifest.EndpointIdentitySetHash),
		SignerProtocolID:               frostActivationHex32(runtimeManifest.SignerProtocolID),
		ReservationProtocolID:          frostActivationHex32(runtimeManifest.ReservationProtocolID),
		EvidenceProtocolID:             frostActivationHex32(profile.EvidenceProtocolID),
		BitcoinOutboxProtocolID:        frostActivationHex32(runtimeManifest.BitcoinOutboxProtocolID),
		InventoryProtocolID:            frostActivationHex32(runtimeManifest.RetainedGroupInventoryProtocolID),
		QuarantineProtocolID:           frostActivationHex32(runtimeManifest.QuarantineJournal.ProtocolID),
		SigningPolicyHash:              frostActivationHex32(runtimeManifest.SigningPolicyHash),
		CanonicalStoreID:               runtimeManifest.CanonicalJournal.StoreID,
		CanonicalStoreFingerprint: frostActivationHex32(
			runtimeManifest.CanonicalJournal.StoreFingerprint,
		),
		CanonicalClusterFingerprint: frostActivationHex32(
			runtimeManifest.CanonicalJournal.ClusterFingerprint,
		),
		QuarantineStoreID: runtimeManifest.QuarantineJournal.StoreID,
		QuarantineStoreFingerprint: frostActivationHex32(
			runtimeManifest.QuarantineJournal.StoreFingerprint,
		),
		QuarantineClusterFingerprint: frostActivationHex32(
			runtimeManifest.QuarantineJournal.ClusterFingerprint,
		),
		SourceIdentity: frostRetainedGroupWireIdentity{
			TrustDomainID: source.identity.TrustDomainID,
			EndpointFingerprint: frostActivationHex32(
				source.identity.EndpointFingerprint,
			),
			OperatorFingerprint: frostActivationHex32(
				source.identity.OperatorFingerprint,
			),
		},
	}
	return frostRetainedGroupDomainHash(
		frostRetainedGroupProtocolBindingDomain,
		binding,
	)
}

func validateFrostRetainedGroupDeploymentEvidence(
	deployments []FrostPreSignDeploymentEvidence,
) (map[string]FrostPreSignDeploymentEvidence, error) {
	requiredRoles := map[string]bool{
		"bridge":                  false,
		"completeRouter":          false,
		"authorizationRegistry":   false,
		"frostWalletRegistry":     false,
		"frostProposalValidator":  false,
		"frostSortitionPool":      false,
		"ecdsaFraudRouter":        false,
		"ecdsaCutoverCoordinator": false,
	}
	if len(deployments) != len(requiredRoles) {
		return nil, fmt.Errorf("retained-group deployment evidence is incomplete")
	}
	result := make(map[string]FrostPreSignDeploymentEvidence, len(deployments))
	for _, deployment := range deployments {
		if _, required := requiredRoles[deployment.Role]; !required ||
			requiredRoles[deployment.Role] ||
			strings.TrimSpace(deployment.Name) == "" ||
			deployment.DeploymentBlock == 0 ||
			deployment.RelevantEventStartBlock < deployment.DeploymentBlock ||
			len(deployment.HistoricalEpochs) == 0 ||
			len(deployment.HistoricalEpochs) > 64 {
			return nil, fmt.Errorf("retained-group deployment [%s] is invalid", deployment.Role)
		}
		requiredRoles[deployment.Role] = true
		if err := validateFrostRetainedGroupDeploymentDescriptor(
			deployment.Current,
		); err != nil {
			return nil, fmt.Errorf("invalid current %s deployment descriptor: [%w]", deployment.Role, err)
		}
		for index, epoch := range deployment.HistoricalEpochs {
			if epoch.Start.BlockNumber == 0 || epoch.Start.BlockHash == [32]byte{} ||
				(index == 0 &&
					epoch.Start.BlockNumber != deployment.DeploymentBlock) ||
				(index+1 < len(deployment.HistoricalEpochs) && epoch.End == nil) ||
				(index+1 == len(deployment.HistoricalEpochs) && epoch.End != nil) {
				return nil, fmt.Errorf("retained-group %s epoch [%d] range is invalid", deployment.Role, index)
			}
			if epoch.End != nil &&
				(epoch.End.BlockNumber < epoch.Start.BlockNumber ||
					epoch.End.BlockHash == [32]byte{}) {
				return nil, fmt.Errorf("retained-group %s epoch [%d] end is invalid", deployment.Role, index)
			}
			if index > 0 {
				previous := deployment.HistoricalEpochs[index-1]
				if previous.End == nil ||
					previous.End.BlockNumber == ^uint64(0) ||
					previous.End.BlockNumber+1 != epoch.Start.BlockNumber {
					return nil, fmt.Errorf("retained-group %s epochs have a gap or overlap", deployment.Role)
				}
			}
			if err := validateFrostRetainedGroupDeploymentDescriptor(
				epoch.Descriptor,
			); err != nil {
				return nil, fmt.Errorf("invalid %s epoch [%d] descriptor: [%w]", deployment.Role, index, err)
			}
		}
		if deployment.RelevantEventStartBlock <
			deployment.HistoricalEpochs[0].Start.BlockNumber ||
			deployment.Current.DescriptorHash !=
				deployment.HistoricalEpochs[len(deployment.HistoricalEpochs)-1].Descriptor.DescriptorHash {
			return nil, fmt.Errorf("retained-group %s epochs do not cover the event range", deployment.Role)
		}
		result[deployment.Role] = cloneFrostRetainedGroupDeploymentEvidence(
			deployment,
		)
	}
	return result, nil
}

func validateFrostRetainedGroupDeploymentDescriptor(
	descriptor FrostPreSignDeploymentDescriptorEvidence,
) error {
	if descriptor.Address == [20]byte{} ||
		descriptor.RuntimeCodeHash == [32]byte{} ||
		descriptor.LinkedLibraryDescriptorHash == [32]byte{} ||
		descriptor.DescriptorHash == [32]byte{} ||
		descriptor.ComputeHash() != descriptor.DescriptorHash {
		return fmt.Errorf("deployment descriptor identity or commitment is invalid")
	}
	switch descriptor.Upgradeability {
	case "immutable":
		if descriptor.ImplementationAddress != [20]byte{} ||
			descriptor.ImplementationCodeHash != [32]byte{} ||
			descriptor.AdminAddress != [20]byte{} ||
			descriptor.AdminCodeHash != [32]byte{} ||
			descriptor.ImplementationSlotValue != [32]byte{} ||
			descriptor.AdminSlotValue != [32]byte{} {
			return fmt.Errorf("immutable deployment descriptor contains proxy fields")
		}
	case "eip1967":
		if descriptor.ImplementationAddress == [20]byte{} ||
			descriptor.ImplementationCodeHash == [32]byte{} ||
			descriptor.AdminAddress == [20]byte{} ||
			descriptor.AdminCodeHash == [32]byte{} ||
			descriptor.ImplementationAddress == descriptor.AdminAddress ||
			descriptor.ImplementationAddress == descriptor.Address ||
			descriptor.AdminAddress == descriptor.Address ||
			!frostRetainedGroupSlotValueBindsAddress(
				descriptor.ImplementationSlotValue,
				descriptor.ImplementationAddress,
			) ||
			!frostRetainedGroupSlotValueBindsAddress(
				descriptor.AdminSlotValue,
				descriptor.AdminAddress,
			) {
			return fmt.Errorf("EIP-1967 deployment descriptor is incomplete")
		}
	default:
		return fmt.Errorf("unsupported upgradeability [%s]", descriptor.Upgradeability)
	}
	count := 0
	if err := validateFrostRetainedGroupLinkedLibraries(
		descriptor.LinkedLibraries,
		0,
		&count,
	); err != nil {
		return err
	}
	computedDescriptorHash, err :=
		frostRetainedGroupLinkedLibraryInventoryHash(
			descriptor.LinkedLibraries,
		)
	if err != nil ||
		computedDescriptorHash != descriptor.LinkedLibraryDescriptorHash {
		return fmt.Errorf("linked-library descriptor hash mismatch")
	}
	return nil
}

func validateFrostRetainedGroupLinkedLibraries(
	libraries []FrostPreSignLinkedLibraryEvidence,
	depth int,
	count *int,
) error {
	if count == nil || depth > 16 {
		return fmt.Errorf("linked-library evidence is too deep")
	}
	roles := make(map[string]bool)
	addresses := make(map[[20]byte]bool)
	var previousRole string
	for index, library := range libraries {
		(*count)++
		if *count > 256 ||
			!frostRetainedGroupValidProtocolRole(library.ProtocolRole) ||
			(index > 0 && library.ProtocolRole <= previousRole) ||
			roles[library.ProtocolRole] || addresses[library.Address] ||
			library.Address == [20]byte{} ||
			library.RuntimeCodeHash == [32]byte{} ||
			library.LinkedLibraryDescriptorHash == [32]byte{} ||
			len(library.References) == 0 {
			return fmt.Errorf("linked-library evidence is noncanonical")
		}
		roles[library.ProtocolRole] = true
		addresses[library.Address] = true
		previousRole = library.ProtocolRole
		for referenceIndex, reference := range library.References {
			if reference.Length != 20 ||
				reference.Start > ^uint64(0)-reference.Length ||
				(referenceIndex > 0 &&
					library.References[referenceIndex-1].Start+
						library.References[referenceIndex-1].Length >
						reference.Start) {
				return fmt.Errorf("linked-library references are noncanonical")
			}
		}
		if err := validateFrostRetainedGroupLinkedLibraries(
			library.LinkedLibraries,
			depth+1,
			count,
		); err != nil {
			return err
		}
		computedDescriptorHash, err :=
			frostRetainedGroupLinkedLibraryInventoryHash(
				library.LinkedLibraries,
			)
		if err != nil ||
			computedDescriptorHash != library.LinkedLibraryDescriptorHash {
			return fmt.Errorf(
				"linked-library [%s] descriptor hash mismatch",
				library.ProtocolRole,
			)
		}
	}
	return nil
}

func frostRetainedGroupValidProtocolRole(value string) bool {
	if len(value) == 0 || len(value) > 255 {
		return false
	}
	for _, character := range []byte(value) {
		if (character >= 'A' && character <= 'Z') ||
			(character >= 'a' && character <= 'z') ||
			(character >= '0' && character <= '9') ||
			strings.ContainsRune("._:/-", rune(character)) {
			continue
		}
		return false
	}
	return true
}

type frostRetainedGroupLinkedLibraryReferenceCommitment struct {
	Start  uint64 `json:"start"`
	Length uint64 `json:"length"`
}

type frostRetainedGroupLinkedLibraryDescriptorCommitment struct {
	ProtocolRole    string                                                `json:"protocolRole"`
	References      []frostRetainedGroupLinkedLibraryReferenceCommitment  `json:"references"`
	LinkedLibraries []frostRetainedGroupLinkedLibraryDescriptorCommitment `json:"linkedLibraries"`
}

func frostRetainedGroupLinkedLibraryDescriptors(
	libraries []FrostPreSignLinkedLibraryEvidence,
) []frostRetainedGroupLinkedLibraryDescriptorCommitment {
	result := make(
		[]frostRetainedGroupLinkedLibraryDescriptorCommitment,
		0,
		len(libraries),
	)
	for _, library := range libraries {
		references := make(
			[]frostRetainedGroupLinkedLibraryReferenceCommitment,
			0,
			len(library.References),
		)
		for _, reference := range library.References {
			references = append(
				references,
				frostRetainedGroupLinkedLibraryReferenceCommitment{
					Start:  reference.Start,
					Length: reference.Length,
				},
			)
		}
		result = append(
			result,
			frostRetainedGroupLinkedLibraryDescriptorCommitment{
				ProtocolRole: library.ProtocolRole,
				References:   references,
				LinkedLibraries: frostRetainedGroupLinkedLibraryDescriptors(
					library.LinkedLibraries,
				),
			},
		)
	}
	return result
}

func frostRetainedGroupLinkedLibraryInventoryHash(
	libraries []FrostPreSignLinkedLibraryEvidence,
) ([32]byte, error) {
	canonical, err := canonicalFrostActivationValue(map[string]interface{}{
		"schema": "tbtc-p2tr-linked-library-inventory/v1",
		"linkedLibraries": frostRetainedGroupLinkedLibraryDescriptors(
			libraries,
		),
	})
	if err != nil {
		return [32]byte{}, err
	}
	return sha256.Sum256(canonical), nil
}

func frostRetainedGroupLinkedLibraryDescriptorSetHash(
	deployments []FrostPreSignDeploymentEvidence,
) ([32]byte, error) {
	type epochDescriptor struct {
		StartBlock      uint64                                                `json:"startBlock"`
		EndBlock        *uint64                                               `json:"endBlock"`
		CodeKind        string                                                `json:"codeKind"`
		LinkedLibraries []frostRetainedGroupLinkedLibraryDescriptorCommitment `json:"linkedLibraries"`
	}
	type contractDescriptor struct {
		ContractRole     string                                                `json:"contractRole"`
		CodeKind         string                                                `json:"codeKind"`
		LinkedLibraries  []frostRetainedGroupLinkedLibraryDescriptorCommitment `json:"linkedLibraries"`
		HistoricalEpochs []epochDescriptor                                     `json:"historicalEpochs"`
	}
	contracts := make([]contractDescriptor, 0, len(deployments))
	for _, deployment := range deployments {
		codeKind := "runtime"
		if deployment.Current.Upgradeability == "eip1967" {
			codeKind = "implementation-runtime"
		}
		historicalEpochs := make(
			[]epochDescriptor,
			0,
			len(deployment.HistoricalEpochs),
		)
		for _, epoch := range deployment.HistoricalEpochs {
			epochCodeKind := "runtime"
			if epoch.Descriptor.Upgradeability == "eip1967" {
				epochCodeKind = "implementation-runtime"
			}
			var endBlock *uint64
			if epoch.End != nil {
				value := epoch.End.BlockNumber
				endBlock = &value
			}
			historicalEpochs = append(
				historicalEpochs,
				epochDescriptor{
					StartBlock: epoch.Start.BlockNumber,
					EndBlock:   endBlock,
					CodeKind:   epochCodeKind,
					LinkedLibraries: frostRetainedGroupLinkedLibraryDescriptors(
						epoch.Descriptor.LinkedLibraries,
					),
				},
			)
		}
		contracts = append(
			contracts,
			contractDescriptor{
				ContractRole: deployment.Role,
				CodeKind:     codeKind,
				LinkedLibraries: frostRetainedGroupLinkedLibraryDescriptors(
					deployment.Current.LinkedLibraries,
				),
				HistoricalEpochs: historicalEpochs,
			},
		)
	}
	sort.Slice(contracts, func(i, j int) bool {
		return contracts[i].ContractRole < contracts[j].ContractRole
	})
	canonical, err := canonicalFrostActivationValue(map[string]interface{}{
		"schema":    "tbtc-p2tr-linked-library-descriptor-set/v2",
		"contracts": contracts,
	})
	if err != nil {
		return [32]byte{}, err
	}
	return sha256.Sum256(canonical), nil
}

func frostRetainedGroupSlotValueBindsAddress(
	value [32]byte,
	address [20]byte,
) bool {
	return bytes.Equal(value[:12], make([]byte, 12)) &&
		bytes.Equal(value[12:], address[:])
}

func cloneFrostRetainedGroupDeploymentEvidence(
	deployment FrostPreSignDeploymentEvidence,
) FrostPreSignDeploymentEvidence {
	result := deployment
	result.Current = cloneFrostRetainedGroupDeploymentDescriptor(
		deployment.Current,
	)
	result.HistoricalEpochs = make(
		[]FrostPreSignDeploymentEpochEvidence,
		len(deployment.HistoricalEpochs),
	)
	for index, epoch := range deployment.HistoricalEpochs {
		result.HistoricalEpochs[index] = epoch
		if epoch.End != nil {
			end := *epoch.End
			result.HistoricalEpochs[index].End = &end
		}
		result.HistoricalEpochs[index].Descriptor =
			cloneFrostRetainedGroupDeploymentDescriptor(epoch.Descriptor)
	}
	return result
}

func cloneFrostRetainedGroupDeploymentDescriptor(
	descriptor FrostPreSignDeploymentDescriptorEvidence,
) FrostPreSignDeploymentDescriptorEvidence {
	result := descriptor
	result.LinkedLibraries = cloneFrostRetainedGroupLinkedLibraries(
		descriptor.LinkedLibraries,
	)
	return result
}

func cloneFrostRetainedGroupLinkedLibraries(
	libraries []FrostPreSignLinkedLibraryEvidence,
) []FrostPreSignLinkedLibraryEvidence {
	result := make([]FrostPreSignLinkedLibraryEvidence, len(libraries))
	for index, library := range libraries {
		result[index] = library
		result[index].References = append(
			[]FrostPreSignLinkedLibraryReference{},
			library.References...,
		)
		result[index].LinkedLibraries = cloneFrostRetainedGroupLinkedLibraries(
			library.LinkedLibraries,
		)
	}
	return result
}

func (source *signedFrostRetainedGroupHistorySource) activationEvidence() (
	*frostRetainedGroupEvidenceProfile,
	error,
) {
	if source == nil {
		return nil, fmt.Errorf("retained-group history source is nil")
	}
	source.evidenceMutex.RLock()
	defer source.evidenceMutex.RUnlock()
	if source.evidence == nil {
		return nil, fmt.Errorf("retained-group history source is not bound to the signed activation manifest")
	}
	return source.evidence, nil
}

func (source *signedFrostRetainedGroupHistorySource) FrostRetainedGroupProtocolBindingHash() (
	[32]byte,
	error,
) {
	evidence, err := source.activationEvidence()
	if err != nil {
		return [32]byte{}, err
	}
	return evidence.bindingHash, nil
}

func (source *signedFrostRetainedGroupHistorySource) verifyHistoryEvidence(
	ctx context.Context,
	mutations []FrostRetainedGroupMutation,
	evidence *frostRetainedGroupEvidenceProfile,
) error {
	if evidence == nil {
		return fmt.Errorf("retained-group activation evidence is nil")
	}
	receipts := make(frostRetainedGroupReceiptCache)
	code := make(frostRetainedGroupCodeCache)
	for index, mutation := range mutations {
		if err := source.verifyMutationEvidence(ctx, mutation, evidence, receipts, code); err != nil {
			return fmt.Errorf("mutation [%d] [%s]: [%w]", index, mutation.Kind, err)
		}
	}
	return nil
}

func (source *signedFrostRetainedGroupHistorySource) verifyMutationEvidence(
	ctx context.Context,
	mutation FrostRetainedGroupMutation,
	evidence *frostRetainedGroupEvidenceProfile,
	receipts frostRetainedGroupReceiptCache,
	code frostRetainedGroupCodeCache,
) error {
	// Quarantine, recovery-required, and lift records are signed operational
	// records, not Ethereum events. Their Point is only a canonical finalized
	// ordering anchor and is deliberately never presented as receipt evidence.
	if isFrostRetainedGroupQuarantineMutation(mutation.Kind) {
		return nil
	}

	switch mutation.Kind {
	case FrostRetainedGroupAdmissionMutation:
		return source.verifyAdmissionEvidence(ctx, mutation, evidence, receipts, code)
	case FrostRetainedGroupMovingFundsMutation,
		FrostRetainedGroupClosingMutation,
		FrostRetainedGroupClosedMutation,
		FrostRetainedGroupTerminatedMutation:
		eventName := map[FrostRetainedGroupMutationKind]string{
			FrostRetainedGroupMovingFundsMutation: "WalletMovingFunds",
			FrostRetainedGroupClosingMutation:     "WalletClosing",
			FrostRetainedGroupClosedMutation:      "WalletClosed",
			FrostRetainedGroupTerminatedMutation:  "WalletTerminated",
		}[mutation.Kind]
		log, err := source.authenticatedEventLog(
			ctx,
			mutation.Point,
			evidence.deployments["bridge"],
			evidence.bridgeABI.Events[eventName].ID,
			receipts,
			code,
		)
		if err != nil {
			return err
		}
		if len(log.Topics) != 3 || len(log.Data) != 0 || log.Topics[1] != (common.Hash{}) ||
			log.Topics[2] != frostRetainedGroupBytes20Topic(mutation.WalletPublicKeyHash) {
			return fmt.Errorf("Bridge lifecycle log does not encode the exported FROST wallet")
		}
		return nil
	case FrostRetainedGroupRegistryClosureMutation:
		log, err := source.authenticatedEventLog(
			ctx,
			mutation.Point,
			evidence.deployments["frostWalletRegistry"],
			evidence.registryABI.Events["WalletClosed"].ID,
			receipts,
			code,
		)
		if err != nil {
			return err
		}
		if len(log.Topics) != 2 || len(log.Data) != 0 || log.Topics[1] != common.Hash(mutation.WalletID) {
			return fmt.Errorf("FROST registry closure log does not encode the exported wallet")
		}
		return nil
	default:
		return fmt.Errorf("unsupported retained-group mutation kind [%s]", mutation.Kind)
	}
}

func (source *signedFrostRetainedGroupHistorySource) verifyAdmissionEvidence(
	ctx context.Context,
	mutation FrostRetainedGroupMutation,
	evidence *frostRetainedGroupEvidenceProfile,
	receipts frostRetainedGroupReceiptCache,
	code frostRetainedGroupCodeCache,
) error {
	if mutation.Point != mutation.BridgeRegistrationPoint ||
		compareFrostRetainedGroupEventPoints(mutation.DkgSubmissionPoint, mutation.DkgApprovalPoint) >= 0 ||
		!sameFrostRetainedGroupTransaction(mutation.DkgApprovalPoint, mutation.CreationPoint) ||
		!sameFrostRetainedGroupTransaction(mutation.CreationPoint, mutation.BridgeRegistrationPoint) ||
		mutation.DkgApprovalPoint.LogIndex >= mutation.CreationPoint.LogIndex ||
		mutation.CreationPoint.LogIndex >= mutation.BridgeRegistrationPoint.LogIndex {
		return fmt.Errorf("admission evidence points are not the required DKG/registration sequence")
	}

	submissionLog, err := source.authenticatedEventLog(
		ctx,
		mutation.DkgSubmissionPoint,
		evidence.deployments["frostWalletRegistry"],
		evidence.registryABI.Events["DkgResultSubmitted"].ID,
		receipts,
		code,
	)
	if err != nil {
		return err
	}
	result, resultHash, err := frostRetainedGroupDecodeDkgSubmission(
		submissionLog,
		evidence,
	)
	if err != nil {
		return err
	}
	fullMembers := frostregistry.FullMembers(result.Members)
	misbehaved := frostregistry.MisbehavedMemberIndices(
		result.MisbehavedMembersIndices,
	)
	activeMembers, err := frostregistry.ActiveMembersFromMisbehaved(
		fullMembers,
		misbehaved,
	)
	if err != nil {
		return fmt.Errorf("DKG submission has invalid misbehaved member indices: [%w]", err)
	}
	if len(fullMembers) > 100 {
		return fmt.Errorf("DKG submission exceeds the supported group size")
	}
	for _, operatorID := range fullMembers {
		if operatorID == 0 {
			return fmt.Errorf("DKG submission contains a zero operator ID")
		}
	}
	activeMembersHash, err := frostregistry.ActiveMembersHash(activeMembers)
	if err != nil {
		return fmt.Errorf("cannot hash active DKG members: [%w]", err)
	}
	if resultHash != mutation.DkgResultHash ||
		result.XOnlyOutputKey != mutation.WalletID ||
		result.MembersHash != mutation.RetainedGroupHash ||
		result.MembersHash != activeMembersHash ||
		!frostRetainedGroupEqualOperatorIDs(
			[]uint32(activeMembers),
			mutation.OperatorIDs,
		) {
		return fmt.Errorf("DKG submission does not encode the exported admission")
	}

	approvalLog, err := source.authenticatedEventLog(
		ctx,
		mutation.DkgApprovalPoint,
		evidence.deployments["frostWalletRegistry"],
		evidence.registryABI.Events["DkgResultApproved"].ID,
		receipts,
		code,
	)
	if err != nil {
		return err
	}
	if len(approvalLog.Topics) != 3 || len(approvalLog.Data) != 0 ||
		approvalLog.Topics[1] != common.Hash(mutation.DkgResultHash) ||
		approvalLog.Topics[2] == (common.Hash{}) ||
		!frostRetainedGroupCanonicalAddressTopic(approvalLog.Topics[2]) {
		return fmt.Errorf("DKG approval log does not approve the exported result")
	}

	creationLog, err := source.authenticatedEventLog(
		ctx,
		mutation.CreationPoint,
		evidence.deployments["frostWalletRegistry"],
		evidence.registryABI.Events["WalletCreated"].ID,
		receipts,
		code,
	)
	if err != nil {
		return err
	}
	if len(creationLog.Topics) != 3 || len(creationLog.Data) != 0 ||
		creationLog.Topics[1] != common.Hash(mutation.WalletID) ||
		creationLog.Topics[2] != common.Hash(mutation.DkgResultHash) {
		return fmt.Errorf("FROST wallet-creation log does not encode the exported admission")
	}

	registrationLog, err := source.authenticatedEventLog(
		ctx,
		mutation.BridgeRegistrationPoint,
		evidence.deployments["bridge"],
		evidence.bridgeABI.Events["NewWalletRegisteredV2"].ID,
		receipts,
		code,
	)
	if err != nil {
		return err
	}
	if len(registrationLog.Topics) != 4 || len(registrationLog.Data) != 0 ||
		registrationLog.Topics[1] != common.Hash(mutation.WalletID) ||
		registrationLog.Topics[2] != (common.Hash{}) ||
		registrationLog.Topics[3] != frostRetainedGroupBytes20Topic(mutation.WalletPublicKeyHash) {
		return fmt.Errorf("Bridge registration log does not encode the exported FROST wallet")
	}
	return nil
}

func frostRetainedGroupDecodeDkgSubmission(
	log *types.Log,
	evidence *frostRetainedGroupEvidenceProfile,
) (result frostabi.FrostDkgResult, resultHash [32]byte, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			result = frostabi.FrostDkgResult{}
			resultHash = [32]byte{}
			err = fmt.Errorf("cannot decode DKG submission result")
		}
	}()
	if log == nil || evidence == nil || len(log.Topics) != 3 || len(log.Data) == 0 {
		return result, resultHash, fmt.Errorf("DKG submission log is malformed")
	}
	resultHash = [32]byte(log.Topics[1])
	if resultHash == [32]byte{} || crypto.Keccak256Hash(log.Data) != common.Hash(resultHash) {
		return result, resultHash, fmt.Errorf("DKG result hash does not commit to the submitted result")
	}
	values, unpackErr := evidence.registryABI.Events["DkgResultSubmitted"].Inputs.NonIndexed().Unpack(log.Data)
	if unpackErr != nil || len(values) != 1 {
		return result, resultHash, fmt.Errorf("cannot decode DKG submitted result: [%w]", unpackErr)
	}
	converted := ethabi.ConvertType(values[0], new(frostabi.FrostDkgResult))
	decoded, ok := converted.(*frostabi.FrostDkgResult)
	if !ok || decoded == nil {
		return result, resultHash, fmt.Errorf("cannot convert DKG submitted result")
	}
	return *decoded, resultHash, nil
}

func (source *signedFrostRetainedGroupHistorySource) authenticatedEventLog(
	ctx context.Context,
	point FrostRetainedGroupEventPoint,
	deployment FrostPreSignDeploymentEvidence,
	topic common.Hash,
	receipts frostRetainedGroupReceiptCache,
	code frostRetainedGroupCodeCache,
) (*types.Log, error) {
	if !point.valid() || topic == (common.Hash{}) {
		return nil, fmt.Errorf("event evidence descriptor is incomplete")
	}
	descriptor, err := frostRetainedGroupDeploymentDescriptorAt(
		deployment,
		point.BlockNumber,
		point.BlockHash,
		true,
	)
	if err != nil {
		return nil, err
	}
	if err := source.authenticateContractDeployment(
		ctx,
		descriptor,
		point.BlockNumber,
		point.BlockHash,
		code,
	); err != nil {
		return nil, err
	}
	transactionHash := common.Hash(point.TransactionHash)
	receipt, ok := receipts[transactionHash]
	if !ok {
		if len(receipts) >= frostRetainedGroupMaximumEvidenceReceipts {
			return nil, fmt.Errorf("retained-group evidence exceeds the receipt limit")
		}
		var err error
		requestContext, cancel := source.requestContext(ctx)
		receipt, err = source.verifier.TransactionReceipt(requestContext, transactionHash)
		cancel()
		if err != nil {
			return nil, fmt.Errorf("cannot read transaction receipt [%s]: [%w]", transactionHash.Hex(), err)
		}
		if receipt == nil {
			return nil, fmt.Errorf("transaction receipt [%s] is missing", transactionHash.Hex())
		}
		receipts[transactionHash] = receipt
	}
	if len(receipt.Logs) > frostRetainedGroupMaximumReceiptLogs {
		return nil, fmt.Errorf("transaction receipt exceeds the retained-group log limit")
	}
	if receipt.Status != types.ReceiptStatusSuccessful || receipt.BlockNumber == nil ||
		!receipt.BlockNumber.IsUint64() || receipt.BlockNumber.Uint64() != point.BlockNumber ||
		receipt.BlockHash != common.Hash(point.BlockHash) || receipt.TxHash != transactionHash ||
		receipt.TransactionIndex != uint(point.TransactionIndex) {
		return nil, fmt.Errorf("transaction receipt does not match the exported event point")
	}
	var matched *types.Log
	for _, candidate := range receipt.Logs {
		if candidate == nil || candidate.Index != uint(point.LogIndex) {
			continue
		}
		if matched != nil {
			return nil, fmt.Errorf("transaction receipt contains duplicate global log index")
		}
		matched = candidate
	}
	if matched == nil || matched.Removed ||
		matched.Address != common.Address(descriptor.Address) ||
		matched.BlockNumber != point.BlockNumber || matched.BlockHash != common.Hash(point.BlockHash) ||
		matched.TxHash != transactionHash || matched.TxIndex != uint(point.TransactionIndex) ||
		len(matched.Topics) == 0 || matched.Topics[0] != topic {
		return nil, fmt.Errorf("receipt log does not match the exact contract/event point")
	}
	return matched, nil
}

func frostRetainedGroupDeploymentDescriptorAt(
	deployment FrostPreSignDeploymentEvidence,
	blockNumber uint64,
	blockHash [32]byte,
	rejectTransitionBlock bool,
) (FrostPreSignDeploymentDescriptorEvidence, error) {
	if blockNumber < deployment.RelevantEventStartBlock || blockHash == [32]byte{} {
		return FrostPreSignDeploymentDescriptorEvidence{},
			fmt.Errorf("retained-group point is outside the authenticated deployment range")
	}
	matchIndex := -1
	for index, epoch := range deployment.HistoricalEpochs {
		if blockNumber < epoch.Start.BlockNumber ||
			(epoch.End != nil && blockNumber > epoch.End.BlockNumber) {
			continue
		}
		if matchIndex >= 0 {
			return FrostPreSignDeploymentDescriptorEvidence{},
				fmt.Errorf("retained-group point matches multiple deployment epochs")
		}
		matchIndex = index
	}
	if matchIndex < 0 {
		return FrostPreSignDeploymentDescriptorEvidence{},
			fmt.Errorf("retained-group point has no authenticated deployment epoch")
	}
	epoch := deployment.HistoricalEpochs[matchIndex]
	if (blockNumber == epoch.Start.BlockNumber &&
		blockHash != epoch.Start.BlockHash) ||
		(epoch.End != nil && blockNumber == epoch.End.BlockNumber &&
			blockHash != epoch.End.BlockHash) {
		return FrostPreSignDeploymentDescriptorEvidence{},
			fmt.Errorf("retained-group point conflicts with a signed deployment boundary")
	}
	if rejectTransitionBlock && matchIndex > 0 &&
		blockNumber == epoch.Start.BlockNumber {
		return FrostPreSignDeploymentDescriptorEvidence{},
			fmt.Errorf("retained-group event occurs in an implementation-transition block")
	}
	return epoch.Descriptor, nil
}

func (source *signedFrostRetainedGroupHistorySource) authenticateContractDeployment(
	ctx context.Context,
	descriptor FrostPreSignDeploymentDescriptorEvidence,
	blockNumber uint64,
	blockHash [32]byte,
	cache frostRetainedGroupCodeCache,
) error {
	if blockNumber == 0 || blockHash == [32]byte{} {
		return fmt.Errorf("retained-group contract deployment point is invalid")
	}
	verifiedKey := frostRetainedGroupCodeCacheKey{
		address:        common.Address(descriptor.Address),
		blockHash:      common.Hash(blockHash),
		codeHash:       common.Hash(descriptor.RuntimeCodeHash),
		descriptorHash: common.Hash(descriptor.DescriptorHash),
		verified:       true,
	}
	if _, ok := cache[verifiedKey]; ok {
		return nil
	}
	proxyCode, err := source.readAuthenticatedCode(
		ctx,
		common.Address(descriptor.Address),
		common.Hash(descriptor.RuntimeCodeHash),
		common.Hash(descriptor.DescriptorHash),
		blockNumber,
		common.Hash(blockHash),
		cache,
	)
	if err != nil {
		return err
	}
	implementationSlot := frostRetainedGroupEIP1967Slot(
		"eip1967.proxy.implementation",
	)
	adminSlot := frostRetainedGroupEIP1967Slot("eip1967.proxy.admin")
	requestContext, cancel := source.requestContext(ctx)
	implementationValue, err := source.verifier.StorageAtHash(
		requestContext,
		common.Address(descriptor.Address),
		implementationSlot,
		common.Hash(blockHash),
	)
	cancel()
	if err != nil || len(implementationValue) != 32 {
		return fmt.Errorf("cannot read retained-group EIP-1967 implementation slot: [%w]", err)
	}
	requestContext, cancel = source.requestContext(ctx)
	adminValue, err := source.verifier.StorageAtHash(
		requestContext,
		common.Address(descriptor.Address),
		adminSlot,
		common.Hash(blockHash),
	)
	cancel()
	if err != nil || len(adminValue) != 32 {
		return fmt.Errorf("cannot read retained-group EIP-1967 admin slot: [%w]", err)
	}
	ownerCode := proxyCode
	switch descriptor.Upgradeability {
	case "immutable":
		if !bytes.Equal(implementationValue, make([]byte, 32)) ||
			!bytes.Equal(adminValue, make([]byte, 32)) {
			return fmt.Errorf("immutable retained-group deployment has populated EIP-1967 slots")
		}
	case "eip1967":
		if !bytes.Equal(implementationValue, descriptor.ImplementationSlotValue[:]) ||
			!bytes.Equal(adminValue, descriptor.AdminSlotValue[:]) {
			return fmt.Errorf("retained-group EIP-1967 slot value mismatch")
		}
		ownerCode, err = source.readAuthenticatedCode(
			ctx,
			common.Address(descriptor.ImplementationAddress),
			common.Hash(descriptor.ImplementationCodeHash),
			common.Hash(descriptor.DescriptorHash),
			blockNumber,
			common.Hash(blockHash),
			cache,
		)
		if err != nil {
			return fmt.Errorf("retained-group implementation authentication failed: [%w]", err)
		}
		if _, err := source.readAuthenticatedCode(
			ctx,
			common.Address(descriptor.AdminAddress),
			common.Hash(descriptor.AdminCodeHash),
			common.Hash(descriptor.DescriptorHash),
			blockNumber,
			common.Hash(blockHash),
			cache,
		); err != nil {
			return fmt.Errorf("retained-group admin authentication failed: [%w]", err)
		}
	default:
		return fmt.Errorf("retained-group deployment upgradeability is unsupported")
	}
	if err := source.authenticateLinkedLibraries(
		ctx,
		ownerCode,
		descriptor.LinkedLibraries,
		common.Hash(descriptor.DescriptorHash),
		blockNumber,
		common.Hash(blockHash),
		cache,
	); err != nil {
		return err
	}
	if len(cache) >= frostRetainedGroupMaximumEvidenceCodePoints {
		return fmt.Errorf("retained-group evidence exceeds the contract-code point limit")
	}
	cache[verifiedKey] = []byte{1}
	return nil
}

func (source *signedFrostRetainedGroupHistorySource) readAuthenticatedCode(
	ctx context.Context,
	address common.Address,
	expectedHash common.Hash,
	descriptorHash common.Hash,
	blockNumber uint64,
	blockHash common.Hash,
	cache frostRetainedGroupCodeCache,
) ([]byte, error) {
	key := frostRetainedGroupCodeCacheKey{
		address:        address,
		blockHash:      blockHash,
		codeHash:       expectedHash,
		descriptorHash: descriptorHash,
	}
	if cached, ok := cache[key]; ok {
		return cached, nil
	}
	if len(cache) >= frostRetainedGroupMaximumEvidenceCodePoints {
		return nil, fmt.Errorf("retained-group evidence exceeds the contract-code point limit")
	}
	requestContext, cancel := source.requestContext(ctx)
	code, err := source.verifier.CodeAtHash(
		requestContext,
		address,
		blockHash,
	)
	cancel()
	if err != nil {
		return nil, fmt.Errorf("cannot read pinned contract code at block [%d]: [%w]", blockNumber, err)
	}
	if len(code) == 0 || len(code) > frostRetainedGroupMaximumContractCodeBytes ||
		crypto.Keccak256Hash(code) != expectedHash {
		return nil, fmt.Errorf("contract code at block [%d] differs from the signed activation manifest", blockNumber)
	}
	copied := append([]byte{}, code...)
	cache[key] = copied
	return copied, nil
}

func (source *signedFrostRetainedGroupHistorySource) authenticateLinkedLibraries(
	ctx context.Context,
	ownerCode []byte,
	libraries []FrostPreSignLinkedLibraryEvidence,
	descriptorHash common.Hash,
	blockNumber uint64,
	blockHash common.Hash,
	cache frostRetainedGroupCodeCache,
) error {
	for _, library := range libraries {
		for _, reference := range library.References {
			if reference.Start > uint64(len(ownerCode)) ||
				reference.Start+reference.Length < reference.Start ||
				reference.Start+reference.Length > uint64(len(ownerCode)) ||
				!bytes.Equal(
					ownerCode[int(reference.Start):int(reference.Start+reference.Length)],
					library.Address[:],
				) {
				return fmt.Errorf("retained-group linked-library reference [%s:%d] mismatch", library.ProtocolRole, reference.Start)
			}
		}
		libraryCode, err := source.readAuthenticatedCode(
			ctx,
			common.Address(library.Address),
			common.Hash(library.RuntimeCodeHash),
			descriptorHash,
			blockNumber,
			blockHash,
			cache,
		)
		if err != nil {
			return fmt.Errorf("retained-group linked library [%s] authentication failed: [%w]", library.ProtocolRole, err)
		}
		if err := source.authenticateLinkedLibraries(
			ctx,
			libraryCode,
			library.LinkedLibraries,
			descriptorHash,
			blockNumber,
			blockHash,
			cache,
		); err != nil {
			return err
		}
	}
	return nil
}

func frostRetainedGroupEIP1967Slot(label string) common.Hash {
	value := crypto.Keccak256Hash([]byte(label)).Big()
	value.Sub(value, big.NewInt(1))
	return common.BigToHash(value)
}

func (source *signedFrostRetainedGroupHistorySource) resolveOperatorIDAt(
	ctx context.Context,
	operator common.Address,
	at FrostPreSignFinality,
	evidence *frostRetainedGroupEvidenceProfile,
) (uint32, error) {
	if evidence == nil || operator == (common.Address{}) {
		return 0, fmt.Errorf("operator-resolution evidence is incomplete")
	}
	deployment := evidence.deployments["frostSortitionPool"]
	descriptor, err := frostRetainedGroupDeploymentDescriptorAt(
		deployment,
		at.BlockNumber,
		at.BlockHash,
		false,
	)
	if err != nil {
		return 0, err
	}
	if err := source.authenticateContractDeployment(
		ctx,
		descriptor,
		at.BlockNumber,
		at.BlockHash,
		make(frostRetainedGroupCodeCache),
	); err != nil {
		return 0, err
	}
	// getOperatorID(address), pinned explicitly rather than learned from an
	// exporter or a mutable ABI service.
	callData := make([]byte, 4+32)
	copy(callData[:4], []byte{0x5a, 0x48, 0xb4, 0x6b})
	copy(callData[4+12:], operator[:])
	to := common.Address(descriptor.Address)
	requestContext, cancel := source.requestContext(ctx)
	output, err := source.verifier.CallContractAtHash(
		requestContext,
		ethereum.CallMsg{To: &to, Data: callData},
		common.Hash(at.BlockHash),
	)
	cancel()
	if err != nil {
		return 0, err
	}
	if len(output) != 32 || !bytes.Equal(output[:28], make([]byte, 28)) {
		return 0, fmt.Errorf("sortition-pool getOperatorID returned noncanonical data")
	}
	operatorID := binary.BigEndian.Uint32(output[28:])
	if operatorID == 0 {
		return 0, fmt.Errorf("operator is not registered in the pinned sortition pool at the requested block")
	}
	return operatorID, nil
}

func frostRetainedGroupEqualOperatorIDs(left []uint32, right []uint32) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func frostRetainedGroupBytes20Topic(value [20]byte) common.Hash {
	var result common.Hash
	copy(result[:20], value[:])
	return result
}

func frostRetainedGroupCanonicalAddressTopic(topic common.Hash) bool {
	return bytes.Equal(topic[:12], make([]byte, 12))
}
