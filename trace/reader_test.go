// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package trace

import (
	"bytes"
	"io"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"

	"golang.org/x/exp/trace/internal/raw"
	"golang.org/x/tools/txtar"
)

func TestReader(t *testing.T) {
	matches, err := filepath.Glob("./testdata/*/*.test")
	if err != nil {
		t.Fatalf("failed to glob for tests: %v", err)
	}
	for _, testPath := range matches {
		testPath := testPath
		testName, err := filepath.Rel("./testdata", testPath)
		if err != nil {
			t.Fatalf("failed to relativize testdata path: %v", err)
		}
		t.Run(testName, func(t *testing.T) {
			trace, exp := parseTestFile(t, testPath)
			r, err := NewReader(trace)
			if err != nil {
				exp.check(t, err)
				return
			}
			var v validator
			for {
				ev, err := r.ReadEvent()
				if err == io.EOF {
					break
				}
				if err != nil {
					exp.check(t, err)
					return
				}
				v.event(t, ev)
			}
			exp.check(t, nil)
		})
	}
}

type validator struct {
	lastTs Time
}

func (v *validator) event(t *testing.T, ev Event) {
	// Validate timestamp order.
	if v.lastTs != 0 {
		if ev.Time() <= v.lastTs {
			t.Errorf("timestamp out-of-order for %+v", ev)
		} else {
			v.lastTs = ev.Time()
		}
	} else {
		v.lastTs = ev.Time()
	}
	if t.Failed() {
		t.FailNow()
	}
}

func parseTestFile(t *testing.T, testPath string) (io.Reader, expectation) {
	t.Helper()

	ar, err := txtar.ParseFile(testPath)
	if err != nil {
		t.Fatalf("failed to read test file for %s: %v", testPath, err)
	}
	if len(ar.Files) != 2 {
		t.Fatalf("malformed test %s: wrong number of files", testPath)
	}
	if ar.Files[0].Name != "expect" {
		t.Fatalf("malformed test %s: bad filename %s", testPath, ar.Files[0].Name)
	}
	if ar.Files[1].Name != "trace" {
		t.Fatalf("malformed test %s: bad filename %s", testPath, ar.Files[1].Name)
	}
	tr, err := raw.NewTextReader(bytes.NewReader(ar.Files[1].Data))
	if err != nil {
		t.Fatalf("malformed test %s: bad trace file: %v", testPath, err)
	}
	var buf bytes.Buffer
	tw, err := raw.NewWriter(&buf)
	if err != nil {
		t.Fatalf("failed to create trace byte writer: %v", err)
	}
	for {
		ev, err := tr.NextEvent()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("malformed test %s: bad trace file: %v", testPath, err)
		}
		if err := tw.WriteEvent(ev); err != nil {
			t.Fatalf("internal error during %s: failed to write trace bytes: %v", testPath, err)
		}
	}
	return &buf, parseExpectation(t, ar.Files[0].Data)
}

type expectation struct {
	failure bool
	matcher *regexp.Regexp
}

func (e expectation) check(t *testing.T, err error) {
	t.Helper()

	if !e.failure && err != nil {
		t.Fatalf("unexpected error while reading the trace: %v", err)
	}
	if e.failure && err == nil {
		t.Fatalf("expected error while reading the trace: want something matching %q, got none", e.matcher)
	}
	if e.failure && err != nil && !e.matcher.MatchString(err.Error()) {
		t.Fatalf("unexpected error while reading the trace: want something matching %q, got %s", e.matcher, err.Error())
	}
}

func parseExpectation(t *testing.T, data []byte) expectation {
	t.Helper()

	data = bytes.TrimSpace(data)
	if len(data) < 7 {
		t.Fatalf("malformed expectation file: %s", data)
	}
	var exp expectation
	switch result := string(data[:7]); result {
	case "SUCCESS":
	case "FAILURE":
		exp.failure = true
	default:
		t.Fatalf("malformed expectation file: %s", data)
	}
	if exp.failure {
		quoted := string(bytes.TrimSpace(data[7:]))
		pattern, err := strconv.Unquote(quoted)
		if err != nil {
			t.Fatalf("malformed pattern: not correctly quoted: %s: %v", quoted, err)
		}
		matcher, err := regexp.Compile(pattern)
		if err != nil {
			t.Fatalf("malformed pattern: not a valid regexp: %s: %v", pattern, err)
		}
		exp.matcher = matcher
	}
	return exp
}
