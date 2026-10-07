package outputs

import (
	"fmt"
	"io"

	"github.com/muesli/obs-cli/internal/recordingsession"
)

// Audio settings are user-owned. Warn about Source Record's custom copy path,
// but never silently replace a source's audio or choose its global mix for it.
func warnCustomSourceAudio(source string, filter recordingsession.Filter, out io.Writer) error {
	if filter.DifferentAudio && (filter.AudioTrack == -1 || (filter.AudioTrack >= 1 && filter.AudioTrack <= 6)) {
		return nil
	}
	_, err := fmt.Fprintf(out, "warning: Source Record on %q uses a custom audio-copy path that can crackle; select a dedicated global OBS audio track (Different Audio, Track 1-6). See README audio-routing setup.\n", source)
	if err != nil {
		return fmt.Errorf("write audio-routing warning: %w", err)
	}
	return nil
}
