package projectdsl

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v5"
	"gopkg.in/yaml.v3"
)

type Diagnostic struct {
	File     string `json:"file,omitempty"`
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Path     string `json:"path,omitempty"`
	Line     int    `json:"line,omitempty"`
	Column   int    `json:"column,omitempty"`
	Message  string `json:"message"`
}

func (d Diagnostic) Error() string {
	location := d.File
	if d.Line > 0 {
		location += fmt.Sprintf(":%d", d.Line)
		if d.Column > 0 {
			location += fmt.Sprintf(":%d", d.Column)
		}
	}
	if location != "" {
		location += ": "
	}
	path := ""
	if d.Path != "" {
		path = " " + d.Path
	}
	return fmt.Sprintf("%s%s [%s]%s: %s", location, d.Severity, d.Code, path, d.Message)
}

type manifestSelector struct {
	ElementIDs []string `yaml:"element_ids"`
	Exclude    []string `yaml:"exclude"`
}

type manifestIndex struct {
	Project struct {
		Inputs struct {
			StructuralModel string   `yaml:"structural_model"`
			QualityModel    string   `yaml:"quality_model"`
			RuntimeModels   []string `yaml:"runtime_models"`
			Requirements    []string `yaml:"requirements"`
		} `yaml:"inputs"`
	} `yaml:"project"`
	Analysis struct {
		Languages []struct {
			ID string `yaml:"id"`
		} `yaml:"languages"`
		Scope struct {
			Rules []struct {
				ID       string   `yaml:"id"`
				Patterns []string `yaml:"patterns"`
			} `yaml:"rules"`
		} `yaml:"scope"`
		GeneratedDetection struct {
			Markers []struct {
				ID    string `yaml:"id"`
				Regex string `yaml:"regex"`
			} `yaml:"markers"`
		} `yaml:"generated_detection"`
	} `yaml:"analysis"`
	Decompositions []struct {
		ID        string `yaml:"id"`
		Districts []struct {
			ID      string           `yaml:"id"`
			Code    string           `yaml:"code"`
			Members manifestSelector `yaml:"members"`
		} `yaml:"districts"`
	} `yaml:"decompositions"`
	Expectations []struct {
		ID     string `yaml:"id"`
		Parent string `yaml:"parent"`
	} `yaml:"expectations"`
	Corrections []struct {
		ID     string `yaml:"id"`
		Target struct {
			Entity   string           `yaml:"entity"`
			Selector manifestSelector `yaml:"selector"`
		} `yaml:"target"`
		Operations []struct {
			Action string    `yaml:"action"`
			Value  yaml.Node `yaml:"value"`
		} `yaml:"operations"`
	} `yaml:"corrections"`
	Boundaries []struct {
		ID             string `yaml:"id"`
		Subject        string `yaml:"subject"`
		SourceRelation string `yaml:"source_relation"`
	} `yaml:"boundaries"`
	SecurityReviews []struct {
		ID       string   `yaml:"id"`
		Subjects []string `yaml:"subjects"`
	} `yaml:"security_reviews"`
	RenderProfiles []struct {
		ID            string `yaml:"id"`
		Decomposition string `yaml:"decomposition"`
		Encodings     struct {
			Bands []struct {
				ID     string `yaml:"id"`
				Metric string `yaml:"metric"`
				Order  int    `yaml:"order"`
			} `yaml:"bands"`
		} `yaml:"encodings"`
	} `yaml:"render_profiles"`
}

var (
	compiledSchema     *jsonschema.Schema
	compiledSchemaErr  error
	compiledSchemaOnce sync.Once
	yamlLinePattern    = regexp.MustCompile(`(?i)line\s+(\d+)(?::(\d+))?`)
)

func loadSchema() (*jsonschema.Schema, error) {
	compiledSchemaOnce.Do(func() {
		compiler := jsonschema.NewCompiler()
		if err := compiler.AddResource("vcm-project.schema.json", bytes.NewReader(schemaDocument)); err != nil {
			compiledSchemaErr = err
			return
		}
		compiledSchema, compiledSchemaErr = compiler.Compile("vcm-project.schema.json")
	})
	return compiledSchema, compiledSchemaErr
}

