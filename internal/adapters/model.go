// Package adapters defines the stable boundary between repository discovery,
// language-native analyzers, and VibeCodeMap's renderer-neutral evidence core.
package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"github.com/mmirolim/vibecodemap/internal/repository"
	"github.com/mmirolim/vibecodemap/internal/scoping"
)

const (
	RequestSchema = "vibecodemap.adapter-request/0.1"
	EventSchema   = "vibecodemap.evidence-event/0.1"
)

type Capability string

const (
	Artifacts          Capability = "artifacts"
	Symbols            Capability = "symbols"
	Imports            Capability = "imports"
	Calls              Capability = "calls"
	Types              Capability = "types"
	Effects            Capability = "effects"
	Complexity         Capability = "complexity"
	Coverage           Capability = "coverage"
	Tests              Capability = "tests"
	Entrypoints        Capability = "entrypoints"
	UIComposition      Capability = "ui_composition"
	Navigation         Capability = "navigation"
	Lifecycle          Capability = "lifecycle"
	Permissions        Capability = "permissions"
	PlatformBoundaries Capability = "platform_boundaries"
)

func (capability Capability) valid() bool {
	switch capability {
	case Artifacts, Symbols, Imports, Calls, Types, Effects, Complexity, Coverage, Tests,
		Entrypoints, UIComposition, Navigation, Lifecycle, Permissions, PlatformBoundaries:
		return true
	default:
		return false
	}
}

type SupportLevel string

const (
	DetectionOnly SupportLevel = "detection_only"
	Prototype     SupportLevel = "prototype"
	Available     SupportLevel = "available"
)

type Descriptor struct {
	ID           string       `json:"id"`
	Version      string       `json:"version"`
	Languages    []string     `json:"languages"`
	Stacks       []string     `json:"stacks"`
	Capabilities []Capability `json:"capabilities"`
	Support      SupportLevel `json:"support"`
	Summary      string       `json:"summary"`
}

func (descriptor Descriptor) validate() error {
	if descriptor.ID == "" || descriptor.Version == "" {
		return fmt.Errorf("adapter id and version are required")
	}
	if len(descriptor.Languages) == 0 || len(descriptor.Stacks) == 0 {
		return fmt.Errorf("adapter %q must declare languages and stacks", descriptor.ID)
	}
	switch descriptor.Support {
	case DetectionOnly, Prototype, Available:
	default:
		return fmt.Errorf("adapter %q has unsupported level %q", descriptor.ID, descriptor.Support)
	}
	if descriptor.Summary == "" {
		return fmt.Errorf("adapter %q must have a summary", descriptor.ID)
	}
	for label, values := range map[string][]string{
		"language": descriptor.Languages,
		"stack":    descriptor.Stacks,
	} {
		seen := make(map[string]struct{}, len(values))
		for _, value := range values {
			if value == "" {
				return fmt.Errorf("adapter %q has an empty %s", descriptor.ID, label)
			}
			if _, duplicate := seen[value]; duplicate {
				return fmt.Errorf("adapter %q repeats %s %q", descriptor.ID, label, value)
			}
			seen[value] = struct{}{}
		}
	}
	capabilities := make(map[Capability]struct{}, len(descriptor.Capabilities))
	for _, capability := range descriptor.Capabilities {
		if !capability.valid() {
			return fmt.Errorf("adapter %q has unknown capability %q", descriptor.ID, capability)
		}
		if _, duplicate := capabilities[capability]; duplicate {
			return fmt.Errorf("adapter %q repeats capability %q", descriptor.ID, capability)
		}
		capabilities[capability] = struct{}{}
	}
	return nil
}

type Detection struct {
	AdapterID  string       `json:"adapter_id"`
	Stack      string       `json:"stack"`
	Scope      string       `json:"scope"`
	Confidence float64      `json:"confidence"`
	Evidence   []string     `json:"evidence"`
	Support    SupportLevel `json:"support"`
}

type Detector interface {
	Descriptor() Descriptor
	Detect(context.Context, repository.Report) ([]Detection, error)
}

// FileInput is the exact centrally scoped file set sent to an analyzer.
// Adapters do not independently recurse through repositories and accidentally
// reintroduce node_modules, build products, or generated sources.
type FileInput struct {
	Path           string         `json:"path"`
	Size           int64          `json:"size_bytes"`
	Action         scoping.Action `json:"action"`
	Classification string         `json:"classification,omitempty"`
}

type AnalyzeRequest struct {
	Schema       string       `json:"schema"`
	Root         string       `json:"root"`
	AdapterID    string       `json:"adapter_id"`
	Capabilities []Capability `json:"capabilities"`
	Files        []FileInput  `json:"files"`
}

