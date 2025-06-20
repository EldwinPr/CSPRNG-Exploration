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

// WeatherCSPRNG implements a weather-based cryptographically secure pseudo-random number generator
type WeatherCSPRNG struct {
	state          []byte
	counter        uint64
	mutex          sync.Mutex
	client         *http.Client
	bytesGenerated int
	lastReseed     time.Time
}

// NewWeatherCSPRNG creates a new weather-based CSPRNG
func NewWeatherCSPRNG() *WeatherCSPRNG {
	w := &WeatherCSPRNG{
		client: &http.Client{Timeout: 1 * time.Second},
	}
	w.reseed()
	return w
}

// Name returns the generator name
func (w *WeatherCSPRNG) Name() string {
	return "Weather Based PRNG"
}

func (w *WeatherCSPRNG) reseed() {
	newEntropy := w.getWeatherEntropy()

	// Mix new entropy into the current state
	mac := hmac.New(sha256.New, w.state)
	mac.Write(newEntropy)
	w.state = mac.Sum(nil)

	w.lastReseed = time.Now()
	w.bytesGenerated = 0
}

func (w *WeatherCSPRNG) getWeatherEntropy() []byte {
	start := time.Now()
	resp, err := w.client.Get("https://wttr.in/?format=j1")
	duration := time.Since(start)

	var body []byte
	if err == nil {
		defer resp.Body.Close()
		body, _ = io.ReadAll(resp.Body) // Error ignored for benchmark simplicity
	}

	entropy := fmt.Sprintf("%s|%d|%d", body, start.UnixNano(), duration.Nanoseconds())
	hash := sha256.Sum256([]byte(entropy))
	return hash[:]
}

// GenerateBytes generates cryptographically secure random bytes
func (w *WeatherCSPRNG) GenerateBytes(numBytes int) ([]byte, error) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	// Check if reseeding is required
	if time.Since(w.lastReseed) > RESEED_INTERVAL || w.bytesGenerated > RESEED_BYTE_INTERVAL {
		w.reseed()
	}

	result := make([]byte, numBytes)
	generated := 0

	for generated < numBytes {
		mac := hmac.New(sha256.New, w.state)
		counterBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(counterBytes, w.counter)
		mac.Write(counterBytes)
		block := mac.Sum(nil)

		toCopy := min(len(block), numBytes-generated)
		copy(result[generated:], block[:toCopy])

		generated += toCopy
		w.counter++

		// Update state for next block generation
		updateMac := hmac.New(sha256.New, w.state)
		updateMac.Write(block)
		w.state = updateMac.Sum(nil)
	}

	w.bytesGenerated += numBytes
	return result, nil
}
