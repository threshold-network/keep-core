package registry

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/ipfs/go-log"

	beaconchain "github.com/keep-network/keep-core/pkg/beacon/chain"
	"github.com/keep-network/keep-core/pkg/beacon/dkg"

	"github.com/keep-network/keep-core/pkg/persistence"
)

// Groups represents a collection of Keep groups in which the given
// client is a member.
type Groups struct {
	mutex sync.Mutex

	logger log.StandardLogger

	// key is group public key in uncompressed form
	myGroups map[string][]*Membership

	beaconChain beaconchain.GroupRegistrationInterface

	storage storage
}

// Membership represents a member of a group
type Membership struct {
	Signer      *dkg.ThresholdSigner
	ChannelName string
}

// NewGroupRegistry returns an empty GroupRegistry.
func NewGroupRegistry(
	logger log.StandardLogger,
	beaconChain beaconchain.GroupRegistrationInterface,
	persistence persistence.ProtectedHandle,
) *Groups {
	return &Groups{
		logger:      logger,
		myGroups:    make(map[string][]*Membership),
		beaconChain: beaconChain,
		storage:     newStorage(persistence),
		mutex:       sync.Mutex{},
	}
}

// RegisterGroup registers that a group was successfully created by the given
// groupPublicKey.
func (g *Groups) RegisterGroup(
	signer *dkg.ThresholdSigner,
	channelName string,
) error {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	groupPublicKey := groupKeyToString(signer.GroupPublicKeyBytes())

	membership := &Membership{
		Signer:      signer,
		ChannelName: channelName,
	}

	err := g.storage.save(membership)
	if err != nil {
		return fmt.Errorf("could not persist membership to the storage: [%v]", err)
	}

	g.myGroups[groupPublicKey] = append(g.myGroups[groupPublicKey], membership)

	return nil
}

// GetGroup gets a group by a groupPublicKey
func (g *Groups) GetGroup(groupPublicKey []byte) []*Membership {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	return g.myGroups[groupKeyToString(groupPublicKey)]
}

// UnregisterStaleGroups lookup for groups that have been marked as stale
// on-chain. A stale group is a group that has expired and a certain time passed
// after the group expiration. This guarantees the group will not be selected to
// a new operation and it cannot have an ongoing operation for which it could be
// selected before it expired. Such a group can be safely removed from the registry
// and archived in the underlying storage.
func (g *Groups) UnregisterStaleGroups(latestGroupPublicKey []byte) {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	for publicKey, memberships := range g.myGroups {
		publicKeyBytes, err := groupKeyFromString(publicKey)
		if err != nil {
			g.logger.Errorf(
				"error occurred while decoding public key into bytes: [%v]",
				err,
			)
			continue
		}

		// There is no need to check if the latest added group is a stale group.
		// It is also to avoid a scenario when there is a delay to sync with the
		// recent state of the chain which might lead to loggin a false positive
		// error: "Group does not exist".
		if !bytes.Equal(latestGroupPublicKey, publicKeyBytes) {
			isStaleGroup, err := g.beaconChain.IsStaleGroup(publicKeyBytes)
			if err != nil {
				g.logger.Errorf(
					"failed to check if stale for group with public key [%s]: [%v]",
					publicKey,
					err,
				)
				continue
			}

			if isStaleGroup {
				if len(memberships) == 0 {
					g.logger.Errorf(
						"inconsistent state; group with public key [%s] has no members",
						publicKey,
					)
					continue
				}

				compressedPublicKey := memberships[0].Signer.GroupPublicKeyBytesCompressed()
				err = g.storage.archive(compressedPublicKey)
				if err != nil {
					g.logger.Errorf("failed to archive group with compressed public key [%s]: [%v]",
						hex.EncodeToString(compressedPublicKey),
						err,
					)
					continue
				}

				g.logger.Infof(
					"archived group with compressed public key [%s]",
					hex.EncodeToString(compressedPublicKey),
				)

				delete(g.myGroups, publicKey)
			}
		}
	}
}

// LoadExistingGroups iterates over all stored memberships on disk and loads them
// into memory
func (g *Groups) LoadExistingGroups() {
	g.myGroups = make(map[string][]*Membership)

	membershipsChannel, errorsChannel := g.storage.readAll()

	// Two goroutines read from memberships and errors channels and either
	// adds memberships to the group registry or outputs an error to stderr.
	// The reason for using two goroutines at the same time - one for
	// memberships and one for errors is because channels do not have to be
	// buffered and we do not know in what order information is written to
	// channels.
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		for membership := range membershipsChannel {
			groupPublicKey := groupKeyToString(
				membership.Signer.GroupPublicKeyBytes(),
			)
			g.myGroups[groupPublicKey] = append(
				g.myGroups[groupPublicKey],
				membership,
			)
		}

		wg.Done()
	}()

	go func() {
		for err := range errorsChannel {
			g.logger.Errorf(
				"could not load membership from disk: [%v]",
				err,
			)
		}

		wg.Done()
	}()

	wg.Wait()

	g.printMemberships()
}

func (g *Groups) printMemberships() {
	for group, memberships := range g.myGroups {
		g.logger.Infof("group [0x%v] loaded with [%v] members", group, len(memberships))
	}
}

func groupKeyToString(groupKey []byte) string {
	return hex.EncodeToString(groupKey)
}

func groupKeyFromString(groupKey string) ([]byte, error) {
	return hex.DecodeString(groupKey)
}
