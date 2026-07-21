package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mmirolim/vibecodemap/internal/adapters"
	"github.com/mmirolim/vibecodemap/internal/viewer"
)

func TestRunAnalyzeWritesOrchestratedEvidence(t *testing.T) {
	t.Parallel()
	registry, err := adapters.BuiltinRegistry()
	if err != nil {
		t.Fatal(err)
	}
	for _, status := range registry.Statuses() {
		if status.Descriptor.ID == "python-ast-v0" && !status.RuntimeAvailable {
			t.Skip(status.RuntimeDetail)
		}
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "app.py"), []byte("def main():\n    return 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "evidence.json")
	if code := runAnalyze([]string{"-output", output, root}); code != 0 {
		t.Fatalf("runAnalyze() code = %d", code)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	var bundle adapters.EvidenceBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		t.Fatal(err)
	}
	if bundle.Schema != adapters.EvidenceBundleSchema || len(bundle.Events) != 1 || bundle.Events[0].Subject != "app.py" {
		t.Fatalf("bundle = %+v", bundle)
	}
}

func TestRunQualityBridgesAnalyzeEvidenceIntoValidatedDSL(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.test/app\n\ngo 1.24\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nfunc main() {\n\tif true {}\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	evidence := filepath.Join(root, ".vibecodemap", "generated", "evidence.json")
	if code := runAnalyze([]string{"-output", evidence, root}); code != 0 {
		t.Fatalf("runAnalyze() code = %d", code)
	}
	model := `vcm: "0.1"
model:
  id: model.test
  name: Test
  repository: {root: .., revision: working_tree, dirty: true}
  generated: {at: now, by: test}
scope: {include: [main.go]}
artifacts:
  - id: file.main
    path: main.go
    kind: source
    language: go
    summary: Test entrypoint.
    metrics: {lines: 5}
    generated: {method: deterministic, confidence: high, rationale: Test fixture.}
elements:
  - id: component.cli
    kind: component
    name: CLI
    summary: Test CLI.
    reality: {intent: required, implementation: present, runtime: unknown}
    source_refs: [{artifact: file.main, symbol: main, lines: [3, 5], role: definition}]
    generated: {method: deterministic, confidence: high, rationale: Test fixture.}
`
	modelPath := filepath.Join(root, ".vibecodemap", "model.vcm.yaml")
	if err := os.WriteFile(modelPath, []byte(model), 0o600); err != nil {
		t.Fatal(err)
	}
	qualityPath := filepath.Join(root, ".vibecodemap", "quality.vcm.yaml")
	if code := runQuality([]string{"-evidence", evidence, "-output", qualityPath, modelPath}); code != 0 {
		t.Fatalf("runQuality() code = %d", code)
	}
	for _, diagnostic := range viewer.ValidateDocument(qualityPath, viewer.ValidationOptions{Kind: "quality", Core: modelPath}) {
		if diagnostic.Severity == "error" {
			t.Fatalf("quality validation error: %s", diagnostic.Error())
		}
	}
}
