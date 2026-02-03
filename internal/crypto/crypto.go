// Package crypto provides encription and decryption utilities
// path: internal/crypto
package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"math"
	"os"
)

const maxChunkSize = 190 // For 2048-bit RSA with OAEP SHA-256

// LoadPublicKey reads and parses an RSA public key from a PEM-encoded file.
// Returns nil if the filename is empty.
func LoadPublicKey(filename string) (*rsa.PublicKey, error) {
	if filename == "" {
		return nil, nil
	}
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("not an RSA public key")
	}

	return rsaPub, nil
}

// LoadPrivateKey reads and parses an RSA private key from a PEM-encoded file.
// Returns nil if the filename is empty.
func LoadPrivateKey(filename string) (*rsa.PrivateKey, error) {
	if filename == "" {
		return nil, nil
	}
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}

	return x509.ParsePKCS1PrivateKey(block.Bytes)

}

// EncryptWithPublicKey encrypts the input data using the provided RSA public key.
// Uses OAEP padding with SHA-256 as the hash function.
// If the public key is nil, returns the data unchanged.
// Automatically splits data into chunks that fit RSA limits.
func EncryptWithPublicKey(publicKey *rsa.PublicKey, data []byte) ([]byte, error) {
	if publicKey == nil {
		return data, nil
	}

	if len(data) <= maxChunkSize {
		return rsa.EncryptOAEP(sha256.New(), rand.Reader, publicKey, data, nil)
	}

	var result []byte
	for i := 0; i < len(data); i += maxChunkSize {
		end := int(math.Min(float64(i+maxChunkSize), float64(len(data))))
		chunk := data[i:end]

		encryptedChunk, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, publicKey, chunk, nil)
		if err != nil {
			return nil, err
		}
		result = append(result, encryptedChunk...)
	}

	return result, nil
}

// DecryptWithPrivateKey decrypts the input data using the provided RSA private key.
// Uses OAEP padding with SHA-256 as the hash function.
// If the private key is nil, returns the data unchanged.
// Automatically splits data into chunks that were encrypted.
func DecryptWithPrivateKey(privateKey *rsa.PrivateKey, data []byte) ([]byte, error) {
	if privateKey == nil {
		return data, nil
	}

	// Calculate the size of each encrypted chunk (depends on key size)
	encryptedChunkSize := privateKey.Size()

	if len(data)%encryptedChunkSize != 0 {
		return nil, errors.New("invalid encrypted data length")
	}

	var result []byte
	for i := 0; i < len(data); i += encryptedChunkSize {
		chunk := data[i : i+encryptedChunkSize]

		decryptedChunk, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, privateKey, chunk, nil)
		if err != nil {
			return nil, err
		}
		result = append(result, decryptedChunk...)
	}

	return result, nil
}
