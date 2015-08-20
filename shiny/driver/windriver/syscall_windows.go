// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package windriver

import "syscall"

//go:generate go run $GOROOT/src/syscall/mksyscall_windows.go -output zsyscall_windows.go syscall_windows.go

type Point struct {
	X int32
	Y int32
}

type Msg struct {
	Hwnd    syscall.Handle
	Message uint32
	Wparam  uintptr
	Lparam  uintptr
	Time    uint32
	Pt      Point
}

//sys	GetMessage(msg *Msg, hwnd syscall.Handle, MsgFilterMin uint32, MsgFilterMax uint32) (ret int32, err error) [failretval==-1] = user32.GetMessageW
//sys	TranslateMessage(msg *Msg) (done bool) = user32.TranslateMessage
//sys	DispatchMessage(msg *Msg) (ret int32) = user32.DispatchMessageW
