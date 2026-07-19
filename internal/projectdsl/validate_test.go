package projectdsl

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmbeddedDocumentsMatchCanonicalFiles(t *testing.T) {
	t.Parallel()
	cases := []struct {
		path     string
		embedded []byte
	}{
		{"../../dsl/vibecodemap-project-0.1.schema.json", Schema()},
		{"../../dsl/VIBECODEMAP_PROJECT_DSL_0_1.md", Grammar()},
	}
	for _, testCase := range cases {
		canonical, err := os.ReadFile(testCase.path)
		if err != nil {
			t.Fatalf("read %s: %v", testCase.path, err)
		}
		if !bytes.Equal(canonical, testCase.embedded) {
			t.Fatalf("embedded document is stale: %s", testCase.path)
		}
	}
}

func TestValidateReportsYAMLSyntaxLine(t *testing.T) {
	t.Parallel()
	diagnostics := Validate([]byte("vcm_project: \"0.1\"\nproject: [\n"), "broken.yaml")
	if len(diagnostics) != 1 || diagnostics[0].Code != "yaml.syntax" {
		t.Fatalf("unexpected diagnostics: %+v", diagnostics)
	}
	if diagnostics[0].Line == 0 {
		t.Fatalf("expected a source line: %+v", diagnostics[0])
	}
}

func TestValidateReportsSchemaLocation(t *testing.T) {
	t.Parallel()
	diagnostics := Validate([]byte("vcm_project: 9\nproject: {}\nanalysis: {}\ndecompositions: []\nrender_profiles: []\n"), "invalid.yaml")
	if len(diagnostics) == 0 {
		t.Fatal("expected schema diagnostics")
	}
	foundLocation := false
	for _, diagnostic := range diagnostics {
		if diagnostic.Line > 0 && diagnostic.Path != "" {
			foundLocation = true
		}
	}
	if !foundLocation {
		t.Fatalf("expected path and line diagnostics: %+v", diagnostics)
	}
}

func TestValidateUzumtoolsProject(t *testing.T) {
	t.Parallel()
	path := filepath.Clean("../../examples/uzumtools/uzumtools.project.vcm.yaml")
	for _, diagnostic := range ValidateFile(path) {
		if diagnostic.Severity == "error" {
			t.Fatalf("unexpected diagnostic: %s", diagnostic.Error())
		}
	}
}

func TestValidateRejectsTrailingYAMLDocument(t *testing.T) {
	data, err := os.ReadFile("../../examples/uzumtools/uzumtools.project.vcm.yaml")
	if err != nil {
		t.Fatal(err)
	}
	diagnostics := Validate(append(data, []byte("\n---\nextra: document\n")...), "multiple.yaml")
	if len(diagnostics) != 1 || diagnostics[0].Code != "yaml.multiple_documents" {
		t.Fatalf("unexpected diagnostics: %+v", diagnostics)
	}
}

func TestValidateRejectsDuplicateGlobalID(t *testing.T) {
	data, err := os.ReadFile("../../examples/uzumtools/uzumtools.project.vcm.yaml")
	if err != nil {
		t.Fatal(err)
	}
	duplicate := strings.Replace(string(data), "id: inferred.feature-affinity-sketch", "id: declared.runtime-boundaries", 1)
	diagnostics := Validate([]byte(duplicate), "duplicate.yaml")
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == "id.duplicate" && diagnostic.Line > 0 {
			return
		}
	}
	t.Fatalf("expected source-located duplicate diagnostic: %+v", diagnostics)
}

func TestValidateRejectsDuplicateLanguageAndBandMetric(t *testing.T) {
	data, err := os.ReadFile("../../examples/uzumtools/uzumtools.project.vcm.yaml")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name       string
		manifest   string
		diagnostic string
	}{
		{"language", strings.Replace(string(data), "- id: javascript", "- id: python", 1), "language.duplicate"},
		{"band metric", strings.Replace(string(data), "metric: quality.complexity.max_operation", "metric: quality.coverage.line_gap", 1), "band.metric_duplicate"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, diagnostic := range Validate([]byte(test.manifest), "duplicate.yaml") {
				if diagnostic.Code == test.diagnostic && diagnostic.Line > 0 {
					return
				}
			}
			t.Fatalf("expected %s", test.diagnostic)
		})
	}
}

func TestReadStructuralIDsRejectsDuplicateAcrossKinds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "structure.yaml")
	data := "artifacts:\n  - id: shared.id\nelements:\n  - id: shared.id\nrelations: []\nfindings: []\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	_, diagnostics := readStructuralIDs(path)
	if len(diagnostics) != 1 || diagnostics[0].Code != "structural.id_duplicate" || diagnostics[0].Line == 0 {
		t.Fatalf("unexpected diagnostics: %+v", diagnostics)
	}
}
