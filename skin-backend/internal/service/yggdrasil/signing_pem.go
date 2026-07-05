package yggdrasil

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strings"
)

func marshalPrivateKeyPEM(privateKey *rsa.PrivateKey) ([]byte, error) {
	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("编码 Yggdrasil 私钥失败: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), nil
}

func marshalPublicKeyPEM(publicKey *rsa.PublicKey) ([]byte, error) {
	der, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return nil, fmt.Errorf("编码 Yggdrasil 公钥失败: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), nil
}

func readPrivateKey(path string) (*rsa.PrivateKey, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("keys.private_key 未配置")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取 Yggdrasil 私钥失败 %q: %w", path, err)
	}
	block, _ := pem.Decode(b)
	if block == nil {
		return nil, fmt.Errorf("Yggdrasil 私钥不是 PEM 格式: %s", path)
	}
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if rsaKey, ok := key.(*rsa.PrivateKey); ok {
			return rsaKey, nil
		}
		return nil, fmt.Errorf("Yggdrasil 私钥不是 RSA 密钥: %s", path)
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	return nil, fmt.Errorf("无法解析 Yggdrasil RSA 私钥: %s", path)
}

func readPublicKey(path string) (string, *rsa.PublicKey, error) {
	if strings.TrimSpace(path) == "" {
		return "", nil, errors.New("keys.public_key 未配置")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", nil, fmt.Errorf("读取 Yggdrasil 公钥失败 %q: %w", path, err)
	}
	block, _ := pem.Decode(b)
	if block == nil {
		return "", nil, fmt.Errorf("Yggdrasil 公钥不是 PEM 格式: %s", path)
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return "", nil, fmt.Errorf("无法解析 Yggdrasil 公钥: %s", path)
	}
	publicKey, ok := key.(*rsa.PublicKey)
	if !ok {
		return "", nil, fmt.Errorf("Yggdrasil 公钥不是 RSA 密钥: %s", path)
	}
	return string(b), publicKey, nil
}
