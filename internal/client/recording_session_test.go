package client

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/andreykaipov/goobs"
	"github.com/gorilla/websocket"
	"github.com/muesli/obs-cli/internal/config"
	"github.com/muesli/obs-cli/internal/recordingsession"
)

// This server implements only the session requests on an ephemeral loopback
// port. It never contacts a running OBS instance or uses user's config.
func testSessionServer(t *testing.T, initialPath any, overrides ...map[string]string) (RecordingSessionOBS, <-chan map[string]any) {
	return testSessionServerWithFault(t, initialPath, nil, overrides...)
}

type sessionServerFault struct {
	request, source string
	applied         bool // An error reply can follow a successful remote mutation.
}

func testSessionServerWithFault(t *testing.T, initialPath any, fault *sessionServerFault, overrides ...map[string]string) (RecordingSessionOBS, <-chan map[string]any) {
	t.Helper()
	parameters := map[string]string{"Output.Mode": "Advanced", "Output.FilenameFormatting": "program", "AdvOut.RecFormat2": "mkv", "SimpleOutput.RecFormat2": "mkv"}
	for _, values := range overrides {
		for key, value := range values {
			parameters[key] = value
		}
	}
	requests := make(chan map[string]any, 512)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		defer close(requests)
		if err := conn.WriteJSON(map[string]any{"op": 0, "d": map[string]any{"obsWebSocketVersion": "5.0.0", "rpcVersion": 1}}); err != nil {
			t.Error(err)
			return
		}
		var identify map[string]any
		if err := conn.ReadJSON(&identify); err != nil {
			t.Error(err)
			return
		}
		if err := conn.WriteJSON(map[string]any{"op": 2, "d": map[string]any{"negotiatedRpcVersion": 1}}); err != nil {
			t.Error(err)
			return
		}
		directory := "/old/program"
		recording := false
		paths := map[string]any{"Desktop": initialPath, "Cam Link": "/old/cam"}
		present := map[string]bool{"Desktop": true, "Cam Link": true}
		indices := map[string]int{"Desktop": 1, "Cam Link": 0}
		enabled := map[string]bool{"Desktop": true, "Cam Link": true}
		fullSettings := map[string]map[string]any{}
		for source, path := range paths {
			fullSettings[source] = map[string]any{"path": path, "filename_formatting": "keep", "record_mode": 3, "rec_format": "mkv", "encoder": "obs_x264", "audio_encoder": "ffmpeg_aac", "x264opts": map[string]any{"crf": 20, "preset": "veryfast", "tune": "zerolatency"}, "width": 0, "height": 0}
		}
		displayUUID := "00000000-0000-0000-0000-000000000002"
		video := map[string]any{"baseWidth": 1920, "baseHeight": 1080, "outputWidth": 1920, "outputHeight": 1080, "fpsNumerator": 60, "fpsDenominator": 1}
		for {
			var message struct {
				Data map[string]any `json:"d"`
			}
			if err := conn.ReadJSON(&message); err != nil {
				return // Disconnect after the test is expected.
			}
			request := message.Data
			requests <- request
			data, _ := request["requestData"].(map[string]any)
			response := map[string]any{}
			failed := fault != nil && request["requestType"] == fault.request && (fault.source == "" || data["sourceName"] == fault.source)
			if !failed || fault.applied {
				switch request["requestType"] {
				case "GetRecordStatus":
					response["outputActive"] = recording
				case "GetStreamStatus":
					response["outputActive"] = false
				case "GetProfileParameter":
					response["parameterValue"] = parameters[data["parameterCategory"].(string)+"."+data["parameterName"].(string)]
				case "GetSceneCollectionList":
					response["currentSceneCollectionName"] = "MultiTrack"
				case "GetProfileList":
					response["currentProfileName"] = "MultiTrack"
				case "GetCurrentProgramScene":
					// OBS 28/29 uses the deprecated field; the adapter must support it.
					response["currentProgramSceneName"] = "Composite"
				case "GetRecordDirectory":
					response["recordDirectory"] = directory
				case "GetSourceFilter":
					source, _ := data["sourceName"].(string)
					response["filterKind"] = "source_record_filter"
					response["filterEnabled"] = enabled[source]
					response["filterIndex"] = indices[source]
					response["filterSettings"] = fullSettings[source]
					if !present[source] {
						failed = true
					}
				case "SetRecordDirectory":
					directory, _ = data["recordDirectory"].(string)
				case "SetSourceFilterSettings":
					source, _ := data["sourceName"].(string)
					settings, _ := data["filterSettings"].(map[string]any)
					paths[source] = settings["path"]
					fullSettings[source]["path"] = settings["path"]
				case "GetSourceFilterList":
					source := data["sourceName"].(string)
					items := []map[string]any{}
					if present[source] {
						items = append(items, map[string]any{"filterName": "Source Record"})
					}
					response["filters"] = items
				case "RemoveSourceFilter":
					present[data["sourceName"].(string)] = false
				case "CreateSourceFilter":
					source := data["sourceName"].(string)
					present[source] = true
					indices[source], enabled[source] = 0, true
					fullSettings[source] = data["filterSettings"].(map[string]any)
					paths[source] = fullSettings[source]["path"]
				case "SetSourceFilterIndex":
					indices[data["sourceName"].(string)] = int(data["filterIndex"].(float64))
				case "SetSourceFilterEnabled":
					enabled[data["sourceName"].(string)] = data["filterEnabled"].(bool)
				case "GetInputSettings":
					response["inputKind"] = "screen_capture"
					// Default type=0 is intentionally absent, as with real GetInputSettings.
					response["inputSettings"] = map[string]any{"display_uuid": displayUUID, "show_cursor": true}
				case "SetInputSettings":
					settings := data["inputSettings"].(map[string]any)
					displayUUID = settings["display_uuid"].(string)
				case "GetSceneItemId":
					response["sceneItemId"] = 1
					if data["sourceName"] == "Cam Link" {
						response["sceneItemId"] = 2
					}
				case "GetSceneItemTransform":
					width, height := 3600, 2338
					if data["sceneItemId"] == float64(2) {
						width, height = 1920, 1080
					}
					// Displayed width is deliberately not the native source width.
					response["sceneItemTransform"] = map[string]any{"sourceWidth": width, "sourceHeight": height, "width": 10, "height": 20, "cropLeft": 40}
				case "GetVideoSettings":
					response = video
				case "SetVideoSettings":
					for k, v := range data {
						video[k] = v
					}
				case "SetSceneItemTransform":
					// Real OBS validates bounds even when boundsType=OBS_BOUNDS_NONE.
					transform := data["sceneItemTransform"].(map[string]any)
					if transform["boundsWidth"].(float64) < 1 || transform["boundsHeight"].(float64) < 1 {
						failed = true
					}
				case "StartRecord":
					recording = true
				default:
					t.Errorf("unexpected OBS request: %v", request)
				}
			}
			code := 100
			if failed {
				code = 500
			}
			if err := conn.WriteJSON(map[string]any{"op": 7, "d": map[string]any{
				"requestType": request["requestType"], "requestId": request["requestId"],
				"requestStatus": map[string]any{"result": !failed, "code": code}, "responseData": response,
			}}); err != nil {
				t.Error(err)
				return
			}
		}
	}))
	t.Cleanup(server.Close)
	connection, err := goobs.New(strings.TrimPrefix(server.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Disconnect() })
	return RecordingSessionOBS{Client: connection}, requests
}

