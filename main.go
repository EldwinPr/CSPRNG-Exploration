package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	insecure_rand "math/rand"
	"net/http"
	"os"
	"strconv"
	"sync"
	"text/tabwriter"
	"time"
)

// --- Generator Function Type ---
type prngFunc func(int) ([]byte, error)

// --- Global State for PRNGs ---

// InsecurePRNG state
var (
	insecureRng   *insecure_rand.Rand
	insecureMutex sync.Mutex
)

// WeatherCSPRNG state
var (
	weatherState   []byte
	weatherCounter uint64
	weatherMutex   sync.Mutex
)

// CustomCSPRNG state
var (
	customState   []byte
	customCounter uint64
	customMutex   sync.Mutex
)

// HybridCSPRNG state
var (
	hybridState   []byte
	hybridCounter uint64
	hybridMutex   sync.Mutex
)

// --- PRNG Initializers ---

// initInsecurePRNG initializes the insecure generator.
func initInsecurePRNG() {
	source := insecure_rand.NewSource(time.Now().UnixNano())
	insecureRng = insecure_rand.New(source)
}

// initWeatherCSPRNG initializes the weather-based generator.
func initWeatherCSPRNG() {
	fmt.Println("Initializing Weather Based PRNG...")
	start := time.Now()
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("https://wttr.in/?format=j1")
	duration := time.Since(start)
	var body []byte
	if err == nil {
		defer resp.Body.Close()
		body, _ = io.ReadAll(resp.Body)
	} else {
		fmt.Println("Weather PRNG failed to get entropy:", err)
	}
	entropy := fmt.Sprintf("%s|%d|%d", body, start.UnixNano(), duration.Nanoseconds())
	hash := sha256.Sum256([]byte(entropy))
	weatherState = hash[:]
	fmt.Println("Weather Based PRNG Initialized.")
}

// initCustomCSPRNG initializes the 3-source generator.
func initCustomCSPRNG() {
	fmt.Println("Initializing 3 Entropy Source PRNG...")
	var wg sync.WaitGroup
	results := make(chan string, 3)

	get := func(url string) string {
		client := &http.Client{Timeout: 2 * time.Second}
		resp, err := client.Get(url)
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return string(body)
	}

	getNetworkTiming := func() string {
		start := time.Now()
		_, err := http.Get("https://www.google.com")
		duration := time.Since(start)
		if err != nil {
			return "network_error"
		}
		return strconv.FormatInt(duration.Nanoseconds(), 10)
	}

	wg.Add(3)
	go func() { defer wg.Done(); results <- get("https://wttr.in/?format=j1") }()
	go func() {
		defer wg.Done()
		results <- get("https://api.coingecko.com/api/v3/simple/price?ids=bitcoin&vs_currencies=usd")
	}()
	go func() { defer wg.Done(); results <- getNetworkTiming() }()
	wg.Wait()
	close(results)

	var entropySources []string
	for r := range results {
		entropySources = append(entropySources, r)
	}

	entropy := fmt.Sprintf("%s|%s|%s|%d", entropySources[0], entropySources[1], entropySources[2], time.Now().UnixNano())
	hash := sha256.Sum256([]byte(entropy))
	customState = hash[:]
	fmt.Println("3 Entropy Source PRNG Initialized.")
}

// initHybridCSPRNG initializes the hybrid generator.
func initHybridCSPRNG() {
	fmt.Println("Initializing Hybrid PRNG...")
	var weatherEntropy, systemEntropy []byte
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		client := &http.Client{Timeout: 2 * time.Second}
		start := time.Now()
		resp, err := client.Get("https://wttr.in/?format=j1")
		duration := time.Since(start)
		if err != nil {
			weatherEntropy = []byte(fmt.Sprintf("error:%d", duration.Nanoseconds()))
			return
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			weatherEntropy = []byte(fmt.Sprintf("readerror:%d", duration.Nanoseconds()))
			return
		}
		weatherEntropy = append(body, []byte(strconv.FormatInt(duration.Nanoseconds(), 10))...)
	}()
	go func() {
		defer wg.Done()
		systemEntropy = make([]byte, 32)
		if _, err := rand.Read(systemEntropy); err != nil {
			systemEntropy = []byte("fallback_system_entropy")
		}
	}()
	wg.Wait()
	combined := append(weatherEntropy, systemEntropy...)
	hash := sha256.Sum256(combined)
	hybridState = hash[:]
	fmt.Println("Hybrid PRNG Initialized.")
}

