package recordingsession

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/muesli/obs-cli/internal/config"
)

var testTime = time.Date(2026, 1, 2, 12, 0, 0, 0, time.Local)

type fakeOBS struct {
	recording, streaming       bool
	collection, profile, scene string
	directory                  string
	filterName                 string
	filters                    map[string]Filter
	log                        []string
	calls                      map[string]int
	failOn                     map[string]int
	ignoreSet                  string
	onCall                     func(string, int)
	starts                     int
	noActivate                 bool
	settings                   RecordingSettings
}

func newFake() *fakeOBS {
	return &fakeOBS{
		collection: "MultiTrack", profile: "MultiTrack", scene: "Composite", directory: "/old/program", filterName: "Source Record",
		settings: RecordingSettings{Format: "mkv", Filename: "program"},
		filters: map[string]Filter{
			"Desktop":  {Kind: sourceRecordKind, Enabled: true, Path: "/old/desktop", Format: "mkv", Mode: 3},
			"Cam Link": {Kind: sourceRecordKind, Enabled: true, Path: "/old/cam", Format: "mkv", Mode: 3},
		},
		calls: map[string]int{}, failOn: map[string]int{},
	}
}

func (f *fakeOBS) call(op string) error {
	f.log = append(f.log, op)
	f.calls[op]++
	if f.onCall != nil {
		f.onCall(op, f.calls[op])
	}
	if f.failOn[op] == f.calls[op] {
		return fmt.Errorf("injected %s failure", op)
	}
	return nil
}

func (f *fakeOBS) RecordActive() (bool, error) {
	err := f.call("record")
	return f.recording, err
}
func (f *fakeOBS) StreamActive() (bool, error) {
	err := f.call("stream")
	return f.streaming, err
}
func (f *fakeOBS) SceneCollection() (string, error) {
	err := f.call("collection")
	return f.collection, err
}
func (f *fakeOBS) Profile() (string, error) {
	err := f.call("profile")
	return f.profile, err
}
func (f *fakeOBS) Scene() (string, error) {
	err := f.call("scene")
	return f.scene, err
}
func (f *fakeOBS) RecordDirectory() (string, error) {
	err := f.call("directory")
	return f.directory, err
}
func (f *fakeOBS) RecordingSettings() (RecordingSettings, error) {
	err := f.call("settings")
	return f.settings, err
}
func (f *fakeOBS) SourceFilter(source, name string) (Filter, error) {
	if name != f.filterName {
		return Filter{}, errors.New("unexpected filter binding")
	}
	if err := f.call("filter:" + source); err != nil {
		return Filter{}, err
	}
	filter, ok := f.filters[source]
	if !ok {
		return Filter{}, errors.New("missing source/filter")
	}
	return filter, nil
}
func (f *fakeOBS) SetRecordDirectory(path string) error {
	if f.ignoreSet != "set:program" {
		f.directory = path
	}
	return f.call("set:program") // Apply first: even a failed write may reach OBS.
}
func (f *fakeOBS) SetFilterPath(source, name, path string) error {
	if name != f.filterName {
		return errors.New("unexpected filter binding")
	}
	if f.ignoreSet != "set:"+source {
		filter := f.filters[source]
		filter.Path = path
		f.filters[source] = filter
	}
	return f.call("set:" + source)
}
func (f *fakeOBS) StartRecord() error {
	f.starts++
	if !f.noActivate {
		f.recording = true
	}
	return f.call("start")
}

func testConfig(t *testing.T) config.RecordingSessionConfig {
	t.Helper()
	c := config.DefaultRecordingSessionConfig()
	c.BaseDirectory = t.TempDir()
	return c
}

func assertRestored(t *testing.T, f *fakeOBS) {
	t.Helper()
	if f.directory != "/old/program" || f.filters["Desktop"].Path != "/old/desktop" || f.filters["Cam Link"].Path != "/old/cam" {
		t.Fatalf("paths not restored: %q, %+v", f.directory, f.filters)
	}
}

