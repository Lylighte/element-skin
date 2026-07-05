package yggdrasil

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"errors"
)

func verifyKeyPair(privateKey *rsa.PrivateKey, publicKey *rsa.PublicKey) error {
	const probe = "element-skin-yggdrasil-key-pair-check"
	digest := sha1.Sum([]byte(probe))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA1, digest[:])
	if err != nil {
		return err
	}
	if err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA1, digest[:], signature); err != nil {
		return errors.New("Yggdrasil 公钥与私钥不匹配")
	}
	return nil
}
