// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slog

import (
	"fmt"
	"testing"
	"time"
	"unsafe"
)

func TestEqual(t *testing.T) {
	var x, y int
	vals := []Attr{
		{},
		Int64("key", 1),
		Int64("key", 2),
		A("key", 3.5),
		A("key", 3.7),
		A("key", true),
		A("key", false),
		A("key", &x),
		A("key", &y),
	}
	for i, v1 := range vals {
		for j, v2 := range vals {
			got := v1.Equal(v2)
			want := i == j
			if got != want {
				t.Errorf("%v.Equal(%v): got %t, want %t", v1, v2, got, want)
			}
		}
	}
}

func TestNilAttr(t *testing.T) {
	n := A[any]("k", nil)
	if g := n.Value(); g != nil {
		t.Errorf("got %#v, want nil", g)
	}
}

func panics(f func()) (b bool) {
	defer func() {
		if x := recover(); x != nil {
			b = true
		}
	}()
	f()
	return false
}

func TestString(t *testing.T) {
	for _, test := range []struct {
		v    Attr
		want string
	}{
		{Int64("key", -3), "-3"},
		{A("key", .15), "0.15"},
		{A("key", true), "true"},
		{String("key", "foo"), "foo"},
		{A("key", time.Duration(3*time.Second)), "3s"},
	} {
		if got := test.v.String(); got != test.want {
			t.Errorf("%#v: got %q, want %q", test.v, got, test.want)
		}
	}
}

func TestAttrNoAlloc(t *testing.T) {
	// Assign values just to make sure the compiler doesn't optimize away the statements.
	var (
		i int64
		u uint64
		f float64
		b bool
		s string
		x any
		p = &i
		d time.Duration
	)
	a := int(testing.AllocsPerRun(1, func() {
		i = Int64("key", 1).Int64()
		u = Uint64("key", 1).Uint64()
		f = A("key", 1.0).Float64()
		b = A("key", true).Bool()
		s = String("key", "foo").String()
		d = Duration("key", d).Duration()
		x = A("key", p).Value()
	}))
	if a != 0 {
		t.Errorf("got %d allocs, want zero", a)
	}
	_ = u
	_ = f
	_ = b
	_ = s
	_ = x
}

func TestAnyLevelAlloc(t *testing.T) {
	// Because typical Levels are small integers,
	// they are zero-alloc.
	var a Attr
	x := DebugLevel + 100
	wantAllocs(t, 0, func() { a = A("k", x) })
	_ = a
}

func TestAnyLevel(t *testing.T) {
	x := DebugLevel + 100
	a := A("k", x)
	v := a.Value()
	if _, ok := v.(Level); !ok {
		t.Errorf("wanted Level, got %T", v)
	}
}

func check[T any](t *testing.T, val T, wantKind Kind, wantVal any) {
	t.Helper()
	got := A("k", val)
	if g, w := got.Kind(), wantKind; g != w {
		t.Errorf("got %s, want %s", g, w)
	}
	if g, w := got.Value(), wantVal; g != w {
		t.Errorf("got %v (%[1]T), want %v (%[2]T)", g, w)
	}
}

func TestA(t *testing.T) {
	check(t, "hello", StringKind, "hello")
	check(t, 34, Int64Kind, int64(34))
	check(t, any("x"), StringKind, "x")
	check(t, any(34), Int64Kind, int64(34))
	check(t, any(nil), AnyKind, nil)
	check(t, 8*time.Hour, DurationKind, 8*time.Hour)
	if A[any]("k", nil).Value() != nil {
		t.Error("wanted nil")
	}
}

// Compare A[T](T) with Any(any).
func BenchmarkGenericAny(b *testing.B) {
	var a Attr
	w := "world"
	str := "hello" + ", " + w
	f := 3.14159
	d := 99 * time.Hour
	var an any = "hello"
	tm := time.Date(2022, 9, 10, 0, 0, 0, 0, time.UTC)
	b.Run("String", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			a = String("key", str)
		}
	})
	b.Run("any-string", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			a = Any("key", str)
		}
	})
	b.Run("generic-string", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			a = A("key", str)
		}
	})
	b.Run("any-float", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			a = Any("key", f)
		}
	})
	b.Run("generic-float", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			a = A("key", f)
		}
	})
	b.Run("Duration", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			a = Duration("key", d)
		}
	})
	b.Run("any-duration", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			a = Any("key", d)
		}
	})
	b.Run("generic-duration", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			a = A("key", d)
		}
	})
	b.Run("Time", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			a = Time("key", tm)
		}
	})
	b.Run("any-time", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			a = Any("key", tm)
		}
	})
	b.Run("generic-time", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			a = A("key", tm)
		}
	})
	b.Run("any-any", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			a = Any("key", an)
		}
	})
	b.Run("generic-any", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			a = A("key", an)
		}
	})
	_ = a
}

// A non-generic implementation of Any.
func Any(key string, val any) Attr {
	a := Attr{key: key}
	a.setAnyValue(val)
	return a
}

//////////////// Benchmark for accessing Attr values

