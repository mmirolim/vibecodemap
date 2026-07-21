package scoping

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Action controls how much work an analyzer performs for a matched path.
// Dependencies that are ignored here should still be represented from package
// manifests as external systems; ignoring means "do not parse installed source".
type Action string

const (
	Analyze     Action = "analyze"
	Summarize   Action = "summarize"
	Externalize Action = "externalize"
	Ignore      Action = "ignore"
)

func (a Action) valid() bool {
	switch a {
	case Analyze, Summarize, Externalize, Ignore:
		return true
	default:
		return false
	}
}

// Rule is a repository-owned path decision. Patterns use slash-separated glob
// syntax with ** for zero or more path segments. Higher priority wins; ties are
// resolved by later declaration so a project rule can override a default.
type Rule struct {
	ID             string
	Pattern        string
	Action         Action
	Classification string
	Reason         string
	Priority       int
}

// MarkerRule classifies a file from a bounded prefix of its contents. It is
// useful for generated-file headers that are more reliable than file names.
type MarkerRule struct {
	ID             string
	Pattern        string
	Action         Action
	Classification string
	Reason         string
	Priority       int
}

type compiledRule struct {
	Rule
	expression *regexp.Regexp
	order      int
}

type compiledMarker struct {
	MarkerRule
	expression *regexp.Regexp
	order      int
}

// Decision records both the action and the rule that caused it so exclusions
// remain inspectable rather than silently disappearing.
type Decision struct {
	Action         Action
	Classification string
	RuleID         string
	Reason         string
	MatchedBy      string
	Priority       int
}

type Policy struct {
	defaultAction Action
	rules         []compiledRule
	markers       []compiledMarker
}

func NewPolicy(defaultAction Action, rules []Rule, markers []MarkerRule) (*Policy, error) {
	if !defaultAction.valid() {
		return nil, fmt.Errorf("invalid default action %q", defaultAction)
	}

	policy := &Policy{defaultAction: defaultAction}
	seen := make(map[string]struct{}, len(rules)+len(markers))
	for index, rule := range rules {
		if rule.ID == "" {
			return nil, fmt.Errorf("path rule %d has no id", index)
		}
		if _, exists := seen[rule.ID]; exists {
			return nil, fmt.Errorf("duplicate scope rule id %q", rule.ID)
		}
		seen[rule.ID] = struct{}{}
		if !rule.Action.valid() {
			return nil, fmt.Errorf("path rule %q has invalid action %q", rule.ID, rule.Action)
		}
		expression, err := compileGlob(rule.Pattern)
		if err != nil {
			return nil, fmt.Errorf("path rule %q: %w", rule.ID, err)
		}
		policy.rules = append(policy.rules, compiledRule{Rule: rule, expression: expression, order: index})
	}

	markerOffset := len(rules)
	for index, marker := range markers {
		if marker.ID == "" {
			return nil, fmt.Errorf("marker rule %d has no id", index)
		}
		if _, exists := seen[marker.ID]; exists {
			return nil, fmt.Errorf("duplicate scope rule id %q", marker.ID)
		}
		seen[marker.ID] = struct{}{}
		if !marker.Action.valid() {
			return nil, fmt.Errorf("marker rule %q has invalid action %q", marker.ID, marker.Action)
		}
		expression, err := regexp.Compile(marker.Pattern)
		if err != nil {
			return nil, fmt.Errorf("marker rule %q: %w", marker.ID, err)
		}
		policy.markers = append(policy.markers, compiledMarker{
			MarkerRule: marker,
			expression: expression,
			order:      markerOffset + index,
		})
	}

	return policy, nil
}

// Evaluate applies both path and content-marker rules. prefix should be a
// bounded initial slice (for example 8 KiB), never an entire large generated
// source file merely for marker detection.
func (p *Policy) Evaluate(path string, prefix []byte) Decision {
	normalized := filepath.ToSlash(strings.TrimPrefix(path, "./"))
	best := Decision{Action: p.defaultAction, MatchedBy: "default", Priority: -1}
	bestOrder := -1

	consider := func(priority, order int, decision Decision) {
		if priority > best.Priority || (priority == best.Priority && order > bestOrder) {
			best = decision
			bestOrder = order
		}
	}

	for _, rule := range p.rules {
		if rule.expression.MatchString(normalized) {
			consider(rule.Priority, rule.order, Decision{
				Action:         rule.Action,
				Classification: rule.Classification,
				RuleID:         rule.ID,
				Reason:         rule.Reason,
				MatchedBy:      "path",
				Priority:       rule.Priority,
			})
		}
	}

	for _, marker := range p.markers {
		if marker.expression.Match(prefix) {
			consider(marker.Priority, marker.order, Decision{
				Action:         marker.Action,
				Classification: marker.Classification,
				RuleID:         marker.ID,
				Reason:         marker.Reason,
				MatchedBy:      "content_marker",
				Priority:       marker.Priority,
			})
		}
	}

	return best
}

