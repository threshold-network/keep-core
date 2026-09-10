package registry

import (
	"fmt"
	"sync"

	"github.com/keep-network/keep-core/pkg/keepcommon/persistence"

	"encoding/hex"
)

type storage interface {
	save(membership *Membership) error
	readAll() (<-chan *Membership, <-chan error)
	archive(groupPublicKey []byte) error
}

type persistentStorage struct {
	handle persistence.ProtectedHandle
}

func newStorage(persistence persistence.ProtectedHandle) storage {
	return &persistentStorage{
		handle: persistence,
	}
}

func (ps *persistentStorage) save(membership *Membership) error {
	membershipBytes, err := membership.Marshal()
	if err != nil {
		return fmt.Errorf("marshalling of the membership failed: [%v]", err)
	}

	hexGroupPublicKey := hex.EncodeToString(membership.Signer.GroupPublicKeyBytesCompressed())

	return ps.handle.Save(membershipBytes, hexGroupPublicKey, "/membership_"+fmt.Sprint(membership.Signer.MemberID()))
}

func (ps *persistentStorage) archive(groupPublicKeyCompressed []byte) error {
	return ps.handle.Archive(hex.EncodeToString(groupPublicKeyCompressed))
}

func (ps *persistentStorage) readAll() (<-chan *Membership, <-chan error) {
	outputMemberships := make(chan *Membership)
	outputErrors := make(chan error)

	inputData, inputErrors := ps.handle.ReadAll()

	// We have two goroutines reading from data and errors channels at the same
	// time. The reason for that is because we don't know in what order
	// producers write information to channels.
	// The third goroutine waits for those two goroutines to finish and it
	// closes the output channels. Channels are not closed by two other goroutines
	// because data goroutine writes both to output memberships and errors
	// channel and we want to avoid a situation when we close the errors channel
	// and errors goroutine tries to write to it. The same the other way round.
	var wg sync.WaitGroup
	wg.Add(2)

	// Close channels when memberships and errors goroutines are done.
	go func() {
		wg.Wait()
		close(outputMemberships)
		close(outputErrors)
	}()

	// Errors goroutine - pass thru errors from input channel to output channel
	// unchanged.
	go func() {
		for err := range inputErrors {
			outputErrors <- err
		}
		wg.Done()
	}()

	// Memberships goroutine reads data from input channel, tries to unmarshal
	// the data to Membership and write the unmarshalled Membership to the
	// output memberships channel. In case of an error, goroutine writes that
	// error to an output errors channel.
	go func() {
		for descriptor := range inputData {
			content, err := descriptor.Content()
			if err != nil {
				outputErrors <- fmt.Errorf(
					"could not unmarshal membership from file [%v] in directory [%v]: [%v]",
					descriptor.Name(),
					descriptor.Directory(),
					err,
				)
				continue
			}

			membership := &Membership{}

			err = membership.Unmarshal(content)
			if err != nil {
				outputErrors <- fmt.Errorf(
					"could not unmarshal membership from file [%v] in directory [%v]: [%v]",
					descriptor.Name(),
					descriptor.Directory(),
					err,
				)
				continue
			}

			outputMemberships <- membership
		}

		wg.Done()
	}()

	return outputMemberships, outputErrors
}
