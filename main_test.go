package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestVersionMetadataWithoutOBS(t *testing.T) {
	name := "obs-cli"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	binary := filepath.Join(t.TempDir(), name)
	if out, err := exec.Command("go", "build", "-ldflags", "-X main.version=1.2.3", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build versioned CLI: %v\n%s", err, out)
	}
	configHome := t.TempDir()
	cmd := exec.Command(binary, "--version", "--host", "127.0.0.1", "--port", "1")
	cmd.Env = append(os.Environ(), "XDG_CONFIG_HOME="+configHome)
	out, err := cmd.CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "obs-cli version 1.2.3" {
		t.Fatalf("version should bypass connection initialization: %v, %q", err, out)
	}
	if _, err := os.Stat(filepath.Join(configHome, "obs-cli")); !os.IsNotExist(err) {
		t.Fatalf("version created or accessed first-run configuration: %v", err)
	}
}
