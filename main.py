import os
import time
import timeit
from pathlib import Path

# Import our RNG classes
from RNG.rand import InsecurePRNG
from RNG.CSPRNG import SystemCSPRNG
from RNG.custom_CSPRNG import CustomCSPRNG

def performance_test(generator, test_size=1024*1024):  # 1MB test
    """Test generator performance"""
    def test_func():
        return generator.generate_bytes(test_size)
    
    # Time the generation
    start_time = time.time()
    data = test_func()
    end_time = time.time()
    
    duration = end_time - start_time
    throughput = test_size / duration / (1024*1024)  # MB/s
    
    return data, duration, throughput

def basic_analysis(data):
    """Basic statistical analysis of generated data"""
    if not data:
        return "No data to analyze"
    
    # Byte frequency analysis
    byte_counts = [0] * 256
    for byte in data:
        byte_counts[byte] += 1
    
    # Calculate uniformity metrics
    expected_freq = len(data) / 256
    chi_square = sum((count - expected_freq) ** 2 / expected_freq for count in byte_counts)
    
    mean_byte = sum(data) / len(data)
    min_freq = min(byte_counts)
    max_freq = max(byte_counts)
    
    return {
        'length': len(data),
        'mean': mean_byte,
        'chi_square': chi_square,
        'min_freq': min_freq,
        'max_freq': max_freq,
        'freq_range': max_freq - min_freq
    }

def save_sample(data, filename, sample_size=10000):
    """Save a sample of data to file for external analysis"""
    sample = data[:sample_size]
    with open(filename, 'wb') as f:
        f.write(sample)

def main():
    print("=" * 60)
    print("CSPRNG Comparison Tool")
    print("=" * 60)
    
    # Create output directory
    Path("output").mkdir(exist_ok=True)
    
    # Initialize generators
    generators = [
        InsecurePRNG(),
        SystemCSPRNG(),
        CustomCSPRNG()
    ]
    
    test_size = 100000  # 100KB for quick testing
    results = []
    
    for generator in generators:
        print(f"\nTesting {generator.name}...")
        print("-" * 40)
        
        try:
            # Performance test
            data, duration, throughput = performance_test(generator, test_size)
            
            # Basic analysis
            analysis = basic_analysis(data)
            
            # Save sample for external analysis
            filename = f"output/{generator.name.lower().replace(' ', '_')}_sample.bin"
            save_sample(data, filename)
            
            # Store results
            result = {
                'name': generator.name,
                'duration': duration,
                'throughput': throughput,
                'analysis': analysis,
                'filename': filename
            }
            results.append(result)
            
            # Print immediate results
            print(f"Duration: {duration:.4f} seconds")
            print(f"Throughput: {throughput:.2f} MB/s")
            print(f"Mean byte value: {analysis['mean']:.2f} (expected: 127.5)")
            print(f"Chi-square: {analysis['chi_square']:.2f}")
            print(f"Frequency range: {analysis['freq_range']}")
            print(f"Sample saved to: {filename}")
            
        except Exception as e:
            print(f"Error testing {generator.name}: {e}")
    
    # Summary comparison
    print("\n" + "=" * 60)
    print("COMPARISON SUMMARY")
    print("=" * 60)
    
    print(f"{'Generator':<20} {'Speed (MB/s)':<12} {'Chi-square':<12} {'Freq Range':<12}")
    print("-" * 60)
    
    for result in results:
        name = result['name'][:19]
        speed = result['throughput']
        chi_sq = result['analysis']['chi_square']
        freq_range = result['analysis']['freq_range']
        
        print(f"{name:<20} {speed:<12.2f} {chi_sq:<12.2f} {freq_range:<12}")
    
    print("\nNotes:")
    print("- Lower chi-square values indicate better uniformity")
    print("- Smaller frequency ranges indicate more even distribution") 
    print("- System CSPRNG should have best statistical properties")
    print("- Custom CSPRNG should be comparable to System CSPRNG")
    print("- Insecure PRNG may show patterns or weaknesses")

if __name__ == "__main__":
    main()