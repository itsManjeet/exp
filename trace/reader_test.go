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
	"strings"
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
				//t.Log(ev.String())
				v.event(t, ev)
			}
			exp.check(t, nil)
		})
	}
}

type validator struct {
	lastTs   Time
	gs       map[GoID]*goState
	ps       map[ProcID]*procState
	ms       map[ThreadID]*schedContext
	ranges   map[ResourceID][]string
	seenSync bool
}

type schedContext struct {
	M ThreadID
	P ProcID
	G GoID
}

type goState struct {
	state   GoState
	binding *schedContext
}

type procState struct {
	state   ProcState
	binding *schedContext
}

func newValidator() *validator {
	return &validator{
		gs:     make(map[GoID]*goState),
		ps:     make(map[ProcID]*procState),
		ms:     make(map[ThreadID]*schedContext),
		ranges: make(map[ResourceID][]string),
	}
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

	// Validate event stack.
	checkStack(t, ev.Stack())

	switch ev.Kind() {
	case EventSync:
		// Just record that we've seen a Sync at some point.
		v.seenSync = true
	case EventMetric:
		m := ev.Metric()
		if !strings.Contains(m.Name, ":") {
			// Should have a ":" as per runtime/metrics convention.
			t.Errorf("invalid metric name %q", m.Name)
		}
		// Make sure the value is OK.
		if m.Value.Kind() == ValueBad {
			t.Error("invalid value")
		}
		switch m.Value.Kind() {
		case ValueUint64:
			// Just make sure it doesn't panic.
			_ = m.Value.Uint64()
		}
	case EventLabel:
		l := ev.Label()

		// Check label.
		if l.Label == "" {
			t.Errorf("invalid label %q", l.Label)
		}

		// Check label resource.
		if l.Resource.Kind == ResourceNone {
			t.Error("label resource none")
		}
		switch l.Resource.Kind {
		case ResourceGoroutine:
			id := l.Resource.Goroutine()
			if _, ok := v.gs[id]; !ok {
				t.Errorf("label for invalid goroutine %d", id)
			}
		case ResourceProc:
			id := l.Resource.Proc()
			if _, ok := v.ps[id]; !ok {
				t.Errorf("label for invalid proc %d", id)
			}
		case ResourceThread:
			id := l.Resource.Thread()
			if _, ok := v.ms[id]; !ok {
				t.Errorf("label for invalid thread %d", id)
			}
		}
	case EventStackSample:
		// Nothing extra to check. It's basically a sched context and a stack.
		// The sched context is also not guaranteed to align.
	case EventStateTransition:
		// Validate state transitions.
		//
		// TODO(mknyszek): A lot of logic is duplicated between goroutines and procs.
		// The two are intentionally handled identically; from the perspective of the
		// API, resources all have the same general properties. Consider making this
		// code generic over resources and implementing validation just once.
		tr := ev.StateTransition()
		checkStack(t, tr.Stack)
		switch tr.Resource.Kind {
		case ResourceGoroutine:
			// Basic state transition validation.
			id := tr.Resource.Goroutine()
			old, new := tr.Goroutine()
			if new == GoUndetermined {
				t.Errorf("transition to undetermined state for goroutine %d", id)
			}
			if v.seenSync && old == GoUndetermined {
				t.Errorf("undetermined goroutine %d after first global sync", id)
			}
			if new == GoNotExist && v.hasAnyRange(goroutine(id)) {
				t.Errorf("goroutine %d died with active ranges", id)
			}
			state, ok := v.gs[id]
			if ok {
				if old != state.state {
					t.Errorf("bad old state for goroutine %d: got %s, want %s", id, old, state.state)
				}
				state.state = new
			} else {
				if old != GoUndetermined && old != GoNotExist {
					t.Errorf("bad old state for unregistered goroutine %d: %s", id, old)
				}
				state = &goState{state: new}
				v.gs[id] = state
			}
			// Validate sched context.
			if new.Executing() {
				ctx := v.getOrCreateThread(t, ev.Thread())
				if ctx.G != NoGoroutine && ctx.G != id {
					t.Errorf("tried to run goroutine %d when one was already executing (%d) on thread %d", id, ctx.G, ev.Thread())
				}
				ctx.G = id
				state.binding = ctx
			} else if old.Executing() && !new.Executing() {
				ctx := state.binding
				if ctx != nil {
					if ctx.G != id {
						t.Errorf("tried to stop goroutine %d when it wasn't currently executing (currently executing %d) on thread %d", id, ctx.G, ev.Thread())
					}
					ctx.G = NoGoroutine
					state.binding = nil
				} else {
					t.Errorf("stopping goroutine %d not bound to any active context", id)
				}
			}
		case ResourceProc:
			// Basic state transition validation.
			id := tr.Resource.Proc()
			old, new := tr.Proc()
			if new == ProcUndetermined {
				t.Errorf("transition to undetermined state for proc %d", id)
			}
			if v.seenSync && old == ProcUndetermined {
				t.Errorf("undetermined proc %d after first global sync", id)
			}
			if new == ProcNotExist && v.hasAnyRange(proc(id)) {
				t.Errorf("proc %d died with active ranges", id)
			}
			state, ok := v.ps[id]
			if ok {
				if old != state.state {
					t.Errorf("bad old state for proc %d: got %s, want %s", id, old, state.state)
				}
				state.state = new
			} else {
				if old != ProcUndetermined && old != ProcNotExist {
					t.Errorf("bad old state for unregistered proc %d: %s", id, old)
				}
				state = &procState{state: new}
				v.ps[id] = state
			}
			// Validate sched context.
			if new.Executing() {
				ctx := v.getOrCreateThread(t, ev.Thread())
				if ctx.P != NoProc && ctx.P != id {
					t.Errorf("tried to run proc %d when one was already executing (%d) on thread %d", id, ctx.P, ev.Thread())
				}
				ctx.P = id
				state.binding = ctx
			} else if old.Executing() && !new.Executing() {
				ctx := state.binding
				if ctx != nil {
					if ctx.P != id {
						t.Errorf("tried to stop proc %d when it wasn't currently executing (currently executing %d) on thread %d", id, ctx.P, ev.Thread())
					}
					ctx.P = NoProc
					state.binding = nil
				} else {
					t.Errorf("stopping proc %d not bound to any active context", id)
				}
			}
		}
	case EventRangeBegin, EventRangeActive, EventRangeEnd:
		// Validate ranges.
		r := ev.Range()
		switch ev.Kind() {
		case EventRangeBegin:
			if v.hasRange(r.Scope, r.Name) {
				t.Errorf("already active range %q on %v begun again", r.Name, r.Scope)
			}
			v.addRange(r.Scope, r.Name)
		case EventRangeActive:
			if !v.hasRange(r.Scope, r.Name) {
				v.addRange(r.Scope, r.Name)
			}
		case EventRangeEnd:
			if !v.hasRange(r.Scope, r.Name) {
				t.Errorf("inactive range %q on %v ended", r.Name, r.Scope)
			}
			v.deleteRange(r.Scope, r.Name)
		}
	case EventLog:
		// There's really not much here to check, except that we can
		// generate a Log. The category and message are entirely user-created,
		// so we can't make any assumptions as to what they are. We also
		// can't validate the task, because proving the task's existence is very
		// much best-effort.
		_ = ev.Log()
	}
}

