// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package x11driver

import (
	"image"
	"sync"

	"github.com/BurntSushi/xgb/render"
	"github.com/BurntSushi/xgb/xproto"

	"golang.org/x/exp/shiny/screen"
)

const textureDepth = 32

type textureImpl struct {
	s *screenImpl

	size image.Point
	xm   xproto.Pixmap
	xp   render.Picture

	mu       sync.Mutex
	released bool
}

func (t *textureImpl) Size() image.Point { return t.size }

func (t *textureImpl) Release() {
	t.mu.Lock()
	released := t.released
	t.released = true
	t.mu.Unlock()

	if released {
		return
	}
	render.FreePicture(t.s.xc, t.xp)
	xproto.FreePixmap(t.s.xc, t.xm)
}

func (t *textureImpl) Upload(dp image.Point, src screen.Buffer, sr image.Rectangle, sender screen.Sender) {
	upload(t.s, t, xproto.Drawable(t.xm), t.s.gcontext32, textureDepth, dp, src.(*bufferImpl), sr, sender)
}
