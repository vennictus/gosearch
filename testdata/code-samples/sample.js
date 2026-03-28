// JavaScript needle search implementation

const NEEDLE_PATTERN = "needle";

/**
 * Search for needle in the given text
 * @param {string} text - The haystack to search
 * @returns {boolean} - True if needle found
 */
function findNeedle(text) {
  // Simple needle search
  return text.includes(NEEDLE_PATTERN);
}

class NeedleSearcher {
  constructor(needle = "needle") {
    this.needle = needle;
  }

  // Search method - finds needle
  search(haystack) {
    const result = haystack.indexOf(this.needle);
    return result !== -1;
  }
}

// Test with needle example
const searcher = new NeedleSearcher();
console.log(searcher.search("haystack with needle"));