func Validate(data []byte, file string) []Diagnostic {
	root, syntaxDiagnostic := parseYAML(data, file)
	if syntaxDiagnostic != nil {
		return []Diagnostic{*syntaxDiagnostic}
	}

	jsonValue, err := yamlNodeToJSONValue(root)
	if err != nil {
		return []Diagnostic{{File: file, Severity: "error", Code: "yaml.value", Message: err.Error()}}
	}

	schema, err := loadSchema()
	if err != nil {
		return []Diagnostic{{File: file, Severity: "error", Code: "schema.compile", Message: err.Error()}}
	}

	diagnostics := make([]Diagnostic, 0)
	if err := schema.Validate(jsonValue); err != nil {
		var validationError *jsonschema.ValidationError
		if errors.As(err, &validationError) {
			diagnostics = append(diagnostics, schemaDiagnostics(validationError, root, file)...)
		} else {
			diagnostics = append(diagnostics, Diagnostic{File: file, Severity: "error", Code: "schema.validate", Message: err.Error()})
		}
	}

	if len(diagnostics) == 0 {
		diagnostics = append(diagnostics, validateCrossReferences(root, file)...)
	}
	sortDiagnostics(diagnostics)
	return diagnostics
}

func ValidateFile(path string) []Diagnostic {
	data, err := os.ReadFile(path)
	if err != nil {
		return []Diagnostic{{File: path, Severity: "error", Code: "file.read", Message: err.Error()}}
	}
	diagnostics := Validate(data, path)
	if hasErrors(diagnostics) {
		return diagnostics
	}

	root, syntaxDiagnostic := parseYAML(data, path)
	if syntaxDiagnostic != nil {
		return append(diagnostics, *syntaxDiagnostic)
	}
	var index manifestIndex
	if err := root.Decode(&index); err != nil {
		return append(diagnostics, Diagnostic{File: path, Severity: "error", Code: "yaml.decode", Message: err.Error()})
	}

	base := filepath.Dir(path)
	modelPath := resolvePath(base, index.Project.Inputs.StructuralModel)
	modelIndex, modelDiagnostics := readStructuralIndex(modelPath)
	diagnostics = append(diagnostics, modelDiagnostics...)
	if !hasErrors(modelDiagnostics) {
		checkReference := func(id, pointer, label string, candidates map[string]struct{}) {
			if _, exists := candidates[id]; !exists {
				diagnostics = append(diagnostics, diagnosticAt(root, path, pointer, "reference.missing", fmt.Sprintf("%s %q does not exist in the structural model", label, id)))
			}
		}
		for decompositionIndex, decomposition := range index.Decompositions {
			for districtIndex, district := range decomposition.Districts {
				for elementIndex, elementID := range district.Members.ElementIDs {
					checkReference(elementID, fmt.Sprintf("/decompositions/%d/districts/%d/members/element_ids/%d", decompositionIndex, districtIndex, elementIndex), "element", modelIndex.elements)
				}
				for elementIndex, elementID := range district.Members.Exclude {
					checkReference(elementID, fmt.Sprintf("/decompositions/%d/districts/%d/members/exclude/%d", decompositionIndex, districtIndex, elementIndex), "excluded element", modelIndex.elements)
				}
			}
		}
		for correctionIndex, correction := range index.Corrections {
			if correction.Target.Entity != "" {
				checkReference(correction.Target.Entity, fmt.Sprintf("/corrections/%d/target/entity", correctionIndex), "correction target", modelIndex.all)
			}
			for elementIndex, elementID := range correction.Target.Selector.ElementIDs {
				checkReference(elementID, fmt.Sprintf("/corrections/%d/target/selector/element_ids/%d", correctionIndex, elementIndex), "correction target element", modelIndex.elements)
			}
			for elementIndex, elementID := range correction.Target.Selector.Exclude {
				checkReference(elementID, fmt.Sprintf("/corrections/%d/target/selector/exclude/%d", correctionIndex, elementIndex), "excluded correction element", modelIndex.elements)
			}
		}
		for itemIndex, boundary := range index.Boundaries {
			checkReference(boundary.Subject, fmt.Sprintf("/boundaries/%d/subject", itemIndex), "subject", modelIndex.all)
			if boundary.SourceRelation != "" {
				checkReference(boundary.SourceRelation, fmt.Sprintf("/boundaries/%d/source_relation", itemIndex), "relation", modelIndex.relations)
			}
		}
		for reviewIndex, review := range index.SecurityReviews {
			for subjectIndex, subject := range review.Subjects {
				checkReference(subject, fmt.Sprintf("/security_reviews/%d/subjects/%d", reviewIndex, subjectIndex), "subject", modelIndex.all)
			}
		}
	}

	for _, candidate := range append(append([]string{}, index.Project.Inputs.RuntimeModels...), index.Project.Inputs.Requirements...) {
		if candidate == "" {
			continue
		}
		resolved := resolvePath(base, candidate)
		if _, err := os.Stat(resolved); err != nil {
			diagnostics = append(diagnostics, Diagnostic{File: resolved, Severity: "warning", Code: "input.missing", Message: err.Error()})
		}
	}
	if index.Project.Inputs.QualityModel != "" {
		resolved := resolvePath(base, index.Project.Inputs.QualityModel)
		if _, err := os.Stat(resolved); err != nil {
			diagnostics = append(diagnostics, Diagnostic{File: resolved, Severity: "error", Code: "input.missing", Message: err.Error()})
		}
	}

	sortDiagnostics(diagnostics)
	return diagnostics
}

