package viewer

import (
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/mmirolim/vibecodemap/internal/roads"
	"github.com/mmirolim/vibecodemap/internal/scoping"
)

type nodeDraft struct {
	element      element
	cityID       string
	baseDistrict district
	sources      []sourceRef
	artifactIDs  []string
	lines        float64
}

// Compose validates the project and its referenced structural/quality models,
// then derives one renderer-neutral view model. One structural model may hold
// several systems; each system becomes a city in the generated workspace.
func Compose(projectPath string, options ComposeOptions) (ViewModel, error) {
	documents, err := loadDocuments(projectPath)
	if err != nil {
		return ViewModel{}, err
	}
	profile, decomposition, err := selectProfile(documents.project, options.Profile)
	if err != nil {
		return ViewModel{}, err
	}

	artifacts := make(map[string]artifact, len(documents.structural.Artifacts))
	for _, item := range documents.structural.Artifacts {
		artifacts[item.ID] = item
	}
	elements := make(map[string]element, len(documents.structural.Elements))
	for _, item := range documents.structural.Elements {
		elements[item.ID] = item
	}

	systems := systemElements(documents.structural)
	if len(systems) == 0 {
		synthetic := element{
			ID: "system." + slug(documents.project.Project.ID), Kind: "system",
			Name: documents.project.Project.Name, Summary: documents.structural.Model.Summary,
		}
		systems = append(systems, synthetic)
		elements[synthetic.ID] = synthetic
	}
	defaultSystem := systems[0].ID
	cityCodes := make(map[string]string, len(systems))
	for index, system := range systems {
		cityCodes[system.ID] = fmt.Sprintf("C%d", index+1)
	}

	explicit := explicitMembers(decomposition)
	important := importantElements(documents.structural, documents.project)
	visible := make(map[string]bool)
	for _, item := range documents.structural.Elements {
		visible[item.ID] = shouldRenderElement(item, explicit, important)
	}

	var drafts []nodeDraft
	for _, item := range documents.structural.Elements {
		if !visible[item.ID] {
			continue
		}
		cityID := owningSystem(item.ID, elements)
		if cityID == "" {
			cityID = defaultSystem
		}
		sources := append([]sourceRef(nil), item.SourceRefs...)
		if len(sources) == 0 {
			sources = append(sources, item.Evidence...)
		}
		artifactIDs := uniqueArtifactIDs(sources)
		lineCount := 0.0
		for _, artifactID := range artifactIDs {
			lineCount += artifacts[artifactID].Metrics["lines"]
		}
		base, matched, matchErr := matchDistrict(item, sources, artifacts, elements, decomposition.Districts)
		if matchErr != nil {
			return ViewModel{}, matchErr
		}
		if !matched {
			base = district{ID: "district.unmapped", Code: "U", Name: "Unmapped", Role: "review_candidate", Summary: "Elements not assigned by the selected decomposition."}
		}
		drafts = append(drafts, nodeDraft{
			element: item, cityID: cityID, baseDistrict: base,
			sources: sources, artifactIDs: artifactIDs, lines: lineCount,
		})
	}
	if len(drafts) == 0 {
		return ViewModel{}, fmt.Errorf("selected structural model has no renderable elements")
	}

	activeCities := make(map[string]bool)
	for _, draft := range drafts {
		activeCities[draft.cityID] = true
	}
	multipleCities := len(activeCities) > 1
	districtKeys := make(map[string]string)
	districtViews := make(map[string]*DistrictView)
	baseCities := make(map[string]map[string]bool)
	for _, draft := range drafts {
		key := draft.baseDistrict.ID + "\x00" + draft.cityID
		viewID := draft.baseDistrict.ID
		code := draft.baseDistrict.Code
		if multipleCities {
			viewID += "." + draft.cityID
			code = cityCodes[draft.cityID] + "." + code
		}
		districtKeys[key] = viewID
		if districtViews[viewID] == nil {
			districtViews[viewID] = &DistrictView{
				ID: viewID, SourceID: draft.baseDistrict.ID, CityID: draft.cityID,
				Code: code, Name: draft.baseDistrict.Name, Role: draft.baseDistrict.Role,
				Summary: draft.baseDistrict.Summary,
			}
		}
		if baseCities[draft.baseDistrict.ID] == nil {
			baseCities[draft.baseDistrict.ID] = make(map[string]bool)
		}
		baseCities[draft.baseDistrict.ID][draft.cityID] = true
	}

	metricDefinitions := make(map[string]metricDefinition)
	for _, definition := range documents.quality.MetricCatalog {
		metricDefinitions[definition.ID] = definition
	}
	measurements := indexMeasurements(documents.quality.Measurements)

	nodes := make([]NodeView, 0, len(drafts))
	nodeIndex := make(map[string]int, len(drafts))
	for _, draft := range drafts {
		districtID := districtKeys[draft.baseDistrict.ID+"\x00"+draft.cityID]
		metrics := collectMetrics(draft, measurements, metricDefinitions)
		source := sourceView(draft.sources, artifacts, documents.repositoryRoot)
		footprint := 1.05
		if draft.lines > 0 {
			footprint = 0.8 + math.Min(2.5, math.Log1p(draft.lines)/4.8)
		}
		heightBasis := draft.lines / 250
		if value, exists := metrics["complexity.cyclomatic.max"]; exists {
			heightBasis = value.Value
		}
		height := 1.15 + math.Min(7.5, math.Log1p(math.Max(0, heightBasis))*0.88)
		node := NodeView{
			ID: draft.element.ID, CityID: draft.cityID, DistrictID: districtID,
			Kind: draft.element.Kind, Name: draft.element.Name, Summary: draft.element.Summary,
			Parent: draft.element.Parent, Runtime: facetString(draft.element.Facets, "runtime"),
			Layer: facetString(draft.element.Facets, "layer"), State: facetString(draft.element.Facets, "state"),
			Implementation: draft.element.Reality.Implementation, Confidence: draft.element.Generated.Confidence,
			Source: source, Lines: draft.lines, Height: height, Footprint: footprint,
			Measurements: metrics,
		}
		nodeIndex[node.ID] = len(nodes)
		nodes = append(nodes, node)
		districtViews[districtID].Nodes = append(districtViews[districtID].Nodes, node.ID)
	}

	relations, mutationCounts, degree, crossDegree, relationWarnings := composeRelations(
		documents.structural.Relations, elements, visible, nodeIndex, nodes, artifacts, documents.repositoryRoot,
	)
	correctionWarnings, mutationOverrides, err := applyVisualCorrections(documents.project, drafts, artifacts)
	if err != nil {
		return ViewModel{}, err
	}
	for index := range nodes {
		nodes[index].DirectMutation = nodeDirectMutation(nodes[index], mutationCounts, mutationOverrides)
		nodes[index].Bands = rawBands(nodes[index], profile.Encodings.Bands, degree, crossDegree, mutationCounts, mutationOverrides)
	}
	normalizeBands(nodes, profile.Encodings.Bands)

	districtList := make([]DistrictView, 0, len(districtViews))
	for _, district := range districtViews {
		sort.Strings(district.Nodes)
		districtList = append(districtList, *district)
	}
	sort.Slice(districtList, func(i, j int) bool {
		if districtList[i].CityID != districtList[j].CityID {
			return districtList[i].CityID < districtList[j].CityID
		}
		return districtList[i].Code < districtList[j].Code
	})

	membership := make(map[string]string, len(nodes))
	for _, node := range nodes {
		membership[node.ID] = node.DistrictID
	}
	roadDistricts := make([]roads.District, 0, len(districtList))
	for _, district := range districtList {
		roadDistricts = append(roadDistricts, roads.District{ID: district.ID, Code: district.Code, Name: district.Name})
	}
	roadRelations := make([]roads.Relation, 0, len(relations))
	for _, relation := range relations {
		roadRelations = append(roadRelations, roads.Relation{
			ID: relation.ID, FromSubject: relation.From, ToSubject: relation.To,
			Family: relation.Family, Execution: relation.Execution, Count: 1, Strength: 1,
		})
	}
	network, err := roads.Build(roadDistricts, membership, roadRelations)
	if err != nil {
		return ViewModel{}, fmt.Errorf("build district roads: %w", err)
	}
	viewRoads := make([]RoadView, 0, len(network.Roads))
	for _, road := range network.Roads {
		item := RoadView{
			ID:    road.ID,
			A:     RoadEnd{DistrictID: road.A.DistrictID, Code: road.A.DistrictCode, Opposite: road.A.OppositeCode},
			B:     RoadEnd{DistrictID: road.B.DistrictID, Code: road.B.DistrictCode, Opposite: road.B.OppositeCode},
			Count: road.TotalCount, Strength: road.Strength,
		}
		for _, lane := range road.Lanes {
			item.Lanes = append(item.Lanes, LaneView{
				From: lane.FromDistrict, To: lane.ToDistrict, Family: lane.Family,
				Execution: lane.Execution, Count: lane.Count, Strength: lane.Strength,
				Relations: lane.RelationIDs,
			})
		}
		viewRoads = append(viewRoads, item)
	}

	cityViews := make([]CityView, 0, len(systems))
	for _, system := range systems {
		if !activeCities[system.ID] {
			continue
		}
		city := CityView{ID: system.ID, Code: cityCodes[system.ID], Name: system.Name, Summary: system.Summary}
		for _, node := range nodes {
			if node.CityID == system.ID {
				city.Nodes = append(city.Nodes, node.ID)
			}
		}
		sort.Strings(city.Nodes)
		cityViews = append(cityViews, city)
	}

	warnings := append([]string(nil), documents.warnings...)
	warnings = append(warnings, relationWarnings...)
	warnings = append(warnings, correctionWarnings...)
	for districtID, cities := range baseCities {
		if districtID != "district.unmapped" && len(cities) > 1 {
			warnings = append(warnings, fmt.Sprintf("district %s spans %d systems and was split into city-local districts", districtID, len(cities)))
		}
	}
	unmapped := 0
	for _, node := range nodes {
		if strings.HasPrefix(node.DistrictID, "district.unmapped") {
			unmapped++
		}
	}
	if unmapped > 0 {
		warnings = append(warnings, fmt.Sprintf("%d renderable elements are not assigned by decomposition %s", unmapped, decomposition.ID))
	}
	if documents.qualityPath == "" {
		warnings = append(warnings, "no quality model is configured; quality bands remain unknown unless derived from structural relations")
	}
	sort.Strings(warnings)

	boundaries := composeBoundaries(documents.project, nodeIndex)
	security := composeSecurity(documents.project, nodeIndex)
	view := ViewModel{
		Schema: ViewSchema, GeneratedAt: time.Now().UTC(),
		Project: ProjectView{
			ID: documents.project.Project.ID, Name: documents.project.Project.Name,
			Summary: documents.structural.Model.Summary, RepositoryRoot: documents.repositoryRoot,
			ProjectManifest: documents.projectPath, StructuralModel: documents.structuralPath,
			QualityModel: documents.qualityPath,
		},
		Profile: ProfileView{
			ID: profile.ID, Name: profile.Kind, Decomposition: decomposition.ID,
			Bands: append([]BandSpec(nil), profile.Encodings.Bands...),
		},
		Cities: cityViews, Districts: districtList, Nodes: nodes, Relations: relations,
		Roads: viewRoads, Boundaries: boundaries, Security: security, Warnings: warnings,
	}
	view.Stats = StatsView{
		Systems: len(cityViews), Districts: len(districtList), Nodes: len(nodes),
		Relations: len(relations), Roads: len(viewRoads), Unrouted: len(network.Unrouted),
		SourceFiles: len(documents.structural.Artifacts),
	}
	return view, nil
}

