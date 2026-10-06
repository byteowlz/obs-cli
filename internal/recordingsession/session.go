// Package recordingsession starts a named session without reconfiguring OBS.
package recordingsession

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/muesli/obs-cli/internal/config"
)

const sourceRecordKind = "source_record_filter"

// Filter contains only the settings this workflow inspects or changes.
type Filter struct {
	Kind    string
	Enabled bool
	Path    string
	Format  string
	Mode    int
}

// RecordingSettings contains the main-output settings required by this workflow.
type RecordingSettings struct {
	Format   string
	Filename string
}

// OBS is the narrow, fakeable boundary for the session workflow. No methods can
// switch collections/profiles/scenes, configure devices, or stop other outputs.
type OBS interface {
	RecordActive() (bool, error)
	StreamActive() (bool, error)
	SceneCollection() (string, error)
	Profile() (string, error)
	Scene() (string, error)
	RecordDirectory() (string, error)
	RecordingSettings() (RecordingSettings, error)
	SourceFilter(source, filter string) (Filter, error)
	SetRecordDirectory(path string) error
	SetFilterPath(source, filter, path string) error
	StartRecord() error
}

// ValidateName accepts a single readable path component, not a filesystem path.
func ValidateName(name string) error {
	if name == "" || len(name) > 80 || name != strings.TrimSpace(name) || name == "." || name == ".." {
		return errors.New("session name must be 1-80 bytes, without surrounding whitespace, and not . or ..")
	}
	for _, r := range name {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != ' ' && r != '-' && r != '_' && r != '.' {
			return errors.New("session name may contain only letters, numbers, spaces, dots, hyphens and underscores (not paths)")
		}
	}
	return nil
}

func validateConfig(c config.RecordingSessionConfig) error {
	for _, value := range []string{c.BaseDirectory, c.SceneCollection, c.OBSProfile, c.Scene, c.FilterName} {
		if strings.TrimSpace(value) == "" {
			return errors.New("recording_session base_directory and bindings must not be empty")
		}
	}
	if len(c.SourceNames) != 2 || strings.TrimSpace(c.SourceNames[0]) == "" || strings.TrimSpace(c.SourceNames[1]) == "" || c.SourceNames[0] == c.SourceNames[1] {
		return errors.New("recording_session.source_names must contain exactly two distinct nonempty source names")
	}
	return nil
}

func checkReady(obs OBS, c config.RecordingSessionConfig) error {
	for _, output := range []struct {
		name string
		read func() (bool, error)
	}{{"recording", obs.RecordActive}, {"streaming", obs.StreamActive}} {
		active, err := output.read()
		if err != nil {
			return fmt.Errorf("get %s status: %w", output.name, err)
		}
		if active {
			return fmt.Errorf("refusing session: %s is active", output.name)
		}
	}
	for _, binding := range []struct {
		name string
		want string
		read func() (string, error)
	}{
		{"scene collection", c.SceneCollection, obs.SceneCollection},
		{"OBS profile", c.OBSProfile, obs.Profile},
		{"program scene", c.Scene, obs.Scene},
	} {
		got, err := binding.read()
		if err != nil {
			return fmt.Errorf("get %s: %w", binding.name, err)
		}
		if got != binding.want {
			return fmt.Errorf("refusing session: %s is %q; expected %q (select it in OBS first)", binding.name, got, binding.want)
		}
	}
	settings, err := obs.RecordingSettings()
	if err != nil {
		return fmt.Errorf("get main recording settings: %w", err)
	}
	if settings.Format != "mkv" || settings.Filename != "program" {
		return errors.New("main recording must use MKV with filename formatting program (configure the OBS profile first)")
	}
	return nil
}

func readFilter(obs OBS, source, name string) (Filter, error) {
	filter, err := obs.SourceFilter(source, name)
	if err != nil {
		return Filter{}, fmt.Errorf("get filter %q on source %q: %w", name, source, err)
	}
	if filter.Kind != sourceRecordKind || !filter.Enabled {
		return Filter{}, fmt.Errorf("source %q needs enabled filter %q of kind %s (got %q, enabled=%t)", source, name, sourceRecordKind, filter.Kind, filter.Enabled)
	}
	if filter.Format != "mkv" || filter.Mode != 3 {
		return Filter{}, fmt.Errorf("source %q filter must use MKV and record_mode=3 (follow main recording)", source)
	}
	return filter, nil
}

