package crypto

import (
	"bytes"
	"testing"
)

func TestEncryptAESGCM_RoundTrip(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	plaintext := []byte("hello, atria!")
	aad := []byte("test-aad")

	// 加密
	ciphertext, err := EncryptAESGCM(key, plaintext, aad)
	if err != nil {
		t.Fatalf("加密失败: %s", err)
	}

	// 密文不应等于明文
	if bytes.Equal(ciphertext, plaintext) {
		t.Error("密文不应等于明文")
	}

	// 解密
	decrypted, err := DecryptAESGCM(key, ciphertext, aad)
	if err != nil {
		t.Fatalf("解密失败: %s", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("解密结果不匹配，期望=%s，实际=%s", plaintext, decrypted)
	}
}

func TestEncryptAESGCM_WrongKey(t *testing.T) {
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	for i := range key1 {
		key1[i] = byte(i)
		key2[i] = byte(i + 1)
	}

	plaintext := []byte("secret data")
	aad := []byte("test-aad")

	ciphertext, err := EncryptAESGCM(key1, plaintext, aad)
	if err != nil {
		t.Fatalf("加密失败: %s", err)
	}

	_, err = DecryptAESGCM(key2, ciphertext, aad)
	if err == nil {
		t.Error("错误密钥解密应该失败")
	}
}

func TestEncryptAESGCM_WrongAAD(t *testing.T) {
	key := make([]byte, 32)
	plaintext := []byte("secret data")

	ciphertext, err := EncryptAESGCM(key, plaintext, []byte("aad1"))
	if err != nil {
		t.Fatalf("加密失败: %s", err)
	}

	_, err = DecryptAESGCM(key, ciphertext, []byte("aad2"))
	if err == nil {
		t.Error("错误 AAD 解密应该失败")
	}
}

func TestEncryptAESGCM_ShortCiphertext(t *testing.T) {
	key := make([]byte, 32)
	_, err := DecryptAESGCM(key, []byte("short"), nil)
	if err == nil {
		t.Error("过短密文解密应该失败")
	}
}

func TestEncryptString_EmptyString(t *testing.T) {
	key := make([]byte, 32)
	result, err := EncryptString(key, "", nil)
	if err != nil {
		t.Fatalf("加密空字符串不应报错: %s", err)
	}
	if result != "" {
		t.Error("加密空字符串应返回空字符串")
	}
}

func TestDecryptString_EmptyString(t *testing.T) {
	key := make([]byte, 32)
	result, err := DecryptString(key, "", nil)
	if err != nil {
		t.Fatalf("解密空字符串不应报错: %s", err)
	}
	if result != "" {
		t.Error("解密空字符串应返回空字符串")
	}
}

func TestEncryptString_RoundTrip(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	plaintext := "api_hash_example_value"
	aad := []byte("atria:api_hash:v1")

	encrypted, err := EncryptString(key, plaintext, aad)
	if err != nil {
		t.Fatalf("加密失败: %s", err)
	}

	if encrypted == plaintext {
		t.Error("加密结果不应等于明文")
	}

	decrypted, err := DecryptString(key, encrypted, aad)
	if err != nil {
		t.Fatalf("解密失败: %s", err)
	}

	if decrypted != plaintext {
		t.Errorf("解密结果不匹配，期望=%s，实际=%s", plaintext, decrypted)
	}
}
