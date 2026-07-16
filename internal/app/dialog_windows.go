//go:build windows

package app

import (
	"syscall"
	"unsafe"
)

// ConfirmYesNo shows a Yes/No dialog anchored near the system tray
// (above the taskbar), so it does not sit on top of the tray icon.
func ConfirmYesNo(title, body string) bool {
	const (
		mbYesNo         = 0x00000004
		mbIconWarning   = 0x00000030
		mbTopmost       = 0x00040000
		mbSetForeground = 0x00010000
		idYes           = 6
	)
	r := messageBoxNearTray(title, body, mbYesNo|mbIconWarning|mbTopmost|mbSetForeground)
	return r == idYes
}

// AlertInfo shows an OK information dialog near the system tray.
func AlertInfo(title, body string) {
	const (
		mbOK            = 0x00000000
		mbIconInfo      = 0x00000040
		mbTopmost       = 0x00040000
		mbSetForeground = 0x00010000
	)
	_ = messageBoxNearTray(title, body, mbOK|mbIconInfo|mbTopmost|mbSetForeground)
}

func messageBoxNearTray(title, body string, flags uint32) int {
	t, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return 0
	}
	m, err := syscall.UTF16PtrFromString(body)
	if err != nil {
		return 0
	}

	owner := createTrayAnchorWindow()
	if owner != 0 {
		defer destroyWindow(owner)
	}

	user32 := syscall.NewLazyDLL("user32.dll")
	proc := user32.NewProc("MessageBoxW")
	r, _, _ := proc.Call(
		owner,
		uintptr(unsafe.Pointer(m)),
		uintptr(unsafe.Pointer(t)),
		uintptr(flags),
	)
	return int(r)
}

// createTrayAnchorWindow places a 1×1 invisible owner window at the
// notification-area corner of the work area (just above/ beside the taskbar).
// MessageBox centers relative to this owner → appears near the tray.
func createTrayAnchorWindow() uintptr {
	user32 := syscall.NewLazyDLL("user32.dll")
	x, y := trayAnchorPoint()

	// WS_POPUP | WS_DISABLED — invisible, non-activating anchor
	const (
		wsPopup    = 0x80000000
		wsDisabled = 0x08000000
		wsExTool   = 0x00000080
		wsExNoAct  = 0x08000000
		swShowNA   = 8
	)
	className, _ := syscall.UTF16PtrFromString("STATIC")
	empty, _ := syscall.UTF16PtrFromString("")

	create := user32.NewProc("CreateWindowExW")
	hwnd, _, _ := create.Call(
		wsExTool|wsExNoAct,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(empty)),
		wsPopup|wsDisabled,
		uintptr(x),
		uintptr(y),
		1,
		1,
		0, 0, 0, 0,
	)
	if hwnd == 0 {
		return 0
	}
	// Ensure position is applied (some DPI setups ignore create coords).
	user32.NewProc("SetWindowPos").Call(
		hwnd, 0,
		uintptr(x), uintptr(y), 1, 1,
		0x0010|0x0040|0x0004, // SWP_NOACTIVATE|SWP_SHOWWINDOW|SWP_NOZORDER — keep simple
	)
	// Prefer no show flash: SWP_NOSIZE already; hide from taskbar via tool window.
	_ = swShowNA
	return hwnd
}

func destroyWindow(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	syscall.NewLazyDLL("user32.dll").NewProc("DestroyWindow").Call(hwnd)
}

type rect struct {
	Left, Top, Right, Bottom int32
}

// trayAnchorPoint returns a screen point just inside the work area,
// near the system notification area (tray).
func trayAnchorPoint() (x, y int) {
	// Default: bottom-right of primary work area (taskbar usually bottom).
	wa := workArea()
	x = int(wa.Right) - 24
	y = int(wa.Bottom) - 24

	// Prefer real tray notify rect when available (handles left/top/right taskbar).
	if nx, ny, ok := notifyAreaCenter(); ok {
		x, y = nx, ny
	}
	return x, y
}

func workArea() rect {
	var r rect
	const spiGetWorkArea = 0x0030
	syscall.NewLazyDLL("user32.dll").NewProc("SystemParametersInfoW").Call(
		spiGetWorkArea, 0, uintptr(unsafe.Pointer(&r)), 0,
	)
	if r.Right == 0 && r.Bottom == 0 {
		// Fallback primary screen
		r.Right = 1280
		r.Bottom = 720
	}
	return r
}

func notifyAreaCenter() (x, y int, ok bool) {
	user32 := syscall.NewLazyDLL("user32.dll")
	findWindow := user32.NewProc("FindWindowW")
	findWindowEx := user32.NewProc("FindWindowExW")
	getRect := user32.NewProc("GetWindowRect")

	trayClass, _ := syscall.UTF16PtrFromString("Shell_TrayWnd")
	tray, _, _ := findWindow.Call(uintptr(unsafe.Pointer(trayClass)), 0)
	if tray == 0 {
		return 0, 0, false
	}

	// TrayNotifyWnd → (optional) SysPager → ToolbarWindow32 chain varies by Windows version.
	// Prefer TrayNotifyWnd rect; fall back to whole tray.
	notifyClass, _ := syscall.UTF16PtrFromString("TrayNotifyWnd")
	notify, _, _ := findWindowEx.Call(tray, 0, uintptr(unsafe.Pointer(notifyClass)), 0)

	var rc rect
	target := notify
	if target == 0 {
		target = tray
	}
	r, _, _ := getRect.Call(target, uintptr(unsafe.Pointer(&rc)))
	if r == 0 || rc.Right <= rc.Left {
		return 0, 0, false
	}

	// Center of notify area; MessageBox will open centered on the 1×1 owner there.
	// Nudge slightly toward the work area so the dialog sits "above" the taskbar strip.
	cx := int((rc.Left + rc.Right) / 2)
	cy := int((rc.Top + rc.Bottom) / 2)

	wa := workArea()
	// If taskbar is bottom, pull anchor up into work area.
	if rc.Top >= wa.Bottom-8 {
		cy = int(wa.Bottom) - 8
	}
	// Taskbar top → pull down.
	if rc.Bottom <= wa.Top+8 {
		cy = int(wa.Top) + 8
	}
	// Taskbar right → pull left.
	if rc.Left >= wa.Right-8 {
		cx = int(wa.Right) - 8
	}
	// Taskbar left → pull right.
	if rc.Right <= wa.Left+8 {
		cx = int(wa.Left) + 8
	}
	return cx, cy, true
}
