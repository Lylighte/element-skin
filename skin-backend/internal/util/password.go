package util

import (
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

func HashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(b), err
}

func VerifyPassword(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func ValidateStrongPassword(password string) []string {
	var errs []string
	if len([]rune(password)) < 8 {
		errs = append(errs, "min_length")
	}
	var lower, upper, number, special bool
	for _, r := range password {
		switch {
		case unicode.IsLower(r):
			lower = true
		case unicode.IsUpper(r):
			upper = true
		case unicode.IsDigit(r):
			number = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			special = true
		}
	}
	categories := 0
	for _, present := range []bool{lower, upper, number, special} {
		if present {
			categories++
		}
	}
	if categories < 2 {
		errs = append(errs, "composition")
	}
	return errs
}
