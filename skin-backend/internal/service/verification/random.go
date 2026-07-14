package verification

import (
	"crypto/rand"
	"errors"
	"math/big"
)

const verificationAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func randomCode(length int) (string, error) {
	if length <= 0 {
		return "", errors.New("verification code length must be positive")
	}
	code := make([]byte, length)
	max := big.NewInt(int64(len(verificationAlphabet)))
	for i := range code {
		value, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		code[i] = verificationAlphabet[value.Int64()]
	}
	return string(code), nil
}