func assertNoWrites(t *testing.T, f *fakeOBS, base string) {
	t.Helper()
	for _, op := range f.log {
		if strings.HasPrefix(op, "set:") || op == "start" {
			t.Fatalf("unexpected mutation: %v", f.log)
		}
	}
	entries, err := os.ReadDir(base)
	if err != nil || len(entries) != 0 {
		t.Fatalf("preflight created a directory: %v, %v", entries, err)
	}
}

// OBS acknowledges StartRecord before encoder initialization completes. The fake
// exposes inactivity until 150ms after acknowledgement, without a racing goroutine.
func TestStartWaitsForActivationAndRejectsImmediateSecondCall(t *testing.T) {
	c, f := testConfig(t), newFake()
	var activeAt time.Time
	f.onCall = func(op string, _ int) {
		if op == "start" {
			activeAt = time.Now().Add(150 * time.Millisecond)
		}
		if op == "record" && !activeAt.IsZero() {
			f.recording = !time.Now().Before(activeAt)
		}
	}
	path, err := Start(f, c, "async", testTime)
	if err != nil {
		t.Fatal(err)
	}
	if !f.recording {
		t.Fatal("Start reported success before OBS recording became active")
	}
	before := len(f.log)
	if _, err := Start(f, c, "async", testTime); err == nil || !strings.Contains(err.Error(), "recording is active") {
		t.Fatalf("immediate second call must reject active recording: %v", err)
	}
	if f.starts != 1 || f.directory != path || len(f.log) != before+1 {
		t.Fatalf("second call changed session paths or started again: %v", f.log)
	}
	entries, err := os.ReadDir(c.BaseDirectory)
	if err != nil || len(entries) != 1 {
		t.Fatalf("second call reserved another directory: %v, %v", entries, err)
	}
}

func TestStartOrdering(t *testing.T) {
	c, f := testConfig(t), newFake()
	want := filepath.Join(c.BaseDirectory, "2026-01-02_Take one")
	f.onCall = func(op string, _ int) {
		if op == "start" && (f.directory != want || f.filters["Desktop"].Path != want || f.filters["Cam Link"].Path != want) {
			t.Fatal("started with partially configured paths")
		}
	}
	got, err := Start(f, c, "Take one", testTime)
	if err != nil || got != want {
		t.Fatalf("Start = %q, %v; want %q", got, err, want)
	}
	wantLog := []string{
		"record", "stream", "collection", "profile", "scene", "settings", "directory", "filter:Desktop", "filter:Cam Link",
		"set:program", "set:Desktop", "set:Cam Link",
		"directory", "filter:Desktop", "filter:Cam Link", "record", "stream", "collection", "profile", "scene", "settings", "start", "record",
	}
	if !reflect.DeepEqual(f.log, wantLog) || f.starts != 1 {
		t.Fatalf("unexpected ordering: %v", f.log)
	}
	if info, err := os.Stat(got); err != nil || !info.IsDir() {
		t.Fatalf("directory not reserved: %v", err)
	}
}

func TestCustomBindings(t *testing.T) {
	c, f := testConfig(t), newFake()
	c.SceneCollection, c.OBSProfile, c.Scene = "Sessions", "Capture", "Program"
	c.SourceNames, c.FilterName = []string{"Screen", "Camera"}, "Capture source"
	f.collection, f.profile, f.scene, f.filterName = c.SceneCollection, c.OBSProfile, c.Scene, c.FilterName
	f.filters = map[string]Filter{"Screen": f.filters["Desktop"], "Camera": f.filters["Cam Link"]}
	path, err := Start(f, c, "custom", testTime)
	if err != nil || f.filters["Screen"].Path != path || f.filters["Camera"].Path != path || f.starts != 1 {
		t.Fatalf("custom bindings not honored: %q, %v, %+v", path, err, f.filters)
	}
}

