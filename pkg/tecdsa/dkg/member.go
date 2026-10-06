package dkg

import (
	"fmt"
	"math/big"

	"github.com/bnb-chain/tss-lib/ecdsa/keygen"
	"github.com/bnb-chain/tss-lib/tss"
	"github.com/ipfs/go-log/v2"
	"github.com/keep-network/keep-core/pkg/crypto/ephemeral"
	"github.com/keep-network/keep-core/pkg/protocol/group"
	"github.com/keep-network/keep-core/pkg/tecdsa"
	"github.com/keep-network/keep-core/pkg/tecdsa/common"
)

// Member represents a DKG protocol member.
type member struct {
	// Logger used to produce log messages.
	logger log.StandardLogger
	// id of this group member.
	id group.MemberIndex
	// Group to which this member belongs.
	group *group.Group
	// Validator allowing to check public key and member index against
	// group members.
	membershipValidator *group.MembershipValidator
	// Identifier of the particular DKG session this member is part of.
	sessionID string
	// TSS pre-parameters getter.
	preParamsFn func() (*PreParams, error)
	// Concurrency level of TSS key-generation protocol.
	keyGenerationConcurrency int
	// Instance of the member identity converter.
	identityConverter *identityConverter
}

// newMember creates a new member in an initial state
func newMember(
	logger log.StandardLogger,
	seed *big.Int,
	memberID group.MemberIndex,
	groupSize,
	dishonestThreshold int,
	membershipValidator *group.MembershipValidator,
	sessionID string,
	preParamsFn func() (*PreParams, error),
	keyGenerationConcurrency int,
) *member {
	return &member{
		logger:                   logger,
		id:                       memberID,
		group:                    group.NewGroup(dishonestThreshold, groupSize),
		membershipValidator:      membershipValidator,
		sessionID:                sessionID,
		preParamsFn:              preParamsFn,
		keyGenerationConcurrency: keyGenerationConcurrency,
		identityConverter:        &identityConverter{seed: seed},
	}
}

// inactiveMemberFilter returns a new instance of the inactive member filter.
func (m *member) inactiveMemberFilter() *group.InactiveMemberFilter {
	return group.NewInactiveMemberFilter(m.logger, m.id, m.group)
}

// shouldAcceptMessage indicates whether the given member should accept
// a message from the given sender.
func (m *member) shouldAcceptMessage(
	senderID group.MemberIndex,
	senderPublicKey []byte,
) bool {
	isMessageFromSelf := senderID == m.id
	isSenderValid := m.membershipValidator.IsValidMembership(
		senderID,
		senderPublicKey,
	)
	isSenderAccepted := m.group.IsOperating(senderID)

	return !isMessageFromSelf && isSenderValid && isSenderAccepted
}

// initializeEphemeralKeysGeneration performs a transition of a member state
// from the initial state to the first phase of the protocol.
func (m *member) initializeEphemeralKeysGeneration() *ephemeralKeyPairGeneratingMember {
	return &ephemeralKeyPairGeneratingMember{
		member:            m,
		ephemeralKeyPairs: make(map[group.MemberIndex]*ephemeral.KeyPair),
	}
}

// ephemeralKeyPairGeneratingMember represents one member in a distributed key
// generating group performing ephemeral key pair generation. It has a full list
// of `memberIndexes` that belong to its threshold group.
type ephemeralKeyPairGeneratingMember struct {
	*member

	// Ephemeral key pairs used to create symmetric keys,
	// generated individually for each other group member.
	ephemeralKeyPairs map[group.MemberIndex]*ephemeral.KeyPair
}

// initializeSymmetricKeyGeneration performs a transition of the member state
// to the next phase. It returns a member instance ready to execute the
// next phase of the protocol.
func (ekpgm *ephemeralKeyPairGeneratingMember) initializeSymmetricKeyGeneration() *symmetricKeyGeneratingMember {
	return &symmetricKeyGeneratingMember{
		ephemeralKeyPairGeneratingMember: ekpgm,
		symmetricKeys:                    make(map[group.MemberIndex]ephemeral.SymmetricKey),
	}
}

// symmetricKeyGeneratingMember represents one member in a distributed key
// generating group performing ephemeral symmetric key generation.
type symmetricKeyGeneratingMember struct {
	*ephemeralKeyPairGeneratingMember

	// Symmetric keys used to encrypt confidential information,
	// generated individually for each other group member by ECDH'ing the
	// broadcasted ephemeral public key intended for this member and the
	// ephemeral private key generated for the other member.
	symmetricKeys map[group.MemberIndex]ephemeral.SymmetricKey
}

// initializeTssRoundOne returns a member to perform next protocol operations.
func (skgm *symmetricKeyGeneratingMember) initializeTssRoundOne() (
	*tssRoundOneMember,
	error,
) {
	// Set up the local TSS party using only operating members. This effectively
	// removes all excluded members who were marked as disqualified at the
	// beginning of the protocol.
	tssPartyID, groupTssPartiesIDs := common.GenerateTssPartiesIDs(
		skgm.id,
		skgm.group.OperatingMemberIndexes(),
		skgm.identityConverter,
	)

	tssParameters := tss.NewParameters(
		tecdsa.Curve,
		tss.NewPeerContext(tss.SortPartyIDs(groupTssPartiesIDs)),
		tssPartyID,
		len(groupTssPartiesIDs),
		skgm.group.HonestThreshold()-1,
	)
	tssParameters.SetConcurrency(skgm.keyGenerationConcurrency)

	tssOutgoingMessagesChan := make(chan tss.Message, len(groupTssPartiesIDs))
	tssResultChan := make(chan keygen.LocalPartySaveData, 1)

	preParams, err := skgm.preParamsFn()
	if err != nil {
		return nil, fmt.Errorf("failed fetching pre-params: [%w]", err)
	}

	tssParty := keygen.NewLocalParty(
		tssParameters,
		tssOutgoingMessagesChan,
		tssResultChan,
		*preParams.data,
	)

	return &tssRoundOneMember{
		symmetricKeyGeneratingMember: skgm,
		tssParty:                     tssParty,
		tssParameters:                tssParameters,
		tssOutgoingMessagesChan:      tssOutgoingMessagesChan,
		tssResultChan:                tssResultChan,
	}, nil
}