func TestRecordingSessionAdapterOverlaysOnlyPaths(t *testing.T) {
	obs, requests := testSessionServer(t, "/old/desktop")
	c := config.DefaultRecordingSessionConfig()
	c.BaseDirectory = t.TempDir()
	c.Composition = "keep"
	path, err := recordingsession.Start(obs, c, "adapter", time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if path != filepath.Join(c.BaseDirectory, "2026-01-02_adapter") {
		t.Fatalf("unexpected path: %q", path)
	}
	// Disconnect closes the request stream. Only fake-server requests are inspected.
	if err := obs.Client.Disconnect(); err != nil {
		t.Fatal(err)
	}
	sets, starts := 0, 0
	for request := range requests {
		if request["requestType"] == "SetSourceFilterSettings" {
			sets++
			data := request["requestData"].(map[string]any)
			settings := data["filterSettings"].(map[string]any)
			if data["overlay"] != true || data["filterName"] != "Source Record" || len(settings) != 1 || settings["path"] != path {
				t.Fatalf("filter write changed more than path: %+v", data)
			}
		}
		if request["requestType"] == "StartRecord" {
			starts++
			if sets != 2 {
				t.Fatal("StartRecord sent before both filter writes")
			}
		}
	}
	if sets != 2 || starts != 1 {
		t.Fatalf("filter sets=%d, starts=%d", sets, starts)
	}
}

func TestRecordingSettingsModes(t *testing.T) {
	for _, mode := range []string{"Simple", "Advanced", "invalid"} {
		t.Run(mode, func(t *testing.T) {
			obs, requests := testSessionServer(t, "/old/desktop", map[string]string{"Output.Mode": mode})
			settings, err := obs.RecordingSettings()
			if mode == "invalid" {
				if err == nil {
					t.Fatal("unsupported output mode accepted")
				}
				return
			}
			if err != nil || settings.Format != "mkv" || settings.Filename != "program" {
				t.Fatalf("settings: %+v, %v", settings, err)
			}
			if err := obs.Client.Disconnect(); err != nil {
				t.Fatal(err)
			}
			for request := range requests {
				data := request["requestData"].(map[string]any)
				if data["parameterName"] == "RecFormat2" {
					want := "AdvOut"
					if mode == "Simple" {
						want = "SimpleOutput"
					}
					if data["parameterCategory"] != want {
						t.Fatalf("wrong recording category: %+v", data)
					}
				}
			}
		})
	}
}

func TestRecordingSessionAdapterRequiresSnapshotablePath(t *testing.T) {
	for _, path := range []any{nil, 42, true} {
		obs, _ := testSessionServer(t, path)
		if _, err := obs.SourceFilter("Desktop", "Source Record"); err == nil {
			t.Fatalf("invalid path setting accepted: %v", path)
		}
	}
}
