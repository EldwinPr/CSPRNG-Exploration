import random
import time

class InsecurePRNG:
    """Simple insecure PRNG using Python's random module"""
    
    def __init__(self):
        self.name = "Insecure PRNG"
        # Seed with predictable time
        self.rng = random.Random(int(time.time()))
    
    def generate_bytes(self, num_bytes):
        """Generate bytes using Python's random module"""
        result = bytearray()
        for _ in range(num_bytes):
            result.append(self.rng.randint(0, 255))
        return bytes(result)