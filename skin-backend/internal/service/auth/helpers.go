package auth

import (
	"crypto/rand"
	"errors"
	"math/big"

	"element-skin/backend/internal/util"
)

func validEmail(s string) bool {
	return util.ValidEmail(s)
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
