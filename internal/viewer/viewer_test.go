package viewer

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestComposeUzumtoolsFixture(t *testing.T) {
	t.Parallel()

	project := filepath.Join("..", "..", "examples", "uzumtools", "uzumtools.project.vcm.yaml")
	view, err := Compose(project, ComposeOptions{})
	if err != nil {
		t.Fatalf("Compose() error = %v", err)
	}

	if view.Schema != ViewSchema {
		t.Fatalf("schema = %q, want %q", view.Schema, ViewSchema)
	}
	if view.Stats.Systems != 1 || view.Stats.Districts != 6 || view.Stats.Nodes != 42 {
		t.Fatalf("unexpected mapped shape: %+v", view.Stats)
	}
	if view.Stats.Relations != 33 || view.Stats.Roads != 7 || view.Stats.Unrouted != 0 {
		t.Fatalf("unexpected relation shape: %+v", view.Stats)
	}
	if len(view.Cities) != 1 || view.Cities[0].Code != "C1" {
		t.Fatalf("cities = %+v", view.Cities)
	}

	for _, road := range view.Roads {
		if road.A.Code == "" || road.B.Code == "" ||
			(road.A.Code[0] >= '0' && road.A.Code[0] <= '9') ||
			(road.B.Code[0] >= '0' && road.B.Code[0] <= '9') {
			t.Fatalf("road labels must use district codes, got %q <-> %q", road.A.Code, road.B.Code)
		}
		if road.A.Opposite != road.B.Code || road.B.Opposite != road.A.Code {
			t.Fatalf("road endpoints disagree: %+v", road)
		}
	}

	for _, node := range view.Nodes {
		if len(node.Bands) != len(view.Profile.Bands) {
			t.Fatalf("node %q has %d bands, want %d", node.ID, len(node.Bands), len(view.Profile.Bands))
		}
		if node.Source != nil && !filepath.IsAbs(node.Source.Absolute) {
			t.Fatalf("node %q source target is not absolute: %q", node.ID, node.Source.Absolute)
		}
		if node.ID == "component.storage-adapters" && !node.DirectMutation {
			t.Fatal("human correction did not mark storage adapters as directly mutating")
		}
	}
}

func TestRenderWritesGeneratedViewAndHTML(t *testing.T) {
	t.Parallel()

	project := filepath.Join("..", "..", "examples", "uzumtools", "uzumtools.project.vcm.yaml")
	output := filepath.Join(t.TempDir(), "map.html")
	result, err := Render(project, RenderOptions{Output: output})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if result.HTMLPath != output {
		t.Fatalf("HTMLPath = %q, want %q", result.HTMLPath, output)
	}

	viewData, err := os.ReadFile(result.JSONPath)
	if err != nil {
		t.Fatalf("read generated JSON: %v", err)
	}
	var decoded ViewModel
	if err := json.Unmarshal(viewData, &decoded); err != nil {
		t.Fatalf("decode generated JSON: %v", err)
	}
	if decoded.Schema != ViewSchema || decoded.Stats.Nodes == 0 {
		t.Fatalf("generated JSON is not a populated view model: %+v", decoded.Stats)
	}

	htmlData, err := os.ReadFile(result.HTMLPath)
	if err != nil {
		t.Fatalf("read generated HTML: %v", err)
	}
	document := string(htmlData)
	for _, forbidden := range []string{"__VCM_TITLE__", "__VCM_DATA__"} {
		if strings.Contains(document, forbidden) {
			t.Fatalf("generated HTML retained placeholder %q", forbidden)
		}
	}
	for _, required := range []string{decoded.Project.Name, ViewSchema, "window.__VCM_READY__", "<canvas id=\"scene\""} {
		if !strings.Contains(document, required) {
			t.Fatalf("generated HTML is missing %q", required)
		}
	}
}

func TestRenderRejectsSharedOutputPath(t *testing.T) {
	t.Parallel()

	project := filepath.Join("..", "..", "examples", "uzumtools", "uzumtools.project.vcm.yaml")
	output := filepath.Join(t.TempDir(), "map.html")
	_, err := Render(project, RenderOptions{Output: output, JSONOutput: output})
	if err == nil || !strings.Contains(err.Error(), "must use different paths") {
		t.Fatalf("Render() error = %v, want shared-output rejection", err)
	}
}

