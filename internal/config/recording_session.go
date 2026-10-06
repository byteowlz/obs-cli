package config

// RecordingSessionConfig binds named sessions to an already configured OBS setup.
type RecordingSessionConfig struct {
	BaseDirectory   string   `toml:"base_directory"`
	SceneCollection string   `toml:"scene_collection"`
	OBSProfile      string   `toml:"obs_profile"`
	Scene           string   `toml:"scene"`
	SourceNames     []string `toml:"source_names"`
	FilterName      string   `toml:"filter_name"`
}

func DefaultRecordingSessionConfig() RecordingSessionConfig {
	return RecordingSessionConfig{
		BaseDirectory:   "~/Movies",
		SceneCollection: "MultiTrack",
		OBSProfile:      "MultiTrack",
		Scene:           "Composite",
		SourceNames:     []string{"Desktop", "Cam Link"},
		FilterName:      "Source Record",
	}
}

// ExpandedBaseDirectory expands home and environment variables as for --config.
func (c RecordingSessionConfig) ExpandedBaseDirectory() string {
	return expandPath(c.BaseDirectory)
}
