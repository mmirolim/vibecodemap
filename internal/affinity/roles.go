package affinity

import (
	"math"
	"sort"
)

// RoleMetrics are measurements used to classify private helpers, cluster hubs,
// connectors, and shared utilities. Thresholds remain profile configuration.
type RoleMetrics struct {
	Subject                  string
	Cluster                  string
	IncomingWeight           float64
	OutgoingWeight           float64
	WithinClusterWeight      float64
	ParticipationCoefficient float64
	WithinClusterDegreeZ     float64
}

type roleAccumulator struct {
	incoming  float64
	outgoing  float64
	byCluster map[string]float64
}

// ComputeRoleMetrics calculates direction-aware degree plus the functional
// cartography measures used to distinguish internal hubs from connectors.
func ComputeRoleMetrics(subjects []string, edges []Edge, membership map[string]string) []RoleMetrics {
	accumulators := make(map[string]*roleAccumulator, len(subjects))
	for _, subject := range subjects {
		accumulators[subject] = &roleAccumulator{byCluster: make(map[string]float64)}
	}
	for _, edge := range edges {
		from := ensureAccumulator(accumulators, edge.From)
		to := ensureAccumulator(accumulators, edge.To)
		from.outgoing += edge.Weight
		to.incoming += edge.Weight
		from.byCluster[membership[edge.To]] += edge.Weight
		to.byCluster[membership[edge.From]] += edge.Weight
	}

	withinByCluster := make(map[string][]float64)
	withinBySubject := make(map[string]float64, len(accumulators))
	for subject, accumulator := range accumulators {
		cluster := membership[subject]
		within := accumulator.byCluster[cluster]
		withinBySubject[subject] = within
		withinByCluster[cluster] = append(withinByCluster[cluster], within)
	}

	type distribution struct{ mean, stddev float64 }
	distributions := make(map[string]distribution, len(withinByCluster))
	for cluster, values := range withinByCluster {
		mean := 0.0
		for _, value := range values {
			mean += value
		}
		mean /= float64(len(values))
		variance := 0.0
		for _, value := range values {
			delta := value - mean
			variance += delta * delta
		}
		variance /= float64(len(values))
		distributions[cluster] = distribution{mean: mean, stddev: math.Sqrt(variance)}
	}

	result := make([]RoleMetrics, 0, len(accumulators))
	for subject, accumulator := range accumulators {
		total := accumulator.incoming + accumulator.outgoing
		participation := 0.0
		if total > 0 {
			sumSquares := 0.0
			for _, value := range accumulator.byCluster {
				ratio := value / total
				sumSquares += ratio * ratio
			}
			participation = 1 - sumSquares
		}
		cluster := membership[subject]
		distribution := distributions[cluster]
		z := 0.0
		if distribution.stddev > 0 {
			z = (withinBySubject[subject] - distribution.mean) / distribution.stddev
		}
		result = append(result, RoleMetrics{
			Subject:                  subject,
			Cluster:                  cluster,
			IncomingWeight:           accumulator.incoming,
			OutgoingWeight:           accumulator.outgoing,
			WithinClusterWeight:      withinBySubject[subject],
			ParticipationCoefficient: participation,
			WithinClusterDegreeZ:     z,
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Subject < result[j].Subject })
	return result
}

func ensureAccumulator(accumulators map[string]*roleAccumulator, subject string) *roleAccumulator {
	accumulator := accumulators[subject]
	if accumulator == nil {
		accumulator = &roleAccumulator{byCluster: make(map[string]float64)}
		accumulators[subject] = accumulator
	}
	return accumulator
}
