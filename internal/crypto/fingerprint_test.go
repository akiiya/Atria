package crypto

import (
	"testing"
)

func TestFingerprint_Stable(t *testing.T) {
	input := "test_api_hash_value"
	fp1 := Fingerprint(input)
	fp2 := Fingerprint(input)

	if fp1 != fp2 {
		t.Errorf("同一输入指纹应稳定，fp1=%s，fp2=%s", fp1, fp2)
	}
}

func TestFingerprint_DifferentInputs(t *testing.T) {
	fp1 := Fingerprint("value1")
	fp2 := Fingerprint("value2")

	if fp1 == fp2 {
		t.Error("不同输入的指纹不应相同")
	}
}

func TestFingerprint_Length(t *testing.T) {
	fp := Fingerprint("some_value")
	if len(fp) != FingerprintLength {
		t.Errorf("指纹长度应为 %d，实际=%d", FingerprintLength, len(fp))
	}
}

func TestFingerprint_EmptyInput(t *testing.T) {
	fp := Fingerprint("")
	if fp != "" {
		t.Error("空输入应返回空指纹")
	}
}

func TestFingerprint_HexFormat(t *testing.T) {
	fp := Fingerprint("test_value")
	for _, c := range fp {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("指纹应为小写十六进制，发现非法字符: %c", c)
		}
	}
}
