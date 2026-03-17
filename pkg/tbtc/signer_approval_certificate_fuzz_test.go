package tbtc

import "testing"

func FuzzDecodeSignerApprovalCertificateHex_NoPanic(f *testing.F) {
	f.Add("0x")
	f.Add("0x00")
	f.Add("0x" + "11")
	f.Add("deadbeef")
	f.Add("0xzz")

	f.Fuzz(func(t *testing.T, value string) {
		_, _ = decodeSignerApprovalCertificateHex(value, 0)
		_, _ = decodeSignerApprovalCertificateHex(value, 32)
	})
}
