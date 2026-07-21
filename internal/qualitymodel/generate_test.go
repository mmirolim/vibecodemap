package qualitymodel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mmirolim/vibecodemap/internal/adapters"
	"github.com/mmirolim/vibecodemap/internal/viewer"
	"gopkg.in/yaml.v3"
)

func TestGenerateMapsFileAndSymbolEvidenceToQualitySubjects(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	source := "package main\n\nfunc main() {\n\tif true {}\n}\n"
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	structural := `vcm: "0.1"
model:
  id: model.test
  name: Test
  repository: {root: ., revision: abc123, dirty: false}
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
	structuralPath := filepath.Join(root, "model.vcm.yaml")
	if err := os.WriteFile(structuralPath, []byte(structural), 0o600); err != nil {
		t.Fatal(err)
	}
	payload := json.RawMessage(`{
  "path":"main.go","language":"go","lines":5,
  "quality":{"complexity_max":7,"complexity_total":7,"nesting_max":2,"max_function_lines":3,"effects":{"filesystem.write":1}},
  "symbols":[{"name":"main","line":3,"end_line":5,"complexity":3,"max_nesting":1,"effects":{}}],
  "limitations":["Go parser/AST only"]
}`)
	bundle := adapters.EvidenceBundle{
		Schema: adapters.EvidenceBundleSchema, Root: root,
		Events: []adapters.EvidenceEvent{{
			Schema: adapters.EventSchema, ID: "go.file.test", Kind: "go.file_analysis", Subject: "main.go",
			Producer: "go-ast-v0", Confidence: 1, Source: &adapters.SourceLocation{Path: "main.go", Line: 1, EndLine: 5}, Payload: payload,
		}},
	}
	evidenceData, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	evidencePath := filepath.Join(root, "evidence.json")
	if err := os.WriteFile(evidencePath, evidenceData, 0o600); err != nil {
		t.Fatal(err)
	}

	generated, summary, err := Generate(structuralPath, evidencePath, Options{Now: time.Date(2026, 7, 21, 20, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	if summary.Artifacts != 1 || summary.Elements != 1 || summary.Unmapped != 0 {
		t.Fatalf("summary = %+v", summary)
	}
	qualityPath := filepath.Join(root, "quality.vcm.yaml")
	if err := os.WriteFile(qualityPath, generated, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, diagnostic := range viewer.ValidateDocument(qualityPath, viewer.ValidationOptions{Kind: "quality", Core: structuralPath}) {
		if diagnostic.Severity == "error" {
			t.Fatalf("generated quality validation failed: %s", diagnostic.Error())
		}
	}

	var decoded document
	if err := yaml.Unmarshal(generated, &decoded); err != nil {
		t.Fatal(err)
	}
	artifact := findMeasurement(t, decoded.Measurements, "measurement.file.main.complexity.cyclomatic.max")
	if artifact.Value == nil || *artifact.Value != 7 {
		t.Fatalf("artifact complexity = %+v", artifact)
	}
	element := findMeasurement(t, decoded.Measurements, "measurement.component.cli.complexity.cyclomatic.max")
	if element.Value == nil || *element.Value != 3 {
		t.Fatalf("element complexity = %+v", element)
	}
	coverage := findMeasurement(t, decoded.Measurements, "measurement.file.main.test.line_coverage")
	if coverage.Status != "unknown" || coverage.Value != nil {
		t.Fatalf("coverage must remain explicit unknown: %+v", coverage)
	}
}

func TestGenerateRejectsEvidenceFromAnotherRepository(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	structural := `vcm: "0.1"
model:
  id: model.test
  name: Test
  repository: {root: ., revision: abc123, dirty: false}
  generated: {at: now, by: test}
scope: {include: [main.go]}
artifacts: []
elements: []
`
	structuralPath := filepath.Join(root, "model.vcm.yaml")
	if err := os.WriteFile(structuralPath, []byte(structural), 0o600); err != nil {
		t.Fatal(err)
	}
	bundle := adapters.EvidenceBundle{Schema: adapters.EvidenceBundleSchema, Root: filepath.Join(root, "another-repository")}
	evidenceData, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	evidencePath := filepath.Join(root, "evidence.json")
	if err := os.WriteFile(evidencePath, evidenceData, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Generate(structuralPath, evidencePath, Options{}); err == nil {
		t.Fatal("expected repository-root mismatch")
	}
}

func TestGenerateRejectsInvalidEvidenceEvents(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	structuralPath := writeQualityStructuralFixture(t, root, "main.go", "go", "")
	bundle := adapters.EvidenceBundle{
		Schema: adapters.EvidenceBundleSchema, Root: root,
		Events: []adapters.EvidenceEvent{{
			Schema: adapters.EventSchema, ID: "go.file.invalid", Kind: "go.file_analysis", Subject: "main.go",
			Producer: "go-ast-v0", Confidence: 2, Payload: json.RawMessage(`{}`),
		}},
	}
	evidencePath := writeEvidenceFixture(t, root, bundle)
	if _, _, err := Generate(structuralPath, evidencePath, Options{}); err == nil {
		t.Fatal("expected invalid evidence event to be rejected")
	}
}

func TestGenerateKeepsSummaryOnlyMeasurementsUnknown(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	structuralPath := writeQualityStructuralFixture(t, root, "generated.pb.go", "go", "")
	bundle := adapters.EvidenceBundle{
		Schema: adapters.EvidenceBundleSchema, Root: root,
		Events: []adapters.EvidenceEvent{{
			Schema: adapters.EventSchema, ID: "go.file.summary", Kind: "go.file_summary", Subject: "generated.pb.go",
			Producer: "go-ast-v0", Confidence: 1, Source: &adapters.SourceLocation{Path: "generated.pb.go"},
			Payload: json.RawMessage(`{"path":"generated.pb.go","language":"go","size_bytes":42,"summary_only":true}`),
		}},
	}
	evidencePath := writeEvidenceFixture(t, root, bundle)
	generated, _, err := Generate(structuralPath, evidencePath, Options{Now: time.Date(2026, 7, 21, 20, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	var decoded document
	if err := yaml.Unmarshal(generated, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, metric := range []string{"complexity.cyclomatic.max", "effects.mutating_sites"} {
		measurement := findMeasurement(t, decoded.Measurements, "measurement.file.main."+metric)
		if measurement.Status != "unknown" || measurement.Value != nil {
			t.Fatalf("%s must remain unknown for summary-only evidence: %+v", metric, measurement)
		}
	}
}

func TestGenerateDoesNotInventLexicalSymbolEffectsOrInvalidRanges(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	structuralPath := writeQualityStructuralFixture(t, root, "src/app.ts", "typescript", "load")
	bundle := adapters.EvidenceBundle{
		Schema: adapters.EvidenceBundleSchema, Root: root,
		Events: []adapters.EvidenceEvent{{
			Schema: adapters.EventSchema, ID: "ts.file.app", Kind: "typescript.file_analysis", Subject: "src/app.ts",
			Producer: "typescript-source-v0", Confidence: 0.85,
			Source:  &adapters.SourceLocation{Path: "src/app.ts", Line: 1, EndLine: 1},
			Payload: json.RawMessage(`{"path":"src/app.ts","language":"typescript","lines":1,"symbols":[{"name":"load","line":1}],"quality":{"decision_tokens":0,"brace_nesting_max":0,"effects":{}}}`),
		}},
	}
	evidencePath := writeEvidenceFixture(t, root, bundle)
	generated, _, err := Generate(structuralPath, evidencePath, Options{Now: time.Date(2026, 7, 21, 20, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	qualityPath := filepath.Join(root, "quality.vcm.yaml")
	if err := os.WriteFile(qualityPath, generated, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, diagnostic := range viewer.ValidateDocument(qualityPath, viewer.ValidationOptions{Kind: "quality", Core: structuralPath}) {
		if diagnostic.Severity == "error" {
			t.Fatalf("lexical symbol generated invalid quality DSL: %s", diagnostic.Error())
		}
	}
	var decoded document
	if err := yaml.Unmarshal(generated, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, item := range decoded.Measurements {
		if item.Subject == "component.main" && item.Metric == "effects.mutating_sites" {
			t.Fatalf("lexical symbol without effect facts received an observed zero: %+v", item)
		}
	}
}

func TestMutatingEffectSitesIncludesGenericMutationCandidates(t *testing.T) {
	t.Parallel()
	if got := mutatingEffectSites(map[string]int{"filesystem.mutate": 2, "filesystem.read": 3}); got != 2 {
		t.Fatalf("mutatingEffectSites() = %d, want 2", got)
	}
}

func TestSourceMatchesSymbolDoesNotFallbackToRangeForWrongNamedSymbol(t *testing.T) {
	t.Parallel()
	ref := sourceRef{Symbol: "wanted", Lines: []int{10, 20}}
	if sourceMatchesSymbol(ref, symbolFacts{Name: "other", Line: 12, EndLine: 14}) {
		t.Fatal("named source reference matched a different symbol by line range")
	}
	if !sourceMatchesSymbol(ref, symbolFacts{Name: "Receiver.wanted", Line: 30, EndLine: 32}) {
		t.Fatal("qualified matching symbol was not recognized")
	}
}

func writeQualityStructuralFixture(t *testing.T, root, path, language, symbol string) string {
	t.Helper()
	absolute := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(absolute, []byte("placeholder\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	symbolField := ""
	if symbol != "" {
		symbolField = ", symbol: " + symbol
	}
	structural := `vcm: "0.1"
model:
  id: model.test
  name: Test
  repository: {root: ., revision: abc123, dirty: false}
  generated: {at: now, by: test}
scope: {include: ["` + path + `"]}
artifacts:
  - id: file.main
    path: ` + path + `
    kind: source
    language: ` + language + `
    summary: Test source.
    metrics: {lines: 1}
    generated: {method: deterministic, confidence: high, rationale: Test fixture.}
elements:
  - id: component.main
    kind: component
    name: Component
    summary: Test component.
    reality: {intent: required, implementation: present, runtime: unknown}
    source_refs: [{artifact: file.main` + symbolField + `, lines: [1, 1], role: definition}]
    generated: {method: deterministic, confidence: high, rationale: Test fixture.}
`
	structuralPath := filepath.Join(root, "model.vcm.yaml")
	if err := os.WriteFile(structuralPath, []byte(structural), 0o600); err != nil {
		t.Fatal(err)
	}
	return structuralPath
}

func writeEvidenceFixture(t *testing.T, root string, bundle adapters.EvidenceBundle) string {
	t.Helper()
	data, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "evidence.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func findMeasurement(t *testing.T, items []measurement, id string) measurement {
	t.Helper()
	for _, item := range items {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("missing measurement %s in %+v", id, items)
	return measurement{}
}
