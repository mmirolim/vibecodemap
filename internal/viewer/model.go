// Package viewer composes validated VibeCodeMap documents into a renderer-ready
// view model and a standalone browser document.
package viewer

import "time"

const ViewSchema = "vibecodemap.view/0.1"

type ComposeOptions struct {
	Profile string
}

type ViewModel struct {
	Schema      string         `json:"schema"`
	GeneratedAt time.Time      `json:"generated_at"`
	Project     ProjectView    `json:"project"`
	Profile     ProfileView    `json:"profile"`
	Stats       StatsView      `json:"stats"`
	Cities      []CityView     `json:"cities"`
	Districts   []DistrictView `json:"districts"`
	Nodes       []NodeView     `json:"nodes"`
	Relations   []RelationView `json:"relations"`
	Roads       []RoadView     `json:"roads"`
	Boundaries  []BoundaryView `json:"boundaries,omitempty"`
	Security    []SecurityView `json:"security,omitempty"`
	Warnings    []string       `json:"warnings,omitempty"`
}

type ProjectView struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Summary         string `json:"summary,omitempty"`
	RepositoryRoot  string `json:"repository_root"`
	ProjectManifest string `json:"project_manifest"`
	StructuralModel string `json:"structural_model"`
	QualityModel    string `json:"quality_model,omitempty"`
}

type ProfileView struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Decomposition string     `json:"decomposition"`
	Bands         []BandSpec `json:"bands"`
}

type BandSpec struct {
	ID            string    `json:"id" yaml:"id"`
	Label         string    `json:"label" yaml:"label"`
	Metric        string    `json:"metric" yaml:"metric"`
	Order         int       `json:"order" yaml:"order"`
	Direction     string    `json:"direction" yaml:"direction"`
	Normalization string    `json:"normalization" yaml:"normalization"`
	Thresholds    []float64 `json:"thresholds,omitempty" yaml:"thresholds"`
	Unknown       string    `json:"unknown,omitempty" yaml:"unknown"`
}

type StatsView struct {
	Systems     int `json:"systems"`
	Districts   int `json:"districts"`
	Nodes       int `json:"nodes"`
	Relations   int `json:"relations"`
	Roads       int `json:"roads"`
	Unrouted    int `json:"unrouted_relations"`
	SourceFiles int `json:"source_files"`
}

type CityView struct {
	ID      string   `json:"id"`
	Code    string   `json:"code"`
	Name    string   `json:"name"`
	Summary string   `json:"summary,omitempty"`
	Nodes   []string `json:"nodes"`
}

type DistrictView struct {
	ID       string   `json:"id"`
	SourceID string   `json:"source_id"`
	CityID   string   `json:"city_id"`
	Code     string   `json:"code"`
	Name     string   `json:"name"`
	Role     string   `json:"role,omitempty"`
	Summary  string   `json:"summary,omitempty"`
	Nodes    []string `json:"nodes"`
}

type NodeView struct {
	ID             string            `json:"id"`
	CityID         string            `json:"city_id"`
	DistrictID     string            `json:"district_id"`
	Kind           string            `json:"kind"`
	Name           string            `json:"name"`
	Summary        string            `json:"summary,omitempty"`
	Parent         string            `json:"parent,omitempty"`
	Runtime        string            `json:"runtime,omitempty"`
	Layer          string            `json:"layer,omitempty"`
	State          string            `json:"state,omitempty"`
	Implementation string            `json:"implementation,omitempty"`
	Confidence     string            `json:"confidence,omitempty"`
	Source         *SourceView       `json:"source,omitempty"`
	Lines          float64           `json:"lines,omitempty"`
	Height         float64           `json:"height"`
	Footprint      float64           `json:"footprint"`
	DirectMutation bool              `json:"direct_mutation"`
	Bands          []BandValue       `json:"bands"`
	Measurements   map[string]Metric `json:"measurements,omitempty"`
}

type SourceView struct {
	Path     string `json:"path"`
	Absolute string `json:"absolute"`
	Symbol   string `json:"symbol,omitempty"`
	Line     int    `json:"line,omitempty"`
}

type Metric struct {
	Value  float64 `json:"value"`
	Status string  `json:"status"`
	Unit   string  `json:"unit,omitempty"`
	Name   string  `json:"name,omitempty"`
}

