//go:build !windows

package screen

import (
	"image"
	"image/color"
	"time"
)

// placeholderCapturer renders a moving gradient so the pipeline can be
// developed and tested on non-Windows machines. It reports a single 1280x720
// "display".
type placeholderCapturer struct{}

func newCapturer() (Capturer, error) { return &placeholderCapturer{}, nil }

func (p *placeholderCapturer) Monitors() ([]Monitor, error) {
	return []Monitor{{Index: 0, Width: 1280, Height: 720, Primary: true}}, nil
}

func (p *placeholderCapturer) Capture(monitor int) (*image.RGBA, error) {
	const w, h = 1280, 720
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	// Animate with the current second so previews visibly update in dev.
	t := time.Now().UnixMilli() / 40
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8((x + int(t)) % 256),
				G: uint8((y + int(t)/2) % 256),
				B: uint8((x + y) % 256),
				A: 255,
			})
		}
	}
	return img, nil
}

func (p *placeholderCapturer) Close() error { return nil }
