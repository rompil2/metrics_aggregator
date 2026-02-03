package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Helper function to generate a temporary RSA key pair for testing
func generateTestKeys() (privKey *rsa.PrivateKey, pubKey *rsa.PublicKey, err error) {
	privKey, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}
	pubKey = &privKey.PublicKey
	return privKey, pubKey, nil
}

// Helper function to save a public key to a temporary file
func savePublicKeyToFile(pubKey *rsa.PublicKey, filename string) error {
	pubBytes, err := x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		return err
	}

	pubPEM := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubBytes,
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	return pem.Encode(file, pubPEM)
}

// Helper function to save a private key to a temporary file
func savePrivateKeyToFile(privKey *rsa.PrivateKey, filename string) error {
	privBytes := x509.MarshalPKCS1PrivateKey(privKey)

	privPEM := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privBytes,
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	return pem.Encode(file, privPEM)
}

func TestLoadPublicKey(t *testing.T) {
	_, pub, err := generateTestKeys()
	assert.NoError(t, err, "Failed to generate test keys")

	tempFile := t.TempDir() + "/public.pem"
	err = savePublicKeyToFile(pub, tempFile)
	assert.NoError(t, err, "Failed to save public key")

	key, err := LoadPublicKey(tempFile)
	assert.NoError(t, err)
	assert.NotNil(t, key)

	// Test with empty filename
	key, err = LoadPublicKey("")
	assert.NoError(t, err)
	assert.Nil(t, key)

	// Test with non-existent file
	_, err = LoadPublicKey("nonexistent.pem")
	assert.Error(t, err)
}

func TestLoadPrivateKey(t *testing.T) {
	priv, _, err := generateTestKeys()
	assert.NoError(t, err, "Failed to generate test keys")

	tempFile := t.TempDir() + "/private.pem"
	err = savePrivateKeyToFile(priv, tempFile)
	assert.NoError(t, err, "Failed to save private key")

	key, err := LoadPrivateKey(tempFile)
	assert.NoError(t, err)
	assert.NotNil(t, key)

	// Test with empty filename
	key, err = LoadPrivateKey("")
	assert.NoError(t, err)
	assert.Nil(t, key)

	// Test with non-existent file
	_, err = LoadPrivateKey("nonexistent.pem")
	assert.Error(t, err)
}

func TestEncryptWithPublicKey(t *testing.T) {
	priv, pub, err := generateTestKeys()
	assert.NoError(t, err, "Failed to generate test keys")

	data := []byte("hello world")

	// Test normal encryption
	encrypted, err := EncryptWithPublicKey(pub, data)
	assert.NoError(t, err)
	assert.NotEmpty(t, encrypted)

	// Test with nil key (should return original data)
	encrypted, err = EncryptWithPublicKey(nil, data)
	assert.NoError(t, err)
	assert.Equal(t, data, encrypted)

	// Test with large data (should work due to chunking)
	largeData := make([]byte, 1000) // Larger than typical RSA-2048 limit
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}
	encrypted, err = EncryptWithPublicKey(pub, largeData)
	assert.NoError(t, err)
	assert.NotEmpty(t, encrypted)
	assert.True(t, len(encrypted) > len(largeData)) // Encrypted data should be larger due to padding

	// Test decryption of large data
	decrypted, err := DecryptWithPrivateKey(priv, encrypted)
	assert.NoError(t, err)
	assert.Equal(t, largeData, decrypted)
}

func TestDecryptWithPrivateKey(t *testing.T) {
	priv, pub, err := generateTestKeys()
	assert.NoError(t, err, "Failed to generate test keys")

	originalData := []byte("hello world")

	encrypted, err := EncryptWithPublicKey(pub, originalData)
	assert.NoError(t, err, "Failed to encrypt")

	// Test normal decryption
	decrypted, err := DecryptWithPrivateKey(priv, encrypted)
	assert.NoError(t, err)
	assert.Equal(t, originalData, decrypted)

	// Test with nil key (should return original data)
	decrypted, err = DecryptWithPrivateKey(nil, encrypted)
	assert.NoError(t, err)
	assert.Equal(t, encrypted, decrypted)

	// Test decryption of large data
	largeData := make([]byte, 1000)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}
	encryptedLarge, err := EncryptWithPublicKey(pub, largeData)
	assert.NoError(t, err)
	decryptedLarge, err := DecryptWithPrivateKey(priv, encryptedLarge)
	assert.NoError(t, err)
	assert.Equal(t, largeData, decryptedLarge)

	// Test with invalid encrypted data length
	invalidData := []byte{1, 2, 3} // Not a multiple of key size
	_, err = DecryptWithPrivateKey(priv, invalidData)
	assert.Error(t, err)
}
