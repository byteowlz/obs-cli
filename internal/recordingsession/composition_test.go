package recordingsession

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

const testDisplayUUID = "00000000-0000-0000-0000-000000000001"

type compositionFake struct {
	*fakeOBS
	input      DesktopInput
	sources    map[string]SceneSource
	video      Video
	placements map[int]Placement
}

func newCompositionFake() *compositionFake {
	return &compositionFake{
		fakeOBS: newFake(), input: DesktopInput{Kind: "screen_capture", DisplayUUID: testDisplayUUID},
		sources: map[string]SceneSource{"Desktop": {1, Size{3600, 2338}}, "Cam Link": {2, Size{1920, 1080}}},
		video:   Video{Size{1920, 1080}, Size{1920, 1080}}, placements: map[int]Placement{},
	}
}

func (f *compositionFake) DesktopInput(source string) (DesktopInput, error) {
	err := f.call("input:" + source)
	return f.input, err
}
func (f *compositionFake) SelectScreen(source, uuid string) error {
	if f.ignoreSet != "set:screen" {
		f.input.DisplayUUID, f.input.CaptureType = uuid, 0
	}
	return f.call("set:screen")
}
func (f *compositionFake) SceneSource(scene, source string) (SceneSource, error) {
	if scene != f.scene {
		return SceneSource{}, fmt.Errorf("unexpected scene %q", scene)
	}
	err := f.call("native:" + source)
	return f.sources[source], err
}
func (f *compositionFake) VideoSettings() (Video, error) {
	err := f.call("video")
	return f.video, err
}
func (f *compositionFake) SetVideoSettings(v Video) error {
	if f.ignoreSet != "set:video" {
		f.video = v
	}
	return f.call("set:video")
}
func (f *compositionFake) SetPlacement(scene string, id int, p Placement) error {
	if scene != f.scene {
		return fmt.Errorf("unexpected scene %q", scene)
	}
	f.placements[id] = p
	return f.call(fmt.Sprintf("set:item:%d", id))
}

func (f *compositionFake) WithDetachedFilters(sources []string, filter string, guard func() error, change func() error) (err error) {
	if err := f.call("detach"); err != nil {
		return err
	}
	defer func() {
		if e := f.call("restore"); e != nil {
			err = fmt.Errorf("%v; restore: %w", err, e)
		}
	}()
	if err := guard(); err != nil {
		return err
	}
	return change()
}

func assertNoReservationOrStart(t *testing.T, f *compositionFake, base string) {
	t.Helper()
	if f.starts != 0 || f.calls["set:program"] != 0 || f.calls["set:Desktop"] != 0 || f.calls["set:Cam Link"] != 0 {
		t.Fatalf("configuration failure started or wrote paths: %v", f.log)
	}
	entries, err := os.ReadDir(base)
	if err != nil || len(entries) != 0 {
		t.Fatalf("configuration failure reserved a folder: %v, %v", entries, err)
	}
}

func TestAdaptiveStartCompositionBeforePaths(t *testing.T) {
	for _, mode := range []string{"screen", "16:9"} {
		t.Run(mode, func(t *testing.T) {
			f, c := newCompositionFake(), testConfig(t)
			c.Composition = mode
			f.onCall = func(op string, _ int) {
				if op == "set:program" && len(f.placements) != 2 {
					t.Fatal("paths reserved before layout")
				}
			}
			path, err := Start(f, c, "adaptive", testTime)
			if err != nil || path == "" || f.starts != 1 {
				t.Fatalf("Start: %q, %v", path, err)
			}
			want, _ := CalculateLayout(mode, f.sources["Desktop"].Size, f.sources["Cam Link"].Size, 1920)
			if f.video != want.Video || f.placements[1] != want.Desktop || f.placements[2] != want.Camera {
				t.Fatalf("layout not applied: %+v", f)
			}
			if f.calls["input:Desktop"] != 0 || f.calls["set:screen"] != 0 {
				t.Fatal("manual desktop choice queried or modified")
			}
			if f.sources["Desktop"].Size != (Size{3600, 2338}) || f.sources["Cam Link"].Size != (Size{1920, 1080}) {
				t.Fatal("native source size changed")
			}
		})
	}
}