func TestSamePathHandlesCaseInsensitivePlatforms(t *testing.T) {
	t.Parallel()

	left := filepath.Join(t.TempDir(), "Map.HTML")
	right := filepath.Join(filepath.Dir(left), "map.html")
	got := samePath(left, right)
	if (runtime.GOOS == "darwin" || runtime.GOOS == "windows") && !got {
		t.Fatal("samePath did not conservatively detect a case-only collision")
	}
}

func TestInstantiateViewerDoesNotReplaceInsideValues(t *testing.T) {
	t.Parallel()

	title := "__VCM_DATA__ <map> __VCM_TITLE__"
	data := []byte(`{"name":"__VCM_TITLE__","marker":"__VCM_DATA__"}`)
	document, err := instantiateViewer(title, data)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(document, "<h1 id=\"title\">__VCM_DATA__ &lt;map&gt; __VCM_TITLE__</h1>") {
		t.Fatal("title placeholders were not treated as opaque title text")
	}
	if !strings.Contains(document, `<script id="vcm-data" type="application/json">`+string(data)+`</script>`) {
		t.Fatal("JSON placeholders were modified while instantiating the viewer")
	}
}

func TestViewerKeepsFilteredLabelsHiddenAndPortsDirectional(t *testing.T) {
	t.Parallel()
	for _, required := range []string{
		"label.enabled=cityOK&&roadOK&&labelMode!=='none'&&modeOK",
		"if(!label.enabled){label.element.style.display='none';continue;}",
		"addBoundaryPort(mesh,node,port.input,index)",
		"input → building",
		"building → output",
		"new THREE.Timer()",
		"timer.update(timestamp)",
		"THREE.PCFShadowMap",
		"fitDistance=Math.max",
	} {
		if !strings.Contains(viewerDocument, required) {
			t.Fatalf("embedded viewer is missing interaction contract %q", required)
		}
	}
	for _, deprecated := range []string{"new THREE.Clock()", "THREE.PCFSoftShadowMap"} {
		if strings.Contains(viewerDocument, deprecated) {
			t.Fatalf("embedded viewer still uses deprecated Three.js API %q", deprecated)
		}
	}
}

func TestComposeCreatesOneCityPerSystem(t *testing.T) {
	t.Parallel()

	sourceDir := filepath.Join("..", "..", "examples", "uzumtools")
	targetDir := t.TempDir()
	for _, name := range []string{"uzumtools.project.vcm.yaml", "uzumtools.quality.vcm.yaml"} {
		data, err := os.ReadFile(filepath.Join(sourceDir, name))
		if err != nil {
			t.Fatalf("read fixture %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(targetDir, name), data, 0o600); err != nil {
			t.Fatalf("copy fixture %s: %v", name, err)
		}
	}

	structuralData, err := os.ReadFile(filepath.Join(sourceDir, "uzumtools.vcm.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := yaml.Unmarshal(structuralData, &document); err != nil {
		t.Fatal(err)
	}
	elements, ok := document["elements"].([]any)
	if !ok {
		t.Fatalf("fixture elements has type %T", document["elements"])
	}
	var template map[string]any
	for _, raw := range elements {
		item := raw.(map[string]any)
		if item["id"] == "system.photochecker" {
			template = item
		}
		if item["id"] == "deployable.browser" {
			item["parent"] = "system-photochecker"
		}
	}
	if template == nil {
		t.Fatal("system fixture not found")
	}
	second := make(map[string]any, len(template))
	for key, value := range template {
		second[key] = value
	}
	// This deliberately has the same slug as system.photochecker. View IDs must
	// preserve semantic identity rather than collapsing slug collisions.
	second["id"] = "system-photochecker"
	second["name"] = "Browser product"
	document["elements"] = append(elements, second)
	structuralData, err = yaml.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "uzumtools.vcm.yaml"), structuralData, 0o600); err != nil {
		t.Fatal(err)
	}

	view, err := Compose(filepath.Join(targetDir, "uzumtools.project.vcm.yaml"), ComposeOptions{})
	if err != nil {
		t.Fatalf("Compose() error = %v", err)
	}
	if view.Stats.Systems != 2 || len(view.Cities) != 2 {
		t.Fatalf("systems = %d, cities = %+v", view.Stats.Systems, view.Cities)
	}
	districtCities := make(map[string]string, len(view.Districts))
	for _, district := range view.Districts {
		if previous, duplicate := districtCities[district.ID]; duplicate {
			t.Fatalf("district id %q is shared by cities %q and %q", district.ID, previous, district.CityID)
		}
		districtCities[district.ID] = district.CityID
	}
	for _, node := range view.Nodes {
		if node.ID == "deployable.browser" && node.CityID != "system-photochecker" {
			t.Fatalf("browser deployable assigned to %q", node.CityID)
		}
		if districtCities[node.DistrictID] != node.CityID {
			t.Fatalf("node %q city %q uses district %q from city %q", node.ID, node.CityID, node.DistrictID, districtCities[node.DistrictID])
		}
	}
	for _, district := range view.Districts {
		if !strings.HasPrefix(district.Code, "C") {
			t.Fatalf("multi-city district code = %q, want city-qualified code", district.Code)
		}
	}
}

