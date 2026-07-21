package viewer

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mmirolim/vibecodemap/internal/projectdsl"
	"github.com/mmirolim/vibecodemap/internal/scoping"
	"gopkg.in/yaml.v3"
)

type ValidationOptions struct {
	Kind string
	Core string
}

// ValidateDocument validates a project, structural, or quality document. Kind
// defaults to auto and is detected from the version key at the document root.
func ValidateDocument(file string, options ValidationOptions) []projectdsl.Diagnostic {
	kind := strings.ToLower(strings.TrimSpace(options.Kind))
	if kind == "" || kind == "auto" {
		var err error
		kind, err = detectDocumentKind(file)
		if err != nil {
			return []projectdsl.Diagnostic{{File: file, Severity: "error", Code: "document.detect", Message: err.Error()}}
		}
	}
	switch kind {
	case "project":
		return validateProjectDocument(file)
	case "structural":
		return validateStructuralDocument(file)
	case "quality":
		return validateQualityDocument(file, options.Core)
	default:
		return []projectdsl.Diagnostic{{File: file, Severity: "error", Code: "document.kind", Message: fmt.Sprintf("unsupported document kind %q", kind)}}
	}
}

func detectDocumentKind(file string) (string, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}
	var header map[string]any
	if err := yaml.Unmarshal(data, &header); err != nil {
		return "", err
	}
	found := ""
	for _, candidate := range []struct {
		key  string
		kind string
	}{{"vcm_project", "project"}, {"vcm", "structural"}, {"vcm_quality", "quality"}} {
		if _, exists := header[candidate.key]; exists {
			if found != "" {
				return "", fmt.Errorf("root contains more than one VCM version key")
			}
			found = candidate.kind
		}
	}
	if found != "" {
		return found, nil
	}
	return "", fmt.Errorf("root must contain one of vcm_project, vcm, or vcm_quality")
}

func validateProjectDocument(file string) []projectdsl.Diagnostic {
	diagnostics := projectdsl.ValidateFile(file)
	if diagnosticsHaveErrors(diagnostics) {
		return diagnostics
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return append(diagnostics, validationDiagnostic(file, "file.read", err))
	}
	var project projectDocument
	if err := yaml.Unmarshal(data, &project); err != nil {
		return append(diagnostics, validationDiagnostic(file, "yaml.decode", err))
	}
	base := filepath.Dir(file)
	structural := resolveDocumentPath(base, project.Project.Inputs.StructuralModel)
	diagnostics = append(diagnostics, validateStructuralDocument(structural)...)
	if project.Project.Inputs.QualityModel != "" {
		quality := resolveDocumentPath(base, project.Project.Inputs.QualityModel)
		diagnostics = append(diagnostics, validateQualityDocument(quality, structural)...)
	}
	sortValidationDiagnostics(diagnostics)
	return diagnostics
}

func validateStructuralDocument(file string) []projectdsl.Diagnostic {
	data, err := os.ReadFile(file)
	if err != nil {
		return []projectdsl.Diagnostic{validationDiagnostic(file, "file.read", err)}
	}
	if err := validateYAMLContract(data, file, loadStructuralSchema); err != nil {
		return []projectdsl.Diagnostic{validationDiagnostic(file, "structural.schema", err)}
	}
	var model structuralDocument
	if err := yaml.Unmarshal(data, &model); err != nil {
		return []projectdsl.Diagnostic{validationDiagnostic(file, "yaml.decode", err)}
	}
	var diagnostics []projectdsl.Diagnostic
	if root, available, err := structuralRepositoryRoot(file, model); err != nil {
		diagnostics = append(diagnostics, validationDiagnostic(file, "structural.repository", err))
	} else if !available {
		diagnostics = append(diagnostics, projectdsl.Diagnostic{
			File: file, Severity: "warning", Code: "structural.repository_unavailable",
			Message: fmt.Sprintf("repository root %s is unavailable; artifact existence, line counts, and source ranges were not checked", root),
		})
	}
	if err := validateStructuralReferences(file, model); err != nil {
		diagnostics = append(diagnostics, validationDiagnostic(file, "structural.references", err))
	}
	if err := validateStructuralSources(file, model); err != nil {
		diagnostics = append(diagnostics, validationDiagnostic(file, "structural.sources", err))
	}
	scopeWarnings, err := validateStructuralScope(file, model)
	if err != nil {
		diagnostics = append(diagnostics, validationDiagnostic(file, "structural.scope", err))
	}
	for _, warning := range scopeWarnings {
		diagnostics = append(diagnostics, projectdsl.Diagnostic{File: file, Severity: "warning", Code: "structural.scope", Message: warning})
	}
	return diagnostics
}

