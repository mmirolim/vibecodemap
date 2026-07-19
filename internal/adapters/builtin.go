package adapters

import (
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mmirolim/vibecodemap/internal/repository"
	"github.com/mmirolim/vibecodemap/internal/scoping"
)

// BuiltinRegistry returns conservative stack detectors. These detectors do not
// claim semantic analysis support: they identify the native adapter(s) a later
// analyze command should invoke for each repository scope.
func BuiltinRegistry() (*Registry, error) {
	return NewRegistry(
		stackDetector{
			descriptor: Descriptor{
				ID: "python-ast-v0", Version: "0.1", Languages: []string{"python"}, Stacks: []string{"python"},
				Capabilities: []Capability{Artifacts, Symbols, Imports, Calls, Effects, Complexity},
				Support:      Prototype, Summary: "Conservative Python AST prototype; orchestration is not wired yet.",
			},
			detect: detectPython,
		},
		stackDetector{
			descriptor: Descriptor{
				ID: "go-packages-v0", Version: "0.1", Languages: []string{"go"}, Stacks: []string{"go-module"},
				Capabilities: []Capability{Artifacts, Symbols, Imports, Calls, Types, Effects, Complexity, Coverage, Tests, Entrypoints},
				Support:      DetectionOnly, Summary: "Go module detector; semantic analysis should use go/packages and go/types.",
			},
			detect: detectGo,
		},
		stackDetector{
			descriptor: Descriptor{
				ID: "typescript-project-v0", Version: "0.1", Languages: []string{"typescript", "javascript"}, Stacks: []string{"typescript"},
				Capabilities: []Capability{Artifacts, Symbols, Imports, Calls, Types, Effects, Complexity, Coverage, Tests, Entrypoints},
				Support:      DetectionOnly, Summary: "TypeScript project detector; semantic analysis is not implemented.",
			},
			detect: detectTypeScript,
		},
		stackDetector{
			descriptor: Descriptor{
				ID: "javascript-package-v0", Version: "0.1", Languages: []string{"javascript"}, Stacks: []string{"javascript"},
				Capabilities: []Capability{Artifacts, Symbols, Imports, Calls, Effects, Complexity, Coverage, Tests, Entrypoints},
				Support:      DetectionOnly, Summary: "JavaScript package detector; semantic analysis is not implemented.",
			},
			detect: detectJavaScript,
		},
		stackDetector{
			descriptor: Descriptor{
				ID: "flutter-dart-v0", Version: "0.1", Languages: []string{"dart"}, Stacks: []string{"flutter"},
				Capabilities: []Capability{Artifacts, Symbols, Imports, Calls, Types, Effects, Complexity, Coverage, Tests, Entrypoints, UIComposition, Navigation, Lifecycle, Permissions, PlatformBoundaries},
				Support:      DetectionOnly, Summary: "Flutter project detector; Dart analyzer integration is not implemented.",
			},
			detect: detectFlutter,
		},
		stackDetector{
			descriptor: Descriptor{
				ID: "dart-package-v0", Version: "0.1", Languages: []string{"dart"}, Stacks: []string{"dart-package"},
				Capabilities: []Capability{Artifacts, Symbols, Imports, Calls, Types, Effects, Complexity, Coverage, Tests, Entrypoints},
				Support:      DetectionOnly, Summary: "Dart package detector; analyzer integration is not implemented.",
			},
			detect: detectDart,
		},
		stackDetector{
			descriptor: Descriptor{
				ID: "android-kotlin-v0", Version: "0.1", Languages: []string{"kotlin", "java"}, Stacks: []string{"android"},
				Capabilities: []Capability{Artifacts, Symbols, Imports, Calls, Types, Effects, Complexity, Coverage, Tests, Entrypoints, UIComposition, Navigation, Lifecycle, Permissions, PlatformBoundaries},
				Support:      DetectionOnly, Summary: "Android module detector; Gradle/Kotlin semantic analysis is not implemented.",
			},
			detect: detectAndroid,
		},
		stackDetector{
			descriptor: Descriptor{
				ID: "kotlin-gradle-v0", Version: "0.1", Languages: []string{"kotlin", "java"}, Stacks: []string{"kotlin-gradle"},
				Capabilities: []Capability{Artifacts, Symbols, Imports, Calls, Types, Effects, Complexity, Coverage, Tests, Entrypoints},
				Support:      DetectionOnly, Summary: "Non-Android Kotlin/Gradle detector; semantic analysis is not implemented.",
			},
			detect: detectKotlin,
		},
		stackDetector{
			descriptor: Descriptor{
				ID: "apple-swift-v0", Version: "0.1", Languages: []string{"swift", "objective-c"}, Stacks: []string{"apple", "swift-package"},
				Capabilities: []Capability{Artifacts, Symbols, Imports, Calls, Types, Effects, Complexity, Coverage, Tests, Entrypoints, UIComposition, Navigation, Lifecycle, Permissions, PlatformBoundaries},
				Support:      DetectionOnly, Summary: "SwiftPM/Xcode detector; SourceKit-LSP integration is not implemented.",
			},
			detect: detectApple,
		},
	)
}

