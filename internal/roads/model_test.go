package roads

import (
	"math"
	"testing"
)

func TestBuildAggregatesRoadsLanesAndFeeders(t *testing.T) {
	districts := []District{
		{ID: "district.api", Code: "D2", Name: "Interfaces"},
		{ID: "district.services", Code: "D3", Name: "Services"},
		{ID: "district.state", Code: "D4", Name: "State"},
	}
	membership := map[string]string{
		"api":      "district.api",
		"analysis": "district.services",
		"billing":  "district.services",
		"database": "district.state",
	}
	relations := []Relation{
		{ID: "r1", FromSubject: "api", ToSubject: "analysis", Family: "call", Execution: "sync", Count: 3, Strength: 2.5},
		{ID: "r2", FromSubject: "api", ToSubject: "billing", Family: "call", Execution: "sync", Count: 2, Strength: 1.5},
		{ID: "r3", FromSubject: "billing", ToSubject: "database", Family: "state", Execution: "sync", Count: 4, Strength: 4},
		{ID: "r4", FromSubject: "analysis", ToSubject: "billing", Family: "call", Execution: "sync", Count: 1},
		{ID: "r5", FromSubject: "unknown", ToSubject: "api", Family: "call", Execution: "sync", Count: 1},
	}

	network, err := Build(districts, membership, relations)
	if err != nil {
		t.Fatal(err)
	}
	if len(network.Roads) != 2 {
		t.Fatalf("roads = %d, want 2: %+v", len(network.Roads), network.Roads)
	}
	first := network.Roads[0]
	if first.ID != "road.d2-d3" {
		t.Fatalf("first road id = %q, want road.d2-d3", first.ID)
	}
	if first.TotalCount != 5 || len(first.Lanes) != 1 {
		t.Fatalf("unexpected aggregate: %+v", first)
	}
	if first.A.OppositeCode != "D3" || first.B.OppositeCode != "D2" {
		t.Fatalf("wrong endpoint labels: %+v %+v", first.A, first.B)
	}
	if len(network.Feeders) != 5 {
		t.Fatalf("feeders = %d, want 5: %+v", len(network.Feeders), network.Feeders)
	}
	if len(network.LocalRelations) != 1 || network.LocalRelations[0] != "r4" {
		t.Fatalf("unexpected local relations: %v", network.LocalRelations)
	}
	if len(network.Unrouted) != 1 || network.Unrouted[0] != "r5" {
		t.Fatalf("unexpected unrouted relations: %v", network.Unrouted)
	}
}

func TestBuildRetainsDirectionAndExecutionAsSeparateLanes(t *testing.T) {
	districts := []District{{ID: "a", Code: "A"}, {ID: "b", Code: "B"}}
	membership := map[string]string{"one": "a", "two": "b"}
	relations := []Relation{
		{ID: "sync", FromSubject: "one", ToSubject: "two", Family: "call", Execution: "sync"},
		{ID: "async", FromSubject: "one", ToSubject: "two", Family: "call", Execution: "async"},
		{ID: "return", FromSubject: "two", ToSubject: "one", Family: "event", Execution: "callback"},
	}

	network, err := Build(districts, membership, relations)
	if err != nil {
		t.Fatal(err)
	}
	if len(network.Roads) != 1 || len(network.Roads[0].Lanes) != 3 {
		t.Fatalf("unexpected lanes: %+v", network.Roads)
	}
}

func TestBuildRejectsDuplicateDistrictCode(t *testing.T) {
	_, err := Build([]District{{ID: "a", Code: "D1"}, {ID: "b", Code: "D1"}}, nil, nil)
	if err == nil {
		t.Fatal("expected duplicate code error")
	}
}

func TestBuildRejectsInvalidRelations(t *testing.T) {
	districts := []District{{ID: "a", Code: "A"}, {ID: "b", Code: "B"}}
	membership := map[string]string{"one": "a", "two": "b"}
	tests := []struct {
		name      string
		relations []Relation
	}{
		{"duplicate id", []Relation{
			{ID: "same", FromSubject: "one", ToSubject: "two", Family: "call", Execution: "sync"},
			{ID: "same", FromSubject: "two", ToSubject: "one", Family: "call", Execution: "sync"},
		}},
		{"missing family", []Relation{{ID: "r", FromSubject: "one", ToSubject: "two", Execution: "sync"}}},
		{"negative count", []Relation{{ID: "r", FromSubject: "one", ToSubject: "two", Family: "call", Execution: "sync", Count: -1}}},
		{"nan strength", []Relation{{ID: "r", FromSubject: "one", ToSubject: "two", Family: "call", Execution: "sync", Strength: math.NaN()}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := Build(districts, membership, test.relations); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestBuildRejectsRoadIDSlugCollision(t *testing.T) {
	districts := []District{
		{ID: "a", Code: "A+B"}, {ID: "b", Code: "C+D"},
		{ID: "c", Code: "A-B"}, {ID: "d", Code: "C-D"},
	}
	membership := map[string]string{"one": "a", "two": "b", "three": "c", "four": "d"}
	relations := []Relation{
		{ID: "r1", FromSubject: "one", ToSubject: "two", Family: "call", Execution: "sync"},
		{ID: "r2", FromSubject: "three", ToSubject: "four", Family: "call", Execution: "sync"},
	}
	if _, err := Build(districts, membership, relations); err == nil {
		t.Fatal("expected road id collision")
	}
}
