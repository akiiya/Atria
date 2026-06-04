// Package security 提供敏感数据的加密辅助函数。
// 包括 API Hash、手机号、Session 数据的加密/解密。
package security

import (
	"github.com/user/atria/internal/crypto"
)

// AAD 常量，用于区分不同类型的加密数据。
var (
	aadAPIHash = []byte("atria:api_hash:v1")
	aadPhone   = []byte("atria:phone:v1")
	aadSession = []byte("atria:session:v1")
)

// EncryptAPIHash 加密 API Hash 并返回密文和指纹。
//
// 返回值：
//   - encrypted: base64 编码的密文
//   - fingerprint: 短指纹（用于脱敏显示）
//   - err: 加密错误
//
// 安全要求：不记录明文 api_hash。
func EncryptAPIHash(key []byte, apiHash string) (encrypted string, fingerprint string, err error) {
	encrypted, err = crypto.EncryptString(key, apiHash, aadAPIHash)
	if err != nil {
		return "", "", err
	}
	fingerprint = crypto.Fingerprint(apiHash)
	return encrypted, fingerprint, nil
}

// DecryptAPIHash 解密 API Hash。
func DecryptAPIHash(key []byte, encrypted string) (string, error) {
	return crypto.DecryptString(key, encrypted, aadAPIHash)
}

// EncryptPhone 加密手机号并返回密文和指纹。
//
// 返回值：
//   - encrypted: base64 编码的密文
//   - fingerprint: 短指纹（用于脱敏显示）
//   - err: 加密错误
//
// 安全要求：不记录明文 phone。
func EncryptPhone(key []byte, phone string) (encrypted string, fingerprint string, err error) {
	encrypted, err = crypto.EncryptString(key, phone, aadPhone)
	if err != nil {
		return "", "", err
	}
	fingerprint = crypto.Fingerprint(phone)
	return encrypted, fingerprint, nil
}

// DecryptPhone 解密手机号。
func DecryptPhone(key []byte, encrypted string) (string, error) {
	return crypto.DecryptString(key, encrypted, aadPhone)
}

// EncryptSessionData 加密 Session 文件数据。
//
// 安全要求：不记录明文 session 数据。
func EncryptSessionData(key []byte, data []byte) ([]byte, error) {
	return crypto.EncryptAESGCM(key, data, aadSession)
}

// DecryptSessionData 解密 Session 文件数据。
func DecryptSessionData(key []byte, encrypted []byte) ([]byte, error) {
	return crypto.DecryptAESGCM(key, encrypted, aadSession)
}
