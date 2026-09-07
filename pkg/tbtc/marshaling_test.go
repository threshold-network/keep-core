package tbtc

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/hex"
	"math/big"
	"reflect"
	"strings"
	"testing"

	"github.com/keep-network/keep-core/pkg/bitcoin"

	fuzz "github.com/google/gofuzz"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/internal/pbutils"
	"github.com/keep-network/keep-core/pkg/protocol/group"
	"github.com/keep-network/keep-core/pkg/tbtc/gen/pb"
	"github.com/keep-network/keep-core/pkg/tecdsa"
	"google.golang.org/protobuf/proto"
)

func TestSignerMarshalling(t *testing.T) {
	marshaled := createMockSigner(t)

	unmarshaled := &signer{}

	if err := pbutils.RoundTrip(marshaled, unmarshaled); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(marshaled, unmarshaled) {
		t.Fatal("unexpected content of unmarshaled signer")
	}
}

func TestSignerMarshalling_NonTECDSAKey(t *testing.T) {
	signer := createMockSigner(t)

	p256 := elliptic.P256()

	// Use a non-secp256k1 based key to cause the expected failure.
	signer.wallet.publicKey = &ecdsa.PublicKey{
		Curve: p256,
		X:     p256.Params().Gx,
		Y:     p256.Params().Gy,
	}

	_, err := signer.Marshal()

	testutils.AssertErrorsSame(t, errIncompatiblePublicKey, err)
}

func TestSignerUnmarshalling_InvalidPublicKey(t *testing.T) {
	marshaled, err := proto.Marshal(&pb.Signer{
		Wallet: &pb.Wallet{PublicKey: []byte{0x04}},
	})
	if err != nil {
		t.Fatal(err)
	}

	err = (&signer{}).Unmarshal(marshaled)
	if err == nil {
		t.Fatal("expected an error for a malformed wallet public key")
	}
	if !strings.Contains(err.Error(), "cannot unmarshal wallet public key") {
		t.Fatalf("unexpected error: [%v]", err)
	}
}

func mustUnmarshalPublicKey(t *testing.T, bytes []byte) *ecdsa.PublicKey {
	t.Helper()

	publicKey, err := unmarshalPublicKey(bytes)
	if err != nil {
		t.Fatal(err)
	}

	return publicKey
}

func TestSigningDoneMessage_MarshalingRoundtrip(t *testing.T) {
	msg := &signingDoneMessage{
		senderID:      group.MemberIndex(10),
		message:       big.NewInt(100),
		attemptNumber: 2,
		signature: &tecdsa.Signature{
			R:          big.NewInt(200),
			S:          big.NewInt(300),
			RecoveryID: 3,
		},
		endBlock: 4500,
	}
	unmarshaled := &signingDoneMessage{}

	err := pbutils.RoundTrip(msg, unmarshaled)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(msg, unmarshaled) {
		t.Fatalf("unexpected content of unmarshaled message")
	}
}

func TestFuzzSigningDoneMessage_MarshalingRoundtrip(t *testing.T) {
	for i := 0; i < 10; i++ {
		var (
			senderID      group.MemberIndex
			message       big.Int
			attemptNumber uint64
			signature     tecdsa.Signature
			endBlock      uint64
		)

		f := fuzz.New().NilChance(0.1).
			NumElements(0, 512).
			Funcs(pbutils.FuzzFuncs()...)

		f.Fuzz(&senderID)
		f.Fuzz(&message)
		f.Fuzz(&attemptNumber)
		f.Fuzz(&signature)
		f.Fuzz(&endBlock)

		doneMessage := &signingDoneMessage{
			senderID:      senderID,
			message:       &message,
			attemptNumber: attemptNumber,
			signature:     &signature,
			endBlock:      endBlock,
		}

		_ = pbutils.RoundTrip(doneMessage, &signingDoneMessage{})
	}
}

func TestFuzzSigningDoneMessage_Unmarshaler(t *testing.T) {
	pbutils.FuzzUnmarshaler(&signingDoneMessage{})
}

