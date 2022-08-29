// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slog

import (
	"strings"
	"testing"
	"time"

	"golang.org/x/exp/slices"
	"golang.org/x/exp/slog/internal/buffer"
)

func TestRecordAttrs(t *testing.T) {
	as := []Attr{Int("k1", 1), String("k2", "foo"), Int("k3", 3),
		Int64("k4", -1), Float64("f", 3.1), Uint64("u", 999)}
	r := newRecordWithAttrs(as)
	if g, w := r.NumAttrs(), len(as); g != w {
		t.Errorf("NumAttrs: got %d, want %d", g, w)
	}
	if got := r.Attrs(); !attrsEqual(got, as) {
		t.Errorf("got %v, want %v", got, as)
	}
}

func TestRecordSourceLine(t *testing.T) {
	// Zero call depth => empty file/line
	for _, test := range []struct {
		depth            int
		wantFile         string
		wantLinePositive bool
	}{
		{0, "", false},
		{-16, "", false},
		{1, "record.go", true},
	} {
		r := NewRecord(time.Time{}, 0, "", test.depth)
		gotFile, gotLine := r.SourceLine()
		if i := strings.LastIndexByte(gotFile, '/'); i >= 0 {
			gotFile = gotFile[i+1:]
		}
		if gotFile != test.wantFile || (gotLine > 0) != test.wantLinePositive {
			t.Errorf("depth %d: got (%q, %d), want (%q, %t)",
				test.depth, gotFile, gotLine, test.wantFile, test.wantLinePositive)
		}
	}
}

func newRecordWithAttrs(as []Attr) Record {
	r := NewRecord(time.Now(), InfoLevel, "", 0)
	for _, a := range as {
		r.AddAttr(a)
	}
	return r
}

func attrsEqual(as1, as2 []Attr) bool {
	return slices.EqualFunc(as1, as2, Attr.Equal)
}

// Currently, pc(2) takes over 400ns, which is too expensive
// to call it for every log message.
func BenchmarkPC(b *testing.B) {
	b.ReportAllocs()
	var x uintptr
	for i := 0; i < b.N; i++ {
		x = pc(3)
	}
	_ = x
}

func BenchmarkSourceLine(b *testing.B) {
	r := NewRecord(time.Now(), InfoLevel, "", 5)
	b.Run("alone", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			file, line := r.SourceLine()
			_ = file
			_ = line
		}
	})
	b.Run("stringifying", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			file, line := r.SourceLine()
			buf := buffer.New()
			buf.WriteString(file)
			buf.WriteByte(':')
			itoa((*[]byte)(buf), line, -1)
			s := string(*buf)
			buf.Free()
			_ = s
		}
	})
}