func selectProfile(project projectDocument, requested string) (struct {
	ID            string `yaml:"id"`
	Kind          string `yaml:"kind"`
	Decomposition string `yaml:"decomposition"`
	Encodings     struct {
		Bands []BandSpec `yaml:"bands"`
	} `yaml:"encodings"`
}, decomposition, error) {
	if len(project.RenderProfiles) == 0 {
		return struct {
			ID            string `yaml:"id"`
			Kind          string `yaml:"kind"`
			Decomposition string `yaml:"decomposition"`
			Encodings     struct {
				Bands []BandSpec `yaml:"bands"`
			} `yaml:"encodings"`
		}{}, decomposition{}, fmt.Errorf("project has no render profiles")
	}
	profile := project.RenderProfiles[0]
	if requested != "" {
		found := false
		for _, candidate := range project.RenderProfiles {
			if candidate.ID == requested {
				profile = candidate
				found = true
				break
			}
		}
		if !found {
			return profile, decomposition{}, fmt.Errorf("render profile %q does not exist", requested)
		}
	}
	profile.Encodings.Bands = orderedBands(profile.Encodings.Bands)
	for _, candidate := range project.Decompositions {
		if candidate.ID == profile.Decomposition {
			return profile, candidate, nil
		}
	}
	return profile, decomposition{}, fmt.Errorf("decomposition %q does not exist", profile.Decomposition)
}

