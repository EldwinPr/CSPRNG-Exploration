package main

import (
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Generator interface for all RNG types
type Generator interface {
	Name() string
	GenerateBytes(numBytes int) ([]byte, error)
}

// TestResult holds individual test results
type TestResult struct {
	Name       string
	TestRun    int
	Duration   time.Duration
	Throughput float64
	Analysis   Analysis
	Filename   string
	Error      error
}

// AggregatedResult holds aggregated test results
type AggregatedResult struct {
	Name               string
	TotalTests         int
	SuccessfulTests    int
	FailedTests        int
	AvgDuration        time.Duration
	MinDuration        time.Duration
	MaxDuration        time.Duration
	AvgThroughput      float64
	MinThroughput      float64
	MaxThroughput      float64
	AvgAnalysis        Analysis
	TotalDataGenerated int64
}

// Analysis holds statistical analysis results
type Analysis struct {
	Length          int
	Mean            float64
	ChiSquare       float64
	MinFreq         int
	MaxFreq         int
	FreqRange       int
	ShannonEntropy  float64
	Autocorrelation float64
}

// ProgressTracker tracks test progress
type ProgressTracker struct {
	completed int64
	total     int64
	mu        sync.Mutex
	lastPrint time.Time
}

func (pt *ProgressTracker) increment() {
	atomic.AddInt64(&pt.completed, 1)

	// Print progress every 5 seconds
	pt.mu.Lock()
	now := time.Now()
	if now.Sub(pt.lastPrint) > 5*time.Second {
		completed := atomic.LoadInt64(&pt.completed)
		percentage := float64(completed) / float64(pt.total) * 100
		fmt.Printf("Progress: %d/%d tests completed (%.1f%%)\n", completed, pt.total, percentage)
		pt.lastPrint = now
	}
	pt.mu.Unlock()
}

// performanceTest tests generator performance
func performanceTest(generator Generator, testSize int) ([]byte, time.Duration, float64, error) {
	start := time.Now()
	data, err := generator.GenerateBytes(testSize)
	if err != nil {
		return nil, 0, 0, err
	}
	duration := time.Since(start)

	// Ensure minimum measurable duration
	if duration < time.Microsecond {
		duration = time.Microsecond
	}

	throughputMBs := float64(testSize) / duration.Seconds() / (1024 * 1024)
	return data, duration, throughputMBs, nil
}

// basicAnalysis performs enhanced statistical analysis
func basicAnalysis(data []byte) Analysis {
	if len(data) == 0 {
		return Analysis{}
	}

	// Byte frequency analysis
	byteCounts := make([]int, 256)
	for _, b := range data {
		byteCounts[b]++
	}

	// Calculate uniformity metrics
	expectedFreq := float64(len(data)) / 256.0
	chiSquare := 0.0
	for _, count := range byteCounts {
		diff := float64(count) - expectedFreq
		chiSquare += (diff * diff) / expectedFreq
	}

	// Calculate mean
	sum := 0
	for _, b := range data {
		sum += int(b)
	}
	mean := float64(sum) / float64(len(data))

	// Find min/max frequencies
	minFreq := byteCounts[0]
	maxFreq := byteCounts[0]
	for i := 0; i < 256; i++ {
		count := byteCounts[i]
		if count < minFreq {
			minFreq = count
		}
		if count > maxFreq {
			maxFreq = count
		}
	}
	freqRange := maxFreq - minFreq

	// Calculate Shannon entropy
	shannon := 0.0
	for _, count := range byteCounts {
		if count > 0 {
			prob := float64(count) / float64(len(data))
			shannon -= prob * math.Log2(prob)
		}
	}

	// Calculate autocorrelation (lag-1) - optimized for performance
	autocorr := 0.0
	sampleSize := len(data)
	if sampleSize > 50000 {
		sampleSize = 50000 // Reduced for better performance
	}
	matches := 0
	for i := 1; i < sampleSize; i++ {
		if data[i] == data[i-1] {
			matches++
		}
	}
	autocorr = float64(matches) / float64(sampleSize-1)

	return Analysis{
		Length:          len(data),
		Mean:            mean,
		ChiSquare:       chiSquare,
		MinFreq:         minFreq,
		MaxFreq:         maxFreq,
		FreqRange:       freqRange,
		ShannonEntropy:  shannon,
		Autocorrelation: autocorr,
	}
}

// saveSample saves a sample of data to file
func saveSample(data []byte, filename string, sampleSize int) error {
	if len(data) < sampleSize {
		sampleSize = len(data)
	}
	sample := data[:sampleSize]
	return os.WriteFile(filename, sample, 0644)
}

// runSingleTest runs a single test for a generator with retries
func runSingleTest(generator Generator, testSize int, testRun int, resultsChannel chan<- TestResult, tracker *ProgressTracker) {
	const maxRetries = 2
	var data []byte
	var duration time.Duration
	var throughput float64
	var err error

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic occurred: %v", r)
		}

		var analysis Analysis
		if data != nil {
			analysis = basicAnalysis(data)
		}

		result := TestResult{
			Name:       generator.Name(),
			TestRun:    testRun,
			Duration:   duration,
			Throughput: throughput,
			Analysis:   analysis,
			Filename:   fmt.Sprintf("output/%s_sample_run%d.bin", strings.ToLower(strings.ReplaceAll(generator.Name(), " ", "_")), testRun),
			Error:      err,
		}

		resultsChannel <- result
		tracker.increment()
	}()

	// Attempt test with retries
	for attempt := 0; attempt <= maxRetries; attempt++ {
		data, duration, throughput, err = performanceTest(generator, testSize)
		if err == nil {
			break // Success
		}

		if attempt < maxRetries {
			backoff := time.Duration(attempt+1) * 100 * time.Millisecond
			time.Sleep(backoff)
		}
	}

	// Save sample if successful (only save samples for first 10 runs to reduce I/O)
	if err == nil && testRun <= 10 {
		filename := fmt.Sprintf("output/%s_sample_run%d.bin",
			strings.ToLower(strings.ReplaceAll(generator.Name(), " ", "_")), testRun)
		saveSample(data, filename, 10000) // Ignore save errors to avoid cluttering output
	}
}

