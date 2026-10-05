//go:build windows

package osops

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procGetForegroundWindow      = user32.NewProc("GetForegroundWindow")
	procGetWindowTextW           = user32.NewProc("GetWindowTextW")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procMessageBoxW              = user32.NewProc("MessageBoxW")
	procGetSystemPowerStatus     = kernel32.NewProc("GetSystemPowerStatus")
)

// Info reports hostname, logged-in user and a friendly Windows product name.
func Info() SysInfo {
	host, _ := os.Hostname()
	user := ""
	if u := os.Getenv("USERNAME"); u != "" {
		user = u
	}
	return SysInfo{Hostname: host, Username: user, OS: productName()}
}

// productName reads the human Windows edition from the registry, e.g.
// "Windows 10 Pro". Falls back to a generic string.
func productName() string {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE,
		`SOFTWARE\Microsoft\Windows NT\CurrentVersion`, registry.QUERY_VALUE)
	if err != nil {
		return "Windows"
	}
	defer k.Close()
	name, _, err := k.GetStringValue("ProductName")
	if err != nil || name == "" {
		return "Windows"
	}
	if rel, _, e := k.GetStringValue("DisplayVersion"); e == nil && rel != "" {
		return name + " " + rel
	}
	return name
}

// ActiveWindow returns the foreground window's executable and title.
func ActiveWindow() Foreground {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return Foreground{}
	}
	// Title
	buf := make([]uint16, 512)
	procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	title := windows.UTF16ToString(buf)

	// Owning process executable
	var pid uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	app := exeName(pid)
	return Foreground{App: app, Title: title}
}

func exeName(pid uint32) string {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(h)
	buf := make([]uint16, windows.MAX_PATH)
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &size); err != nil {
		return ""
	}
	return filepath.Base(windows.UTF16ToString(buf[:size]))
}

// systemPowerStatus mirrors SYSTEM_POWER_STATUS.
type systemPowerStatus struct {
	ACLineStatus        byte
	BatteryFlag         byte
	BatteryLifePercent  byte
	SystemStatusFlag    byte
	BatteryLifeTime     uint32
	BatteryFullLifeTime uint32
}

// Battery returns the charge percent, or -1 if there is no battery.
func Battery() int {
	var s systemPowerStatus
	r, _, _ := procGetSystemPowerStatus.Call(uintptr(unsafe.Pointer(&s)))
	if r == 0 || s.BatteryFlag == 128 /* no system battery */ || s.BatteryLifePercent == 255 {
		return -1
	}
	return int(s.BatteryLifePercent)
}

// Power performs a shutdown/reboot/logoff/sleep via the system tools, which is
// the most reliable cross-version approach.
func Power(action string, delaySec int) error {
	d := strconv.Itoa(delaySec)
	switch action {
	case "shutdown":
		return run("shutdown", "/s", "/t", d, "/f")
	case "reboot":
		return run("shutdown", "/r", "/t", d, "/f")
	case "logoff":
		return run("shutdown", "/l")
	case "sleep":
		return run("rundll32.exe", "powrprof.dll,SetSuspendState", "0,1,0")
	default:
		return fmt.Errorf("osops: unknown power action %q", action)
	}
}

// Launch opens a URL in the default browser or runs an application.
func Launch(target string, args []string, isURL bool) error {
	if isURL {
		return run("rundll32", "url.dll,FileProtocolHandler", target)
	}
	cmd := exec.Command(target, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: false}
	return cmd.Start()
}

// ShowMessage shows a native message box. A non-zero timeout is advisory only
// (MessageBox has no built-in timeout), so it is ignored here; timed toasts are
// handled by the UI layer when needed.
func ShowMessage(title, body string, timeoutSec int) error {
	t, _ := windows.UTF16PtrFromString(title)
	b, _ := windows.UTF16PtrFromString(body)
	const mbTopmost = 0x00040000
	const mbIconInfo = 0x00000040
	go procMessageBoxW.Call(0, uintptr(unsafe.Pointer(b)), uintptr(unsafe.Pointer(t)), mbTopmost|mbIconInfo)
	return nil
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Start()
}
