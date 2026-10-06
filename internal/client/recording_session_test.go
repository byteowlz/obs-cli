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
func testSessionServer(t *testing.T, initialPath any) (RecordingSessionOBS, <-chan map[string]any) {
	t.Helper()
	requests := make(chan map[string]any, 40)
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
		paths := map[string]any{"Desktop": initialPath, "Cam Link": "/old/cam"}
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
			switch request["requestType"] {
			case "GetRecordStatus", "GetStreamStatus":
				response["outputActive"] = false
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
				response["filterEnabled"] = true
				response["filterSettings"] = map[string]any{"path": paths[source], "filename_formatting": "keep", "record_mode": 3}
			case "SetRecordDirectory":
				directory, _ = data["recordDirectory"].(string)
			case "SetSourceFilterSettings":
				source, _ := data["sourceName"].(string)
				settings, _ := data["filterSettings"].(map[string]any)
				paths[source] = settings["path"]
			case "StartRecord":
			default:
				t.Errorf("unexpected OBS request: %v", request)
			}
			if err := conn.WriteJSON(map[string]any{"op": 7, "d": map[string]any{
				"requestType": request["requestType"], "requestId": request["requestId"],
				"requestStatus": map[string]any{"result": true, "code": 100}, "responseData": response,
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

func TestRecordingSessionAdapterRequiresSnapshotablePath(t *testing.T) {
	for _, path := range []any{nil, 42, true} {
		obs, _ := testSessionServer(t, path)
		if _, err := obs.SourceFilter("Desktop", "Source Record"); err == nil {
			t.Fatalf("invalid path setting accepted: %v", path)
		}
	}
}