type BandValue struct {
	ID         string   `json:"id"`
	Label      string   `json:"label"`
	Metric     string   `json:"metric"`
	Raw        *float64 `json:"raw,omitempty"`
	Status     string   `json:"status"`
	Normalized *float64 `json:"normalized,omitempty"`
}

type RelationView struct {
	ID        string      `json:"id"`
	From      string      `json:"from"`
	To        string      `json:"to"`
	Kind      string      `json:"kind"`
	Family    string      `json:"family"`
	Execution string      `json:"execution"`
	Protocol  string      `json:"protocol,omitempty"`
	Summary   string      `json:"summary,omitempty"`
	Mutates   bool        `json:"mutates"`
	Source    *SourceView `json:"source,omitempty"`
}

type RoadView struct {
	ID       string     `json:"id"`
	A        RoadEnd    `json:"a"`
	B        RoadEnd    `json:"b"`
	Lanes    []LaneView `json:"lanes"`
	Count    int        `json:"count"`
	Strength float64    `json:"strength"`
}

type RoadEnd struct {
	DistrictID string `json:"district_id"`
	Code       string `json:"code"`
	Opposite   string `json:"opposite"`
}

type LaneView struct {
	From      string   `json:"from"`
	To        string   `json:"to"`
	Family    string   `json:"family"`
	Execution string   `json:"execution"`
	Count     int      `json:"count"`
	Strength  float64  `json:"strength"`
	Relations []string `json:"relations"`
}

type BoundaryView struct {
	ID             string   `json:"id"`
	Subject        string   `json:"subject"`
	Direction      string   `json:"direction"`
	Transport      string   `json:"transport,omitempty"`
	Payloads       []string `json:"payloads,omitempty"`
	TrustFrom      string   `json:"trust_from,omitempty"`
	TrustTo        string   `json:"trust_to,omitempty"`
	Authentication string   `json:"authentication,omitempty"`
	Summary        string   `json:"summary,omitempty"`
}

type SecurityView struct {
	ID         string   `json:"id"`
	Subjects   []string `json:"subjects"`
	Category   string   `json:"category"`
	Status     string   `json:"status"`
	Severity   string   `json:"severity"`
	Confidence string   `json:"confidence"`
	Summary    string   `json:"summary"`
	Impact     string   `json:"impact,omitempty"`
}

type sourceRef struct {
	Artifact string `yaml:"artifact"`
	Symbol   string `yaml:"symbol"`
	Lines    []int  `yaml:"lines"`
	Supports string `yaml:"supports"`
}

type provenance struct {
	Method     string `yaml:"method"`
	Confidence string `yaml:"confidence"`
	Rationale  string `yaml:"rationale"`
}

type projectDocument struct {
	Version string `yaml:"vcm_project"`
	Project struct {
		ID         string `yaml:"id"`
		Name       string `yaml:"name"`
		Repository struct {
			Root string `yaml:"root"`
		} `yaml:"repository"`
		Inputs struct {
			StructuralModel string `yaml:"structural_model"`
			QualityModel    string `yaml:"quality_model"`
		} `yaml:"inputs"`
	} `yaml:"project"`
	Decompositions []decomposition `yaml:"decompositions"`
	Corrections    []struct {
		ID     string `yaml:"id"`
		Target struct {
			Entity   string   `yaml:"entity"`
			Selector selector `yaml:"selector"`
		} `yaml:"target"`
		Operations []struct {
			Action string `yaml:"action"`
			Path   string `yaml:"path"`
			Value  any    `yaml:"value"`
		} `yaml:"operations"`
	} `yaml:"corrections"`
	Boundaries []struct {
		ID             string   `yaml:"id"`
		Subject        string   `yaml:"subject"`
		Direction      string   `yaml:"direction"`
		Transport      string   `yaml:"transport"`
		Payloads       []string `yaml:"payloads"`
		TrustFrom      string   `yaml:"trust_from"`
		TrustTo        string   `yaml:"trust_to"`
		Authentication string   `yaml:"authentication"`
		Summary        string   `yaml:"summary"`
	} `yaml:"boundaries"`
	SecurityReviews []struct {
		ID         string     `yaml:"id"`
		Subjects   []string   `yaml:"subjects"`
		Category   string     `yaml:"category"`
		Status     string     `yaml:"status"`
		Severity   string     `yaml:"severity"`
		Summary    string     `yaml:"summary"`
		Impact     string     `yaml:"impact"`
		Provenance provenance `yaml:"provenance"`
	} `yaml:"security_reviews"`
	RenderProfiles []struct {
		ID            string `yaml:"id"`
		Kind          string `yaml:"kind"`
		Decomposition string `yaml:"decomposition"`
		Encodings     struct {
			Bands []BandSpec `yaml:"bands"`
		} `yaml:"encodings"`
	} `yaml:"render_profiles"`
}

