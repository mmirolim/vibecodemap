package adapters

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"

	"github.com/mmirolim/vibecodemap/internal/scoping"
)

type goAnalyzer struct {
	stackDetector
}

func newGoAnalyzer() goAnalyzer {
	return goAnalyzer{stackDetector: stackDetector{
		descriptor: Descriptor{
			ID: "go-ast-v0", Version: "0.1", Languages: []string{"go"}, Stacks: []string{"go-module"},
			Capabilities: []Capability{Artifacts, Symbols, Imports, Calls, Types, Effects, Complexity, Tests, Entrypoints},
			Support:      Prototype,
			Summary:      "In-process Go AST evidence analyzer; calls and effects remain static candidates.",
		},
		detect: detectGo,
	}}
}

func (analyzer goAnalyzer) Analyze(ctx context.Context, request AnalyzeRequest, sink Sink) error {
	if sink == nil {
		return fmt.Errorf("Go analyzer sink is required")
	}
	for _, input := range request.Files {
		if !strings.HasSuffix(strings.ToLower(input.Path), ".go") {
			continue
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		payload, lineCount, kind, err := analyzeGoFile(request.Root, input)
		if err != nil {
			return fmt.Errorf("analyze Go source %s: %w", input.Path, err)
		}
		raw, err := marshalPayload(payload)
		if err != nil {
			return err
		}
		source := &SourceLocation{Path: input.Path}
		if lineCount > 0 {
			source.Line = 1
			source.EndLine = lineCount
		}
		if err := sink.Emit(ctx, EvidenceEvent{
			Schema: EventSchema, ID: evidenceID("go", input.Path), Kind: kind,
			Subject: input.Path, Producer: analyzer.Descriptor().ID, Confidence: 1,
			Source: source, Payload: raw,
		}); err != nil {
			return err
		}
	}
	return nil
}

func analyzeGoFile(root string, input FileInput) (map[string]any, int, string, error) {
	if input.Action == scoping.Summarize {
		return map[string]any{
			"path": input.Path, "language": "go", "size_bytes": input.Size,
			"summary_only": true, "reason": "central scope classified this file as summarize",
		}, 0, "go.file_summary", nil
	}
	data, err := readScopedSource(root, input.Path)
	if err != nil {
		return nil, 0, "", err
	}
	lineCount := sourceLineCount(data)
	fileset := token.NewFileSet()
	file, parseErr := parser.ParseFile(fileset, input.Path, data, parser.ParseComments|parser.SkipObjectResolution)
	if parseErr != nil {
		return map[string]any{
			"path": input.Path, "language": "go", "parse_error": parseErr.Error(),
			"limitations": []string{"No semantic facts were extracted from this syntactically invalid file"},
		}, lineCount, "go.parse_error", nil
	}

	imports := make([]string, 0, len(file.Imports))
	for _, item := range file.Imports {
		value, err := strconv.Unquote(item.Path.Value)
		if err == nil {
			imports = append(imports, value)
		}
	}
	imports, _ = uniqueSortedLimited(imports, 500)

	var symbols []map[string]any
	var routes []map[string]any
	var entrypoints []map[string]any
	allEffects := make(map[string]int)
	allCalls := make([]string, 0)
	complexityTotal := 0
	complexityMax := 0
	nestingMax := 0
	functionCount := 0
	maxFunctionLines := 0

	for _, declaration := range file.Decls {
		switch item := declaration.(type) {
		case *ast.GenDecl:
			if item.Tok != token.TYPE {
				continue
			}
			for _, spec := range item.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				kind := "type"
				switch typeSpec.Type.(type) {
				case *ast.StructType:
					kind = "struct"
				case *ast.InterfaceType:
					kind = "interface"
				}
				symbols = append(symbols, map[string]any{
					"name": typeSpec.Name.Name, "kind": kind,
					"line":     fileset.Position(typeSpec.Pos()).Line,
					"end_line": fileset.Position(typeSpec.End()).Line,
				})
			}
		case *ast.FuncDecl:
			functionCount++
			start := fileset.Position(item.Pos()).Line
			end := fileset.Position(item.End()).Line
			lines := end - start + 1
			if lines > maxFunctionLines {
				maxFunctionLines = lines
			}
			facts := inspectGoFunction(item.Body, fileset)
			complexityTotal += facts.complexity
			if facts.complexity > complexityMax {
				complexityMax = facts.complexity
			}
			if facts.maxNesting > nestingMax {
				nestingMax = facts.maxNesting
			}
			for effect, count := range facts.effects {
				allEffects[effect] += count
			}
			allCalls = append(allCalls, facts.calls...)
			receiver := goReceiver(item)
			name := item.Name.Name
			qualified := name
			if receiver != "" {
				qualified = receiver + "." + name
			}
			symbols = append(symbols, map[string]any{
				"name": qualified, "kind": "function", "line": start, "end_line": end,
				"receiver": receiver, "complexity": facts.complexity,
				"max_nesting": facts.maxNesting, "calls": facts.calls,
				"effects":     facts.effects,
				"concurrency": map[string]int{"go_statements": facts.goStatements, "channel_sends": facts.channelSends, "channel_receives": facts.channelReceives},
			})
			if file.Name.Name == "main" && item.Recv == nil && name == "main" {
				entrypoints = append(entrypoints, map[string]any{"kind": "process", "symbol": name, "line": start})
			}
			if strings.HasSuffix(input.Path, "_test.go") && item.Recv == nil && strings.HasPrefix(name, "Test") {
				entrypoints = append(entrypoints, map[string]any{"kind": "test", "symbol": name, "line": start})
			}
			for _, route := range facts.routes {
				routes = append(routes, route)
			}
		}
	}
	allCalls, callsTruncated := uniqueSortedLimited(allCalls, 1000)
	return map[string]any{
		"path": input.Path, "language": "go", "package": file.Name.Name,
		"lines": lineCount, "imports": imports, "symbols": symbols,
		"calls": allCalls, "calls_truncated": callsTruncated,
		"routes": routes, "entrypoints": entrypoints,
		"quality": map[string]any{
			"function_count": functionCount, "max_function_lines": maxFunctionLines,
			"complexity_total": complexityTotal, "complexity_max": complexityMax,
			"nesting_max": nestingMax, "effects": allEffects,
		},
		"limitations": []string{
			"Go parser/AST only; package loading, types, build tags, generics instantiation, and dynamic dispatch are unresolved",
			"Calls and effects are static candidates, not observed runtime behavior",
		},
	}, lineCount, "go.file_analysis", nil
}