func parseYAML(data []byte, file string) (*yaml.Node, *Diagnostic) {
	var root yaml.Node
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&root); err != nil {
		line, column := yamlErrorLocation(err.Error())
		return nil, &Diagnostic{File: file, Severity: "error", Code: "yaml.syntax", Line: line, Column: column, Message: err.Error()}
	}
	if len(root.Content) == 0 {
		return nil, &Diagnostic{File: file, Severity: "error", Code: "yaml.empty", Message: "document is empty"}
	}
	var trailing yaml.Node
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err != nil {
			line, column := yamlErrorLocation(err.Error())
			return nil, &Diagnostic{File: file, Severity: "error", Code: "yaml.syntax", Line: line, Column: column, Message: err.Error()}
		}
		line, column := 0, 0
		if len(trailing.Content) > 0 {
			line, column = trailing.Content[0].Line, trailing.Content[0].Column
		}
		return nil, &Diagnostic{File: file, Severity: "error", Code: "yaml.multiple_documents", Line: line, Column: column, Message: "exactly one YAML document is allowed"}
	}
	return &root, nil
}

func yamlNodeToJSONValue(root *yaml.Node) (any, error) {
	var value any
	if err := root.Decode(&value); err != nil {
		return nil, fmt.Errorf("decode YAML: %w", err)
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("convert YAML to JSON: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	var jsonValue any
	if err := decoder.Decode(&jsonValue); err != nil {
		return nil, fmt.Errorf("decode JSON value: %w", err)
	}
	return jsonValue, nil
}

func schemaDiagnostics(validationError *jsonschema.ValidationError, root *yaml.Node, file string) []Diagnostic {
	if len(validationError.Causes) == 0 {
		return []Diagnostic{diagnosticAt(root, file, validationError.InstanceLocation, "schema.invalid", validationError.Message)}
	}
	result := make([]Diagnostic, 0)
	for _, cause := range validationError.Causes {
		result = append(result, schemaDiagnostics(cause, root, file)...)
	}
	return result
}

func validateCrossReferences(root *yaml.Node, file string) []Diagnostic {
	var index manifestIndex
	if err := root.Decode(&index); err != nil {
		return []Diagnostic{{File: file, Severity: "error", Code: "yaml.decode", Message: err.Error()}}
	}
	diagnostics := make([]Diagnostic, 0)
	decompositions := make(map[string]struct{})
	districts := make(map[string]struct{})
	globalIDs := make(map[string]string)
	registerID := func(id, pointer, kind string) {
		if previous, duplicate := globalIDs[id]; duplicate {
			diagnostics = append(diagnostics, diagnosticAt(root, file, pointer, "id.duplicate", fmt.Sprintf("%s id %q duplicates %s", kind, id, previous)))
			return
		}
		globalIDs[id] = kind + " at " + pointer
	}
	languages := make(map[string]int)
	for languageIndex, language := range index.Analysis.Languages {
		if previous, duplicate := languages[language.ID]; duplicate {
			diagnostics = append(diagnostics, diagnosticAt(root, file, fmt.Sprintf("/analysis/languages/%d/id", languageIndex), "language.duplicate", fmt.Sprintf("language %q duplicates profile index %d", language.ID, previous)))
		} else {
			languages[language.ID] = languageIndex
		}
	}
	for ruleIndex, rule := range index.Analysis.Scope.Rules {
		registerID(rule.ID, fmt.Sprintf("/analysis/scope/rules/%d/id", ruleIndex), "scope rule")
		for patternIndex, pattern := range rule.Patterns {
			if strings.Contains(pattern, "\\") {
				diagnostics = append(diagnostics, diagnosticAt(root, file, fmt.Sprintf("/analysis/scope/rules/%d/patterns/%d", ruleIndex, patternIndex), "scope.backslash", "globs must use forward slashes"))
			}
		}
	}
	for markerIndex, marker := range index.Analysis.GeneratedDetection.Markers {
		registerID(marker.ID, fmt.Sprintf("/analysis/generated_detection/markers/%d/id", markerIndex), "generated marker")
		if _, err := regexp.Compile(marker.Regex); err != nil {
			diagnostics = append(diagnostics, diagnosticAt(root, file, fmt.Sprintf("/analysis/generated_detection/markers/%d/regex", markerIndex), "marker.regex_invalid", err.Error()))
		}
	}
	for decompositionIndex, decomposition := range index.Decompositions {
		decompositionPointer := fmt.Sprintf("/decompositions/%d/id", decompositionIndex)
		registerID(decomposition.ID, decompositionPointer, "decomposition")
		if _, duplicate := decompositions[decomposition.ID]; duplicate {
			// registerID already emitted the source-located diagnostic.
		} else {
			decompositions[decomposition.ID] = struct{}{}
		}
		codes := make(map[string]int)
		for districtIndex, district := range decomposition.Districts {
			districtPointer := fmt.Sprintf("/decompositions/%d/districts/%d/id", decompositionIndex, districtIndex)
			registerID(district.ID, districtPointer, "district")
			if _, duplicate := districts[district.ID]; !duplicate {
				districts[district.ID] = struct{}{}
			}
			if previous, duplicate := codes[district.Code]; duplicate {
				diagnostics = append(diagnostics, diagnosticAt(root, file, fmt.Sprintf("/decompositions/%d/districts/%d/code", decompositionIndex, districtIndex), "district.code_duplicate", fmt.Sprintf("code %q duplicates district index %d in this decomposition", district.Code, previous)))
			} else {
				codes[district.Code] = districtIndex
			}
		}
	}
	for itemIndex, item := range index.Expectations {
		registerID(item.ID, fmt.Sprintf("/expectations/%d/id", itemIndex), "expectation")
	}
	for itemIndex, item := range index.Corrections {
		registerID(item.ID, fmt.Sprintf("/corrections/%d/id", itemIndex), "correction")
		for operationIndex, operation := range item.Operations {
			pointer := fmt.Sprintf("/corrections/%d/operations/%d", itemIndex, operationIndex)
			hasValue := operation.Value.Kind != 0
			if (operation.Action == "set" || operation.Action == "merge" || operation.Action == "add") && !hasValue {
				diagnostics = append(diagnostics, diagnosticAt(root, file, pointer, "correction.value_required", fmt.Sprintf("action %q requires value", operation.Action)))
			}
			if operation.Action == "remove" && hasValue {
				diagnostic := diagnosticAt(root, file, pointer+"/value", "correction.value_ignored", "remove ignores its value")
				diagnostic.Severity = "warning"
				diagnostics = append(diagnostics, diagnostic)
			}
		}
	}
	for itemIndex, item := range index.Boundaries {
		registerID(item.ID, fmt.Sprintf("/boundaries/%d/id", itemIndex), "boundary")
	}
	for itemIndex, item := range index.SecurityReviews {
		registerID(item.ID, fmt.Sprintf("/security_reviews/%d/id", itemIndex), "security review")
	}
	for itemIndex, item := range index.RenderProfiles {
		registerID(item.ID, fmt.Sprintf("/render_profiles/%d/id", itemIndex), "render profile")
		for bandIndex, band := range item.Encodings.Bands {
			registerID(band.ID, fmt.Sprintf("/render_profiles/%d/encodings/bands/%d/id", itemIndex, bandIndex), "render band")
		}
	}
	for index, expectation := range index.Expectations {
		if expectation.Parent == "" {
			continue
		}
		if _, exists := districts[expectation.Parent]; !exists {
			diagnostics = append(diagnostics, diagnosticAt(root, file, fmt.Sprintf("/expectations/%d/parent", index), "reference.missing", fmt.Sprintf("district %q does not exist", expectation.Parent)))
		}
	}
	for profileIndex, profile := range index.RenderProfiles {
		if _, exists := decompositions[profile.Decomposition]; !exists {
			diagnostics = append(diagnostics, diagnosticAt(root, file, fmt.Sprintf("/render_profiles/%d/decomposition", profileIndex), "reference.missing", fmt.Sprintf("decomposition %q does not exist", profile.Decomposition)))
		}
		seenOrders := make(map[int]int)
		seenMetrics := make(map[string]int)
		for bandIndex, band := range profile.Encodings.Bands {
			if previous, duplicate := seenOrders[band.Order]; duplicate {
				diagnostics = append(diagnostics, diagnosticAt(root, file, fmt.Sprintf("/render_profiles/%d/encodings/bands/%d/order", profileIndex, bandIndex), "band.order_duplicate", fmt.Sprintf("order %d duplicates band index %d", band.Order, previous)))
			} else {
				seenOrders[band.Order] = bandIndex
			}
			if previous, duplicate := seenMetrics[band.Metric]; duplicate {
				diagnostics = append(diagnostics, diagnosticAt(root, file, fmt.Sprintf("/render_profiles/%d/encodings/bands/%d/metric", profileIndex, bandIndex), "band.metric_duplicate", fmt.Sprintf("metric %q duplicates band index %d", band.Metric, previous)))
			} else {
				seenMetrics[band.Metric] = bandIndex
			}
		}
		for expected := 1; expected <= len(profile.Encodings.Bands); expected++ {
			if _, exists := seenOrders[expected]; !exists {
				diagnostics = append(diagnostics, diagnosticAt(root, file, fmt.Sprintf("/render_profiles/%d/encodings/bands", profileIndex), "band.order_gap", fmt.Sprintf("band order %d is missing", expected)))
			}
		}
	}
	return diagnostics
}

type structuralIndex struct {
	all       map[string]struct{}
	elements  map[string]struct{}
	relations map[string]struct{}
}

func readStructuralIndex(path string) (structuralIndex, []Diagnostic) {
	data, err := os.ReadFile(path)
	if err != nil {
		return structuralIndex{}, []Diagnostic{{File: path, Severity: "error", Code: "input.read", Message: err.Error()}}
	}
	root, syntaxDiagnostic := parseYAML(data, path)
	if syntaxDiagnostic != nil {
		return structuralIndex{}, []Diagnostic{*syntaxDiagnostic}
	}
	var model struct {
		Artifacts []struct {
			ID string `yaml:"id"`
		} `yaml:"artifacts"`
		Elements []struct {
			ID string `yaml:"id"`
		} `yaml:"elements"`
		Relations []struct {
			ID string `yaml:"id"`
		} `yaml:"relations"`
		Findings []struct {
			ID string `yaml:"id"`
		} `yaml:"findings"`
	}
	if err := root.Decode(&model); err != nil {
		return structuralIndex{}, []Diagnostic{{File: path, Severity: "error", Code: "input.decode", Message: err.Error()}}
	}
	result := structuralIndex{
		all:       make(map[string]struct{}),
		elements:  make(map[string]struct{}),
		relations: make(map[string]struct{}),
	}
	diagnostics := make([]Diagnostic, 0)
	groups := []struct {
		name  string
		items []struct {
			ID string `yaml:"id"`
		}
	}{
		{name: "artifacts", items: model.Artifacts},
		{name: "elements", items: model.Elements},
		{name: "relations", items: model.Relations},
		{name: "findings", items: model.Findings},
	}
	seenAt := make(map[string]string)
	for _, group := range groups {
		for index, item := range group.items {
			pointer := fmt.Sprintf("/%s/%d/id", group.name, index)
			if item.ID == "" {
				diagnostics = append(diagnostics, diagnosticAt(root, path, pointer, "structural.id_missing", "structural record id is empty"))
				continue
			}
			if previous, duplicate := seenAt[item.ID]; duplicate {
				diagnostics = append(diagnostics, diagnosticAt(root, path, pointer, "structural.id_duplicate", fmt.Sprintf("id %q duplicates %s", item.ID, previous)))
				continue
			}
			seenAt[item.ID] = pointer
			result.all[item.ID] = struct{}{}
			switch group.name {
			case "elements":
				result.elements[item.ID] = struct{}{}
			case "relations":
				result.relations[item.ID] = struct{}{}
			}
		}
	}
	return result, diagnostics
}

func diagnosticAt(root *yaml.Node, file, pointer, code, message string) Diagnostic {
	node := nodeAtPointer(root, pointer)
	diagnostic := Diagnostic{File: file, Severity: "error", Code: code, Path: pointer, Message: message}
	if node != nil {
		diagnostic.Line = node.Line
		diagnostic.Column = node.Column
	}
	return diagnostic
}

func nodeAtPointer(root *yaml.Node, pointer string) *yaml.Node {
	node := root
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		node = node.Content[0]
	}
	if pointer == "" || pointer == "/" {
		return node
	}
	for _, rawPart := range strings.Split(strings.TrimPrefix(pointer, "/"), "/") {
		part := strings.ReplaceAll(strings.ReplaceAll(rawPart, "~1", "/"), "~0", "~")
		switch node.Kind {
		case yaml.MappingNode:
			found := false
			for index := 0; index+1 < len(node.Content); index += 2 {
				if node.Content[index].Value == part {
					node = node.Content[index+1]
					found = true
					break
				}
			}
			if !found {
				return node
			}
		case yaml.SequenceNode:
			index, err := strconv.Atoi(part)
			if err != nil || index < 0 || index >= len(node.Content) {
				return node
			}
			node = node.Content[index]
		default:
			return node
		}
	}
	return node
}

func yamlErrorLocation(message string) (int, int) {
	match := yamlLinePattern.FindStringSubmatch(message)
	if len(match) < 2 {
		return 0, 0
	}
	line, _ := strconv.Atoi(match[1])
	column := 0
	if len(match) > 2 && match[2] != "" {
		column, _ = strconv.Atoi(match[2])
	}
	return line, column
}

func resolvePath(base, value string) string {
	if filepath.IsAbs(value) {
		return value
	}
	return filepath.Clean(filepath.Join(base, value))
}

func hasErrors(diagnostics []Diagnostic) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == "error" {
			return true
		}
	}
	return false
}

func sortDiagnostics(diagnostics []Diagnostic) {
	sort.SliceStable(diagnostics, func(i, j int) bool {
		if diagnostics[i].File != diagnostics[j].File {
			return diagnostics[i].File < diagnostics[j].File
		}
		if diagnostics[i].Line != diagnostics[j].Line {
			return diagnostics[i].Line < diagnostics[j].Line
		}
		if diagnostics[i].Column != diagnostics[j].Column {
			return diagnostics[i].Column < diagnostics[j].Column
		}
		return diagnostics[i].Path < diagnostics[j].Path
	})
}