// tssRoundOneMember represents one member in a distributed key generating
// group performing the first round of the TSS keygen.
type tssRoundOneMember struct {
	*symmetricKeyGeneratingMember

	tssParty                tss.Party
	tssParameters           *tss.Parameters
	tssOutgoingMessagesChan <-chan tss.Message
	tssResultChan           <-chan keygen.LocalPartySaveData
}

// initializeTssRoundTwo returns a member to perform next protocol operations.
func (trom *tssRoundOneMember) initializeTssRoundTwo() *tssRoundTwoMember {
	return &tssRoundTwoMember{
		tssRoundOneMember: trom,
	}
}

// tssRoundTwoMember represents one member in a distributed key generating
// group performing the second round of the TSS keygen.
type tssRoundTwoMember struct {
	*tssRoundOneMember
}

// initializeTssRoundThree returns a member to perform next protocol operations.
func (trtm *tssRoundTwoMember) initializeTssRoundThree() *tssRoundThreeMember {
	return &tssRoundThreeMember{
		tssRoundTwoMember: trtm,
	}
}

// tssRoundThreeMember represents one member in a distributed key generating
// group performing the third round of the TSS keygen.
type tssRoundThreeMember struct {
	*tssRoundTwoMember
}

// initializeFinalization returns a member to perform next protocol operations.
func (trtm *tssRoundThreeMember) initializeFinalization() *finalizingMember {
	return &finalizingMember{
		tssRoundThreeMember: trtm,
	}
}

// finalizingMember represents one member of the given group performing the
// finalization of the TSS process and preparing the distributed key generation
// result.
type finalizingMember struct {
	*tssRoundThreeMember

	tssResult keygen.LocalPartySaveData
}

// Result is the successful computation of the distributed key generation process.
func (fm *finalizingMember) Result() *Result {
	return &Result{
		Group:           fm.group,
		PrivateKeyShare: tecdsa.NewPrivateKeyShare(fm.tssResult),
	}
}

// signingMember represents a group member sharing their preferred DKG result hash
// and signature (over this hash) with other peer members.
type signingMember struct {
	logger      log.StandardLogger
	memberIndex group.MemberIndex
	// Group to which this member belongs.
	group *group.Group
	// Validator allowing to check public key and member index
	// against group members
	membershipValidator *group.MembershipValidator
	// Identifier of the particular DKG session this member is part of.
	sessionID string
	// Hash of DKG result preferred by the current participant.
	preferredDKGResultHash ResultSignatureHash
	// Signature over preferredDKGResultHash calculated by the member.
	selfDKGResultSignature []byte
}

// newSigningMember creates a new signingMember in the initial state.
func newSigningMember(
	logger log.StandardLogger,
	memberIndex group.MemberIndex,
	group *group.Group,
	membershipValidator *group.MembershipValidator,
	sessionID string,
) *signingMember {
	return &signingMember{
		logger:              logger,
		memberIndex:         memberIndex,
		group:               group,
		membershipValidator: membershipValidator,
		sessionID:           sessionID,
	}
}

// shouldAcceptMessage indicates whether the given member should accept
// a message from the given sender.
func (sm *signingMember) shouldAcceptMessage(
	senderID group.MemberIndex,
	senderPublicKey []byte,
) bool {
	isMessageFromSelf := senderID == sm.memberIndex
	isSenderValid := sm.membershipValidator.IsValidMembership(
		senderID,
		senderPublicKey,
	)
	isSenderAccepted := sm.group.IsOperating(senderID)

	return !isMessageFromSelf && isSenderValid && isSenderAccepted
}

// initializeSubmittingMember performs a transition of a member state to the
// next phase of the protocol.
func (sm *signingMember) initializeSubmittingMember() *submittingMember {
	return &submittingMember{
		signingMember: sm,
	}
}

// submittingMember represents a member submitting a DKG result to the
// blockchain along with signatures received from other group members supporting
// the result.
type submittingMember struct {
	*signingMember
}

// identityConverter implements the common.IdentityConverter for tECDSA DKG.
// It maps every member index to a party ID by adding a constant seed value.
type identityConverter struct {
	seed *big.Int
}

func (ic *identityConverter) MemberIndexToTssPartyID(
	memberIndex group.MemberIndex,
) *tss.PartyID {
	partyIDKey := ic.MemberIndexToTssPartyIDKey(memberIndex)

	return tss.NewPartyID(
		partyIDKey.Text(10),
		fmt.Sprintf("member-%v", memberIndex),
		partyIDKey,
	)
}

func (ic *identityConverter) MemberIndexToTssPartyIDKey(
	memberIndex group.MemberIndex,
) *big.Int {
	return new(big.Int).Add(ic.seed, big.NewInt(int64(memberIndex)))
}

func (ic *identityConverter) TssPartyIDToMemberIndex(
	partyID *tss.PartyID,
) group.MemberIndex {
	if ic.seed.Cmp(partyID.KeyInt()) > 0 { // is seed > party ID?
		return group.MemberIndex(0)
	}

	index := new(big.Int).Sub(partyID.KeyInt(), ic.seed)
	if !index.IsUint64() {
		return 0
	}
	value := index.Uint64()
	if value > group.MaxMemberIndex {
		return 0
	}
	return group.MemberIndex(value)
}