func TestPreflightRejectsWithoutMutation(t *testing.T) {
	cases := map[string]func(*fakeOBS){
		"recording active":    func(f *fakeOBS) { f.recording = true },
		"streaming active":    func(f *fakeOBS) { f.streaming = true },
		"wrong collection":    func(f *fakeOBS) { f.collection = "Other" },
		"wrong profile":       func(f *fakeOBS) { f.profile = "Other" },
		"wrong program scene": func(f *fakeOBS) { f.scene = "Other" },
		"wrong main format":   func(f *fakeOBS) { f.settings.Format = "mp4" },
		"wrong main filename": func(f *fakeOBS) { f.settings.Filename = "other" },
	}
	for _, source := range []string{"Desktop", "Cam Link"} {
		cases["missing "+source] = func(f *fakeOBS) { delete(f.filters, source) }
		cases["wrong kind "+source] = func(f *fakeOBS) {
			filter := f.filters[source]
			filter.Kind = "color_filter"
			f.filters[source] = filter
		}
		cases["wrong mode "+source] = func(f *fakeOBS) {
			filter := f.filters[source]
			filter.Mode = 1
			f.filters[source] = filter
		}
		cases["wrong format "+source] = func(f *fakeOBS) {
			filter := f.filters[source]
			filter.Format = "mp4"
			f.filters[source] = filter
		}
		cases["disabled "+source] = func(f *fakeOBS) {
			filter := f.filters[source]
			filter.Enabled = false
			f.filters[source] = filter
		}
	}
	for _, op := range []string{"record", "stream", "collection", "profile", "scene", "settings", "directory", "filter:Desktop", "filter:Cam Link"} {
		cases["read error "+op] = func(f *fakeOBS) { f.failOn[op] = 1 }
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			c, f := testConfig(t), newFake()
			setup(f)
			if _, err := Start(f, c, "demo", testTime); err == nil {
				t.Fatal("expected preflight rejection")
			}
			assertNoWrites(t, f, c.BaseDirectory)
		})
	}
}

func TestPathFailuresRollbackAttemptedWrites(t *testing.T) {
	for i, op := range []string{"set:program", "set:Desktop", "set:Cam Link"} {
		t.Run(op, func(t *testing.T) {
			c, f := testConfig(t), newFake()
			f.failOn[op] = 1
			if _, err := Start(f, c, "demo", testTime); err == nil || !strings.Contains(err.Error(), "reserved") {
				t.Fatalf("expected contextual write failure, got %v", err)
			}
			if f.starts != 0 {
				t.Fatal("started after write failure")
			}
			assertRestored(t, f)
			mutations := []string{}
			for _, entry := range f.log {
				if strings.HasPrefix(entry, "set:") {
					mutations = append(mutations, entry)
				}
			}
			forward := []string{"set:program", "set:Desktop", "set:Cam Link"}[:i+1]
			want := append([]string{}, forward...)
			for j := i; j >= 0; j-- {
				want = append(want, forward[j])
			}
			if !reflect.DeepEqual(mutations, want) {
				t.Fatalf("rollback ordering: %v, want %v", mutations, want)
			}
		})
	}
}

func TestVerificationFailuresNeverStart(t *testing.T) {
	cases := map[string]func(*fakeOBS){}
	for _, op := range []string{"directory", "filter:Desktop", "filter:Cam Link", "record", "stream", "collection", "profile", "scene", "settings"} {
		cases["second read fails "+op] = func(f *fakeOBS) { f.failOn[op] = 2 }
	}
	for _, op := range []string{"set:program", "set:Desktop", "set:Cam Link"} {
		cases["silent no-op "+op] = func(f *fakeOBS) { f.ignoreSet = op }
	}
	cases["stream starts during configuration"] = func(f *fakeOBS) {
		f.onCall = func(op string, n int) {
			if op == "stream" && n == 2 {
				f.streaming = true
			}
		}
	}
	cases["profile changes during configuration"] = func(f *fakeOBS) {
		f.onCall = func(op string, n int) {
			if op == "profile" && n == 2 {
				f.profile = "Other"
			}
		}
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			c, f := testConfig(t), newFake()
			setup(f)
			if _, err := Start(f, c, "demo", testTime); err == nil {
				t.Fatal("expected verification failure")
			}
			if f.starts != 0 {
				t.Fatal("started without verified paths/readiness")
			}
			assertRestored(t, f)
		})
	}
}

