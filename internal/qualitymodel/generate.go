// Package qualitymodel turns centrally scoped adapter evidence into an
// explicit, source-linked VCM quality document. It does not infer architecture
// or invent unavailable measurements.
package qualitymodel

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mmirolim/vibecodemap/internal/adapters"
	"gopkg.in/yaml.v3"
)

const Version = "0.2-draft"

type Options struct {
	Now time.Time
}

type Summary struct {
	Artifacts    int
	Elements     int
	Measurements int
	Unmapped     int
}

type sourceRef struct {
	Artifact string `yaml:"artifact"`
	Symbol   string `yaml:"symbol,omitempty"`
	Lines    []int  `yaml:"lines,omitempty"`
	Role     string `yaml:"role,omitempty"`
	Supports string `yaml:"supports,omitempty"`
}

type structuralInput struct {
	Model struct {
		ID         string `yaml:"id"`
		Repository struct {
			Root     string `yaml:"root"`
			Revision string `yaml:"revision"`
			Dirty    bool   `yaml:"dirty"`
		} `yaml:"repository"`
	} `yaml:"model"`
	Artifacts []struct {
		ID       string             `yaml:"id"`
		Path     string             `yaml:"path"`
		Kind     string             `yaml:"kind"`
		Language string             `yaml:"language"`
		Metrics  map[string]float64 `yaml:"metrics"`
	} `yaml:"artifacts"`
	Elements []struct {
		ID         string      `yaml:"id"`
		SourceRefs []sourceRef `yaml:"source_refs"`
	} `yaml:"elements"`
}

type evidencePayload struct {
	Language    string          `json:"language"`
	Lines       int             `json:"lines"`
	Limitations []string        `json:"limitations"`
	Quality     evidenceQuality `json:"quality"`
	Symbols     []symbolFacts   `json:"symbols"`
}

type evidenceQuality struct {
	ComplexityMax    *float64       `json:"complexity_max"`
	ComplexityTotal  *float64       `json:"complexity_total"`
	NestingMax       *float64       `json:"nesting_max"`
	MaxFunctionLines *float64       `json:"max_function_lines"`
	DecisionTokens   *float64       `json:"decision_tokens"`
	BraceNestingMax  *float64       `json:"brace_nesting_max"`
	Effects          map[string]int `json:"effects"`
}

type symbolFacts struct {
	Name       string         `json:"name"`
	Line       int            `json:"line"`
	EndLine    int            `json:"end_line"`
	Complexity *float64       `json:"complexity"`
	MaxNesting *float64       `json:"max_nesting"`
	Effects    map[string]int `json:"effects"`
}

type document struct {
	Version string `yaml:"vcm_quality"`
	Model   struct {
		CoreModel   string `yaml:"core_model"`
		Revision    string `yaml:"revision"`
		Dirty       bool   `yaml:"dirty"`
		GeneratedAt string `yaml:"generated_at"`
		Scope       string `yaml:"scope"`
		Notes       string `yaml:"notes,omitempty"`
	} `yaml:"model"`
	MetricCatalog []metricDefinition `yaml:"metric_catalog"`
	Measurements  []measurement      `yaml:"measurements"`
}

type metricDefinition struct {
	ID           string   `yaml:"id"`
	Name         string   `yaml:"name"`
	ValueType    string   `yaml:"value_type"`
	Unit         string   `yaml:"unit"`
	Direction    string   `yaml:"direction"`
	Description  string   `yaml:"description,omitempty"`
	SubjectKinds []string `yaml:"subject_kinds,omitempty"`
	Aggregation  struct {
		Default   string   `yaml:"default"`
		Allowed   []string `yaml:"allowed"`
		Forbidden []string `yaml:"forbidden,omitempty"`
	} `yaml:"aggregation"`
	Caveats []string `yaml:"caveats,omitempty"`
}

type measurement struct {
	ID         string         `yaml:"id"`
	Subject    string         `yaml:"subject"`
	Metric     string         `yaml:"metric"`
	Status     string         `yaml:"status"`
	Value      *float64       `yaml:"value,omitempty"`
	Dimensions map[string]any `yaml:"dimensions,omitempty"`
	MeasuredAt string         `yaml:"measured_at"`
	Revision   string         `yaml:"revision"`
	SourceRefs []sourceRef    `yaml:"source_refs,omitempty"`
	Provenance provenance     `yaml:"provenance"`
}

type provenance struct {
	Producer        string   `yaml:"producer"`
	ProducerVersion string   `yaml:"producer_version,omitempty"`
	Method          string   `yaml:"method"`
	Configuration   string   `yaml:"configuration,omitempty"`
	Scope           string   `yaml:"scope"`
	Limitations     []string `yaml:"limitations,omitempty"`
}