// aggregateResults aggregates multiple test results
func aggregateResults(results []TestResult) map[string]AggregatedResult {
	aggregated := make(map[string]AggregatedResult)

	for _, result := range results {
		name := result.Name
		agg, exists := aggregated[name]
		if !exists {
			agg = AggregatedResult{
				Name:          name,
				MinDuration:   time.Hour,
				MaxDuration:   0,
				MinThroughput: 1e9,
				MaxThroughput: 0,
			}
		}

		agg.TotalTests++
		if result.Error == nil {
			agg.SuccessfulTests++
			agg.TotalDataGenerated += int64(result.Analysis.Length)

			// Update duration stats
			if result.Duration < agg.MinDuration {
				agg.MinDuration = result.Duration
			}
			if result.Duration > agg.MaxDuration {
				agg.MaxDuration = result.Duration
			}

			// Update throughput stats
			if result.Throughput < agg.MinThroughput {
				agg.MinThroughput = result.Throughput
			}
			if result.Throughput > agg.MaxThroughput {
				agg.MaxThroughput = result.Throughput
			}
		} else {
			agg.FailedTests++
		}

		aggregated[name] = agg
	}

	// Calculate averages
	for name, agg := range aggregated {
		if agg.SuccessfulTests > 0 {
			var totalDuration time.Duration
			var totalThroughput float64
			var totalAnalysis Analysis

			successCount := 0
			for _, result := range results {
				if result.Name == name && result.Error == nil {
					totalDuration += result.Duration
					totalThroughput += result.Throughput
					totalAnalysis.Length += result.Analysis.Length
					totalAnalysis.Mean += result.Analysis.Mean
					totalAnalysis.ChiSquare += result.Analysis.ChiSquare
					totalAnalysis.MinFreq += result.Analysis.MinFreq
					totalAnalysis.MaxFreq += result.Analysis.MaxFreq
					totalAnalysis.FreqRange += result.Analysis.FreqRange
					totalAnalysis.ShannonEntropy += result.Analysis.ShannonEntropy
					totalAnalysis.Autocorrelation += result.Analysis.Autocorrelation
					successCount++
				}
			}

			if successCount > 0 {
				agg.AvgDuration = totalDuration / time.Duration(successCount)
				agg.AvgThroughput = totalThroughput / float64(successCount)
				agg.AvgAnalysis = Analysis{
					Mean:            totalAnalysis.Mean / float64(successCount),
					ChiSquare:       totalAnalysis.ChiSquare / float64(successCount),
					MinFreq:         totalAnalysis.MinFreq / successCount,
					MaxFreq:         totalAnalysis.MaxFreq / successCount,
					FreqRange:       totalAnalysis.FreqRange / successCount,
					ShannonEntropy:  totalAnalysis.ShannonEntropy / float64(successCount),
					Autocorrelation: totalAnalysis.Autocorrelation / float64(successCount),
				}
			}
		}
		aggregated[name] = agg
	}

	return aggregated
}

