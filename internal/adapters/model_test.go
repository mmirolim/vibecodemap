package adapters

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mmirolim/vibecodemap/internal/repository"
	"github.com/mmirolim/vibecodemap/internal/scoping"
)

func TestBuiltinRegistryDetectsLayeredGoAndFlutterRepository(t *testing.T) {
	root := t.TempDir()
	writeAdapterFixture(t, root, "go.mod", "module example.test/tool\n")
	writeAdapterFixture(t, root, "cmd/tool/main.go", "package main\n")
	writeAdapterFixture(t, root, "mobile/pubspec.yaml", "dependencies:\n  flutter:\n    sdk: flutter\n")
	writeAdapterFixture(t, root, "mobile/lib/main.dart", "void main() {}\n")
	writeAdapterFixture(t, root, "mobile/android/app/src/main/AndroidManifest.xml", "<manifest/>\n")
	writeAdapterFixture(t, root, "mobile/android/app/build.gradle.kts", "plugins {}\n")

	report, err := repository.Scan(context.Background(), root, repository.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	registry, err := BuiltinRegistry()
	if err != nil {
		t.Fatal(err)
	}
	detections, err := registry.Detect(context.Background(), report)
	if err != nil {
		t.Fatal(err)
	}

	assertDetection(t, detections, "go-packages-v0", "go-module", ".")
	assertDetection(t, detections, "flutter-dart-v0", "flutter", "mobile")
	assertDetection(t, detections, "android-kotlin-v0", "android", "mobile/android/app")
	for _, detection := range detections {
		if detection.AdapterID == "dart-package-v0" || detection.AdapterID == "kotlin-gradle-v0" {
			t.Fatalf("framework detector should supersede generic detection: %+v", detection)
		}
	}
}

func TestAnalyzeRequestUsesOnlyCentrallyApprovedFiles(t *testing.T) {
	report := repository.Report{Root: "/repo", Entries: []repository.Entry{
		{Path: "z.go", Kind: repository.File, Action: scoping.Analyze},
		{Path: "generated.pb.go", Kind: repository.File, Action: scoping.Summarize, Classification: "generated"},
		{Path: "vendor/x.go", Kind: repository.File, Action: scoping.Ignore},
		{Path: "dependency", Kind: repository.Directory, Action: scoping.Externalize},
	}}
	descriptor := Descriptor{ID: "test", Version: "1", Languages: []string{"go"}, Stacks: []string{"go"}, Capabilities: []Capability{Artifacts, Types}, Support: Available, Summary: "test"}
	request, err := NewAnalyzeRequest(report, descriptor, []Capability{Types, Artifacts})
	if err != nil {
		t.Fatal(err)
	}
	if request.Schema != RequestSchema || len(request.Files) != 2 {
		t.Fatalf("unexpected request: %+v", request)
	}
	if request.Files[0].Path != "generated.pb.go" || request.Files[1].Path != "z.go" {
		t.Fatalf("files are not deterministic or correctly scoped: %+v", request.Files)
	}
	if request.Capabilities[0] != Artifacts || request.Capabilities[1] != Types {
		t.Fatalf("capabilities are not deterministic: %+v", request.Capabilities)
	}
}

func TestKotlinDetectorKeepsNonAndroidMonorepoSources(t *testing.T) {
	root := t.TempDir()
	writeAdapterFixture(t, root, "app/src/main/AndroidManifest.xml", "<manifest/>\n")
	writeAdapterFixture(t, root, "app/src/main/kotlin/App.kt", "class App\n")
	writeAdapterFixture(t, root, "server/src/main/kotlin/Server.kt", "class Server\n")
	writeAdapterFixture(t, root, "settings.gradle.kts", "rootProject.name = \"mixed\"\n")
	report, err := repository.Scan(context.Background(), root, repository.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	registry, err := BuiltinRegistry()
	if err != nil {
		t.Fatal(err)
	}
	detections, err := registry.Detect(context.Background(), report)
	if err != nil {
		t.Fatal(err)
	}
	assertDetection(t, detections, "android-kotlin-v0", "android", "app")
	assertDetection(t, detections, "kotlin-gradle-v0", "kotlin-gradle", ".")
}

func TestAnalyzeRequestRejectsUndeclaredCapability(t *testing.T) {
	descriptor := Descriptor{ID: "test", Version: "1", Languages: []string{"go"}, Stacks: []string{"go"}, Capabilities: []Capability{Artifacts}, Support: Available, Summary: "test"}
	if _, err := NewAnalyzeRequest(repository.Report{Root: "/repo"}, descriptor, []Capability{Calls}); err == nil {
		t.Fatal("expected undeclared capability error")
	}
}

func TestEvidenceEventValidation(t *testing.T) {
	event := EvidenceEvent{
		Schema: EventSchema, ID: "call.1", Kind: "relation", Subject: "function.main",
		Producer: "test-adapter", Confidence: 0.8, Payload: json.RawMessage(`{"family":"call"}`),
	}
	if err := event.Validate(); err != nil {
		t.Fatal(err)
	}
	event.Confidence = 2
	if err := event.Validate(); err == nil {
		t.Fatal("expected invalid confidence error")
	}
}

func assertDetection(t *testing.T, detections []Detection, adapter, stack, scope string) {
	t.Helper()
	for _, detection := range detections {
		if detection.AdapterID == adapter && detection.Stack == stack && detection.Scope == scope {
			return
		}
	}
	t.Fatalf("missing %s/%s/%s in %+v", adapter, stack, scope, detections)
}

func writeAdapterFixture(t *testing.T, root, relative, contents string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}
