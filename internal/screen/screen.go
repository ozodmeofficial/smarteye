// Package screen captures the client's display(s). The real implementation is
// Windows-only (GDI BitBlt, no cgo); on other platforms a placeholder capturer
// lets the rest of SmartEYE build and run during development.
package screen

import (
	"bytes"
	"image"
	"image/jpeg"

	"github.com/ozodmeofficial/smarteye/internal/protocol"
)

// Monitor describes one display.
type Monitor = protocol.MonitorInfo

// Capturer grabs frames from the client's displays.
type Capturer interface {
	// Monitors lists the available displays.
	Monitors() ([]Monitor, error)
	// Capture returns the current contents of the given monitor index.
	Capture(monitor int) (*image.RGBA, error)
	// Close releases any OS resources.
	Close() error
}

// New returns the platform capturer.
func New() (Capturer, error) { return newCapturer() }

// EncodeJPEG encodes an image to JPEG bytes at the given quality (1-100),
// optionally downscaling so the longest side is at most maxWidth (0 = no scale).
func EncodeJPEG(img image.Image, quality, maxWidth int) ([]byte, int, int, error) {
	scaled := img
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if maxWidth > 0 && w > maxWidth {
		nh := h * maxWidth / w
		scaled = downscale(img, maxWidth, nh)
		w, h = maxWidth, nh
	}
	var buf bytes.Buffer
	if quality <= 0 || quality > 100 {
		quality = 70
	}
	if err := jpeg.Encode(&buf, scaled, &jpeg.Options{Quality: quality}); err != nil {
		return nil, 0, 0, err
	}
	return buf.Bytes(), w, h, nil
}

// downscale does a fast box-sampling resize. It is good enough for preview
// thumbnails and avoids pulling in an image-processing dependency.
func downscale(src image.Image, dw, dh int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	for y := 0; y < dh; y++ {
		sy := sb.Min.Y + y*sh/dh
		for x := 0; x < dw; x++ {
			sx := sb.Min.X + x*sw/dw
			r, g, b, a := src.At(sx, sy).RGBA()
			i := dst.PixOffset(x, y)
			dst.Pix[i+0] = uint8(r >> 8)
			dst.Pix[i+1] = uint8(g >> 8)
			dst.Pix[i+2] = uint8(b >> 8)
			dst.Pix[i+3] = uint8(a >> 8)
		}
	}
	return dst
}
