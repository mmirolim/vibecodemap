package viewer

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/mmirolim/vibecodemap/internal/projectdsl"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v5"
	"gopkg.in/yaml.v3"
)

var (
	//go:embed assets/structural.schema.json
	structuralSchemaDocument []byte

	//go:embed assets/quality.schema.json
	qualitySchemaDocument []byte

	structuralSchemaOnce sync.Once
	structuralSchema     *jsonschema.Schema
	structuralSchemaErr  error
	qualitySchemaOnce    sync.Once
	qualitySchema        *jsonschema.Schema
	qualitySchemaErr     error
)

type loadedDocuments struct {
	projectPath    string
	structuralPath string
	qualityPath    string
	repositoryRoot string
	project        projectDocument
	structural     structuralDocument
	quality        qualityDocument
	warnings       []string
}

// ContractSchema returns the embedded JSON Schema for a renderer-consumed
// document kind. The returned bytes are a copy and may be safely modified by
// the caller.
func ContractSchema(kind string) ([]byte, error) {
	var document []byte
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "structural", "structure", "core":
		document = structuralSchemaDocument
	case "quality":
		document = qualitySchemaDocument
	default:
		return nil, fmt.Errorf("schema kind %q is not structural or quality", kind)
	}
	return append([]byte(nil), document...), nil
}

func loadDocuments(projectPath string) (loadedDocuments, error) {
	absoluteProject, err := filepath.Abs(projectPath)
	if err != nil {
		return loadedDocuments{}, fmt.Errorf("resolve project manifest: %w", err)
	}

	diagnostics := projectdsl.ValidateFile(absoluteProject)
	var validationErrors []string
	var warnings []string
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == "error" {
			validationErrors = append(validationErrors, diagnostic.Error())
		} else {
			warnings = append(warnings, diagnostic.Error())
		}
	}
	if len(validationErrors) > 0 {
		return loadedDocuments{}, fmt.Errorf("project validation failed:\n  %s", strings.Join(validationErrors, "\n  "))
	}

	projectData, err := os.ReadFile(absoluteProject)
	if err != nil {
		return loadedDocuments{}, fmt.Errorf("read project manifest: %w", err)
	}
	var project projectDocument
	if err := yaml.Unmarshal(projectData, &project); err != nil {
		return loadedDocuments{}, fmt.Errorf("decode project manifest: %w", err)
	}

	base := filepath.Dir(absoluteProject)
	structuralPath := resolveDocumentPath(base, project.Project.Inputs.StructuralModel)
	structuralData, err := os.ReadFile(structuralPath)
	if err != nil {
		return loadedDocuments{}, fmt.Errorf("read structural model %s: %w", structuralPath, err)
	}
	if err := validateYAMLContract(structuralData, structuralPath, loadStructuralSchema); err != nil {
		return loadedDocuments{}, err
	}
	var structural structuralDocument
	if err := yaml.Unmarshal(structuralData, &structural); err != nil {
		return loadedDocuments{}, fmt.Errorf("decode structural model: %w", err)
	}
	if err := validateStructuralReferences(structuralPath, structural); err != nil {
		return loadedDocuments{}, err
	}
	if err := validateStructuralSources(structuralPath, structural); err != nil {
		return loadedDocuments{}, err
	}
	scopeWarnings, err := validateStructuralScope(structuralPath, structural)
	if err != nil {
		return loadedDocuments{}, err
	}
	warnings = append(warnings, scopeWarnings...)
	if root, available, err := structuralRepositoryRoot(structuralPath, structural); err != nil {
		return loadedDocuments{}, err
	} else if !available {
		warnings = append(warnings, fmt.Sprintf("repository root %s is unavailable; artifact existence, line counts, source ranges, and scope completeness were not checked", root))
	}

	var quality qualityDocument
	qualityPath := ""
	if project.Project.Inputs.QualityModel != "" {
		qualityPath = resolveDocumentPath(base, project.Project.Inputs.QualityModel)
		qualityData, readErr := os.ReadFile(qualityPath)
		if readErr != nil {
			return loadedDocuments{}, fmt.Errorf("read quality model %s: %w", qualityPath, readErr)
		}
		if err := validateYAMLContract(qualityData, qualityPath, loadQualitySchema); err != nil {
			return loadedDocuments{}, err
		}
		if err := yaml.Unmarshal(qualityData, &quality); err != nil {
			return loadedDocuments{}, fmt.Errorf("decode quality model: %w", err)
		}
		if err := validateQualityReferences(qualityPath, structural, quality); err != nil {
			return loadedDocuments{}, err
		}
		warnings = append(warnings, qualityWarnings(quality)...)
	}

	repositoryRoot := structural.Model.Repository.Root
	if repositoryRoot == "" {
		repositoryRoot = project.Project.Repository.Root
		base = filepath.Dir(absoluteProject)
	} else {
		base = filepath.Dir(structuralPath)
	}
	repositoryRoot = resolveDocumentPath(base, repositoryRoot)

	sort.Strings(warnings)
	return loadedDocuments{
		projectPath: absoluteProject, structuralPath: structuralPath,
		qualityPath: qualityPath, repositoryRoot: repositoryRoot,
		project: project, structural: structural, quality: quality,
		warnings: warnings,
	}, nil
}

