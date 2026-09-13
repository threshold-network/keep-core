package gjkr

import (
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/keep-network/keep-core/pkg/beacon/gjkr/gen/pb"
)

func TestAccusationsUnmarshalValidationErrors(t *testing.T) {
	for _, keys := range []map[uint32][]byte{{256: {1}}, {1: nil}} {
		cases := []struct {
			message   proto.Message
			unmarshal func([]byte) error
		}{
			{
				message:   &pb.SecretSharesAccusations{SenderID: 1, AccusedMembersKeys: keys, SessionID: "session"},
				unmarshal: new(SecretSharesAccusationsMessage).Unmarshal,
			},
			{
				message:   &pb.PointsAccusations{SenderID: 1, AccusedMembersKeys: keys, SessionID: "session"},
				unmarshal: new(PointsAccusationsMessage).Unmarshal,
			},
		}
		for _, tc := range cases {
			data, err := proto.Marshal(tc.message)
			if err != nil {
				t.Fatal(err)
			}
			if err := tc.unmarshal(data); err == nil {
				t.Fatalf("%T accepted invalid accusation keys", tc.message)
			}
		}
	}
}
