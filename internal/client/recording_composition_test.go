package client

import (
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/andreykaipov/goobs/api/requests/filters"
	"github.com/muesli/obs-cli/internal/config"
	"github.com/muesli/obs-cli/internal/recordingsession"
)

const adapterDisplayUUID = "00000000-0000-0000-0000-000000000001"

func drainSessionRequests(t *testing.T, o RecordingSessionOBS, requests <-chan map[string]any) []map[string]any {
	t.Helper()
	if err := o.Client.Disconnect(); err != nil {
		t.Fatal(err)
	}
	var out []map[string]any
	for request := range requests {
		out = append(out, request)
	}
	return out
}

func TestAdaptiveAdapterNativeDimensionsOverlayAndLifecycleOrder(t *testing.T) {
	o, requests := testSessionServer(t, "/old/desktop")
	c := config.DefaultRecordingSessionConfig()
	c.BaseDirectory = t.TempDir()
	path, err := recordingsession.StartWithOptions(o, c, "adaptive", time.Now(), recordingsession.Options{ScreenUUID: adapterDisplayUUID})
	if err != nil {
		t.Fatal(err)
	}
	removed, restored, transforms, starts, selections := 0, 0, 0, 0, 0
	for _, request := range drainSessionRequests(t, o, requests) {
		data, _ := request["requestData"].(map[string]any)
		switch request["requestType"] {
		case "SetInputSettings":
			selections++
			settings := data["inputSettings"].(map[string]any)
			if data["inputName"] != "Desktop" || data["overlay"] != true || len(settings) != 2 || settings["display_uuid"] != adapterDisplayUUID || settings["type"] != float64(0) {
				t.Fatalf("unsafe selection overlay: %+v", data)
			}
		case "RemoveSourceFilter":
			removed++
		case "SetVideoSettings":
			if removed != 2 || restored != 0 || len(data) != 4 || data["baseWidth"] != float64(3600) || data["baseHeight"] != float64(2338) || data["outputWidth"] != float64(1920) || data["outputHeight"] != float64(1246) {
				t.Fatalf("video reset before detach, wrong native sizing, or changed FPS: %+v", data)
			}
		case "SetSceneItemTransform":
			transforms++
			if removed != 2 || restored != 0 {
				t.Fatal("transforms must occur while both filters detached")
			}
			transform := data["sceneItemTransform"].(map[string]any)
			if transform["alignment"] != float64(5) || transform["boundsType"] != "OBS_BOUNDS_NONE" || transform["rotation"] != float64(0) || transform["cropLeft"] != float64(0) || transform["cropRight"] != float64(0) || transform["cropTop"] != float64(0) || transform["cropBottom"] != float64(0) || transform["scaleX"] != transform["scaleY"] {
				t.Fatalf("unsafe crop/transform: %+v", transform)
			}
			if transform["boundsWidth"].(float64) < 1 || transform["boundsHeight"].(float64) < 1 {
				t.Fatal("OBS validates serialized bounds minimum even for NONE")
			}
			if data["sceneItemId"] == float64(1) && transform["scaleX"] != float64(1) {
				t.Fatal("native source dimensions not used")
			}
		case "CreateSourceFilter":
			restored++
			if removed != 2 || transforms != 2 {
				t.Fatal("filters restored before full layout")
			}
			settings := data["filterSettings"].(map[string]any)
			if data["filterKind"] != "source_record_filter" || data["filterName"] != "Source Record" || settings["encoder"] != "obs_x264" || settings["audio_encoder"] != "ffmpeg_aac" || settings["filename_formatting"] != "keep" || settings["width"] != float64(0) || settings["height"] != float64(0) {
				t.Fatalf("filter state lost: %+v", data)
			}
			x264 := settings["x264opts"].(map[string]any)
			if x264["crf"] != float64(20) || x264["preset"] != "veryfast" || x264["tune"] != "zerolatency" {
				t.Fatal("encoder settings changed")
			}
		case "SetSourceFilterSettings":
			settings := data["filterSettings"].(map[string]any)
			if restored != 2 || len(settings) != 1 || settings["path"] != path || data["overlay"] != true {
				t.Fatalf("path write not isolated or too early: %+v", data)
			}
		case "StartRecord":
			starts++
			if restored != 2 || transforms != 2 {
				t.Fatal("start before complete restoration")
			}
		}
	}
	if removed != 2 || restored != 2 || starts != 1 || selections != 1 {
		t.Fatalf("unexpected lifecycle: removed=%d restored=%d starts=%d", removed, restored, starts)
	}
}

func TestAdaptiveAdapterUnchangedVideoSkipsResetAndFilterLifecycle(t *testing.T) {
	o, requests := testSessionServer(t, "/old/desktop")
	c := config.DefaultRecordingSessionConfig()
	c.BaseDirectory, c.Composition = t.TempDir(), "16:9" // Matches fixture's video settings.
	if _, err := recordingsession.Start(o, c, "same", time.Now()); err != nil {
		t.Fatal(err)
	}
	for _, request := range drainSessionRequests(t, o, requests) {
		switch request["requestType"] {
		case "SetVideoSettings", "RemoveSourceFilter", "CreateSourceFilter", "SetSourceFilterEnabled", "SetSourceFilterIndex", "SetInputSettings":
			t.Fatalf("unchanged video disturbed views/input: %+v", request)
		}
	}
}

