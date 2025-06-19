package main

import (
	"math/rand"
	"sync"
	"time"
)

// MathPRNG implements a simple Math PRNG using math/rand
type MathPRNG struct {
	rng  *rand.Rand
	lock sync.Mutex
}

// NewMathPRNG creates a new Math PRNG
func NewMathPRNG() *MathPRNG {
	// Seed with predictable time
	source := rand.NewSource(time.Now().UnixNano())
	return &MathPRNG{
		rng: rand.New(source),
	}
}

// Name returns the generator name
func (p *MathPRNG) Name() string {
	return "Math PRNG"
}

// GenerateBytes generates bytes using math/rand
func (p *MathPRNG) GenerateBytes(numBytes int) ([]byte, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	result := make([]byte, numBytes)

	// Generate 4 bytes at a time for better performance
	for i := 0; i < numBytes; i += 4 {
		remaining := numBytes - i
		val := p.rng.Uint32()

		// Write up to 4 bytes
		result[i] = byte(val)
		if remaining > 1 {
			result[i+1] = byte(val >> 8)
		}
		if remaining > 2 {
			result[i+2] = byte(val >> 16)
		}
		if remaining > 3 {
			result[i+3] = byte(val >> 24)
		}
	}
	return result, nil
}
