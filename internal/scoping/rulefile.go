package scoping

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
)

// ParseRuleFile reads a .vcmignore file. A line is either:
//
//	PATTERN              shorthand for "ignore PATTERN"
//	!PATTERN             opt a path back into analysis
//	analyze PATTERN
//	summarize PATTERN
//	externalize PATTERN
//	ignore PATTERN
//
// Patterns use the same repository-relative glob grammar as project manifests.
// All file rules have project priority and later lines win ties.
func ParseRuleFile(path string) ([]Rule, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var rules []Rule
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		action := Ignore
		pattern := line
		if strings.HasPrefix(line, "!") {
			action = Analyze
			pattern = strings.TrimSpace(strings.TrimPrefix(line, "!"))
		} else if fields := strings.Fields(line); len(fields) > 1 {
			candidate := Action(fields[0])
			if candidate.valid() {
				action = candidate
				pattern = strings.TrimSpace(strings.TrimPrefix(line, fields[0]))
			}
		}
		pattern = strings.TrimPrefix(pattern, "/")
		if pattern == "" {
			return nil, fmt.Errorf("%s:%d: scope rule has no pattern", path, lineNumber)
		}
		if _, err := compileGlob(pattern); err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, lineNumber, err)
		}
		rules = append(rules, Rule{
			ID:             fmt.Sprintf("vcmignore.%d", lineNumber),
			Pattern:        pattern,
			Action:         action,
			Classification: "custom",
			Reason:         fmt.Sprintf("Declared in %s at line %d.", path, lineNumber),
			Priority:       1000,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return rules, nil
}

// ParseRuleFileIfExists treats a missing repository-owned rule file as an
// empty policy, while preserving all other read and syntax errors.
func ParseRuleFileIfExists(path string) ([]Rule, error) {
	rules, err := ParseRuleFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return rules, err
}
