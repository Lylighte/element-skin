package yggdrasil

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"strings"

	"element-skin/backend/internal/config"
)

type Signer struct {
	privateKey *rsa.PrivateKey
	publicPEM  string
}

func NewSigner(cfg config.Config) (*Signer, error) {
	privatePath := strings.TrimSpace(cfg.PrivateKeyPath)
	if privatePath == "" {
		return nil, errors.New("keys.private_key 未配置")
	}
	publicPath := strings.TrimSpace(cfg.PublicKeyPath)
	if publicPath == "" {
		return nil, errors.New("keys.public_key 未配置")
	}

	if err := ensureSigningKeyFiles(privatePath, publicPath); err != nil {
		return nil, err
	}
	privateKey, err := readPrivateKey(privatePath)
	if err != nil {
		return nil, err
	}
	publicPEM, publicKey, err := readPublicKey(publicPath)
	if err != nil {
		return nil, err
	}
	if err := verifyKeyPair(privateKey, publicKey); err != nil {
		return nil, err
	}
	return &Signer{privateKey: privateKey, publicPEM: publicPEM}, nil
}

func (s *Signer) PublicKeyPEM() string {
	if s == nil {
		return ""
	}
	return s.publicPEM
}

func (s *Signer) SignPropertyValue(value string) (string, error) {
	if s == nil || s.privateKey == nil {
		return "", errors.New("yggdrasil signing key is not loaded")
	}
	digest := sha1.Sum([]byte(value))
	signature, err := rsa.SignPKCS1v15(rand.Reader, s.privateKey, crypto.SHA1, digest[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(signature), nil
}