func validateQualityDocument(file, coreFile string) []projectdsl.Diagnostic {
	data, err := os.ReadFile(file)
	if err != nil {
		return []projectdsl.Diagnostic{validationDiagnostic(file, "file.read", err)}
	}
	if err := validateYAMLContract(data, file, loadQualitySchema); err != nil {
		return []projectdsl.Diagnostic{validationDiagnostic(file, "quality.schema", err)}
	}
	var quality qualityDocument
	if err := yaml.Unmarshal(data, &quality); err != nil {
		return []projectdsl.Diagnostic{validationDiagnostic(file, "yaml.decode", err)}
	}
	var structural structuralDocument
	var diagnostics []projectdsl.Diagnostic
	if coreFile == "" {
		structural = syntheticStructuralSubjects(quality)
		diagnostics = append(diagnostics, projectdsl.Diagnostic{
			File: file, Severity: "warning", Code: "quality.core_skipped",
			Message: "cross-model subjects and source artifacts were not checked; pass -core STRUCTURAL.vcm.yaml",
		})
	} else {
		coreData, err := os.ReadFile(coreFile)
		if err != nil {
			return append(diagnostics, validationDiagnostic(coreFile, "file.read", err))
		}
		if err := validateYAMLContract(coreData, coreFile, loadStructuralSchema); err != nil {
			return append(diagnostics, validationDiagnostic(coreFile, "structural.schema", err))
		}
		if err := yaml.Unmarshal(coreData, &structural); err != nil {
			return append(diagnostics, validationDiagnostic(coreFile, "yaml.decode", err))
		}
	}
	if err := validateQualityReferences(file, structural, quality); err != nil {
		diagnostics = append(diagnostics, validationDiagnostic(file, "quality.references", err))
	}
	for _, warning := range qualityWarnings(quality) {
		diagnostics = append(diagnostics, projectdsl.Diagnostic{File: file, Severity: "warning", Code: "quality.evidence", Message: warning})
	}
	return diagnostics
}

func syntheticStructuralSubjects(quality qualityDocument) structuralDocument {
	var model structuralDocument
	model.Model.ID = quality.Model.CoreModel
	subjects := make(map[string]struct{})
	artifacts := make(map[string]struct{})
	addSources := func(refs []sourceRef) {
		for _, ref := range refs {
			artifacts[ref.Artifact] = struct{}{}
		}
	}
	for _, item := range quality.Measurements {
		subjects[item.Subject] = struct{}{}
		addSources(item.SourceRefs)
	}
	for _, item := range quality.AnalyzerFindings {
		subjects[item.Subject] = struct{}{}
		addSources(item.SourceRefs)
	}
	for _, item := range quality.QualityFindings {
		for _, subject := range item.Subjects {
			subjects[subject] = struct{}{}
		}
		addSources(item.Evidence)
	}
	for _, item := range quality.PriorityResults {
		subjects[item.Subject] = struct{}{}
	}
	delete(subjects, model.Model.ID)
	for subject := range subjects {
		model.Elements = append(model.Elements, element{ID: subject})
	}
	for artifactID := range artifacts {
		model.Artifacts = append(model.Artifacts, artifact{ID: artifactID})
	}
	return model
}

