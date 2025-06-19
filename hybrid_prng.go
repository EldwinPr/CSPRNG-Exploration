package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// HybridCSPRNG implements a high-performance hybrid CSPRNG
type HybridCSPRNG struct {
	state   []byte
	counter uint64
	mutex   sync.Mutex
}

// NewHybridCSPRNG creates a new hybrid CSPRNG
func NewHybridCSPRNG() *HybridCSPRNG {
	h := &HybridCSPRNG{}

	// Hybrid seeding: Weather + System CSPRNG
	weatherEntropy := h.getWeatherEntropy()
	systemEntropy := make([]byte, 32)
	if _, err := rand.Read(systemEntropy); err != nil {
		systemEntropy = []byte("fallback_system_entropy")
	}

	combined := append(weatherEntropy, systemEntropy...)
	hash := sha256.Sum256(combined)
	h.state = hash[:]

	return h
}

// Name returns the generator name
func (h *HybridCSPRNG) Name() string {
	return "Hybrid PRNG (Weather + System)"
}

// getWeatherEntropy efficiently gathers weather data
func (h *HybridCSPRNG) getWeatherEntropy() []byte {
	client := &http.Client{Timeout: 1 * time.Second}
	start := time.Now()
	resp, err := client.Get("https://wttr.in/?format=j1")
	duration := time.Since(start)

	if err != nil {
		return []byte(fmt.Sprintf("error:%d", duration.Nanoseconds()))
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return []byte(fmt.Sprintf("readerror:%d", duration.Nanoseconds()))
	}

	// Include timing in entropy
	return append(body, []byte(strconv.FormatInt(duration.Nanoseconds(), 10))...)
}

// GenerateBytes generates bytes with high performance
func (h *HybridCSPRNG) GenerateBytes(numBytes int) ([]byte, error) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	result := make([]byte, numBytes)
	generated := 0

	for generated < numBytes {
		// Fast HMAC generation
		mac := hmac.New(sha256.New, h.state)
		counterBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(counterBytes, h.counter)
		mac.Write(counterBytes)
		block := mac.Sum(nil)

		// Copy as much as needed
		toCopy := len(block)
		if remaining := numBytes - generated; remaining < toCopy {
			toCopy = remaining
		}
		copy(result[generated:], block[:toCopy])

		generated += toCopy
		h.counter++
	}

	// Efficient state update
	mac := hmac.New(sha256.New, h.state)
	mac.Write([]byte("update"))
	mac.Write(result[:min(32, len(result))])
	h.state = mac.Sum(nil)

	return result, nil
}