type decomposition struct {
	ID        string     `yaml:"id"`
	Name      string     `yaml:"name"`
	Kind      string     `yaml:"kind"`
	Districts []district `yaml:"districts"`
}

type district struct {
	ID      string   `yaml:"id"`
	Code    string   `yaml:"code"`
	Name    string   `yaml:"name"`
	Role    string   `yaml:"role"`
	Summary string   `yaml:"summary"`
	Members selector `yaml:"members"`
}

type selector struct {
	ElementIDs []string       `yaml:"element_ids"`
	PathGlobs  []string       `yaml:"path_globs"`
	Facets     map[string]any `yaml:"facets"`
	Exclude    []string       `yaml:"exclude"`
}

type structuralDocument struct {
	Version string `yaml:"vcm"`
	Model   struct {
		ID         string `yaml:"id"`
		Name       string `yaml:"name"`
		Summary    string `yaml:"summary"`
		Repository struct {
			Root string `yaml:"root"`
		} `yaml:"repository"`
	} `yaml:"model"`
	Artifacts []artifact `yaml:"artifacts"`
	Elements  []element  `yaml:"elements"`
	Relations []relation `yaml:"relations"`
	Findings  []finding  `yaml:"findings"`
}

type finding struct {
	ID       string      `yaml:"id"`
	Kind     string      `yaml:"kind"`
	Severity string      `yaml:"severity"`
	Summary  string      `yaml:"summary"`
	Subjects []string    `yaml:"subjects"`
	Evidence []sourceRef `yaml:"evidence"`
}

type artifact struct {
	ID        string             `yaml:"id"`
	Path      string             `yaml:"path"`
	Kind      string             `yaml:"kind"`
	Language  string             `yaml:"language"`
	Summary   string             `yaml:"summary"`
	Metrics   map[string]float64 `yaml:"metrics"`
	Generated provenance         `yaml:"generated"`
}

type element struct {
	ID               string         `yaml:"id"`
	Kind             string         `yaml:"kind"`
	Name             string         `yaml:"name"`
	Parent           string         `yaml:"parent"`
	Summary          string         `yaml:"summary"`
	Responsibilities []string       `yaml:"responsibilities"`
	Facets           map[string]any `yaml:"facets"`
	Reality          struct {
		Implementation string `yaml:"implementation"`
		Runtime        string `yaml:"runtime"`
	} `yaml:"reality"`
	Execution  execution   `yaml:"execution"`
	SourceRefs []sourceRef `yaml:"source_refs"`
	Evidence   []sourceRef `yaml:"evidence"`
	Generated  provenance  `yaml:"generated"`
}

type execution struct {
	Trigger  string `yaml:"trigger"`
	Style    string `yaml:"style"`
	Blocking string `yaml:"blocking"`
	Delivery string `yaml:"delivery"`
}

type relation struct {
	ID        string    `yaml:"id"`
	From      string    `yaml:"from"`
	To        string    `yaml:"to"`
	Kind      string    `yaml:"kind"`
	Summary   string    `yaml:"summary"`
	Protocol  string    `yaml:"protocol"`
	Execution execution `yaml:"execution"`
	Effect    struct {
		Domain       string `yaml:"domain"`
		Operation    string `yaml:"operation"`
		MutatesState string `yaml:"mutates_state"`
	} `yaml:"effect"`
	Evidence  []sourceRef `yaml:"evidence"`
	Generated provenance  `yaml:"generated"`
}

type qualityDocument struct {
	Version       string             `yaml:"vcm_quality"`
	MetricCatalog []metricDefinition `yaml:"metric_catalog"`
	Measurements  []measurement      `yaml:"measurements"`
}

type metricDefinition struct {
	ID        string `yaml:"id"`
	Name      string `yaml:"name"`
	Unit      string `yaml:"unit"`
	Direction string `yaml:"direction"`
}

type measurement struct {
	ID      string  `yaml:"id"`
	Subject string  `yaml:"subject"`
	Metric  string  `yaml:"metric"`
	Status  string  `yaml:"status"`
	Value   float64 `yaml:"value"`
}
