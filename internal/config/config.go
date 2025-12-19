package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

const (
	AppName           = "obs-cli"
	DefaultConfigName = "config.toml"
)

// Config holds the application configuration
type Config struct {
	Schema   string         `toml:"$schema,omitempty"`
	Host     string         `toml:"host"`
	Port     uint32         `toml:"port"`
	Password string         `toml:"password"`
	Profiles []ServerConfig `toml:"profiles,omitempty"`
}

// ServerConfig allows defining multiple OBS server profiles
type ServerConfig struct {
	Name     string `toml:"name"`
	Host     string `toml:"host"`
	Port     uint32 `toml:"port"`
	Password string `toml:"password"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Schema:   "https://raw.githubusercontent.com/byteowlz/schemas/refs/heads/main/obs-cli/obs-cli.config.schema.json",
		Host:     "localhost",
		Port:     4455,
		Password: "",
	}
}

// GetConfigDir returns the XDG-compliant config directory
func GetConfigDir() string {
	// Check XDG_CONFIG_HOME first
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return filepath.Join(expandPath(xdgConfig), AppName)
	}

	// Fall back to ~/.config
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", AppName)
}

// GetConfigPath returns the full path to the config file
func GetConfigPath() string {
	return filepath.Join(GetConfigDir(), DefaultConfigName)
}

// expandPath expands ~ and environment variables in a path
func expandPath(path string) string {
	// Expand ~ to home directory
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, path[1:])
		}
	}

	// Expand environment variables
	path = os.ExpandEnv(path)

	return path
}

// Load loads configuration with the following priority:
// 1. Command line flags (handled in main.go after this returns)
// 2. Environment variables
// 3. Config file
// 4. Defaults
func Load(configPath string) (*Config, error) {
	cfg := DefaultConfig()

	// Determine which config file to use
	cfgPath := configPath
	if cfgPath == "" {
		cfgPath = GetConfigPath()
	} else {
		cfgPath = expandPath(cfgPath)
	}

	// Try to load config file
	if cfgPath != "" {
		if _, err := os.Stat(cfgPath); err == nil {
			if _, err := toml.DecodeFile(cfgPath, cfg); err != nil {
				return nil, fmt.Errorf("failed to parse config file %s: %w", cfgPath, err)
			}
		}
	}

	// Override with environment variables
	cfg.applyEnvOverrides()

	return cfg, nil
}

// applyEnvOverrides applies environment variable overrides to the config
func (c *Config) applyEnvOverrides() {
	if host := os.Getenv("OBS_CLI_HOST"); host != "" {
		c.Host = host
	}

	if port := os.Getenv("OBS_CLI_PORT"); port != "" {
		var p uint32
		if _, err := fmt.Sscanf(port, "%d", &p); err == nil {
			c.Port = p
		}
	}

	if password := os.Getenv("OBS_CLI_PASSWORD"); password != "" {
		c.Password = password
	}
}

// EnsureConfigExists creates the config file with defaults if it doesn't exist
func EnsureConfigExists() error {
	cfgPath := GetConfigPath()
	if cfgPath == "" {
		return nil // Can't determine config path, skip
	}

	// Check if config already exists
	if _, err := os.Stat(cfgPath); err == nil {
		return nil // Config exists
	}

	// Create config directory
	cfgDir := filepath.Dir(cfgPath)
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write default config with comments
	return writeDefaultConfig(cfgPath)
}

// writeDefaultConfig writes a commented default config file
func writeDefaultConfig(path string) error {
	content := `# obs-cli configuration file
# Schema reference for LSP support (e.g., Even Better TOML extension)
"$schema" = "https://raw.githubusercontent.com/byteowlz/schemas/refs/heads/main/obs-cli/obs-cli.config.schema.json"

# OBS WebSocket server host
host = "localhost"

# OBS WebSocket server port (default: 4455 for OBS 28+, was 4444 for older versions)
port = 4455

# OBS WebSocket server password (leave empty if authentication is disabled)
password = ""

# Optional: Define multiple server profiles
# [[profiles]]
# name = "local"
# host = "localhost"
# port = 4455
# password = ""
#
# [[profiles]]
# name = "remote"
# host = "192.168.1.100"
# port = 4455
# password = "secret"
`

	return os.WriteFile(path, []byte(content), 0644)
}

// GetProfile returns a server profile by name, or nil if not found
func (c *Config) GetProfile(name string) *ServerConfig {
	for _, p := range c.Profiles {
		if p.Name == name {
			return &p
		}
	}
	return nil
}