func NewAnalyzeRequest(report repository.Report, descriptor Descriptor, capabilities []Capability) (AnalyzeRequest, error) {
	if err := descriptor.validate(); err != nil {
		return AnalyzeRequest{}, err
	}
	if report.Root == "" {
		return AnalyzeRequest{}, fmt.Errorf("adapter request repository root is required")
	}
	supported := make(map[Capability]struct{}, len(descriptor.Capabilities))
	for _, capability := range descriptor.Capabilities {
		supported[capability] = struct{}{}
	}
	seenCapabilities := make(map[Capability]struct{}, len(capabilities))
	for _, capability := range capabilities {
		if _, exists := supported[capability]; !exists {
			return AnalyzeRequest{}, fmt.Errorf("adapter %q does not declare capability %q", descriptor.ID, capability)
		}
		if _, duplicate := seenCapabilities[capability]; duplicate {
			return AnalyzeRequest{}, fmt.Errorf("adapter request repeats capability %q", capability)
		}
		seenCapabilities[capability] = struct{}{}
	}
	files := make([]FileInput, 0)
	for _, entry := range report.Entries {
		if entry.Kind != repository.File || (entry.Action != scoping.Analyze && entry.Action != scoping.Summarize) {
			continue
		}
		files = append(files, FileInput{
			Path:           entry.Path,
			Size:           entry.Size,
			Action:         entry.Action,
			Classification: entry.Classification,
		})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	capabilities = append([]Capability(nil), capabilities...)
	sort.Slice(capabilities, func(i, j int) bool { return capabilities[i] < capabilities[j] })
	return AnalyzeRequest{
		Schema:       RequestSchema,
		Root:         report.Root,
		AdapterID:    descriptor.ID,
		Capabilities: capabilities,
		Files:        files,
	}, nil
}

type SourceLocation struct {
	Path      string `json:"path"`
	Line      int    `json:"line,omitempty"`
	Column    int    `json:"column,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
	EndColumn int    `json:"end_column,omitempty"`
	Symbol    string `json:"symbol,omitempty"`
}

// EvidenceEvent is deliberately renderer-neutral. Adapter-specific payloads
// remain versioned JSON while identity, provenance, confidence, and source
// navigation are uniform across Python, Go, Swift, Kotlin, and Dart.
type EvidenceEvent struct {
	Schema     string          `json:"schema"`
	ID         string          `json:"id"`
	Kind       string          `json:"kind"`
	Subject    string          `json:"subject"`
	Producer   string          `json:"producer"`
	Confidence float64         `json:"confidence"`
	Source     *SourceLocation `json:"source,omitempty"`
	Payload    json.RawMessage `json:"payload"`
}

func (event EvidenceEvent) Validate() error {
	if event.Schema != EventSchema {
		return fmt.Errorf("evidence event %q has schema %q, want %q", event.ID, event.Schema, EventSchema)
	}
	if event.ID == "" || event.Kind == "" || event.Subject == "" || event.Producer == "" {
		return fmt.Errorf("evidence event identity, kind, subject, and producer are required")
	}
	if math.IsNaN(event.Confidence) || math.IsInf(event.Confidence, 0) || event.Confidence < 0 || event.Confidence > 1 {
		return fmt.Errorf("evidence event %q has confidence outside [0,1]", event.ID)
	}
	if len(event.Payload) == 0 || !json.Valid(event.Payload) {
		return fmt.Errorf("evidence event %q has invalid JSON payload", event.ID)
	}
	if event.Source != nil {
		source := event.Source
		if source.Path == "" {
			return fmt.Errorf("evidence event %q has a source location without a path", event.ID)
		}
		if source.Line < 0 || source.Column < 0 || source.EndLine < 0 || source.EndColumn < 0 {
			return fmt.Errorf("evidence event %q has a negative source position", event.ID)
		}
		if (source.Line == 0 && source.Column > 0) || (source.EndLine == 0 && source.EndColumn > 0) {
			return fmt.Errorf("evidence event %q has a column without a line", event.ID)
		}
		if source.EndLine > 0 && source.Line == 0 {
			return fmt.Errorf("evidence event %q has an end position without a start line", event.ID)
		}
		if source.EndLine > 0 && (source.EndLine < source.Line || (source.EndLine == source.Line && source.EndColumn > 0 && source.EndColumn < source.Column)) {
			return fmt.Errorf("evidence event %q has an end position before its start", event.ID)
		}
	}
	return nil
}

type Sink interface {
	Emit(context.Context, EvidenceEvent) error
}

// Analyzer is implemented by an in-process analyzer or a versioned subprocess
// adapter. Detection-only descriptors intentionally do not satisfy Analyzer.
type Analyzer interface {
	Detector
	Analyze(context.Context, AnalyzeRequest, Sink) error
}

type Registry struct {
	detectors []Detector
}

func NewRegistry(detectors ...Detector) (*Registry, error) {
	seen := make(map[string]struct{}, len(detectors))
	for _, detector := range detectors {
		if detector == nil {
			return nil, fmt.Errorf("nil adapter detector")
		}
		descriptor := detector.Descriptor()
		if err := descriptor.validate(); err != nil {
			return nil, err
		}
		if _, duplicate := seen[descriptor.ID]; duplicate {
			return nil, fmt.Errorf("duplicate adapter id %q", descriptor.ID)
		}
		seen[descriptor.ID] = struct{}{}
	}
	return &Registry{detectors: append([]Detector(nil), detectors...)}, nil
}

func (registry *Registry) Descriptors() []Descriptor {
	result := make([]Descriptor, 0, len(registry.detectors))
	for _, detector := range registry.detectors {
		result = append(result, detector.Descriptor())
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func (registry *Registry) Detect(ctx context.Context, report repository.Report) ([]Detection, error) {
	var result []Detection
	for _, detector := range registry.detectors {
		detections, err := detector.Detect(ctx, report)
		if err != nil {
			return nil, fmt.Errorf("detect with %s: %w", detector.Descriptor().ID, err)
		}
		for index := range detections {
			if detections[index].AdapterID == "" {
				detections[index].AdapterID = detector.Descriptor().ID
			}
			if detections[index].Support == "" {
				detections[index].Support = detector.Descriptor().Support
			}
			sort.Strings(detections[index].Evidence)
			if err := validateDetection(detections[index], detector.Descriptor()); err != nil {
				return nil, err
			}
		}
		result = append(result, detections...)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Confidence != result[j].Confidence {
			return result[i].Confidence > result[j].Confidence
		}
		if result[i].Scope != result[j].Scope {
			return result[i].Scope < result[j].Scope
		}
		return result[i].AdapterID < result[j].AdapterID
	})
	return result, nil
}
