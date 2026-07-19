// Package repository creates an inspectable, policy-scoped repository
// inventory before any language adapter performs expensive semantic analysis.
package repository

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mmirolim/vibecodemap/internal/scoping"
)

const Schema = "vibecodemap.inventory/0.1"

type EntryKind string

const (
	File      EntryKind = "file"
	Directory EntryKind = "directory"
	Symlink   EntryKind = "symlink"
)

type Entry struct {
	Path           string         `json:"path"`
	Kind           EntryKind      `json:"kind"`
	Size           int64          `json:"size_bytes,omitempty"`
	Action         scoping.Action `json:"action"`
	Classification string         `json:"classification,omitempty"`
	RuleID         string         `json:"rule_id,omitempty"`
	Reason         string         `json:"reason,omitempty"`
	MatchedBy      string         `json:"matched_by"`
	Priority       int            `json:"priority"`
	Pruned         bool           `json:"pruned,omitempty"`
}

type Totals struct {
	Entries           int   `json:"entries"`
	Files             int   `json:"files"`
	Directories       int   `json:"directories"`
	Symlinks          int   `json:"symlinks"`
	PrunedDirectories int   `json:"pruned_directories"`
	Bytes             int64 `json:"bytes"`
}

type Report struct {
	Schema           string                    `json:"schema"`
	Root             string                    `json:"root"`
	RuleFile         string                    `json:"rule_file,omitempty"`
	GitignoreApplied bool                      `json:"gitignore_applied"`
	Entries          []Entry                   `json:"entries"`
	Totals           map[scoping.Action]Totals `json:"totals"`
	Warnings         []string                  `json:"warnings,omitempty"`
}

type Options struct {
	// RuleFile defaults to ROOT/.vcmignore. Set it to "-" to disable the
	// repository-owned rule file.
	RuleFile       string
	ReadGitignore  bool
	MaxHeaderBytes int64
	MaxFileBytes   int64
	Rules          []scoping.Rule
	Markers        []scoping.MarkerRule
}

func DefaultOptions() Options {
	return Options{
		ReadGitignore:  true,
		MaxHeaderBytes: 8192,
		MaxFileBytes:   10 * 1024 * 1024,
	}
}

