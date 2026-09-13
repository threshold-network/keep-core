package bitcoin

import (
	"bytes"
	"math"
	"testing"
)

func TestSignedVersionWireCompatibility(t *testing.T) {
	for _, tc := range []struct {
		version int32
		bytes   []byte
	}{
		{0, []byte{0, 0, 0, 0}},
		{-1, []byte{255, 255, 255, 255}},
		{math.MinInt32, []byte{0, 0, 0, 128}},
		{math.MaxInt32, []byte{255, 255, 255, 127}},
	} {
		header := (&BlockHeader{Version: tc.version}).Serialize()
		version := (&Transaction{Version: tc.version}).SerializeVersion()
		if !bytes.Equal(header[:4], tc.bytes) || !bytes.Equal(version[:], tc.bytes) {
			t.Fatalf("version %d changed wire encoding", tc.version)
		}
		decoded := new(BlockHeader)
		decoded.Deserialize(header)
		if decoded.Version != tc.version {
			t.Fatalf("version %d decoded as %d", tc.version, decoded.Version)
		}
	}
}

func TestTransactionFeeEstimatorOverflow(t *testing.T) {
	chain := newLocalChain()
	for _, tc := range []struct{ rate, size int64 }{
		{math.MaxInt64, 2}, {1<<62 + 1, 4}, {-1, 1}, {1, -1},
	} {
		chain.setSatPerVByteFee(tc.rate)
		if fee, err := NewTransactionFeeEstimator(chain).EstimateFee(tc.size); err == nil {
			t.Fatalf("accepted rate %d, size %d: %d", tc.rate, tc.size, fee)
		}
	}
	chain.setSatPerVByteFee(math.MaxInt64)
	if fee, err := NewTransactionFeeEstimator(chain).EstimateFee(1); err != nil || fee != math.MaxInt64 {
		t.Fatalf("valid boundary: %d, %v", fee, err)
	}
}
