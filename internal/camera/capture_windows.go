//go:build windows

package camera

import (
	"syscall"
	"unsafe"

	rl "github.com/gen2brain/raylib-go/raylib"
)

var (
	user32                  = syscall.NewLazyDLL("user32.dll")
	procClipCursor          = user32.NewProc("ClipCursor")
	procGetClientRect       = user32.NewProc("GetClientRect")
	procClientToScreen      = user32.NewProc("ClientToScreen")
	procGetActiveWindow     = user32.NewProc("GetActiveWindow")
	procGetForegroundWindow = user32.NewProc("GetForegroundWindow")
	procShowCursor          = user32.NewProc("ShowCursor")

	isPlatformCursorHidden bool
)

type winRect struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

type winPoint struct {
	X int32
	Y int32
}

// getGameWindowHWND retrieves the active Win32 HWND for the game window.
func getGameWindowHWND() uintptr {
	hwnd, _, _ := procGetActiveWindow.Call()
	if hwnd != 0 {
		return hwnd
	}
	hwnd, _, _ = procGetForegroundWindow.Call()
	return hwnd
}

// trapMouseToWindow physically confines the mouse pointer inside the game window's client area,
// preventing it from escaping to other monitors or outside the window borders.
func trapMouseToWindow() {
	if !rl.IsWindowReady() {
		return
	}

	hwnd := getGameWindowHWND()
	if hwnd != 0 {
		var rc winRect
		r1, _, _ := procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
		if r1 != 0 {
			ptTopLeft := winPoint{X: rc.Left, Y: rc.Top}
			ptBottomRight := winPoint{X: rc.Right, Y: rc.Bottom}

			procClientToScreen.Call(hwnd, uintptr(unsafe.Pointer(&ptTopLeft)))
			procClientToScreen.Call(hwnd, uintptr(unsafe.Pointer(&ptBottomRight)))

			screenRect := winRect{
				Left:   ptTopLeft.X,
				Top:    ptTopLeft.Y,
				Right:  ptBottomRight.X,
				Bottom: ptBottomRight.Y,
			}
			procClipCursor.Call(uintptr(unsafe.Pointer(&screenRect)))
			return
		}
	}

	// Fallback using Raylib monitor window telemetry
	winPos := rl.GetWindowPosition()
	w := float32(rl.GetScreenWidth())
	h := float32(rl.GetScreenHeight())
	screenRect := winRect{
		Left:   int32(winPos.X),
		Top:    int32(winPos.Y),
		Right:  int32(winPos.X + w),
		Bottom: int32(winPos.Y + h),
	}
	procClipCursor.Call(uintptr(unsafe.Pointer(&screenRect)))
}

// releaseMouseFromWindow removes any cursor confinement, allowing free mouse movement.
func releaseMouseFromWindow() {
	procClipCursor.Call(0)
}

// hidePlatformCursor ensures the OS mouse cursor is hidden.
func hidePlatformCursor() {
	if isPlatformCursorHidden {
		return
	}
	for i := 0; i < 16; i++ {
		r, _, _ := procShowCursor.Call(0)
		if int32(r) < 0 {
			break
		}
	}
	isPlatformCursorHidden = true
}

// showPlatformCursor ensures the OS mouse cursor is restored and visible.
func showPlatformCursor() {
	if !isPlatformCursorHidden {
		return
	}
	for i := 0; i < 16; i++ {
		r, _, _ := procShowCursor.Call(1)
		if int32(r) >= 0 {
			break
		}
	}
	isPlatformCursorHidden = false
}
