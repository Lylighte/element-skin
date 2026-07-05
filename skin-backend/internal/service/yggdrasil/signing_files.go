package yggdrasil

import (
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func ensureSigningKeyFiles(privatePath, publicPath string) error {
	privateExists, err := fileExists(privatePath)
	if err != nil {
		return err
	}
	publicExists, err := fileExists(publicPath)
	if err != nil {
		return err
	}
	if privateExists && publicExists {
		return nil
	}
	if !privateExists && publicExists {
		return nil
	}

	var privateKey *rsa.PrivateKey
	if privateExists {
		privateKey, err = readPrivateKey(privatePath)
		if err != nil {
			return err
		}
	} else {
		privateKey, err = rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return fmt.Errorf("生成 Yggdrasil RSA 密钥失败: %w", err)
		}
		privatePEM, err := marshalPrivateKeyPEM(privateKey)
		if err != nil {
			return err
		}
		if err := writeKeyFile(privatePath, privatePEM, 0o600); err != nil {
			return err
		}
	}

	publicPEM, err := marshalPublicKeyPEM(&privateKey.PublicKey)
	if err != nil {
		return err
	}
	if err := writeKeyFile(publicPath, publicPEM, 0o644); err != nil {
		return err
	}
	return nil
}

func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func writeKeyFile(path string, data []byte, mode os.FileMode) error {
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("创建 Yggdrasil 密钥目录失败 %q: %w", dir, err)
		}
	}
	if err := os.WriteFile(path, data, mode); err != nil {
		return fmt.Errorf("写入 Yggdrasil 密钥失败 %q: %w", path, err)
	}
	return nil
}
