package adapters

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/mmirolim/vibecodemap/internal/repository"
	"github.com/mmirolim/vibecodemap/internal/scoping"
)

const EvidenceBundleSchema = "vibecodemap.evidence-bundle/0.1"

const DefaultAdapterTimeout = 2 * time.Minute

type AnalysisOptions struct {
	AdapterTimeout time.Duration
}

type AnalysisRun struct {
	AdapterID string       `json:"adapter_id"`
	Scopes    []string     `json:"scopes"`
	Support   SupportLevel `json:"support"`
	Status    string       `json:"status"`
	Detail    string       `json:"detail,omitempty"`
	Events    int          `json:"events"`
}

// EvidenceBundle is an intermediate evidence report for an AI or human DSL
// author. It is not VCM DSL and does not infer architecture from file facts.
type EvidenceBundle struct {
	Schema     string          `json:"schema"`
	Root       string          `json:"root"`
	Detections []Detection     `json:"detections"`
	Runs       []AnalysisRun   `json:"runs"`
	Events     []EvidenceEvent `json:"events"`
}

type collectingSink struct {
	events  []EvidenceEvent
	seen    map[string]struct{}
	allowed map[string]struct{}
}

func (sink *collectingSink) Emit(_ context.Context, event EvidenceEvent) error {
	if err := event.Validate(); err != nil {
		return err
	}
	if _, duplicate := sink.seen[event.ID]; duplicate {
		return fmt.Errorf("adapter emitted duplicate evidence id %q", event.ID)
	}
	if event.Source != nil {
		if _, exists := sink.allowed[event.Source.Path]; !exists {
			return fmt.Errorf("evidence event %q cites unscoped source %q", event.ID, event.Source.Path)
		}
	}
	sink.seen[event.ID] = struct{}{}
	sink.events = append(sink.events, event)
	return nil
}

// Analyze runs every implemented analyzer detected in a centrally scoped
// repository report. Detection-only stacks are recorded as not_implemented;
// they are not silently presented as semantically analyzed.
func (registry *Registry) Analyze(ctx context.Context, report repository.Report) (EvidenceBundle, error) {
	return registry.AnalyzeWithOptions(ctx, report, AnalysisOptions{AdapterTimeout: DefaultAdapterTimeout})
}

// AnalyzeWithOptions runs detected analyzers independently. A broken optional
// runtime must not prevent evidence from healthy adapters being written: a
// failed or timed-out run is recorded explicitly and its partial events are
// discarded.
func (registry *Registry) AnalyzeWithOptions(ctx context.Context, report repository.Report, options AnalysisOptions) (EvidenceBundle, error) {
	if options.AdapterTimeout <= 0 {
		return EvidenceBundle{}, fmt.Errorf("adapter timeout must be positive")
	}
	detections, err := registry.Detect(ctx, report)
	if err != nil {
		return EvidenceBundle{}, err
	}
	allowed := make(map[string]struct{})
	for _, entry := range report.Entries {
		if entry.Kind == repository.File && (entry.Action == scoping.Analyze || entry.Action == scoping.Summarize) {
			allowed[entry.Path] = struct{}{}
		}
	}
	sink := &collectingSink{seen: make(map[string]struct{}), allowed: allowed}

	type detectedAdapter struct {
		descriptor Descriptor
		scopes     map[string]struct{}
	}
	detected := make(map[string]*detectedAdapter)
	for _, detection := range detections {
		item := detected[detection.AdapterID]
		if item == nil {
			for _, descriptor := range registry.Descriptors() {
				if descriptor.ID == detection.AdapterID {
					item = &detectedAdapter{descriptor: descriptor, scopes: make(map[string]struct{})}
					detected[detection.AdapterID] = item
					break
				}
			}
		}
		if item != nil {
			item.scopes[detection.Scope] = struct{}{}
		}
	}

	ids := make([]string, 0, len(detected))
	for id := range detected {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	runs := make([]AnalysisRun, 0, len(ids))
	for _, id := range ids {
		item := detected[id]
		scopes := make([]string, 0, len(item.scopes))
		for scope := range item.scopes {
			scopes = append(scopes, scope)
		}
		sort.Strings(scopes)
		run := AnalysisRun{AdapterID: id, Scopes: scopes, Support: item.descriptor.Support}
		analyzer, exists := registry.Analyzer(id)
		if !exists {
			run.Status = "not_implemented"
			run.Detail = "stack detection is available, but semantic analysis is not implemented"
			runs = append(runs, run)
			continue
		}
		if runtime, ok := analyzer.(RuntimeAware); ok {
			if available, detail := runtime.RuntimeStatus(); !available {
				run.Status = "runtime_unavailable"
				run.Detail = detail
				runs = append(runs, run)
				continue
			}
		}
		request, err := NewAnalyzeRequest(report, item.descriptor, item.descriptor.Capabilities, scopes...)
		if err != nil {
			return EvidenceBundle{}, err
		}
		adapterSink := &collectingSink{seen: make(map[string]struct{}), allowed: allowed}
		adapterContext, cancel := context.WithTimeout(ctx, options.AdapterTimeout)
		analyzeErr := analyzer.Analyze(adapterContext, request, adapterSink)
		adapterContextErr := adapterContext.Err()
		cancel()
		if err := ctx.Err(); err != nil {
			return EvidenceBundle{}, err
		}
		if errors.Is(adapterContextErr, context.DeadlineExceeded) {
			run.Status = "timed_out"
			run.Detail = fmt.Sprintf("exceeded adapter timeout %s; partial evidence was discarded", options.AdapterTimeout)
			runs = append(runs, run)
			continue
		}
		if analyzeErr != nil {
			run.Status = "failed"
			run.Detail = fmt.Sprintf("%v; partial evidence was discarded", analyzeErr)
			runs = append(runs, run)
			continue
		}
		for _, event := range adapterSink.events {
			if err := sink.Emit(ctx, event); err != nil {
				return EvidenceBundle{}, fmt.Errorf("merge evidence from %s: %w", id, err)
			}
		}
		run.Status = "completed"
		run.Events = len(adapterSink.events)
		runs = append(runs, run)
	}
	sort.Slice(sink.events, func(i, j int) bool { return sink.events[i].ID < sink.events[j].ID })
	return EvidenceBundle{Schema: EvidenceBundleSchema, Root: report.Root, Detections: detections, Runs: runs, Events: sink.events}, nil
}
