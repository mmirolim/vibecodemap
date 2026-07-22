package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mmirolim/vibecodemap/internal/adapters"
	"github.com/mmirolim/vibecodemap/internal/projectdsl"
	"github.com/mmirolim/vibecodemap/internal/qualitymodel"
	"github.com/mmirolim/vibecodemap/internal/repository"
	"github.com/mmirolim/vibecodemap/internal/scoping"
	"github.com/mmirolim/vibecodemap/internal/viewer"
)

const usage = `VibeCodeMap repository model tool

Usage:
  vibecodemap describe [project]  print the project-manifest DSL grammar
  vibecodemap schema [KIND]       print project, structural, or quality JSON Schema
  vibecodemap validate [flags] PROJECT.vcm.yaml
  vibecodemap inspect [flags] [REPOSITORY]
  vibecodemap analyze [flags] [REPOSITORY]
  vibecodemap quality [flags] STRUCTURAL.vcm.yaml
  vibecodemap adapters [-json]
  vibecodemap render [flags] PROJECT.vcm.yaml
  vibecodemap show [flags] PROJECT.vcm.yaml

Validate flags:
  -json                          emit machine-readable diagnostics
  -kind KIND                     auto, project, structural, or quality
  -core FILE                     structural model used for direct quality validation

Validation reports YAML syntax, schema, source evidence, graph, quality,
district-code, band-order, and cross-document reference errors. A project
manifest validates its referenced structural and quality documents too.

Inspect flags:
  -json                          emit the full machine-readable inventory
  -entries                       print every non-analyzed entry in text mode
  -rules FILE                    use FILE instead of REPOSITORY/.vcmignore;
                                 use -rules=- to disable repository rules
  -gitignore                     apply exact Git ignore rules (default true)
  -max-header-bytes N            generated-marker prefix budget (default 8192)
  -max-file-bytes N              detailed-analysis file budget (default 10 MiB)

Inspect produces a scoped repository inventory and stack candidates. It does
not run semantic analyzers or generate VCM DSL, a view model, or an HTML map.
Scope correction is optional; use .vcmignore only for false classifications.

Analyze flags:
  -output FILE                   evidence JSON output; default is
                                 REPOSITORY/.vibecodemap/generated/evidence.json
  -adapter-timeout DURATION      maximum time for each analyzer (default 2m)
  -rules, -gitignore,
  -max-header-bytes,
  -max-file-bytes               same central scope controls as inspect

analyze scans once and automatically runs every implemented semantic adapter
for detected stacks. Detection-only stacks are explicitly reported as not
implemented. The evidence helps an AI or human author DSL; it is not DSL.

Quality flags:
  -evidence FILE                 analyzer evidence JSON; default is
                                 STRUCTURAL_DIR/generated/evidence.json
  -output FILE                   generated quality DSL; default is
                                 STRUCTURAL_DIR/quality.vcm.yaml

quality maps deterministic file/symbol measurements onto structural artifact
and element IDs. It keeps unavailable coverage and unsupported metrics unknown;
the project manifest must explicitly reference the generated quality model.

Render flags:
  -output FILE                   write standalone HTML to FILE
  -json-output FILE              write derived view-model JSON to FILE
  -profile ID                    select a render profile (default: first)
  -open                          open generated HTML in the browser

render validates all referenced DSL, creates JSON and HTML, and writes them to
PROJECT_DIR/out by default. show performs the same pipeline and opens the map.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	switch os.Args[1] {
	case "describe", "grammar":
		os.Exit(runDescribe(os.Args[2:]))
	case "schema":
		os.Exit(runSchema(os.Args[2:]))
	case "validate":
		os.Exit(runValidate(os.Args[2:]))
	case "inspect", "inventory":
		os.Exit(runInspect(os.Args[2:]))
	case "analyze":
		os.Exit(runAnalyze(os.Args[2:]))
	case "quality":
		os.Exit(runQuality(os.Args[2:]))
	case "adapters":
		os.Exit(runAdapters(os.Args[2:]))
	case "render":
		os.Exit(runRender(os.Args[2:], false))
	case "show":
		os.Exit(runRender(os.Args[2:], true))
	case "help", "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}
}

func runDescribe(arguments []string) int {
	if len(arguments) > 1 || (len(arguments) == 1 && arguments[0] != "project") {
		fmt.Fprintln(os.Stderr, "describe currently supports only the project grammar; use schema structural|quality for the other contracts")
		return 2
	}
	_, _ = os.Stdout.Write(projectdsl.Grammar())
	return 0
}

func runSchema(arguments []string) int {
	if len(arguments) > 1 {
		fmt.Fprintln(os.Stderr, "schema accepts at most one kind: project, structural, or quality")
		return 2
	}
	kind := "project"
	if len(arguments) == 1 {
		kind = strings.ToLower(arguments[0])
	}
	var document []byte
	var err error
	if kind == "project" {
		document = projectdsl.Schema()
	} else {
		document, err = viewer.ContractSchema(kind)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	_, _ = os.Stdout.Write(document)
	return 0
}

func runRender(arguments []string, openByDefault bool) int {
	name := "render"
	if openByDefault {
		name = "show"
	}
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	output := flags.String("output", "", "standalone HTML output path")
	jsonOutput := flags.String("json-output", "", "derived view-model JSON output path")
	profile := flags.String("profile", "", "render profile id")
	openBrowser := flags.Bool("open", openByDefault, "open generated HTML in the browser")
	if err := flags.Parse(arguments); err != nil {
		return 2
	}
	if flags.NArg() != 1 {
		fmt.Fprintf(os.Stderr, "%s requires exactly one project manifest\n", name)
		return 2
	}
	result, err := viewer.Render(flags.Arg(0), viewer.RenderOptions{
		Profile: *profile, Output: *output, JSONOutput: *jsonOutput,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("validated:  %s\n", result.ViewModel.Project.ProjectManifest)
	fmt.Printf("view model: %s\n", result.JSONPath)
	fmt.Printf("HTML map:   %s\n", result.HTMLPath)
	fmt.Printf("mapped:     %d cities, %d districts, %d buildings, %d roads\n",
		result.ViewModel.Stats.Systems, result.ViewModel.Stats.Districts,
		result.ViewModel.Stats.Nodes, result.ViewModel.Stats.Roads)
	if *openBrowser {
		if err := viewer.Open(result.HTMLPath); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Println("opened:     default browser")
	}
	return 0
}

type inspectOutput struct {
	Schema     string                `json:"schema"`
	Inventory  repository.Report     `json:"inventory"`
	Detections []adapters.Detection  `json:"detections"`
	Adapters   []adapters.Descriptor `json:"adapters"`
}

func runInspect(arguments []string) int {
	flags := flag.NewFlagSet("inspect", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	jsonOutput := flags.Bool("json", false, "emit JSON inventory")
	showEntries := flags.Bool("entries", false, "print non-analyzed entries")
	ruleFile := flags.String("rules", "", "repository rule file; '-' disables it")
	readGitignore := flags.Bool("gitignore", true, "apply Git ignore rules")
	maxHeaderBytes := flags.Int64("max-header-bytes", 8192, "generated marker prefix budget")
	maxFileBytes := flags.Int64("max-file-bytes", 10*1024*1024, "detailed analysis file budget")
	if err := flags.Parse(arguments); err != nil {
		return 2
	}
	if flags.NArg() > 1 {
		fmt.Fprintln(os.Stderr, "inspect accepts at most one repository path")
		return 2
	}
	root := "."
	if flags.NArg() == 1 {
		root = flags.Arg(0)
	}
	if *maxHeaderBytes <= 0 || *maxFileBytes <= 0 {
		fmt.Fprintln(os.Stderr, "inspection byte budgets must be positive")
		return 2
	}

	options := repository.DefaultOptions()
	options.RuleFile = *ruleFile
	options.ReadGitignore = *readGitignore
	options.MaxHeaderBytes = *maxHeaderBytes
	options.MaxFileBytes = *maxFileBytes
	report, err := repository.Scan(context.Background(), root, options)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	registry, err := adapters.BuiltinRegistry()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	detections, err := registry.Detect(context.Background(), report)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	output := inspectOutput{
		Schema: "vibecodemap.inspect/0.1", Inventory: report,
		Detections: detections, Adapters: registry.Descriptors(),
	}
	if *jsonOutput {
		return encodeJSON(output)
	}
	printInspect(output, *showEntries)
	return 0
}

func printInspect(output inspectOutput, showEntries bool) {
	fmt.Printf("Repository: %s\n", output.Inventory.Root)
	if output.Inventory.RuleFile != "" {
		fmt.Printf("Rules:      %s\n", output.Inventory.RuleFile)
	}
	fmt.Printf("Git ignore: %t\n\n", output.Inventory.GitignoreApplied)
	fmt.Println("Inventory")
	for _, action := range []scoping.Action{scoping.Analyze, scoping.Summarize, scoping.Externalize, scoping.Ignore} {
		total := output.Inventory.Totals[action]
		pruned := ""
		if total.PrunedDirectories > 0 {
			pruned = fmt.Sprintf(" + %d pruned dirs", total.PrunedDirectories)
		}
		fmt.Printf("  %-11s %6d files  %10s%s\n", action, total.Files, humanBytes(total.Bytes), pruned)
	}

	fmt.Println("\nDetected stacks")
	if len(output.Detections) == 0 {
		fmt.Println("  none")
	}
	for _, detection := range output.Detections {
		fmt.Printf("  %-18s %-16s scope=%-20s confidence=%3.0f%%  %s\n",
			detection.Stack, detection.AdapterID, detection.Scope, detection.Confidence*100, detection.Support)
	}
	fmt.Println("\nResult")
	fmt.Println("  Inventory and stack candidates only; no semantic DSL or HTML map was generated.")
	fmt.Println("  Scope correction is optional. Proceed unchanged when owned source and manifests are")
	fmt.Println("  analyzed while dependencies, generated code, caches, and build output are not.")
	fmt.Println("  Add .vcmignore only to correct a false classification, then rerun inspect.")
	fmt.Println("\nScope actions")
	fmt.Println("  analyze      full source input for adapters and AI investigation")
	fmt.Println("  summarize    retain file/volume evidence without detailed symbol analysis")
	fmt.Println("  externalize  represent a dependency or boundary without reading internals")
	fmt.Println("  ignore       omit irrelevant or derived content")

	counts := make(map[string]int)
	for _, entry := range output.Inventory.Entries {
		if entry.Action != scoping.Analyze {
			key := entry.RuleID
			if key == "" {
				key = string(entry.Action)
			}
			counts[key]++
		}
	}
	type ruleCount struct {
		id    string
		count int
	}
	ordered := make([]ruleCount, 0, len(counts))
	for id, count := range counts {
		ordered = append(ordered, ruleCount{id: id, count: count})
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].count != ordered[j].count {
			return ordered[i].count > ordered[j].count
		}
		return ordered[i].id < ordered[j].id
	})
	if len(ordered) > 0 {
		fmt.Println("\nScope decisions (top rules)")
		for index, item := range ordered {
			if index == 10 {
				break
			}
			fmt.Printf("  %-34s %6d entries\n", item.id, item.count)
		}
	}
	for _, warning := range output.Inventory.Warnings {
		fmt.Printf("\nwarning: %s\n", warning)
	}
	if showEntries {
		fmt.Println("\nNon-analyzed entries")
		for _, entry := range output.Inventory.Entries {
			if entry.Action != scoping.Analyze {
				fmt.Printf("  %-11s %-52s %s\n", entry.Action, entry.Path, entry.RuleID)
			}
		}
	}
}

func runAdapters(arguments []string) int {
	flags := flag.NewFlagSet("adapters", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	jsonOutput := flags.Bool("json", false, "emit JSON descriptors")
	if err := flags.Parse(arguments); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "adapters accepts no positional arguments")
		return 2
	}
	registry, err := adapters.BuiltinRegistry()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	statuses := registry.Statuses()
	if *jsonOutput {
		return encodeJSON(statuses)
	}
	for _, status := range statuses {
		analysis := "not implemented"
		if status.SemanticAnalysis && status.RuntimeAvailable {
			analysis = "ready"
		} else if status.SemanticAnalysis {
			analysis = "runtime missing"
		}
		fmt.Printf("%-24s detect=yes  analyze=%-15s %-15s %s\n",
			status.Descriptor.ID, analysis, status.Descriptor.Support, status.Descriptor.Summary)
	}
	return 0
}

func runAnalyze(arguments []string) int {
	flags := flag.NewFlagSet("analyze", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	outputPath := flags.String("output", "", "evidence JSON output path; '-' writes stdout")
	ruleFile := flags.String("rules", "", "repository rule file; '-' disables it")
	readGitignore := flags.Bool("gitignore", true, "apply Git ignore rules")
	maxHeaderBytes := flags.Int64("max-header-bytes", 8192, "generated marker prefix budget")
	maxFileBytes := flags.Int64("max-file-bytes", 10*1024*1024, "detailed analysis file budget")
	adapterTimeout := flags.Duration("adapter-timeout", adapters.DefaultAdapterTimeout, "maximum time for each analyzer")
	if err := flags.Parse(arguments); err != nil {
		return 2
	}
	if flags.NArg() > 1 {
		fmt.Fprintln(os.Stderr, "analyze accepts at most one repository path")
		return 2
	}
	root := "."
	if flags.NArg() == 1 {
		root = flags.Arg(0)
	}
	if *maxHeaderBytes <= 0 || *maxFileBytes <= 0 || *adapterTimeout <= 0 {
		fmt.Fprintln(os.Stderr, "analysis byte budgets and adapter timeout must be positive")
		return 2
	}
	options := repository.DefaultOptions()
	options.RuleFile = *ruleFile
	options.ReadGitignore = *readGitignore
	options.MaxHeaderBytes = *maxHeaderBytes
	options.MaxFileBytes = *maxFileBytes
	report, err := repository.Scan(context.Background(), root, options)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	registry, err := adapters.BuiltinRegistry()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	bundle, err := registry.AnalyzeWithOptions(context.Background(), report, adapters.AnalysisOptions{AdapterTimeout: *adapterTimeout})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if *outputPath == "-" {
		return encodeJSON(bundle)
	}
	destination := *outputPath
	if destination == "" {
		destination = filepath.Join(report.Root, ".vibecodemap", "generated", "evidence.json")
	} else if !filepath.IsAbs(destination) {
		absolute, err := filepath.Abs(destination)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		destination = absolute
	}
	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := os.WriteFile(destination, data, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("evidence: %s\n", destination)
	fmt.Printf("events:   %d\n", len(bundle.Events))
	var incomplete []string
	for _, run := range bundle.Runs {
		fmt.Printf("  %-24s %-20s %d events", run.AdapterID, run.Status, run.Events)
		if run.Detail != "" {
			fmt.Printf(" — %s", run.Detail)
		}
		fmt.Println()
		if run.Status == "runtime_unavailable" || run.Status == "failed" || run.Status == "timed_out" {
			incomplete = append(incomplete, run.AdapterID)
		}
	}
	if len(incomplete) > 0 {
		fmt.Printf("warning: analyzer evidence is incomplete for %s; use the run details, select a working runtime, or investigate that source directly\n", strings.Join(incomplete, ", "))
	}
	fmt.Println("next: use this evidence plus approved source to author structural DSL; then generate/link quality DSL and run show")
	return 0
}

func runQuality(arguments []string) int {
	flags := flag.NewFlagSet("quality", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	evidencePath := flags.String("evidence", "", "analyzer evidence JSON path")
	outputPath := flags.String("output", "", "quality VCM YAML output path")
	if err := flags.Parse(arguments); err != nil {
		return 2
	}
	if flags.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "quality requires exactly one structural model")
		return 2
	}
	structuralPath, err := filepath.Abs(flags.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	for _, diagnostic := range viewer.ValidateDocument(structuralPath, viewer.ValidationOptions{Kind: "structural"}) {
		if diagnostic.Severity == "error" {
			fmt.Fprintln(os.Stderr, diagnostic.Error())
			return 1
		}
	}
	evidence := *evidencePath
	if evidence == "" {
		evidence = filepath.Join(filepath.Dir(structuralPath), "generated", "evidence.json")
	} else if !filepath.IsAbs(evidence) {
		evidence, err = filepath.Abs(evidence)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	destination := *outputPath
	if destination == "" {
		destination = filepath.Join(filepath.Dir(structuralPath), "quality.vcm.yaml")
	} else if !filepath.IsAbs(destination) {
		destination, err = filepath.Abs(destination)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	data, summary, err := qualitymodel.Generate(structuralPath, evidence, qualitymodel.Options{})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := os.WriteFile(destination, data, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	diagnostics := viewer.ValidateDocument(destination, viewer.ValidationOptions{Kind: "quality", Core: structuralPath})
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == "error" {
			fmt.Fprintln(os.Stderr, diagnostic.Error())
			return 1
		}
	}
	fmt.Printf("quality model: %s\n", destination)
	fmt.Printf("measurements: %d (%d measured artifacts, %d symbol-mapped elements, %d unmapped evidence events)\n",
		summary.Measurements, summary.Artifacts, summary.Elements, summary.Unmapped)
	fmt.Println("next: reference this file as project.inputs.quality_model, then run show")
	return 0
}

func encodeJSON(value any) int {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	return 0
}

func humanBytes(value int64) string {
	const unit = 1024
	if value < unit {
		return fmt.Sprintf("%d B", value)
	}
	divisor, exponent := int64(unit), 0
	for quotient := value / unit; quotient >= unit; quotient /= unit {
		divisor *= unit
		exponent++
	}
	return fmt.Sprintf("%.1f %ciB", float64(value)/float64(divisor), "KMGTPE"[exponent])
}

func runValidate(arguments []string) int {
	flags := flag.NewFlagSet("validate", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	jsonOutput := flags.Bool("json", false, "emit JSON diagnostics")
	kind := flags.String("kind", "auto", "document kind: auto, project, structural, or quality")
	core := flags.String("core", "", "structural model for direct quality validation")
	if err := flags.Parse(arguments); err != nil {
		return 2
	}
	if flags.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "validate requires exactly one VCM document")
		return 2
	}

	diagnostics := viewer.ValidateDocument(flags.Arg(0), viewer.ValidationOptions{Kind: *kind, Core: *core})
	if *jsonOutput {
		if code := encodeJSON(diagnostics); code != 0 {
			return code
		}
	} else if len(diagnostics) == 0 {
		fmt.Printf("valid: %s\n", flags.Arg(0))
	} else {
		for _, diagnostic := range diagnostics {
			fmt.Println(diagnostic.Error())
		}
	}

	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == "error" {
			return 1
		}
	}
	return 0
}
