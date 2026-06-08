package util

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"io"
	"strings"
)

func GenerateUUIDNoDash() (string, error) {
	return uuidNoDashFromReader(rand.Reader)
}

func uuidNoDashFromReader(r io.Reader) (string, error) {
	var b [16]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return hex.EncodeToString(b[:]), nil
}

func OfflineUUIDNoDash(name string) string {
	h := md5.Sum([]byte("OfflinePlayer:" + name))
	b := h[:]
	b[6] = (b[6] & 0x0f) | 0x30
	b[8] = (b[8] & 0x3f) | 0x80
	return hex.EncodeToString(b)
}

func StripUUIDDashes(id string) string {
	return strings.ReplaceAll(id, "-", "")
}
