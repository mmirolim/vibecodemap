package adapters

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/mmirolim/vibecodemap/internal/scoping"
)

type javascriptAnalyzer struct {
	stackDetector
	language   string
	extensions map[string]struct{}
}

func newTypeScriptAnalyzer() javascriptAnalyzer {
	return javascriptAnalyzer{
		stackDetector: stackDetector{descriptor: Descriptor{
			ID: "typescript-source-v0", Version: "0.1", Languages: []string{"typescript"}, Stacks: []string{"typescript"},
			Capabilities: []Capability{Artifacts, Symbols, Imports, Calls, Effects, Complexity, Tests, Entrypoints},
			Support:      Prototype,
			Summary:      "In-process TypeScript lexical evidence analyzer; compiler/type resolution is not implemented.",
		}, detect: detectTypeScript},
		language: "typescript", extensions: extensionSet(".ts", ".tsx", ".mts", ".cts"),
	}
}

func newJavaScriptAnalyzer() javascriptAnalyzer {
	return javascriptAnalyzer{
		stackDetector: stackDetector{descriptor: Descriptor{
			ID: "javascript-source-v0", Version: "0.1", Languages: []string{"javascript"}, Stacks: []string{"javascript"},
			Capabilities: []Capability{Artifacts, Symbols, Imports, Calls, Effects, Complexity, Tests, Entrypoints},
			Support:      Prototype,
			Summary:      "In-process JavaScript lexical evidence analyzer; dynamic dispatch and bundler resolution are unresolved.",
		}, detect: detectJavaScript},
		language: "javascript", extensions: extensionSet(".js", ".jsx", ".mjs", ".cjs"),
	}
}