// --- PRNG IMPLEMENTATIONS (Functions) ---

func generateSystemBytes(numBytes int) ([]byte, error) {
	result := make([]byte, numBytes)
	_, err := rand.Read(result)
	return result, err
}

func generateInsecureBytes(numBytes int) ([]byte, error) {
	insecureMutex.Lock()
	defer insecureMutex.Unlock()
	result := make([]byte, numBytes)
	for i := 0; i < numBytes; i++ {
		result[i] = byte(insecureRng.Intn(256))
	}
	return result, nil
}

func generateWithState(numBytes int, state *[]byte, counter *uint64, mutex *sync.Mutex) ([]byte, error) {
	mutex.Lock()
	defer mutex.Unlock()

	result := make([]byte, numBytes)
	generated := 0
	for generated < numBytes {
		mac := hmac.New(sha256.New, *state)
		counterBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(counterBytes, *counter)
		mac.Write(counterBytes)
		block := mac.Sum(nil)

		toCopy := len(block)
		if remaining := numBytes - generated; remaining < toCopy {
			toCopy = remaining
		}
		copy(result[generated:], block[:toCopy])
		generated += toCopy
		*counter++
	}

	mac := hmac.New(sha256.New, *state)
	mac.Write([]byte("update"))
	mac.Write(result[:min(32, len(result))])
	*state = mac.Sum(nil)

	return result, nil
}

func generateWeatherBytes(numBytes int) ([]byte, error) {
	return generateWithState(numBytes, &weatherState, &weatherCounter, &weatherMutex)
}

func generateCustomBytes(numBytes int) ([]byte, error) {
	return generateWithState(numBytes, &customState, &customCounter, &customMutex)
}

func generateHybridBytes(numBytes int) ([]byte, error) {
	return generateWithState(numBytes, &hybridState, &hybridCounter, &hybridMutex)
}

// --- Statistical Analysis ---
func calculateChiSquare(data []byte) float64 {
	counts := make(map[byte]int, 256)
	for _, b := range data {
		counts[b]++
	}
	if len(data) == 0 {
		return 0
	}
	expected := float64(len(data)) / 256.0
	var chiSquareStat float64
	for i := 0; i < 256; i++ {
		observed := float64(counts[byte(i)])
		diff := observed - expected
		chiSquareStat += (diff * diff) / expected
	}
	return chiSquareStat
}

func calculateShannonEntropy(data []byte) float64 {
	if len(data) == 0 {
		return 0.0
	}
	counts := make(map[byte]int)
	for _, b := range data {
		counts[b]++
	}
	var entropy float64
	dataLen := float64(len(data))
	for _, count := range counts {
		if count > 0 {
			p := float64(count) / dataLen
			entropy -= p * math.Log2(p)
		}
	}
	return entropy
}

func nistFrequencyMonobitTest(data []byte) float64 {
	n := len(data) * 8
	if n == 0 {
		return 0.0
	}
	s := 0
	for _, b := range data {
		for i := 0; i < 8; i++ {
			if (b>>i)&1 == 1 {
				s++
			} else {
				s--
			}
		}
	}
	sObs := math.Abs(float64(s)) / math.Sqrt(float64(n))
	pValue := math.Erfc(sObs / math.Sqrt2)
	return pValue
}

// --- Main Benchmark Logic ---

type TestResult struct {
	Name                 string
	TotalTime            time.Duration
	AvgOpsPerSec         float64
	AvgChiSquare         float64
	AvgShannonEntropy    float64
	AvgNISTMonobitPValue float64
	PassedNISTMonobit    int
	TotalIterations      int
}