func orderedBands(bands []BandSpec) []BandSpec {
	result := append([]BandSpec(nil), bands...)
	sort.SliceStable(result, func(left, right int) bool { return result[left].Order < result[right].Order })
	return result
}

func systemElements(model structuralDocument) []element {
	var result []element
	for _, item := range model.Elements {
		if item.Kind == "system" {
			result = append(result, item)
		}
	}
	return result
}

func explicitMembers(decomposition decomposition) map[string]bool {
	result := make(map[string]bool)
	for _, district := range decomposition.Districts {
		for _, id := range district.Members.ElementIDs {
			result[id] = true
		}
	}
	return result
}

func importantElements(model structuralDocument, project projectDocument) map[string]bool {
	result := make(map[string]bool)
	for _, relation := range model.Relations {
		result[relation.From] = true
		result[relation.To] = true
	}
	for _, boundary := range project.Boundaries {
		result[boundary.Subject] = true
	}
	for _, review := range project.SecurityReviews {
		for _, subject := range review.Subjects {
			result[subject] = true
		}
	}
	return result
}

func shouldRenderElement(item element, explicit, important map[string]bool) bool {
	switch item.Kind {
	case "system", "layer":
		return false
	case "operation", "process", "deployable":
		return explicit[item.ID] || important[item.ID]
	default:
		return true
	}
}

