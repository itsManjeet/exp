// MACHINE GENERATED BY 'go generate' COMMAND; DO NOT EDIT

package windriver

import "unsafe"
import "syscall"

var _ unsafe.Pointer

var (
	moduser32 = syscall.NewLazyDLL("user32.dll")

	procGetMessageW      = moduser32.NewProc("GetMessageW")
	procTranslateMessage = moduser32.NewProc("TranslateMessage")
	procDispatchMessageW = moduser32.NewProc("DispatchMessageW")
	procDefWindowProcW   = moduser32.NewProc("DefWindowProcW")
	procRegisterClassW   = moduser32.NewProc("RegisterClassW")
	procCreateWindowExW  = moduser32.NewProc("CreateWindowExW")
	procDestroyWindow    = moduser32.NewProc("DestroyWindow")
	procSendMessageW     = moduser32.NewProc("SendMessageW")
	procLoadIconW        = moduser32.NewProc("LoadIconW")
	procLoadCursorW      = moduser32.NewProc("LoadCursorW")
	procShowWindow       = moduser32.NewProc("ShowWindow")
	procGetClientRect    = moduser32.NewProc("GetClientRect")
	procGetDC            = moduser32.NewProc("GetDC")
	procReleaseDC        = moduser32.NewProc("ReleaseDC")
)

func _GetMessage(msg *_MSG, hwnd syscall.Handle, msgfiltermin uint32, msgfiltermax uint32) (ret int32, err error) {
	r0, _, e1 := syscall.Syscall6(procGetMessageW.Addr(), 4, uintptr(unsafe.Pointer(msg)), uintptr(hwnd), uintptr(msgfiltermin), uintptr(msgfiltermax), 0, 0)
	ret = int32(r0)
	if ret == -1 {
		if e1 != 0 {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

func _TranslateMessage(msg *_MSG) (done bool) {
	r0, _, _ := syscall.Syscall(procTranslateMessage.Addr(), 1, uintptr(unsafe.Pointer(msg)), 0, 0)
	done = r0 != 0
	return
}

func _DispatchMessage(msg *_MSG) (ret int32) {
	r0, _, _ := syscall.Syscall(procDispatchMessageW.Addr(), 1, uintptr(unsafe.Pointer(msg)), 0, 0)
	ret = int32(r0)
	return
}

func _DefWindowProc(hwnd syscall.Handle, uMsg uint32, wParam uintptr, lParam uintptr) (lResult uintptr) {
	r0, _, _ := syscall.Syscall6(procDefWindowProcW.Addr(), 4, uintptr(hwnd), uintptr(uMsg), uintptr(wParam), uintptr(lParam), 0, 0)
	lResult = uintptr(r0)
	return
}

func _RegisterClass(wc *_WNDCLASS) (atom uint16, err error) {
	r0, _, e1 := syscall.Syscall(procRegisterClassW.Addr(), 1, uintptr(unsafe.Pointer(wc)), 0, 0)
	atom = uint16(r0)
	if atom == 0 {
		if e1 != 0 {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

func _CreateWindowEx(exstyle uint32, className *uint16, windowText *uint16, style uint32, x int32, y int32, width int32, height int32, parent syscall.Handle, menu syscall.Handle, hInstance syscall.Handle, lpParam uintptr) (hwnd syscall.Handle, err error) {
	r0, _, e1 := syscall.Syscall12(procCreateWindowExW.Addr(), 12, uintptr(exstyle), uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(windowText)), uintptr(style), uintptr(x), uintptr(y), uintptr(width), uintptr(height), uintptr(parent), uintptr(menu), uintptr(hInstance), uintptr(lpParam))
	hwnd = syscall.Handle(r0)
	if hwnd == 0 {
		if e1 != 0 {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

func _DestroyWindow(hwnd syscall.Handle) (err error) {
	r1, _, e1 := syscall.Syscall(procDestroyWindow.Addr(), 1, uintptr(hwnd), 0, 0)
	if r1 == 0 {
		if e1 != 0 {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

func _SendMessage(hwnd syscall.Handle, uMsg uint32, wParam uintptr, lParam uintptr) (lResult uintptr) {
	r0, _, _ := syscall.Syscall6(procSendMessageW.Addr(), 4, uintptr(hwnd), uintptr(uMsg), uintptr(wParam), uintptr(lParam), 0, 0)
	lResult = uintptr(r0)
	return
}

func _LoadIcon(hInstance syscall.Handle, iconName uintptr) (icon syscall.Handle, err error) {
	r0, _, e1 := syscall.Syscall(procLoadIconW.Addr(), 2, uintptr(hInstance), uintptr(iconName), 0)
	icon = syscall.Handle(r0)
	if icon == 0 {
		if e1 != 0 {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

func _LoadCursor(hInstance syscall.Handle, cursorName uintptr) (cursor syscall.Handle, err error) {
	r0, _, e1 := syscall.Syscall(procLoadCursorW.Addr(), 2, uintptr(hInstance), uintptr(cursorName), 0)
	cursor = syscall.Handle(r0)
	if cursor == 0 {
		if e1 != 0 {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

func _ShowWindow(hwnd syscall.Handle, cmdshow int32) (wasvisible bool) {
	r0, _, _ := syscall.Syscall(procShowWindow.Addr(), 2, uintptr(hwnd), uintptr(cmdshow), 0)
	wasvisible = r0 != 0
	return
}

func _GetClientRect(hwnd syscall.Handle, rect *_RECT) (err error) {
	r1, _, e1 := syscall.Syscall(procGetClientRect.Addr(), 2, uintptr(hwnd), uintptr(unsafe.Pointer(rect)), 0)
	if r1 == 0 {
		if e1 != 0 {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

func _GetDC(hwnd syscall.Handle) (dc syscall.Handle, err error) {
	r0, _, e1 := syscall.Syscall(procGetDC.Addr(), 1, uintptr(hwnd), 0, 0)
	dc = syscall.Handle(r0)
	if dc == 0 {
		if e1 != 0 {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

func _ReleaseDC(hwnd syscall.Handle, dc syscall.Handle) (err error) {
	r1, _, e1 := syscall.Syscall(procReleaseDC.Addr(), 2, uintptr(hwnd), uintptr(dc), 0)
	if r1 == 0 {
		if e1 != 0 {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}