type decodedEvent struct {
	event   adapters.EvidenceEvent
	payload evidencePayload
}

func Generate(structuralPath, evidencePath string, options Options) ([]byte, Summary, error) {
	structuralData, err := os.ReadFile(structuralPath)
	if err != nil {
		return nil, Summary{}, fmt.Errorf("read structural model: %w", err)
	}
	var structural structuralInput
	if err := yaml.Unmarshal(structuralData, &structural); err != nil {
		return nil, Summary{}, fmt.Errorf("decode structural model: %w", err)
	}
	if structural.Model.ID == "" {
		return nil, Summary{}, fmt.Errorf("structural model id is required")
	}

	evidenceData, err := os.ReadFile(evidencePath)
	if err != nil {
		return nil, Summary{}, fmt.Errorf("read evidence bundle: %w", err)
	}
	var bundle adapters.EvidenceBundle
	if err := json.Unmarshal(evidenceData, &bundle); err != nil {
		return nil, Summary{}, fmt.Errorf("decode evidence bundle: %w", err)
	}
	if bundle.Schema != adapters.EvidenceBundleSchema {
		return nil, Summary{}, fmt.Errorf("evidence schema %q is not supported", bundle.Schema)
	}
	if strings.TrimSpace(bundle.Root) == "" {
		return nil, Summary{}, fmt.Errorf("evidence bundle repository root is required")
	}
	seenEventIDs := make(map[string]struct{}, len(bundle.Events))
	for _, event := range bundle.Events {
		if err := event.Validate(); err != nil {
			return nil, Summary{}, fmt.Errorf("validate evidence event: %w", err)
		}
		if _, duplicate := seenEventIDs[event.ID]; duplicate {
			return nil, Summary{}, fmt.Errorf("evidence bundle repeats event id %q", event.ID)
		}
		seenEventIDs[event.ID] = struct{}{}
	}
	if structural.Model.Repository.Root != "" && bundle.Root != "" {
		modelRoot := structural.Model.Repository.Root
		if !filepath.IsAbs(modelRoot) {
			modelRoot = filepath.Join(filepath.Dir(structuralPath), modelRoot)
		}
		modelRoot, err = filepath.Abs(modelRoot)
		if err != nil {
			return nil, Summary{}, fmt.Errorf("resolve structural repository root: %w", err)
		}
		evidenceRoot, err := filepath.Abs(bundle.Root)
		if err != nil {
			return nil, Summary{}, fmt.Errorf("resolve evidence repository root: %w", err)
		}
		if filepath.Clean(modelRoot) != filepath.Clean(evidenceRoot) {
			return nil, Summary{}, fmt.Errorf("evidence repository root %q does not match structural repository root %q", filepath.Clean(evidenceRoot), filepath.Clean(modelRoot))
		}
	}

	now := options.Now
	if now.IsZero() {
		now = time.Now()
	}
	measuredAt := now.UTC().Format(time.RFC3339)
	revision := structural.Model.Repository.Revision
	if revision == "" {
		revision = "working_tree"
	}

	var result document
	result.Version = Version
	result.Model.CoreModel = structural.Model.ID
	result.Model.Revision = revision
	result.Model.Dirty = structural.Model.Repository.Dirty
	result.Model.GeneratedAt = measuredAt
	result.Model.Scope = "centrally scoped adapter evidence from " + evidencePath
	result.Model.Notes = "Generated deterministic measurements remain static evidence; missing coverage and unsupported metrics stay unknown."
	result.MetricCatalog = defaultMetricCatalog()

	artifactsByPath := make(map[string]struct {
		ID       string
		Kind     string
		Language string
		Lines    float64
	}, len(structural.Artifacts))
	for _, item := range structural.Artifacts {
		artifactsByPath[item.Path] = struct {
			ID       string
			Kind     string
			Language string
			Lines    float64
		}{ID: item.ID, Kind: item.Kind, Language: strings.ToLower(item.Language), Lines: item.Metrics["lines"]}
	}

	eventsByPath := make(map[string]decodedEvent)
	summary := Summary{}
	for _, event := range bundle.Events {
		artifact, mapped := artifactsByPath[event.Subject]
		if !mapped {
			summary.Unmapped++
			continue
		}
		var payload evidencePayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return nil, Summary{}, fmt.Errorf("decode evidence payload %s: %w", event.ID, err)
		}
		decoded := decodedEvent{event: event, payload: payload}
		eventsByPath[event.Subject] = decoded
		before := len(result.Measurements)
		result.Measurements = appendArtifactMeasurements(result.Measurements, artifact.ID, event.Subject, artifact.Language, decoded, measuredAt, revision)
		if len(result.Measurements) > before {
			summary.Artifacts++
		}
	}

	for _, artifact := range structural.Artifacts {
		if !isCodeArtifact(artifact.Kind, artifact.Language) {
			continue
		}
		// Add explicit fallbacks for every code artifact. uniqueMeasurements keeps
		// an observed value when one was produced, while summary-only, parse-error,
		// and unsupported evidence remains unknown rather than becoming zero.
		result.Measurements = append(result.Measurements,
			unknownMeasurement(artifact.ID, "complexity.cyclomatic.max", artifact.Path, measuredAt, revision, "No implemented analyzer supplied cyclomatic complexity for this artifact."),
			unknownMeasurement(artifact.ID, "effects.mutating_sites", artifact.Path, measuredAt, revision, "No implemented analyzer supplied direct mutating-effect evidence for this artifact."),
		)
		result.Measurements = append(result.Measurements, unknownMeasurement(artifact.ID, "test.line_coverage", artifact.Path, measuredAt, revision, "No coverage report was supplied; unknown is not zero."))
	}

	for _, element := range structural.Elements {
		measurements := elementMeasurements(element.ID, element.SourceRefs, artifactsByPath, eventsByPath, measuredAt, revision)
		if len(measurements) > 0 {
			summary.Elements++
			result.Measurements = append(result.Measurements, measurements...)
		}
	}

	result.Measurements = uniqueMeasurements(result.Measurements)
	summary.Measurements = len(result.Measurements)
	data, err := yaml.Marshal(result)
	if err != nil {
		return nil, Summary{}, fmt.Errorf("encode quality model: %w", err)
	}
	header := "# Generated from source-linked adapter evidence. Regenerate after analysis; do not treat review metrics as correctness scores.\n"
	return append([]byte(header), data...), summary, nil
}