func TestCoordinationMessage_MarshalingRoundtrip(t *testing.T) {
	parseHash := func(hash string) bitcoin.Hash {
		parsed, err := bitcoin.NewHashFromString(hash, bitcoin.InternalByteOrder)
		if err != nil {
			t.Fatal(err)
		}

		return parsed
	}

	parseScript := func(script string) bitcoin.Script {
		parsed, err := hex.DecodeString(script)
		if err != nil {
			t.Fatal(err)
		}

		return parsed
	}

	toByte20 := func(s string) [20]byte {
		bytes, err := hex.DecodeString(s)
		if err != nil {
			t.Fatal(err)
		}

		if len(bytes) != 20 {
			t.Fatal("incorrect hexstring length")
		}

		var result [20]byte
		copy(result[:], bytes[:])
		return result
	}

	tests := map[string]struct {
		proposal CoordinationProposal
	}{
		"with noop proposal": {
			proposal: &NoopProposal{},
		},
		"with heartbeat proposal": {
			proposal: &HeartbeatProposal{
				Message: [16]byte{0x01, 0x02},
			},
		},
		"with deposit sweep proposal": {
			proposal: &DepositSweepProposal{
				DepositsKeys: []DepositKey{
					{
						FundingTxHash:      parseHash("709b55bd3da0f5a838125bd0ee20c5bfdd7caba173912d4281cae816b79a201b"),
						FundingOutputIndex: 0,
					},
					{
						FundingTxHash:      parseHash("27ca64c092a959c7edc525ed45e845b1de6a7590d173fd2fad9133c8a779a1e3"),
						FundingOutputIndex: 1,
					},
				},
				SweepTxFee: big.NewInt(10000),
				DepositsRevealBlocks: []*big.Int{
					big.NewInt(100),
					big.NewInt(300),
				},
			},
		},
		"with redemption proposal": {
			proposal: &RedemptionProposal{
				RedeemersOutputScripts: []bitcoin.Script{
					parseScript("00148db50eb52063ea9d98b3eac91489a90f738986f6"),
					parseScript("76a9148db50eb52063ea9d98b3eac91489a90f738986f688ac"),
				},
				RedemptionTxFee: big.NewInt(10000),
			},
		},
		"with moving funds proposal": {
			proposal: &MovingFundsProposal{
				TargetWallets: [][20]byte{
					toByte20("cb7d88a87c37aff0c1535fa4efe6f0a2406ea5e9"),
					toByte20("f87eb7ec3b15a3fdd7b57754d765694b3e0b4bf4"),
				},
				MovingFundsTxFee: big.NewInt(10000),
			},
		},
		"with moved funds sweep proposal": {
			proposal: &MovedFundsSweepProposal{
				MovingFundsTxHash:        parseHash("27ca64c092a959c7edc525ed45e845b1de6a7590d173fd2fad9133c8a779a1e3"),
				MovingFundsTxOutputIndex: 3,
				SweepTxFee:               big.NewInt(8000),
			},
		},
	}

	walletPublicKeyHash := toByte20("aa768412ceed10bd423c025542ca90071f9fb62d")

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			msg := &coordinationMessage{
				senderID:            group.MemberIndex(10),
				coordinationBlock:   900,
				walletPublicKeyHash: walletPublicKeyHash,
				proposal:            test.proposal,
			}
			unmarshaled := &coordinationMessage{}

			err := pbutils.RoundTrip(msg, unmarshaled)
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(msg, unmarshaled) {
				t.Fatalf("unexpected content of unmarshaled message")
			}
		})
	}
}

func TestFuzzCoordinationMessage_MarshalingRoundtrip_WithHeartbeatProposal(t *testing.T) {
	for i := 0; i < 10; i++ {
		var (
			senderID            group.MemberIndex
			coordinationBlock   uint64
			walletPublicKeyHash [20]byte
			proposal            HeartbeatProposal
		)

		f := fuzz.New().NilChance(0.1).
			NumElements(0, 512).
			Funcs(pbutils.FuzzFuncs()...)

		f.Fuzz(&senderID)
		f.Fuzz(&coordinationBlock)
		f.Fuzz(&walletPublicKeyHash)
		f.Fuzz(&proposal)

		coordinationMsg := &coordinationMessage{
			senderID:            senderID,
			coordinationBlock:   coordinationBlock,
			walletPublicKeyHash: walletPublicKeyHash,
			proposal:            &proposal,
		}

		_ = pbutils.RoundTrip(coordinationMsg, &coordinationMessage{})
	}
}

func TestFuzzCoordinationMessage_MarshalingRoundtrip_WithDepositSweepProposal(t *testing.T) {
	for i := 0; i < 10; i++ {
		var (
			senderID            group.MemberIndex
			coordinationBlock   uint64
			walletPublicKeyHash [20]byte
			proposal            DepositSweepProposal
		)

		f := fuzz.New().NilChance(0.1).
			NumElements(0, 512).
			Funcs(pbutils.FuzzFuncs()...)

		f.Fuzz(&senderID)
		f.Fuzz(&coordinationBlock)
		f.Fuzz(&walletPublicKeyHash)
		f.Fuzz(&proposal)

		coordinationMsg := &coordinationMessage{
			senderID:            senderID,
			coordinationBlock:   coordinationBlock,
			walletPublicKeyHash: walletPublicKeyHash,
			proposal:            &proposal,
		}

		_ = pbutils.RoundTrip(coordinationMsg, &coordinationMessage{})
	}
}

