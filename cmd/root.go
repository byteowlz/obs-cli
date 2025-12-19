package cmd

import (
	"fmt"
	"os"

	"github.com/muesli/coral"
	"github.com/muesli/obs-cli/cmd/inputs"
	"github.com/muesli/obs-cli/cmd/obsconfig"
	"github.com/muesli/obs-cli/cmd/outputs"
	"github.com/muesli/obs-cli/cmd/scenes"
	"github.com/muesli/obs-cli/cmd/ui"
	"github.com/muesli/obs-cli/internal/client"
	"github.com/muesli/obs-cli/internal/config"
)

var (
	host       string
	password   string
	port       uint32
	configPath string
	profile    string

	cfg *config.Config

	// RootCmd is the main command for obs-cli
	RootCmd = &coral.Command{
		Use:   "obs-cli",
		Short: "obs-cli is a command-line remote control for OBS",
	}
)

func init() {
	coral.OnInitialize(initConfig, connectOBS)

	// Config file flags
	RootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "config file path (default: $XDG_CONFIG_HOME/obs-cli/config.toml)")
	RootCmd.PersistentFlags().StringVar(&profile, "profile", "", "use a named server profile from config")

	// Connection flags (these override config file and env vars)
	RootCmd.PersistentFlags().StringVar(&host, "host", "", "host to connect to")
	RootCmd.PersistentFlags().StringVar(&password, "password", "", "password for connection")
	RootCmd.PersistentFlags().Uint32VarP(&port, "port", "p", 0, "port to connect to")

	// Register all subcommands
	outputs.RegisterStreamCommands(RootCmd)
	outputs.RegisterRecordingCommands(RootCmd)
	outputs.RegisterVirtualCamCommands(RootCmd)
	outputs.RegisterReplayBufferCommands(RootCmd)
	scenes.RegisterSceneCommands(RootCmd)
	scenes.RegisterSceneItemCommands(RootCmd)
	inputs.RegisterSourceCommands(RootCmd)
	inputs.RegisterLabelCommands(RootCmd)
	obsconfig.RegisterProfileCommands(RootCmd)
	obsconfig.RegisterSceneCollectionCommands(RootCmd)
	ui.RegisterStudioModeCommands(RootCmd)
}

// initConfig loads configuration with priority: CLI flags > env vars > config file > defaults
func initConfig() {
	var err error

	// Ensure config file exists on first run
	if err = config.EnsureConfigExists(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not create config file: %v\n", err)
	}

	// Load config (applies defaults, then config file, then env vars)
	cfg, err = config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		os.Exit(1)
	}

	// Apply profile if specified
	if profile != "" {
		p := cfg.GetProfile(profile)
		if p == nil {
			fmt.Fprintf(os.Stderr, "error: profile %q not found in config\n", profile)
			os.Exit(1)
		}
		cfg.Host = p.Host
		cfg.Port = p.Port
		cfg.Password = p.Password
	}

	// Apply CLI flag overrides (highest priority)
	if host == "" {
		host = cfg.Host
	}
	if port == 0 {
		port = cfg.Port
	}
	if password == "" {
		password = cfg.Password
	}
}

func connectOBS() {
	if err := client.Connect(host, port, password); err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
}

// Execute runs the root command
func Execute() error {
	return RootCmd.Execute()
}