func appendArtifactMeasurements(items []measurement, subject, path, language string, decoded decodedEvent, measuredAt, revision string) []measurement {
	payload := decoded.payload
	source := evidenceSource(subject, decoded.event, payload.Lines)
	producer := decoded.event.Producer
	limitations := append([]string(nil), payload.Limitations...)
	addObserved := func(metric string, value *float64, configuration string) {
		if value == nil {
			return
		}
		items = append(items, observedMeasurement(subject, metric, *value, path, language, measuredAt, revision, source, producer, configuration, limitations))
	}
	if language == "go" || language == "python" {
		addObserved("complexity.cyclomatic.max", payload.Quality.ComplexityMax, language+"-ast-cyclomatic-v0.1")
		addObserved("complexity.nesting.max", payload.Quality.NestingMax, language+"-ast-nesting-v0.1")
		addObserved("function.lines.max", payload.Quality.MaxFunctionLines, language+"-ast-function-lines-v0.1")
	}
	if language == "javascript" || language == "typescript" {
		addObserved("complexity.decision_points.file", payload.Quality.DecisionTokens, "lexical-decision-points-v0.1")
		addObserved("complexity.brace_nesting.max", payload.Quality.BraceNestingMax, "lexical-brace-nesting-v0.1")
		items = append(items, unknownMeasurement(subject, "complexity.cyclomatic.max", path, measuredAt, revision, "The lexical JS/TS adapter does not compute compiler-grade per-operation cyclomatic complexity."))
	}
	lineValue := float64(payload.Lines)
	if lineValue == 0 && decoded.event.Source != nil && decoded.event.Source.Line == 1 {
		lineValue = float64(decoded.event.Source.EndLine)
	}
	if lineValue > 0 {
		items = append(items, observedMeasurement(subject, "code.lines.physical", lineValue, path, language, measuredAt, revision, source, producer, "source-line-count-v0.1", limitations))
	}
	if payload.Quality.Effects != nil {
		mutations := float64(mutatingEffectSites(payload.Quality.Effects))
		items = append(items, observedMeasurement(subject, "effects.mutating_sites", mutations, path, language, measuredAt, revision, source, producer, "conservative-direct-effects-v0.1", append(limitations, "Only directly recognized mutating calls are counted; transitive effects remain unresolved.")))
	}
	return items
}

