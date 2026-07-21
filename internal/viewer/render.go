package viewer

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

var (
	//go:embed assets/viewer.html
	viewerDocument string
)

type RenderOptions struct {
	Profile    string
	Output     string
	JSONOutput string
}

type RenderResult struct {
	ViewModel ViewModel
	HTMLPath  string
	JSONPath  string
}

// Render validates, composes, and writes both the inspectable view-model JSON
// and a standalone HTML document. JSON generation is an internal pipeline step,
// not an additional command the user must run.
func Render(projectPath string, options RenderOptions) (RenderResult, error) {
	view, err := Compose(projectPath, ComposeOptions{Profile: options.Profile})
	if err != nil {
		return RenderResult{}, err
	}
	projectPath, err = filepath.Abs(projectPath)
	if err != nil {
		return RenderResult{}, err
	}
	outDir := filepath.Join(filepath.Dir(projectPath), "out")
	htmlPath := options.Output
	if htmlPath == "" {
		htmlPath = filepath.Join(outDir, view.Project.ID+".html")
	}
	htmlPath, err = filepath.Abs(htmlPath)
	if err != nil {
		return RenderResult{}, fmt.Errorf("resolve HTML output: %w", err)
	}
	jsonPath := options.JSONOutput
	if jsonPath == "" {
		jsonPath = strings.TrimSuffix(htmlPath, filepath.Ext(htmlPath)) + ".view.json"
	}
	jsonPath, err = filepath.Abs(jsonPath)
	if err != nil {
		return RenderResult{}, fmt.Errorf("resolve JSON output: %w", err)
	}
	if samePath(htmlPath, jsonPath) {
		return RenderResult{}, fmt.Errorf("HTML and JSON outputs must use different paths: %s", htmlPath)
	}
	if err := os.MkdirAll(filepath.Dir(htmlPath), 0o755); err != nil {
		return RenderResult{}, fmt.Errorf("create HTML output directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(jsonPath), 0o755); err != nil {
		return RenderResult{}, fmt.Errorf("create JSON output directory: %w", err)
	}

	jsonData, err := json.MarshalIndent(view, "", "  ")
	if err != nil {
		return RenderResult{}, fmt.Errorf("encode view model: %w", err)
	}
	jsonData = append(jsonData, '\n')
	document, err := instantiateViewer(view.Project.Name, jsonData)
	if err != nil {
		return RenderResult{}, err
	}
	if err := os.WriteFile(jsonPath, jsonData, 0o644); err != nil {
		return RenderResult{}, fmt.Errorf("write view model: %w", err)
	}

	if err := os.WriteFile(htmlPath, []byte(document), 0o644); err != nil {
		return RenderResult{}, fmt.Errorf("write HTML map: %w", err)
	}
	return RenderResult{ViewModel: view, HTMLPath: htmlPath, JSONPath: jsonPath}, nil
}

func instantiateViewer(title string, jsonData []byte) (string, error) {
	const dataPlaceholder = "__VCM_DATA__"
	prefix, suffix, found := strings.Cut(viewerDocument, dataPlaceholder)
	if !found || strings.Contains(suffix, dataPlaceholder) {
		return "", fmt.Errorf("viewer template must contain exactly one %s placeholder", dataPlaceholder)
	}
	escapedTitle := html.EscapeString(title)
	prefix = strings.ReplaceAll(prefix, "__VCM_TITLE__", escapedTitle)
	suffix = strings.ReplaceAll(suffix, "__VCM_TITLE__", escapedTitle)
	return prefix + string(jsonData) + suffix, nil
}

func samePath(left, right string) bool {
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		if strings.EqualFold(filepath.Clean(left), filepath.Clean(right)) {
			return true
		}
	} else if filepath.Clean(left) == filepath.Clean(right) {
		return true
	}
	leftInfo, leftErr := os.Stat(left)
	rightInfo, rightErr := os.Stat(right)
	return leftErr == nil && rightErr == nil && os.SameFile(leftInfo, rightInfo)
}

// Open launches the platform browser for a generated local HTML document.
func Open(path string) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.Command("open", absolute)
	case "windows":
		command = exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", absolute)
	default:
		command = exec.Command("xdg-open", absolute)
	}
	if err := command.Run(); err != nil {
		return fmt.Errorf("open browser: %w", err)
	}
	return nil
}
