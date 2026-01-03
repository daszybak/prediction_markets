package config

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
)

// RSAPrivateKey wraps *rsa.PrivateKey and implements yaml.Unmarshaler
// to decode from base64-encoded PEM.
type RSAPrivateKey struct {
	*rsa.PrivateKey
}

// UnmarshalYAML decodes a base64-encoded PEM RSA private key.
func (k *RSAPrivateKey) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var encoded string
	if err := unmarshal(&encoded); err != nil {
		return err
	}

	if encoded == "" {
		return nil
	}

	key, err := decodeRSAPrivateKey(encoded)
	if err != nil {
		return fmt.Errorf("decode RSA private key: %w", err)
	}

	k.PrivateKey = key
	return nil
}

func decodeRSAPrivateKey(encoded string) (*rsa.PrivateKey, error) {
	pemBytes, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("base64 decode: %w", err)
	}

	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found")
	}

	// Try PKCS#1 first, then PKCS#8
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}

	keyAny, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	key, ok := keyAny.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA private key")
	}

	return key, nil
}
