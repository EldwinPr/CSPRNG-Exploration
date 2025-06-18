# RNG/custom_CSPRNG.py - Improved Custom Hash-based CSPRNG implementation
import hashlib
import time
import requests
import json
import threading
from concurrent.futures import ThreadPoolExecutor, as_completed

class CustomCSPRNG:
    """Custom CSPRNG based on Hash_DRBG design using real-world data"""
    
    def __init__(self, seed=None):
        self.name = "Custom CSPRNG"
        
        # Generate entropy-rich seed if none provided
        if seed is None:
            seed = self._gather_entropy()
        
        # Initialize internal state
        self.state = hashlib.sha256(seed).digest()
        self.counter = 0
        
        # Cache for entropy to avoid repeated network calls
        self._entropy_cache = {}
        self._cache_timestamp = 0
        self._cache_duration = 60  # Cache for 60 seconds
    
    def _gather_entropy(self):
        """Gather entropy from weather, market data, and network timing with improved methods"""
        print("Gathering entropy from real-world sources...")
        
        # Use concurrent requests for faster entropy gathering
        entropy_sources = []
        
        with ThreadPoolExecutor(max_workers=3, thread_name_prefix="entropy") as executor:
            # Submit all entropy gathering tasks concurrently
            future_weather = executor.submit(self._get_weather_entropy)
            future_market = executor.submit(self._get_market_entropy)
            future_network = executor.submit(self._get_network_entropy)
            
            # Collect results as they complete
            futures = [future_weather, future_market, future_network]
            for future in as_completed(futures, timeout=10):
                try:
                    result = future.result()
                    entropy_sources.append(result)
                except Exception as e:
                    print(f"Entropy source failed: {e}")
                    entropy_sources.append(f"failed_{time.time_ns()}".encode())
        
        # Add local high-precision timing entropy
        entropy_sources.append(str(time.time_ns()).encode())
        entropy_sources.append(str(time.perf_counter_ns()).encode())
        
        # Concatenate all entropy sources with separators
        separator = b"|ENTROPY_SEP|"
        combined = separator.join(entropy_sources)
        
        # Multi-round hashing for better mixing
        hash1 = hashlib.sha256(combined).digest()
        hash2 = hashlib.sha256(hash1 + combined).digest()
        final_seed = hashlib.sha256(hash2 + hash1).digest()
        
        print(f"Final entropy seed length: {len(final_seed)} bytes")
        return final_seed
    
    def _get_weather_entropy(self):
        """Get entropy from weather data with better error handling"""
        try:
            # Multiple weather endpoints for redundancy
            endpoints = [
                'https://wttr.in/?format=j1',
                'https://wttr.in/London?format=j1',
                'https://api.openweathermap.org/data/2.5/weather?q=London&appid=demo'  # Demo endpoint
            ]
            
            for endpoint in endpoints:
                try:
                    response = requests.get(endpoint, timeout=3)
                    if response.status_code == 200:
                        data = response.json()
                        
                        # Extract all numeric values from weather data
                        weather_values = []
                        if 'current_condition' in data:
                            # wttr.in format
                            current = data['current_condition'][0]
                            weather_values.extend([
                                current.get('temp_C', '0'),
                                current.get('temp_F', '0'),
                                current.get('humidity', '0'),
                                current.get('pressure', '0'),
                                current.get('windspeedKmph', '0'),
                                current.get('winddirDegree', '0'),
                                current.get('visibility', '0'),
                                current.get('cloudcover', '0')
                            ])
                        elif 'main' in data:
                            # OpenWeather format
                            main = data['main']
                            weather_values.extend([
                                str(main.get('temp', 0)),
                                str(main.get('humidity', 0)),
                                str(main.get('pressure', 0)),
                                str(data.get('wind', {}).get('speed', 0)),
                                str(data.get('wind', {}).get('deg', 0))
                            ])
                        
                        # Concatenate with timestamps
                        entropy_data = ''.join(weather_values) + str(time.time_ns())
                        print(f"Weather entropy: {len(entropy_data)} chars from {endpoint.split('/')[2]}")
                        return entropy_data.encode()
                        
                except requests.RequestException:
                    continue
            
            raise Exception("All weather endpoints failed")
            
        except Exception as e:
            print(f"Weather entropy failed: {e}")
            # Fallback: use multiple timestamp variations
            fallback = f"weather_failed_{time.time_ns()}_{time.perf_counter_ns()}_{id(object())}"
            return fallback.encode()
    
    def _get_market_entropy(self):
        """Get entropy from financial market data with multiple sources"""
        try:
            # Multiple market data sources
            market_data = []
            
            # CoinGecko API
            try:
                response = requests.get(
                    'https://api.coingecko.com/api/v3/simple/price?ids=bitcoin,ethereum,cardano&vs_currencies=usd',
                    timeout=3
                )
                if response.status_code == 200:
                    data = response.json()
                    for coin, price_data in data.items():
                        market_data.append(str(price_data.get('usd', 0)))
                    print(f"Market entropy: Got {len(data)} crypto prices")
            except:
                pass
            
            # Coinbase API as backup
            try:
                response = requests.get('https://api.coinbase.com/v2/exchange-rates?currency=BTC', timeout=3)
                if response.status_code == 200:
                    data = response.json()
                    rates = data.get('data', {}).get('rates', {})
                    market_data.extend([rates.get('USD', '0'), rates.get('EUR', '0')])
                    print("Market entropy: Got Coinbase rates")
            except:
                pass
            
            if market_data:
                # Combine all market data with timestamps
                entropy_data = ''.join(market_data) + str(time.time_ns()) + str(time.perf_counter_ns())
                return entropy_data.encode()
            else:
                raise Exception("No market data sources available")
                
        except Exception as e:
            print(f"Market entropy failed: {e}")
            # Fallback: use multiple timestamp variations
            fallback = f"market_failed_{time.time_ns()}_{time.perf_counter_ns()}_{threading.get_ident()}"
            return fallback.encode()
    
    def _get_network_entropy(self):
        """Get entropy from network timing and system variations"""
        try:
            # Use faster, more reliable endpoints
            endpoints = [
                'https://httpbin.org/uuid',
                'https://api.github.com/zen',
                'https://jsonplaceholder.typicode.com/posts/1'
            ]
            
            timings = []
            responses = []
            
            for endpoint in endpoints:
                try:
                    start = time.perf_counter_ns()
                    response = requests.get(endpoint, timeout=1.5)  # Shorter timeout
                    end = time.perf_counter_ns()
                    
                    if response.status_code == 200:
                        timing = end - start
                        timings.append(str(timing))
                        
                        # Extract some response data for additional entropy
                        content = response.text[:100]  # First 100 chars
                        responses.append(content)
                        
                        print(f"Network entropy: {timing} ns from {endpoint.split('/')[2]}")
                        break  # Got one successful response
                        
                except requests.RequestException:
                    continue
            
            if timings:
                # Combine timing data with thread info and memory addresses
                entropy_data = (
                    ''.join(timings) + 
                    ''.join(responses) + 
                    str(time.time_ns()) + 
                    str(threading.get_ident()) +
                    str(id(timings)) +
                    str(hash(str(timings)))
                )
                return entropy_data.encode()
            else:
                raise Exception("All network endpoints failed")
                
        except Exception as e:
            print(f"Network entropy failed: {e}")
            # Fallback: use system-level timing variations
            fallback_data = []
            for i in range(10):
                start = time.perf_counter_ns()
                # Small computation to create timing variation
                hash_obj = hashlib.sha256(str(i).encode())
                end = time.perf_counter_ns()
                fallback_data.append(str(end - start))
            
            fallback = 'network_failed_' + ''.join(fallback_data) + str(time.time_ns())
            return fallback.encode()
    
    def generate_bytes(self, num_bytes):
        """Generate bytes using improved Hash_DRBG algorithm"""
        result = bytearray()
        
        while len(result) < num_bytes:
            # Create more complex hash input
            hash_input = (
                self.state + 
                self.counter.to_bytes(8, 'big') + 
                b'CSPRNG_OUTPUT' +
                str(time.perf_counter_ns()).encode()[-8:]  # Last 8 chars of high-precision time
            )
            
            # Generate output block with double hashing
            temp_hash = hashlib.sha256(hash_input).digest()
            output_block = hashlib.sha256(temp_hash + hash_input).digest()
            
            # Add to result
            result.extend(output_block)
            
            # Update state for forward security with more complexity
            state_update_input = (
                self.state + 
                b'STATE_UPDATE' + 
                self.counter.to_bytes(8, 'big') +
                output_block[:8]  # Feedback from output
            )
            self.state = hashlib.sha256(state_update_input).digest()
            self.counter += 1
        
        # Return exact number of bytes requested
        return bytes(result[:num_bytes])