// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slog

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"golang.org/x/exp/slices"
	"golang.org/x/exp/slog/internal/buffer"
)

func TestRecordAttrs(t *testing.T) {
	as := []Attr{Int("k1", 1), String("k2", "foo"), Int("k3", 3),
		Int64("k4", -1), A("f", 3.1), Uint64("u", 999)}
	r := newRecordWithAttrs(as)
	if g, w := r.Attrs().Len(), len(as); g != w {
		t.Errorf("NumAttrs: got %d, want %d", g, w)
	}
	if got := r.Attrs().Append(nil); !attrsEqual(got, as) {
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
		r := MakeRecord(time.Time{}, 0, "", test.depth)
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

func TestAliasing(t *testing.T) {
	intAttrs := func(from, to int) []Attr {
		var as []Attr
		for i := from; i < to; i++ {
			as = append(as, Int("k", i))
		}
		return as
	}

	check := func(r *Record, want []Attr) {
		t.Helper()
		got := r.Attrs().Append(nil)
		if !attrsEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	}

	r1 := MakeRecord(time.Time{}, 0, "", 0)
	for i := 0; i < nAttrsInline+3; i++ {
		r1.AddAttrs(Int("k", i))
	}
	check(&r1, intAttrs(0, nAttrsInline+3))
	r2 := r1
	check(&r2, intAttrs(0, nAttrsInline+3))
	// if cap(r1.attrs2) <= len(r1.attrs2) {
	// 	t.Fatal("cap not greater than len")
	// }
	r1.AddAttrs(Int("k", nAttrsInline+3))
	r2.AddAttrs(Int("k", -1))
	check(&r1, intAttrs(0, nAttrsInline+4))
	check(&r2, append(intAttrs(0, nAttrsInline+3), Int("k", -1)))
}

func newRecordWithAttrs(as []Attr) Record {
	r := MakeRecord(time.Now(), InfoLevel, "", 0)
	r.AddAttrs(as...)
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
	r := MakeRecord(time.Now(), InfoLevel, "", 5)
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

func TestAttrIter(t *testing.T) {
	var want []Attr
	for i := 0; i < nAttrsInline*3+2; i++ {
		want = append(want, Int("k", i))
	}
	var al AttrList
	al.Add(want[:6]...)
	al.Add(want[6:12]...)
	al.Add(want[12:]...)
	var got []Attr
	it := al.Range()
	for {
		a, ok := it.Next()
		if !ok {
			break
		}
		got = append(got, a)
	}
	if !attrsEqual(got, want) {
		t.Errorf("\ngot  %v\nwant %v", got, want)
	}
}

func BenchmarkAttrIteration(b *testing.B) {
	attrs := make([]Attr, nAttrsInline)
	for i := 0; i < len(attrs); i++ {
		attrs[i] = Int("k", i)
	}

	var out Attr
	for _, nChunks := range []int{1, 2, 3} {
		var al AttrList
		for i := 0; i < nChunks; i++ {
			al.Add(attrs...)
		}
		if got, want := al.Len(), nAttrsInline*nChunks; got != want {
			b.Fatalf("got len %d, want %d", got, want)
		}
		b.Run(fmt.Sprintf("%d chunks", nChunks), func(b *testing.B) {
			b.Run("Each", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					al.each(func(a Attr) { out = a })
				}
			})
			b.Run("Iter", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					it := al.Range()
					for {
						a, ok := it.Next()
						if !ok {
							break
						}
						out = a
					}
				}
			})
		})
	}
	_ = out

}
