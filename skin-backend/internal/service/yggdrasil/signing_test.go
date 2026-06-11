package yggdrasil_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/testutil"
)

func TestNewSignerRejectsMissingKeyFiles(t *testing.T) {
	cfg := testutil.TestConfig()
	cfg.PrivateKeyPath = filepath.Join(t.TempDir(), "missing-private.pem")

	if _, err := yggdrasil.NewSigner(cfg); err == nil || !strings.Contains(err.Error(), "私钥") {
		t.Fatalf("missing private key should fail clearly, got %v", err)
	}
}

func TestNewSignerRejectsMalformedAndMissingKeyConfigurationExactly(t *testing.T) {
	valid := testutil.TestConfig()
	dir := t.TempDir()
	malformedPrivate := filepath.Join(dir, "malformed-private.pem")
	if err := os.WriteFile(malformedPrivate, []byte("not pem"), 0o600); err != nil {
		t.Fatal(err)
	}
	malformedPublic := filepath.Join(dir, "malformed-public.pem")
	if err := os.WriteFile(malformedPublic, []byte("not pem"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name string
		cfg  func()
		want string
	}{
		{
			name: "private path omitted",
			cfg: func() {
				valid.PrivateKeyPath = ""
			},
			want: "keys.private_key 未配置",
		},
		{
			name: "private PEM malformed",
			cfg: func() {
				valid.PrivateKeyPath = malformedPrivate
			},
			want: "Yggdrasil 私钥不是 PEM 格式: " + malformedPrivate,
		},
		{
			name: "public path omitted",
			cfg: func() {
				valid = testutil.TestConfig()
				valid.PublicKeyPath = ""
			},
			want: "keys.public_key 未配置",
		},
		{
			name: "public PEM malformed",
			cfg: func() {
				valid = testutil.TestConfig()
				valid.PublicKeyPath = malformedPublic
			},
			want: "Yggdrasil 公钥不是 PEM 格式: " + malformedPublic,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.cfg()
			signer, err := yggdrasil.NewSigner(valid)
			if signer != nil || err == nil || err.Error() != tc.want {
				t.Fatalf("NewSigner()=%#v, %v; want nil and %q", signer, err, tc.want)
			}
		})
	}
}

func TestNewSignerRejectsMismatchedKeyPairExactly(t *testing.T) {
	cfg := testutil.TestConfig()
	otherPrivate, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&otherPrivate.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	cfg.PublicKeyPath = filepath.Join(t.TempDir(), "other-public.pem")
	if err := os.WriteFile(cfg.PublicKeyPath, pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicDER,
	}), 0o644); err != nil {
		t.Fatal(err)
	}

	signer, err := yggdrasil.NewSigner(cfg)
	if signer != nil || err == nil || err.Error() != "Yggdrasil 公钥与私钥不匹配" {
		t.Fatalf("mismatched NewSigner()=%#v, %v; want nil and exact mismatch error", signer, err)
	}
}

func TestNilSignerMethodsAreSafeAndExact(t *testing.T) {
	var signer *yggdrasil.Signer
	if got := signer.PublicKeyPEM(); got != "" {
		t.Fatalf("nil signer public key=%q; want empty string", got)
	}
	signature, err := signer.SignPropertyValue("value")
	if signature != "" || err == nil || err.Error() != "yggdrasil signing key is not loaded" {
		t.Fatalf("nil signer signature=%q err=%v; want empty signature and exact error", signature, err)
	}
}