// saveResultsToCSV saves detailed results to CSV
func saveResultsToCSV(results []TestResult, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{
		"Generator", "TestRun", "Duration_ms", "Throughput_MBps",
		"Mean", "ChiSquare", "MinFreq", "MaxFreq", "FreqRange",
		"ShannonEntropy", "Autocorrelation",
		"DataLength", "SampleFile", "Error",
	}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Write data
	for _, result := range results {
		errorStr := ""
		if result.Error != nil {
			errorStr = result.Error.Error()
		}

		record := []string{
			result.Name,
			strconv.Itoa(result.TestRun),
			strconv.FormatFloat(float64(result.Duration.Nanoseconds())/1000000, 'f', 2, 64),
			strconv.FormatFloat(result.Throughput, 'f', 2, 64),
			strconv.FormatFloat(result.Analysis.Mean, 'f', 2, 64),
			strconv.FormatFloat(result.Analysis.ChiSquare, 'f', 2, 64),
			strconv.Itoa(result.Analysis.MinFreq),
			strconv.Itoa(result.Analysis.MaxFreq),
			strconv.Itoa(result.Analysis.FreqRange),
			strconv.FormatFloat(result.Analysis.ShannonEntropy, 'f', 4, 64),
			strconv.FormatFloat(result.Analysis.Autocorrelation, 'f', 6, 64),
			strconv.Itoa(result.Analysis.Length),
			result.Filename,
			errorStr,
		}

		if err := writer.Write(record); err != nil {
			return err
		}
	}

	return nil
}

// saveAggregatedToCSV saves aggregated results to CSV
func saveAggregatedToCSV(aggregated map[string]AggregatedResult, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{
		"Generator", "TotalTests", "SuccessfulTests", "FailedTests",
		"AvgDuration_ms", "MinDuration_ms", "MaxDuration_ms",
		"AvgThroughput_MBps", "MinThroughput_MBps", "MaxThroughput_MBps",
		"AvgMean", "AvgChiSquare", "AvgFreqRange",
		"AvgShannonEntropy", "AvgAutocorrelation",
		"TotalDataGenerated_MB",
	}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Write data
	for _, agg := range aggregated {
		record := []string{
			agg.Name,
			strconv.Itoa(agg.TotalTests),
			strconv.Itoa(agg.SuccessfulTests),
			strconv.Itoa(agg.FailedTests),
			strconv.FormatFloat(float64(agg.AvgDuration.Nanoseconds())/1000000, 'f', 2, 64),
			strconv.FormatFloat(float64(agg.MinDuration.Nanoseconds())/1000000, 'f', 2, 64),
			strconv.FormatFloat(float64(agg.MaxDuration.Nanoseconds())/1000000, 'f', 2, 64),
			strconv.FormatFloat(agg.AvgThroughput, 'f', 2, 64),
			strconv.FormatFloat(agg.MinThroughput, 'f', 2, 64),
			strconv.FormatFloat(agg.MaxThroughput, 'f', 2, 64),
			strconv.FormatFloat(agg.AvgAnalysis.Mean, 'f', 2, 64),
			strconv.FormatFloat(agg.AvgAnalysis.ChiSquare, 'f', 2, 64),
			strconv.Itoa(agg.AvgAnalysis.FreqRange),
			strconv.FormatFloat(agg.AvgAnalysis.ShannonEntropy, 'f', 4, 64),
			strconv.FormatFloat(agg.AvgAnalysis.Autocorrelation, 'f', 6, 64),
			strconv.FormatFloat(float64(agg.TotalDataGenerated)/(1024*1024), 'f', 2, 64),
		}

		if err := writer.Write(record); err != nil {
			return err
		}
	}

	return nil
}

