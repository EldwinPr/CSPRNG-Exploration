package main

import (
	"crypto/rand"
)

// CryptoCSPRNG implements a wrapper for the operating Crypto's CSPRNG
type CryptoCSPRNG struct{}

// NewCryptoCSPRNG creates a new Crypto CSPRNG
func NewCryptoCSPRNG() *CryptoCSPRNG {
	return &CryptoCSPRNG{}
}

// Name returns the generator name
func (s *CryptoCSPRNG) Name() string {
	return "Crypto CSPRNG"
}

// GenerateBytes generates cryptographically secure random bytes
func (s *CryptoCSPRNG) GenerateBytes(numBytes int) ([]byte, error) {
	result := make([]byte, numBytes)
	_, err := rand.Read(result)
	return result, err
}