package tbtc

import (
	"fmt"
	"sync"

	"github.com/keep-network/keep-core/pkg/frost/signing"
)

type frostDurableSessionStoreIdentityReader func() (
	*signing.NativeTBTCSignerDurableStoreIdentity,
	error,
)

// frostDurableSessionStoreBinding pins the store libfrost_tbtc actually has
// open to the fingerprint in the authenticated activation manifest. It is
// checked at startup and again at every authorization/readiness boundary, so
// path, backend, lock, or storage replacement cannot be hidden by echoing the
// manifest value.
type frostDurableSessionStoreBinding struct {
	expectedFingerprint [32]byte
	readIdentity        frostDurableSessionStoreIdentityReader

	mutex    sync.Mutex
	baseline *signing.NativeTBTCSignerDurableStoreIdentity
}

func newFrostDurableSessionStoreBinding(
	expectedFingerprint string,
	readIdentity frostDurableSessionStoreIdentityReader,
) (*frostDurableSessionStoreBinding, error) {
	parsed, err := parseFrostActivationHex32(expectedFingerprint)
	if err != nil || parsed == [32]byte{} {
		return nil, fmt.Errorf(
			"invalid FROST durable session store fingerprint in activation manifest",
		)
	}
	if readIdentity == nil {
		return nil, fmt.Errorf("FROST durable session store identity reader is nil")
	}

	binding := &frostDurableSessionStoreBinding{
		expectedFingerprint: parsed,
		readIdentity:        readIdentity,
	}
	if _, err := binding.verify(); err != nil {
		return nil, err
	}
	return binding, nil
}

func (fdssb *frostDurableSessionStoreBinding) verify() (
	[32]byte,
	error,
) {
	if fdssb == nil || fdssb.readIdentity == nil ||
		fdssb.expectedFingerprint == [32]byte{} {
		return [32]byte{}, fmt.Errorf("FROST durable session store binding is unavailable")
	}

	identity, err := fdssb.readIdentity()
	if err != nil {
		return [32]byte{}, fmt.Errorf(
			"cannot read active FROST durable session store identity: %w",
			err,
		)
	}
	if identity == nil {
		return [32]byte{}, fmt.Errorf(
			"active FROST durable session store identity is nil",
		)
	}
	readback := *identity
	computed, err := signing.ComputeNativeTBTCSignerDurableStoreFingerprint(&readback)
	if err != nil {
		return [32]byte{}, fmt.Errorf(
			"active FROST durable session store identity is invalid: %w",
			err,
		)
	}
	if readback.Fingerprint != computed {
		return [32]byte{}, fmt.Errorf(
			"active FROST durable session store returned a self-inconsistent fingerprint",
		)
	}
	if computed != fdssb.expectedFingerprint {
		return [32]byte{}, fmt.Errorf(
			"active FROST durable session store differs from the signed activation manifest",
		)
	}

	fdssb.mutex.Lock()
	defer fdssb.mutex.Unlock()
	if fdssb.baseline != nil && *fdssb.baseline != readback {
		return [32]byte{}, fmt.Errorf(
			"active FROST durable session store identity changed after startup",
		)
	}
	baseline := readback
	fdssb.baseline = &baseline

	return computed, nil
}