func TestPercentileUsesMidrankForTies(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		values []float64
		value  float64
		want   float64
	}{
		{name: "all equal", values: []float64{4, 4, 4}, value: 4, want: 0.5},
		{name: "middle tie", values: []float64{1, 2, 2, 3}, value: 2, want: 0.5},
		{name: "below range", values: []float64{1, 2, 3}, value: 0, want: 0},
		{name: "above range", values: []float64{1, 2, 3}, value: 4, want: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := percentile(test.values, test.value); math.Abs(got-test.want) > 1e-9 {
				t.Fatalf("percentile(%v, %v) = %v, want %v", test.values, test.value, got, test.want)
			}
		})
	}
}

func TestThresholdRatioDoesNotDependOnManifestOrder(t *testing.T) {
	t.Parallel()

	if ordered, unordered := thresholdRatio([]float64{10, 20, 30}, 15), thresholdRatio([]float64{30, 10, 20}, 15); ordered != unordered {
		t.Fatalf("ordered threshold ratio %v differs from unordered ratio %v", ordered, unordered)
	}
}

func TestBandOrderUsesExplicitOrderField(t *testing.T) {
	t.Parallel()

	bands := orderedBands([]BandSpec{{ID: "third", Order: 3}, {ID: "first", Order: 1}, {ID: "second", Order: 2}})
	if got := []string{bands[0].ID, bands[1].ID, bands[2].ID}; !slices.Equal(got, []string{"first", "second", "third"}) {
		t.Fatalf("ordered band IDs = %v", got)
	}
}

func TestElementMetricOverridesInheritedArtifactMetric(t *testing.T) {
	t.Parallel()

	draft := nodeDraft{
		element:     element{ID: "component.api"},
		artifactIDs: []string{"file.api"},
	}
	indexed := indexMeasurements([]measurement{
		{ID: "element.coverage", Subject: "component.api", Metric: "test.line_coverage", Status: "observed", Value: 0.9},
		{ID: "file.coverage", Subject: "file.api", Metric: "test.line_coverage", Status: "observed", Value: 0.1},
	})
	metrics := collectMetrics(draft, indexed, map[string]metricDefinition{
		"test.line_coverage": {ID: "test.line_coverage", Unit: "ratio"},
	})
	if got := metrics["test.line_coverage"].Value; got != 0.9 {
		t.Fatalf("line coverage = %v, want explicit element value 0.9", got)
	}
}

func TestMutationCorrectionOverridesMeasuredAndRelatedEffects(t *testing.T) {
	t.Parallel()

	node := NodeView{
		ID:    "component.storage",
		Lines: 100,
		Measurements: map[string]Metric{
			"effects.mutating_sites":          {Value: 3, Status: "observed"},
			"effects.direct_mutation_density": {Value: 30, Status: "observed"},
		},
	}
	counts := map[string]int{node.ID: 2}
	if !nodeDirectMutation(node, counts, nil) {
		t.Fatal("observed mutation evidence did not ground the node")
	}
	if !nodeDirectMutation(NodeView{ID: node.ID, Measurements: map[string]Metric{
		"effects.direct_mutation_density": {Value: 1, Status: "observed"},
	}}, nil, nil) {
		t.Fatal("observed direct mutation density did not ground the node")
	}
	overrides := map[string]bool{node.ID: false}
	if nodeDirectMutation(node, counts, overrides) {
		t.Fatal("explicit false correction did not override mutation evidence")
	}
	value, status, exists := bandMetric(node, "effects.direct_mutation_density", nil, nil, counts, overrides)
	if !exists || value != 0 || status != "corrected" {
		t.Fatalf("corrected mutation band = (%v, %q, %t), want (0, corrected, true)", value, status, exists)
	}
	overrides[node.ID] = true
	value, status, exists = bandMetric(node, "effects.direct_mutation_density", nil, nil, counts, overrides)
	if !exists || value != 30 || status != "corrected" {
		t.Fatalf("confirmed mutation band = (%v, %q, %t), want (30, corrected, true)", value, status, exists)
	}
}

