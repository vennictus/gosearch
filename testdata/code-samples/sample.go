package main

import (
	"fmt"
	"strings"
)

// SearchForNeedle looks for the needle pattern
func SearchForNeedle(haystack string) bool {
	// Simple needle detection
	return strings.Contains(haystack, "needle")
}

func main() {
	data := "haystack with needle inside"
	if SearchForNeedle(data) {
		fmt.Println("Found the needle!")
	}
}
