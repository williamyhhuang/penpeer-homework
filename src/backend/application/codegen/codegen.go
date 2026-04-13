package codegen

import (
	"crypto/rand"
	"math/big"
)

const (
	// base62 字符集：0-9 + a-z + A-Z，短碼不易混淆且 URL 安全
	charset  = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	codeLen  = 7 // 62^7 ≈ 35 億，足夠一般使用規模
)

// GenerateCode 使用 crypto/rand 產生密碼學安全的短碼
// 避免使用 math/rand，防止短碼可被預測
func GenerateCode() (string, error) {
	result := make([]byte, codeLen)
	charsetLen := big.NewInt(int64(len(charset)))

	for i := range result {
		idx, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			return "", err
		}
		result[i] = charset[idx.Int64()]
	}
	return string(result), nil
}
