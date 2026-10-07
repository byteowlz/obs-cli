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
	var opts recordingsession.Options
	cmd := &coral.Command{
		Use:   "session [name]",
		Short: "Start a named program and source recording session",
		Long: `Start recording into a unique YYYY-MM-DD_name directory (default: ~/Movies).
Requires the configured collection, OBS profile, program scene and two enabled
Source Record filters. Composition defaults to the actual Desktop capture aspect,
with even output dimensions capped at 1920 pixels (configurable; no upscaling).
--screen optionally selects a macOS display UUID; absent it, Desktop Properties
selection is retained. --composition 16:9 fits inside a fixed canvas; keep preserves
video settings and scene transforms. Layout never crops Desktop; camera PIP stays
inside Desktop content. Source Record resolutions, encoders and audio are untouched.
Active recording/streaming is refused before changes. Failed selection/layout may
remain applied; recording will not start after any configuration failure.
Prints the absolute session directory on stdout. Omitting name prompts on a TTY.
Use recording stop/status as usual; pause propagation is not verified here.`,
		Example: "  obs-cli recording session demo\n  obs-cli recording session demo --screen <display-UUID> --composition screen\n  obs-cli recording session demo --composition keep\n  obs-cli --config ~/.config/obs-cli/user-config.toml recording session \"Take one\"",
		Args:    coral.MaximumNArgs(1),
		RunE: func(cmd *coral.Command, args []string) error {
			name, err := sessionName(args, os.Stdin, cmd.ErrOrStderr(), term.IsTerminal(int(os.Stdin.Fd())) && os.Getenv("CI") == "")
			if err != nil {
				return err
			}
			obs := client.RecordingSessionOBS{Client: client.Client}
			opts.AudioDiagnostic = func(source string, filter recordingsession.Filter) error {
				return warnCustomSourceAudio(source, filter, cmd.ErrOrStderr())
			}
			directory, err := recordingsession.StartWithOptions(obs, recordingSessionConfig, name, time.Now(), opts)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), directory)
			return err
		},
	}
	cmd.Flags().StringVar(&opts.ScreenUUID, "screen", "", "Select macOS Desktop screen_capture display UUID (optional; no enumeration)")
	cmd.Flags().StringVar(&opts.Composition, "composition", "", "Program layout: screen, 16:9 or keep (default: recording_session.composition)")
	return cmd
}