func owningSystem(id string, elements map[string]element) string {
	seen := make(map[string]bool)
	for id != "" && !seen[id] {
		seen[id] = true
		item, exists := elements[id]
		if !exists {
			return ""
		}
		if item.Kind == "system" {
			return item.ID
		}
		id = item.Parent
	}
	return ""
}

func matchDistrict(item element, sources []sourceRef, artifacts map[string]artifact, elements map[string]element, districts []district) (district, bool, error) {
	// Explicit semantic membership always outranks broad path selectors. Without
	// this pass an evidence reference in a controller file could pull a service
	// component into the interface district merely because that district appears
	// first in the manifest.
	for _, candidate := range districts {
		if contains(candidate.Members.Exclude, item.ID) {
			continue
		}
		for _, member := range candidate.Members.ElementIDs {
			if item.ID == member {
				return candidate, true, nil
			}
		}
	}
	for _, candidate := range districts {
		if contains(candidate.Members.Exclude, item.ID) {
			continue
		}
		for _, member := range candidate.Members.ElementIDs {
			if hasAncestor(item.ID, member, elements) {
				return candidate, true, nil
			}
		}
	}
	for _, candidate := range districts {
		if contains(candidate.Members.Exclude, item.ID) {
			continue
		}
		matched := false
		if len(candidate.Members.PathGlobs) > 0 {
			for _, source := range sources {
				path := artifacts[source.Artifact].Path
				for _, pattern := range candidate.Members.PathGlobs {
					matches, err := scoping.MatchPath(pattern, path)
					if err != nil {
						return district{}, false, fmt.Errorf("district %s pattern %q: %w", candidate.ID, pattern, err)
					}
					if matches {
						matched = true
						break
					}
				}
				if matched {
					break
				}
			}
		}
		if matched {
			return candidate, true, nil
		}
	}
	for _, candidate := range districts {
		if contains(candidate.Members.Exclude, item.ID) {
			continue
		}
		matched := false
		if len(candidate.Members.Facets) > 0 {
			matched = facetsMatch(item.Facets, candidate.Members.Facets)
		}
		if matched {
			return candidate, true, nil
		}
	}
	return district{}, false, nil
}

func hasAncestor(id, ancestor string, elements map[string]element) bool {
	seen := make(map[string]bool)
	for id != "" && !seen[id] {
		seen[id] = true
		item, exists := elements[id]
		if !exists || item.Parent == "" {
			return false
		}
		if item.Parent == ancestor {
			return true
		}
		id = item.Parent
	}
	return false
}