func main() {
	// --- Configuration ---
	iterations := 10000
	dataSize := 1024 * 256 // 256 KB
	// --- End Configuration ---

	// --- Initialization ---
	initInsecurePRNG()
	initWeatherCSPRNG()
	initCustomCSPRNG()
	initHybridCSPRNG()
	fmt.Println("\nAll generators initialized.")
	// --- End Initialization ---

	fmt.Printf("\nStarting PRNG benchmark...\n")
	fmt.Printf("Iterations: %d, Data Size: %d bytes\n\n", iterations, dataSize)

	generators := map[string]prngFunc{
		"System CSPRNG":                generateSystemBytes,
		"Insecure PRNG":                generateInsecureBytes,
		"Weather Based PRNG":           generateWeatherBytes,
		"3 Entropy Source PRNG":        generateCustomBytes,
		"Hybrid PRNG (Weather+System)": generateHybridBytes,
	}

	resultsChan := make(chan TestResult, len(generators))
	var wg sync.WaitGroup

	for name, fn := range generators {
		wg.Add(1)
		go func(name string, generator prngFunc, iter, size int) {
			defer wg.Done()
			resultsChan <- runBenchmark(name, generator, iter, size)
		}(name, fn, iterations, dataSize)
	}

	wg.Wait()
	close(resultsChan)

	var finalResults []TestResult
	for result := range resultsChan {
		finalResults = append(finalResults, result)
	}
	printResults(finalResults)
}

func runBenchmark(name string, generator prngFunc, iterations, dataSize int) TestResult {
	var totalTime time.Duration
	var totalChiSquare, totalShannonEntropy, totalNISTMonobitPValue float64
	var passedNIST int

	fmt.Printf("Testing: %s...\n", name)

	for i := 0; i < iterations; i++ {
		start := time.Now()
		data, err := generator(dataSize)
		duration := time.Since(start)

		if err != nil {
			fmt.Printf("Error generating bytes for %s: %v\n", name, err)
			continue
		}

		totalTime += duration
		totalChiSquare += calculateChiSquare(data)
		totalShannonEntropy += calculateShannonEntropy(data)

		nistPValue := nistFrequencyMonobitTest(data)
		totalNISTMonobitPValue += nistPValue
		if nistPValue >= 0.01 {
			passedNIST++
		}
	}

	avgOps := float64(iterations) / totalTime.Seconds()

	return TestResult{
		Name:                 name,
		TotalTime:            totalTime,
		AvgOpsPerSec:         avgOps,
		AvgChiSquare:         totalChiSquare / float64(iterations),
		AvgShannonEntropy:    totalShannonEntropy / float64(iterations),
		AvgNISTMonobitPValue: totalNISTMonobitPValue / float64(iterations),
		PassedNISTMonobit:    passedNIST,
		TotalIterations:      iterations,
	}
}

func printResults(results []TestResult) {
	fmt.Printf("\n--- Benchmark Results ---\n\n")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.Debug)
	fmt.Fprintln(w, "Generator\tAvg Ops/sec\tAvg Chi-Square\tAvg Shannon\tNIST Monobit Pass Rate\tAvg NIST P-Value")
	fmt.Fprintln(w, "---------\t------------\t--------------\t-------------\t----------------------\t----------------")

	for _, r := range results {
		passRate := fmt.Sprintf("%.2f%% (%d/%d)", (float64(r.PassedNISTMonobit)/float64(r.TotalIterations))*100, r.PassedNISTMonobit, r.TotalIterations)
		fmt.Fprintf(w, "%s\t%.2f\t%.4f\t%.4f\t%s\t%.6f\n",
			r.Name,
			r.AvgOpsPerSec,
			r.AvgChiSquare,
			r.AvgShannonEntropy,
			passRate,
			r.AvgNISTMonobitPValue,
		)
	}
	w.Flush()
	fmt.Println("\n--- Explanations ---")
	fmt.Println("* Ops/sec: Higher is better (faster performance).")
	fmt.Println("* Chi-Square: Measures uniformity. For 256 bytes, values closer to 255.0 are ideal. Extreme values can indicate bias.")
	fmt.Println("* Shannon Entropy: Measures unpredictability. Ideal for random bytes is 8.0. Higher is better.")
	fmt.Println("* NIST Monobit Pass Rate: Shows how often the generator passed the NIST Frequency test (p-value >= 0.01). Higher is better.")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