// displayRealTimeResults shows results as they come in
func displayRealTimeResults(results []TestResult, generators []Generator) {
	if len(results) == 0 {
		return
	}

	// Count results per generator
	counts := make(map[string]int)
	successes := make(map[string]int)
	avgThroughput := make(map[string]float64)
	avgChiSquare := make(map[string]float64)
	avgEntropy := make(map[string]float64)

	for _, result := range results {
		counts[result.Name]++
		if result.Error == nil {
			successes[result.Name]++
			avgThroughput[result.Name] += result.Throughput
			avgChiSquare[result.Name] += result.Analysis.ChiSquare
			avgEntropy[result.Name] += result.Analysis.ShannonEntropy
		}
	}

	// Calculate averages
	for name := range successes {
		if successes[name] > 0 {
			avgThroughput[name] /= float64(successes[name])
			avgChiSquare[name] /= float64(successes[name])
			avgEntropy[name] /= float64(successes[name])
		}
	}

	// Clear screen and show current status
	fmt.Print("\033[2J\033[H") // Clear screen and move cursor to top
	fmt.Printf("Real-time Results Summary (Last updated: %s)\n", time.Now().Format("15:04:05"))
	fmt.Println(strings.Repeat("=", 90))
	fmt.Printf("%-25s %-10s %-12s %-12s %-10s\n", "Generator", "Completed", "Avg MB/s", "Avg χ²", "Avg H")
	fmt.Println(strings.Repeat("-", 90))

	for _, gen := range generators {
		name := gen.Name()
		completed := counts[name]
		successful := successes[name]

		statusStr := fmt.Sprintf("%d/%d", successful, completed)
		throughputStr := fmt.Sprintf("%.2f", avgThroughput[name])
		chiStr := fmt.Sprintf("%.1f", avgChiSquare[name])
		entropyStr := fmt.Sprintf("%.3f", avgEntropy[name])

		if successful == 0 {
			throughputStr = "N/A"
			chiStr = "N/A"
			entropyStr = "N/A"
		}

		fmt.Printf("%-25s %-10s %-12s %-12s %-10s\n",
			name, statusStr, throughputStr, chiStr, entropyStr)
	}
}

