package outputs

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/muesli/coral"
	"github.com/muesli/obs-cli/internal/client"
	"github.com/muesli/obs-cli/internal/config"
	"github.com/muesli/obs-cli/internal/recordingsession"
	"golang.org/x/term"
)

var recordingSessionConfig = config.DefaultRecordingSessionConfig()

// ConfigureRecordingSession applies the already loaded user configuration.
func ConfigureRecordingSession(c config.RecordingSessionConfig) {
	recordingSessionConfig = c
}

func sessionName(args []string, in io.Reader, prompt io.Writer, interactive bool) (string, error) {
	if len(args) > 1 {
		return "", errors.New("expected one session name; quote names containing spaces")
	}
	var name string
	if len(args) == 1 {
		name = args[0]
	} else {
		if !interactive {
			return "", errors.New("session name required: obs-cli recording session <name> (prompt is TTY-only)")
		}
		if _, err := fmt.Fprint(prompt, "Session name: "); err != nil {
			return "", err
		}
		line, err := bufio.NewReader(io.LimitReader(in, 256)).ReadString('\n')
		if err != nil && err != io.EOF {
			return "", fmt.Errorf("read session name: %w", err)
		}
		name = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
	}
	if err := recordingsession.ValidateName(name); err != nil {
		return "", err
	}
	return name, nil
}

func newRecordingSessionCommand() *coral.Command {
	return &coral.Command{
		Use:   "session [name]",
		Short: "Start a named program and source recording session",
		Long: `Start recording into a unique YYYY-MM-DD_name directory (default: ~/Movies).
Requires the configured collection, OBS profile, program scene and two enabled
Source Record filters. Changes only output paths; does not configure OBS.
Prints the absolute session directory on stdout. Omitting name prompts on a TTY.
Use recording stop/status as usual; pause propagation is not verified here.`,
		Example: "  obs-cli recording session demo\n  obs-cli --config ~/.config/obs-cli/user-config.toml recording session \"Take one\"",
		Args:    coral.MaximumNArgs(1),
		RunE: func(cmd *coral.Command, args []string) error {
			name, err := sessionName(args, os.Stdin, cmd.ErrOrStderr(), term.IsTerminal(int(os.Stdin.Fd())) && os.Getenv("CI") == "")
			if err != nil {
				return err
			}
			directory, err := recordingsession.Start(client.RecordingSessionOBS{Client: client.Client}, recordingSessionConfig, name, time.Now())
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), directory)
			return err
		},
	}
}
