package main

import "testing"

func FuzzParseSize(f *testing.F) {
	seeds := []string{"", "1", "128KB", "2MB", "3GB", "-1", "abc", "10 B"}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		_, _ = parseSize(input)
	})
}

func FuzzMatcherFindRanges(f *testing.F) {
	f.Add("needle", "this has needle")
	f.Add("abc", "ABC abc")
	f.Add("x", "")

	f.Fuzz(func(t *testing.T, pattern string, line string) {
		if pattern == "" {
			pattern = "x"
		}
		matcher := newMatcher(pattern, true, false)
		ranges := matcher.FindRanges(line)
		for _, r := range ranges {
			if r.Start < 0 || r.End < r.Start || r.End > len(line) {
				t.Fatalf("invalid range %#v for line length %d", r, len(line))
			}
		}
	})
}

func FuzzRuleMatch(f *testing.F) {
	f.Add("*.txt", "a.txt")
	f.Add("vendor/*", "vendor/a.go")
	f.Add("needle", "some/needle/path")

	f.Fuzz(func(t *testing.T, pattern string, relPath string) {
		rule := ignoreRule{pattern: pattern, hasPath: true}
		_ = ruleMatch(rule, relPath)
	})
}