// Scan walks root without following symlinks, applies path rules before
// reading file contents, asks Git to apply exact gitignore semantics in one
// batch, and only then reads bounded prefixes for generated-code markers.
func Scan(ctx context.Context, root string, options Options) (Report, error) {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return Report{}, fmt.Errorf("resolve repository root: %w", err)
	}
	info, err := os.Stat(absoluteRoot)
	if err != nil {
		return Report{}, fmt.Errorf("stat repository root: %w", err)
	}
	if !info.IsDir() {
		return Report{}, fmt.Errorf("repository root is not a directory: %s", absoluteRoot)
	}
	if options.MaxHeaderBytes <= 0 {
		options.MaxHeaderBytes = 8192
	}
	if options.MaxFileBytes <= 0 {
		options.MaxFileBytes = 10 * 1024 * 1024
	}

	ruleFile := options.RuleFile
	if ruleFile == "" {
		ruleFile = filepath.Join(absoluteRoot, ".vcmignore")
	} else if ruleFile != "-" && !filepath.IsAbs(ruleFile) {
		ruleFile = filepath.Join(absoluteRoot, ruleFile)
	}
	var fileRules []scoping.Rule
	if ruleFile != "-" {
		fileRules, err = scoping.ParseRuleFileIfExists(ruleFile)
		if err != nil {
			return Report{}, err
		}
	}
	policy, err := scoping.NewDefaultPolicy(append(options.Rules, fileRules...), options.Markers)
	if err != nil {
		return Report{}, fmt.Errorf("compile scope policy: %w", err)
	}

	gitIgnoredRoots := map[string]struct{}{}
	gitignoreAvailable := false
	var gitignoreWarning string
	if options.ReadGitignore {
		gitIgnoredRoots, gitignoreAvailable, gitignoreWarning = gitStatusIgnored(ctx, absoluteRoot)
	}

	report := Report{
		Schema:           Schema,
		Root:             absoluteRoot,
		Totals:           make(map[scoping.Action]Totals),
		Entries:          make([]Entry, 0),
		GitignoreApplied: gitignoreAvailable,
	}
	if gitignoreWarning != "" {
		report.Warnings = append(report.Warnings, gitignoreWarning)
	}
	if len(fileRules) > 0 {
		report.RuleFile = ruleFile
	}

	err = filepath.WalkDir(absoluteRoot, func(path string, item fs.DirEntry, walkErr error) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if walkErr != nil {
			report.Warnings = append(report.Warnings, fmt.Sprintf("walk %s: %v", path, walkErr))
			if item != nil && item.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if path == absoluteRoot {
			return nil
		}
		relative, err := filepath.Rel(absoluteRoot, path)
		if err != nil {
			return fmt.Errorf("make path relative: %w", err)
		}
		relative = filepath.ToSlash(relative)

		if item.IsDir() {
			decision := policy.Evaluate(relative+"/", nil)
			if _, ignored := gitIgnoredRoots[relative+"/"]; ignored && decision.Priority < 900 {
				decision = gitignoreDecision()
			}
			if decision.Action == scoping.Ignore || decision.Action == scoping.Externalize {
				entry := entryFromDecision(relative, Directory, 0, decision)
				entry.Pruned = true
				report.Entries = append(report.Entries, entry)
				return filepath.SkipDir
			}
			return nil
		}

		kind := File
		if item.Type()&os.ModeSymlink != 0 {
			kind = Symlink
		}
		itemInfo, err := item.Info()
		if err != nil {
			report.Warnings = append(report.Warnings, fmt.Sprintf("stat %s: %v", relative, err))
			return nil
		}
		decision := policy.Evaluate(relative, nil)
		if _, ignored := gitIgnoredRoots[relative]; ignored && decision.Priority < 900 {
			decision = gitignoreDecision()
		}
		if kind == Symlink && decision.Action == scoping.Analyze {
			decision = scoping.Decision{
				Action:         scoping.Summarize,
				Classification: "symlink",
				RuleID:         "filesystem.symlink",
				Reason:         "Symlink targets are not followed during repository inventory.",
				MatchedBy:      "filesystem",
				Priority:       850,
			}
		}
		if kind == File && itemInfo.Size() > options.MaxFileBytes && decision.Priority < 850 {
			decision = scoping.Decision{
				Action:         scoping.Summarize,
				Classification: "large_file",
				RuleID:         "budget.max-file-bytes",
				Reason:         fmt.Sprintf("File exceeds the %d-byte detailed-analysis budget.", options.MaxFileBytes),
				MatchedBy:      "size_budget",
				Priority:       850,
			}
		}
		report.Entries = append(report.Entries, entryFromDecision(relative, kind, itemInfo.Size(), decision))
		return nil
	})
	if err != nil {
		return Report{}, fmt.Errorf("walk repository: %w", err)
	}

	if options.ReadGitignore && gitignoreAvailable {
		ignored, applied, warning := gitIgnored(ctx, absoluteRoot, report.Entries)
		report.GitignoreApplied = report.GitignoreApplied && applied
		if warning != "" {
			report.Warnings = append(report.Warnings, warning)
		}
		for index := range report.Entries {
			entry := &report.Entries[index]
			if _, exists := ignored[entry.Path]; !exists || entry.Priority >= 900 {
				continue
			}
			decision := gitignoreDecision()
			entry.Action = decision.Action
			entry.Classification = decision.Classification
			entry.RuleID = decision.RuleID
			entry.Reason = decision.Reason
			entry.MatchedBy = decision.MatchedBy
			entry.Priority = decision.Priority
		}
	}

	for index := range report.Entries {
		entry := &report.Entries[index]
		if entry.Kind != File || entry.Action == scoping.Ignore || entry.Action == scoping.Externalize {
			continue
		}
		prefix, err := readPrefix(filepath.Join(absoluteRoot, filepath.FromSlash(entry.Path)), options.MaxHeaderBytes)
		if err != nil {
			report.Warnings = append(report.Warnings, fmt.Sprintf("read prefix %s: %v", entry.Path, err))
			continue
		}
		decision := policy.Evaluate(entry.Path, prefix)
		if decision.Priority >= entry.Priority {
			*entry = entryFromDecision(entry.Path, entry.Kind, entry.Size, decision)
		}
	}

	sort.Slice(report.Entries, func(i, j int) bool { return report.Entries[i].Path < report.Entries[j].Path })
	sort.Strings(report.Warnings)
	for _, entry := range report.Entries {
		total := report.Totals[entry.Action]
		total.Entries++
		switch entry.Kind {
		case File:
			total.Files++
			total.Bytes += entry.Size
		case Directory:
			total.Directories++
			if entry.Pruned {
				total.PrunedDirectories++
			}
		case Symlink:
			total.Symlinks++
		}
		report.Totals[entry.Action] = total
	}
	return report, nil
}

