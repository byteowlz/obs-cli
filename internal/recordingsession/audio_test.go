package recordingsession

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestAudioDiagnosticsUseValidatedSnapshotsBeforeWrites(t *testing.T) {
	c, f := testConfig(t), newFake()
	var seen []string
	opts := Options{Composition: "keep", AudioDiagnostic: func(source string, filter Filter) error {
		if f.calls["set:program"] != 0 || f.starts != 0 {
			t.Fatal("diagnostics ran after mutation/start")
		}
		if filter.Kind != sourceRecordKind || !filter.Enabled || filter.Mode != 3 {
			t.Fatal("unvalidated snapshot reached diagnostics")
		}
		seen = append(seen, source)
		return nil
	}}
	if _, err := StartWithOptions(f, c, "audio", testTime, opts); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(seen, c.SourceNames) {
		t.Fatalf("diagnostic source order: %v", seen)
	}
}

func TestAudioDiagnosticFailureNeverMutatesOBS(t *testing.T) {
	c, f := testConfig(t), newFake()
	opts := Options{AudioDiagnostic: func(string, Filter) error { return errors.New("closed warning pipe") }}
	if _, err := StartWithOptions(f, c, "audio", testTime, opts); err == nil || !strings.Contains(err.Error(), "audio diagnostics") {
		t.Fatalf("diagnostic error not surfaced: %v", err)
	}
	assertNoWrites(t, f, c.BaseDirectory)
}
