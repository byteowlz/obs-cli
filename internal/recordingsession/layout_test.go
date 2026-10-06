package recordingsession

import (
	"math"
	"testing"
)

func TestScreenLayoutNativeAspectEvenOutputNoUpscale(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		desktop, base, output Size
		cap                   int
	}{
		{"selected screen", Size{3600, 2338}, Size{3600, 2338}, Size{1920, 1246}, 1920},
		{"portrait", Size{2338, 3600}, Size{2338, 3600}, Size{1246, 1920}, 1920},
		{"small", Size{640, 480}, Size{640, 480}, Size{640, 480}, 1920},
		{"odd native", Size{1001, 701}, Size{1000, 700}, Size{1000, 700}, 1920},
		{"odd cap", Size{3840, 2160}, Size{3840, 2160}, Size{1278, 718}, 1279},
		{"even rounding", Size{2560, 1440}, Size{2560, 1440}, Size{1920, 1080}, 1920},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l, err := CalculateLayout("screen", tc.desktop, Size{1920, 1080}, tc.cap)
			if err != nil || l.Video.Base != tc.base || l.Video.Output != tc.output {
				t.Fatalf("layout = %+v, %v; want base %+v output %+v", l, err, tc.base, tc.output)
			}
			if l.Desktop.Scale > 1 || l.Video.Output.Width > tc.desktop.Width || l.Video.Output.Height > tc.desktop.Height {
				t.Fatal("small screen upscaled")
			}
			if l.Video.Output.Width%2 != 0 || l.Video.Output.Height%2 != 0 {
				t.Fatal("encoder output must be even")
			}
			assertPIPInsideDesktop(t, l)
		})
	}
}

func assertPIPInsideDesktop(t *testing.T, l Layout) {
	t.Helper()
	d, p := l.Desktop, l.Camera
	if p.X < d.X-1e-8 || p.Y < d.Y-1e-8 || p.X+p.Width > d.X+d.Width+1e-8 || p.Y+p.Height > d.Y+d.Height+1e-8 || p.Scale <= 0 {
		t.Fatalf("PIP outside desktop content: %+v", l)
	}
}

func TestFixedLayoutLetterboxAndCameraAspect(t *testing.T) {
	for _, desktop := range []Size{{3600, 2338}, {1080, 1920}, {3840, 1080}, {16384, 2}} {
		for _, camera := range []Size{{1920, 1080}, {1080, 1920}, {2, 16384}} {
			l, err := CalculateLayout("16:9", desktop, camera, 1920)
			if err != nil || l.Video != (Video{Size{1920, 1080}, Size{1920, 1080}}) {
				t.Fatalf("fixed layout = %+v, %v", l, err)
			}
			assertPIPInsideDesktop(t, l)
			d, p := l.Desktop, l.Camera
			if math.Abs(d.Width/d.Height-float64(desktop.Width)/float64(desktop.Height)) > 1e-8 ||
				math.Abs(p.Width/p.Height-float64(camera.Width)/float64(camera.Height)) > 1e-8 {
				t.Fatal("layout distorted source aspect")
			}
			if math.Abs((1920-d.Width)/2-d.X) > 1e-8 || math.Abs((1080-d.Height)/2-d.Y) > 1e-8 {
				t.Fatal("desktop not centered")
			}
		}
	}
	l, err := CalculateLayout("16:9", Size{3600, 2338}, Size{1920, 1080}, 1280)
	if err != nil || l.Video.Base != (Size{1280, 720}) || l.Video.Output != l.Video.Base {
		t.Fatalf("fixed cap ignored: %+v, %v", l, err)
	}
	if math.Abs(l.Camera.Width-l.Desktop.Width/4) > 1e-8 || math.Abs(l.Desktop.X+l.Desktop.Width-l.Camera.X-l.Camera.Width-l.Desktop.Width*0.0125) > 1e-8 {
		t.Fatal("quarter-width PIP or width-based inset incorrect")
	}
}

func TestInvalidLayout(t *testing.T) {
	for _, size := range []Size{{}, {0, 1080}, {1920, 0}, {-1, 20}, {1, 20}, {16385, 20}, {20, 16385}} {
		if _, err := CalculateLayout("screen", size, Size{1920, 1080}, 1920); err == nil {
			t.Fatalf("invalid desktop dimensions accepted: %+v", size)
		}
		if _, err := CalculateLayout("screen", Size{1920, 1080}, size, 1920); err == nil {
			t.Fatalf("invalid camera dimensions accepted: %+v", size)
		}
	}
	for _, mode := range []string{"", "wide", "SCREEN", "keep"} {
		if _, err := CalculateLayout(mode, Size{1920, 1080}, Size{1920, 1080}, 1920); err == nil {
			t.Fatalf("uncalculable composition accepted: %q", mode)
		}
	}
	for _, cap := range []int{-1, 0, 1, 16385} {
		if _, err := CalculateLayout("screen", Size{1920, 1080}, Size{1920, 1080}, cap); err == nil {
			t.Fatalf("invalid cap accepted: %d", cap)
		}
	}
}