func (v *validator) hasRange(r ResourceID, name string) bool {
	ranges, ok := v.ranges[r]
	return ok && slices.Contains(ranges, name)
}

func (v *validator) addRange(r ResourceID, name string) {
	ranges, _ := v.ranges[r]
	ranges = append(ranges, name)
	v.ranges[r] = ranges
}

func (v *validator) hasAnyRange(r ResourceID) bool {
	ranges, ok := v.ranges[r]
	return ok && len(ranges) != 0
}

func (v *validator) deleteRange(r ResourceID, name string) {
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

func (v *validator) getOrCreateThread(t *testing.T, m ThreadID) *schedContext {
	if m == NoThread {
		t.Errorf("must have thread, but thread ID is none")
		return nil
	}
	s, ok := v.ms[m]
	if !ok {
		s = &schedContext{M: m, P: NoProc, G: NoGoroutine}
		v.ms[m] = s
		return s
	}
	return s
}

func goroutine(id GoID) ResourceID {
	return ResourceID{Kind: ResourceGoroutine, id: int64(id)}
}

func proc(id ProcID) ResourceID {
	return ResourceID{Kind: ResourceProc, id: int64(id)}
}

func checkStack(t *testing.T, stk Stack) {
	// Check for non-empty values, but we also check for crashes due to incorrect validation.
	i := 0
	stk.Frames(func(f StackFrame) bool {
		if i == 0 {
			// Allow for one fully zero stack.
			//
			// TODO(mknyszek): Investigate why that happens.
			return true
		}
		if f.Func == "" || f.File == "" || f.PC == 0 || f.Line == 0 {
			t.Errorf("invalid stack frame %#v: missing information", f)
		}
		i++
		return true
	})
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
