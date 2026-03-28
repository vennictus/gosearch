// Package ignore handles .gitignore and .gosearchignore parsing and matching.
package ignore

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Rule represents a single ignore rule from .gitignore or .gosearchignore.
type Rule struct {
	BaseDir string
	Pattern string
	Negate  bool
	DirOnly bool
	HasPath bool
}

// LoadRules loads ignore rules from the current directory, merging with inherited rules.
func LoadRules(currentDir string, inherited []Rule) ([]Rule, error) {
	rules := make([]Rule, 0, len(inherited)+8)
	rules = append(rules, inherited...)

	for _, fileName := range []string{".gitignore", ".gosearchignore"} {
		pathToIgnore := filepath.Join(currentDir, fileName)
		file, err := os.Open(pathToIgnore)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return rules, fmt.Errorf("%s: %w", pathToIgnore, err)
		}

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}

			negate := strings.HasPrefix(line, "!")
			if negate {
				line = strings.TrimSpace(strings.TrimPrefix(line, "!"))
			}
			if line == "" {
				continue
			}

			dirOnly := strings.HasSuffix(line, "/")
			line = strings.TrimSuffix(line, "/")
			if line == "" {
				continue
			}

			rules = append(rules, Rule{
				BaseDir: currentDir,
				Pattern: line,
				Negate:  negate,
				DirOnly: dirOnly,
				HasPath: strings.Contains(line, "/"),
			})
		}
		if err := scanner.Err(); err != nil {
			_ = file.Close()
			return rules, fmt.Errorf("%s: %w", pathToIgnore, err)
		}
		_ = file.Close()
	}
	return rules, nil
}

// ShouldIgnore checks if a path should be ignored based on the rules and default ignore dirs.
func ShouldIgnore(defaultIgnoreDirs map[string]struct{}, rules []Rule, fullPath string, isDir bool) bool {
	name := strings.ToLower(filepath.Base(fullPath))
	if isDir {
		if _, blocked := defaultIgnoreDirs[name]; blocked {
			return true
		}
	}

	ignored := false
	for _, rule := range rules {
		if rule.DirOnly && !isDir {
			continue
		}

		rel, err := filepath.Rel(rule.BaseDir, fullPath)
		if err != nil {
			continue
		}
		relSlash := filepath.ToSlash(rel)
		if relSlash == "." || strings.HasPrefix(relSlash, "../") {
			continue
		}

		if ruleMatch(rule, relSlash) {
			ignored = !rule.Negate
		}
	}
	return ignored
}

func ruleMatch(rule Rule, relSlash string) bool {
	patternText := strings.ReplaceAll(rule.Pattern, "**", "*")
	if rule.HasPath {
		if globMatch(patternText, relSlash) {
			return true
		}
		prefix := strings.TrimSuffix(patternText, "/") + "/"
		return strings.HasPrefix(relSlash, prefix)
	}

	for _, segment := range strings.Split(relSlash, "/") {
		if globMatch(patternText, segment) {
			return true
		}
	}
	return false
}

func globMatch(patternText string, value string) bool {
	matched, err := path.Match(patternText, value)
	if err != nil {
		return false
	}
	return matched
}