type stackDetector struct {
	descriptor Descriptor
	detect     func(repository.Report) ([]stackMatch, error)
}

type stackMatch struct {
	stack      string
	scope      string
	confidence float64
	evidence   []string
}

func (detector stackDetector) Descriptor() Descriptor { return detector.descriptor }

func (detector stackDetector) Detect(_ context.Context, report repository.Report) ([]Detection, error) {
	matches, err := detector.detect(report)
	if err != nil {
		return nil, err
	}
	result := make([]Detection, 0, len(matches))
	for _, match := range matches {
		result = append(result, Detection{
			AdapterID: detector.descriptor.ID,
			Stack:     match.stack, Scope: cleanScope(match.scope), Confidence: match.confidence,
			Evidence: uniqueSorted(match.evidence), Support: detector.descriptor.Support,
		})
	}
	return result, nil
}

type inventoryIndex struct {
	root  string
	files []string
	set   map[string]repository.Entry
}

func indexReport(report repository.Report) inventoryIndex {
	index := inventoryIndex{root: report.Root, set: make(map[string]repository.Entry)}
	for _, entry := range report.Entries {
		if entry.Kind != repository.File || (entry.Action != scoping.Analyze && entry.Action != scoping.Summarize) {
			continue
		}
		index.files = append(index.files, entry.Path)
		index.set[entry.Path] = entry
	}
	sort.Strings(index.files)
	return index
}

func (index inventoryIndex) matchingBase(base string) []string {
	var result []string
	for _, path := range index.files {
		if filepath.Base(path) == base {
			result = append(result, path)
		}
	}
	return result
}

func (index inventoryIndex) matchingSuffix(suffixes ...string) []string {
	var result []string
	for _, path := range index.files {
		lower := strings.ToLower(path)
		for _, suffix := range suffixes {
			if strings.HasSuffix(lower, strings.ToLower(suffix)) {
				result = append(result, path)
				break
			}
		}
	}
	return result
}

func (index inventoryIndex) contains(path string, needle string) bool {
	if _, exists := index.set[path]; !exists {
		return false
	}
	file, err := os.Open(filepath.Join(index.root, filepath.FromSlash(path)))
	if err != nil {
		return false
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 2*1024*1024))
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(data)), strings.ToLower(needle))
}

func detectGo(report repository.Report) ([]stackMatch, error) {
	index := indexReport(report)
	modules := index.matchingBase("go.mod")
	if len(modules) == 0 {
		if files := index.matchingSuffix(".go"); len(files) > 0 {
			return []stackMatch{{stack: "go-module", scope: ".", confidence: 0.55, evidence: sample(files, 3)}}, nil
		}
		return nil, nil
	}
	result := make([]stackMatch, 0, len(modules))
	for _, module := range modules {
		result = append(result, stackMatch{stack: "go-module", scope: filepath.ToSlash(filepath.Dir(module)), confidence: 1, evidence: []string{module}})
	}
	return result, nil
}

func detectPython(report repository.Report) ([]stackMatch, error) {
	index := indexReport(report)
	var markers []string
	for _, base := range []string{"pyproject.toml", "setup.py", "setup.cfg", "Pipfile", "poetry.lock"} {
		markers = append(markers, index.matchingBase(base)...)
	}
	for _, path := range index.files {
		base := strings.ToLower(filepath.Base(path))
		if strings.HasPrefix(base, "requirements") && strings.HasSuffix(base, ".txt") {
			markers = append(markers, path)
		}
	}
	python := index.matchingSuffix(".py")
	if len(markers) == 0 && len(python) == 0 {
		return nil, nil
	}
	confidence := 0.65
	evidence := sample(python, 3)
	if len(markers) > 0 {
		confidence = 0.95
		evidence = append(sample(uniqueSorted(markers), 3), evidence...)
	}
	return []stackMatch{{stack: "python", scope: ".", confidence: confidence, evidence: sample(uniqueSorted(evidence), 5)}}, nil
}