func main() {
	totalStartTime := time.Now()

	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("ENHANCED CSPRNG COMPARISON TOOL")
	fmt.Println(strings.Repeat("=", 80))

	// Configurable test parameters
	testSize := 1024 * 1024 // 1 MB
	numRuns := 1000         // Number of runs per generator
	maxConcurrency := 10    // Maximum concurrent tests

	// Allow override via command line
	if len(os.Args) > 1 {
		if runs, err := strconv.Atoi(os.Args[1]); err == nil && runs > 0 {
			numRuns = runs
		}
	}
	if len(os.Args) > 2 {
		if size, err := strconv.Atoi(os.Args[2]); err == nil && size > 0 {
			testSize = size * 1024 * 1024 // Convert MB to bytes
		}
	}

	fmt.Printf("Test Configuration:\n")
	fmt.Printf("- Test size: %d MB\n", testSize/(1024*1024))
	fmt.Printf("- Number of runs per generator: %d\n", numRuns)
	fmt.Printf("- Maximum concurrent tests: %d\n", maxConcurrency)
	fmt.Printf("- Sample files: Saving first 10 runs only\n")
	fmt.Printf("- Progress updates: Every 5 seconds\n")

	// Initialize generators
	generators := []Generator{
		NewInsecurePRNG(),
		NewSystemCSPRNG(),
		NewCustomCSPRNG(),
		NewWeatherCSPRNG(),
		NewHybridCSPRNG(),
	}

	totalTests := numRuns * len(generators)
	fmt.Printf("- Total tests: %d\n", totalTests)
	fmt.Println()

	// Create output directory
	if err := os.MkdirAll("output", 0755); err != nil {
		fmt.Printf("Error creating output directory: %v\n", err)
		return
	}

	// Progress tracking
	tracker := &ProgressTracker{
		total:     int64(totalTests),
		lastPrint: time.Now(),
	}

	// Channel for collecting results
	resultsChannel := make(chan TestResult, totalTests)

	// Semaphore for controlling concurrency
	semaphore := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup

	// Start all tests
	testStartTime := time.Now()
	fmt.Printf("Starting %d test runs at %s\n", totalTests, testStartTime.Format("15:04:05"))
	fmt.Println("Use Ctrl+C to stop early if needed")
	fmt.Println()

	for _, generator := range generators {
		for run := 1; run <= numRuns; run++ {
			wg.Add(1)
			go func(gen Generator, testRun int) {
				defer wg.Done()

				// Acquire semaphore
				semaphore <- struct{}{}
				defer func() { <-semaphore }()

				runSingleTest(gen, testSize, testRun, resultsChannel, tracker)
			}(generator, run)
		}
	}

	// Collect results in background
	var allResults []TestResult
	resultsDone := make(chan bool)

	go func() {
		for result := range resultsChannel {
			allResults = append(allResults, result)

			// Show real-time results every 100 completed tests
			if len(allResults)%100 == 0 {
				displayRealTimeResults(allResults, generators)
			}
		}
		resultsDone <- true
	}()

	// Wait for all tests to complete and close channel
	go func() {
		wg.Wait()
		close(resultsChannel)
	}()

	// Wait for results collection to complete
	<-resultsDone

	testDuration := time.Since(testStartTime)

	// Clear screen and show final results
	fmt.Print("\033[2J\033[H")
	fmt.Printf("All %d tests completed in: %v\n", len(allResults), testDuration)
	fmt.Println()

	// Count errors
	errorCount := 0
	for _, result := range allResults {
		if result.Error != nil {
			errorCount++
		}
	}
	if errorCount > 0 {
		fmt.Printf("⚠️  %d tests failed with errors\n", errorCount)
		fmt.Println()
	}

	// Save detailed results to CSV
	detailedCSV := "output/detailed_results.csv"
	if err := saveResultsToCSV(allResults, detailedCSV); err != nil {
		fmt.Printf("Error saving detailed results: %v\n", err)
	} else {
		fmt.Printf("✅ Detailed results saved to: %s\n", detailedCSV)
	}

	// Aggregate results
	aggregated := aggregateResults(allResults)

	// Save aggregated results to CSV
	aggregatedCSV := "output/aggregated_results.csv"
	if err := saveAggregatedToCSV(aggregated, aggregatedCSV); err != nil {
		fmt.Printf("Error saving aggregated results: %v\n", err)
	} else {
		fmt.Printf("✅ Aggregated results saved to: %s\n", aggregatedCSV)
	}

	// Display final summary
	fmt.Printf("\n%s\n", strings.Repeat("=", 80))
	fmt.Println("FINAL PERFORMANCE SUMMARY")
	fmt.Printf("%s\n", strings.Repeat("=", 80))

	fmt.Printf("%-25s %-8s %-12s %-12s %-12s %-12s %-10s\n",
		"Generator", "Success", "Avg MB/s", "Min MB/s", "Max MB/s", "Avg χ²", "Avg H")
	fmt.Println(strings.Repeat("-", 90))

	for _, gen := range generators {
		if agg, exists := aggregated[gen.Name()]; exists {
			successRate := float64(agg.SuccessfulTests) / float64(agg.TotalTests) * 100
			fmt.Printf("%-25s %6.1f%% %12.2f %12.2f %12.2f %12.2f %10.3f\n",
				agg.Name,
				successRate,
				agg.AvgThroughput,
				agg.MinThroughput,
				agg.MaxThroughput,
				agg.AvgAnalysis.ChiSquare,
				agg.AvgAnalysis.ShannonEntropy)
		}
	}

	totalDuration := time.Since(totalStartTime)
	fmt.Printf("\n%s\n", strings.Repeat("=", 80))
	fmt.Printf("TOTAL RUNTIME: %v\n", totalDuration)
	fmt.Printf("TEST EXECUTION TIME: %v\n", testDuration)
	fmt.Printf("OVERHEAD TIME: %v\n", totalDuration-testDuration)
	fmt.Printf("AVERAGE TIME PER TEST: %v\n", testDuration/time.Duration(len(allResults)))
	fmt.Printf("%s\n", strings.Repeat("=", 80))

	fmt.Println("\nOutput Files:")
	fmt.Printf("📊 %s (detailed test results)\n", detailedCSV)
	fmt.Printf("📈 %s (aggregated statistics)\n", aggregatedCSV)
	fmt.Println("🗂️  output/*_sample_run*.bin (binary samples, first 10 runs only)")

	fmt.Println("\n📖 Key Metrics Guide:")
	fmt.Println("• χ² (Chi-Square): Lower = better uniformity (ideal ~255)")
	fmt.Println("• H (Shannon Entropy): Higher = better (max 8.0 bits/byte)")
	fmt.Println("• Autocorrelation: Lower = better (measures serial dependency)")
	fmt.Println("• Success Rate: Percentage of tests that completed without errors")
}
