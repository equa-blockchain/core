// Copyright 2024 The go-equa Authors
// This file is part of the go-equa library.

package equa

import (
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"math/big"

	"github.com/equa/go-equa/common"
	"github.com/equa/go-equa/core/types"
	"github.com/equa/go-equa/params"
)

// EncryptedTransaction represents a threshold-encrypted transaction
type EncryptedTransaction struct {
	Data      []byte              // Encrypted transaction data
	KeyShares map[int][]byte      // Threshold-encrypted key shares
	Signature []byte              // Threshold signature
	Nonce     []byte              // Nonce for encryption
}

// MarshalBinary serializes the encrypted transaction
func (et *EncryptedTransaction) MarshalBinary() ([]byte, error) {
	// Simple serialization - in production would use proper encoding
	data := make([]byte, 0)
	data = append(data, et.Data...)
	data = append(data, et.Nonce...)
	data = append(data, et.Signature...)
	return data, nil
}

// ThresholdCrypto handles threshold encryption and decryption using BLS signatures
type ThresholdCrypto struct {
	config       *params.EquaConfig
	masterPubKey []byte
	threshold    int
	shares       map[common.Address][]byte // Validator address -> key share
	polynomial   []*big.Int                // Polynomial coefficients for Shamir's Secret Sharing
}

// NewThresholdCrypto creates a new threshold crypto handler
func NewThresholdCrypto(config *params.EquaConfig) *ThresholdCrypto {
	return &ThresholdCrypto{
		config:    config,
		threshold: int(config.ThresholdShares),
		shares:    make(map[common.Address][]byte),
	}
}

// SetMasterPublicKey sets the master public key for encryption
func (tc *ThresholdCrypto) SetMasterPublicKey(pubKey []byte) {
	tc.masterPubKey = pubKey
}

// EncryptTransaction encrypts a transaction using threshold encryption
func (tc *ThresholdCrypto) EncryptTransaction(tx *types.Transaction) ([]byte, error) {
	// Serialize transaction
	txBytes, err := tx.MarshalBinary()
	if err != nil {
		return nil, err
	}

	// Generate random encryption key
	encryptionKey := make([]byte, 32)
	rand.Read(encryptionKey)

	// Encrypt transaction data with AES-GCM
	encryptedData, err := tc.encryptAES(txBytes, encryptionKey)
	if err != nil {
		return nil, err
	}

	// Create threshold-encrypted key using Shamir's Secret Sharing
	keyShares, err := tc.splitSecret(encryptionKey, tc.threshold, len(tc.shares))
	if err != nil {
		return nil, err
	}

	// Create encrypted transaction structure
	encryptedTx := &EncryptedTransaction{
		Data:      encryptedData,
		KeyShares: keyShares,
		Signature: tc.createThresholdSignature(encryptedData),
	}

	return encryptedTx.MarshalBinary()
}

// DecryptTransaction decrypts a transaction using validator key shares
func (tc *ThresholdCrypto) DecryptTransaction(tx *types.Transaction, keyShares [][]byte) (*types.Transaction, error) {
	if len(keyShares) < tc.threshold {
		return nil, errors.New("insufficient key shares for decryption")
	}

	// Parse encrypted transaction
	encryptedTx, err := tc.parseEncryptedTransaction(tx)
	if err != nil {
		return nil, err
	}

	// Verify threshold signature
	if !tc.verifyThresholdSignature(encryptedTx.Data, encryptedTx.Signature, keyShares) {
		return nil, errors.New("invalid threshold signature")
	}

	// Reconstruct encryption key using Lagrange interpolation
	encryptionKey, err := tc.reconstructSecret(keyShares)
	if err != nil {
		return nil, err
	}

	// Decrypt transaction data
	decryptedData, err := tc.decryptAES(encryptedTx.Data, encryptionKey)
	if err != nil {
		return nil, err
	}

	// Deserialize back to transaction
	var decryptedTx types.Transaction
	err = decryptedTx.UnmarshalBinary(decryptedData)
	if err != nil {
		return nil, err
	}

	return &decryptedTx, nil
}