func validateYAMLContract(data []byte, file string, loader func() (*jsonschema.Schema, error)) error {
	var value any
	yamlDecoder := yaml.NewDecoder(bytes.NewReader(data))
	if err := yamlDecoder.Decode(&value); err != nil {
		return fmt.Errorf("%s: YAML syntax: %w", file, err)
	}
	var trailing any
	if err := yamlDecoder.Decode(&trailing); err != io.EOF {
		if err != nil {
			return fmt.Errorf("%s: YAML syntax: %w", file, err)
		}
		return fmt.Errorf("%s: YAML syntax: exactly one document is allowed", file)
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("%s: convert YAML to JSON: %w", file, err)
	}
	var jsonValue any
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	if err := decoder.Decode(&jsonValue); err != nil {
		return fmt.Errorf("%s: decode JSON value: %w", file, err)
	}
	schema, err := loader()
	if err != nil {
		return fmt.Errorf("compile schema: %w", err)
	}
	if err := schema.Validate(jsonValue); err != nil {
		var validationError *jsonschema.ValidationError
		if !errors.As(err, &validationError) {
			return fmt.Errorf("%s: schema validation: %w", file, err)
		}
		messages := flattenSchemaErrors(validationError)
		return fmt.Errorf("%s: schema validation failed:\n  %s", file, strings.Join(messages, "\n  "))
	}
	return nil
}

func loadStructuralSchema() (*jsonschema.Schema, error) {
	structuralSchemaOnce.Do(func() {
		structuralSchema, structuralSchemaErr = compileSchema("structural.schema.json", structuralSchemaDocument)
	})
	return structuralSchema, structuralSchemaErr
}

func loadQualitySchema() (*jsonschema.Schema, error) {
	qualitySchemaOnce.Do(func() {
		qualitySchema, qualitySchemaErr = compileSchema("quality.schema.json", qualitySchemaDocument)
	})
	return qualitySchema, qualitySchemaErr
}

func compileSchema(name string, data []byte) (*jsonschema.Schema, error) {
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(name, bytes.NewReader(data)); err != nil {
		return nil, err
	}
	return compiler.Compile(name)
}

func flattenSchemaErrors(item *jsonschema.ValidationError) []string {
	if len(item.Causes) == 0 {
		location := item.InstanceLocation
		if location == "" {
			location = "/"
		}
		return []string{location + ": " + item.Message}
	}
	var result []string
	for _, cause := range item.Causes {
		result = append(result, flattenSchemaErrors(cause)...)
	}
	return result
}

