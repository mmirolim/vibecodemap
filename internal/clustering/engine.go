// Package clustering defines the replaceable boundary between VibeCodeMap's
// explainable affinity graph and concrete community-detection engines.
package clustering

import (
	"context"

	"github.com/mmirolim/vibecodemap/internal/affinity"
)

// Config records every parameter required to reproduce a cluster run.
type Config struct {
	Algorithm      string
	Implementation string
	Version        string
	Objective      string
	Resolution     float64
	Seed           int64
}

// Membership is a hard primary assignment plus algorithm-specific diagnostics.
// Connector and utility roles are calculated separately from this assignment.
type Membership struct {
	Subject   string
	Cluster   string
	Strength  float64
	Stability float64
}

// Partition is a reproducible engine result; it is not declared architecture.
type Partition struct {
	Config      Config
	Objective   float64
	Memberships []Membership
}

// Engine allows Leiden, Infomap, Bunch, or experimental implementations to be
// compared without changing the canonical affinity evidence.
type Engine interface {
	Cluster(context.Context, []affinity.Edge, Config) (Partition, error)
}
