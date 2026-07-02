package site

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math/big"

	"element-skin/backend/internal/util"
)

func validEmail(s string) bool {
	return util.ValidEmail(s)
}

func TextureHashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func strconvI(i int) string {
	if i == 0 {
		return "0"
	}
	digits := "0123456789"
	var out []byte
	for i > 0 {
		out = append([]byte{digits[i%10]}, out...)
		i /= 10
	}
	return string(out)
}

func randomVerificationCode(length int) (string, error) {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	out := make([]byte, length)
	max := big.NewInt(int64(len(alphabet)))
	for i := range out {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		out[i] = alphabet[n.Int64()]
	}
	return string(out), nil
}

var ErrUnauthorized = errors.New("unauthorized")

func asCursorMap(v any) map[string]any {
	if v == nil {
		return nil
	}
	m, _ := v.(map[string]any)
	return m
}
