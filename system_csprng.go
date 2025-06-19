package main

import (
	"crypto/rand"
)

// SystemCSPRNG implements a wrapper for the operating system's CSPRNG
type SystemCSPRNG struct{}

// NewSystemCSPRNG creates a new system CSPRNG
func NewSystemCSPRNG() *SystemCSPRNG {
	return &SystemCSPRNG{}
}

// Name returns the generator name
func (s *SystemCSPRNG) Name() string {
	return "System CSPRNG"
}

// GenerateBytes generates cryptographically secure random bytes
func (s *SystemCSPRNG) GenerateBytes(numBytes int) ([]byte, error) {
	result := make([]byte, numBytes)
	_, err := rand.Read(result)
	return result, err
}