type goFunctionFacts struct {
	complexity      int
	maxNesting      int
	goStatements    int
	channelSends    int
	channelReceives int
	calls           []string
	effects         map[string]int
	routes          []map[string]any
}

func inspectGoFunction(body *ast.BlockStmt, fileset *token.FileSet) goFunctionFacts {
	facts := goFunctionFacts{complexity: 1, effects: make(map[string]int)}
	if body == nil {
		return facts
	}
	depth := 0
	var decisions []bool
	ast.Inspect(body, func(node ast.Node) bool {
		if node == nil {
			if len(decisions) > 0 {
				if decisions[len(decisions)-1] {
					depth--
				}
				decisions = decisions[:len(decisions)-1]
			}
			return true
		}
		decision := goDecisionNode(node)
		decisions = append(decisions, decision)
		if decision {
			facts.complexity++
			depth++
			if depth > facts.maxNesting {
				facts.maxNesting = depth
			}
		}
		switch item := node.(type) {
		case *ast.BinaryExpr:
			if item.Op == token.LAND || item.Op == token.LOR {
				facts.complexity++
			}
		case *ast.GoStmt:
			facts.goStatements++
		case *ast.SendStmt:
			facts.channelSends++
		case *ast.UnaryExpr:
			if item.Op == token.ARROW {
				facts.channelReceives++
			}
		case *ast.CallExpr:
			name := goExpressionName(item.Fun)
			if name != "" {
				facts.calls = append(facts.calls, name)
				for _, effect := range classifyGoCall(name) {
					facts.effects[effect]++
				}
				if route := goRouteCandidate(name, item.Args, fileset.Position(item.Pos()).Line); route != nil {
					facts.routes = append(facts.routes, route)
				}
			}
		}
		return true
	})
	facts.calls, _ = uniqueSortedLimited(facts.calls, 500)
	return facts
}

