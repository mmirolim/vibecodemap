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

func TestBuiltinPythonAnalyzerUsesCentrallyScopedFiles(t *testing.T) {
	t.Parallel()
	if _, err := pythonCommand(); err != nil {
		t.Skip(err)
	}
	root := t.TempDir()
	writeAdapterFixture(t, root, "app.py", "\"\"\"HTTP client module.\"\"\"\nimport requests\n\nasync def fetch():\n    return requests.get('https://example.test')\n")
	writeAdapterFixture(t, root, "node_modules/ignored.py", "def ignored(): pass\n")
	report, err := repository.Scan(context.Background(), root, repository.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	registry, err := BuiltinRegistry()
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := registry.Analyze(context.Background(), report)
	if err != nil {
		t.Fatal(err)
	}
	if len(bundle.Runs) != 1 || bundle.Runs[0].AdapterID != "python-ast-v0" || bundle.Runs[0].Status != "completed" {
		t.Fatalf("runs = %+v", bundle.Runs)
	}
	if len(bundle.Events) != 1 || bundle.Events[0].Subject != "app.py" {
		t.Fatalf("events = %+v", bundle.Events)
	}
	var payload struct {
		ModuleDoc string   `json:"module_doc"`
		Imports   []string `json:"imports"`
		Symbols   []struct {
			Execution string         `json:"execution"`
			Effects   map[string]int `json:"effects"`
		} `json:"symbols"`
	}
	if err := json.Unmarshal(bundle.Events[0].Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.ModuleDoc != "HTTP client module." || len(payload.Imports) != 1 || payload.Imports[0] != "requests" || len(payload.Symbols) != 1 || payload.Symbols[0].Execution != "async" {
		t.Fatalf("payload = %+v", payload)
	}
	if payload.Symbols[0].Effects["network.call"] != 1 {
		t.Fatalf("effects = %+v", payload.Symbols[0].Effects)
	}
}

func TestBuiltinGoAnalyzerExtractsASTEvidence(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeAdapterFixture(t, root, "go.mod", "module example.test/app\n\ngo 1.24\n")
	writeAdapterFixture(t, root, "main.go", `package main
import (
    "net/http"
    "os"
    "sync"
)
type Server struct{}
func main() {
    http.HandleFunc("/health", health)
    var once sync.Once
    once.Do(func() {})
    _, _ = http.DefaultClient.Do(nil)
    _ = os.WriteFile("state.txt", nil, 0o600)
}
func health(http.ResponseWriter, *http.Request) {}
`)
	report, err := repository.Scan(context.Background(), root, repository.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	registry, err := BuiltinRegistry()
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := registry.Analyze(context.Background(), report)
	if err != nil {
		t.Fatal(err)
	}
	event := evidenceForSubject(t, bundle.Events, "main.go")
	if event.Producer != "go-ast-v0" || event.Kind != "go.file_analysis" {
		t.Fatalf("event = %+v", event)
	}
	var payload struct {
		Imports     []string         `json:"imports"`
		Symbols     []map[string]any `json:"symbols"`
		Routes      []map[string]any `json:"routes"`
		Entrypoints []map[string]any `json:"entrypoints"`
		Quality     struct {
			Effects map[string]int `json:"effects"`
		} `json:"quality"`
	}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Symbols) < 3 || len(payload.Routes) != 1 || payload.Routes[0]["path"] != "/health" || len(payload.Entrypoints) != 1 {
		t.Fatalf("payload = %+v", payload)
	}
	if payload.Quality.Effects["filesystem.write"] != 1 {
		t.Fatalf("effects = %+v", payload.Quality.Effects)
	}
	if payload.Quality.Effects["network.call"] != 1 {
		t.Fatalf("sync.Once.Do must not be classified as network I/O: %+v", payload.Quality.Effects)
	}
}

func TestBuiltinJavaScriptAndTypeScriptAnalyzersExtractEvidence(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeAdapterFixture(t, root, "package.json", `{"name":"web"}`)
	writeAdapterFixture(t, root, "tsconfig.json", `{}`)
	writeAdapterFixture(t, root, "src/server.ts", `import express from "express";
const app = express();
export async function load() { return fetch("/api/items"); }
app.get("/items", load);
`)
	writeAdapterFixture(t, root, "scripts/save.js", `const fs = require("fs");
function save() { fs.writeFile("state.txt", "ok", () => {}); }
`)
	report, err := repository.Scan(context.Background(), root, repository.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	registry, err := BuiltinRegistry()
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := registry.Analyze(context.Background(), report)
	if err != nil {
		t.Fatal(err)
	}
	typescript := evidenceForSubject(t, bundle.Events, "src/server.ts")
	javascript := evidenceForSubject(t, bundle.Events, "scripts/save.js")
	if typescript.Producer != "typescript-source-v0" || javascript.Producer != "javascript-source-v0" {
		t.Fatalf("unexpected producers: %s, %s", typescript.Producer, javascript.Producer)
	}
	var tsPayload struct {
		Imports []string         `json:"imports"`
		Routes  []map[string]any `json:"routes"`
		Quality struct {
			Effects map[string]int `json:"effects"`
		} `json:"quality"`
	}
	if err := json.Unmarshal(typescript.Payload, &tsPayload); err != nil {
		t.Fatal(err)
	}
	if len(tsPayload.Imports) != 1 || tsPayload.Imports[0] != "express" || len(tsPayload.Routes) != 1 || tsPayload.Quality.Effects["network.call"] != 1 {
		t.Fatalf("TypeScript payload = %+v", tsPayload)
	}
	var jsPayload struct {
		Imports []string `json:"imports"`
		Quality struct {
			Effects map[string]int `json:"effects"`
		} `json:"quality"`
	}
	if err := json.Unmarshal(javascript.Payload, &jsPayload); err != nil {
		t.Fatal(err)
	}
	if len(jsPayload.Imports) != 1 || jsPayload.Imports[0] != "fs" || jsPayload.Quality.Effects["filesystem.write"] != 1 {
		t.Fatalf("JavaScript payload = %+v", jsPayload)
	}
}

func TestBuiltinStatusesSeparateDetectionFromAnalysis(t *testing.T) {
	t.Parallel()
	registry, err := BuiltinRegistry()
	if err != nil {
		t.Fatal(err)
	}
	semantic := make(map[string]bool)
	for _, status := range registry.Statuses() {
		semantic[status.Descriptor.ID] = status.SemanticAnalysis
	}
	for _, id := range []string{"python-ast-v0", "go-ast-v0", "typescript-source-v0", "javascript-source-v0"} {
		if !semantic[id] {
			t.Fatalf("%s analyzer was not advertised as implemented", id)
		}
	}
	if semantic["flutter-dart-v0"] || semantic["android-kotlin-v0"] || semantic["apple-swift-v0"] {
		t.Fatalf("detection-only stacks were advertised as analyzers: %+v", semantic)
	}
}

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

	assertDetection(t, detections, "go-ast-v0", "go-module", ".")
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

func TestAnalyzeRequestAppliesDetectedModuleScopes(t *testing.T) {
	report := repository.Report{Root: "/repo", Entries: []repository.Entry{
		{Path: "apps/api/go.mod", Kind: repository.File, Action: scoping.Analyze},
		{Path: "apps/api/main.go", Kind: repository.File, Action: scoping.Analyze},
		{Path: "apps/worker/main.go", Kind: repository.File, Action: scoping.Analyze},
		{Path: "README.md", Kind: repository.File, Action: scoping.Analyze},
	}}
	descriptor := Descriptor{ID: "test", Version: "1", Languages: []string{"go"}, Stacks: []string{"go"}, Capabilities: []Capability{Artifacts}, Support: Available, Summary: "test"}
	request, err := NewAnalyzeRequest(report, descriptor, []Capability{Artifacts}, "apps/api")
	if err != nil {
		t.Fatal(err)
	}
	if len(request.Files) != 2 || request.Files[0].Path != "apps/api/go.mod" || request.Files[1].Path != "apps/api/main.go" {
		t.Fatalf("detected scope was not applied: %+v", request.Files)
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

func evidenceForSubject(t *testing.T, events []EvidenceEvent, subject string) EvidenceEvent {
	t.Helper()
	for _, event := range events {
		if event.Subject == subject {
			return event
		}
	}
	t.Fatalf("missing evidence for %s in %+v", subject, events)
	return EvidenceEvent{}
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