func TestStartUncertainOutcomeRetainsPathsAndReservation(t *testing.T) {
	for _, failure := range []string{"request error", "request timeout", "poll error", "activation timeout"} {
		t.Run(failure, func(t *testing.T) {
			c, f := testConfig(t), newFake()
			switch failure {
			case "request error", "request timeout":
				f.failOn["start"] = 1
				// Even an error reply/timeout may leave OBS starting recording.
				f.onCall = func(op string, _ int) {
					if op == "start" {
						f.recording = true
					}
				}
			case "poll error":
				f.failOn["record"] = 3 // Two readiness checks precede StartRecord.
			case "activation timeout":
				f.noActivate = true
			}
			started := time.Now()
			_, err := Start(f, c, "uncertain", testTime)
			if err == nil || !strings.Contains(err.Error(), "recording status") {
				t.Fatalf("missing check-status guidance: %v", err)
			}
			if failure == "activation timeout" && (time.Since(started) < 5*time.Second || time.Since(started) > 6*time.Second) {
				t.Fatalf("activation wait not bounded to five seconds: %v", time.Since(started))
			}
			path := filepath.Join(c.BaseDirectory, "2026-01-02_uncertain")
			if f.directory != path || f.filters["Desktop"].Path != path || f.filters["Cam Link"].Path != path {
				t.Fatalf("paths rolled back after start attempt: %q, %+v", f.directory, f.filters)
			}
			if info, err := os.Stat(path); err != nil || !info.IsDir() {
				t.Fatalf("reservation removed: %v", err)
			}
			if f.starts != 1 || f.calls["set:program"] != 1 || f.calls["set:Desktop"] != 1 || f.calls["set:Cam Link"] != 1 {
				t.Fatalf("unexpected post-start writes: %v", f.log)
			}
		})
	}
}

func TestRollbackContinuesAfterErrors(t *testing.T) {
	c, f := testConfig(t), newFake()
	f.failOn["directory"] = 2 // Pre-start verification failure still rolls back.
	f.failOn["set:Cam Link"] = 2
	f.failOn["set:Desktop"] = 2
	_, err := Start(f, c, "demo", testTime)
	for _, text := range []string{"verify program directory", "rollback Cam Link filter path", "rollback Desktop filter path"} {
		if err == nil || !strings.Contains(err.Error(), text) {
			t.Fatalf("missing failure %q: %v", text, err)
		}
	}
	if f.calls["set:program"] != 2 {
		t.Fatal("rollback stopped before program directory")
	}
}

func TestNameSafety(t *testing.T) {
	for _, name := range []string{"", ".", "..", "../escape", "/tmp", "a/b", `a\b`, "a:b", "a\x00b", "a\nb", " name", "name ", strings.Repeat("x", 81)} {
		t.Run(fmt.Sprintf("%q", name), func(t *testing.T) {
			c, f := testConfig(t), newFake()
			if _, err := Start(f, c, name, testTime); err == nil {
				t.Fatal("unsafe name accepted")
			}
			assertNoWrites(t, f, c.BaseDirectory)
			if len(f.log) != 0 {
				t.Fatal("invalid name reached OBS")
			}
		})
	}
	for _, name := range []string{"Demo", "Take one", "Änderung_1.2-test", "a..b", strings.Repeat("x", 80)} {
		if err := ValidateName(name); err != nil {
			t.Fatalf("safe name %q rejected: %v", name, err)
		}
	}
}

