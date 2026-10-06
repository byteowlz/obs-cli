package recordingsession

import (
	"fmt"
	"math"
)

// MaxDimension is OBS's maximum canvas edge; reject unreasonable capture sizes.
const MaxDimension = 16384

// Size is an unscaled source or video resolution in pixels.
type Size struct{ Width, Height int }

func (s Size) Validate() error {
	if s.Width < 2 || s.Height < 2 || s.Width > MaxDimension || s.Height > MaxDimension {
		return fmt.Errorf("dimensions %dx%d must be between 2 and %d pixels per edge", s.Width, s.Height, MaxDimension)
	}
	return nil
}

// Video contains only resolution settings, never FPS or encoder settings.
type Video struct{ Base, Output Size }

// Placement uses a top-left origin, uniform scaling, no rotation and no crop.
// Width/Height describe the displayed rectangle (not the native source size).
type Placement struct{ X, Y, Width, Height, Scale float64 }

// Layout is the complete program composition; source recordings stay native.
type Layout struct {
	Video           Video
	Desktop, Camera Placement
}

func ValidateComposition(mode string, maxEdge int) error {
	if mode != "screen" && mode != "16:9" && mode != "keep" {
		return fmt.Errorf("composition must be screen, 16:9 or keep (got %q)", mode)
	}
	if maxEdge < 2 || maxEdge > MaxDimension {
		return fmt.Errorf("recording_session.output_max_dimension must be between 2 and %d", MaxDimension)
	}
	return nil
}

func evenFloor(n int) int { return n / 2 * 2 }

// scaledOutput rounds to the nearest even pixel, bounded by canvas and cap.
// Independent even rounding introduces at most one pixel of aspect error.
func scaledOutput(base Size, maxEdge int) Size {
	ratio := math.Min(1, float64(evenFloor(maxEdge))/float64(max(base.Width, base.Height)))
	even := func(n int) int {
		return min(evenFloor(n), max(2, int(math.Round(float64(n)*ratio/2))*2))
	}
	return Size{even(base.Width), even(base.Height)}
}

// CalculateLayout fits Desktop without crop, then anchors a quarter-width camera
// inside its content rectangle, not inside a letterbox bar. Very tall cameras
// are reduced further to fit; the inset is 1.25% of Desktop's displayed width.
func CalculateLayout(mode string, desktop, camera Size, maxEdge int) (Layout, error) {
	if err := ValidateComposition(mode, maxEdge); err != nil {
		return Layout{}, err
	}
	if mode == "keep" {
		return Layout{}, fmt.Errorf("keep composition has no calculated layout")
	}
	for name, size := range map[string]Size{"desktop": desktop, "camera": camera} {
		if err := size.Validate(); err != nil {
			return Layout{}, fmt.Errorf("%s: %w", name, err)
		}
	}
	base := Size{evenFloor(desktop.Width), evenFloor(desktop.Height)}
	if mode == "16:9" {
		base = scaledOutput(Size{1920, 1080}, maxEdge)
	}
	video := Video{Base: base, Output: scaledOutput(base, maxEdge)}
	desktopScale := math.Min(float64(base.Width)/float64(desktop.Width), float64(base.Height)/float64(desktop.Height))
	d := Placement{Width: float64(desktop.Width) * desktopScale, Height: float64(desktop.Height) * desktopScale, Scale: desktopScale}
	d.X, d.Y = (float64(base.Width)-d.Width)/2, (float64(base.Height)-d.Height)/2
	// In extreme panoramic content, bound the inset so a positive PIP still fits.
	inset := math.Min(d.Width*0.0125, d.Height/4)
	cameraScale := math.Min(d.Width/4/float64(camera.Width), (d.Height-2*inset)/float64(camera.Height))
	p := Placement{Width: float64(camera.Width) * cameraScale, Height: float64(camera.Height) * cameraScale, Scale: cameraScale}
	p.X, p.Y = d.X+d.Width-inset-p.Width, d.Y+d.Height-inset-p.Height
	return Layout{Video: video, Desktop: d, Camera: p}, nil
}
