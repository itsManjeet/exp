// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"testing"
	"time"
)

func TestOfAs(t *testing.T) {
	const i = 3
	v := OfInt(i)
	if got := v.AsInt(); got != i {
		t.Errorf("got %v, want %v", got, i)
	}
	v = OfUint(i)
	if got := v.AsUint(); got != i {
		t.Errorf("got %v, want %v", got, i)
	}
	v = OfFloat(i)
	if got := v.AsFloat(); got != i {
		t.Errorf("got %v, want %v", got, i)
	}
	v = OfBool(true)
	if got := v.AsBool(); got != true {
		t.Errorf("got %v, want %v", got, true)
	}
	const s = "foo"
	v = OfString(s)
	if got := v.AsString(); got != s {
		t.Errorf("got %v, want %v", got, s)
	}
	tm := time.Now()
	v = OfInterface(tm)
	if got := v.AsInterface(); got != tm {
		t.Errorf("got %v, want %v", got, tm)
	}
	var vnil Value
	if got := vnil.AsInterface(); got != nil {
		t.Errorf("got %v, want nil", got)
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

func TestPanics(t *testing.T) {
	for _, test := range []struct {
		name string
		f    func()
	}{
		{"int", func() { OfFloat(3).AsInt() }},
		{"uint", func() { OfInt(3).AsUint() }},
		{"float", func() { OfUint(3).AsFloat() }},
		{"bool", func() { OfInt(3).AsBool() }},
		{"string", func() { OfInterface("foo").AsString() }},
	} {
		if !panics(test.f) {
			t.Errorf("%s: got no panic, want panic", test.name)
		}
	}
}

func TestString(t *testing.T) {
	for _, test := range []struct {
		v    Value
		want string
	}{
		{OfInt(-3), "-3"},
		{OfUint(3), "3"},
		{OfFloat(.15), "0.15"},
		{OfBool(true), "true"},
		{OfString("foo"), "foo"},
		{OfInterface(time.Duration(3 * time.Second)), "3s"},
	} {
		if got := test.v.String(); got != test.want {
			t.Errorf("%#v: got %q, want %q", test.v, got, test.want)
		}
	}
}

func TestNoAlloc(t *testing.T) {
	// Assign values just to make sure the compiler doesn't optimize away the statements.
	var (
		i int64
		u uint64
		f float64
		b bool
		s string
		x interface{}
		p = &i
	)
	a := int(testing.AllocsPerRun(5, func() {
		i = OfInt(1).AsInt()
		u = OfUint(1).AsUint()
		f = OfFloat(1).AsFloat()
		b = OfBool(true).AsBool()
		s = OfString("foo").AsString()
		x = OfInterface(p).AsInterface()
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