func TestInvalidBindings(t *testing.T) {
	cases := []func(*config.RecordingSessionConfig){
		func(c *config.RecordingSessionConfig) { c.BaseDirectory = "" },
		func(c *config.RecordingSessionConfig) { c.SceneCollection = "" },
		func(c *config.RecordingSessionConfig) { c.OBSProfile = "" },
		func(c *config.RecordingSessionConfig) { c.Scene = "" },
		func(c *config.RecordingSessionConfig) { c.FilterName = "" },
		func(c *config.RecordingSessionConfig) { c.SourceNames = nil },
		func(c *config.RecordingSessionConfig) { c.SourceNames = []string{"Desktop"} },
		func(c *config.RecordingSessionConfig) { c.SourceNames = []string{"Desktop", "Cam Link", "Third"} },
		func(c *config.RecordingSessionConfig) { c.SourceNames = []string{"Desktop", "Desktop"} },
		func(c *config.RecordingSessionConfig) { c.SourceNames = []string{"Desktop", " "} },
	}
	for i, setup := range cases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			c, f := testConfig(t), newFake()
			setup(&c)
			if _, err := Start(f, c, "demo", testTime); err == nil || len(f.log) != 0 {
				t.Fatalf("invalid config reached OBS: %v, %v", err, f.log)
			}
		})
	}
}

func TestDirectoryCollisionsNeverOverwrite(t *testing.T) {
	base := t.TempDir()
	stem := filepath.Join(base, "2026-01-02_demo")
	if err := os.Mkdir(stem, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stem, "existing.mkv"), []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stem+"_2", []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(stem, stem+"_3"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(base, "missing"), stem+"_4"); err != nil {
		t.Fatal(err)
	}
	got, err := reserveDirectory(base, "demo", testTime)
	if err != nil || got != stem+"_5" {
		t.Fatalf("collision result = %q, %v", got, err)
	}
	for _, path := range []string{filepath.Join(stem, "existing.mkv"), stem + "_2"} {
		if data, err := os.ReadFile(path); err != nil || string(data) != "keep" {
			t.Fatalf("existing content changed: %q, %v", data, err)
		}
	}
}

func TestDirectoryReservationsAreAtomic(t *testing.T) {
	base := t.TempDir()
	results := make(chan string, 20)
	errors := make(chan error, 20)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			path, err := reserveDirectory(base, "demo", testTime)
			results <- path
			errors <- err
		}()
	}
	wg.Wait()
	close(results)
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	seen := map[string]bool{}
	for path := range results {
		if seen[path] {
			t.Fatalf("duplicate reservation: %s", path)
		}
		seen[path] = true
	}
}

func TestDirectoryReservationFailures(t *testing.T) {
	t.Run("relative base", func(t *testing.T) {
		if _, err := reserveDirectory("relative", "demo", testTime); err == nil {
			t.Fatal("relative base accepted")
		}
	})
	t.Run("base is file", func(t *testing.T) {
		base := filepath.Join(t.TempDir(), "file")
		if err := os.WriteFile(base, nil, 0644); err != nil {
			t.Fatal(err)
		}
		c, f := testConfig(t), newFake()
		c.BaseDirectory = base
		if _, err := Start(f, c, "demo", testTime); err == nil {
			t.Fatal("file base accepted")
		}
		assertRestored(t, f)
		if f.starts != 0 {
			t.Fatal("started after filesystem failure")
		}
	})
	t.Run("bounded collisions", func(t *testing.T) {
		base := t.TempDir()
		for i := 1; i <= 1000; i++ {
			leaf := "2026-01-02_demo"
			if i > 1 {
				leaf += fmt.Sprintf("_%d", i)
			}
			if err := os.Mkdir(filepath.Join(base, leaf), 0755); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := reserveDirectory(base, "demo", testTime); err == nil {
			t.Fatal("collision search was not bounded")
		}
	})
}
