package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// multEntropyCSPRNG implements a multi-source entropy CSPRNG
// using weather, market, and network data as entropy sources
type multEntropyCSPRNG struct {
	state   []byte
	counter uint64
	mutex   sync.Mutex
}

// NewmultEntropyCSPRNG creates a new multi-entropy CSPRNG
func NewmultEntropyCSPRNG() *multEntropyCSPRNG {
	c := &multEntropyCSPRNG{}
	c.state = c.gatherEntropy()
	return c
}

// Name returns the generator name
func (c *multEntropyCSPRNG) Name() string {
	return "3 Entropy Source PRNG"
}

// gatherEntropy collects entropy from multiple sources
func (c *multEntropyCSPRNG) gatherEntropy() []byte {

	weather := c.getWeather()

	market := c.getMarket()

	network := c.getNetwork()

	entropy := fmt.Sprintf("%s|%s|%s|%d", weather, market, network, time.Now().UnixNano())
	hash := sha256.Sum256([]byte(entropy))
	return hash[:]
}

// getWeather fetches weather data as entropy source
func (c *multEntropyCSPRNG) getWeather() string {
	client := &http.Client{Timeout: 1 * time.Second}
	resp, err := client.Get("https://wttr.in/?format=j1")
	if err != nil {
		return "weather_error"
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	return string(body)
}

// getMarket fetches cryptocurrency market data as entropy source
func (c *multEntropyCSPRNG) getMarket() string {
	client := &http.Client{Timeout: 1 * time.Second}
	resp, err := client.Get("https://api.coingecko.com/api/v3/simple/price?ids=bitcoin&vs_currencies=usd")
	if err != nil {
		return "market_error"
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	return string(body)
}

// getNetwork measures network latency as entropy source
func (c *multEntropyCSPRNG) getNetwork() string {
	start := time.Now()
	client := &http.Client{Timeout: 1 * time.Second}
	_, err := client.Get("https://www.google.com")
	duration := time.Since(start)

	if err != nil {
		return "network_error"
	}
	return strconv.FormatInt(duration.Nanoseconds(), 10)
}

// GenerateBytes generates cryptographically secure random bytes
func (c *multEntropyCSPRNG) GenerateBytes(numBytes int) ([]byte, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	result := make([]byte, numBytes)
	generated := 0

	for generated < numBytes {

		mac := hmac.New(sha256.New, c.state)
		counterBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(counterBytes, c.counter)
		mac.Write(counterBytes)
		block := mac.Sum(nil)

		toCopy := len(block)
		if remaining := numBytes - generated; remaining < toCopy {
			toCopy = remaining
		}
		copy(result[generated:], block[:toCopy])

		generated += toCopy
		c.counter++
	}

	mac := hmac.New(sha256.New, c.state)
	mac.Write([]byte("update"))
	mac.Write(result[:32])
	c.state = mac.Sum(nil)

	return result, nil
}