// DefaultRuleSet covers expensive dependency trees, build/cache outputs, and
// common generated-code conventions. It intentionally includes mobile build
// systems even before their semantic analyzers exist: repository discovery
// must not parse DerivedData, Gradle caches, or Flutter tool output as source.
func DefaultRuleSet() ([]Rule, []MarkerRule) {
	rules := []Rule{
		{ID: "vcm.generated-output", Pattern: "**/.vibecodemap/{out,generated}/**", Action: Ignore, Classification: "generated_report", Priority: 90, Reason: "VibeCodeMap renderer output is derived from committed DSL."},
		{ID: "vcm.local-state", Pattern: "**/.vcm/{cache,runs,local}/**", Action: Ignore, Classification: "cache", Priority: 90, Reason: "VibeCodeMap local run state is derived and machine-specific."},
		{ID: "deps.node-modules", Pattern: "**/node_modules/**", Action: Ignore, Classification: "installed_dependency", Priority: 50, Reason: "Represent dependencies from manifests; do not parse installed package source."},
		{ID: "deps.python-site-packages", Pattern: "**/site-packages/**", Action: Ignore, Classification: "installed_dependency", Priority: 50, Reason: "Represent installed Python packages from manifests; do not parse installed source."},
		{ID: "deps.python-egg-info", Pattern: "**/*.egg-info/**", Action: Ignore, Classification: "installed_dependency", Priority: 50, Reason: "Python distribution metadata is derived from package configuration."},
		{ID: "deps.python-venv", Pattern: "**/{.venv,venv,env}/**", Action: Ignore, Classification: "installed_dependency", Priority: 50, Reason: "Represent dependencies from manifests; do not parse virtual-environment source."},
		{ID: "deps.go-vendor", Pattern: "**/vendor/**", Action: Ignore, Classification: "vendored_dependency", Priority: 50, Reason: "Represent vendored modules as external dependencies by default."},
		{ID: "deps.apple-pods", Pattern: "**/Pods/**", Action: Ignore, Classification: "installed_dependency", Priority: 50, Reason: "Represent CocoaPods dependencies from manifests; do not parse installed source."},
		{ID: "deps.apple-carthage", Pattern: "**/Carthage/Build/**", Action: Ignore, Classification: "installed_dependency", Priority: 50, Reason: "Carthage build products are installed dependency output."},
		{ID: "deps.dart-pub-cache", Pattern: "**/.pub-cache/**", Action: Ignore, Classification: "installed_dependency", Priority: 50, Reason: "Represent Dart packages from pubspec and lock metadata."},
		{ID: "vcs.metadata", Pattern: "**/{.git,.hg,.svn}/**", Action: Ignore, Classification: "vcs_metadata", Priority: 100, Reason: "Version-control internals are not application source."},
		{ID: "os.metadata", Pattern: "**/{.DS_Store,Thumbs.db}", Action: Ignore, Classification: "os_metadata", Priority: 100, Reason: "Operating-system folder metadata is not application source."},
		{ID: "ide.metadata", Pattern: "**/{.idea,.fleet}/**", Action: Ignore, Classification: "cache", Priority: 80, Reason: "IDE indexes and local workspace metadata are derived state."},
		{ID: "reports.vscode-counter", Pattern: "**/.VSCodeCounter/**", Action: Ignore, Classification: "generated_report", Priority: 80, Reason: "VSCode Counter reports are derived repository measurements."},
		{ID: "cache.python", Pattern: "**/__pycache__/**", Action: Ignore, Classification: "cache", Priority: 80, Reason: "Python bytecode cache."},
		{ID: "cache.tools", Pattern: "**/{.mypy_cache,.pytest_cache,.ruff_cache,.tox,.nox}/**", Action: Ignore, Classification: "cache", Priority: 80, Reason: "Tool cache output."},
		{ID: "cache.javascript", Pattern: "**/{.npm,.pnpm-store,.turbo,.yarn/cache}/**", Action: Ignore, Classification: "cache", Priority: 80, Reason: "JavaScript package-manager and build-tool cache output."},
		{ID: "cache.dart", Pattern: "**/.dart_tool/**", Action: Ignore, Classification: "cache", Priority: 80, Reason: "Dart and Flutter tool state is derived from package metadata."},
		{ID: "cache.gradle", Pattern: "**/{.gradle,.kotlin}/**", Action: Ignore, Classification: "cache", Priority: 80, Reason: "Gradle and Kotlin compiler caches are derived state."},
		{ID: "cache.xcode", Pattern: "**/{DerivedData,xcuserdata}/**", Action: Ignore, Classification: "cache", Priority: 80, Reason: "Xcode indexes and per-user workspace state are derived data."},
		{ID: "build.outputs", Pattern: "**/{dist,build,coverage,.coverage,out,target}/**", Action: Ignore, Classification: "build_output", Priority: 60, Reason: "Derived build or report output."},
		{ID: "build.javascript", Pattern: "**/{.next,.nuxt,.svelte-kit,.vite,.parcel-cache}/**", Action: Ignore, Classification: "build_output", Priority: 65, Reason: "Framework and bundler output is derived from source."},
		{ID: "build.android", Pattern: "**/{.cxx,.externalNativeBuild}/**", Action: Ignore, Classification: "build_output", Priority: 65, Reason: "Android native build intermediates are derived output."},
		{ID: "build.swift", Pattern: "**/.build/**", Action: Ignore, Classification: "build_output", Priority: 65, Reason: "SwiftPM build and index output is derived data."},
		{ID: "build.flutter-ephemeral", Pattern: "**/Flutter/ephemeral/**", Action: Ignore, Classification: "build_output", Priority: 65, Reason: "Flutter platform-runner intermediates are generated output."},
		{ID: "build.flutter-plugins", Pattern: "**/.plugin_symlinks/**", Action: Ignore, Classification: "build_output", Priority: 65, Reason: "Flutter plugin symlinks are generated package wiring."},
		{ID: "build.flutter-ios-symlinks", Pattern: "**/.symlinks/**", Action: Ignore, Classification: "build_output", Priority: 65, Reason: "Flutter/CocoaPods package symlinks are generated dependency wiring."},
		{ID: "generated.directory", Pattern: "**/{generated,gen}/**", Action: Summarize, Classification: "generated", Priority: 55, Reason: "Keep aggregate volume and boundaries without indexing every generated symbol."},
		{ID: "generated.go-protobuf", Pattern: "**/*.pb.go", Action: Summarize, Classification: "generated", Priority: 70, Reason: "Generated protobuf implementation; retain aggregate size and dependencies only."},
		{ID: "generated.python-protobuf", Pattern: "**/*_pb2.py", Action: Summarize, Classification: "generated", Priority: 70, Reason: "Generated protobuf implementation; retain aggregate size and dependencies only."},
		{ID: "generated.python-grpc", Pattern: "**/*_pb2_grpc.py", Action: Summarize, Classification: "generated", Priority: 72, Reason: "Generated gRPC implementation; retain aggregate size and dependencies only."},
		{ID: "generated.javascript", Pattern: "**/*.{generated,gen}.{js,jsx,ts,tsx}", Action: Summarize, Classification: "generated", Priority: 70, Reason: "Generated JavaScript or TypeScript; retain aggregate size and dependencies only."},
		{ID: "generated.javascript-minified", Pattern: "**/*.min.js", Action: Summarize, Classification: "generated", Priority: 68, Reason: "Minified JavaScript is not useful symbol-level review input."},
		{ID: "generated.dart", Pattern: "**/*.{g,freezed,pb}.dart", Action: Summarize, Classification: "generated", Priority: 70, Reason: "Generated Dart implementation; retain aggregate size and dependencies only."},
		{ID: "generated.swift", Pattern: "**/*.{generated,pb,grpc}.swift", Action: Summarize, Classification: "generated", Priority: 70, Reason: "Generated Swift implementation; retain aggregate size and dependencies only."},
		{ID: "generated.kotlin-protobuf", Pattern: "**/*.pb.kt", Action: Summarize, Classification: "generated", Priority: 70, Reason: "Generated Kotlin protobuf implementation; retain aggregate size and dependencies only."},
		{ID: "generated.java-protobuf", Pattern: "**/*Proto.java", Action: Summarize, Classification: "generated", Priority: 65, Reason: "Likely generated Java protobuf surface; project rules may opt source back in."},
		{ID: "metadata.flutter-plugins", Pattern: "**/{.flutter-plugins,.flutter-plugins-dependencies}", Action: Ignore, Classification: "build_output", Priority: 65, Reason: "Flutter regenerates plugin wiring metadata."},
		{ID: "metadata.xcode-user-state", Pattern: "**/*.xcuserstate", Action: Ignore, Classification: "cache", Priority: 80, Reason: "Xcode per-user UI state is not source."},
		{ID: "metadata.javascript-cache", Pattern: "**/.eslintcache", Action: Ignore, Classification: "cache", Priority: 80, Reason: "ESLint cache is derived state."},
	}
	markers := []MarkerRule{
		{ID: "generated.go-header", Pattern: `(?m)^// Code generated .* DO NOT EDIT\.$`, Action: Summarize, Classification: "generated", Priority: 90, Reason: "Go generated-code header."},
		{ID: "content.binary", Pattern: `\x00`, Action: Summarize, Classification: "binary_or_media", Priority: 85, Reason: "Binary/media content is retained as an aggregate asset, not parsed as source text."},
		{ID: "generated.generic-header", Pattern: `(?im)^(//|#|/\*|\*)\s*(code generated|auto-?generated|generated (by|file|code)|do not edit)\b`, Action: Summarize, Classification: "generated", Priority: 75, Reason: "Common generated-code header."},
	}
	return rules, markers

}

