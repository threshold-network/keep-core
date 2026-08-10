//go:build frost_native

package signing

import (
	"bytes"
	"context"
	"testing"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

func TestShareRepairTransportRejectsMalformedFrames(t *testing.T) {
	valid := shareRepairMessage{
		Type:               shareRepairAnnouncementMessage,
		Sender:             1,
		ContextDigest:      [32]byte{0x01},
		EphemeralPublicKey: bytes.Repeat([]byte{0x02}, 33),
	}
	wire, err := (&shareRepairTransportMessage{message: valid}).Marshal()
	if err != nil {
		t.Fatal(err)
	}
	decoded := &shareRepairTransportMessage{}
	if err := decoded.Unmarshal(wire); err != nil || decoded.message.Sender != 1 {
		t.Fatalf("valid share-repair frame failed round trip: %v", err)
	}

	mutations := map[string]func([]byte) []byte{
		"unknown type": func(value []byte) []byte {
			value[0] = 0xff
			return value
		},
		"zero sender": func(value []byte) []byte {
			for index := 1; index < 5; index++ {
				value[index] = 0
			}
			return value
		},
		"zero context": func(value []byte) []byte {
			for index := 9; index < 41; index++ {
				value[index] = 0
			}
			return value
		},
		"truncated ephemeral key": func(value []byte) []byte {
			return value[:len(value)-1]
		},
		"announcement recipient": func(value []byte) []byte {
			value[8] = 2
			return value
		},
		"announcement payload": func(value []byte) []byte {
			return append(value, 0x01)
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			candidate := mutate(append([]byte(nil), wire...))
			if err := (&shareRepairTransportMessage{}).Unmarshal(candidate); err == nil {
				t.Fatal("malformed share-repair frame was accepted")
			}
		})
	}
}

func TestShareRepairBusAuthenticatesSenderAndSuppressesReplay(t *testing.T) {
	fixture := newRunnerBusAuthFixture(t, 8)
	channel := &immediateRecvBroadcastChannel{}
	busInterface, err := newBroadcastChannelShareRepairBus(
		context.Background(),
		&testutils.MockLogger{},
		channel,
		fixture.validator,
	)
	if err != nil {
		t.Fatal(err)
	}
	bus := busInterface.(*broadcastChannelShareRepairBus)
	stream := bus.Subscribe(group.MemberIndex(2))
	message := shareRepairMessage{
		Type:               shareRepairAnnouncementMessage,
		Sender:             1,
		ContextDigest:      [32]byte{0x44},
		EphemeralPublicKey: bytes.Repeat([]byte{0x03}, 33),
	}
	wire := &shareRepairTransportMessage{message: message}
	bus.handleMessage(fakeNetMessage{senderPublicKey: fixture.operatorB, payload: wire})
	select {
	case <-stream:
		t.Fatal("claimed sender authenticated by the wrong operator was delivered")
	default:
	}

	authenticated := fakeNetMessage{senderPublicKey: fixture.operatorA, payload: wire}
	bus.handleMessage(authenticated)
	bus.handleMessage(authenticated)
	select {
	case received := <-stream:
		if received.Sender != 1 || received.ContextDigest != message.ContextDigest {
			t.Fatalf("unexpected authenticated share-repair message: %+v", received)
		}
	default:
		t.Fatal("authenticated share-repair message was not delivered")
	}
	select {
	case <-stream:
		t.Fatal("replayed share-repair message was delivered twice")
	default:
	}
}