func elementMeasurements(elementID string, refs []sourceRef, artifacts map[string]struct {
	ID       string
	Kind     string
	Language string
	Lines    float64
}, events map[string]decodedEvent, measuredAt, revision string) []measurement {
	var matched []struct {
		facts    symbolFacts
		ref      sourceRef
		path     string
		language string
		producer string
		limits   []string
	}
	pathByArtifact := make(map[string]string, len(artifacts))
	for path, artifact := range artifacts {
		pathByArtifact[artifact.ID] = path
	}
	seenMatches := make(map[string]struct{})
	for _, ref := range refs {
		path := pathByArtifact[ref.Artifact]
		decoded, exists := events[path]
		if !exists {
			continue
		}
		for _, symbol := range decoded.payload.Symbols {
			if sourceMatchesSymbol(ref, symbol) {
				key := fmt.Sprintf("%s\x00%s\x00%d\x00%d", path, symbol.Name, symbol.Line, symbol.EndLine)
				if _, duplicate := seenMatches[key]; duplicate {
					continue
				}
				seenMatches[key] = struct{}{}
				matched = append(matched, struct {
					facts    symbolFacts
					ref      sourceRef
					path     string
					language string
					producer string
					limits   []string
				}{symbol, ref, path, decoded.payload.Language, decoded.event.Producer, decoded.payload.Limitations})
			}
		}
	}
	if len(matched) == 0 {
		return nil
	}

	maxComplexity, hasComplexity := 0.0, false
	maxNesting, hasNesting := 0.0, false
	maxLines := 0.0
	mutations := 0.0
	hasMutations := false
	producers := make(map[string]struct{})
	var sources []sourceRef
	var limitations []string
	for _, item := range matched {
		if item.facts.Complexity != nil {
			if !hasComplexity || *item.facts.Complexity > maxComplexity {
				maxComplexity = *item.facts.Complexity
			}
			hasComplexity = true
		}
		if item.facts.MaxNesting != nil {
			if !hasNesting || *item.facts.MaxNesting > maxNesting {
				maxNesting = *item.facts.MaxNesting
			}
			hasNesting = true
		}
		if item.facts.EndLine >= item.facts.Line && item.facts.Line > 0 {
			maxLines = max(maxLines, float64(item.facts.EndLine-item.facts.Line+1))
		}
		if item.facts.Effects != nil {
			mutations += float64(mutatingEffectSites(item.facts.Effects))
			hasMutations = true
		}
		producers[item.producer] = struct{}{}
		limitations = append(limitations, item.limits...)
		ref := sourceRef{Artifact: item.ref.Artifact, Symbol: item.facts.Name, Role: "evidence"}
		if item.facts.Line > 0 {
			end := item.facts.EndLine
			if end < item.facts.Line {
				end = item.facts.Line
			}
			ref.Lines = []int{item.facts.Line, end}
		}
		sources = append(sources, ref)
	}
	producerList := make([]string, 0, len(producers))
	for producer := range producers {
		producerList = append(producerList, producer)
	}
	sort.Strings(producerList)
	producer := strings.Join(producerList, "+")
	path := "mapped structural element " + elementID
	var result []measurement
	appendMetric := func(metric string, value float64, present bool, configuration string) {
		if !present {
			return
		}
		result = append(result, observedMeasurement(elementID, metric, value, path, "mixed", measuredAt, revision, sources, producer, configuration, uniqueStrings(limitations)))
	}
	appendMetric("complexity.cyclomatic.max", maxComplexity, hasComplexity, "matched-symbol-cyclomatic-v0.1")
	appendMetric("complexity.nesting.max", maxNesting, hasNesting, "matched-symbol-nesting-v0.1")
	appendMetric("function.lines.max", maxLines, maxLines > 0, "matched-symbol-function-lines-v0.1")
	appendMetric("effects.mutating_sites", mutations, hasMutations, "matched-symbol-direct-effects-v0.1")
	return result
}

func sourceMatchesSymbol(ref sourceRef, symbol symbolFacts) bool {
	if ref.Symbol != "" {
		return ref.Symbol == symbol.Name || strings.HasSuffix(symbol.Name, "."+ref.Symbol)
	}
	return len(ref.Lines) == 2 && symbol.Line > 0 && symbol.Line >= ref.Lines[0] && symbol.Line <= ref.Lines[1]
}

func evidenceSource(artifactID string, event adapters.EvidenceEvent, payloadLines int) []sourceRef {
	ref := sourceRef{Artifact: artifactID, Role: "evidence"}
	end := payloadLines
	if event.Source != nil && event.Source.EndLine > end {
		end = event.Source.EndLine
	}
	if end > 0 {
		ref.Lines = []int{1, end}
	}
	return []sourceRef{ref}
}

