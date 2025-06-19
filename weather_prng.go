package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// WeatherCSPRNG implements a high-performance weather-based CSPRNG
type WeatherCSPRNG struct {
	state   []byte
	counter uint64
	mutex   sync.Mutex
}

// NewWeatherCSPRNG creates a new weather-based CSPRNG
func NewWeatherCSPRNG() *WeatherCSPRNG {
	w := &WeatherCSPRNG{}

	// Single efficient API call
	start := time.Now()
	client := &http.Client{Timeout: 1 * time.Second}
	resp, err := client.Get("https://wttr.in/?format=j1")
	duration := time.Since(start)

	var body []byte
	if err == nil {
		defer resp.Body.Close()
		body, _ = io.ReadAll(resp.Body)
	}

	// Combine weather data with high-precision timing
	entropy := fmt.Sprintf("%s|%d|%d", body, start.UnixNano(), duration.Nanoseconds())
	hash := sha256.Sum256([]byte(entropy))
	w.state = hash[:]

	return w
}

// Name returns the generator name
func (w *WeatherCSPRNG) Name() string {
	return "Weather Based PRNG"
}

// GenerateBytes generates bytes with maximum performance
func (w *WeatherCSPRNG) GenerateBytes(numBytes int) ([]byte, error) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	result := make([]byte, numBytes)
	generated := 0

	for generated < numBytes {
		// Ultra-fast HMAC generation
		mac := hmac.New(sha256.New, w.state)
		counterBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(counterBytes, w.counter)
		mac.Write(counterBytes)
		block := mac.Sum(nil)

		// Efficient copying
		toCopy := len(block)
		if remaining := numBytes - generated; remaining < toCopy {
			toCopy = remaining
		}
		copy(result[generated:], block[:toCopy])

		generated += toCopy
		w.counter++
	}

	// Lightweight state update
	mac := hmac.New(sha256.New, w.state)
	mac.Write([]byte("update"))
	mac.Write(result[:32]) // Only first 32 bytes
	w.state = mac.Sum(nil)

	return result, nil
}