// NewDefaultPolicy appends repository-owned corrections after the built-ins.
// A higher priority wins and a later rule wins a tie, so explicit project rules
// can opt a false-positive path back into full analysis.
func NewDefaultPolicy(rules []Rule, markers []MarkerRule) (*Policy, error) {
	defaultRules, defaultMarkers := DefaultRuleSet()
	return NewPolicy(Analyze, append(defaultRules, rules...), append(defaultMarkers, markers...))
}

func DefaultPolicy() (*Policy, error) {
	return NewDefaultPolicy(nil, nil)
}

// MatchPath applies the repository glob grammar to one slash-separated path.
// Inventory rules and renderer district selectors use this same matcher.
func MatchPath(pattern, path string) (bool, error) {
	expression, err := compileGlob(pattern)
	if err != nil {
		return false, err
	}
	normalized := filepath.ToSlash(strings.TrimPrefix(path, "./"))
	return expression.MatchString(normalized), nil
}

// Rules returns a deterministic copy useful for diagnostics and DSL emission.
func (p *Policy) Rules() []Rule {
	result := make([]Rule, 0, len(p.rules))
	for _, rule := range p.rules {
		result = append(result, rule.Rule)
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Priority != result[j].Priority {
			return result[i].Priority > result[j].Priority
		}
		return result[i].ID < result[j].ID
	})
	return result
}

