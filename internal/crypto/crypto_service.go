// Package handler provides a service for decrypting request bodies using RSA private key.
// path: internal/handler/crypto_service.go
package crypto

import (
	"bytes"
	"crypto/rsa"
	"io"
)

// CryptoService provides methods for decrypting HTTP request bodies.
type CryptoService struct {
	privateKey *rsa.PrivateKey
}

// NewCryptoService creates a new instance of CryptoService with the given private key.
func NewCryptoService(privateKey *rsa.PrivateKey) *CryptoService {
	return &CryptoService{
		privateKey: privateKey,
	}
}

// DecryptRequestBody takes a request body, decrypts it using the private key,
// and returns the decrypted Reader. If the key is not set, it returns the original Reader.
func (cs *CryptoService) DecryptRequestBody(body io.ReadCloser) (io.ReadCloser, error) {
	if cs.privateKey == nil {
		return body, nil
	}

	data, err := io.ReadAll(body)
	if err != nil {
		return nil, err
	}
	body.Close()

	decryptedData, err := DecryptWithPrivateKey(cs.privateKey, data)
	if err != nil {
		return nil, err
	}

	return io.NopCloser(bytes.NewReader(decryptedData)), nil
}
