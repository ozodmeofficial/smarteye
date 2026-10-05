//go:build windows

package osops

import (
	"runtime"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	gdi32L = windows.NewLazySystemDLL("gdi32.dll")

	procRegisterClassExW = user32.NewProc("RegisterClassExW")
	procCreateWindowExW  = user32.NewProc("CreateWindowExW")
	procDefWindowProcW   = user32.NewProc("DefWindowProcW")
	procShowWindow       = user32.NewProc("ShowWindow")
	procUpdateWindow     = user32.NewProc("UpdateWindow")
	procGetMessageW      = user32.NewProc("GetMessageW")
	procDispatchMessageW = user32.NewProc("DispatchMessageW")
	procTranslateMessage = user32.NewProc("TranslateMessage")
	procPostQuitMessage  = user32.NewProc("PostQuitMessage")
	procPostMessageW     = user32.NewProc("PostMessageW")
	procDestroyWindow    = user32.NewProc("DestroyWindow")
	procBeginPaint       = user32.NewProc("BeginPaint")
	procEndPaint         = user32.NewProc("EndPaint")
	procFillRect         = user32.NewProc("FillRect")
	procDrawTextW        = user32.NewProc("DrawTextW")
	procSetWindowPos     = user32.NewProc("SetWindowPos")
	procSetTimer         = user32.NewProc("SetTimer")
	procInvalidateRect   = user32.NewProc("InvalidateRect")
	procGetSystemMetrics = user32.NewProc("GetSystemMetrics")
	procLoadCursorW      = user32.NewProc("LoadCursorW")

	procCreateSolidBrush = gdi32L.NewProc("CreateSolidBrush")
	procCreateFontW      = gdi32L.NewProc("CreateFontW")
	procSelectObject     = gdi32L.NewProc("SelectObject")
	procSetTextColor     = gdi32L.NewProc("SetTextColor")
	procSetBkMode        = gdi32L.NewProc("SetBkMode")
	procDeleteObject     = gdi32L.NewProc("DeleteObject")
)

const (
	wsPopup   = 0x80000000
	wsVisible = 0x10000000

	wsExTopmost     = 0x00000008
	wsExToolWindow  = 0x00000080

	swShow = 5

	wmDestroy = 0x0002
	wmClose   = 0x0010
	wmPaint   = 0x000F
	wmTimer   = 0x0113
	wmEraseBg = 0x0014

	swpNoMove   = 0x0002
	swpNoSize   = 0x0001
	swpShow     = 0x0040
	hwndTopmost = ^uintptr(0) // (HWND)-1

	dtCenter   = 0x0001
	dtVCenter  = 0x0004
	dtWordBrk  = 0x0010
	dtNoClip   = 0x0100

	transparent = 1

	smCXScreen = 0
	smCYScreen = 1
	smCXVirt   = 78
	smCYVirt   = 79
	smXVirt    = 76
	smYVirt    = 77

	idcArrow = 32512
)

type wndClassExW struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     windows.Handle
	hIcon         windows.Handle
	hCursor       windows.Handle
	hbrBackground windows.Handle
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       windows.Handle
}

type msg struct {
	hwnd    windows.Handle
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      struct{ x, y int32 }
}

type paintStruct struct {
	hdc         windows.Handle
	fErase      int32
	rcPaint     rectL
	fRestore    int32
	fIncUpdate  int32
	rgbReserved [32]byte
}

type rectL struct{ Left, Top, Right, Bottom int32 }

// winLocker drives a single fullscreen overlay window on a dedicated OS thread.
type winLocker struct {
	mu       sync.Mutex
	hwnd     windows.Handle
	locked   bool
	title    string
	message  string
	classReg bool
	className *uint16
}

var theLocker = &winLocker{}

func newLocker() Locker { return theLocker }

func metric(i int) int32 {
	r, _, _ := procGetSystemMetrics.Call(uintptr(i))
	return int32(r)
}

func (l *winLocker) Show(title, message string) error {
	l.mu.Lock()
	l.title = title
	l.message = message
	already := l.locked
	hwnd := l.hwnd
	l.mu.Unlock()

	if already && hwnd != 0 {
		// Just refresh the text.
		procInvalidateRect.Call(uintptr(hwnd), 0, 1)
		return nil
	}

	ready := make(chan error, 1)
	go l.run(ready)
	return <-ready
}

func (l *winLocker) Hide() error {
	l.mu.Lock()
	hwnd := l.hwnd
	l.locked = false
	l.mu.Unlock()
	if hwnd != 0 {
		procPostMessageW.Call(uintptr(hwnd), wmClose, 0, 0)
	}
	return nil
}

func (l *winLocker) IsLocked() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.locked
}