func observedMeasurement(subject, metric string, value float64, scope, language, measuredAt, revision string, sources []sourceRef, producer, configuration string, limitations []string) measurement {
	copy := value
	return measurement{
		ID: "measurement." + subject + "." + metric, Subject: subject, Metric: metric, Status: "observed", Value: &copy,
		Dimensions: map[string]any{"language": language}, MeasuredAt: measuredAt, Revision: revision, SourceRefs: sources,
		Provenance: provenance{Producer: producer, ProducerVersion: "0.1", Method: "deterministic", Configuration: configuration, Scope: scope, Limitations: uniqueStrings(limitations)},
	}
}

func unknownMeasurement(subject, metric, scope, measuredAt, revision, limitation string) measurement {
	return measurement{
		ID: "measurement." + subject + "." + metric, Subject: subject, Metric: metric, Status: "unknown",
		MeasuredAt: measuredAt, Revision: revision,
		Provenance: provenance{Producer: "vibecodemap-quality-bridge", ProducerVersion: "0.1", Method: "deterministic", Configuration: "explicit-unknown-v0.1", Scope: scope, Limitations: []string{limitation}},
	}
}

func uniqueMeasurements(items []measurement) []measurement {
	byID := make(map[string]measurement, len(items))
	for _, item := range items {
		previous, exists := byID[item.ID]
		if !exists || (previous.Status == "unknown" && item.Status == "observed") {
			byID[item.ID] = item
		}
	}
	result := make([]measurement, 0, len(byID))
	for _, item := range byID {
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func mutatingEffectSites(effects map[string]int) int {
	total := 0
	for effect, count := range effects {
		lower := strings.ToLower(effect)
		if strings.Contains(lower, "write") || strings.Contains(lower, "mutate") || strings.Contains(lower, "delete") || strings.Contains(lower, "remove") || strings.Contains(lower, "create") || strings.Contains(lower, "update") || strings.Contains(lower, "spawn") {
			total += count
		}
	}
	return total
}

func isCodeArtifact(kind, language string) bool {
	if kind != "source" && kind != "test" && kind != "template" {
		return false
	}
	return language != "" && language != "markdown" && language != "text"
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func defaultMetricCatalog() []metricDefinition {
	definitions := []metricDefinition{
		metric("code.lines.physical", "Physical source lines", "integer", "lines", "contextual", "sum", []string{"sum", "distribution"}, "Source size is context, not a defect score."),
		metric("complexity.cyclomatic.max", "Maximum operation cyclomatic complexity", "number", "paths", "contextual", "max", []string{"max", "p95", "distribution"}, "Language analyzers can count constructs differently; compare within one producer/configuration."),
		metric("complexity.nesting.max", "Maximum operation nesting", "integer", "levels", "contextual", "max", []string{"max", "p95", "distribution"}, "Nesting is a review signal, not proof of poor design."),
		metric("function.lines.max", "Longest operation", "integer", "lines", "contextual", "max", []string{"max", "p95", "distribution"}, "Long functions can be appropriate for generated or declarative code."),
		metric("effects.mutating_sites", "Direct mutating effect sites", "integer", "sites", "contextual", "sum", []string{"sum", "max", "distribution"}, "Only directly recognized calls are counted; transitive effects remain unknown."),
		metric("complexity.decision_points.file", "Lexical decision points per file", "integer", "tokens", "contextual", "sum", []string{"sum", "max", "distribution"}, "JS/TS lexical tokens are not compiler-grade cyclomatic complexity."),
		metric("complexity.brace_nesting.max", "Maximum lexical brace nesting", "integer", "levels", "contextual", "max", []string{"max", "distribution"}, "Brace nesting can be distorted by syntax the lexical prototype does not resolve."),
		metric("test.line_coverage", "Line coverage", "ratio", "ratio", "lower_is_risk", "weighted_ratio", []string{"weighted_ratio", "distribution"}, "Coverage requires a named, revision-matched coverage report and does not prove correctness."),
	}
	sort.Slice(definitions, func(i, j int) bool { return definitions[i].ID < definitions[j].ID })
	return definitions
}

func metric(id, name, valueType, unit, direction, defaultAggregation string, allowed []string, caveat string) metricDefinition {
	result := metricDefinition{ID: id, Name: name, ValueType: valueType, Unit: unit, Direction: direction, SubjectKinds: []string{"artifact", "element"}, Caveats: []string{caveat}}
	result.Aggregation.Default = defaultAggregation
	result.Aggregation.Allowed = allowed
	if id == "complexity.cyclomatic.max" {
		result.Aggregation.Forbidden = []string{"sum_as_quality"}
	}
	return result
}
