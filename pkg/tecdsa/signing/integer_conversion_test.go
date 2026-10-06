package signing

import (
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/keep-network/keep-core/pkg/tecdsa/signing/gen/pb"
)

// TestUnmarshal_SenderIDBounds verifies that every round message rejects a
// SenderID value that overflows group.MaxMemberIndex.
func TestUnmarshal_SenderIDBounds(t *testing.T) {
	tests := map[string]struct {
		marshal func(senderID uint32) ([]byte, error)
		decode  func(bytes []byte) error
	}{
		"ephemeralPublicKeyMessage": {
			marshal: func(senderID uint32) ([]byte, error) {
				return proto.Marshal(&pb.EphemeralPublicKeyMessage{SenderID: senderID})
			},
			decode: func(bytes []byte) error {
				return new(ephemeralPublicKeyMessage).Unmarshal(bytes)
			},
		},
		"tssRoundOneMessage": {
			marshal: func(senderID uint32) ([]byte, error) {
				return proto.Marshal(&pb.TSSRoundOneMessage{SenderID: senderID})
			},
			decode: func(bytes []byte) error {
				return new(tssRoundOneMessage).Unmarshal(bytes)
			},
		},
		"tssRoundTwoMessage": {
			marshal: func(senderID uint32) ([]byte, error) {
				return proto.Marshal(&pb.TSSRoundTwoMessage{SenderID: senderID})
			},
			decode: func(bytes []byte) error {
				return new(tssRoundTwoMessage).Unmarshal(bytes)
			},
		},
		"tssRoundThreeMessage": {
			marshal: func(senderID uint32) ([]byte, error) {
				return proto.Marshal(&pb.TSSRoundThreeMessage{SenderID: senderID})
			},
			decode: func(bytes []byte) error {
				return new(tssRoundThreeMessage).Unmarshal(bytes)
			},
		},
		"tssRoundFourMessage": {
			marshal: func(senderID uint32) ([]byte, error) {
				return proto.Marshal(&pb.TSSRoundFourMessage{SenderID: senderID})
			},
			decode: func(bytes []byte) error {
				return new(tssRoundFourMessage).Unmarshal(bytes)
			},
		},
		"tssRoundFiveMessage": {
			marshal: func(senderID uint32) ([]byte, error) {
				return proto.Marshal(&pb.TSSRoundFiveMessage{SenderID: senderID})
			},
			decode: func(bytes []byte) error {
				return new(tssRoundFiveMessage).Unmarshal(bytes)
			},
		},
		"tssRoundSixMessage": {
			marshal: func(senderID uint32) ([]byte, error) {
				return proto.Marshal(&pb.TSSRoundSixMessage{SenderID: senderID})
			},
			decode: func(bytes []byte) error {
				return new(tssRoundSixMessage).Unmarshal(bytes)
			},
		},
		"tssRoundSevenMessage": {
			marshal: func(senderID uint32) ([]byte, error) {
				return proto.Marshal(&pb.TSSRoundSevenMessage{SenderID: senderID})
			},
			decode: func(bytes []byte) error {
				return new(tssRoundSevenMessage).Unmarshal(bytes)
			},
		},
		"tssRoundEightMessage": {
			marshal: func(senderID uint32) ([]byte, error) {
				return proto.Marshal(&pb.TSSRoundEightMessage{SenderID: senderID})
			},
			decode: func(bytes []byte) error {
				return new(tssRoundEightMessage).Unmarshal(bytes)
			},
		},
		"tssRoundNineMessage": {
			marshal: func(senderID uint32) ([]byte, error) {
				return proto.Marshal(&pb.TSSRoundNineMessage{SenderID: senderID})
			},
			decode: func(bytes []byte) error {
				return new(tssRoundNineMessage).Unmarshal(bytes)
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			data, err := test.marshal(256)
			if err != nil {
				t.Fatal(err)
			}

			if err := test.decode(data); err == nil {
				t.Fatal("expected out-of-range SenderID to be rejected")
			}
		})
	}
}

// TestUnmarshal_PeerMapKeyBounds verifies that the messages carrying a
// per-member map reject a map key that overflows group.MaxMemberIndex.
func TestUnmarshal_PeerMapKeyBounds(t *testing.T) {
	overflowKeyMap := map[uint32][]byte{256: []byte{1}}

	tests := map[string]struct {
		marshal func() ([]byte, error)
		decode  func(bytes []byte) error
	}{
		"ephemeralPublicKeyMessage/EphemeralPublicKeys": {
			marshal: func() ([]byte, error) {
				return proto.Marshal(&pb.EphemeralPublicKeyMessage{
					SenderID:            1,
					EphemeralPublicKeys: overflowKeyMap,
				})
			},
			decode: func(bytes []byte) error {
				return new(ephemeralPublicKeyMessage).Unmarshal(bytes)
			},
		},
		"tssRoundOneMessage/PeersPayload": {
			marshal: func() ([]byte, error) {
				return proto.Marshal(&pb.TSSRoundOneMessage{
					SenderID:     1,
					PeersPayload: overflowKeyMap,
				})
			},
			decode: func(bytes []byte) error {
				return new(tssRoundOneMessage).Unmarshal(bytes)
			},
		},
		"tssRoundTwoMessage/PeersPayload": {
			marshal: func() ([]byte, error) {
				return proto.Marshal(&pb.TSSRoundTwoMessage{
					SenderID:     1,
					PeersPayload: overflowKeyMap,
				})
			},
			decode: func(bytes []byte) error {
				return new(tssRoundTwoMessage).Unmarshal(bytes)
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			data, err := test.marshal()
			if err != nil {
				t.Fatal(err)
			}

			if err := test.decode(data); err == nil {
				t.Fatal("expected out-of-range peer map key to be rejected")
			}
		})
	}
}