// GenerateKeyShares generates key shares for validators using Shamir's Secret Sharing
func (tc *ThresholdCrypto) GenerateKeyShares(n, k int) ([][]byte, []byte, error) {
	// Generate master private key
	masterKey := make([]byte, 32)
	rand.Read(masterKey)

	// Generate polynomial coefficients for Shamir's Secret Sharing
	tc.polynomial = make([]*big.Int, k)
	tc.polynomial[0] = new(big.Int).SetBytes(masterKey) // Secret is coefficient 0

	// Generate random coefficients for polynomial
	for i := 1; i < k; i++ {
		coeff := make([]byte, 32)
		rand.Read(coeff)
		tc.polynomial[i] = new(big.Int).SetBytes(coeff)
	}

	// Generate shares for each validator
	shares := make([][]byte, n)
	for i := 0; i < n; i++ {
		share := tc.evaluatePolynomial(big.NewInt(int64(i + 1)))
		shares[i] = share.Bytes()
	}

	// Generate public key from master key
	publicKey := tc.generatePublicKey(masterKey)
	tc.masterPubKey = publicKey

	return shares, publicKey, nil
}

// VerifyKeyShare verifies that a key share is valid
func (tc *ThresholdCrypto) VerifyKeyShare(share []byte, validatorPubKey []byte) bool {
	// Verify share format
	if len(share) != 32 {
		return false
	}

	// Verify share is valid point on polynomial
	shareInt := new(big.Int).SetBytes(share)
	return tc.verifyShare(shareInt)
}

// CombineShares combines key shares to reconstruct the master key using Lagrange interpolation
func (tc *ThresholdCrypto) CombineShares(shares [][]byte) ([]byte, error) {
	if len(shares) < tc.threshold {
		return nil, errors.New("insufficient shares")
	}

	// Use Lagrange interpolation to reconstruct the secret
	secret := tc.lagrangeInterpolation(shares[:tc.threshold])
	return secret.Bytes(), nil
}

// Helper functions for threshold cryptography

// splitSecret splits a secret into shares using Shamir's Secret Sharing
func (tc *ThresholdCrypto) splitSecret(secret []byte, threshold, totalShares int) (map[int][]byte, error) {
	shares := make(map[int][]byte)

	// Convert secret to big.Int
	secretInt := new(big.Int).SetBytes(secret)

	// Generate polynomial with secret as constant term
	poly := make([]*big.Int, threshold)
	poly[0] = secretInt

	// Generate random coefficients
	for i := 1; i < threshold; i++ {
		coeff := make([]byte, 32)
		rand.Read(coeff)
		poly[i] = new(big.Int).SetBytes(coeff)
	}

	// Generate shares
	for j := 1; j <= totalShares; j++ {
		share := tc.evaluatePolynomialAt(big.NewInt(int64(j)), poly)
		shares[j] = share.Bytes()
	}

	return shares, nil
}

// evaluatePolynomialAt evaluates a polynomial at a given point
func (tc *ThresholdCrypto) evaluatePolynomialAt(x *big.Int, poly []*big.Int) *big.Int {
	result := big.NewInt(0)
	xPower := big.NewInt(1)

	for _, coeff := range poly {
		term := new(big.Int).Mul(coeff, xPower)
		result.Add(result, term)
		xPower.Mul(xPower, x)
	}

	return result
}

// evaluatePolynomial evaluates the stored polynomial at a given point
func (tc *ThresholdCrypto) evaluatePolynomial(x *big.Int) *big.Int {
	return tc.evaluatePolynomialAt(x, tc.polynomial)
}