func TestFilterLifecycleFailuresBestEffortAndNeverStart(t *testing.T) {
	for _, fault := range []sessionServerFault{
		{request: "GetSourceFilter", source: "Cam Link"},
		{request: "GetInputSettings"},
		{request: "SetInputSettings"},
		{request: "GetSceneItemId"},
		{request: "GetSceneItemTransform"},
		{request: "GetVideoSettings"},
		{request: "RemoveSourceFilter", source: "Desktop"},
		{request: "RemoveSourceFilter", source: "Desktop", applied: true},
		{request: "RemoveSourceFilter", source: "Cam Link"},
		{request: "RemoveSourceFilter", source: "Cam Link", applied: true},
		{request: "SetVideoSettings"},
		{request: "SetSceneItemTransform"},
		{request: "CreateSourceFilter", source: "Desktop"},
		{request: "CreateSourceFilter", source: "Desktop", applied: true},
		{request: "SetSourceFilterIndex", source: "Desktop"},
		{request: "SetSourceFilterEnabled", source: "Desktop"},
	} {
		t.Run(fault.request+"/"+fault.source+"/"+map[bool]string{true: "applied", false: "unapplied"}[fault.applied], func(t *testing.T) {
			o, requests := testSessionServerWithFault(t, "/old/desktop", &fault)
			c := config.DefaultRecordingSessionConfig()
			c.BaseDirectory = t.TempDir()
			if _, err := recordingsession.StartWithOptions(o, c, "fail", time.Now(), recordingsession.Options{ScreenUUID: adapterDisplayUUID}); err == nil {
				t.Fatal("injected configuration failure accepted")
			}
			createdCam, removedCam := false, false
			for _, request := range drainSessionRequests(t, o, requests) {
				data, _ := request["requestData"].(map[string]any)
				switch request["requestType"] {
				case "StartRecord", "SetRecordDirectory":
					t.Fatalf("started/reserved paths after failure: %+v", request)
				case "RemoveSourceFilter":
					if data["sourceName"] == "Cam Link" {
						removedCam = true
					}
				case "CreateSourceFilter":
					if data["sourceName"] == "Cam Link" {
						createdCam = true
					}
				}
			}
			if removedCam && !(fault.request == "RemoveSourceFilter" && !fault.applied) && !createdCam {
				t.Fatal("restoration stopped before independent camera recovery")
			}
		})
	}
}

func TestFilterLifecycleRestoresExactSnapshotsOnChangeError(t *testing.T) {
	o, requests := testSessionServer(t, "/old/desktop")
	// The lifecycle primitive also preserves disabled filters when used directly.
	if _, err := o.Client.Filters.SetSourceFilterEnabled(filters.NewSetSourceFilterEnabledParams().WithSourceName("Desktop").WithFilterName("Source Record").WithFilterEnabled(false)); err != nil {
		t.Fatal(err)
	}
	before := make([]filterSnapshot, 2)
	for i, source := range []string{"Desktop", "Cam Link"} {
		var err error
		before[i], err = o.snapshotFilter(source, "Source Record")
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := o.WithDetachedFilters([]string{"Desktop", "Cam Link"}, "Source Record", func() error { return nil }, func() error { return errors.New("change failed") }); err == nil {
		t.Fatal("change error discarded")
	}
	for _, snapshot := range before {
		after, err := o.snapshotFilter(snapshot.source, snapshot.name)
		if err != nil || !reflect.DeepEqual(after, snapshot) {
			t.Fatalf("snapshot not restored: %+v, %v", after, err)
		}
	}
	drainSessionRequests(t, o, requests)
}

func TestFilterLifecycleIdleGuardBeforeRemovals(t *testing.T) {
	o, requests := testSessionServer(t, "/old/desktop")
	changed := false
	err := o.WithDetachedFilters([]string{"Desktop", "Cam Link"}, "Source Record", func() error { return errors.New("streaming is active") }, func() error { changed = true; return nil })
	if err == nil || changed {
		t.Fatal("guard failed to block reset")
	}
	for _, request := range drainSessionRequests(t, o, requests) {
		if request["requestType"] != "GetSourceFilter" {
			t.Fatalf("mutation after idle rejection: %+v", request)
		}
	}
}

func TestNativeDimensionsRejectUnknownFractionalAndOversized(t *testing.T) {
	for _, width := range []float64{-1, 16385, 1920.5, math.NaN(), math.Inf(1)} {
		if _, err := nativeSize(width, 1080); err == nil {
			t.Fatalf("invalid native width accepted: %v", width)
		}
		if _, err := nativeSize(1920, width); err == nil {
			t.Fatalf("invalid native height accepted: %v", width)
		}
	}
}
