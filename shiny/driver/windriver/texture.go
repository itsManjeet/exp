// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build windows

package windriver

import (
	"errors"
	"image"
	"image/color"
	"image/draw"
	"runtime"
	"sync"
	"syscall"

	"golang.org/x/exp/shiny/driver/internal/win32"
	"golang.org/x/exp/shiny/screen"
)

type textureImpl struct {
	size   image.Point
	dc     win32.HDC
	bitmap syscall.Handle

	mu       sync.Mutex
	released bool
}

func newTexture(size image.Point) (screen.Texture, error) {
	// run this function on single OS thread, because
	// DC returned by GetDC(nil) has to be released on the same thread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	screenDC, err := win32.GetDC(0)
	if err != nil {
		return nil, err
	}
	defer win32.ReleaseDC(0, screenDC)

	dc, err := _CreateCompatibleDC(screenDC)
	if err != nil {
		return nil, err
	}
	bitmap, err := _CreateCompatibleBitmap(screenDC, int32(size.X), int32(size.Y))
	if err != nil {
		_DeleteDC(dc)
		return nil, err
	}
	return &textureImpl{
		size:   size,
		dc:     dc,
		bitmap: bitmap,
	}, nil
}

func (t *textureImpl) Bounds() image.Rectangle {
	return image.Rectangle{Max: t.size}
}

func (t *textureImpl) Fill(r image.Rectangle, c color.Color, op draw.Op) {
	// TODO
}

func (t *textureImpl) Release() {
	err := t.release()
	if err != nil {
		panic(err) // TODO handle error
	}
}

func (t *textureImpl) release() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.released {
		return nil
	}
	t.released = true

	err := _DeleteObject(t.bitmap)
	if err != nil {
		return err
	}
	return _DeleteDC(t.dc)
}

func (t *textureImpl) Size() image.Point {
	return t.size
}

func (t *textureImpl) Upload(dp image.Point, src screen.Buffer, sr image.Rectangle) {
	err := t.upload(dp, src, sr)
	if err != nil {
		panic(err) // TODO handle error
	}
}

func (t *textureImpl) upload(dp image.Point, src screen.Buffer, sr image.Rectangle) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.released {
		return errors.New("windriver: Texture.Upload called after Texture.Release")
	}

	// Select t.bitmap into t.dc, so our drawing gets recorded
	// into t.bitmap and not into 1x1 default bitmap created
	// during CreateCompatibleDC call.
	prev, err := _SelectObject(t.dc, t.bitmap)
	if err != nil {
		return err
	}
	defer func() {
		_, err2 := _SelectObject(t.dc, prev)
		if err == nil {
			err = err2
		}
	}()

	return src.(*bufferImpl).blitToDC(t.dc, dp, sr)
}
