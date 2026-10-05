//go:build windows

package control

import (
	"fmt"
	"unsafe"

	"github.com/ozodmeofficial/smarteye/internal/protocol"
	"golang.org/x/sys/windows"
)

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procSendInput        = user32.NewProc("SendInput")
	procGetSystemMetrics = user32.NewProc("GetSystemMetrics")

	procOpenClipboard    = user32.NewProc("OpenClipboard")
	procCloseClipboard   = user32.NewProc("CloseClipboard")
	procEmptyClipboard   = user32.NewProc("EmptyClipboard")
	procGetClipboardData = user32.NewProc("GetClipboardData")
	procSetClipboardData = user32.NewProc("SetClipboardData")

	procGlobalAlloc  = kernel32.NewProc("GlobalAlloc")
	procGlobalLock   = kernel32.NewProc("GlobalLock")
	procGlobalUnlock = kernel32.NewProc("GlobalUnlock")
)

const (
	inputMouse    = 0
	inputKeyboard = 1

	meMove        = 0x0001
	meLeftDown    = 0x0002
	meLeftUp      = 0x0004
	meRightDown   = 0x0008
	meRightUp     = 0x0010
	meMiddleDown  = 0x0020
	meMiddleUp    = 0x0040
	meWheel       = 0x0800
	meAbsolute    = 0x8000
	meVirtualDesk = 0x4000

	keyEventKeyUp = 0x0002

	smXVirtualScreen  = 76
	smYVirtualScreen  = 77
	smCXVirtualScreen = 78
	smCYVirtualScreen = 79

	cfUnicodeText = 13
	gmemMoveable  = 0x0002
)

// Both structs are laid out to the exact 40-byte size of INPUT on amd64.
type mouseInput struct {
	typ       uint32
	_         uint32
	dx        int32
	dy        int32
	mouseData uint32
	dwFlags   uint32
	time      uint32
	_         uint32
	extraInfo uintptr
}

type keybdInput struct {
	typ       uint32
	_         uint32
	wVk       uint16
	wScan     uint16
	dwFlags   uint32
	time      uint32
	_         uint32
	extraInfo uintptr
	_         uint64
}

type winInjector struct{}

func newInjector() Injector { return &winInjector{} }

func metric(i int) int32 {
	r, _, _ := procGetSystemMetrics.Call(uintptr(i))
	return int32(r)
}

func (w *winInjector) Apply(ev protocol.InputEvent, monW, monH, monX, monY int) error {
	switch ev.Kind {
	case "mouse_move", "mouse_down", "mouse_up", "wheel":
		return w.mouse(ev, monW, monH, monX, monY)
	case "key_down", "key_up":
		return w.key(ev)
	default:
		return nil
	}
}

func (w *winInjector) mouse(ev protocol.InputEvent, monW, monH, monX, monY int) error {
	vx, vy := metric(smXVirtualScreen), metric(smYVirtualScreen)
	vw, vh := metric(smCXVirtualScreen), metric(smCYVirtualScreen)
	if vw <= 0 || vh <= 0 {
		return fmt.Errorf("control: bad virtual screen size")
	}
	// Map normalized monitor coords -> absolute virtual-desktop coords (0..65535).
	px := float64(monX) + ev.X*float64(monW)
	py := float64(monY) + ev.Y*float64(monH)
	absX := int32((px - float64(vx)) * 65535.0 / float64(vw))
	absY := int32((py - float64(vy)) * 65535.0 / float64(vh))

	in := mouseInput{typ: inputMouse, dx: absX, dy: absY}
	switch ev.Kind {
	case "mouse_move":
		in.dwFlags = meMove | meAbsolute | meVirtualDesk
	case "wheel":
		in.dwFlags = meWheel
		in.mouseData = uint32(int32(ev.Delta))
	case "mouse_down":
		in.dwFlags = meAbsolute | meVirtualDesk | buttonFlag(ev.Button, true)
	case "mouse_up":
		in.dwFlags = meAbsolute | meVirtualDesk | buttonFlag(ev.Button, false)
	}
	return send(unsafe.Pointer(&in))
}

func buttonFlag(button string, down bool) uint32 {
	switch button {
	case "right":
		if down {
			return meRightDown
		}
		return meRightUp
	case "middle":
		if down {
			return meMiddleDown
		}
		return meMiddleUp
	default: // left
		if down {
			return meLeftDown
		}
		return meLeftUp
	}
}

func (w *winInjector) key(ev protocol.InputEvent) error {
	in := keybdInput{typ: inputKeyboard, wVk: uint16(ev.KeyCode)}
	if ev.Kind == "key_up" {
		in.dwFlags = keyEventKeyUp
	}
	return send(unsafe.Pointer(&in))
}

func send(p unsafe.Pointer) error {
	const sizeofInput = 40
	r, _, err := procSendInput.Call(1, uintptr(p), sizeofInput)
	if r != 1 {
		return fmt.Errorf("control: SendInput failed: %v", err)
	}
	return nil
}

// --- Clipboard --------------------------------------------------------------

func (w *winInjector) SetClipboard(text string) error {
	u16, err := windows.UTF16FromString(text)
	if err != nil {
		return err
	}
	size := uintptr(len(u16) * 2)

	if r, _, _ := procOpenClipboard.Call(0); r == 0 {
		return fmt.Errorf("control: OpenClipboard failed")
	}
	defer procCloseClipboard.Call()
	procEmptyClipboard.Call()

	h, _, _ := procGlobalAlloc.Call(gmemMoveable, size)
	if h == 0 {
		return fmt.Errorf("control: GlobalAlloc failed")
	}
	ptr, _, _ := procGlobalLock.Call(h)
	if ptr == 0 {
		return fmt.Errorf("control: GlobalLock failed")
	}
	// Copy UTF-16 string into the global memory.
	dst := unsafe.Slice((*uint16)(unsafe.Pointer(ptr)), len(u16))
	copy(dst, u16)
	procGlobalUnlock.Call(h)

	if r, _, _ := procSetClipboardData.Call(cfUnicodeText, h); r == 0 {
		return fmt.Errorf("control: SetClipboardData failed")
	}
	return nil
}

func (w *winInjector) GetClipboard() (string, error) {
	if r, _, _ := procOpenClipboard.Call(0); r == 0 {
		return "", fmt.Errorf("control: OpenClipboard failed")
	}
	defer procCloseClipboard.Call()

	h, _, _ := procGetClipboardData.Call(cfUnicodeText)
	if h == 0 {
		return "", nil // clipboard has no text
	}
	ptr, _, _ := procGlobalLock.Call(h)
	if ptr == 0 {
		return "", fmt.Errorf("control: GlobalLock failed")
	}
	defer procGlobalUnlock.Call(h)
	return windows.UTF16PtrToString((*uint16)(unsafe.Pointer(ptr))), nil
}