func detectTypeScript(report repository.Report) ([]stackMatch, error) {
	index := indexReport(report)
	ts := index.matchingSuffix(".ts", ".tsx", ".mts", ".cts")
	configs := index.matchingBase("tsconfig.json")
	if len(ts) == 0 && len(configs) == 0 {
		return nil, nil
	}
	confidence := 0.7
	if len(configs) > 0 {
		confidence = 1
	}
	evidence := append(sample(configs, 3), sample(ts, 3)...)
	return []stackMatch{{stack: "typescript", scope: ".", confidence: confidence, evidence: evidence}}, nil
}

func detectJavaScript(report repository.Report) ([]stackMatch, error) {
	index := indexReport(report)
	javascript := index.matchingSuffix(".js", ".jsx", ".mjs", ".cjs")
	packages := index.matchingBase("package.json")
	if len(javascript) == 0 {
		return nil, nil
	}
	confidence := 0.65
	if len(packages) > 0 {
		confidence = 0.9
	}
	evidence := append(sample(packages, 3), sample(javascript, 3)...)
	return []stackMatch{{stack: "javascript", scope: ".", confidence: confidence, evidence: evidence}}, nil
}

func detectFlutter(report repository.Report) ([]stackMatch, error) {
	index := indexReport(report)
	var result []stackMatch
	for _, manifest := range index.matchingBase("pubspec.yaml") {
		if !index.contains(manifest, "flutter:") && !index.contains(manifest, "sdk: flutter") {
			continue
		}
		scope := filepath.ToSlash(filepath.Dir(manifest))
		evidence := []string{manifest}
		for _, candidate := range []string{"lib/main.dart", "android/app/src/main/AndroidManifest.xml", "ios/Runner/Info.plist"} {
			path := joinScope(scope, candidate)
			if _, exists := index.set[path]; exists {
				evidence = append(evidence, path)
			}
		}
		result = append(result, stackMatch{stack: "flutter", scope: scope, confidence: 1, evidence: evidence})
	}
	return result, nil
}

func detectDart(report repository.Report) ([]stackMatch, error) {
	index := indexReport(report)
	var result []stackMatch
	manifests := index.matchingBase("pubspec.yaml")
	for _, manifest := range manifests {
		if index.contains(manifest, "flutter:") || index.contains(manifest, "sdk: flutter") {
			continue
		}
		result = append(result, stackMatch{stack: "dart-package", scope: filepath.ToSlash(filepath.Dir(manifest)), confidence: 0.95, evidence: []string{manifest}})
	}
	if len(result) == 0 && len(manifests) == 0 {
		if dart := index.matchingSuffix(".dart"); len(dart) > 0 {
			result = append(result, stackMatch{stack: "dart-package", scope: ".", confidence: 0.55, evidence: sample(dart, 3)})
		}
	}
	return result, nil
}

func detectAndroid(report repository.Report) ([]stackMatch, error) {
	index := indexReport(report)
	var result []stackMatch
	for _, manifest := range index.matchingBase("AndroidManifest.xml") {
		marker := "/src/main/AndroidManifest.xml"
		position := strings.LastIndex(manifest, marker)
		if position < 0 {
			continue
		}
		scope := manifest[:position]
		evidence := []string{manifest}
		for _, candidate := range []string{"build.gradle.kts", "build.gradle"} {
			path := joinScope(scope, candidate)
			if _, exists := index.set[path]; exists {
				evidence = append(evidence, path)
			}
		}
		result = append(result, stackMatch{stack: "android", scope: scope, confidence: 1, evidence: evidence})
	}
	return result, nil
}

func detectKotlin(report repository.Report) ([]stackMatch, error) {
	index := indexReport(report)
	android, _ := detectAndroid(report)
	androidScopes := make([]string, 0, len(android))
	for _, match := range android {
		androidScopes = append(androidScopes, cleanScope(match.scope))
	}
	var kotlin []string
	for _, path := range index.matchingSuffix(".kt") {
		insideAndroid := false
		for _, scope := range androidScopes {
			if scope == "." || path == scope || strings.HasPrefix(path, strings.TrimSuffix(scope, "/")+"/") {
				insideAndroid = true
				break
			}
		}
		if !insideAndroid {
			kotlin = append(kotlin, path)
		}
	}
	if len(kotlin) == 0 {
		return nil, nil
	}
	gradle := append(index.matchingBase("build.gradle.kts"), index.matchingBase("settings.gradle.kts")...)
	confidence := 0.6
	if len(gradle) > 0 {
		confidence = 0.95
	}
	return []stackMatch{{stack: "kotlin-gradle", scope: ".", confidence: confidence, evidence: append(sample(gradle, 3), sample(kotlin, 3)...)}}, nil
}