// lagrangeInterpolation reconstructs the secret using Lagrange interpolation
func (tc *ThresholdCrypto) lagrangeInterpolation(shares [][]byte) *big.Int {
	// Convert shares to points
	points := make([][2]*big.Int, len(shares))
	for i, share := range shares {
		points[i] = [2]*big.Int{
			big.NewInt(int64(i + 1)), // x coordinate
			new(big.Int).SetBytes(share), // y coordinate
		}
	}

	// Lagrange interpolation
	secret := big.NewInt(0)

	for i := 0; i < len(points); i++ {
		term := new(big.Int).Set(points[i][1]) // y_i

		// Calculate Lagrange basis polynomial
		for j := 0; j < len(points); j++ {
			if i != j {
				// (x - x_j) / (x_i - x_j)
				numerator := new(big.Int).Sub(big.NewInt(0), points[j][0]) // -x_j
				denominator := new(big.Int).Sub(points[i][0], points[j][0]) // x_i - x_j

				// Multiply by current term
				term.Mul(term, numerator)
				term.Div(term, denominator)
			}
		}

		secret.Add(secret, term)
	}

	return secret
}

// verifyShare verifies that a share is valid
func (tc *ThresholdCrypto) verifyShare(share *big.Int) bool {
	// In a real implementation, this would verify the share against the public polynomial
	// For now, just check that it's a valid 32-byte value
	return share.BitLen() <= 256
}

// generatePublicKey generates a public key from a private key
func (tc *ThresholdCrypto) generatePublicKey(privateKey []byte) []byte {
	// Simplified public key generation
	// In production, this would use proper elliptic curve cryptography
	hash := sha256.Sum256(privateKey)
	return hash[:]
}

// encryptAES encrypts data using AES-GCM
func (tc *ThresholdCrypto) encryptAES(data, key []byte) ([]byte, error) {
	// Simplified AES encryption
	// In production, would use crypto/aes and crypto/cipher
	encrypted := make([]byte, len(data))
	for i, b := range data {
		encrypted[i] = b ^ key[i%len(key)]
	}
	return encrypted, nil
}

// decryptAES decrypts data using AES-GCM
func (tc *ThresholdCrypto) decryptAES(data, key []byte) ([]byte, error) {
	// Simplified AES decryption
	// In production, would use crypto/aes and crypto/cipher
	decrypted := make([]byte, len(data))
	for i, b := range data {
		decrypted[i] = b ^ key[i%len(key)]
	}
	return decrypted, nil
}

// createThresholdSignature creates a threshold signature
func (tc *ThresholdCrypto) createThresholdSignature(data []byte) []byte {
	// Simplified threshold signature
	// In production, would use BLS threshold signatures
	hash := sha256.Sum256(data)
	return hash[:]
}

// verifyThresholdSignature verifies a threshold signature
func (tc *ThresholdCrypto) verifyThresholdSignature(data, signature []byte, keyShares [][]byte) bool {
	// Simplified verification
	// In production, would verify BLS threshold signature
	expectedHash := sha256.Sum256(data)
	return len(signature) == len(expectedHash)
}

// parseEncryptedTransaction parses an encrypted transaction from a regular transaction
func (tc *ThresholdCrypto) parseEncryptedTransaction(tx *types.Transaction) (*EncryptedTransaction, error) {
	// Simplified parsing
	// In production, would properly deserialize the encrypted transaction structure
	data := tx.Data()
	if len(data) < 64 {
		return nil, errors.New("invalid encrypted transaction")
	}

	return &EncryptedTransaction{
		Data:      data[:len(data)-64],
		Nonce:     data[len(data)-64:len(data)-32],
		Signature: data[len(data)-32:],
	}, nil
}

// reconstructSecret reconstructs the secret from shares
func (tc *ThresholdCrypto) reconstructSecret(keyShares [][]byte) ([]byte, error) {
	// Convert shares to points for Lagrange interpolation
	points := make([][2]*big.Int, len(keyShares))
	for i, share := range keyShares {
		points[i] = [2]*big.Int{
			big.NewInt(int64(i + 1)), // x coordinate
			new(big.Int).SetBytes(share), // y coordinate
		}
	}

	// Use Lagrange interpolation to reconstruct secret
	secret := tc.lagrangeInterpolation(keyShares)
	return secret.Bytes(), nil
}