func facetsMatch(actual, selector map[string]any) bool {
	for key, expected := range selector {
		value, exists := actual[key]
		if !exists || fmt.Sprint(value) != fmt.Sprint(expected) {
			return false
		}
	}
	return true
}

func uniqueArtifactIDs(refs []sourceRef) []string {
	seen := make(map[string]bool)
	var result []string
	for _, ref := range refs {
		if ref.Artifact != "" && !seen[ref.Artifact] {
			seen[ref.Artifact] = true
			result = append(result, ref.Artifact)
		}
	}
	return result
}

func sourceView(refs []sourceRef, artifacts map[string]artifact, repositoryRoot string) *SourceView {
	if len(refs) == 0 {
		return nil
	}
	ref := refs[0]
	item, exists := artifacts[ref.Artifact]
	if !exists || item.Path == "" {
		return nil
	}
	line := 0
	if len(ref.Lines) > 0 {
		line = ref.Lines[0]
	}
	return &SourceView{
		Path: item.Path, Absolute: filepath.Join(repositoryRoot, filepath.FromSlash(item.Path)),
		Symbol: ref.Symbol, Line: line,
	}
}

func indexMeasurements(items []measurement) map[string]map[string][]measurement {
	result := make(map[string]map[string][]measurement)
	for _, item := range items {
		if result[item.Subject] == nil {
			result[item.Subject] = make(map[string][]measurement)
		}
		result[item.Subject][item.Metric] = append(result[item.Subject][item.Metric], item)
	}
	return result
}

func collectMetrics(draft nodeDraft, indexed map[string]map[string][]measurement, definitions map[string]metricDefinition) map[string]Metric {
	byMetric := make(map[string][]measurement)
	directMetrics := indexed[draft.element.ID]
	for metric, items := range directMetrics {
		byMetric[metric] = append(byMetric[metric], items...)
	}
	for _, artifactID := range draft.artifactIDs {
		for metric, items := range indexed[artifactID] {
			// An element-level measurement is an explicit aggregate and therefore
			// outranks measurements inherited from its source artifacts. Combining
			// both would double-count the same code.
			if len(directMetrics[metric]) > 0 {
				continue
			}
			byMetric[metric] = append(byMetric[metric], items...)
		}
	}
	result := make(map[string]Metric)
	for metricID, items := range byMetric {
		var values []float64
		status := "unknown"
		for _, item := range items {
			if item.Status != "observed" && item.Status != "stale" {
				continue
			}
			values = append(values, item.Value)
			if item.Status == "stale" {
				status = "stale"
			} else if status == "unknown" {
				status = "observed"
			}
		}
		if len(values) == 0 {
			continue
		}
		value := aggregateMetric(metricID, values)
		definition := definitions[metricID]
		result[metricID] = Metric{Value: value, Status: status, Unit: definition.Unit, Name: definition.Name}
	}
	return result
}

func aggregateMetric(metric string, values []float64) float64 {
	if strings.Contains(metric, "coverage") {
		total := 0.0
		for _, value := range values {
			total += value
		}
		return total / float64(len(values))
	}
	if metric == "code.lines.physical" {
		total := 0.0
		for _, value := range values {
			total += value
		}
		return total
	}
	maximum := values[0]
	for _, value := range values[1:] {
		maximum = math.Max(maximum, value)
	}
	return maximum
}