func TestFuzzCoordinationMessage_MarshalingRoundtrip_WithRedemptionProposal(t *testing.T) {
	for i := 0; i < 10; i++ {
		var (
			senderID            group.MemberIndex
			coordinationBlock   uint64
			walletPublicKeyHash [20]byte
			proposal            RedemptionProposal
		)

		f := fuzz.New().NilChance(0.1).
			NumElements(0, 512).
			Funcs(pbutils.FuzzFuncs()...)

		f.Fuzz(&senderID)
		f.Fuzz(&coordinationBlock)
		f.Fuzz(&walletPublicKeyHash)
		f.Fuzz(&proposal)

		coordinationMsg := &coordinationMessage{
			senderID:            senderID,
			coordinationBlock:   coordinationBlock,
			walletPublicKeyHash: walletPublicKeyHash,
			proposal:            &proposal,
		}

		_ = pbutils.RoundTrip(coordinationMsg, &coordinationMessage{})
	}
}

func TestFuzzCoordinationMessage_MarshalingRoundtrip_WithMovingFundsProposal(t *testing.T) {
	for i := 0; i < 10; i++ {
		var (
			senderID            group.MemberIndex
			coordinationBlock   uint64
			walletPublicKeyHash [20]byte
			proposal            MovingFundsProposal
		)

		f := fuzz.New().NilChance(0.1).
			NumElements(0, 512).
			Funcs(pbutils.FuzzFuncs()...)

		f.Fuzz(&senderID)
		f.Fuzz(&coordinationBlock)
		f.Fuzz(&walletPublicKeyHash)
		f.Fuzz(&proposal)

		coordinationMsg := &coordinationMessage{
			senderID:            senderID,
			coordinationBlock:   coordinationBlock,
			walletPublicKeyHash: walletPublicKeyHash,
			proposal:            &proposal,
		}

		_ = pbutils.RoundTrip(coordinationMsg, &coordinationMessage{})
	}
}

func TestFuzzCoordinationMessage_MarshalingRoundtrip_WithMovedFundsSweepProposal(t *testing.T) {
	for i := 0; i < 10; i++ {
		var (
			senderID            group.MemberIndex
			coordinationBlock   uint64
			walletPublicKeyHash [20]byte
			proposal            MovedFundsSweepProposal
		)

		f := fuzz.New().NilChance(0.1).
			NumElements(0, 512).
			Funcs(pbutils.FuzzFuncs()...)

		f.Fuzz(&senderID)
		f.Fuzz(&coordinationBlock)
		f.Fuzz(&walletPublicKeyHash)
		f.Fuzz(&proposal)

		coordinationMsg := &coordinationMessage{
			senderID:            senderID,
			coordinationBlock:   coordinationBlock,
			walletPublicKeyHash: walletPublicKeyHash,
			proposal:            &proposal,
		}

		_ = pbutils.RoundTrip(coordinationMsg, &coordinationMessage{})
	}
}

func TestFuzzCoordinationMessage_MarshalingRoundtrip_WithNoopProposal(t *testing.T) {
	for i := 0; i < 10; i++ {
		var (
			senderID            group.MemberIndex
			coordinationBlock   uint64
			walletPublicKeyHash [20]byte
			proposal            NoopProposal
		)

		f := fuzz.New().NilChance(0.1).
			NumElements(0, 512).
			Funcs(pbutils.FuzzFuncs()...)

		f.Fuzz(&senderID)
		f.Fuzz(&coordinationBlock)
		f.Fuzz(&walletPublicKeyHash)
		f.Fuzz(&proposal)

		coordinationMsg := &coordinationMessage{
			senderID:            senderID,
			coordinationBlock:   coordinationBlock,
			walletPublicKeyHash: walletPublicKeyHash,
			proposal:            &proposal,
		}

		_ = pbutils.RoundTrip(coordinationMsg, &coordinationMessage{})
	}
}

func TestFuzzCoordinationMessage_Unmarshaler(t *testing.T) {
	pbutils.FuzzUnmarshaler(&coordinationMessage{})
}
