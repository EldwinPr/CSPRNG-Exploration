import os

class SystemCSPRNG:
    """Wrapper for the operating system's CSPRNG"""
    
    def __init__(self):
        self.name = "System CSPRNG"
    
    def generate_bytes(self, num_bytes):
        """Generate cryptographically secure random bytes"""
        return os.urandom(num_bytes)