package crypto

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewCryptoService tests the creation of a new CryptoService instance with a valid private key.
func TestNewCryptoService(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	service := NewCryptoService(privKey)
	assert.Equal(t, privKey, service.privateKey)
}

// TestCryptoService_DecryptRequestBody_WithNilKey tests that DecryptRequestBody returns the original body unchanged when the private key is nil.
func TestCryptoService_DecryptRequestBody_WithNilKey(t *testing.T) {
	service := &CryptoService{privateKey: nil}

	originalBody := io.NopCloser(bytes.NewReader([]byte("test data")))

	decryptedBody, err := service.DecryptRequestBody(originalBody)
	require.NoError(t, err)

	// Should return original body unchanged
	data, err := io.ReadAll(decryptedBody)
	require.NoError(t, err)
	assert.Equal(t, "test data", string(data))
}

// TestCryptoService_DecryptRequestBody_Success tests successful decryption of an encrypted request body.
func TestCryptoService_DecryptRequestBody_Success(t *testing.T) {
	// Generate a key pair
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	service := &CryptoService{privateKey: privKey}

	// Encrypt some data using the public key
	originalData := []byte("Hello, encrypted world!")
	encryptedData, err := EncryptWithPublicKey(&privKey.PublicKey, originalData)
	require.NoError(t, err)

	// Create an encrypted body
	encryptedBody := io.NopCloser(bytes.NewReader(encryptedData))

	// Decrypt it using the service
	decryptedBody, err := service.DecryptRequestBody(encryptedBody)
	require.NoError(t, err)

	// Read the decrypted data
	data, err := io.ReadAll(decryptedBody)
	require.NoError(t, err)

	// Should match original data
	assert.Equal(t, originalData, data)
}

// TestCryptoService_DecryptRequestBody_Failure tests that DecryptRequestBody returns an error when decryption fails.
func TestCryptoService_DecryptRequestBody_Failure(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	service := &CryptoService{privateKey: privKey}

	// Provide corrupted data that cannot be decrypted
	corruptedData := []byte("this is not encrypted data")
	corruptedBody := io.NopCloser(bytes.NewReader(corruptedData))

	// Attempt to decrypt
	_, err = service.DecryptRequestBody(corruptedBody)

	// Should return an error
	assert.Error(t, err)
}
