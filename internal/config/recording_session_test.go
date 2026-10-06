package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadRecordingSessionDefaultsAndOverrides(t *testing.T) {
	t.Setenv("OBS_CLI_HOST", "")
	t.Setenv("OBS_CLI_PORT", "")
	t.Setenv("OBS_CLI_PASSWORD", "")
	path := filepath.Join(t.TempDir(), "user-config.toml")
	if err := os.WriteFile(path, []byte(`host = "localhost"
[recording_session]
base_directory = "~/Videos"
scene_collection = "Sessions"
obs_profile = "Capture"
scene = "Program"
source_names = ["Screen", "Camera"]
filter_name = "Capture source"
`), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	want := RecordingSessionConfig{"~/Videos", "Sessions", "Capture", "Program", []string{"Screen", "Camera"}, "Capture source"}
	if !reflect.DeepEqual(cfg.RecordingSession, want) {
		t.Fatalf("overrides not loaded: %+v", cfg.RecordingSession)
	}
	if err := os.WriteFile(path, []byte("[recording_session]\nbase_directory = \"~/Videos\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err = Load(path)
	if err != nil {
		t.Fatal(err)
	}
	want = DefaultRecordingSessionConfig()
	want.BaseDirectory = "~/Videos"
	if !reflect.DeepEqual(cfg.RecordingSession, want) {
		t.Fatalf("partial config lost defaults: %+v", cfg.RecordingSession)
	}
	cfg, err = Load(filepath.Join(t.TempDir(), "nonexistent.toml"))
	if err != nil || !reflect.DeepEqual(cfg.RecordingSession, DefaultRecordingSessionConfig()) {
		t.Fatalf("default config = %+v, %v", cfg, err)
	}
}

func TestRecordingSessionPathExpansion(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("OBS_SESSION_TEST_BASE", filepath.Join(home, "Other"))
	for path, want := range map[string]string{
		"~/Movies":               filepath.Join(home, "Movies"),
		"$OBS_SESSION_TEST_BASE": filepath.Join(home, "Other"),
	} {
		c := DefaultRecordingSessionConfig()
		c.BaseDirectory = path
		if got := c.ExpandedBaseDirectory(); got != want {
			t.Fatalf("expand %q = %q, want %q", path, got, want)
		}
	}
}

func TestFirstRunWritesRecordingSessionDefaults(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := EnsureConfigExists(); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(GetConfigPath())
	if err != nil || !reflect.DeepEqual(cfg.RecordingSession, DefaultRecordingSessionConfig()) {
		t.Fatalf("first-run defaults = %+v, %v", cfg, err)
	}
	before, err := os.ReadFile(GetConfigPath())
	if err != nil {
		t.Fatal(err)
	}
	if err := EnsureConfigExists(); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(GetConfigPath())
	if err != nil || string(before) != string(after) {
		t.Fatal("existing config overwritten")
	}
}

func TestRecordingSessionSchemaMatchesDefaults(t *testing.T) {
	data, err := os.ReadFile("../../examples/config.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Properties map[string]struct {
			Properties map[string]struct {
				Default any `json:"default"`
			} `json:"properties"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	c := DefaultRecordingSessionConfig()
	want := map[string]any{
		"base_directory": c.BaseDirectory, "scene_collection": c.SceneCollection,
		"obs_profile": c.OBSProfile, "scene": c.Scene, "filter_name": c.FilterName,
		"source_names": []any{c.SourceNames[0], c.SourceNames[1]},
	}
	properties := schema.Properties["recording_session"].Properties
	if len(properties) != len(want) {
		t.Fatalf("session schema has %d properties, want %d", len(properties), len(want))
	}
	for name, value := range want {
		if !reflect.DeepEqual(properties[name].Default, value) {
			t.Fatalf("schema default %s = %v, want %v", name, properties[name].Default, value)
		}
	}
}
