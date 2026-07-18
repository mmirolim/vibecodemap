// Package affinity builds an explainable, typed graph for software clustering.
// It does not decide that a computed partition is the intended architecture.
package affinity

import (
	"fmt"
	"math"
	"sort"
)

// Signal names one independently normalized source of relationship evidence.
type Signal string

const (
	SignalCall              Signal = "direct_call"
	SignalReferencedImport  Signal = "referenced_import"
	SignalSharedFlow        Signal = "shared_flow"
	SignalRuntimeTransition Signal = "runtime_transition"
	SignalSharedState       Signal = "shared_state"
	SignalCoChange          Signal = "co_change"
	SignalSemantic          Signal = "semantic_similarity"
)

// Normalization controls scaling within one signal layer.
type Normalization string

const (
	NormalizeNone   Normalization = "none"
	NormalizeMax    Normalization = "within_layer_max"
	NormalizeP95Cap Normalization = "within_layer_p95_cap"
)

// EvidenceEdge is a measured or evidence-linked directed relationship.
type EvidenceEdge struct {
	ID         string
	From       string
	To         string
	Signal     Signal
	Strength   float64
	Confidence float64
}

// LayerProfile declares how one signal contributes to an affinity model.
type LayerProfile struct {
	Signal        Signal
	Weight        float64
	Normalization Normalization
	AttenuateHubs bool
}

// Profile is versioned experiment configuration, not a universal quality rule.
type Profile struct {
	ID                 string
	Layers             []LayerProfile
	DedicationExponent float64
}

// Contribution explains one signal's contribution to a combined affinity edge.
type Contribution struct {
	Signal        Signal
	RawStrength   float64
	Normalized    float64
	LayerWeight   float64
	Dedication    float64
	Confidence    float64
	WeightedValue float64
	EvidenceIDs   []string
}

// Edge is the combined directed affinity between two software subjects.
type Edge struct {
	From          string
	To            string
	Weight        float64
	Contributions []Contribution
}

type groupedKey struct {
	from   string
	to     string
	signal Signal
}

type groupedEvidence struct {
	strength         float64
	confidenceMass   float64
	confidenceWeight float64
	evidenceIDs      []string
}

type pairKey struct {
	from string
	to   string
}

// Build constructs a directed affinity graph while preserving every factor.
// Hub attenuation uses SArF's simple Dedication score 1/fan-in(target) within
// each configured signal layer. Raw topology remains outside this derived graph.
func Build(evidence []EvidenceEdge, profile Profile) ([]Edge, error) {
	layers, err := validateProfile(profile)
	if err != nil {
		return nil, err
	}

	grouped := make(map[groupedKey]*groupedEvidence)
	for index, item := range evidence {
		if _, enabled := layers[item.Signal]; !enabled {
			continue
		}
		if item.ID == "" {
			return nil, fmt.Errorf("evidence edge %d has no ID", index)
		}
		if item.From == "" || item.To == "" {
			return nil, fmt.Errorf("evidence edge %q has an empty endpoint", item.ID)
		}
		if item.From == item.To {
			return nil, fmt.Errorf("evidence edge %q is a self edge", item.ID)
		}
		if math.IsNaN(item.Strength) || math.IsInf(item.Strength, 0) || item.Strength < 0 {
			return nil, fmt.Errorf("evidence edge %q has invalid strength", item.ID)
		}
		if math.IsNaN(item.Confidence) || math.IsInf(item.Confidence, 0) || item.Confidence < 0 || item.Confidence > 1 {
			return nil, fmt.Errorf("evidence edge %q has confidence outside [0,1]", item.ID)
		}
		if item.Strength == 0 {
			continue
		}

		key := groupedKey{from: item.From, to: item.To, signal: item.Signal}
		group := grouped[key]
		if group == nil {
			group = &groupedEvidence{}
			grouped[key] = group
		}
		group.strength += item.Strength
		group.confidenceMass += item.Strength * item.Confidence
		group.confidenceWeight += item.Strength
		group.evidenceIDs = append(group.evidenceIDs, item.ID)
	}

	scales := layerScales(grouped, layers)
	fanIn := layerFanIn(grouped)
	pairs := make(map[pairKey][]Contribution)

	keys := make([]groupedKey, 0, len(grouped))
	for key := range grouped {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].from != keys[j].from {
			return keys[i].from < keys[j].from
		}
		if keys[i].to != keys[j].to {
			return keys[i].to < keys[j].to
		}
		return keys[i].signal < keys[j].signal
	})

	for _, key := range keys {
		group := grouped[key]
		layer := layers[key.signal]
		normalized := normalize(group.strength, scales[key.signal], layer.Normalization)
		dedication := 1.0
		if layer.AttenuateHubs {
			count := fanIn[key.signal][key.to]
			if count > 0 {
				dedication = math.Pow(1/float64(count), profile.DedicationExponent)
			}
		}
		confidence := group.confidenceMass / group.confidenceWeight
		weighted := normalized * layer.Weight * dedication * confidence
		ids := append([]string(nil), group.evidenceIDs...)
		sort.Strings(ids)
		pair := pairKey{from: key.from, to: key.to}
		pairs[pair] = append(pairs[pair], Contribution{
			Signal:        key.signal,
			RawStrength:   group.strength,
			Normalized:    normalized,
			LayerWeight:   layer.Weight,
			Dedication:    dedication,
			Confidence:    confidence,
			WeightedValue: weighted,
			EvidenceIDs:   ids,
		})
	}

	result := make([]Edge, 0, len(pairs))
	for pair, contributions := range pairs {
		weight := 0.0
		for _, contribution := range contributions {
			weight += contribution.WeightedValue
		}
		result = append(result, Edge{
			From:          pair.from,
			To:            pair.to,
			Weight:        weight,
			Contributions: contributions,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].From != result[j].From {
			return result[i].From < result[j].From
		}
		return result[i].To < result[j].To
	})
	return result, nil
}

