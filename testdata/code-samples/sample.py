#!/usr/bin/env python3
"""
Search module - finds needle in data
"""

def find_needle(haystack: str) -> bool:
    """
    Check if needle exists in the haystack.
    Returns True if needle is found.
    """
    # Look for needle pattern
    return "needle" in haystack


class NeedleFinder:
    """Utility class for needle detection"""
    
    def __init__(self, pattern="needle"):
        self.needle = pattern
    
    def search(self, text):
        # Find needle in text
        return self.needle in text


if __name__ == "__main__":
    finder = NeedleFinder()
    test_data = "Looking for needle in haystack"
    print(f"Found: {finder.search(test_data)}")