func TestAdaptiveSelectionOptionalAndKeepCompatible(t *testing.T) {
	c := testConfig(t)
	f := newCompositionFake()
	f.input.DisplayUUID = "00000000-0000-0000-0000-000000000002"
	f.input.CaptureType = 1
	_, err := StartWithOptions(f, c, "select", testTime, Options{ScreenUUID: testDisplayUUID})
	if err != nil {
		t.Fatal(err)
	}
	if f.input.DisplayUUID != testDisplayUUID || f.input.CaptureType != 0 || f.calls["set:screen"] != 1 || f.calls["native:Desktop"] != 2 {
		t.Fatalf("selection not confirmed: %+v", f)
	}
	if f.calls["video"] != 0 || len(f.placements) != 0 {
		t.Fatal("keep wrote/read layout")
	}

	// Keep needs no new adapter methods, preserving the existing path-only setup.
	if _, err := Start(newFake(), c, "legacy", testTime); err != nil {
		t.Fatal(err)
	}
	c.Composition = "screen"
	if _, err := StartWithOptions(newFake(), c, "override", testTime, Options{Composition: "keep"}); err != nil {
		t.Fatalf("composition flag did not override config: %v", err)
	}
}

func TestDesktopReadinessRequiresMatchingSelectionAndStableNativeSize(t *testing.T) {
	f := newCompositionFake()
	f.onCall = func(op string, n int) {
		if op == "input:Desktop" {
			f.input.DisplayUUID = testDisplayUUID
			if n == 1 {
				f.input.DisplayUUID = "other"
			}
		}
		if op == "native:Desktop" {
			size := Size{3600, 2338}
			if n == 2 {
				size = Size{}
			}
			if n == 3 {
				size = Size{1920, 1080}
			}
			f.sources["Desktop"] = SceneSource{1, size}
		}
	}
	pauses := 0
	source, err := waitForDesktop(f, "Composite", "Desktop", testDisplayUUID, 6, func() { pauses++ })
	if err != nil || source.Size != (Size{3600, 2338}) || pauses != 4 {
		t.Fatalf("readiness: %+v, pauses %d, %v", source, pauses, err)
	}
	for _, invalid := range []string{"mismatch", "zero", "changing", "wrong type"} {
		t.Run(invalid, func(t *testing.T) {
			f := newCompositionFake()
			f.onCall = func(op string, n int) {
				switch invalid {
				case "mismatch":
					f.input.DisplayUUID = "other"
				case "zero":
					f.sources["Desktop"] = SceneSource{1, Size{}}
				case "changing":
					f.sources["Desktop"] = SceneSource{1, Size{2000 + n, 1000}}
				case "wrong type":
					f.input.CaptureType = 1
				}
			}
			pauses := 0
			if _, err := waitForDesktop(f, "Composite", "Desktop", testDisplayUUID, 3, func() { pauses++ }); err == nil || pauses != 2 {
				t.Fatalf("unready capture accepted or unbounded: %v, %d", err, pauses)
			}
		})
	}
}

