package pkg

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"github.com/dgrijalva/jwt-go"
	"time"
)

func GenerateRSAKeys(bits int) (privateKeyPemBlock *pem.Block, publicKeyPemBlock *pem.Block, error error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, nil, err
	}
	privateBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privateBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateBytes,
	}

	publicKey := &privateKey.PublicKey
	publicBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	publicBlock := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicBytes,
	}
	if err != nil {
		return nil, nil, err
	}
	return privateBlock, publicBlock, nil
}

func SignedToken(tokenName string, privateKeyPem []byte) (string, error) {
	tokenClaims := jwt.StandardClaims{
		Id:       tokenName,
		IssuedAt: time.Now().Unix(),
		Issuer:   "FUNCEASY_ACCESS_SIGNER.funceasy.com",
		Subject:  tokenName,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, tokenClaims)
	signKey, err := jwt.ParseRSAPrivateKeyFromPEM(privateKeyPem)
	if err != nil {
		return "", err
	}
	tokenStr, err := token.SignedString(signKey)
	if err != nil {
		return "", err
	}
	return tokenStr, nil
}
