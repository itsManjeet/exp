// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TODO: verify that the output of Marshal{Text,JSON} is suitably escaped.

package slog

import (
	"testing"
)

func TestWith(t *testing.T) {
	d := &defaultHandler{}
	if g := len(d.attrs); g != 0 {
		t.Errorf("got %d, want 0", g)
	}
	a1 := []Attr{Int("a", 1)}
	d2 := d.With(a1)
	if g := d2.(*defaultHandler).attrs; !attrsEqual(g, a1) {
		t.Errorf("got %v, want %v", g, a1)
	}
	d3 := d2.With([]Attr{String("b", "two")})
	want := append(a1, String("b", "two"))
	if g := d3.(*defaultHandler).attrs; !attrsEqual(g, want) {
		t.Errorf("got %v, want %v", g, want)
	}
}
