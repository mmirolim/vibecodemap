package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/mmirolim/vibecodemap/internal/adapters"
	"github.com/mmirolim/vibecodemap/internal/projectdsl"
	"github.com/mmirolim/vibecodemap/internal/repository"
	"github.com/mmirolim/vibecodemap/internal/scoping"
)

const usage = `VibeCodeMap repository model tool

Usage:
  vibecodemap describe            print the complete human DSL grammar
  vibecodemap schema              print the complete JSON Schema
  vibecodemap validate [flags] PROJECT.vcm.yaml
  vibecodemap inspect [flags] [REPOSITORY]
  vibecodemap adapters [-json]

Validate flags:
  -json                          emit machine-readable diagnostics

Validation reports YAML syntax, schema, cross-record, district-code, band-order,
and structural-model reference errors with source path, line, and column where
the parser provides them.

Inspect flags:
  -json                          emit the full machine-readable inventory
  -entries                       print every non-analyzed entry in text mode
  -rules FILE                    use FILE instead of REPOSITORY/.vcmignore;
                                 use -rules=- to disable repository rules
  -gitignore                     apply exact Git ignore rules (default true)
  -max-header-bytes N            generated-marker prefix budget (default 8192)
  -max-file-bytes N              detailed-analysis file budget (default 10 MiB)
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	switch os.Args[1] {
	case "describe", "grammar":
		_, _ = os.Stdout.Write(projectdsl.Grammar())
	case "schema":
		_, _ = os.Stdout.Write(projectdsl.Schema())
	case "validate":
		os.Exit(runValidate(os.Args[2:]))
	case "inspect", "inventory":
		os.Exit(runInspect(os.Args[2:]))
	case "adapters":
		os.Exit(runAdapters(os.Args[2:]))
	case "help", "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}
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
	descriptors := registry.Descriptors()
	if *jsonOutput {
		return encodeJSON(descriptors)
	}
	for _, descriptor := range descriptors {
		fmt.Printf("%-24s %-15s %s\n", descriptor.ID, descriptor.Support, descriptor.Summary)
	}
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
	if err := flags.Parse(arguments); err != nil {
		return 2
	}
	if flags.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "validate requires exactly one project manifest")
		return 2
	}

	diagnostics := projectdsl.ValidateFile(flags.Arg(0))
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