func detectApple(report repository.Report) ([]stackMatch, error) {
	index := indexReport(report)
	type scopedEvidence struct {
		stack    string
		evidence []string
	}
	scopes := make(map[string]*scopedEvidence)
	for _, manifest := range index.matchingBase("Package.swift") {
		scope := cleanScope(filepath.ToSlash(filepath.Dir(manifest)))
		scopes[scope] = &scopedEvidence{stack: "swift-package", evidence: []string{manifest}}
	}
	for _, project := range index.matchingBase("project.pbxproj") {
		projectBundle := filepath.ToSlash(filepath.Dir(project))
		scope := cleanScope(filepath.ToSlash(filepath.Dir(projectBundle)))
		item := scopes[scope]
		if item == nil {
			item = &scopedEvidence{stack: "apple"}
			scopes[scope] = item
		}
		item.stack = "apple"
		item.evidence = append(item.evidence, project)
	}
	for _, plist := range index.matchingBase("Info.plist") {
		for scope, item := range scopes {
			if scope == "." || strings.HasPrefix(plist, strings.TrimSuffix(scope, "/")+"/") {
				item.evidence = append(item.evidence, plist)
			}
		}
	}
	sources := index.matchingSuffix(".swift", ".m", ".mm")
	if len(scopes) == 0 && len(sources) == 0 {
		return nil, nil
	}
	if len(scopes) == 0 {
		return []stackMatch{{stack: "apple", scope: ".", confidence: 0.6, evidence: sample(sources, 3)}}, nil
	}
	orderedScopes := make([]string, 0, len(scopes))
	for scope := range scopes {
		orderedScopes = append(orderedScopes, scope)
	}
	sort.Strings(orderedScopes)
	result := make([]stackMatch, 0, len(scopes))
	for _, scope := range orderedScopes {
		item := scopes[scope]
		for _, source := range sources {
			if scope == "." || strings.HasPrefix(source, strings.TrimSuffix(scope, "/")+"/") {
				item.evidence = append(item.evidence, source)
			}
		}
		result = append(result, stackMatch{stack: item.stack, scope: scope, confidence: 0.95, evidence: sample(uniqueSorted(item.evidence), 6)})
	}
	return result, nil
}

func cleanScope(scope string) string {
	scope = filepath.ToSlash(filepath.Clean(scope))
	if scope == "" || scope == "/" {
		return "."
	}
	return scope
}

func joinScope(scope, path string) string {
	if scope == "" || scope == "." {
		return path
	}
	return strings.TrimSuffix(scope, "/") + "/" + path
}

func sample(values []string, maximum int) []string {
	if len(values) <= maximum {
		return append([]string(nil), values...)
	}
	return append([]string(nil), values[:maximum]...)
}

func uniqueSorted(values []string) []string {
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
	return result
}

func validateDetection(detection Detection, descriptor Descriptor) error {
	if detection.AdapterID != descriptor.ID {
		return fmt.Errorf("adapter id %q does not match descriptor %q", detection.AdapterID, descriptor.ID)
	}
	if detection.Stack == "" || detection.Scope == "" {
		return fmt.Errorf("adapter %q returned a detection without stack or scope", descriptor.ID)
	}
	if math.IsNaN(detection.Confidence) || math.IsInf(detection.Confidence, 0) || detection.Confidence < 0 || detection.Confidence > 1 {
		return fmt.Errorf("adapter %s returned confidence outside [0,1]", descriptor.ID)
	}
	if detection.Support != descriptor.Support {
		return fmt.Errorf("adapter %q returned support %q, want %q", descriptor.ID, detection.Support, descriptor.Support)
	}
	stackDeclared := false
	for _, stack := range descriptor.Stacks {
		if detection.Stack == stack {
			stackDeclared = true
			break
		}
	}
	if !stackDeclared {
		return fmt.Errorf("adapter %q returned undeclared stack %q", descriptor.ID, detection.Stack)
	}
	return nil
}
