package client

import (
	"fmt"

	"github.com/andreykaipov/goobs"
	obsconfig "github.com/andreykaipov/goobs/api/requests/config"
	"github.com/andreykaipov/goobs/api/requests/filters"
	"github.com/muesli/obs-cli/internal/recordingsession"
)

// RecordingSessionOBS adapts the existing connection, with path-only filter writes.
type RecordingSessionOBS struct {
	Client *goobs.Client
}

var _ recordingsession.OBS = RecordingSessionOBS{}

func (o RecordingSessionOBS) RecordActive() (bool, error) {
	r, err := o.Client.Record.GetRecordStatus()
	if err != nil {
		return false, err
	}
	return r.OutputActive, nil
}

func (o RecordingSessionOBS) StreamActive() (bool, error) {
	r, err := o.Client.Stream.GetStreamStatus()
	if err != nil {
		return false, err
	}
	return r.OutputActive, nil
}

func (o RecordingSessionOBS) SceneCollection() (string, error) {
	r, err := o.Client.Config.GetSceneCollectionList()
	if err != nil {
		return "", err
	}
	return r.CurrentSceneCollectionName, nil
}

func (o RecordingSessionOBS) Profile() (string, error) {
	r, err := o.Client.Config.GetProfileList()
	if err != nil {
		return "", err
	}
	return r.CurrentProfileName, nil
}

func (o RecordingSessionOBS) Scene() (string, error) {
	r, err := o.Client.Scenes.GetCurrentProgramScene()
	if err != nil {
		return "", err
	}
	if r.SceneName != "" {
		return r.SceneName, nil
	}
	return r.CurrentProgramSceneName, nil
}

func (o RecordingSessionOBS) RecordDirectory() (string, error) {
	r, err := o.Client.Config.GetRecordDirectory()
	if err != nil {
		return "", err
	}
	return r.RecordDirectory, nil
}

func (o RecordingSessionOBS) SourceFilter(source, filter string) (recordingsession.Filter, error) {
	r, err := o.Client.Filters.GetSourceFilter(filters.NewGetSourceFilterParams().WithSourceName(source).WithFilterName(filter))
	if err != nil {
		return recordingsession.Filter{}, err
	}
	path, ok := r.FilterSettings["path"].(string)
	if !ok {
		return recordingsession.Filter{}, fmt.Errorf("filter %q on %q has no string path setting to snapshot", filter, source)
	}
	return recordingsession.Filter{Kind: r.FilterKind, Enabled: r.FilterEnabled, Path: path}, nil
}

func (o RecordingSessionOBS) SetRecordDirectory(path string) error {
	_, err := o.Client.Config.SetRecordDirectory(obsconfig.NewSetRecordDirectoryParams().WithRecordDirectory(path))
	return err
}

func (o RecordingSessionOBS) SetFilterPath(source, filter, path string) error {
	_, err := o.Client.Filters.SetSourceFilterSettings(filters.NewSetSourceFilterSettingsParams().
		WithSourceName(source).WithFilterName(filter).WithOverlay(true).
		WithFilterSettings(map[string]any{"path": path}))
	return err
}

func (o RecordingSessionOBS) StartRecord() error {
	_, err := o.Client.Record.StartRecord()
	return err
}
