package main

import (
	"math/rand"
	"sync"
	"time"
)

// InsecurePRNG implements a simple insecure PRNG using math/rand
type InsecurePRNG struct {
	rng  *rand.Rand
	lock sync.Mutex
}

// NewInsecurePRNG creates a new insecure PRNG
func NewInsecurePRNG() *InsecurePRNG {
	// Seed with predictable time
	source := rand.NewSource(time.Now().UnixNano())
	return &InsecurePRNG{
		rng: rand.New(source),
	}
}

// Name returns the generator name
func (p *InsecurePRNG) Name() string {
	return "Insecure PRNG"
}

// GenerateBytes generates bytes using math/rand
func (p *InsecurePRNG) GenerateBytes(numBytes int) ([]byte, error) {
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
