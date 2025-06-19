package main

import (
	"fmt"
	"math"
	"os"
	"text/tabwriter"
	"time"
)

// TestResult holds benchmark results
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

type Generator interface {
	Name() string
	GenerateBytes(numBytes int) ([]byte, error)
}

// Statistical analysis functions
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
	return math.Erfc(sObs / math.Sqrt2)
}

// Progress indicator
func showProgress(current, total int, name string) {
	percent := float64(current) / float64(total) * 100
	bar := make([]byte, 20)
	filled := int(percent / 5)

	for i := 0; i < len(bar); i++ {
		if i < filled {
			bar[i] = '|'
		} else {
			bar[i] = '_'
		}
	}

	fmt.Printf("\r%s: [%s] %.1f%% (%d/%d)", name, string(bar), percent, current, total)
}

// Benchmark runner
func runBenchmark(generator Generator, iterations, dataSize int) TestResult {
	var totalTime time.Duration
	var totalChiSquare, totalShannonEntropy, totalNISTMonobitPValue float64
	var passedNIST int

	name := generator.Name()
	fmt.Printf("\nTesting: %s\n", name)

	for i := 0; i < iterations; i++ {
		if i%100 == 0 || i == iterations-1 {
			showProgress(i+1, iterations, name)
		}

		start := time.Now()
		data, err := generator.GenerateBytes(dataSize)
		duration := time.Since(start)

		if err != nil {
			fmt.Printf("\nError generating bytes for %s: %v\n", name, err)
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

	fmt.Printf(" ✓\n")
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

// Results printer
func printResults(results []TestResult) {
	fmt.Printf("\n=== Benchmark Results ===\n\n")

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Generator\tOps/sec\tChi-Square\tShannon\tNIST Pass Rate\tNIST P-Value")
	fmt.Fprintln(w, "---------\t-------\t----------\t-------\t--------------\t------------")

	for _, r := range results {
		passRate := fmt.Sprintf("%.1f%%", (float64(r.PassedNISTMonobit)/float64(r.TotalIterations))*100)
		fmt.Fprintf(w, "%s\t%.0f\t%.2f\t%.4f\t%s\t%.4f\n",
			r.Name,
			r.AvgOpsPerSec,
			r.AvgChiSquare,
			r.AvgShannonEntropy,
			passRate,
			r.AvgNISTMonobitPValue,
		)
	}
	w.Flush()

	fmt.Println("\n=== Quality Metrics ===")
	fmt.Println("• Ops/sec: Higher = better performance")
	fmt.Println("• Chi-Square: ~255 = good uniformity")
	fmt.Println("• Shannon: ~8.0 = maximum entropy")
	fmt.Println("• NIST Pass: >95% = good randomness")
}

func main() {
	// Configuration
	iterations := 10000
	dataSize := 1024 * 16

	fmt.Printf("🎲 PRNG Benchmark Suite\n")
	fmt.Printf("Iterations: %d | Data Size: %d bytes\n", iterations, dataSize)

	// Initialize generators
	fmt.Println("\n📊 Initializing generators...")
	generators := []Generator{
		NewCryptoCSPRNG(),
		NewMathPRNG(),
		NewWeatherCSPRNG(),
		NewmultEntropyCSPRNG(),
		NewHybridCSPRNG(),
	}

	// Run benchmarks
	fmt.Println("\n🚀 Starting benchmarks...")
	start := time.Now()

	var results []TestResult
	for _, gen := range generators {
		result := runBenchmark(gen, iterations, dataSize)
		results = append(results, result)
	}

	totalTime := time.Since(start)

	// Print results
	printResults(results)
	fmt.Printf("\n⏱️  Total benchmark time: %v\n", totalTime.Round(time.Millisecond))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
