package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mmirolim/vibecodemap/internal/repository"
	"github.com/mmirolim/vibecodemap/internal/scoping"
)

type controlledAnalyzer struct {
	descriptor Descriptor
	analyze    func(context.Context, AnalyzeRequest, Sink) error
}

func (analyzer controlledAnalyzer) Descriptor() Descriptor { return analyzer.descriptor }

func (analyzer controlledAnalyzer) Detect(_ context.Context, _ repository.Report) ([]Detection, error) {
	return []Detection{{
		AdapterID:  analyzer.descriptor.ID,
		Stack:      analyzer.descriptor.Stacks[0],
		Scope:      ".",
		Confidence: 1,
		Evidence:   []string{"fixture.test"},
		Support:    analyzer.descriptor.Support,
	}}, nil
}

func (analyzer controlledAnalyzer) Analyze(ctx context.Context, request AnalyzeRequest, sink Sink) error {
	return analyzer.analyze(ctx, request, sink)
}

func TestAnalyzeRecordsTimeoutAndReturnsWithoutPartialEvidence(t *testing.T) {
	descriptor := testAnalyzerDescriptor()
	registry, err := NewRegistry(controlledAnalyzer{
		descriptor: descriptor,
		analyze: func(ctx context.Context, _ AnalyzeRequest, _ Sink) error {
			<-ctx.Done()
			return ctx.Err()
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	started := time.Now()
	bundle, err := registry.AnalyzeWithOptions(context.Background(), testAnalyzerReport(), AnalysisOptions{AdapterTimeout: 20 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("analysis did not honor timeout: %s", elapsed)
	}
	if len(bundle.Runs) != 1 || bundle.Runs[0].Status != "timed_out" {
		t.Fatalf("runs = %+v", bundle.Runs)
	}
	if !strings.Contains(bundle.Runs[0].Detail, "20ms") || len(bundle.Events) != 0 {
		t.Fatalf("unexpected timeout result: runs=%+v events=%+v", bundle.Runs, bundle.Events)
	}
}

func TestAnalyzeRecordsFailureAndDiscardsPartialEvidence(t *testing.T) {
	descriptor := testAnalyzerDescriptor()
	registry, err := NewRegistry(controlledAnalyzer{
		descriptor: descriptor,
		analyze: func(ctx context.Context, _ AnalyzeRequest, sink Sink) error {
			if err := sink.Emit(ctx, EvidenceEvent{
				Schema: EventSchema, ID: "test.partial", Kind: "test.file", Subject: "fixture.test",
				Producer: descriptor.ID, Confidence: 1, Source: &SourceLocation{Path: "fixture.test"},
				Payload: json.RawMessage(`{"partial":true}`),
			}); err != nil {
				return err
			}
			return errors.New("fixture analyzer failed")
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	bundle, err := registry.AnalyzeWithOptions(context.Background(), testAnalyzerReport(), AnalysisOptions{AdapterTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if len(bundle.Runs) != 1 || bundle.Runs[0].Status != "failed" || !strings.Contains(bundle.Runs[0].Detail, "fixture analyzer failed") {
		t.Fatalf("runs = %+v", bundle.Runs)
	}
	if len(bundle.Events) != 0 {
		t.Fatalf("partial events were retained: %+v", bundle.Events)
	}
}

func testAnalyzerDescriptor() Descriptor {
	return Descriptor{
		ID: "test-analyzer", Version: "0.1", Languages: []string{"test"}, Stacks: []string{"test"},
		Capabilities: []Capability{Artifacts}, Support: Prototype, Summary: "Controlled test analyzer.",
	}
}

func testAnalyzerReport() repository.Report {
	return repository.Report{Root: "/repo", Entries: []repository.Entry{{
		Path: "fixture.test", Kind: repository.File, Action: scoping.Analyze,
	}}}
}