// The "As" form is the slowest.
// The switch-panic and visitor times are almost the same.
// BenchmarkDispatch/switch-checked-8         	 8669427	       137.7 ns/op
// BenchmarkDispatch/As-8                     	 8212087	       145.3 ns/op
// BenchmarkDispatch/Visit-8                  	 8926146	       135.3 ns/op
func BenchmarkDispatch(b *testing.B) {
	kvs := []Attr{
		Int64("i", 32768),
		Uint64("u", 0xfacecafe),
		String("s", "anything"),
		A("b", true),
		A("f", 1.2345),
		Duration("d", time.Second),
		A("a", b),
	}
	var (
		ii int64
		s  string
		bb bool
		u  uint64
		d  time.Duration
		f  float64
		a  any
	)
	b.Run("switch-checked", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for _, kv := range kvs {
				switch kv.Kind() {
				case StringKind:
					s = kv.String()
				case Int64Kind:
					ii = kv.Int64()
				case Uint64Kind:
					u = kv.Uint64()
				case Float64Kind:
					f = kv.Float64()
				case BoolKind:
					bb = kv.Bool()
				case DurationKind:
					d = kv.Duration()
				case AnyKind:
					a = kv.Value()
				default:
					panic("bad kind")
				}
			}
		}
		_ = ii
		_ = s
		_ = bb
		_ = u
		_ = d
		_ = f
		_ = a

	})
	b.Run("As", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for _, kv := range kvs {
				if v, ok := kv.AsString(); ok {
					s = v
				} else if v, ok := kv.AsInt64(); ok {
					ii = v
				} else if v, ok := kv.AsUint64(); ok {
					u = v
				} else if v, ok := kv.AsFloat64(); ok {
					f = v
				} else if v, ok := kv.AsBool(); ok {
					bb = v
				} else if v, ok := kv.AsDuration(); ok {
					d = v
				} else if v, ok := kv.AsAny(); ok {
					a = v
				} else {
					panic("bad kind")
				}
			}
		}
		_ = ii
		_ = s
		_ = bb
		_ = u
		_ = d
		_ = f
		_ = a
	})

	b.Run("Visit", func(b *testing.B) {
		v := &setVisitor{}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for _, kv := range kvs {
				kv.Visit(v)
			}
		}
	})
}

type setVisitor struct {
	i int64
	s string
	b bool
	u uint64
	d time.Duration
	f float64
	a any
}

func (v *setVisitor) String(s string)          { v.s = s }
func (v *setVisitor) Int64(i int64)            { v.i = i }
func (v *setVisitor) Uint64(x uint64)          { v.u = x }
func (v *setVisitor) Float64(x float64)        { v.f = x }
func (v *setVisitor) Bool(x bool)              { v.b = x }
func (v *setVisitor) Duration(x time.Duration) { v.d = x }
func (v *setVisitor) Any(x any)                { v.a = x }

// When dispatching on all types, the "As" functions are slightly slower
// than switching on the kind and then calling a function that checks
// the kind again. See BenchmarkDispatch above.

func (a Attr) AsString() (string, bool) {
	if a.Kind() == StringKind {
		return a.str(), true
	}
	return "", false
}

func (a Attr) AsInt64() (int64, bool) {
	if a.Kind() == Int64Kind {
		return int64(a.num), true
	}
	return 0, false
}

func (a Attr) AsUint64() (uint64, bool) {
	if a.Kind() == Uint64Kind {
		return a.num, true
	}
	return 0, false
}

func (a Attr) AsFloat64() (float64, bool) {
	if a.Kind() == Float64Kind {
		return a.float(), true
	}
	return 0, false
}

func (a Attr) AsBool() (bool, bool) {
	if a.Kind() == BoolKind {
		return a.bool(), true
	}
	return false, false
}

func (a Attr) AsDuration() (time.Duration, bool) {
	if a.Kind() == DurationKind {
		return a.duration(), true
	}
	return 0, false
}

func (a Attr) AsAny() (any, bool) {
	if a.Kind() == AnyKind {
		return a.any, true
	}
	return nil, false
}

// Problem: adding a type means adding a method, which is a breaking change.
// Using an unexported method to force embedding will make programs compile,
// But they will panic at runtime when we call the new method.
type Visitor interface {
	String(string)
	Int64(int64)
	Uint64(uint64)
	Float64(float64)
	Bool(bool)
	Duration(time.Duration)
	Any(any)
}

func (a Attr) Visit(v Visitor) {
	switch a.Kind() {
	case StringKind:
		v.String(a.str())
	case Int64Kind:
		v.Int64(int64(a.num))
	case Uint64Kind:
		v.Uint64(a.num)
	case BoolKind:
		v.Bool(a.bool())
	case Float64Kind:
		v.Float64(a.float())
	case DurationKind:
		v.Duration(a.duration())
	case AnyKind:
		v.Any(a.any)
	default:
		panic("bad kind")
	}
}

// An Attr with "unsafe" strings is significantly faster:
// safe:  1785 ns/op, 0 allocs
// unsafe: 690 ns/op, 0 allocs

// Run this with and without -tags unsafe_kvs to compare.
func BenchmarkUnsafeStrings(b *testing.B) {
	b.ReportAllocs()
	dst := make([]Attr, 100)
	src := make([]Attr, len(dst))
	b.Logf("Attr size = %d", unsafe.Sizeof(Attr{}))
	for i := range src {
		src[i] = String("k", fmt.Sprintf("string#%d", i))
	}
	b.ResetTimer()
	var d string
	for i := 0; i < b.N; i++ {
		copy(dst, src)
		for _, a := range dst {
			d = a.String()
		}
	}
	_ = d
}