func composeRelations(items []relation, elements map[string]element, visible map[string]bool, nodeIndex map[string]int, nodes []NodeView, artifacts map[string]artifact, repositoryRoot string) ([]RelationView, map[string]int, map[string]int, map[string]int, []string) {
	var result []RelationView
	mutationCounts := make(map[string]int)
	degree := make(map[string]int)
	crossDegree := make(map[string]int)
	var warnings []string
	for _, item := range items {
		from := resolveVisibleEndpoint(item.From, elements, visible)
		to := resolveVisibleEndpoint(item.To, elements, visible)
		if from == "" || to == "" || from == to {
			warnings = append(warnings, fmt.Sprintf("relation %s is not visible at the current building level", item.ID))
			continue
		}
		fromIndex, fromExists := nodeIndex[from]
		toIndex, toExists := nodeIndex[to]
		if !fromExists || !toExists {
			warnings = append(warnings, fmt.Sprintf("relation %s could not resolve a rendered endpoint", item.ID))
			continue
		}
		mutates := item.Effect.MutatesState == "yes"
		if mutates {
			mutationCounts[from]++
		}
		degree[from]++
		degree[to]++
		if nodes[fromIndex].DistrictID != nodes[toIndex].DistrictID {
			crossDegree[from]++
			crossDegree[to]++
		}
		result = append(result, RelationView{
			ID: item.ID, From: from, To: to, Kind: item.Kind,
			Family: relationFamily(item), Execution: relationExecution(item),
			Protocol: item.Protocol, Summary: item.Summary, Mutates: mutates,
			Source: sourceView(item.Evidence, artifacts, repositoryRoot),
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, mutationCounts, degree, crossDegree, warnings
}

func resolveVisibleEndpoint(id string, elements map[string]element, visible map[string]bool) string {
	seen := make(map[string]bool)
	for id != "" && !seen[id] {
		seen[id] = true
		if visible[id] {
			return id
		}
		item, exists := elements[id]
		if !exists {
			return ""
		}
		id = item.Parent
	}
	return ""
}

func relationFamily(item relation) string {
	switch item.Kind {
	case "imports", "implements", "depends_on", "contains":
		return "import"
	case "publishes", "subscribes":
		return "event"
	case "reads", "writes", "deletes", "queries", "persists":
		return "state"
	case "authenticates_with", "redirects_to", "callback_from", "loads":
		return "provider"
	default:
		if item.Effect.Domain == "database" || item.Effect.Domain == "filesystem" || item.Effect.Domain == "browser_storage" {
			return "state"
		}
		if item.Effect.Domain == "network" || item.Effect.Domain == "external_state" {
			return "provider"
		}
		return "call"
	}
}

func relationExecution(item relation) string {
	if item.Kind == "imports" || item.Kind == "implements" || item.Kind == "depends_on" {
		return "static"
	}
	if item.Kind == "callback_from" || item.Execution.Trigger == "callback" {
		return "callback"
	}
	if item.Execution.Style == "asynchronous" || item.Execution.Style == "event_driven" || item.Kind == "publishes" || item.Kind == "subscribes" {
		return "async"
	}
	if item.Execution.Style == "unknown" || item.Execution.Style == "" {
		return "unknown"
	}
	return "sync"
}

// applyVisualCorrections handles correction paths that currently affect the
// renderer. Unsupported paths become visible warnings instead of being
// silently validated and ignored.
func applyVisualCorrections(project projectDocument, drafts []nodeDraft, artifacts map[string]artifact) ([]string, map[string]bool, error) {
	var warnings []string
	mutationOverrides := make(map[string]bool)
	for _, correction := range project.Corrections {
		targets, err := correctionTargetIDs(correction.Target.Entity, correction.Target.Selector, drafts, artifacts)
		if err != nil {
			return nil, nil, fmt.Errorf("correction %s: %w", correction.ID, err)
		}
		if len(targets) == 0 {
			warnings = append(warnings, fmt.Sprintf("correction %s does not target a rendered element", correction.ID))
			continue
		}
		for _, operation := range correction.Operations {
			if !strings.HasPrefix(operation.Path, "effects.") || !strings.HasSuffix(operation.Path, ".mutates_state") {
				warnings = append(warnings, fmt.Sprintf("correction %s path %s is validated but not applied by view composer 0.1", correction.ID, operation.Path))
				continue
			}
			value, valid := correctionBoolean(operation.Action, operation.Value)
			if !valid {
				warnings = append(warnings, fmt.Sprintf("correction %s path %s has a non-boolean visual value", correction.ID, operation.Path))
				continue
			}
			for _, target := range targets {
				mutationOverrides[target] = value
			}
		}
	}
	return warnings, mutationOverrides, nil
}

func correctionTargetIDs(entity string, selector selector, drafts []nodeDraft, artifacts map[string]artifact) ([]string, error) {
	selected := make(map[string]bool)
	if entity != "" {
		selected[entity] = true
	}
	for _, id := range selector.ElementIDs {
		selected[id] = true
	}
	for _, draft := range drafts {
		if contains(selector.Exclude, draft.element.ID) {
			delete(selected, draft.element.ID)
			continue
		}
		matched := len(selector.Facets) > 0 && facetsMatch(draft.element.Facets, selector.Facets)
		if !matched {
			for _, pattern := range selector.PathGlobs {
				for _, source := range draft.sources {
					pathMatches, err := scoping.MatchPath(pattern, artifacts[source.Artifact].Path)
					if err != nil {
						return nil, fmt.Errorf("selector pattern %q: %w", pattern, err)
					}
					if pathMatches {
						matched = true
						break
					}
				}
				if matched {
					break
				}
			}
		}
		if matched {
			selected[draft.element.ID] = true
		}
	}

	var result []string
	for _, draft := range drafts {
		if selected[draft.element.ID] && !contains(selector.Exclude, draft.element.ID) {
			result = append(result, draft.element.ID)
		}
	}
	sort.Strings(result)
	return result, nil
}

func correctionBoolean(action string, value any) (bool, bool) {
	if action == "remove" {
		return false, true
	}
	switch typed := value.(type) {
	case bool:
		return typed, true
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "true", "yes":
			return true, true
		case "false", "no":
			return false, true
		}
	}
	return false, false
}

func nodeDirectMutation(node NodeView, mutationCounts map[string]int, mutationOverrides map[string]bool) bool {
	direct := mutationCounts[node.ID] > 0
	if sites, exists := node.Measurements["effects.mutating_sites"]; exists && sites.Status == "observed" && sites.Value > 0 {
		direct = true
	}
	if density, exists := node.Measurements["effects.direct_mutation_density"]; exists && density.Status == "observed" && density.Value > 0 {
		direct = true
	}
	if corrected, exists := mutationOverrides[node.ID]; exists {
		return corrected
	}
	return direct
}

func rawBands(node NodeView, specs []BandSpec, degree, crossDegree, mutationCounts map[string]int, mutationOverrides map[string]bool) []BandValue {
	result := make([]BandValue, 0, len(specs))
	for _, spec := range specs {
		value, status, exists := bandMetric(node, spec.Metric, degree, crossDegree, mutationCounts, mutationOverrides)
		band := BandValue{ID: spec.ID, Label: spec.Label, Metric: spec.Metric, Status: status}
		if exists {
			copy := value
			band.Raw = &copy
		}
		result = append(result, band)
	}
	return result
}

func bandMetric(node NodeView, metric string, degree, crossDegree, mutationCounts map[string]int, mutationOverrides map[string]bool) (float64, string, bool) {
	if metric == "effects.direct_mutation_density" {
		if corrected, exists := mutationOverrides[node.ID]; exists {
			if !corrected {
				return 0, "corrected", true
			}
			if direct, measured := node.Measurements[metric]; measured {
				return direct.Value, "corrected", true
			}
			denominator := math.Max(1, node.Lines/1000)
			if sites, measured := node.Measurements["effects.mutating_sites"]; measured {
				return sites.Value / denominator, "corrected", true
			}
			return float64(max(1, mutationCounts[node.ID])) / denominator, "corrected", true
		}
	}
	if direct, exists := node.Measurements[metric]; exists {
		return direct.Value, direct.Status, true
	}
	switch metric {
	case "quality.coverage.line_gap":
		if coverage, exists := node.Measurements["test.line_coverage"]; exists {
			return 1 - coverage.Value, coverage.Status, true
		}
	case "quality.complexity.max_operation":
		if complexity, exists := node.Measurements["complexity.cyclomatic.max"]; exists {
			return complexity.Value, complexity.Status, true
		}
	case "topology.participation":
		if degree[node.ID] > 0 {
			return float64(crossDegree[node.ID]) / float64(degree[node.ID]), "derived", true
		}
	case "effects.direct_mutation_density":
		if sites, exists := node.Measurements["effects.mutating_sites"]; exists {
			denominator := math.Max(1, node.Lines/1000)
			return sites.Value / denominator, sites.Status, true
		}
		if mutationCounts[node.ID] > 0 {
			denominator := math.Max(1, node.Lines/1000)
			return float64(mutationCounts[node.ID]) / denominator, "derived", true
		}
	}
	return 0, "unknown", false
}

func normalizeBands(nodes []NodeView, specs []BandSpec) {
	for bandIndex, spec := range specs {
		var values []float64
		for _, node := range nodes {
			if bandIndex < len(node.Bands) && node.Bands[bandIndex].Raw != nil {
				values = append(values, *node.Bands[bandIndex].Raw)
			}
		}
		sorted := append([]float64(nil), values...)
		sort.Float64s(sorted)
		maximum := 0.0
		if len(sorted) > 0 {
			maximum = sorted[len(sorted)-1]
		}
		for nodeIndex := range nodes {
			band := &nodes[nodeIndex].Bands[bandIndex]
			if band.Raw == nil {
				continue
			}
			value := *band.Raw
			normalized := 0.0
			switch spec.Normalization {
			case "ratio_0_1":
				normalized = clamp(value, 0, 1)
			case "log_project_percentile":
				if maximum > 0 {
					normalized = math.Log1p(math.Max(0, value)) / math.Log1p(maximum)
				}
			case "project_percentile":
				normalized = percentile(sorted, value)
			case "absolute_threshold":
				normalized = thresholdRatio(spec.Thresholds, value)
			default:
				if maximum > 0 {
					normalized = value / maximum
				}
			}
			if spec.Direction == "lower_attention" {
				normalized = 1 - normalized
			}
			normalized = clamp(normalized, 0, 1)
			band.Normalized = &normalized
		}
	}
}

func percentile(sorted []float64, value float64) float64 {
	if len(sorted) <= 1 {
		return 0.5
	}
	lower := sort.Search(len(sorted), func(index int) bool { return sorted[index] >= value })
	upper := sort.Search(len(sorted), func(index int) bool { return sorted[index] > value })
	if lower == len(sorted) {
		return 1
	}
	if upper == lower {
		upper = lower + 1
	}
	midrank := float64(lower+upper-1) / 2
	return midrank / float64(len(sorted)-1)
}

func thresholdRatio(thresholds []float64, value float64) float64 {
	if len(thresholds) == 0 {
		return 0
	}
	ordered := append([]float64(nil), thresholds...)
	sort.Float64s(ordered)
	index := sort.SearchFloat64s(ordered, value)
	return float64(index) / float64(len(ordered))
}

func composeBoundaries(project projectDocument, nodes map[string]int) []BoundaryView {
	var result []BoundaryView
	for _, boundary := range project.Boundaries {
		if _, exists := nodes[boundary.Subject]; !exists {
			continue
		}
		result = append(result, BoundaryView{
			ID: boundary.ID, Subject: boundary.Subject, Direction: boundary.Direction,
			Transport: boundary.Transport, Payloads: boundary.Payloads,
			TrustFrom: boundary.TrustFrom, TrustTo: boundary.TrustTo,
			Authentication: boundary.Authentication, Summary: boundary.Summary,
		})
	}
	return result
}

func composeSecurity(project projectDocument, nodes map[string]int) []SecurityView {
	var result []SecurityView
	for _, review := range project.SecurityReviews {
		var subjects []string
		for _, subject := range review.Subjects {
			if _, exists := nodes[subject]; exists {
				subjects = append(subjects, subject)
			}
		}
		if len(subjects) == 0 {
			continue
		}
		result = append(result, SecurityView{
			ID: review.ID, Subjects: subjects, Category: review.Category,
			Status: review.Status, Severity: review.Severity,
			Confidence: review.Provenance.Confidence, Summary: review.Summary, Impact: review.Impact,
		})
	}
	return result
}

func facetString(facets map[string]any, key string) string {
	value, exists := facets[key]
	if !exists {
		return ""
	}
	return fmt.Sprint(value)
}

func contains(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

func clamp(value, minimum, maximum float64) float64 {
	return math.Min(maximum, math.Max(minimum, value))
}

func slug(value string) string {
	var result strings.Builder
	dash := false
	for _, character := range strings.ToLower(value) {
		if unicode.IsLetter(character) || unicode.IsDigit(character) {
			result.WriteRune(character)
			dash = false
		} else if result.Len() > 0 && !dash {
			result.WriteByte('-')
			dash = true
		}
	}
	return strings.Trim(result.String(), "-")
}