func validateProfile(profile Profile) (map[Signal]LayerProfile, error) {
	if profile.ID == "" {
		return nil, fmt.Errorf("affinity profile has no ID")
	}
	if len(profile.Layers) == 0 {
		return nil, fmt.Errorf("affinity profile %q has no layers", profile.ID)
	}
	if math.IsNaN(profile.DedicationExponent) || math.IsInf(profile.DedicationExponent, 0) || profile.DedicationExponent < 0 {
		return nil, fmt.Errorf("affinity profile %q has an invalid dedication exponent", profile.ID)
	}
	layers := make(map[Signal]LayerProfile, len(profile.Layers))
	for _, layer := range profile.Layers {
		if layer.Signal == "" {
			return nil, fmt.Errorf("affinity profile %q has an unnamed layer", profile.ID)
		}
		if _, duplicate := layers[layer.Signal]; duplicate {
			return nil, fmt.Errorf("affinity profile %q repeats layer %q", profile.ID, layer.Signal)
		}
		if math.IsNaN(layer.Weight) || math.IsInf(layer.Weight, 0) || layer.Weight < 0 {
			return nil, fmt.Errorf("affinity profile %q has an invalid weight for %q", profile.ID, layer.Signal)
		}
		switch layer.Normalization {
		case NormalizeNone, NormalizeMax, NormalizeP95Cap:
		default:
			return nil, fmt.Errorf("affinity profile %q has unsupported normalization %q", profile.ID, layer.Normalization)
		}
		layers[layer.Signal] = layer
	}
	return layers, nil
}

func layerScales(grouped map[groupedKey]*groupedEvidence, layers map[Signal]LayerProfile) map[Signal]float64 {
	values := make(map[Signal][]float64)
	for key, group := range grouped {
		values[key.signal] = append(values[key.signal], group.strength)
	}
	scales := make(map[Signal]float64, len(layers))
	for signal, layer := range layers {
		items := values[signal]
		if len(items) == 0 || layer.Normalization == NormalizeNone {
			scales[signal] = 1
			continue
		}
		sort.Float64s(items)
		if layer.Normalization == NormalizeMax {
			scales[signal] = items[len(items)-1]
			continue
		}
		index := int(math.Ceil(float64(len(items))*0.95)) - 1
		if index < 0 {
			index = 0
		}
		scales[signal] = items[index]
	}
	return scales
}

func layerFanIn(grouped map[groupedKey]*groupedEvidence) map[Signal]map[string]int {
	sources := make(map[Signal]map[string]map[string]struct{})
	for key := range grouped {
		if sources[key.signal] == nil {
			sources[key.signal] = make(map[string]map[string]struct{})
		}
		if sources[key.signal][key.to] == nil {
			sources[key.signal][key.to] = make(map[string]struct{})
		}
		sources[key.signal][key.to][key.from] = struct{}{}
	}
	result := make(map[Signal]map[string]int, len(sources))
	for signal, targets := range sources {
		result[signal] = make(map[string]int, len(targets))
		for target, incoming := range targets {
			result[signal][target] = len(incoming)
		}
	}
	return result
}

func normalize(value, scale float64, mode Normalization) float64 {
	if mode == NormalizeNone || scale <= 0 {
		return value
	}
	return math.Min(value/scale, 1)
}