func TestAdaptiveFailuresNeverReserveOrStart(t *testing.T) {
	cases := map[string]func(*compositionFake){
		"recording active":  func(f *compositionFake) { f.recording = true },
		"streaming active":  func(f *compositionFake) { f.streaming = true },
		"wrong binding":     func(f *compositionFake) { f.profile = "Other" },
		"bad main format":   func(f *compositionFake) { f.settings.Format = "mp4" },
		"bad filter":        func(f *compositionFake) { delete(f.filters, "Cam Link") },
		"wrong input kind":  func(f *compositionFake) { f.input.Kind = "other" },
		"oversized desktop": func(f *compositionFake) { f.sources["Desktop"] = SceneSource{1, Size{20000, 1080}} },
		"zero camera":       func(f *compositionFake) { f.sources["Cam Link"] = SceneSource{2, Size{}} },
		"oversized camera":  func(f *compositionFake) { f.sources["Cam Link"] = SceneSource{2, Size{1920, 20000}} },
		"video no-op":       func(f *compositionFake) { f.ignoreSet = "set:video" },
	}
	for _, op := range []string{"input:Desktop", "native:Desktop", "native:Cam Link", "video", "set:screen", "set:video", "set:item:1", "set:item:2", "detach", "restore"} {
		cases["failure "+op] = func(f *compositionFake) { f.failOn[op] = 1 }
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			c, f := testConfig(t), newCompositionFake()
			c.Composition = "screen"
			setup(f)
			if _, err := StartWithOptions(f, c, "failure", testTime, Options{ScreenUUID: testDisplayUUID}); err == nil {
				t.Fatal("expected failure")
			}
			assertNoReservationOrStart(t, f, c.BaseDirectory)
			if name == "recording active" || name == "streaming active" || name == "wrong binding" || name == "bad main format" || name == "bad filter" {
				assertNoWrites(t, f.fakeOBS, c.BaseDirectory)
				if f.calls["input:Desktop"] != 0 {
					t.Fatal("input queried before strict preflight")
				}
			}
		})
	}
}

func TestAdaptiveIdleRecheckedBeforeEachLayoutWrite(t *testing.T) {
	for _, trigger := range []string{"input:Desktop", "video", "set:video", "set:item:1", "set:item:2"} {
		t.Run(trigger, func(t *testing.T) {
			c, f := testConfig(t), newCompositionFake()
			c.Composition = "screen"
			f.onCall = func(op string, _ int) {
				if op == trigger {
					f.streaming = true
				}
			}
			if _, err := StartWithOptions(f, c, "race", testTime, Options{ScreenUUID: testDisplayUUID}); err == nil || !strings.Contains(err.Error(), "streaming is active") {
				t.Fatalf("idle race accepted: %v", err)
			}
			assertNoReservationOrStart(t, f, c.BaseDirectory)
			for i, op := range f.log {
				if op == trigger && i+1 < len(f.log) {
					for _, next := range f.log[i+1:] {
						if strings.HasPrefix(next, "set:") {
							t.Fatalf("mutated after active status: %v", f.log)
						}
					}
					break
				}
			}
		})
	}
}

func TestAdaptiveInvalidOptionsRejectedBeforeOBS(t *testing.T) {
	for _, opts := range []Options{{ScreenUUID: "not-a-uuid"}, {Composition: "invalid"}} {
		c, f := testConfig(t), newCompositionFake()
		if _, err := StartWithOptions(f, c, "invalid", testTime, opts); err == nil || len(f.log) != 0 {
			t.Fatalf("invalid options reached OBS: %v, %v", err, f.log)
		}
	}
	for _, cap := range []int{0, 1, 16385} {
		c, f := testConfig(t), newCompositionFake()
		c.OutputMaxDimension = cap
		if _, err := Start(f, c, "invalid", testTime); err == nil || len(f.log) != 0 {
			t.Fatalf("invalid config cap reached OBS: %v", err)
		}
	}
}

func TestAdaptiveZeroDesktopBoundedBeforePaths(t *testing.T) {
	c, f := testConfig(t), newCompositionFake()
	c.Composition = "screen"
	f.sources["Desktop"] = SceneSource{1, Size{}}
	started := time.Now()
	_, err := Start(f, c, "unready", testTime)
	if err == nil || time.Since(started) < 5*time.Second || time.Since(started) > 7*time.Second {
		t.Fatalf("readiness timeout not bounded: %v (%v)", err, time.Since(started))
	}
	assertNoReservationOrStart(t, f, c.BaseDirectory)
}