func goDecisionNode(node ast.Node) bool {
	switch node.(type) {
	case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.CaseClause, *ast.CommClause, *ast.TypeSwitchStmt, *ast.SelectStmt:
		return true
	default:
		return false
	}
}

func goExpressionName(expression ast.Expr) string {
	switch item := expression.(type) {
	case *ast.Ident:
		return item.Name
	case *ast.SelectorExpr:
		prefix := goExpressionName(item.X)
		if prefix == "" {
			return item.Sel.Name
		}
		return prefix + "." + item.Sel.Name
	case *ast.IndexExpr:
		return goExpressionName(item.X)
	case *ast.IndexListExpr:
		return goExpressionName(item.X)
	case *ast.ParenExpr:
		return goExpressionName(item.X)
	case *ast.StarExpr:
		return goExpressionName(item.X)
	default:
		return ""
	}
}

func goReceiver(function *ast.FuncDecl) string {
	if function.Recv == nil || len(function.Recv.List) == 0 {
		return ""
	}
	return strings.TrimPrefix(goExpressionName(function.Recv.List[0].Type), "*")
}

func classifyGoCall(name string) []string {
	var effects []string
	switch {
	case name == "os.ReadFile" || name == "os.Open" || name == "io.ReadAll":
		effects = append(effects, "filesystem.read")
	case name == "os.WriteFile" || name == "os.Create" || name == "os.Remove" || name == "os.RemoveAll" || name == "os.Rename" || name == "os.Mkdir" || name == "os.MkdirAll":
		effects = append(effects, "filesystem.write")
	}
	// Without go/types, an arbitrary receiver named x.Do cannot be proven to be
	// net/http. In particular, sync.Once.Do is common and must not become a
	// network effect. Keep only package-qualified calls that the AST can identify
	// without guessing the receiver type.
	if name == "http.Get" || name == "http.Post" || name == "http.PostForm" || name == "http.DefaultClient.Do" || name == "http.Client.Do" {
		effects = append(effects, "network.call")
	}
	if strings.HasSuffix(name, ".Query") || strings.HasSuffix(name, ".QueryContext") || strings.HasSuffix(name, ".QueryRow") || strings.HasSuffix(name, ".QueryRowContext") {
		effects = append(effects, "database.read")
	}
	if strings.HasSuffix(name, ".Exec") || strings.HasSuffix(name, ".ExecContext") || strings.HasSuffix(name, ".Commit") || strings.HasSuffix(name, ".Rollback") {
		effects = append(effects, "database.write")
	}
	if name == "exec.Command" || name == "exec.CommandContext" {
		effects = append(effects, "process.spawn")
	}
	if strings.HasPrefix(name, "log.") || strings.HasPrefix(name, "slog.") || strings.Contains(name, ".Logger.") {
		effects = append(effects, "telemetry.log")
	}
	return effects
}

func goRouteCandidate(name string, arguments []ast.Expr, line int) map[string]any {
	method := ""
	switch {
	case name == "http.Handle" || name == "http.HandleFunc" || strings.HasSuffix(name, ".Handle") || strings.HasSuffix(name, ".HandleFunc"):
		method = "ANY"
	case strings.HasSuffix(name, ".GET") || strings.HasSuffix(name, ".Get"):
		method = "GET"
	case strings.HasSuffix(name, ".POST") || strings.HasSuffix(name, ".Post"):
		method = "POST"
	case strings.HasSuffix(name, ".PUT") || strings.HasSuffix(name, ".Put"):
		method = "PUT"
	case strings.HasSuffix(name, ".PATCH") || strings.HasSuffix(name, ".Patch"):
		method = "PATCH"
	case strings.HasSuffix(name, ".DELETE") || strings.HasSuffix(name, ".Delete"):
		method = "DELETE"
	default:
		return nil
	}
	path := "<dynamic>"
	if len(arguments) > 0 {
		if literal, ok := arguments[0].(*ast.BasicLit); ok && literal.Kind == token.STRING {
			if value, err := strconv.Unquote(literal.Value); err == nil {
				path = value
			}
		}
	}
	return map[string]any{"registration": name, "method": method, "path": path, "line": line}
}
