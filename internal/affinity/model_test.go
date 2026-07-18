package affinity

import (
	"math"
	"testing"
)

func TestBuildAttenuatesOmnipresentTargets(t *testing.T) {
	profile := Profile{
		ID:                 "static-test",
		DedicationExponent: 1,
		Layers: []LayerProfile{{
			Signal:        SignalReferencedImport,
			Weight:        1,
			Normalization: NormalizeNone,
			AttenuateHubs: true,
		}},
	}
	evidence := []EvidenceEdge{
		{ID: "a-shared", From: "a", To: "shared", Signal: SignalReferencedImport, Strength: 1, Confidence: 1},
		{ID: "b-shared", From: "b", To: "shared", Signal: SignalReferencedImport, Strength: 1, Confidence: 1},
		{ID: "c-shared", From: "c", To: "shared", Signal: SignalReferencedImport, Strength: 1, Confidence: 1},
		{ID: "a-private", From: "a", To: "private", Signal: SignalReferencedImport, Strength: 1, Confidence: 1},
	}

	edges, err := Build(evidence, profile)
	if err != nil {
		t.Fatal(err)
	}
	weights := make(map[string]float64)
	for _, edge := range edges {
		weights[edge.From+"->"+edge.To] = edge.Weight
	}
	if !near(weights["a->shared"], 1.0/3.0) {
		t.Fatalf("shared utility weight = %v, want 1/3", weights["a->shared"])
	}
	if !near(weights["a->private"], 1) {
		t.Fatalf("private dependency weight = %v, want 1", weights["a->private"])
	}
}

func TestBuildPreservesSignalContributions(t *testing.T) {
	profile := Profile{
		ID:                 "multi-signal-test",
		DedicationExponent: 1,
		Layers: []LayerProfile{
			{Signal: SignalCall, Weight: 1, Normalization: NormalizeMax},
			{Signal: SignalSharedFlow, Weight: 0.5, Normalization: NormalizeNone},
		},
	}
	evidence := []EvidenceEdge{
		{ID: "call-1", From: "api", To: "service", Signal: SignalCall, Strength: 4, Confidence: 1},
		{ID: "flow-1", From: "api", To: "service", Signal: SignalSharedFlow, Strength: 1, Confidence: 0.8},
	}

	edges, err := Build(evidence, profile)
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 1 || len(edges[0].Contributions) != 2 {
		t.Fatalf("got %#v, want one edge with two contributions", edges)
	}
	if !near(edges[0].Weight, 1.4) {
		t.Fatalf("combined weight = %v, want 1.4", edges[0].Weight)
	}
}

func TestComputeRoleMetricsSeparatesConnectorParticipation(t *testing.T) {
	edges := []Edge{
		{From: "utility", To: "analysis", Weight: 1},
		{From: "utility", To: "payment", Weight: 1},
		{From: "utility", To: "browser", Weight: 1},
		{From: "analysis", To: "analysis-helper", Weight: 2},
	}
	membership := map[string]string{
		"utility": "shared", "analysis": "analysis", "analysis-helper": "analysis",
		"payment": "payment", "browser": "browser",
	}
	metrics := ComputeRoleMetrics([]string{"utility", "analysis", "analysis-helper", "payment", "browser"}, edges, membership)
	bySubject := make(map[string]RoleMetrics)
	for _, metric := range metrics {
		bySubject[metric.Subject] = metric
	}
	if bySubject["utility"].ParticipationCoefficient <= bySubject["analysis-helper"].ParticipationCoefficient {
		t.Fatalf("utility participation %v should exceed private helper participation %v",
			bySubject["utility"].ParticipationCoefficient,
			bySubject["analysis-helper"].ParticipationCoefficient,
		)
	}
}

func near(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}
