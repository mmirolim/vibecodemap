package adapters

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestPythonRuntimeProbeTimesOut(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture uses a POSIX shell script")
	}
	path := filepath.Join(t.TempDir(), "slow-python")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nsleep 10\n"), 0o700); err != nil {
		t.Fatal(err)
	}

	started := time.Now()
	_, err := findPythonCommand([]string{path}, 20*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "startup/import probe exceeded 20ms") {
		t.Fatalf("error = %v", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("runtime probe did not honor timeout: %s", elapsed)
	}
}
