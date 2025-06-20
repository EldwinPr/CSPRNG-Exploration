package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	RESEED_INTERVAL      = 10 * time.Minute
	RESEED_BYTE_INTERVAL = 500 * 1024 * 1024 // 500 MB
)

// multEntropyCSPRNG implements a multi-source entropy CSPRNG
// using weather, market, and network data as entropy sources
type multEntropyCSPRNG struct {
	state          []byte
	counter        uint64
	mutex          sync.Mutex
	client         *http.Client
	bytesGenerated int
	lastReseed     time.Time
}

// NewmultEntropyCSPRNG creates a new multi-entropy CSPRNG
func NewmultEntropyCSPRNG() *multEntropyCSPRNG {
	c := &multEntropyCSPRNG{
		client: &http.Client{Timeout: 2 * time.Second}, // Increased timeout for global pings
	}
	c.reseed()
	return c
}

// Name returns the generator name
func (c *multEntropyCSPRNG) Name() string {
	return "3 Entropy Source PRNG"
}

// reseed gathers fresh entropy and mixes it into the state
func (c *multEntropyCSPRNG) reseed() {
	newEntropy := c.gatherEntropy()
	
	// Mix new entropy into the current state using HMAC
	mac := hmac.New(sha256.New, c.state) // Use old state as key
	mac.Write(newEntropy)
	c.state = mac.Sum(nil)
	
	c.lastReseed = time.Now()
	c.bytesGenerated = 0
}

// gatherEntropy collects entropy from multiple sources concurrently
func (c *multEntropyCSPRNG) gatherEntropy() []byte {
	var wg sync.WaitGroup
	wg.Add(3)

	var weatherData, marketData, networkData string

	go func() {
		defer wg.Done()
		weatherData = c.getWeather()
	}()
	go func() {
		defer wg.Done()
		marketData = c.getMarket()
	}()
	go func() {
		defer wg.Done()
		networkData = c.getNetworkJitter()
	}()

	wg.Wait()

	entropy := fmt.Sprintf("%s|%s|%s|%d", weatherData, marketData, networkData, time.Now().UnixNano())
	hash := sha256.Sum256([]byte(entropy))
	return hash[:]
}

// getWeather fetches weather data as an entropy source
func (c *multEntropyCSPRNG) getWeather() string {
	resp, err := c.client.Get("https://wttr.in/?format=j1")
	if err != nil {
		return "weather_error"
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "weather_read_error"
	}
	return string(body)
}

// getMarket fetches cryptocurrency market data as an entropy source
func (c *multEntropyCSPRNG) getMarket() string {
	resp, err := c.client.Get("https://api.coingecko.com/api/v3/simple/price?ids=bitcoin&vs_currencies=usd")
	if err != nil {
		return "market_error"
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "market_read_error"
	}
	return string(body)
}

// getNetworkJitter measures network latency to multiple global endpoints
// This aligns with the methodology of using diverse nodes.
func (c *multEntropyCSPRNG) getNetworkJitter() string {
	endpoints := []string{
		"https://www.google.com",     // North America
		"https://www.yandex.ru",      // Europe/Russia
		"https://www.baidu.com",      // Asia
		"https://www.mercadolibre.com.ar", // South America
	}

	var latencies []string
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, endpoint := range endpoints {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			start := time.Now()
			resp, err := c.client.Get(url)
			duration := time.Since(start)
			if err == nil {
				resp.Body.Close()
				mu.Lock()
				latencies = append(latencies, strconv.FormatInt(duration.Nanoseconds(), 10))
				mu.Unlock()
			}
		}(endpoint)
	}

	wg.Wait()
	if len(latencies) == 0 {
		return "network_error_all"
	}
	return strings.Join(latencies, ",")
}

// GenerateBytes generates cryptographically secure random bytes
func (c *multEntropyCSPRNG) GenerateBytes(numBytes int) ([]byte, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Check if reseeding is required 
	if time.Since(c.lastReseed) > RESEED_INTERVAL || c.bytesGenerated > RESEED_BYTE_INTERVAL {
		c.reseed()
	}

	result := make([]byte, numBytes)
	generated := 0

	for generated < numBytes {
		mac := hmac.New(sha256.New, c.state)
		counterBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(counterBytes, c.counter)
		mac.Write(counterBytes)
		block := mac.Sum(nil)

		toCopy := min(len(block), numBytes-generated)
		copy(result[generated:], block[:toCopy])

		generated += toCopy
		c.counter++

		// Update state for next block generation
		updateMac := hmac.New(sha256.New, c.state)
		updateMac.Write(block)
		c.state = updateMac.Sum(nil)
	}

	c.bytesGenerated += numBytes
	return result, nil
}