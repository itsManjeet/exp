// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slog

import (
	"context"
	"testing"
)

func TestContext(t *testing.T) {
	// If there is no Logger in the context, FromContext returns the default
	// Logger.
	ctx := context.Background()
	gotl := FromContext(ctx)
	if _, ok := gotl.Handler().(*defaultHandler); !ok {
		t.Error("did not get default Logger")
	}

	// If there is a Logger in the context, FromContext returns it, with the ctx
	// arg.
	h := &captureHandler{}
	ctx = context.WithValue(ctx, "ID", 1)
	ctx = NewContext(ctx, New(h))
	gotl = FromContext(ctx)
	if gotl.Handler() != h {
		t.Fatal("did not get the right logger")
	}
	gotl.Info("")
	if g, w := h.r.Context().Value("ID"), 1; g != w {
		t.Errorf("got ID %v, want %v", g, w)
	}
}