func validateStructuralReferences(file string, model structuralDocument) error {
	artifacts := make(map[string]struct{}, len(model.Artifacts))
	elements := make(map[string]element, len(model.Elements))
	relations := make(map[string]struct{}, len(model.Relations))
	identities := make(map[string]string)
	artifactPaths := make(map[string]string)
	problemSet := make(map[string]struct{})
	addProblem := func(problem string) {
		problemSet[problem] = struct{}{}
	}
	registerID := func(id, location string) {
		if previous, exists := identities[id]; exists {
			addProblem(fmt.Sprintf("duplicate id %q at %s; first seen at %s", id, location, previous))
			return
		}
		identities[id] = location
	}
	for _, artifact := range model.Artifacts {
		registerID(artifact.ID, "artifact "+artifact.ID)
		artifacts[artifact.ID] = struct{}{}
		normalized := path.Clean(strings.ReplaceAll(artifact.Path, "\\", "/"))
		hasWindowsVolume := len(normalized) >= 2 && normalized[1] == ':' &&
			((normalized[0] >= 'a' && normalized[0] <= 'z') || (normalized[0] >= 'A' && normalized[0] <= 'Z'))
		if path.IsAbs(normalized) || hasWindowsVolume || normalized == ".." || strings.HasPrefix(normalized, "../") {
			addProblem(fmt.Sprintf("artifact %q path escapes repository root: %q", artifact.ID, artifact.Path))
		}
		if previous, exists := artifactPaths[normalized]; exists {
			addProblem(fmt.Sprintf("artifact path %q is shared by %q and %q", normalized, previous, artifact.ID))
		} else {
			artifactPaths[normalized] = artifact.ID
		}
		if artifact.Generated.Confidence == "low" {
			addProblem(fmt.Sprintf("low-confidence artifact %q has no source evidence", artifact.ID))
		}
	}
	for _, element := range model.Elements {
		registerID(element.ID, "element "+element.ID)
		elements[element.ID] = element
	}
	for _, relation := range model.Relations {
		registerID(relation.ID, "relation "+relation.ID)
		relations[relation.ID] = struct{}{}
	}
	for _, flow := range model.Flows {
		registerID(flow.ID, "flow "+flow.ID)
	}
	for _, style := range model.Architecture.Styles {
		registerID(style.ID, "architecture style "+style.ID)
	}
	for _, constraint := range model.Architecture.Constraints {
		registerID(constraint.ID, "architecture constraint "+constraint.ID)
	}
	for _, finding := range model.Findings {
		registerID(finding.ID, "finding "+finding.ID)
	}
	for _, view := range model.Views {
		registerID(view.ID, "view "+view.ID)
	}
	checkSources := func(owner string, refs []sourceRef) {
		for _, ref := range refs {
			if _, exists := artifacts[ref.Artifact]; !exists {
				addProblem(fmt.Sprintf("%s references missing artifact %q", owner, ref.Artifact))
			}
			if len(ref.Lines) == 2 && ref.Lines[1] < ref.Lines[0] {
				addProblem(fmt.Sprintf("%s has descending source range %v for artifact %q", owner, ref.Lines, ref.Artifact))
			}
		}
	}
	for _, element := range model.Elements {
		if element.Parent != "" {
			if _, exists := elements[element.Parent]; !exists {
				addProblem(fmt.Sprintf("element %q has missing parent %q", element.ID, element.Parent))
			}
		}
		checkSources("element "+element.ID, element.SourceRefs)
		checkSources("element "+element.ID, element.Evidence)
		if element.Kind != "actor" && element.Kind != "external_system" && len(element.SourceRefs)+len(element.Evidence) == 0 {
			addProblem(fmt.Sprintf("internal element %q has no source_refs or evidence", element.ID))
		}
		if element.Reality.Implementation == "missing" && len(element.SourceRefs)+len(element.Evidence) == 0 {
			addProblem(fmt.Sprintf("missing element %q must cite caller or requirement evidence", element.ID))
		}
		if (element.Reality.Runtime == "observed" || element.Reality.Runtime == "contradicted") && element.Generated.Method != "runtime_observed" {
			addProblem(fmt.Sprintf("element %q claims runtime=%q without runtime_observed provenance", element.ID, element.Reality.Runtime))
		}
		if element.Generated.Method == "ai_inferred" && len(element.SourceRefs)+len(element.Evidence) == 0 {
			addProblem(fmt.Sprintf("AI-inferred element %q has no source evidence", element.ID))
		}
		if element.Generated.Confidence == "low" && len(element.SourceRefs)+len(element.Evidence) == 0 {
			addProblem(fmt.Sprintf("low-confidence element %q has no source evidence", element.ID))
		}
	}
	effectTargetKinds := map[string]struct{}{
		"data_store": {}, "resource": {}, "external_system": {}, "operation": {},
		"component": {}, "policy": {}, "expected_component": {},
	}
	for _, relation := range model.Relations {
		if _, exists := elements[relation.From]; !exists {
			addProblem(fmt.Sprintf("relation %q has missing source %q", relation.ID, relation.From))
		}
		if _, exists := elements[relation.To]; !exists {
			addProblem(fmt.Sprintf("relation %q has missing target %q", relation.ID, relation.To))
		}
		checkSources("relation "+relation.ID, relation.Evidence)
		if relation.Effect.Domain != "" {
			if target, exists := elements[relation.To]; exists {
				if _, allowed := effectTargetKinds[target.Kind]; !allowed {
					addProblem(fmt.Sprintf("side-effect relation %q targets unsupported kind %q", relation.ID, target.Kind))
				}
			}
		}
		if (relation.Reality.Runtime == "observed" || relation.Reality.Runtime == "contradicted") && relation.Generated.Method != "runtime_observed" {
			addProblem(fmt.Sprintf("relation %q claims runtime=%q without runtime_observed provenance", relation.ID, relation.Reality.Runtime))
		}
		if relation.Generated.Method == "ai_inferred" && len(relation.Evidence) == 0 {
			addProblem(fmt.Sprintf("AI-inferred relation %q has no source evidence", relation.ID))
		}
		if relation.Generated.Confidence == "low" && len(relation.Evidence) == 0 {
			addProblem(fmt.Sprintf("low-confidence relation %q has no source evidence", relation.ID))
		}
	}
	for _, flow := range model.Flows {
		if _, exists := elements[flow.Trigger]; !exists {
			addProblem(fmt.Sprintf("flow %q has missing trigger %q", flow.ID, flow.Trigger))
		}
		if flow.Generated.Confidence == "low" {
			addProblem(fmt.Sprintf("low-confidence flow %q has no source evidence", flow.ID))
		}
		steps := make(map[string]struct{}, len(flow.Steps))
		for _, step := range flow.Steps {
			if _, duplicate := steps[step.ID]; duplicate {
				addProblem(fmt.Sprintf("flow %q has duplicate step id %q", flow.ID, step.ID))
			}
			steps[step.ID] = struct{}{}
			if _, exists := relations[step.Relation]; !exists {
				addProblem(fmt.Sprintf("flow %q step %q has missing relation %q", flow.ID, step.ID, step.Relation))
			}
		}
		for _, step := range flow.Steps {
			for _, next := range step.Next {
				if _, exists := steps[next]; !exists {
					addProblem(fmt.Sprintf("flow %q step %q has missing next step %q", flow.ID, step.ID, next))
				}
			}
		}
	}
	for _, style := range model.Architecture.Styles {
		if _, exists := elements[style.Scope]; !exists {
			addProblem(fmt.Sprintf("architecture style %q has missing scope %q", style.ID, style.Scope))
		}
		checkSources("architecture style "+style.ID, style.Evidence)
		if (style.Generated.Method == "ai_inferred" || style.Generated.Confidence == "low") && len(style.Evidence) == 0 {
			addProblem(fmt.Sprintf("architecture style %q requires source evidence", style.ID))
		}
	}
	for _, constraint := range model.Architecture.Constraints {
		if _, exists := elements[constraint.Scope]; !exists {
			addProblem(fmt.Sprintf("architecture constraint %q has missing scope %q", constraint.ID, constraint.Scope))
		}
		checkSources("architecture constraint "+constraint.ID, constraint.Evidence)
	}
	for _, finding := range model.Findings {
		for _, subject := range finding.Subjects {
			if _, exists := identities[subject]; !exists {
				addProblem(fmt.Sprintf("finding %q has missing subject %q", finding.ID, subject))
			}
		}
		checkSources("finding "+finding.ID, finding.Evidence)
		if (finding.Generated.Method == "ai_inferred" || finding.Generated.Confidence == "low") && len(finding.Evidence) == 0 {
			addProblem(fmt.Sprintf("finding %q requires source evidence", finding.ID))
		}
	}
	for _, view := range model.Views {
		for _, root := range view.Roots {
			if _, exists := elements[root]; !exists {
				addProblem(fmt.Sprintf("view %q has missing root %q", view.ID, root))
			}
		}
	}
	for _, cycle := range elementParentCycles(elements) {
		addProblem("element parent cycle: " + strings.Join(cycle, " -> "))
	}
	problems := make([]string, 0, len(problemSet))
	for problem := range problemSet {
		problems = append(problems, problem)
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("%s: structural references failed:\n  %s", file, strings.Join(problems, "\n  "))
	}
	return nil
}

