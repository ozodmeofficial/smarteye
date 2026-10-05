//go:build windows

package screen

import (
	"fmt"
	"image"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32 = windows.NewLazySystemDLL("user32.dll")
	gdi32  = windows.NewLazySystemDLL("gdi32.dll")

	procGetDC               = user32.NewProc("GetDC")
	procReleaseDC           = user32.NewProc("ReleaseDC")
	procGetSystemMetrics    = user32.NewProc("GetSystemMetrics")
	procEnumDisplayMonitors = user32.NewProc("EnumDisplayMonitors")
	procGetMonitorInfoW     = user32.NewProc("GetMonitorInfoW")
	procSetProcessDPIAware  = user32.NewProc("SetProcessDPIAware")

	procCreateCompatibleDC     = gdi32.NewProc("CreateCompatibleDC")
	procCreateCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	procSelectObject           = gdi32.NewProc("SelectObject")
	procBitBlt                 = gdi32.NewProc("BitBlt")
	procDeleteObject           = gdi32.NewProc("DeleteObject")
	procDeleteDC               = gdi32.NewProc("DeleteDC")
	procGetDIBits              = gdi32.NewProc("GetDIBits")
)

const (
	smXVirtualScreen  = 76
	smYVirtualScreen  = 77
	smCXVirtualScreen = 78
	smCYVirtualScreen = 79

	srcCopy      = 0x00CC0020
	captureBLT   = 0x40000000 // include layered windows
	biRGB        = 0
	dibRGBColors = 0
)

type rect struct{ Left, Top, Right, Bottom int32 }

type monitorInfo struct {
	CbSize    uint32
	RcMonitor rect
	RcWork    rect
	DwFlags   uint32
}

type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

type bitmapInfo struct {
	Header bitmapInfoHeader
	Colors [1]uint32
}

type winCapturer struct {
	mu       sync.Mutex
	monitors []monRect
}

type monRect struct {
	info Monitor
	r    rect
}

func newCapturer() (Capturer, error) {
	// Report true pixels on high-DPI displays.
	procSetProcessDPIAware.Call()
	c := &winCapturer{}
	if err := c.refresh(); err != nil {
		return nil, err
	}
	return c, nil
}

// refresh re-enumerates monitors (called on construction and when the list
// looks empty, e.g. after a display change).
func (c *winCapturer) refresh() error {
	var mons []monRect
	cb := windows.NewCallback(func(hMonitor, hdc uintptr, lprc *rect, lparam uintptr) uintptr {
		var mi monitorInfo
		mi.CbSize = uint32(unsafe.Sizeof(mi))
		r, _, _ := procGetMonitorInfoW.Call(hMonitor, uintptr(unsafe.Pointer(&mi)))
		if r == 0 {
			return 1 // continue
		}
		idx := len(mons)
		mons = append(mons, monRect{
			info: Monitor{
				Index:   idx,
				Width:   int(mi.RcMonitor.Right - mi.RcMonitor.Left),
				Height:  int(mi.RcMonitor.Bottom - mi.RcMonitor.Top),
				Primary: mi.DwFlags&1 != 0, // MONITORINFOF_PRIMARY
			},
			r: mi.RcMonitor,
		})
		return 1 // continue enumeration
	})
	procEnumDisplayMonitors.Call(0, 0, cb, 0)

	if len(mons) == 0 {
		// Fall back to the whole virtual screen as a single monitor.
		w, _, _ := procGetSystemMetrics.Call(smCXVirtualScreen)
		h, _, _ := procGetSystemMetrics.Call(smCYVirtualScreen)
		x, _, _ := procGetSystemMetrics.Call(smXVirtualScreen)
		y, _, _ := procGetSystemMetrics.Call(smYVirtualScreen)
		mons = append(mons, monRect{
			info: Monitor{Index: 0, Width: int(int32(w)), Height: int(int32(h)), Primary: true},
			r:    rect{Left: int32(x), Top: int32(y), Right: int32(x) + int32(w), Bottom: int32(y) + int32(h)},
		})
	}
	c.mu.Lock()
	c.monitors = mons
	c.mu.Unlock()
	return nil
}

func (c *winCapturer) Monitors() ([]Monitor, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.monitors) == 0 {
		c.mu.Unlock()
		_ = c.refresh()
		c.mu.Lock()
	}
	out := make([]Monitor, len(c.monitors))
	for i, m := range c.monitors {
		out[i] = m.info
	}
	return out, nil
}

func (c *winCapturer) Capture(monitor int) (*image.RGBA, error) {
	c.mu.Lock()
	if monitor < 0 || monitor >= len(c.monitors) {
		c.mu.Unlock()
		if err := c.refresh(); err != nil {
			return nil, err
		}
		c.mu.Lock()
		if monitor < 0 || monitor >= len(c.monitors) {
			c.mu.Unlock()
			return nil, fmt.Errorf("screen: monitor %d out of range", monitor)
		}
	}
	r := c.monitors[monitor].r
	c.mu.Unlock()

	width := int(r.Right - r.Left)
	height := int(r.Bottom - r.Top)
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("screen: invalid monitor size %dx%d", width, height)
	}

	screenDC, _, _ := procGetDC.Call(0)
	if screenDC == 0 {
		return nil, fmt.Errorf("screen: GetDC failed")
	}
	defer procReleaseDC.Call(0, screenDC)

	memDC, _, _ := procCreateCompatibleDC.Call(screenDC)
	if memDC == 0 {
		return nil, fmt.Errorf("screen: CreateCompatibleDC failed")
	}
	defer procDeleteDC.Call(memDC)

	bmp, _, _ := procCreateCompatibleBitmap.Call(screenDC, uintptr(width), uintptr(height))
	if bmp == 0 {
		return nil, fmt.Errorf("screen: CreateCompatibleBitmap failed")
	}
	defer procDeleteObject.Call(bmp)

	old, _, _ := procSelectObject.Call(memDC, bmp)
	defer procSelectObject.Call(memDC, old)

	ret, _, _ := procBitBlt.Call(
		memDC, 0, 0, uintptr(width), uintptr(height),
		screenDC, uintptr(r.Left), uintptr(r.Top), srcCopy|captureBLT,
	)
	if ret == 0 {
		return nil, fmt.Errorf("screen: BitBlt failed")
	}

	// Request a top-down 32-bit BGRA buffer.
	var bi bitmapInfo
	bi.Header.Size = uint32(unsafe.Sizeof(bi.Header))
	bi.Header.Width = int32(width)
	bi.Header.Height = -int32(height) // negative = top-down
	bi.Header.Planes = 1
	bi.Header.BitCount = 32
	bi.Header.Compression = biRGB

	buf := make([]byte, width*height*4)
	got, _, _ := procGetDIBits.Call(
		memDC, bmp, 0, uintptr(height),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&bi)), dibRGBColors,
	)
	if got == 0 {
		return nil, fmt.Errorf("screen: GetDIBits failed")
	}

	// Convert BGRA -> RGBA in place into an image.RGBA.
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for i := 0; i < len(buf); i += 4 {
		img.Pix[i+0] = buf[i+2] // R
		img.Pix[i+1] = buf[i+1] // G
		img.Pix[i+2] = buf[i+0] // B
		img.Pix[i+3] = 255      // A (opaque; desktop has no alpha)
	}
	return img, nil
}

func (c *winCapturer) Close() error { return nil }
