// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package trace

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"

	"golang.org/x/exp/slices"
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
			v := newValidator()
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
	gs     map[GoID]GoState
	ps     map[ProcID]ProcState
	ranges map[resource][]string
}

func newValidator() *validator {
	return &validator{
		gs:     make(map[GoID]GoState),
		ps:     make(map[ProcID]ProcState),
		ranges: make(map[resource][]string),
	}
}

func (v *validator) hasRange(r resource, name string) bool {
	ranges, ok := v.ranges[r]
	return ok && slices.Contains(ranges, name)
}

func (v *validator) addRange(r resource, name string) {
	ranges, _ := v.ranges[r]
	ranges = append(ranges, name)
	v.ranges[r] = ranges
}

func (v *validator) hasAnyRange(r resource) bool {
	ranges, ok := v.ranges[r]
	return ok && len(ranges) != 0
}

func (v *validator) deleteRange(r resource, name string) {
	ranges, ok := v.ranges[r]
	if !ok {
		return
	}
	i := slices.Index(ranges, name)
	if i < 0 {
		return
	}
	v.ranges[r] = slices.Delete(ranges, i, i+1)
}

func (v *validator) event(t *testing.T, ev Event) {
	defer func() {
		if t.Failed() {
			t.FailNow()
		}
	}()

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

	switch ev.Kind() {
	case EventStateTransition:
		// Validate state transitions.
		tr := ev.StateTransition()
		switch tr.Resource {
		case ResourceGoroutine:
			id, old, new := tr.Goroutine()
			if new == GoUndetermined {
				t.Errorf("transition to undetermined state for goroutine %d", id)
			}
			if new == GoNotExist && v.hasAnyRange(goroutine(id)) {
				t.Errorf("goroutine %d died with active ranges", id)
			}
			if state, ok := v.gs[id]; ok {
				if old != state {
					t.Errorf("bad old state for goroutine %d: got %s, want %s", id, old, state)
				}
			} else {
				if old != GoUndetermined && old != GoNotExist {
					t.Errorf("bad old state for unregistered goroutine %d: %s", id, old)
				}
			}
			v.gs[id] = new
		case ResourceProc:
			id, old, new := tr.Proc()
			if new == ProcUndetermined {
				t.Errorf("transition to undetermined state for proc %d", id)
			}
			if new == ProcNotExist && v.hasAnyRange(proc(id)) {
				t.Errorf("proc %d died with active ranges", id)
			}
			if state, ok := v.ps[id]; ok {
				if old != state {
					t.Errorf("bad old state for proc %d: got %s, want %s", id, old, state)
				}
			} else {
				if old != ProcUndetermined && old != ProcNotExist {
					t.Errorf("bad old state for unregistered proc %d: %s", id, old)
				}
			}
			v.ps[id] = new
		}
	case EventRangeBegin, EventRangeActive, EventRangeEnd:
		// Validate ranges.
		r := ev.Range()
		rs := resourceOf(ev, r.Scope)
		switch ev.Kind() {
		case EventRangeBegin:
			if v.hasRange(rs, r.Name) {
				t.Errorf("already active range %q on %v begun again", r.Name, rs)
			}
			v.addRange(rs, r.Name)
		case EventRangeActive:
			if v.hasRange(rs, r.Name) {
				t.Errorf("already active range %q on %v marked active again", r.Name, rs)
			}
			v.addRange(rs, r.Name)
		case EventRangeEnd:
			if !v.hasRange(rs, r.Name) {
				t.Errorf("inactive range %q on %v ended", r.Name, rs)
			}
			v.deleteRange(rs, r.Name)
		}
	}
}

type resource struct {
	kind ResourceKind
	id   int64
}

func goroutine(id GoID) resource {
	return resource{kind: ResourceGoroutine, id: int64(id)}
}

func proc(id ProcID) resource {
	return resource{kind: ResourceProc, id: int64(id)}
}

func resourceOf(ev Event, kind ResourceKind) resource {
	rs := resource{kind: kind}
	switch kind {
	case ResourceGoroutine:
		rs.id = int64(ev.Goroutine())
	case ResourceProc:
		rs.id = int64(ev.Proc())
	case ResourceThread:
		rs.id = int64(ev.Thread())
	}
	return rs
}

func (r resource) String() string {
	switch r.kind {
	case ResourceNone:
		return "(global)"
	case ResourceGoroutine:
		return fmt.Sprintf("goroutine %d", r.id)
	case ResourceProc:
		return fmt.Sprintf("proc %d", r.id)
	case ResourceThread:
		return fmt.Sprintf("thread %d", r.id)
	}
	panic("unknown resource")
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
