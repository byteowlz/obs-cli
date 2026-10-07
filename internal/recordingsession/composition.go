package recordingsession

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/muesli/obs-cli/internal/config"
)

// Options are invocation-only overrides. An empty Composition uses config.
// ScreenUUID is optional; absent selection preserves Desktop Properties choice.
type Options struct {
	ScreenUUID, Composition string
	// Optional diagnostics run on validated filter snapshots, before any mutation.
	AudioDiagnostic func(source string, filter Filter) error
}

// DesktopInput describes only the macOS selection settings we inspect.
type DesktopInput struct {
	Kind, DisplayUUID string
	CaptureType       int
}

// SceneSource reports native dimensions, never the transformed width/height.
type SceneSource struct {
	ID   int
	Size Size
}

// CompositionOBS is a deliberately small boundary, not a scene editor. It never
// enumerates OBS input properties or modifies devices, filters, audio or encoders.
type CompositionOBS interface {
	DesktopInput(source string) (DesktopInput, error)
	SelectScreen(source, uuid string) error
	SceneSource(scene, source string) (SceneSource, error)
	VideoSettings() (Video, error)
	SetVideoSettings(Video) error
	SetPlacement(scene string, id int, p Placement) error
	// WithDetachedFilters snapshots/restores the complete plugin state around a
	// video reset. guard runs before mutations, including best-effort recovery.
	WithDetachedFilters(sources []string, filter string, guard func() error, change func() error) error
}

var displayUUIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func validateOptions(c config.RecordingSessionConfig, opts Options) (string, error) {
	mode := c.Composition
	if opts.Composition != "" {
		mode = opts.Composition
	}
	if err := ValidateComposition(mode, c.OutputMaxDimension); err != nil {
		return "", err
	}
	if opts.ScreenUUID != "" && !displayUUIDPattern.MatchString(opts.ScreenUUID) {
		return "", errors.New("--screen must be a macOS display UUID (XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX)")
	}
	return mode, nil
}

func matchingScreen(o CompositionOBS, source, uuid string) (bool, error) {
	input, err := o.DesktopInput(source)
	if err != nil {
		return false, fmt.Errorf("read desktop input settings: %w", err)
	}
	if input.Kind != "screen_capture" {
		return false, fmt.Errorf("--screen requires a macOS screen_capture input, got %q", input.Kind)
	}
	return strings.EqualFold(input.DisplayUUID, uuid) && input.CaptureType == 0, nil
}

// waitForDesktop requires two successive equal valid native sizes. With explicit
// selection, both reads must also confirm the requested display UUID and type=0.
// Attempts and sleeps are bounded; each RPC retains the connection's timeout.
func waitForDesktop(o CompositionOBS, scene, source, uuid string, attempts int, pause func()) (SceneSource, error) {
	var previous SceneSource
	for i := 0; i < attempts; i++ {
		matches := true
		if uuid != "" {
			var err error
			matches, err = matchingScreen(o, source, uuid)
			if err != nil {
				return SceneSource{}, err
			}
		}
		current, err := o.SceneSource(scene, source)
		if err != nil {
			return SceneSource{}, fmt.Errorf("read desktop native dimensions: %w", err)
		}
		if current.Size.Width < 0 || current.Size.Height < 0 || current.Size.Width > MaxDimension || current.Size.Height > MaxDimension {
			return SceneSource{}, fmt.Errorf("desktop: invalid dimensions %+v", current.Size)
		}
		if matches && current.Size.Validate() == nil {
			if previous == current {
				return current, nil
			}
			previous = current
		} else {
			previous = SceneSource{}
		}
		if i+1 < attempts {
			pause()
		}
	}
	return SceneSource{}, errors.New("desktop capture did not become nonzero and stable with the requested selection within five seconds; check Desktop Properties")
}

// prepareComposition runs after strict session/filter preflight, before reserving
// paths. Selection/layout changes are intentionally retained on failure; restoring
// an asynchronous device selection is unsafe. No failure proceeds to StartRecord.
func prepareComposition(obs OBS, c config.RecordingSessionConfig, opts Options, mode string) error {
	if mode == "keep" && opts.ScreenUUID == "" {
		return nil
	}
	o, ok := obs.(CompositionOBS)
	if !ok {
		return errors.New("OBS adapter does not support recording composition")
	}
	if opts.ScreenUUID != "" {
		if _, err := matchingScreen(o, c.SourceNames[0], opts.ScreenUUID); err != nil {
			return err
		}
		if err := checkReady(obs, c); err != nil {
			return err
		}
		if err := o.SelectScreen(c.SourceNames[0], opts.ScreenUUID); err != nil {
			return fmt.Errorf("select desktop screen: %w", err)
		}
	}
	desktop, err := waitForDesktop(o, c.Scene, c.SourceNames[0], opts.ScreenUUID, 51, func() { time.Sleep(100 * time.Millisecond) })
	if err != nil {
		return err
	}
	if mode != "keep" {
		if err := applyLayout(obs, o, c, desktop, mode); err != nil {
			return err
		}
	}
	// Polling and video changes take time: recheck before reserving/writing paths.
	return checkReady(obs, c)
}

func applyLayout(obs OBS, o CompositionOBS, c config.RecordingSessionConfig, desktop SceneSource, mode string) error {
	camera, err := o.SceneSource(c.Scene, c.SourceNames[1])
	if err != nil {
		return fmt.Errorf("read camera native dimensions: %w", err)
	}
	layout, err := CalculateLayout(mode, desktop.Size, camera.Size, c.OutputMaxDimension)
	if err != nil {
		return err
	}
	video, err := o.VideoSettings()
	if err != nil {
		return fmt.Errorf("read video settings: %w", err)
	}
	if video != layout.Video {
		return o.WithDetachedFilters(c.SourceNames, c.FilterName, func() error { return checkReady(obs, c) }, func() error {
			if err := checkReady(obs, c); err != nil {
				return err
			}
			if err := o.SetVideoSettings(layout.Video); err != nil {
				return fmt.Errorf("set composition video settings: %w", err)
			}
			return applyPlacements(obs, o, c, desktop, camera, layout)
		})
	}
	// No video reset: never detach/recreate healthy plugin views unnecessarily.
	return applyPlacements(obs, o, c, desktop, camera, layout)
}

func applyPlacements(obs OBS, o CompositionOBS, c config.RecordingSessionConfig, desktop, camera SceneSource, layout Layout) error {
	for _, item := range []struct {
		id        int
		placement Placement
	}{{desktop.ID, layout.Desktop}, {camera.ID, layout.Camera}} {
		if err := checkReady(obs, c); err != nil {
			return err
		}
		if err := o.SetPlacement(c.Scene, item.id, item.placement); err != nil {
			return fmt.Errorf("set composition scene item %d: %w", item.id, err)
		}
	}
	actual, err := o.VideoSettings()
	if err != nil {
		return fmt.Errorf("verify composition video settings: %w", err)
	}
	if actual != layout.Video {
		return errors.New("video settings did not match requested composition")
	}
	return nil
}
