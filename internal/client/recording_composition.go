package client

import (
	"fmt"
	"math"

	obsconfig "github.com/andreykaipov/goobs/api/requests/config"
	"github.com/andreykaipov/goobs/api/requests/inputs"
	"github.com/andreykaipov/goobs/api/requests/sceneitems"
	"github.com/andreykaipov/goobs/api/typedefs"
	"github.com/muesli/obs-cli/internal/recordingsession"
)

var _ recordingsession.CompositionOBS = RecordingSessionOBS{}

func (o RecordingSessionOBS) DesktopInput(source string) (recordingsession.DesktopInput, error) {
	r, err := o.Client.Inputs.GetInputSettings(inputs.NewGetInputSettingsParams().WithInputName(source))
	if err != nil {
		return recordingsession.DesktopInput{}, err
	}
	// GetInputSettings omits defaults; screen_capture's default type is display (0).
	captureType := 0
	if value, exists := r.InputSettings["type"]; exists {
		v, ok := value.(float64)
		if !ok || v != math.Trunc(v) || v < 0 || v > 2 {
			return recordingsession.DesktopInput{}, fmt.Errorf("desktop input has invalid capture type %v", value)
		}
		captureType = int(v)
	}
	uuid, _ := r.InputSettings["display_uuid"].(string)
	return recordingsession.DesktopInput{Kind: r.InputKind, DisplayUUID: uuid, CaptureType: captureType}, nil
}

func (o RecordingSessionOBS) SelectScreen(source, uuid string) error {
	_, err := o.Client.Inputs.SetInputSettings(inputs.NewSetInputSettingsParams().WithInputName(source).
		WithOverlay(true).WithInputSettings(map[string]any{"display_uuid": uuid, "type": 0}))
	return err
}

func nativeSize(width, height float64) (recordingsession.Size, error) {
	// Zero is passed through for bounded readiness polling, not accepted as ready.
	if math.IsNaN(width) || math.IsNaN(height) || math.IsInf(width, 0) || math.IsInf(height, 0) ||
		width < 0 || height < 0 || width > recordingsession.MaxDimension || height > recordingsession.MaxDimension ||
		width != math.Trunc(width) || height != math.Trunc(height) {
		return recordingsession.Size{}, fmt.Errorf("invalid native dimensions %vx%v", width, height)
	}
	return recordingsession.Size{Width: int(width), Height: int(height)}, nil
}

func (o RecordingSessionOBS) SceneSource(scene, source string) (recordingsession.SceneSource, error) {
	id, err := o.Client.SceneItems.GetSceneItemId(sceneitems.NewGetSceneItemIdParams().WithSceneName(scene).WithSourceName(source))
	if err != nil {
		return recordingsession.SceneSource{}, err
	}
	r, err := o.Client.SceneItems.GetSceneItemTransform(sceneitems.NewGetSceneItemTransformParams().WithSceneName(scene).WithSceneItemId(id.SceneItemId))
	if err != nil {
		return recordingsession.SceneSource{}, err
	}
	if r.SceneItemTransform == nil {
		return recordingsession.SceneSource{}, fmt.Errorf("missing scene item transform for %q", source)
	}
	size, err := nativeSize(r.SceneItemTransform.SourceWidth, r.SceneItemTransform.SourceHeight)
	return recordingsession.SceneSource{ID: id.SceneItemId, Size: size}, err
}

func (o RecordingSessionOBS) VideoSettings() (recordingsession.Video, error) {
	r, err := o.Client.Config.GetVideoSettings()
	if err != nil {
		return recordingsession.Video{}, err
	}
	base, err := nativeSize(r.BaseWidth, r.BaseHeight)
	if err != nil {
		return recordingsession.Video{}, err
	}
	output, err := nativeSize(r.OutputWidth, r.OutputHeight)
	return recordingsession.Video{Base: base, Output: output}, err
}

func (o RecordingSessionOBS) SetVideoSettings(v recordingsession.Video) error {
	_, err := o.Client.Config.SetVideoSettings(obsconfig.NewSetVideoSettingsParams().
		WithBaseWidth(float64(v.Base.Width)).WithBaseHeight(float64(v.Base.Height)).
		WithOutputWidth(float64(v.Output.Width)).WithOutputHeight(float64(v.Output.Height)))
	return err
}

func (o RecordingSessionOBS) SetPlacement(scene string, id int, p recordingsession.Placement) error {
	// goobs serializes all transform fields. OBS ignores read-only size fields;
	// writable fields explicitly reset crop/rotation/bounds for an uncropped fit.
	_, err := o.Client.SceneItems.SetSceneItemTransform(sceneitems.NewSetSceneItemTransformParams().
		WithSceneName(scene).WithSceneItemId(id).WithSceneItemTransform(&typedefs.SceneItemTransform{
		Alignment: 5, BoundsType: "OBS_BOUNDS_NONE", PositionX: p.X, PositionY: p.Y,
		// OBS validates serialized bounds even when bounds are disabled.
		BoundsWidth: math.Max(1, p.Width), BoundsHeight: math.Max(1, p.Height),
		ScaleX: p.Scale, ScaleY: p.Scale,
	}))
	return err
}
