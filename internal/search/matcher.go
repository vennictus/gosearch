// Package search provides the core search engine functionality.
package search

import (
	"regexp"
	"strings"
)

// Result represents a single search match.
type Result struct {
	Path   string
	Line   int
	Text   string
	Ranges []MatchRange
}

// MatchRange represents the start and end position of a match within a line.
type MatchRange struct {
	Start int
	End   int
}

// MatchStrategy defines the interface for pattern matching strategies.
type MatchStrategy interface {
	FindRanges(line string) []MatchRange
}

// Matcher implements substring matching.
type Matcher struct {
	pattern     string
	patternFold string
	ignoreCase  bool
	wholeWord   bool
}

// RegexStrategy implements regex-based matching.
type RegexStrategy struct {
	expression *regexp.Regexp
}

// NewMatcher creates a new substring matcher.
func NewMatcher(pattern string, ignoreCase bool, wholeWord bool) Matcher {
	matcher := Matcher{pattern: pattern, ignoreCase: ignoreCase, wholeWord: wholeWord}
	if ignoreCase {
		matcher.patternFold = strings.ToLower(pattern)
	}
	return matcher
}

// FindRanges finds all substring matches in a line.
func (matcher Matcher) FindRanges(line string) []MatchRange {
	needle := matcher.pattern
	haystack := line
	if matcher.ignoreCase {
		needle = matcher.patternFold
		haystack = strings.ToLower(line)
	}

	if needle == "" {
		return nil
	}

	ranges := make([]MatchRange, 0)
	searchFrom := 0
	for {
		index := strings.Index(haystack[searchFrom:], needle)
		if index < 0 {
			break
		}

		start := searchFrom + index
		end := start + len(needle)
		if !matcher.wholeWord || isWholeWordMatch(line, start, end) {
			ranges = append(ranges, MatchRange{Start: start, End: end})
			searchFrom = end
			continue
		}
		searchFrom = start + 1
	}

	return ranges
}

// NewRegexStrategy creates a new regex-based strategy.
func NewRegexStrategy(pattern string, ignoreCase bool, wholeWord bool) (RegexStrategy, error) {
	p := pattern
	if wholeWord {
		p = "\\b(?:" + p + ")\\b"
	}
	if ignoreCase {
		p = "(?i)" + p
	}

	re, err := regexp.Compile(p)
	if err != nil {
		return RegexStrategy{}, err
	}
	return RegexStrategy{expression: re}, nil
}

// FindRanges finds all regex matches in a line.
func (strategy RegexStrategy) FindRanges(line string) []MatchRange {
	indices := strategy.expression.FindAllStringIndex(line, -1)
	if len(indices) == 0 {
		return nil
	}
	ranges := make([]MatchRange, 0, len(indices))
	for _, match := range indices {
		ranges = append(ranges, MatchRange{Start: match[0], End: match[1]})
	}
	return ranges
}

func isWholeWordMatch(line string, start int, end int) bool {
	leftBoundary := start == 0 || !isWordByte(line[start-1])
	rightBoundary := end == len(line) || !isWordByte(line[end])
	return leftBoundary && rightBoundary
}

func isWordByte(value byte) bool {
	return (value >= 'a' && value <= 'z') ||
		(value >= 'A' && value <= 'Z') ||
		(value >= '0' && value <= '9') ||
		value == '_'
}

// BuildStrategy creates the appropriate match strategy based on config.
func BuildStrategy(pattern string, useRegex bool, ignoreCase bool, wholeWord bool) (MatchStrategy, error) {
	if !useRegex {
		return NewMatcher(pattern, ignoreCase, wholeWord), nil
	}
	return NewRegexStrategy(pattern, ignoreCase, wholeWord)
}
