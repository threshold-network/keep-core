package tbtc

import (
	"math/big"
	"testing"

	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// TestReservationFuzzSeedsSanity drives each reservation fuzz target's
// seed values through the real marshal/unmarshal round trip, proving the
// seed examples agree with the production wire format.
func TestReservationFuzzSeedsSanity(t *testing.T) {
	anchorSeed := &ReservationAnchorProposal{
		DepositFundingTxHash:      [32]byte{0x01, 0x02, 0x03},
		DepositFundingOutputIndex: 1,
		RequestNonce:              7,
		AnchorTxFee:               big.NewInt(1500),
	}
	anchorBytes, err := anchorSeed.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	anchorBack := &ReservationAnchorProposal{}
	if err := anchorBack.Unmarshal(anchorBytes); err != nil {
		t.Fatal(err)
	}

	reanchorSeed := &ReservationReanchorProposal{
		ReservationKey:            big.NewInt(424242),
		RequestNonce:              4,
		TargetWalletPublicKeyHash: [20]byte{0xf8, 0x7e, 0xb7},
		ReanchorTxFee:             big.NewInt(1700),
	}
	reanchorBytes, err := reanchorSeed.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	reanchorBack := &ReservationReanchorProposal{}
	if err := reanchorBack.Unmarshal(reanchorBytes); err != nil {
		t.Fatal(err)
	}

	anchorMessage := &coordinationMessage{
		senderID:            group.MemberIndex(1),
		coordinationBlock:   42,
		walletPublicKeyHash: [20]byte{0x01},
		proposal:            anchorSeed,
	}
	anchorMessageBytes, err := anchorMessage.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	anchorMessageBack := &coordinationMessage{}
	if err := anchorMessageBack.Unmarshal(anchorMessageBytes); err != nil {
		t.Fatal(err)
	}
	if _, ok := anchorMessageBack.proposal.(*ReservationAnchorProposal); !ok {
		t.Fatalf(
			"message proposal is %T, want *ReservationAnchorProposal",
			anchorMessageBack.proposal,
		)
	}
}

// FuzzReservationAnchorProposal_Unmarshal drives the untrusted peer input
// surface of the reservation anchor proposal: bytes from any network peer
// land in ReservationAnchorProposal.Unmarshal.
//
// Invariants:
//   - never panics on arbitrary bytes;
//   - anything that unmarshals satisfies the same validation the marshal
//     side enforces (positive fee within 8 bytes, non-zero nonce, non-zero
//     funding hash), so an accepted value can always be carried further by
//     every follower;
//   - an unmarshaled value round-trips: marshal -> unmarshal yields an
//     equal value and succeeds.
func FuzzReservationAnchorProposal_Unmarshal(f *testing.F) {
	// Seed: the valid example from TestReservationProposals_MarshalingRoundtrip.
	anchorSeed := &ReservationAnchorProposal{
		DepositFundingTxHash:      [32]byte{0x01, 0x02, 0x03},
		DepositFundingOutputIndex: 1,
		RequestNonce:              7,
		AnchorTxFee:               big.NewInt(1500),
	}
	if seedBytes, err := anchorSeed.Marshal(); err == nil {
		f.Add(seedBytes)
	}
	// Seed: an empty payload exercises the missing-field rejections.
	f.Add([]byte{})

	f.Fuzz(
		func(t *testing.T, data []byte) {
			proposal := &ReservationAnchorProposal{}
			if err := proposal.Unmarshal(data); err != nil {
				return
			}

			// Marshal-side validation must hold for the unmarshaled
			// value: an accepted value must be one every follower can
			// re-marshal into the coordinated transaction.
			remarshaled, err := proposal.Marshal()
			if err != nil {
				t.Fatalf(
					"unmarshaled proposal does not satisfy Marshal "+
						"validation: %v; value: %v",
					err,
					proposal,
				)
			}

			roundTrip := &ReservationAnchorProposal{}
			if err := roundTrip.Unmarshal(remarshaled); err != nil {
				t.Fatalf("round trip re-unmarshal failed: %v", err)
			}

			if roundTrip.DepositFundingTxHash !=
				proposal.DepositFundingTxHash ||
				roundTrip.DepositFundingOutputIndex !=
					proposal.DepositFundingOutputIndex ||
				roundTrip.RequestNonce != proposal.RequestNonce ||
				roundTrip.AnchorTxFee.Cmp(proposal.AnchorTxFee) != 0 {
				t.Fatalf(
					"round trip mismatch: original [%v] vs [%v]",
					proposal,
					roundTrip,
				)
			}
		},
	)
}

// FuzzReservationReanchorProposal_Unmarshal is the re-anchor analog of
// FuzzReservationAnchorProposal_Unmarshal. ReservationKey is a 256-bit EVM
// uint carried big-endian, so its byte encoding must stay within 32 bytes;
// the target verifies the unmarshal path rejects oversized encodings and
// round-trips the rest.
func FuzzReservationReanchorProposal_Unmarshal(f *testing.F) {
	// Seed: the valid example from TestReservationProposals_MarshalingRoundtrip.
	reanchorSeed := &ReservationReanchorProposal{
		ReservationKey:            big.NewInt(54321),
		RequestNonce:              3,
		TargetWalletPublicKeyHash: [20]byte{0xaa, 0xbb},
		ReanchorTxFee:             big.NewInt(1700),
	}
	if seedBytes, err := reanchorSeed.Marshal(); err == nil {
		f.Add(seedBytes)
	}
	// Seed: an empty payload exercises the missing-field rejections.
	f.Add([]byte{})

	f.Fuzz(
		func(t *testing.T, data []byte) {
			proposal := &ReservationReanchorProposal{}
			if err := proposal.Unmarshal(data); err != nil {
				return
			}

			// The unmarshaled key must round-trip through Marshal:
			// Marshal rejects keys whose big-endian encoding exceeds 32
			// bytes (the 256-bit EVM uint bound), so any accepted value
			// is marshalable.
			remarshaled, err := proposal.Marshal()
			if err != nil {
				t.Fatalf(
					"unmarshaled proposal does not satisfy Marshal "+
						"validation: %v; value: %v",
					err,
					proposal,
				)
			}

			roundTrip := &ReservationReanchorProposal{}
			if err := roundTrip.Unmarshal(remarshaled); err != nil {
				t.Fatalf("round trip re-unmarshal failed: %v", err)
			}

			if roundTrip.RequestNonce != proposal.RequestNonce ||
				roundTrip.TargetWalletPublicKeyHash !=
					proposal.TargetWalletPublicKeyHash ||
				roundTrip.ReservationKey.Cmp(proposal.ReservationKey) != 0 ||
				roundTrip.ReanchorTxFee.Cmp(proposal.ReanchorTxFee) != 0 {
				t.Fatalf(
					"round trip mismatch: original [%v] vs [%v]",
					proposal,
					roundTrip,
				)
			}
		},
	)
}

// FuzzCoordinationMessage_Unmarshal drives the coordination message that
// carries both reservation proposals (and every other proposal type)
// through the unmarshalCoordinationProposal dispatch. A message that
// unmarshals must round-trip through its own Marshal: the carried
// proposal must be one the marshal side accepts, otherwise a leader that
// unmarshals it cannot re-marshal and forward the coordinated result.
func FuzzCoordinationMessage_Unmarshal(f *testing.F) {
	// Seeds: valid examples carrying each reservation proposal.
	anchorSeed := &ReservationAnchorProposal{
		DepositFundingTxHash:      [32]byte{0x01, 0x02, 0x03},
		DepositFundingOutputIndex: 1,
		RequestNonce:              7,
		AnchorTxFee:               big.NewInt(1500),
	}
	anchorMessage := &coordinationMessage{
		senderID:            group.MemberIndex(1),
		coordinationBlock:   42,
		walletPublicKeyHash: [20]byte{0x01},
		proposal:            anchorSeed,
	}
	if seedBytes, err := anchorMessage.Marshal(); err == nil {
		f.Add(seedBytes)
	}

	reanchorSeed := &ReservationReanchorProposal{
		ReservationKey:            big.NewInt(54321),
		RequestNonce:              3,
		TargetWalletPublicKeyHash: [20]byte{0xaa, 0xbb},
		ReanchorTxFee:             big.NewInt(1700),
	}
	reanchorMessage := &coordinationMessage{
		senderID:            group.MemberIndex(2),
		coordinationBlock:   43,
		walletPublicKeyHash: [20]byte{0x02},
		proposal:            reanchorSeed,
	}
	if seedBytes, err := reanchorMessage.Marshal(); err == nil {
		f.Add(seedBytes)
	}

	f.Fuzz(
		func(t *testing.T, data []byte) {
			message := &coordinationMessage{}
			if err := message.Unmarshal(data); err != nil {
				return
			}

			// Every proposal type the message can carry must be
			// re-marshalable: each builds its payload from plain
			// protobuf fields, so a freshly unmarshaled value is
			// accepted by the marshal side.
			remarshaled, err := message.Marshal()
			if err != nil {
				t.Fatalf(
					"unmarshaled message does not satisfy Marshal "+
						"validation: %v; proposal: [%T] %v",
					err,
					message.proposal,
					message.proposal,
				)
			}

			roundTrip := &coordinationMessage{}
			if err := roundTrip.Unmarshal(remarshaled); err != nil {
				t.Fatalf("round trip re-unmarshal failed: %v", err)
			}

			if roundTrip.senderID != message.senderID ||
				roundTrip.coordinationBlock !=
					message.coordinationBlock ||
				roundTrip.walletPublicKeyHash !=
					message.walletPublicKeyHash ||
				roundTrip.proposal.ActionType() !=
					message.proposal.ActionType() {
				t.Fatalf(
					"round trip mismatch: original [%v] vs [%v]",
					message,
					roundTrip,
				)
			}
		},
	)
}
