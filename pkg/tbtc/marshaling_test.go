package tbtc

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"reflect"
	"strings"
	"testing"

	fuzz "github.com/google/gofuzz"
	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/internal/pbutils"
	"github.com/keep-network/keep-core/pkg/protocol/group"
	"github.com/keep-network/keep-core/pkg/tbtc/gen/pb"
	"github.com/keep-network/keep-core/pkg/tecdsa"
	"google.golang.org/protobuf/proto"
)

func TestValidateMemberIndex(t *testing.T) {
	tests := map[string]struct {
		protoIndex uint32
		wantErr    bool
	}{
		"valid index 1": {
			protoIndex: 1,
			wantErr:    false,
		},
		"valid index 255": {
			protoIndex: 255,
			wantErr:    false,
		},
		"invalid index 0": {
			protoIndex: 0,
			wantErr:    true,
		},
		"invalid index 256": {
			protoIndex: 256,
			wantErr:    true,
		},
		"invalid index 300": {
			protoIndex: 300,
			wantErr:    true,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			err := validateMemberIndex(test.protoIndex)
			if (err != nil) != test.wantErr {
				t.Errorf("validateMemberIndex() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}
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
				DepositsKeys: []struct {
					FundingTxHash      bitcoin.Hash
					FundingOutputIndex uint32
				}{
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

func FuzzReservationAnchorProposal_Unmarshal(f *testing.F) {
	proposal := &ReservationAnchorProposal{
		DepositFundingTxHash:      bitcoin.Hash{0x01},
		DepositFundingOutputIndex: 1,
		RequestNonce:              1,
		AnchorTxFee:               big.NewInt(1000),
	}
	bytes, _ := proposal.Marshal()
	f.Add(bytes)
	f.Fuzz(func(t *testing.T, data []byte) {
		_ = (&ReservationAnchorProposal{}).Unmarshal(data)
	})
}

func FuzzReservedRedemptionProposal_Unmarshal(f *testing.F) {
	proposal := &ReservedRedemptionProposal{
		ReservationKey:       big.NewInt(12345),
		RequestNonce:         1,
		RedeemerOutputScript: bitcoin.Script{0x01},
		RedemptionTxFee:      big.NewInt(1000),
	}
	bytes, _ := proposal.Marshal()
	f.Add(bytes)
	f.Fuzz(func(t *testing.T, data []byte) {
		_ = (&ReservedRedemptionProposal{}).Unmarshal(data)
	})
}

func FuzzReservationReanchorProposal_Unmarshal(f *testing.F) {
	proposal := &ReservationReanchorProposal{
		ReservationKey:            big.NewInt(12345),
		RequestNonce:              1,
		TargetWalletPublicKeyHash: [20]byte{0x01},
		ReanchorTxFee:             big.NewInt(1000),
	}
	bytes, _ := proposal.Marshal()
	f.Add(bytes)
	f.Fuzz(func(t *testing.T, data []byte) {
		_ = (&ReservationReanchorProposal{}).Unmarshal(data)
	})
}

func FuzzReservationDissolutionProposal_Unmarshal(f *testing.F) {
	proposal := &ReservationDissolutionProposal{
		ReservationKey:   big.NewInt(12345),
		RequestNonce:     1,
		DissolutionTxFee: big.NewInt(1000),
	}
	bytes, _ := proposal.Marshal()
	f.Add(bytes)
	f.Fuzz(func(t *testing.T, data []byte) {
		_ = (&ReservationDissolutionProposal{}).Unmarshal(data)
	})
}
func TestReservationAnchorProposal_UnmarshalRejectsMalformedInput(t *testing.T) {
	t.Run("truncated json", func(t *testing.T) {
		if err := (&ReservationAnchorProposal{}).Unmarshal([]byte(`{"depositFundingTxHash":`)); err == nil {
			t.Fatal("expected error for truncated json")
		}
	})

	t.Run("zero request nonce", func(t *testing.T) {
		data, _ := json.Marshal(ReservationAnchorProposal{
			DepositFundingTxHash: bitcoin.Hash{0x01},
			RequestNonce:         0,
			AnchorTxFee:          big.NewInt(100),
		})
		if err := (&ReservationAnchorProposal{}).Unmarshal(data); err == nil {
			t.Fatal("expected error for zero request nonce")
		}
	})

	t.Run("negative fee", func(t *testing.T) {
		data, _ := json.Marshal(ReservationAnchorProposal{
			DepositFundingTxHash: bitcoin.Hash{0x01},
			RequestNonce:         1,
			AnchorTxFee:          big.NewInt(-100),
		})
		if err := (&ReservationAnchorProposal{}).Unmarshal(data); err == nil {
			t.Fatal("expected error for negative fee")
		}
	})
	t.Run("oversized fee", func(t *testing.T) {
		data, _ := json.Marshal(ReservationAnchorProposal{
			DepositFundingTxHash: bitcoin.Hash{0x01},
			RequestNonce:         1,
			AnchorTxFee:          new(big.Int).Lsh(big.NewInt(1), 64),
		})
		if err := (&ReservationAnchorProposal{}).Unmarshal(data); err == nil {
			t.Fatal("expected error for oversized fee")
		}
	})

	t.Run("wrong-length deposit funding transaction hash", func(t *testing.T) {
		// Real wire format (see Marshal, which json.Marshals the [32]byte
		// field directly): a JSON array of byte values, not a base64
		// string. Length 3 instead of 32 must be rejected explicitly,
		// not silently zero-filled the way [32]byte unmarshal would.
		data, err := json.Marshal(map[string]interface{}{
			"depositFundingTxHash":      []int{0x01, 0x02, 0x03},
			"depositFundingOutputIndex": 0,
			"requestNonce":              1,
			"anchorTxFee":               100,
		})
		if err != nil {
			t.Fatal(err)
		}
		err = (&ReservationAnchorProposal{}).Unmarshal(data)
		if err == nil || err.Error() != "invalid deposit funding transaction hash length: [3]" {
			t.Fatalf("expected wrong-length error, got [%v]", err)
		}
	})

	t.Run("full-length deposit funding transaction hash round-trips", func(t *testing.T) {
		data, err := (&ReservationAnchorProposal{
			DepositFundingTxHash: bitcoin.Hash{0x01},
			RequestNonce:         1,
			AnchorTxFee:          big.NewInt(100),
		}).Marshal()
		if err != nil {
			t.Fatal(err)
		}
		var proposal ReservationAnchorProposal
		if err := proposal.Unmarshal(data); err != nil {
			t.Fatalf("expected no error, got [%v]", err)
		}
		if proposal.DepositFundingTxHash != (bitcoin.Hash{0x01}) {
			t.Fatalf("unexpected hash: [%v]", proposal.DepositFundingTxHash)
		}
	})
}

func TestReservedRedemptionProposal_UnmarshalRejectsMalformedInput(t *testing.T) {
	t.Run("truncated json", func(t *testing.T) {
		if err := (&ReservedRedemptionProposal{}).Unmarshal([]byte(`{"reservationKey":`)); err == nil {
			t.Fatal("expected error for truncated json")
		}
	})

	t.Run("zero request nonce", func(t *testing.T) {
		data, _ := json.Marshal(ReservedRedemptionProposal{
			ReservationKey:  big.NewInt(12345),
			RequestNonce:    0,
			RedemptionTxFee: big.NewInt(100),
		})
		if err := (&ReservedRedemptionProposal{}).Unmarshal(data); err == nil {
			t.Fatal("expected error for zero request nonce")
		}
	})

	t.Run("negative fee", func(t *testing.T) {
		data, _ := json.Marshal(ReservedRedemptionProposal{
			ReservationKey:  big.NewInt(12345),
			RequestNonce:    1,
			RedemptionTxFee: big.NewInt(-100),
		})
		if err := (&ReservedRedemptionProposal{}).Unmarshal(data); err == nil {
			t.Fatal("expected error for negative fee")
		}
	})
	t.Run("negative reservation key", func(t *testing.T) {
		data, _ := json.Marshal(ReservedRedemptionProposal{
			ReservationKey:  big.NewInt(-1),
			RequestNonce:    1,
			RedemptionTxFee: big.NewInt(100),
		})
		if err := (&ReservedRedemptionProposal{}).Unmarshal(data); err == nil {
			t.Fatal("expected error for negative reservation key")
		}
	})

	t.Run("oversized reservation key", func(t *testing.T) {
		data, _ := json.Marshal(ReservedRedemptionProposal{
			ReservationKey:  new(big.Int).Lsh(big.NewInt(1), 256), // > 256 bits
			RequestNonce:    1,
			RedemptionTxFee: big.NewInt(100),
		})
		if err := (&ReservedRedemptionProposal{}).Unmarshal(data); err == nil {
			t.Fatal("expected error for oversized reservation key")
		}
	})
	t.Run("oversized fee", func(t *testing.T) {
		data, _ := json.Marshal(ReservedRedemptionProposal{
			ReservationKey:  big.NewInt(12345),
			RequestNonce:    1,
			RedemptionTxFee: new(big.Int).Lsh(big.NewInt(1), 64),
		})
		if err := (&ReservedRedemptionProposal{}).Unmarshal(data); err == nil {
			t.Fatal("expected error for oversized fee")
		}
	})
}

func TestReservationReanchorProposal_UnmarshalRejectsMalformedInput(t *testing.T) {
	t.Run("truncated json", func(t *testing.T) {
		if err := (&ReservationReanchorProposal{}).Unmarshal([]byte(`{"reservationKey":`)); err == nil {
			t.Fatal("expected error for truncated json")
		}
	})

	t.Run("zero request nonce", func(t *testing.T) {
		data, _ := json.Marshal(ReservationReanchorProposal{
			ReservationKey: big.NewInt(12345),
			RequestNonce:   0,
			ReanchorTxFee:  big.NewInt(100),
		})
		if err := (&ReservationReanchorProposal{}).Unmarshal(data); err == nil {
			t.Fatal("expected error for zero request nonce")
		}
	})

	t.Run("negative fee", func(t *testing.T) {
		data, _ := json.Marshal(ReservationReanchorProposal{
			ReservationKey: big.NewInt(12345),
			RequestNonce:   1,
			ReanchorTxFee:  big.NewInt(-100),
		})
		if err := (&ReservationReanchorProposal{}).Unmarshal(data); err == nil {
			t.Fatal("expected error for negative fee")
		}
	})
	t.Run("negative reservation key", func(t *testing.T) {
		data, _ := json.Marshal(ReservationReanchorProposal{
			ReservationKey: big.NewInt(-1),
			RequestNonce:   1,
			ReanchorTxFee:  big.NewInt(100),
		})
		if err := (&ReservationReanchorProposal{}).Unmarshal(data); err == nil {
			t.Fatal("expected error for negative reservation key")
		}
	})

	t.Run("oversized reservation key", func(t *testing.T) {
		data, _ := json.Marshal(ReservationReanchorProposal{
			ReservationKey: new(big.Int).Lsh(big.NewInt(1), 256), // > 256 bits
			RequestNonce:   1,
			ReanchorTxFee:  big.NewInt(100),
		})
		if err := (&ReservationReanchorProposal{}).Unmarshal(data); err == nil {
			t.Fatal("expected error for oversized reservation key")
		}
	})
	t.Run("oversized fee", func(t *testing.T) {
		data, _ := json.Marshal(ReservationReanchorProposal{
			ReservationKey: big.NewInt(12345),
			RequestNonce:   1,
			ReanchorTxFee:  new(big.Int).Lsh(big.NewInt(1), 64),
		})
		if err := (&ReservationReanchorProposal{}).Unmarshal(data); err == nil {
			t.Fatal("expected error for oversized fee")
		}
	})

	t.Run("wrong-length target wallet public key hash", func(t *testing.T) {
		data, err := json.Marshal(map[string]interface{}{
			"reservationKey":            12345,
			"requestNonce":              1,
			"targetWalletPublicKeyHash": []int{0x01, 0x02, 0x03},
			"reanchorTxFee":             100,
		})
		if err != nil {
			t.Fatal(err)
		}
		err = (&ReservationReanchorProposal{}).Unmarshal(data)
		if err == nil || err.Error() != "invalid target wallet public key hash length: [3]" {
			t.Fatalf("expected wrong-length error, got [%v]", err)
		}
	})

	t.Run("full-length target wallet public key hash round-trips", func(t *testing.T) {
		data, err := (&ReservationReanchorProposal{
			ReservationKey:            big.NewInt(12345),
			RequestNonce:              1,
			TargetWalletPublicKeyHash: [20]byte{0x01},
			ReanchorTxFee:             big.NewInt(100),
		}).Marshal()
		if err != nil {
			t.Fatal(err)
		}
		var proposal ReservationReanchorProposal
		if err := proposal.Unmarshal(data); err != nil {
			t.Fatalf("expected no error, got [%v]", err)
		}
		if proposal.TargetWalletPublicKeyHash != ([20]byte{0x01}) {
			t.Fatalf("unexpected hash: [%v]", proposal.TargetWalletPublicKeyHash)
		}
	})
}

func TestReservationDissolutionProposal_UnmarshalRejectsMalformedInput(t *testing.T) {
	t.Run("truncated json", func(t *testing.T) {
		if err := (&ReservationDissolutionProposal{}).Unmarshal([]byte(`{"reservationKey":`)); err == nil {
			t.Fatal("expected error for truncated json")
		}
	})

	t.Run("zero request nonce", func(t *testing.T) {
		data, _ := json.Marshal(ReservationDissolutionProposal{
			ReservationKey:   big.NewInt(12345),
			RequestNonce:     0,
			DissolutionTxFee: big.NewInt(100),
		})
		if err := (&ReservationDissolutionProposal{}).Unmarshal(data); err == nil {
			t.Fatal("expected error for zero request nonce")
		}
	})

	t.Run("negative fee", func(t *testing.T) {
		data, _ := json.Marshal(ReservationDissolutionProposal{
			ReservationKey:   big.NewInt(12345),
			RequestNonce:     1,
			DissolutionTxFee: big.NewInt(-100),
		})
		if err := (&ReservationDissolutionProposal{}).Unmarshal(data); err == nil {
			t.Fatal("expected error for negative fee")
		}
	})
	t.Run("negative reservation key", func(t *testing.T) {
		data, _ := json.Marshal(ReservationDissolutionProposal{
			ReservationKey:   big.NewInt(-1),
			RequestNonce:     1,
			DissolutionTxFee: big.NewInt(100),
		})
		if err := (&ReservationDissolutionProposal{}).Unmarshal(data); err == nil {
			t.Fatal("expected error for negative reservation key")
		}
	})

	t.Run("oversized reservation key", func(t *testing.T) {
		data, _ := json.Marshal(ReservationDissolutionProposal{
			ReservationKey:   new(big.Int).Lsh(big.NewInt(1), 256), // > 256 bits
			RequestNonce:     1,
			DissolutionTxFee: big.NewInt(100),
		})
		if err := (&ReservationDissolutionProposal{}).Unmarshal(data); err == nil {
			t.Fatal("expected error for oversized reservation key")
		}
	})
	t.Run("oversized fee", func(t *testing.T) {
		data, _ := json.Marshal(ReservationDissolutionProposal{
			ReservationKey:   big.NewInt(12345),
			RequestNonce:     1,
			DissolutionTxFee: new(big.Int).Lsh(big.NewInt(1), 64),
		})
		if err := (&ReservationDissolutionProposal{}).Unmarshal(data); err == nil {
			t.Fatal("expected error for oversized fee")
		}
	})
}
