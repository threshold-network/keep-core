//go:build frost_native

package signing

import (
	"fmt"

	"github.com/keep-network/keep-core/pkg/frost/roast"
	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// ActiveRoastAttempt is the immutable, single-source-of-truth binding for one
// interactive signing attempt. The runner constructs it once - from the
// session/signer-material registry plus the Coordinator handle - and derives
// every engine, collector, and verifier call from it.
//
// Why a dedicated, validated, immutable binding: the engine resolves the group
// key from the sessionID and tweaks by the taproot root, and it cannot tell a
// valid-but-WRONG session/root from the right one. A mis-bound sessionID,
// attempt-context hash, or root would make the engine verify honest shares
// against the wrong material and produce FALSE blame. So the binding is
// validated once here and then held immutable; the runner never re-derives or
// re-extracts these values from peer messages.
type ActiveRoastAttempt struct {
	sessionID          string
	context            attempt.AttemptContext
	contextHash        [attempt.MessageDigestLength]byte
	handle             roast.AttemptHandle
	electedCoordinator group.MemberIndex
	taprootMerkleRoot  *[32]byte
	dkgGroupPublicKey  []byte
}

// NewActiveRoastAttempt binds an attempt after enforcing the consistency
// assertions that keep blame sound:
//
//   - sessionID is non-empty and equals ctx.SessionID - the engine resolves the
//     group key from this sessionID, so it must be the session that produced
//     ctx (the RFC-21 attempt-context hash folds in the session);
//   - the handle was minted for ctx (ctx.Hash() == handle.ContextHash());
//   - the elected coordinator is taken AUTHORITATIVELY from the handle via
//     coordinator.SelectedCoordinator, never a caller-supplied value;
//   - dkgGroupPublicKey is non-empty.
//
// The taproot root and DKG group public key are copied so the binding cannot be
// mutated through the caller's pointer/slice: NextAttempt must derive every
// attempt's seed from the SAME dkgGroupPublicKey bytes, and the verifier must
// tweak by the SAME root, for the whole signing session.
func NewActiveRoastAttempt(
	coordinator roast.Coordinator,
	handle roast.AttemptHandle,
	ctx attempt.AttemptContext,
	sessionID string,
	taprootMerkleRoot *[32]byte,
	dkgGroupPublicKey []byte,
) (*ActiveRoastAttempt, error) {
	if coordinator == nil {
		return nil, fmt.Errorf("roast runner: coordinator is nil")
	}
	if sessionID == "" {
		return nil, fmt.Errorf("roast runner: session id is empty")
	}
	if sessionID != ctx.SessionID {
		return nil, fmt.Errorf(
			"roast runner: session id %q does not match attempt context session id %q",
			sessionID,
			ctx.SessionID,
		)
	}
	if ctx.Hash() != handle.ContextHash() {
		return nil, fmt.Errorf(
			"roast runner: attempt context hash does not match the handle's bound context",
		)
	}
	elected, err := coordinator.SelectedCoordinator(handle)
	if err != nil {
		return nil, fmt.Errorf("roast runner: resolve elected coordinator: %w", err)
	}
	if len(dkgGroupPublicKey) == 0 {
		return nil, fmt.Errorf("roast runner: dkg group public key is empty")
	}

	var rootCopy *[32]byte
	if taprootMerkleRoot != nil {
		root := *taprootMerkleRoot
		rootCopy = &root
	}

	return &ActiveRoastAttempt{
		sessionID:          sessionID,
		context:            ctx,
		contextHash:        ctx.Hash(),
		handle:             handle,
		electedCoordinator: elected,
		taprootMerkleRoot:  rootCopy,
		dkgGroupPublicKey:  append([]byte(nil), dkgGroupPublicKey...),
	}, nil
}

// SessionID is the engine DKG session this attempt signs under.
func (a *ActiveRoastAttempt) SessionID() string { return a.sessionID }

// Context is the RFC-21 attempt context.
func (a *ActiveRoastAttempt) Context() attempt.AttemptContext { return a.context }

// ContextHash is the attempt-context hash (== Handle().ContextHash()).
func (a *ActiveRoastAttempt) ContextHash() [attempt.MessageDigestLength]byte {
	return a.contextHash
}

// Handle is the Coordinator handle for this attempt.
func (a *ActiveRoastAttempt) Handle() roast.AttemptHandle { return a.handle }

// ElectedCoordinator is the attempt's elected coordinator, resolved
// authoritatively from the handle at construction.
func (a *ActiveRoastAttempt) ElectedCoordinator() group.MemberIndex {
	return a.electedCoordinator
}

// TaprootMerkleRoot returns a COPY of the bound root (nil for a key-path
// spend), so a caller cannot mutate the immutable binding.
func (a *ActiveRoastAttempt) TaprootMerkleRoot() *[32]byte {
	if a.taprootMerkleRoot == nil {
		return nil
	}
	root := *a.taprootMerkleRoot
	return &root
}

// DkgGroupPublicKey returns a COPY of the bound DKG group public key. The same
// bytes feed every NextAttempt seed derivation for this session.
func (a *ActiveRoastAttempt) DkgGroupPublicKey() []byte {
	return append([]byte(nil), a.dkgGroupPublicKey...)
}
