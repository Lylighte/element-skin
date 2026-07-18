package util

import (
	"net/mail"
	"strings"
)

func ValidEmail(s string) bool {
	if strings.ContainsAny(s, "\r\n") {
		return false
	}
	addr, err := mail.ParseAddress(s)
	if err != nil || addr.Address != s {
		return false
	}
	parts := strings.Split(addr.Address, "@")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return false
	}
	return strings.Contains(parts[1], ".")
}