// Markers returns a deterministic copy useful for diagnostics and composition.
func (p *Policy) Markers() []MarkerRule {
	result := make([]MarkerRule, 0, len(p.markers))
	for _, marker := range p.markers {
		result = append(result, marker.MarkerRule)
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Priority != result[j].Priority {
			return result[i].Priority > result[j].Priority
		}
		return result[i].ID < result[j].ID
	})
	return result
}

func compileGlob(pattern string) (*regexp.Regexp, error) {
	pattern = filepath.ToSlash(strings.TrimSpace(pattern))
	pattern = strings.TrimPrefix(pattern, "./")
	if pattern == "" {
		return nil, fmt.Errorf("empty glob")
	}
	if strings.HasSuffix(pattern, "/") {
		pattern += "**"
	}

	var expression strings.Builder
	expression.WriteString("^")
	for index := 0; index < len(pattern); {
		character := pattern[index]
		switch character {
		case '*':
			if index+1 < len(pattern) && pattern[index+1] == '*' {
				index += 2
				if index < len(pattern) && pattern[index] == '/' {
					expression.WriteString("(?:.*/)?")
					index++
				} else {
					expression.WriteString(".*")
				}
			} else {
				expression.WriteString("[^/]*")
				index++
			}
		case '?':
			expression.WriteString("[^/]")
			index++
		case '{':
			closing := strings.IndexByte(pattern[index+1:], '}')
			if closing < 0 {
				return nil, fmt.Errorf("unclosed brace group in %q", pattern)
			}
			closing += index + 1
			parts := strings.Split(pattern[index+1:closing], ",")
			if len(parts) < 2 {
				return nil, fmt.Errorf("brace group must contain alternatives in %q", pattern)
			}
			expression.WriteString("(?:")
			for partIndex, part := range parts {
				if partIndex > 0 {
					expression.WriteString("|")
				}
				expression.WriteString(regexp.QuoteMeta(part))
			}
			expression.WriteString(")")
			index = closing + 1
		default:
			expression.WriteString(regexp.QuoteMeta(string(character)))
			index++
		}
	}
	expression.WriteString("$")
	return regexp.Compile(expression.String())
}
