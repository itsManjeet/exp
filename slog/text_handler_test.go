// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slog

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"testing"
	"time"
)

var testTime = time.Date(2000, 1, 2, 3, 4, 5, 0, time.UTC)

func TestTextHandler(t *testing.T) {
	for _, test := range []struct {
		name string
		attr Attr
		want string
	}{
		{
			"unquoted",
			Int("a", 1),
			"a=1",
		},
		{
			"quoted",
			String("x = y", `qu"o`),
			`"x = y"="qu\"o"`,
		},
		{
			"Sprint",
			Any("name", name{"Ren", "Hoek"}),
			`name="Hoek, Ren"`,
		},
		{
			"TextMarshaler",
			Any("t", text{"abc"}),
			`t="text{\"abc\"}"`,
		},
		{
			"TextMarshaler error",
			Any("t", text{""}),
			`t="!ERROR:text: empty string"`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var buf bytes.Buffer
			h := NewTextHandler(&buf)
			r := MakeRecord(testTime, InfoLevel, "a message", 0)
			r.AddAttr(test.attr)
			if err := h.Handle(r); err != nil {
				t.Fatal(err)
			}
			got := buf.String()
			// Remove final newline.
			got = got[:len(got)-1]
			want := `time=2000-01-02T03:04:05.000Z level=INFO msg="a message" ` + test.want
			if got != want {
				t.Errorf("\ngot  %s\nwant %s", got, want)
			}
		})
	}
}

// for testing fmt.Sprint
type name struct {
	First, Last string
}

func (n name) String() string { return n.Last + ", " + n.First }

// for testing TextMarshaler
type text struct {
	s string
}

func (t text) String() string { return t.s } // should be ignored

func (t text) MarshalText() ([]byte, error) {
	if t.s == "" {
		return nil, errors.New("text: empty string")
	}
	return []byte(fmt.Sprintf("text{%q}", t.s)), nil
}

func TestTextHandlerSource(t *testing.T) {
	var buf bytes.Buffer
	h := HandlerOptions{AddSource: true}.NewTextHandler(&buf)
	r := MakeRecord(testTime, InfoLevel, "m", 2)
	if err := h.Handle(r); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	wantRE := `source=([A-Z]:)?[^:]+text_handler_test\.go:\d+ msg`
	matched, err := regexp.MatchString(wantRE, got)
	if err != nil {
		t.Fatal(err)
	}
	if !matched {
		t.Errorf("got\n%q\nwanted to match %s", got, wantRE)
	}
}

func TestTextHandlerPreformatted(t *testing.T) {
	var buf bytes.Buffer
	var h Handler = NewTextHandler(&buf)
	h = h.With([]Attr{Duration("dur", time.Minute), Bool("b", true)})
	// Also test omitting time and level.
	r := MakeRecord(time.Time{}, 0, "m", 0)
	r.AddAttr(Int("a", 1))
	if err := h.Handle(r); err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSuffix(buf.String(), "\n")
	want := `msg=m dur=1m0s b=true a=1`
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestTextHandlerAlloc(t *testing.T) {
	r := MakeRecord(time.Now(), InfoLevel, "msg", 0)
	for i := 0; i < 10; i++ {
		r.AddAttr(Int("x = y", i))
	}
	h := NewTextHandler(io.Discard)
	wantAllocs(t, 0, func() { h.Handle(r) })
}

func TestNeedsQuoting(t *testing.T) {
	for _, test := range []struct {
		in   string
		want bool
	}{
		{"", false},
		{"ab", false},
		{"a=b", true},
		{`"ab"`, true},
		{"\a\b", true},
		{"a\tb", true},
		{"µåπ", false},
	} {
		got := needsQuoting(test.in)
		if got != test.want {
			t.Errorf("%q: got %t, want %t", test.in, got, test.want)
		}
	}
}