func TestStructuralReferenceValidationRejectsAmbiguousGraph(t *testing.T) {
	t.Parallel()

	var model structuralDocument
	model.Artifacts = []artifact{
		{ID: "shared.id", Path: `..\outside.go`},
		{ID: "file.copy", Path: "../outside.go"},
	}
	model.Elements = []element{
		{ID: "shared.id", Parent: "component.b"},
		{ID: "component.b", Parent: "shared.id"},
	}
	model.Relations = []relation{{ID: "relation.bad", From: "shared.id", To: "component.missing"}}
	model.Findings = []finding{{ID: "finding.bad", Subjects: []string{"component.missing"}}}

	err := validateStructuralReferences("model.yaml", model)
	if err == nil {
		t.Fatal("validateStructuralReferences() accepted an ambiguous graph")
	}
	for _, expected := range []string{
		`duplicate id "shared.id"`,
		`path escapes repository root`,
		`artifact path "../outside.go" is shared`,
		`element parent cycle`,
		`missing target "component.missing"`,
		`missing subject "component.missing"`,
	} {
		if !strings.Contains(err.Error(), expected) {
			t.Fatalf("validation error is missing %q:\n%s", expected, err)
		}
	}
}

func TestStructuralContractRejectsTrailingYAMLDocument(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(filepath.Join("..", "..", "examples", "uzumtools", "uzumtools.vcm.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	err = validateYAMLContract(append(data, []byte("\n---\nextra: document\n")...), "model.yaml", loadStructuralSchema)
	if err == nil {
		t.Fatal("structural contract accepted a trailing YAML document")
	}
}

func TestQualityReferenceValidationRejectsDuplicateIDs(t *testing.T) {
	t.Parallel()

	var structural structuralDocument
	structural.Model.ID = "model.test"
	quality := qualityDocument{
		MetricCatalog: []metricDefinition{{ID: "metric.test"}, {ID: "metric.test"}},
		Measurements: []measurement{
			{ID: "measurement.test", Subject: "model.test", Metric: "metric.test"},
			{ID: "measurement.test", Subject: "model.test", Metric: "metric.test"},
		},
	}
	err := validateQualityReferences("quality.yaml", structural, quality)
	if err == nil || !strings.Contains(err.Error(), `duplicate metric id "metric.test"`) ||
		!strings.Contains(err.Error(), `duplicate measurement id "measurement.test"`) {
		t.Fatalf("validateQualityReferences() error = %v", err)
	}
}

func TestEmbeddedSchemasMatchCanonicalContracts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path     string
		embedded []byte
	}{
		{filepath.Join("..", "..", "dsl", "vibecodemap-0.1.schema.json"), structuralSchemaDocument},
		{filepath.Join("..", "..", "dsl", "vibecodemap-quality-0.2.schema.json"), qualitySchemaDocument},
	}
	for _, test := range tests {
		canonical, err := os.ReadFile(test.path)
		if err != nil {
			t.Fatalf("read canonical schema %s: %v", test.path, err)
		}
		if !bytes.Equal(canonical, test.embedded) {
			t.Fatalf("embedded schema differs from %s", test.path)
		}
	}
}

func TestContractSchemaExposesStructuralAndQualityContracts(t *testing.T) {
	t.Parallel()
	for kind, marker := range map[string]string{
		"structural": `"vcm"`,
		"quality":    `"vcm_quality"`,
	} {
		document, err := ContractSchema(kind)
		if err != nil {
			t.Fatalf("ContractSchema(%q): %v", kind, err)
		}
		if !bytes.Contains(document, []byte(marker)) {
			t.Fatalf("ContractSchema(%q) is missing %s", kind, marker)
		}
	}
	if _, err := ContractSchema("project"); err == nil {
		t.Fatal("viewer contract schema should reject project; projectdsl owns it")
	}
}

func TestValidateDocumentAutoDetectsAllDocumentKinds(t *testing.T) {
	t.Parallel()
	base := filepath.Join("..", "..", "examples", "uzumtools")
	for _, name := range []string{"uzumtools.vcm.yaml", "uzumtools.quality.vcm.yaml", "uzumtools.project.vcm.yaml"} {
		diagnostics := ValidateDocument(filepath.Join(base, name), ValidationOptions{})
		for _, diagnostic := range diagnostics {
			if diagnostic.Severity == "error" {
				t.Fatalf("%s validation error: %s", name, diagnostic.Error())
			}
		}
	}
}

