package adapters

import (
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

//go:embed python_adapter.py
var pythonAdapterSource string

type pythonAnalyzer struct {
	stackDetector
}

func newPythonAnalyzer() pythonAnalyzer {
	return pythonAnalyzer{stackDetector: stackDetector{
		descriptor: Descriptor{
			ID: "python-ast-v0", Version: "0.1", Languages: []string{"python"}, Stacks: []string{"python"},
			Capabilities: []Capability{Artifacts, Symbols, Imports, Calls, Effects, Complexity, Entrypoints},
			Support:      Prototype,
			Summary:      "Go-orchestrated Python AST evidence adapter; static candidates are not runtime observations.",
		},
		detect: detectPython,
	}}
}

func pythonCommand() (string, error) {
	for _, candidate := range []string{"python3", "python"} {
		path, err := exec.LookPath(candidate)
		if err != nil {
			continue
		}
		check := exec.Command(path, "-c", "import sys; raise SystemExit(0 if sys.version_info >= (3, 10) else 1)")
		if err := check.Run(); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("Python 3.10 or newer runtime not found in PATH")
}

func (analyzer pythonAnalyzer) RuntimeStatus() (bool, string) {
	command, err := pythonCommand()
	if err != nil {
		return false, err.Error()
	}
	return true, command
}

func (analyzer pythonAnalyzer) Analyze(ctx context.Context, request AnalyzeRequest, sink Sink) error {
	if sink == nil {
		return fmt.Errorf("python analyzer sink is required")
	}
	if request.AdapterID != analyzer.Descriptor().ID {
		return fmt.Errorf("request adapter %q does not match %q", request.AdapterID, analyzer.Descriptor().ID)
	}
	commandPath, err := pythonCommand()
	if err != nil {
		return err
	}
	requestData, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("encode Python adapter request: %w", err)
	}
	command := exec.CommandContext(ctx, commandPath, "-c", pythonAdapterSource)
	command.Stdin = bytes.NewReader(requestData)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	stdout, err := command.StdoutPipe()
	if err != nil {
		return fmt.Errorf("open Python adapter output: %w", err)
	}
	if err := command.Start(); err != nil {
		return fmt.Errorf("start Python adapter: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 32*1024*1024)
	for scanner.Scan() {
		var event EvidenceEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			_ = command.Process.Kill()
			_ = command.Wait()
			return fmt.Errorf("decode Python adapter event: %w", err)
		}
		if err := event.Validate(); err != nil {
			_ = command.Process.Kill()
			_ = command.Wait()
			return err
		}
		if event.Producer != analyzer.Descriptor().ID {
			_ = command.Process.Kill()
			_ = command.Wait()
			return fmt.Errorf("Python adapter event %q has unexpected producer %q", event.ID, event.Producer)
		}
		if err := sink.Emit(ctx, event); err != nil {
			_ = command.Process.Kill()
			_ = command.Wait()
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		_ = command.Process.Kill()
		_ = command.Wait()
		return fmt.Errorf("read Python adapter output: %w", err)
	}
	if err := command.Wait(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return fmt.Errorf("Python adapter failed: %s", detail)
		}
		return fmt.Errorf("Python adapter failed: %w", err)
	}
	return nil
}
