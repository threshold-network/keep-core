package dkg

import (
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/keep-network/keep-core/pkg/tecdsa/dkg/gen/pb"
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
		"tssFinalizationMessage": {
			marshal: func(senderID uint32) ([]byte, error) {
				return proto.Marshal(&pb.TSSFinalizationMessage{SenderID: senderID})
			},
			decode: func(bytes []byte) error {
				return new(tssFinalizationMessage).Unmarshal(bytes)
			},
		},
		"resultSignatureMessage": {
			marshal: func(senderID uint32) ([]byte, error) {
				return proto.Marshal(&pb.ResultSignatureMessage{SenderID: senderID})
			},
			decode: func(bytes []byte) error {
				return new(resultSignatureMessage).Unmarshal(bytes)
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