// reserveDirectory uses atomic Mkdir, never MkdirAll on the session directory:
// existing files, folders and symlinks all count as collisions. Retries are bounded.
func reserveDirectory(base, name string, now time.Time) (string, error) {
	if base == "" || !filepath.IsAbs(base) {
		return "", errors.New("recording_session.base_directory must expand to an absolute local path")
	}
	if err := os.MkdirAll(base, 0755); err != nil {
		return "", fmt.Errorf("create session base directory: %w", err)
	}
	stem := now.Format("2006-01-02") + "_" + name
	for i := 1; i <= 1000; i++ {
		leaf := stem
		if i > 1 {
			leaf = fmt.Sprintf("%s_%d", stem, i)
		}
		path := filepath.Join(base, leaf)
		if err := os.Mkdir(path, 0755); err == nil {
			return path, nil
		} else if !os.IsExist(err) {
			return "", fmt.Errorf("reserve session directory: %w", err)
		}
	}
	return "", errors.New("could not reserve a session directory after 1000 name collisions")
}

type pathChange struct {
	label string
	set   func(string) error
	old   string
}

func rollback(changes []pathChange, cause error) error {
	for i := len(changes) - 1; i >= 0; i-- {
		if err := changes[i].set(changes[i].old); err != nil {
			cause = errors.Join(cause, fmt.Errorf("rollback %s: %w", changes[i].label, err))
		}
	}
	return cause
}

func verifyPaths(obs OBS, c config.RecordingSessionConfig, directory string) error {
	got, err := obs.RecordDirectory()
	if err != nil {
		return fmt.Errorf("verify program directory: %w", err)
	}
	if got != directory {
		return errors.New("program directory did not match requested session directory")
	}
	for _, source := range c.SourceNames {
		filter, err := readFilter(obs, source, c.FilterName)
		if err != nil {
			return err
		}
		if filter.Path != directory {
			return fmt.Errorf("source %q path did not match requested session directory", source)
		}
	}
	return nil
}

// waitForActivation separates an accepted RPC from an established recording.
func waitForActivation(obs OBS) error {
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	ticks := time.NewTicker(100 * time.Millisecond)
	defer ticks.Stop()
	for {
		active, err := obs.RecordActive()
		if err != nil {
			return fmt.Errorf("confirm recording activation: %w", err)
		}
		if active {
			return nil
		}
		select {
		case <-deadline.C:
			return errors.New("recording did not become active within five seconds")
		case <-ticks.C:
		}
	}
}

func uncertainStart(directory string, cause error) error {
	return fmt.Errorf("session directory %q retained; %w; start outcome is uncertain, check recording status before retrying (output paths were not rolled back)", directory, cause)
}

// Start validates all bindings and snapshots all paths before any mutation. A
// failed/ambiguous write is included in rollback, since OBS may have applied it.
// Reserved directories are kept even on failure: uncertain remote outcomes must
// never cause a later invocation to reuse a potentially recorded-in directory.
// The OBS API is not transactional; callers must not concurrently control OBS.
func Start(obs OBS, c config.RecordingSessionConfig, name string, now time.Time) (string, error) {
	if err := ValidateName(name); err != nil {
		return "", err
	}
	if err := validateConfig(c); err != nil {
		return "", err
	}
	if err := checkReady(obs, c); err != nil {
		return "", err
	}
	oldDirectory, err := obs.RecordDirectory()
	if err != nil {
		return "", fmt.Errorf("get program directory: %w", err)
	}
	changes := []pathChange{{"program directory", obs.SetRecordDirectory, oldDirectory}}
	for _, source := range c.SourceNames {
		filter, err := readFilter(obs, source, c.FilterName)
		if err != nil {
			return "", err
		}
		changes = append(changes, pathChange{source + " filter path", func(path string) error {
			return obs.SetFilterPath(source, c.FilterName, path)
		}, filter.Path})
	}
	directory, err := reserveDirectory(c.ExpandedBaseDirectory(), name, now)
	if err != nil {
		return "", err
	}
	fail := func(cause error, attempted []pathChange) (string, error) {
		return "", fmt.Errorf("session directory %q reserved; %w", directory, rollback(attempted, cause))
	}
	for i, change := range changes {
		if err := change.set(directory); err != nil {
			return fail(fmt.Errorf("set %s: %w", change.label, err), changes[:i+1])
		}
	}
	if err := verifyPaths(obs, c, directory); err != nil {
		return fail(err, changes)
	}
	// Recheck outputs and bindings immediately before start to catch changes
	// during configuration; this cannot eliminate races with other OBS clients.
	if err := checkReady(obs, c); err != nil {
		return fail(err, changes)
	}
	if err := obs.StartRecord(); err != nil {
		return "", uncertainStart(directory, fmt.Errorf("start recording: %w", err))
	}
	if err := waitForActivation(obs); err != nil {
		return "", uncertainStart(directory, err)
	}
	return directory, nil
}
