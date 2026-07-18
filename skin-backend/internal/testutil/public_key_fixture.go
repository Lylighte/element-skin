package testutil

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"testing"
)

type PublicKeyFixture struct {
	DERBase64 string
	PEM       string
}

func NewPublicKeyFixture(t testing.TB) PublicKeyFixture {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	return PublicKeyFixture{
		DERBase64: base64.StdEncoding.EncodeToString(der),
		PEM:       string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})),
	}
}
