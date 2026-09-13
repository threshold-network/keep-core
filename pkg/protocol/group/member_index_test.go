package group

import (
	"fmt"
	"math"
	"testing"
)

func TestMemberIndexFromUint32(t *testing.T) {
	for _, value := range []uint32{0, 1, 255, 256, math.MaxUint32} {
		t.Run(fmt.Sprint(value), func(t *testing.T) {
			index, err := MemberIndexFromUint32(value)
			if value > MaxMemberIndex {
				if err == nil {
					t.Fatal("expected out-of-range index to be rejected")
				}
				return
			}
			if err != nil || uint32(index) != value {
				t.Fatalf("index %d: got %d, %v", value, index, err)
			}
		})
	}
}