func extensionSet(values ...string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func (analyzer javascriptAnalyzer) Analyze(ctx context.Context, request AnalyzeRequest, sink Sink) error {
	if sink == nil {
		return fmt.Errorf("%s analyzer sink is required", analyzer.language)
	}
	for _, input := range request.Files {
		if _, supported := analyzer.extensions[strings.ToLower(filepath.Ext(input.Path))]; !supported {
			continue
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		payload, lineCount, kind, err := analyzeJavaScriptFile(request.Root, input, analyzer.language)
		if err != nil {
			return fmt.Errorf("analyze %s source %s: %w", analyzer.language, input.Path, err)
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
		prefix := "js"
		if analyzer.language == "typescript" {
			prefix = "ts"
		}
		if err := sink.Emit(ctx, EvidenceEvent{
			Schema: EventSchema, ID: evidenceID(prefix, input.Path), Kind: kind,
			Subject: input.Path, Producer: analyzer.Descriptor().ID, Confidence: 0.85,
			Source: source, Payload: raw,
		}); err != nil {
			return err
		}
	}
	return nil
}

var (
	jsImportPattern      = regexp.MustCompile(`^\s*import\s+(?:type\s+)?(?:[^'\"]*?\s+from\s+)?['\"]([^'\"]+)['\"]`)
	jsExportFromPattern  = regexp.MustCompile(`^\s*export\s+[^'\"]*?\s+from\s+['\"]([^'\"]+)['\"]`)
	jsRequirePattern     = regexp.MustCompile(`\b(?:require|import)\s*\(\s*['\"]([^'\"]+)['\"]\s*\)`)
	jsFunctionPattern    = regexp.MustCompile(`\b(async\s+)?function\s+([A-Za-z_$][A-Za-z0-9_$]*)`)
	jsClassPattern       = regexp.MustCompile(`\bclass\s+([A-Za-z_$][A-Za-z0-9_$]*)`)
	jsArrowPattern       = regexp.MustCompile(`\b(?:const|let|var)\s+([A-Za-z_$][A-Za-z0-9_$]*)\s*=\s*(async\s+)?(?:\([^)]*\)|[A-Za-z_$][A-Za-z0-9_$]*)\s*=>`)
	jsCallPattern        = regexp.MustCompile(`([A-Za-z_$][A-Za-z0-9_$]*(?:\.[A-Za-z_$][A-Za-z0-9_$]*)*)\s*\(`)
	jsDecisionPattern    = regexp.MustCompile(`\b(?:if|for|while|case|catch|switch)\b`)
	jsRoutePattern       = regexp.MustCompile(`([A-Za-z_$][A-Za-z0-9_$]*(?:\.[A-Za-z_$][A-Za-z0-9_$]*)*)\.(get|post|put|patch|delete|use)\s*\(\s*["'\x60]([^"'\x60]+)["'\x60]`)
	jsHTTPExportPattern  = regexp.MustCompile(`\bexport\s+(?:async\s+)?(?:function|const)\s+(GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS)\b`)
	jsIgnoredCallKeyword = map[string]struct{}{
		"if": {}, "for": {}, "while": {}, "switch": {}, "catch": {}, "function": {},
		"typeof": {}, "delete": {}, "new": {}, "return": {}, "throw": {}, "import": {},
	}
)

func analyzeJavaScriptFile(root string, input FileInput, language string) (map[string]any, int, string, error) {
	if input.Action == scoping.Summarize {
		return map[string]any{
			"path": input.Path, "language": language, "size_bytes": input.Size,
			"summary_only": true, "reason": "central scope classified this file as summarize",
		}, 0, language + ".file_summary", nil
	}
	data, err := readScopedSource(root, input.Path)
	if err != nil {
		return nil, 0, "", err
	}
	text := string(data)
	lines := strings.Split(text, "\n")
	if len(data) > 0 && data[len(data)-1] == '\n' {
		lines = lines[:len(lines)-1]
	}
	imports := make([]string, 0)
	symbols := make([]map[string]any, 0)
	calls := make([]string, 0)
	routes := make([]map[string]any, 0)
	entrypoints := make([]map[string]any, 0)
	effects := make(map[string]int)
	decisionTokens := 0
	braceDepth := 0
	maxBraceDepth := 0
	blockComment := false

	for index, original := range lines {
		lineNumber := index + 1
		code, nextBlock := stripJavaScriptComments(original, blockComment)
		blockComment = nextBlock
		if match := jsImportPattern.FindStringSubmatch(code); len(match) > 1 {
			imports = append(imports, match[1])
		}
		if match := jsExportFromPattern.FindStringSubmatch(code); len(match) > 1 {
			imports = append(imports, match[1])
		}
		for _, match := range jsRequirePattern.FindAllStringSubmatch(code, -1) {
			imports = append(imports, match[1])
		}

		masked := maskJavaScriptStrings(code)
		declared := make(map[string]struct{})
		for _, match := range jsFunctionPattern.FindAllStringSubmatch(masked, -1) {
			name := match[2]
			declared[name] = struct{}{}
			symbols = append(symbols, jsSymbol(name, "function", lineNumber, strings.TrimSpace(match[1]) != ""))
			if isLikelyJavaScriptComponent(name, input.Path) {
				symbols[len(symbols)-1]["ui_component_candidate"] = true
			}
		}
		for _, match := range jsArrowPattern.FindAllStringSubmatch(masked, -1) {
			name := match[1]
			declared[name] = struct{}{}
			symbols = append(symbols, jsSymbol(name, "arrow_function", lineNumber, strings.TrimSpace(match[2]) != ""))
			if isLikelyJavaScriptComponent(name, input.Path) {
				symbols[len(symbols)-1]["ui_component_candidate"] = true
			}
		}
		for _, match := range jsClassPattern.FindAllStringSubmatch(masked, -1) {
			name := match[1]
			declared[name] = struct{}{}
			symbols = append(symbols, map[string]any{"name": name, "kind": "class", "line": lineNumber})
		}
		for _, match := range jsCallPattern.FindAllStringSubmatch(masked, -1) {
			name := match[1]
			if _, ignored := jsIgnoredCallKeyword[name]; ignored {
				continue
			}
			if _, isDeclaration := declared[name]; isDeclaration {
				continue
			}
			calls = append(calls, name)
			for _, effect := range classifyJavaScriptCall(name) {
				effects[effect]++
			}
			if name == "ReactDOM.createRoot" || name == "createRoot" || name == "app.listen" || name == "server.listen" {
				entrypoints = append(entrypoints, map[string]any{"kind": "startup_candidate", "call": name, "line": lineNumber})
			}
		}
		for _, match := range jsRoutePattern.FindAllStringSubmatch(code, -1) {
			routes = append(routes, map[string]any{
				"registration": match[1] + "." + match[2], "method": strings.ToUpper(match[2]),
				"path": match[3], "line": lineNumber,
			})
		}
		for _, match := range jsHTTPExportPattern.FindAllStringSubmatch(masked, -1) {
			entrypoints = append(entrypoints, map[string]any{"kind": "http_handler_candidate", "method": match[1], "line": lineNumber})
		}
		decisionTokens += len(jsDecisionPattern.FindAllString(masked, -1))
		decisionTokens += strings.Count(masked, "&&") + strings.Count(masked, "||")
		braceDepth += strings.Count(masked, "{") - strings.Count(masked, "}")
		if braceDepth > maxBraceDepth {
			maxBraceDepth = braceDepth
		}
		if braceDepth < 0 {
			braceDepth = 0
		}
	}
	if isJavaScriptTestPath(input.Path) {
		entrypoints = append(entrypoints, map[string]any{"kind": "test_file", "line": 1})
	}
	imports, importsTruncated := uniqueSortedLimited(imports, 500)
	calls, callsTruncated := uniqueSortedLimited(calls, 1000)
	sort.Slice(symbols, func(i, j int) bool {
		left, right := symbols[i]["line"].(int), symbols[j]["line"].(int)
		if left != right {
			return left < right
		}
		return fmt.Sprint(symbols[i]["name"]) < fmt.Sprint(symbols[j]["name"])
	})
	return map[string]any{
		"path": input.Path, "language": language, "lines": len(lines),
		"imports": imports, "imports_truncated": importsTruncated,
		"symbols": symbols, "calls": calls, "calls_truncated": callsTruncated,
		"routes": routes, "entrypoints": entrypoints,
		"quality": map[string]any{
			"symbol_candidates": len(symbols), "decision_tokens": decisionTokens,
			"brace_nesting_max": maxBraceDepth, "effects": effects,
		},
		"limitations": []string{
			"Dependency-free lexical source analysis, not a JavaScript/TypeScript compiler AST",
			"Multiline declarations, aliases, types, JSX structure, scope, overloads, and dynamic dispatch may be incomplete",
			"Calls, routes, components, and effects are static candidates, not observed runtime behavior",
		},
	}, len(lines), language + ".file_analysis", nil
}

func jsSymbol(name, kind string, line int, async bool) map[string]any {
	result := map[string]any{"name": name, "kind": kind, "line": line, "execution": "sync"}
	if async {
		result["execution"] = "async_syntax"
	}
	return result
}

func classifyJavaScriptCall(name string) []string {
	lower := strings.ToLower(name)
	var effects []string
	if lower == "fetch" || strings.HasPrefix(lower, "axios.") || lower == "http.request" || lower == "https.request" || strings.HasSuffix(lower, ".fetch") {
		effects = append(effects, "network.call")
	}
	if strings.HasPrefix(lower, "fs.read") || strings.HasPrefix(lower, "fspromises.read") {
		effects = append(effects, "filesystem.read")
	}
	if strings.HasPrefix(lower, "fs.write") || strings.HasPrefix(lower, "fs.append") || strings.HasPrefix(lower, "fs.rm") || strings.HasPrefix(lower, "fs.unlink") || strings.HasPrefix(lower, "fs.rename") || strings.HasPrefix(lower, "fspromises.write") {
		effects = append(effects, "filesystem.write")
	}
	dataCall := strings.HasPrefix(lower, "db.") || strings.HasPrefix(lower, "prisma.") || strings.Contains(lower, ".repository.") || strings.Contains(lower, ".model.")
	if dataCall && (strings.HasSuffix(lower, ".findmany") || strings.HasSuffix(lower, ".findunique") || strings.HasSuffix(lower, ".findfirst") || strings.HasSuffix(lower, ".query")) {
		effects = append(effects, "database.read")
	}
	if dataCall && (strings.HasSuffix(lower, ".create") || strings.HasSuffix(lower, ".update") || strings.HasSuffix(lower, ".delete") || strings.HasSuffix(lower, ".upsert") || strings.HasSuffix(lower, ".insert")) {
		effects = append(effects, "database.write_candidate")
	}
	if lower == "localstorage.setitem" || lower == "sessionstorage.setitem" || lower == "indexeddb.open" {
		effects = append(effects, "browser_storage.write")
	}
	if lower == "child_process.spawn" || lower == "child_process.exec" || lower == "child_process.execfile" {
		effects = append(effects, "process.spawn")
	}
	if strings.HasPrefix(lower, "console.") {
		effects = append(effects, "telemetry.log")
	}
	return effects
}

func stripJavaScriptComments(line string, blockComment bool) (string, bool) {
	var builder strings.Builder
	quote := byte(0)
	escaped := false
	for index := 0; index < len(line); index++ {
		current := line[index]
		next := byte(0)
		if index+1 < len(line) {
			next = line[index+1]
		}
		if blockComment {
			if current == '*' && next == '/' {
				blockComment = false
				index++
			}
			continue
		}
		if quote != 0 {
			builder.WriteByte(current)
			if escaped {
				escaped = false
			} else if current == '\\' {
				escaped = true
			} else if current == quote {
				quote = 0
			}
			continue
		}
		if current == '/' && next == '/' {
			break
		}
		if current == '/' && next == '*' {
			blockComment = true
			index++
			continue
		}
		if current == '\'' || current == '"' || current == '`' {
			quote = current
		}
		builder.WriteByte(current)
	}
	return builder.String(), blockComment
}

func maskJavaScriptStrings(line string) string {
	data := []byte(line)
	quote := byte(0)
	escaped := false
	for index, current := range data {
		if quote == 0 {
			if current == '\'' || current == '"' || current == '`' {
				quote = current
				data[index] = ' '
			}
			continue
		}
		data[index] = ' '
		if escaped {
			escaped = false
		} else if current == '\\' {
			escaped = true
		} else if current == quote {
			quote = 0
		}
	}
	return string(data)
}

func isLikelyJavaScriptComponent(name, path string) bool {
	if name == "" || name[0] < 'A' || name[0] > 'Z' {
		return false
	}
	extension := strings.ToLower(filepath.Ext(path))
	return extension == ".jsx" || extension == ".tsx"
}

func isJavaScriptTestPath(path string) bool {
	lower := strings.ToLower(filepath.ToSlash(path))
	return strings.Contains(lower, ".test.") || strings.Contains(lower, ".spec.") || strings.Contains(lower, "/__tests__/")
}