// run creates the window and pumps its message loop. It must stay on one OS
// thread for the lifetime of the window.
func (l *winLocker) run(ready chan<- error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// hInstance 0 is accepted for our custom window class and is simplest here.
	var hInst windows.Handle

	l.mu.Lock()
	if !l.classReg {
		l.className, _ = windows.UTF16PtrFromString("SmartEYELockClass")
		black, _, _ := procCreateSolidBrush.Call(0x00140F0D) // near-black (BGR)
		cursor, _, _ := procLoadCursorW.Call(0, idcArrow)
		wc := wndClassExW{
			cbSize:        uint32(unsafe.Sizeof(wndClassExW{})),
			lpfnWndProc:   windows.NewCallback(wndProc),
			hInstance:     hInst,
			hCursor:       windows.Handle(cursor),
			hbrBackground: windows.Handle(black),
			lpszClassName: l.className,
		}
		if r, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
			l.mu.Unlock()
			ready <- err
			return
		}
		l.classReg = true
	}
	className := l.className
	l.mu.Unlock()

	x := metric(smXVirt)
	y := metric(smYVirt)
	w := metric(smCXVirt)
	h := metric(smCYVirt)
	if w == 0 || h == 0 {
		w, h = metric(smCXScreen), metric(smCYScreen)
	}
	title, _ := windows.UTF16PtrFromString("SmartEYE")

	hwndR, _, err := procCreateWindowExW.Call(
		wsExTopmost|wsExToolWindow,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(title)),
		wsPopup|wsVisible,
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		0, 0, uintptr(hInst), 0,
	)
	if hwndR == 0 {
		ready <- err
		return
	}
	hwnd := windows.Handle(hwndR)

	l.mu.Lock()
	l.hwnd = hwnd
	l.locked = true
	l.mu.Unlock()

	procShowWindow.Call(hwndR, swShow)
	procUpdateWindow.Call(hwndR)
	// Keep it pinned above everything; re-assert every 500ms.
	procSetTimer.Call(hwndR, 1, 500, 0)
	procSetWindowPos.Call(hwndR, hwndTopmost, 0, 0, 0, 0, swpNoMove|swpNoSize|swpShow)

	ready <- nil

	// Message loop.
	var m msg
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 { // 0 = WM_QUIT, -1 = error
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}

	l.mu.Lock()
	l.hwnd = 0
	l.locked = false
	l.mu.Unlock()
}

// wndProc handles paint, topmost re-assertion and teardown.
func wndProc(hwnd windows.Handle, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case wmEraseBg:
		return 1 // we paint the background ourselves; avoid flicker
	case wmTimer:
		procSetWindowPos.Call(uintptr(hwnd), hwndTopmost, 0, 0, 0, 0, swpNoMove|swpNoSize|swpShow)
		return 0
	case wmPaint:
		paintLock(hwnd)
		return 0
	case wmClose:
		procDestroyWindow.Call(uintptr(hwnd))
		return 0
	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(uintptr(hwnd), uintptr(message), wParam, lParam)
	return r
}

func paintLock(hwnd windows.Handle) {
	var ps paintStruct
	hdcR, _, _ := procBeginPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
	defer procEndPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))

	// Background.
	bg, _, _ := procCreateSolidBrush.Call(0x00140F0D)
	procFillRect.Call(hdcR, uintptr(unsafe.Pointer(&ps.rcPaint)), bg)
	procDeleteObject.Call(bg)

	full := rectL{ps.rcPaint.Left, ps.rcPaint.Top, ps.rcPaint.Right, ps.rcPaint.Bottom}
	procSetBkMode.Call(hdcR, transparent)

	theLocker.mu.Lock()
	title := theLocker.title
	message := theLocker.message
	theLocker.mu.Unlock()
	if title == "" {
		title = "Kompyuter vaqtincha bloklangan"
	}

	// Title: large warm-white text, upper third.
	titleFont, _, _ := procCreateFontW.Call(64, 0, 0, 0, 700, 0, 0, 0, 1, 0, 0, 4, 0,
		uintptr(unsafe.Pointer(utf16("Segoe UI"))))
	oldF, _, _ := procSelectObject.Call(hdcR, titleFont)
	procSetTextColor.Call(hdcR, 0x00E8ECEF) // light (BGR)
	tr := rectL{full.Left, full.Top + (full.Bottom-full.Top)/3, full.Right, full.Top + (full.Bottom-full.Top)/2}
	drawText(hdcR, title, &tr)
	procSelectObject.Call(hdcR, oldF)
	procDeleteObject.Call(titleFont)

	// Message: smaller, warm accent, centered below.
	if message != "" {
		msgFont, _, _ := procCreateFontW.Call(32, 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 4, 0,
			uintptr(unsafe.Pointer(utf16("Segoe UI"))))
		oldF2, _, _ := procSelectObject.Call(hdcR, msgFont)
		procSetTextColor.Call(hdcR, 0x005B7FD9) // terracotta-ish (BGR of #D97F5B)
		mr := rectL{full.Left + 80, full.Top + (full.Bottom-full.Top)/2, full.Right - 80, full.Bottom - 80}
		drawText(hdcR, message, &mr)
		procSelectObject.Call(hdcR, oldF2)
		procDeleteObject.Call(msgFont)
	}
}

func drawText(hdc uintptr, text string, r *rectL) {
	p := utf16(text)
	procDrawTextW.Call(hdc, uintptr(unsafe.Pointer(p)), ^uintptr(0),
		uintptr(unsafe.Pointer(r)), dtCenter|dtWordBrk|dtNoClip)
}

func utf16(s string) *uint16 {
	p, _ := windows.UTF16PtrFromString(s)
	return p
}
