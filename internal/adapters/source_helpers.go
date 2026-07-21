package adapters

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func readScopedSource(root, relative string) ([]byte, error) {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	candidate := filepath.Clean(filepath.Join(absoluteRoot, filepath.FromSlash(relative)))
	within, err := filepath.Rel(absoluteRoot, candidate)
	if err != nil || within == ".." || strings.HasPrefix(within, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("scoped path escapes repository root: %q", relative)
	}
	return os.ReadFile(candidate)
}

func evidenceID(prefix, path string) string {
	digest := sha256.Sum256([]byte(path))
	return prefix + ".file." + hex.EncodeToString(digest[:8])
}

func marshalPayload(value any) (json.RawMessage, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

func uniqueSortedLimited(values []string, maximum int) ([]string, bool) {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value != "" {
			seen[value] = struct{}{}
		}
	}
	result := make([]string, 0, len(seen))
	for value := range seen {
		result = append(result, value)
	}
	sort.Strings(result)
	if maximum > 0 && len(result) > maximum {
		return result[:maximum], true
	}
	return result, false
}

func sourceLineCount(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	count := 1
	for _, item := range data {
		if item == '\n' {
			count++
		}
	}
	if data[len(data)-1] == '\n' {
		count--
	}
	return count
}