func TestValidateDocumentRejectsAmbiguousVersionKeys(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "ambiguous.vcm.yaml")
	if err := os.WriteFile(path, []byte("vcm: \"0.1\"\nvcm_quality: \"0.2-draft\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	diagnostics := ValidateDocument(path, ValidationOptions{})
	if len(diagnostics) != 1 || diagnostics[0].Code != "document.detect" || !strings.Contains(diagnostics[0].Message, "more than one") {
		t.Fatalf("ambiguous version keys were not rejected: %+v", diagnostics)
	}
}

func TestStructuralValidationChecksActualSourceRanges(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	source := filepath.Join(root, "app.py")
	if err := os.WriteFile(source, []byte("print('ok')\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	model := `vcm: "0.1"
model:
  id: model.test
  name: Test
  repository: {root: "` + root + `"}
  generated: {at: now, by: test}
scope: {include: [app.py]}
artifacts:
  - id: file.app
    path: app.py
    kind: source
    language: python
    summary: Test source.
    metrics: {lines: 1}
    generated: {method: deterministic, confidence: high, rationale: Test fixture.}
elements:
  - id: component.app
    kind: component
    name: App
    summary: Test app.
    reality: {intent: required, implementation: present, runtime: unknown}
    source_refs: [{artifact: file.app, lines: [1, 2]}]
    generated: {method: deterministic, confidence: high, rationale: Test fixture.}
`
	path := filepath.Join(root, "model.vcm.yaml")
	if err := os.WriteFile(path, []byte(model), 0o600); err != nil {
		t.Fatal(err)
	}
	diagnostics := ValidateDocument(path, ValidationOptions{})
	found := false
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == "structural.sources" && strings.Contains(diagnostic.Message, "outside artifact") {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing source range diagnostic: %+v", diagnostics)
	}
}

func TestStructuralValidationChecksScopeCompleteness(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, name := range []string{"modeled.go", "missing.go"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("package test\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	model := `vcm: "0.1"
model:
  id: model.test
  name: Test
  repository: {root: "` + root + `"}
  generated: {at: now, by: test}
scope: {include: ["*.go"]}
artifacts:
  - id: file.modeled
    path: modeled.go
    kind: source
    language: go
    summary: Modeled source.
    metrics: {lines: 1}
    generated: {method: deterministic, confidence: high, rationale: Test fixture.}
elements:
  - id: component.modeled
    kind: component
    name: Modeled
    summary: Test component.
    reality: {intent: required, implementation: present, runtime: unknown}
    source_refs: [{artifact: file.modeled, lines: [1, 1]}]
    generated: {method: deterministic, confidence: high, rationale: Test fixture.}
`
	path := filepath.Join(root, "model.vcm.yaml")
	if err := os.WriteFile(path, []byte(model), 0o600); err != nil {
		t.Fatal(err)
	}
	diagnostics := ValidateDocument(path, ValidationOptions{})
	found := false
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == "structural.scope" && diagnostic.Severity == "error" && strings.Contains(diagnostic.Message, "missing.go") {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing scope-completeness diagnostic: %+v", diagnostics)
	}
}

func TestStructuralReferenceValidationPreservesProvenanceEvidenceRules(t *testing.T) {
	t.Parallel()
	var model structuralDocument
	model.Artifacts = []artifact{{ID: "file.uncertain", Path: "uncertain.go", Generated: provenance{Method: "deterministic", Confidence: "low"}}}
	model.Elements = []element{{ID: "actor.user", Kind: "actor", Generated: provenance{Method: "human_declared", Confidence: "high"}}}
	model.Flows = []flow{{ID: "flow.uncertain", Trigger: "actor.user", Generated: provenance{Method: "human_declared", Confidence: "low"}}}
	model.Findings = []finding{{
		ID: "finding.uncertain", Subjects: []string{"actor.user"},
		Generated: provenance{Method: "ai_inferred", Confidence: "medium"},
	}}

	err := validateStructuralReferences("model.vcm.yaml", model)
	if err == nil {
		t.Fatal("expected provenance evidence errors")
	}
	for _, expected := range []string{
		`low-confidence artifact "file.uncertain" has no source evidence`,
		`low-confidence flow "flow.uncertain" has no source evidence`,
		`finding "finding.uncertain" requires source evidence`,
	} {
		if !strings.Contains(err.Error(), expected) {
			t.Fatalf("missing %q in %v", expected, err)
		}
	}
}