func gitignoreDecision() scoping.Decision {
	return scoping.Decision{
		Action:         scoping.Ignore,
		Classification: "git_ignored",
		RuleID:         "gitignore",
		Reason:         "Ignored by Git's repository rules.",
		MatchedBy:      "gitignore",
		Priority:       900,
	}
}

func entryFromDecision(path string, kind EntryKind, size int64, decision scoping.Decision) Entry {
	return Entry{
		Path:           path,
		Kind:           kind,
		Size:           size,
		Action:         decision.Action,
		Classification: decision.Classification,
		RuleID:         decision.RuleID,
		Reason:         decision.Reason,
		MatchedBy:      decision.MatchedBy,
		Priority:       decision.Priority,
	}
}

func readPrefix(path string, maximum int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(io.LimitReader(file, maximum))
}

func gitIgnored(ctx context.Context, root string, entries []Entry) (map[string]struct{}, bool, string) {
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.Kind != Directory {
			paths = append(paths, entry.Path)
		}
	}
	if len(paths) == 0 {
		return map[string]struct{}{}, true, ""
	}
	input := append([]byte(strings.Join(paths, "\x00")), 0)
	command := exec.CommandContext(ctx, "git", "-C", root, "check-ignore", "-z", "--stdin")
	command.Stdin = bytes.NewReader(input)
	var output bytes.Buffer
	var standardError bytes.Buffer
	command.Stdout = &output
	command.Stderr = &standardError
	err := command.Run()
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) && exitError.ExitCode() == 1 {
			return map[string]struct{}{}, true, ""
		}
		if errors.Is(err, exec.ErrNotFound) {
			return nil, false, "Git is unavailable; .gitignore rules were not applied."
		}
		message := strings.TrimSpace(standardError.String())
		if message == "" {
			message = err.Error()
		}
		return nil, false, fmt.Sprintf("could not apply .gitignore rules: %s", message)
	}
	ignored := make(map[string]struct{})
	for _, raw := range bytes.Split(output.Bytes(), []byte{0}) {
		if len(raw) == 0 {
			continue
		}
		ignored[filepath.ToSlash(string(raw))] = struct{}{}
	}
	return ignored, true, ""
}

// gitStatusIgnored asks Git for collapsed ignored paths before walking. This
// allows large ignored directories to be pruned without reimplementing Git's
// pattern semantics. gitIgnored still performs a batched exact check for the
// remaining files after the walk.
func gitStatusIgnored(ctx context.Context, root string) (map[string]struct{}, bool, string) {
	command := exec.CommandContext(ctx, "git", "-C", root, "status", "--ignored", "--porcelain=v1", "-z", "--untracked-files=normal")
	var output bytes.Buffer
	var standardError bytes.Buffer
	command.Stdout = &output
	command.Stderr = &standardError
	if err := command.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, false, "Git is unavailable; .gitignore rules were not applied."
		}
		message := strings.TrimSpace(standardError.String())
		if message == "" {
			message = err.Error()
		}
		return nil, false, fmt.Sprintf("could not discover .gitignore roots: %s", message)
	}
	ignored := make(map[string]struct{})
	for _, raw := range bytes.Split(output.Bytes(), []byte{0}) {
		if len(raw) < 4 || !bytes.HasPrefix(raw, []byte("!! ")) {
			continue
		}
		path := filepath.ToSlash(string(raw[3:]))
		ignored[path] = struct{}{}
	}
	return ignored, true, ""
}