func validateStructuralSources(file string, model structuralDocument) error {
	root, available, err := structuralRepositoryRoot(file, model)
	if err != nil {
		return err
	}
	if !available {
		return nil
	}
	counts := make(map[string]int, len(model.Artifacts))
	paths := make(map[string]string, len(model.Artifacts))
	problemSet := make(map[string]struct{})
	addProblem := func(problem string) { problemSet[problem] = struct{}{} }
	for _, artifact := range model.Artifacts {
		candidate := filepath.Clean(filepath.Join(root, filepath.FromSlash(artifact.Path)))
		relative, relErr := filepath.Rel(root, candidate)
		if relErr != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			addProblem(fmt.Sprintf("artifact %q path escapes repository root: %q", artifact.ID, artifact.Path))
			continue
		}
		data, readErr := os.ReadFile(candidate)
		if readErr != nil {
			addProblem(fmt.Sprintf("artifact %q path %q cannot be read: %v", artifact.ID, artifact.Path, readErr))
			continue
		}
		lineCount := 0
		if len(data) > 0 {
			lineCount = bytes.Count(data, []byte{'\n'})
			if data[len(data)-1] != '\n' {
				lineCount++
			}
		}
		counts[artifact.ID] = lineCount
		paths[artifact.ID] = artifact.Path
		if declared, exists := artifact.Metrics["lines"]; exists && int(declared) != lineCount {
			addProblem(fmt.Sprintf("artifact %q declares %.0f lines, actual file has %d: %s", artifact.ID, declared, lineCount, artifact.Path))
		}
	}
	checkRefs := func(owner string, refs []sourceRef) {
		for _, ref := range refs {
			if len(ref.Lines) == 0 {
				continue
			}
			maximum, exists := counts[ref.Artifact]
			if !exists || len(ref.Lines) != 2 || ref.Lines[0] < 1 || ref.Lines[1] < ref.Lines[0] || ref.Lines[1] > maximum {
				addProblem(fmt.Sprintf("%s source range %v is outside artifact %q (%s, 1..%d)", owner, ref.Lines, ref.Artifact, paths[ref.Artifact], maximum))
			}
		}
	}
	for _, item := range model.Elements {
		checkRefs("element "+item.ID, item.SourceRefs)
		checkRefs("element "+item.ID, item.Evidence)
	}
	for _, item := range model.Relations {
		checkRefs("relation "+item.ID, item.Evidence)
	}
	for _, item := range model.Findings {
		checkRefs("finding "+item.ID, item.Evidence)
	}
	for _, item := range model.Architecture.Styles {
		checkRefs("architecture style "+item.ID, item.Evidence)
	}
	for _, item := range model.Architecture.Constraints {
		checkRefs("architecture constraint "+item.ID, item.Evidence)
	}
	problems := make([]string, 0, len(problemSet))
	for problem := range problemSet {
		problems = append(problems, problem)
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("%s: source validation failed:\n  %s", file, strings.Join(problems, "\n  "))
	}
	return nil
}

func structuralRepositoryRoot(file string, model structuralDocument) (string, bool, error) {
	root := model.Model.Repository.Root
	if !filepath.IsAbs(root) {
		root = filepath.Join(filepath.Dir(file), root)
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return root, false, err
	}
	info, err := os.Stat(root)
	if os.IsNotExist(err) {
		return root, false, nil
	}
	if err != nil {
		return root, false, fmt.Errorf("repository root %s: %w", root, err)
	}
	if !info.IsDir() {
		return root, false, fmt.Errorf("repository root %s is not a directory", root)
	}
	return root, true, nil
}

func validateStructuralScope(file string, model structuralDocument) ([]string, error) {
	root, available, err := structuralRepositoryRoot(file, model)
	if err != nil || !available {
		return nil, err
	}
	includeMatcher, err := scoping.CompilePathMatcher(model.Scope.Include)
	if err != nil {
		return nil, fmt.Errorf("compile include scope: %w", err)
	}
	excludeMatcher, err := scoping.CompilePathMatcher(model.Scope.Exclude)
	if err != nil {
		return nil, fmt.Errorf("compile exclude scope: %w", err)
	}
	expected := make(map[string]struct{})
	err = filepath.WalkDir(root, func(candidate string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		relative, err := filepath.Rel(root, candidate)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if !includeMatcher.Match(relative) {
			return nil
		}
		if !excludeMatcher.Match(relative) {
			expected[relative] = struct{}{}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk declared scope under %s: %w", root, err)
	}

	modeled := make(map[string]struct{}, len(model.Artifacts))
	for _, artifact := range model.Artifacts {
		modeled[filepath.ToSlash(filepath.Clean(artifact.Path))] = struct{}{}
	}
	var missing []string
	for path := range expected {
		if _, exists := modeled[path]; !exists {
			missing = append(missing, path)
		}
	}
	var warnings []string
	for path := range modeled {
		if _, exists := expected[path]; !exists {
			warnings = append(warnings, fmt.Sprintf("artifact is outside declared scope: %s", path))
		}
	}
	sort.Strings(missing)
	sort.Strings(warnings)
	if len(missing) > 0 {
		return warnings, fmt.Errorf("artifact inventory does not cover declared scope:\n  scoped file has no artifact: %s", strings.Join(missing, "\n  scoped file has no artifact: "))
	}
	return warnings, nil
}

func validationDiagnostic(file, code string, err error) projectdsl.Diagnostic {
	return projectdsl.Diagnostic{File: file, Severity: "error", Code: code, Message: err.Error()}
}

func diagnosticsHaveErrors(diagnostics []projectdsl.Diagnostic) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == "error" {
			return true
		}
	}
	return false
}

func sortValidationDiagnostics(diagnostics []projectdsl.Diagnostic) {
	sort.SliceStable(diagnostics, func(i, j int) bool {
		if diagnostics[i].File != diagnostics[j].File {
			return diagnostics[i].File < diagnostics[j].File
		}
		if diagnostics[i].Line != diagnostics[j].Line {
			return diagnostics[i].Line < diagnostics[j].Line
		}
		return diagnostics[i].Code < diagnostics[j].Code
	})
}
