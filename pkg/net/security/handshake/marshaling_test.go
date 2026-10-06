package handshake

import (
	"reflect"
	"testing"

	fuzz "github.com/google/gofuzz"

	"github.com/keep-network/keep-core/pkg/internal/pbutils"
)

func TestAct1MessageRoundTrip(t *testing.T) {
	message := &Act1Message{
		nonce1:    100,
		protocol1: "keep-beacon",
	}

	unmarshaler := &Act1Message{}

	err := pbutils.RoundTrip(message, unmarshaler)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(message, unmarshaler) {
		t.Fatalf("unexpected content of unmarshaled message")
	}
}

func TestFuzzAct1MessageRoundtrip(t *testing.T) {
	for i := 0; i < 10; i++ {
		var (
			nonce1   uint64
			protocol string
		)

		f := fuzz.New().NilChance(0.1)
		f.Fuzz(&nonce1)
		f.Fuzz(&protocol)

		message := &Act1Message{
			nonce1:    nonce1,
			protocol1: protocol,
		}

		if err := pbutils.RoundTrip(message, &Act1Message{}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFuzzAct1MessageUnmarshaler(t *testing.T) {
	pbutils.AssertUnmarshalDoesNotPanic(&Act1Message{})
}

func TestAct2MessageRoundTrip(t *testing.T) {
	challenge := [32]byte{
		100, 101, 102, 103, 104, 105, 106, 107,
		100, 101, 102, 103, 104, 105, 106, 107,
		100, 101, 102, 103, 104, 105, 106, 107,
		100, 101, 102, 103, 104, 105, 106, 107,
	}

	message := &Act2Message{
		nonce2:    100,
		challenge: challenge,
		protocol2: "keep-ecdsa",
	}

	unmarshaler := &Act2Message{}

	err := pbutils.RoundTrip(message, unmarshaler)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(message, unmarshaler) {
		t.Fatalf("unexpected content of unmarshaled message")
	}
}

func TestFuzzAct2MessageRoundtrip(t *testing.T) {
	for i := 0; i < 10; i++ {
		var (
			nonce2    uint64
			challenge [32]byte
			protocol  string
		)

		f := fuzz.New().NilChance(0.1)
		f.Fuzz(&nonce2)
		f.Fuzz(&challenge)
		f.Fuzz(&protocol)

		message := &Act2Message{
			nonce2:    nonce2,
			challenge: challenge,
			protocol2: protocol,
		}

		if err := pbutils.RoundTrip(message, &Act2Message{}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFuzzAct2MessageUnmarshaler(t *testing.T) {
	pbutils.AssertUnmarshalDoesNotPanic(&Act2Message{})
}

func TestAct3MessageRoundTrip(t *testing.T) {
	challenge := [32]byte{
		100, 101, 102, 103, 104, 105, 106, 107,
		100, 101, 102, 103, 104, 105, 106, 107,
		100, 101, 102, 103, 104, 105, 106, 107,
		100, 101, 102, 103, 104, 105, 106, 107,
	}

	message := &Act3Message{
		challenge: challenge,
	}

	unmarshaler := &Act3Message{}

	err := pbutils.RoundTrip(message, unmarshaler)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(message, unmarshaler) {
		t.Fatalf("unexpected content of unmarshaled message")
	}
}

func TestFuzzAct3MessageRoundtrip(t *testing.T) {
	for i := 0; i < 10; i++ {
		var challenge [32]byte

		f := fuzz.New().NilChance(0.1)
		f.Fuzz(&challenge)

		message := &Act3Message{
			challenge: challenge,
		}

		if err := pbutils.RoundTrip(message, &Act3Message{}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFuzzAct3MessageUnmarshaler(t *testing.T) {
	pbutils.AssertUnmarshalDoesNotPanic(&Act3Message{})
}
