package viewer

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	}
	for _, element := range model.Elements {
		registerID(element.ID, "element "+element.ID)
		elements[element.ID] = element
	}
	for _, relation := range model.Relations {
		registerID(relation.ID, "relation "+relation.ID)
	}
	for _, finding := range model.Findings {
		registerID(finding.ID, "finding "+finding.ID)
	}
	checkSources := func(owner string, refs []sourceRef) {
		for _, ref := range refs {
			if _, exists := artifacts[ref.Artifact]; !exists {
				addProblem(fmt.Sprintf("%s references missing artifact %q", owner, ref.Artifact))
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
	}
	for _, relation := range model.Relations {
		if _, exists := elements[relation.From]; !exists {
			addProblem(fmt.Sprintf("relation %q has missing source %q", relation.ID, relation.From))
		}
		if _, exists := elements[relation.To]; !exists {
			addProblem(fmt.Sprintf("relation %q has missing target %q", relation.ID, relation.To))
		}
		checkSources("relation "+relation.ID, relation.Evidence)
	}
	for _, finding := range model.Findings {
		for _, subject := range finding.Subjects {
			if _, exists := identities[subject]; !exists {
				addProblem(fmt.Sprintf("finding %q has missing subject %q", finding.ID, subject))
			}
		}
		checkSources("finding "+finding.ID, finding.Evidence)
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
	for _, artifact := range structural.Artifacts {
		subjects[artifact.ID] = struct{}{}
	}
	for _, element := range structural.Elements {
		subjects[element.ID] = struct{}{}
	}
	metrics := make(map[string]struct{}, len(quality.MetricCatalog))
	metricIDs := make(map[string]bool, len(quality.MetricCatalog))
	measurementIDs := make(map[string]bool, len(quality.Measurements))
	var problems []string
	for _, metric := range quality.MetricCatalog {
		if metricIDs[metric.ID] {
			problems = append(problems, fmt.Sprintf("duplicate metric id %q", metric.ID))
		}
		metricIDs[metric.ID] = true
		metrics[metric.ID] = struct{}{}
	}
	for _, measurement := range quality.Measurements {
		if measurementIDs[measurement.ID] {
			problems = append(problems, fmt.Sprintf("duplicate measurement id %q", measurement.ID))
		}
		measurementIDs[measurement.ID] = true
		if _, exists := subjects[measurement.Subject]; !exists {
			problems = append(problems, fmt.Sprintf("measurement %q has missing subject %q", measurement.ID, measurement.Subject))
		}
		if _, exists := metrics[measurement.Metric]; !exists {
			problems = append(problems, fmt.Sprintf("measurement %q has missing metric %q", measurement.ID, measurement.Metric))
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("%s: quality references failed:\n  %s", file, strings.Join(problems, "\n  "))
	}
	return nil
}

func resolveDocumentPath(base, value string) string {
	if filepath.IsAbs(value) {
		return filepath.Clean(value)
	}
	return filepath.Clean(filepath.Join(base, value))
}
