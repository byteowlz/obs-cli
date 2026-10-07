package outputs

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/muesli/obs-cli/internal/recordingsession"
)

func TestCustomSourceAudioWarnings(t *testing.T) {
	for _, tc := range []struct {
		name      string
		different bool
		track     int
		warn      bool
	}{
		{"parent source", false, 0, true},
		{"named source", true, 0, true},
		{"disabled different audio", false, 2, true},
		{"desktop mix", true, 2, false},
		{"microphone mix", true, 3, false},
		{"all global mixes", true, -1, false},
		{"invalid mix", true, 7, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			filter := recordingsession.Filter{Kind: "source_record_filter", Enabled: true, DifferentAudio: tc.different, AudioTrack: tc.track}
			var warning bytes.Buffer
			if err := warnCustomSourceAudio("Desktop", filter, &warning); err != nil {
				t.Fatal(err)
			}
			if (warning.Len() > 0) != tc.warn {
				t.Fatalf("warning=%q, expected=%t", warning.String(), tc.warn)
			}
			if tc.warn && (!strings.Contains(warning.String(), "crackle") || !strings.Contains(warning.String(), "global OBS audio track")) {
				t.Fatalf("warning lacks fix: %q", warning.String())
			}
			if filter.AudioTrack != tc.track || filter.DifferentAudio != tc.different {
				t.Fatal("diagnostics changed user-owned routing")
			}
		})
	}
}

type failingAudioWarningWriter struct{}

func (failingAudioWarningWriter) Write([]byte) (int, error) {
	return 0, errors.New("closed warning pipe")
}

func TestAudioWarningErrors(t *testing.T) {
	custom := recordingsession.Filter{Kind: "source_record_filter", Enabled: true}
	if err := warnCustomSourceAudio("Desktop", custom, failingAudioWarningWriter{}); err == nil {
		t.Fatal("warning write error hidden")
	}
}
