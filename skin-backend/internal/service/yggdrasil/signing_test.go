package yggdrasil_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
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

func TestNewSignerGeneratesMissingKeyPair(t *testing.T) {
	dir := t.TempDir()
	cfg := testutil.TestConfig()
	cfg.PrivateKeyPath = filepath.Join(dir, "private.pem")
	cfg.PublicKeyPath = filepath.Join(dir, "public.pem")

	signer, err := yggdrasil.NewSigner(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if signer == nil {
		t.Fatal("generated signer is nil")
	}
	if _, err := os.Stat(cfg.PrivateKeyPath); err != nil {
		t.Fatalf("private key should be generated: %v", err)
	}
	if _, err := os.Stat(cfg.PublicKeyPath); err != nil {
		t.Fatalf("public key should be generated: %v", err)
	}
	if signature, err := signer.SignPropertyValue("value"); signature == "" || err != nil {
		t.Fatalf("generated signer should sign values: signature=%q err=%v", signature, err)
	}
}

func TestNewSignerRegeneratesMissingPublicKeyFromPrivateKey(t *testing.T) {
	dir := t.TempDir()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}

	cfg := testutil.TestConfig()
	cfg.PrivateKeyPath = filepath.Join(dir, "private.pem")
	cfg.PublicKeyPath = filepath.Join(dir, "public.pem")
	if err := os.WriteFile(cfg.PrivateKeyPath, pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateDER,
	}), 0o600); err != nil {
		t.Fatal(err)
	}

	signer, err := yggdrasil.NewSigner(cfg)
	if err != nil {
		t.Fatal(err)
	}
	publicPEM, err := os.ReadFile(cfg.PublicKeyPath)
	if err != nil {
		t.Fatalf("public key should be regenerated: %v", err)
	}
	if signer.PublicKeyPEM() != string(publicPEM) {
		t.Fatal("signer should use regenerated public key")
	}
}

func TestNewSignerAcceptsPKCS1PrivateKeyExactly(t *testing.T) {
	dir := t.TempDir()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	privatePEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})
	publicDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	publicPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER})

	cfg := testutil.TestConfig()
	cfg.PrivateKeyPath = filepath.Join(dir, "pkcs1-private.pem")
	cfg.PublicKeyPath = filepath.Join(dir, "pkcs1-public.pem")
	if err := os.WriteFile(cfg.PrivateKeyPath, privatePEM, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg.PublicKeyPath, publicPEM, 0o644); err != nil {
		t.Fatal(err)
	}

	signer, err := yggdrasil.NewSigner(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if signer.PublicKeyPEM() != string(publicPEM) {
		t.Fatalf("PKCS#1 signer public key mismatch: got=%q want=%q", signer.PublicKeyPEM(), string(publicPEM))
	}
	signature, err := signer.SignPropertyValue("pkcs1-value")
	if err != nil || signature == "" {
		t.Fatalf("PKCS#1 signer should produce signature: signature=%q err=%v", signature, err)
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

func TestNewSignerRejectsNonRSAAndUnparseableKeysExactly(t *testing.T) {
	dir := t.TempDir()
	ecdsaKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ecdsaPrivateDER, err := x509.MarshalPKCS8PrivateKey(ecdsaKey)
	if err != nil {
		t.Fatal(err)
	}
	ecdsaPublicDER, err := x509.MarshalPKIXPublicKey(&ecdsaKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	rsaPrivateDER, err := x509.MarshalPKCS8PrivateKey(rsaKey)
	if err != nil {
		t.Fatal(err)
	}

	nonRSAPrivate := filepath.Join(dir, "ecdsa-private.pem")
	if err := os.WriteFile(nonRSAPrivate, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: ecdsaPrivateDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	nonRSAPublic := filepath.Join(dir, "ecdsa-public.pem")
	if err := os.WriteFile(nonRSAPublic, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: ecdsaPublicDER}), 0o644); err != nil {
		t.Fatal(err)
	}
	validPrivate := filepath.Join(dir, "rsa-private.pem")
	if err := os.WriteFile(validPrivate, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: rsaPrivateDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	invalidParsePrivate := filepath.Join(dir, "bad-private.pem")
	if err := os.WriteFile(invalidParsePrivate, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("bad")}), 0o600); err != nil {
		t.Fatal(err)
	}
	invalidParsePublic := filepath.Join(dir, "bad-public.pem")
	if err := os.WriteFile(invalidParsePublic, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: []byte("bad")}), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name       string
		privateKey string
		publicKey  string
		want       string
	}{
		{
			name:       "non RSA private",
			privateKey: nonRSAPrivate,
			publicKey:  nonRSAPublic,
			want:       "Yggdrasil 私钥不是 RSA 密钥: " + nonRSAPrivate,
		},
		{
			name:       "unparseable private",
			privateKey: invalidParsePrivate,
			publicKey:  nonRSAPublic,
			want:       "无法解析 Yggdrasil RSA 私钥: " + invalidParsePrivate,
		},
		{
			name:       "non RSA public",
			privateKey: validPrivate,
			publicKey:  nonRSAPublic,
			want:       "Yggdrasil 公钥不是 RSA 密钥: " + nonRSAPublic,
		},
		{
			name:       "unparseable public",
			privateKey: validPrivate,
			publicKey:  invalidParsePublic,
			want:       "无法解析 Yggdrasil 公钥: " + invalidParsePublic,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testutil.TestConfig()
			cfg.PrivateKeyPath = tc.privateKey
			cfg.PublicKeyPath = tc.publicKey
			signer, err := yggdrasil.NewSigner(cfg)
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