func elementParentCycles(elements map[string]element) [][]string {
	cycles := make(map[string][]string)
	for start := range elements {
		positions := make(map[string]int)
		var chain []string
		current := start
		for current != "" {
			if position, exists := positions[current]; exists {
				cycle := append([]string(nil), chain[position:]...)
				cycle = canonicalCycle(cycle)
				key := strings.Join(cycle, "\x00")
				cycles[key] = append(cycle, cycle[0])
				break
			}
			item, exists := elements[current]
			if !exists {
				break
			}
			positions[current] = len(chain)
			chain = append(chain, current)
			current = item.Parent
		}
	}
	keys := make([]string, 0, len(cycles))
	for key := range cycles {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([][]string, 0, len(keys))
	for _, key := range keys {
		result = append(result, cycles[key])
	}
	return result
}

func canonicalCycle(cycle []string) []string {
	if len(cycle) < 2 {
		return cycle
	}
	minimum := 0
	for index := 1; index < len(cycle); index++ {
		if cycle[index] < cycle[minimum] {
			minimum = index
		}
	}
	return append(append([]string(nil), cycle[minimum:]...), cycle[:minimum]...)
}

func validateQualityReferences(file string, structural structuralDocument, quality qualityDocument) error {
	subjects := map[string]struct{}{structural.Model.ID: {}}
	artifacts := make(map[string]struct{}, len(structural.Artifacts))
	for _, artifact := range structural.Artifacts {
		subjects[artifact.ID] = struct{}{}
		artifacts[artifact.ID] = struct{}{}
	}
	for _, element := range structural.Elements {
		subjects[element.ID] = struct{}{}
	}
	for _, relation := range structural.Relations {
		subjects[relation.ID] = struct{}{}
	}
	for _, flow := range structural.Flows {
		subjects[flow.ID] = struct{}{}
	}
	for _, style := range structural.Architecture.Styles {
		subjects[style.ID] = struct{}{}
	}
	for _, constraint := range structural.Architecture.Constraints {
		subjects[constraint.ID] = struct{}{}
	}
	for _, finding := range structural.Findings {
		subjects[finding.ID] = struct{}{}
	}
	for _, view := range structural.Views {
		subjects[view.ID] = struct{}{}
	}
	metrics := make(map[string]struct{}, len(quality.MetricCatalog))
	measurementIDs := make(map[string]struct{}, len(quality.Measurements))
	priorityModels := make(map[string]struct{}, len(quality.PriorityModels))
	layouts := make(map[string]struct{}, len(quality.LayoutProfiles))
	identities := make(map[string]string)
	problemSet := make(map[string]struct{})
	addProblem := func(problem string) { problemSet[problem] = struct{}{} }
	registerID := func(id, owner string) {
		if previous, duplicate := identities[id]; duplicate {
			addProblem(fmt.Sprintf("duplicate id %q at %s; first seen at %s", id, owner, previous))
			return
		}
		identities[id] = owner
	}
	checkSubject := func(owner, subject string) {
		if _, exists := subjects[subject]; !exists {
			addProblem(fmt.Sprintf("%s has missing subject %q", owner, subject))
		}
	}
	checkSources := func(owner string, refs []sourceRef) {
		for _, ref := range refs {
			if _, exists := artifacts[ref.Artifact]; !exists {
				addProblem(fmt.Sprintf("%s references missing artifact %q", owner, ref.Artifact))
			}
			if len(ref.Lines) == 2 && ref.Lines[1] < ref.Lines[0] {
				addProblem(fmt.Sprintf("%s has descending source range %v for artifact %q", owner, ref.Lines, ref.Artifact))
			}
		}
	}
	if quality.Model.CoreModel != structural.Model.ID {
		addProblem(fmt.Sprintf("quality core model %q does not match structural model %q", quality.Model.CoreModel, structural.Model.ID))
	}
	for _, metric := range quality.MetricCatalog {
		if _, duplicate := metrics[metric.ID]; duplicate {
			addProblem(fmt.Sprintf("duplicate metric id %q", metric.ID))
		}
		registerID(metric.ID, "metric "+metric.ID)
		metrics[metric.ID] = struct{}{}
		allowed := false
		for _, aggregation := range metric.Aggregation.Allowed {
			if aggregation == metric.Aggregation.Default {
				allowed = true
				break
			}
		}
		if !allowed {
			addProblem(fmt.Sprintf("metric %q default aggregation %q is not allowed", metric.ID, metric.Aggregation.Default))
		}
	}
	for _, measurement := range quality.Measurements {
		if _, duplicate := measurementIDs[measurement.ID]; duplicate {
			addProblem(fmt.Sprintf("duplicate measurement id %q", measurement.ID))
		}
		registerID(measurement.ID, "measurement "+measurement.ID)
		measurementIDs[measurement.ID] = struct{}{}
		checkSubject("measurement "+measurement.ID, measurement.Subject)
		if _, exists := metrics[measurement.Metric]; !exists {
			addProblem(fmt.Sprintf("measurement %q has missing metric %q", measurement.ID, measurement.Metric))
		}
		if (measurement.Status == "observed" || measurement.Status == "stale") && !measurement.HasValue {
			addProblem(fmt.Sprintf("measurement %q with status %q requires value", measurement.ID, measurement.Status))
		}
		if measurement.Provenance.Method == "ai_inferred" && measurement.HasValue {
			addProblem(fmt.Sprintf("measurement %q is numeric but produced by ai_inferred provenance", measurement.ID))
		}
		checkSources("measurement "+measurement.ID, measurement.SourceRefs)
	}
	for _, finding := range quality.AnalyzerFindings {
		registerID(finding.ID, "analyzer finding "+finding.ID)
		checkSubject("analyzer finding "+finding.ID, finding.Subject)
		checkSources("analyzer finding "+finding.ID, finding.SourceRefs)
	}
	for _, finding := range quality.QualityFindings {
		registerID(finding.ID, "quality finding "+finding.ID)
		for _, subject := range finding.Subjects {
			checkSubject("quality finding "+finding.ID, subject)
		}
		for _, factor := range finding.Factors {
			if _, exists := measurementIDs[factor]; !exists {
				addProblem(fmt.Sprintf("quality finding %q has missing measurement factor %q", finding.ID, factor))
			}
		}
		checkSources("quality finding "+finding.ID, finding.Evidence)
	}
	for _, model := range quality.PriorityModels {
		registerID(model.ID, "priority model "+model.ID)
		priorityModels[model.ID] = struct{}{}
		total := 0.0
		for _, factor := range model.Factors {
			total += factor.Weight
			if _, exists := metrics[factor.Metric]; !exists {
				addProblem(fmt.Sprintf("priority model %q has missing metric %q", model.ID, factor.Metric))
			}
		}
		if math.Abs(total-1) > 1e-9 {
			addProblem(fmt.Sprintf("priority model %q factor weights sum to %.6f, expected 1", model.ID, total))
		}
	}
	for index, result := range quality.PriorityResults {
		if _, exists := priorityModels[result.Model]; !exists {
			addProblem(fmt.Sprintf("priority result %d has missing model %q", index, result.Model))
		}
		checkSubject(fmt.Sprintf("priority result %d", index), result.Subject)
	}
	for _, layout := range quality.LayoutProfiles {
		registerID(layout.ID, "layout profile "+layout.ID)
		layouts[layout.ID] = struct{}{}
		for field, metric := range map[string]string{"footprint": layout.Footprint, "height": layout.Height, "effect_contact.metric": layout.EffectContact.Metric} {
			if metric == "" {
				continue
			}
			if _, exists := metrics[metric]; !exists {
				addProblem(fmt.Sprintf("layout profile %q %s has missing metric %q", layout.ID, field, metric))
			}
		}
	}
	for _, lens := range quality.Lenses {
		registerID(lens.ID, "lens "+lens.ID)
		if _, exists := layouts[lens.Layout]; !exists {
			addProblem(fmt.Sprintf("lens %q has missing layout %q", lens.ID, lens.Layout))
		}
		if lens.Heat.Metric != "" {
			if _, exists := metrics[lens.Heat.Metric]; !exists {
				addProblem(fmt.Sprintf("lens %q has missing heat metric %q", lens.ID, lens.Heat.Metric))
			}
			for _, field := range []string{"name", "low_label", "high_label"} {
				if value, exists := lens.Legend[field]; !exists || fmt.Sprint(value) == "" {
					addProblem(fmt.Sprintf("lens %q requires legend.%s when heat is encoded", lens.ID, field))
				}
			}
		}
	}
	problems := make([]string, 0, len(problemSet))
	for problem := range problemSet {
		problems = append(problems, problem)
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("%s: quality references failed:\n  %s", file, strings.Join(problems, "\n  "))
	}
	return nil
}

func qualityWarnings(quality qualityDocument) []string {
	warnings := make([]string, 0)
	hasUnknownOrStale := false
	for _, measurement := range quality.Measurements {
		if measurement.Status == "unknown" || measurement.Status == "stale" {
			hasUnknownOrStale = true
		}
		if (measurement.Status == "unknown" || measurement.Status == "not_applicable" || measurement.Status == "invalid") && measurement.HasValue {
			warnings = append(warnings, fmt.Sprintf("measurement %q has status %q but also contains value", measurement.ID, measurement.Status))
		}
	}
	if !hasUnknownOrStale {
		warnings = append(warnings, "no unknown or stale quality evidence is represented; verify that absence was not coerced to zero")
	}
	sort.Strings(warnings)
	return warnings
}

func resolveDocumentPath(base, value string) string {
	if filepath.IsAbs(value) {
		return filepath.Clean(value)
	}
	return filepath.Clean(filepath.Join(base, value))
}
