// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package trace

import (
	"fmt"
	"math"
	"time"

	"golang.org/x/exp/trace/internal/event"
	v2 "golang.org/x/exp/trace/internal/v2"
)

// EventKind indicates the kind of event this is.
//
// Use this information to obtain a more specific event that
// allows access to more detailed information.
type EventKind uint16

const (
	EventBad EventKind = iota

	// EventKindSync is an event that indicates a global synchronization
	// point in the trace. At the point of a sync event, the
	// trace reader can be certain that all resources (e.g. threads,
	// goroutines) that have existed until that point have been enumerated.
	EventSync

	// EventMetric is an event that represents the value of a metric at
	// a particular point in time.
	EventMetric

	// EventLabel attaches a label to a resource.
	EventLabel

	// EventStackSample represents an execution sample, indicating what a
	// thread/proc/goroutine was doing at a particular point in time via
	// its backtrace.
	//
	// Note: Samples should be considered a close approximation of
	// what a thread/proc/goroutine was executing at a given point in time.
	// These events may slightly contradict the situation StateTransitions
	// describe, so they should only be treated as a best-effort annotation.
	EventStackSample

	// EvRangeBegin and EvRangeEnd are a pair of generic events representing
	// a special range of time. Ranges are named and scoped to some resource
	// (identified via ResourceKind). A range that has begun but has not ended
	// is considered active.
	//
	// EvRangeBegin and EvRangeEnd will share the same name, and an End will always
	// follow a Begin on the same instance of the resource. The associated
	// resource ID can be obtained from the Event. ResourceNone indicates the
	// range is globally scoped. That is, any goroutine/proc/thread can start or
	// stop, but only one such range may be active at any given time.
	EventRangeBegin
	EventRangeEnd

	// EvTaskBegin and EvTaskEnd are a pair of events representing a runtime/trace.Task.
	EventTaskBegin
	EventTaskEnd

	// EventRegionBegin and EventRegionEnd are a pair of events represent a runtime/trace.Region.
	EventRegionBegin
	EventRegionEnd

	// EventLog represents a runtime/trace.Log call.
	EventLog

	// Transitions in state for some resource.
	EventStateTransition
)

// String returns a string form of the EventKind.
func (e EventKind) String() string {
	if int(e) >= len(eventKindStrings) {
		return eventKindStrings[0]
	}
	return eventKindStrings[e]
}

var eventKindStrings = [...]string{
	EventBad:             "Bad",
	EventSync:            "Sync",
	EventMetric:          "Metric",
	EventLabel:           "Label",
	EventStackSample:     "StackSample",
	EventRangeBegin:      "RangeBegin",
	EventRangeEnd:        "RangeEnd",
	EventTaskBegin:       "TaskBegin",
	EventTaskEnd:         "TaskEnd",
	EventRegionBegin:     "RegionBegin",
	EventRegionEnd:       "RegionEnd",
	EventLog:             "Log",
	EventStateTransition: "StateTransition",
}

const maxTime = Time(math.MaxInt64)

// Time is a timestamp in nanoseconds.
//
// It corresponds to the monotonic clock on the platform that the
// trace was taken, and so is possible to correlate with timestamps
// for other traces taken on the same machine using the same clock
// (i.e. no reboots in between).
//
// The actual absolute value of the timestamp is only meaningful in
// relation to other timestamps from the same clock.
//
// BUG: Timestamps coming from traces on Windows platforms are
// only comparable with timetamps from the same trace. Timestamps
// across traces cannot be compared, because the system clock is
// not used as of Go 1.22.
//
// BUG: Traces produced by Go versions 1.21 and earlier cannot be
// compared with timestamps from other traces taken on the same
// machine. This is because the system clock was not used at all
// to collect those timestamps.
type Time int64

// Sub subtracts t0 from t, returning the duration in nanoseconds.
func (t Time) Sub(t0 Time) time.Duration {
	return time.Duration(int64(t) - int64(t0))
}

// Metric provides details about a Metric event.
type Metric struct {
	// Name is the name of the sampled metric.
	//
	// Names follow the same convention as metric names in the
	// runtime/metrics package, meaning it includes the unit.
	// Names that match with the runtime/metrics package represent
	// the same quantity. Note that this corresponds to the
	// runtime/metrics package for the Go version this trace was
	// collected for.
	Name string

	// Value is the sampled value of the metric.
	//
	// The Value's Kind is tied to the name of the metric, and so is
	// guaranteed to be the same for metric samples for the same metric.
	Value Value
}

// Label provides details about a Label event.
type Label struct {
	// Label is the label applied to some resource.
	Label string

	// Resource is the resource to which this label was applied for
	// the current event. For example, if Resource is ResourceGoroutine,
	// then the label applies to Event.Goroutine for the Event that produced
	// this Label value.
	Resource ResourceKind
}

// Range provides details about a Range event.
type Range struct {
	// Name is a human-readable name for the range.
	//
	// This name can be used to identify the end of the range for the resource
	// its scoped to, because only one of each type of range may be active on
	// a particular resource. The relevant resource should be obtained from the
	// Event that produced these details. The corresponding RangeEnd will have
	// an identical name.
	Name string

	// Scope is the resource that the range is scoped to.
	//
	// For example, a ResourceGoroutine scope means that the same goroutine
	// must have a start and end for the range, and that goroutine can only
	// have one range of a particular name active at any given time. The
	// ID that this range is scoped to may be obtained via Event.Goroutine.
	//
	// The ResourceNone scope means that the range is globally scoped. As a
	// result, any goroutine/proc/thread may start or end the range, and only
	// one such named range may be active globally at any given time.
	Scope ResourceKind
}

// RangeAttributes provides attributes about a complated Range.
type RangeAttribute struct {
	// Name is the human-readable name for the range.
	Name string

	// Value is the value of the attribute.
	Value Value
}

// TaskID is the internal ID of a task used to disambiguate tasks (even if they
// are of the same type).
type TaskID uint64

// Task provides details about a Task event.
type Task struct {
	// ID is a unique identifier for the task.
	//
	// This can be used to associate the beginning of a task with its end.
	ID TaskID

	// ParentID is s a unique identifier for the task's parent task.
	Parent TaskID

	// Type is the taskType that was passed to runtime/trace.NewTask.
	Type string
}

// Region provides details about a Region event.
type Region struct {
	// Task is the ID of the task this region is associated with.
	Task TaskID

	// Type is the regionType that was passed to runtime/trace.StartRegion or runtime/trace.WithRegion.
	Type string
}

// Log provides details about a Log event.
type Log struct {
	// Task is the ID of the task this region is associated with.
	Task TaskID

	// Category is the category that was passed to runtime/trace.Log or runtime/trace.Logf.
	Category string

	// Message is the message that was passed to runtime/trace.Log or runtime/trace.Logf.
	Message string
}

// StackFrame represents a single frame of a stack.
type StackFrame struct {
	// PC is the program counter of the function call if this
	// is not a leaf frame. If it's a leaf frame, it's the point
	// at which the stack trace was taken.
	PC uint64

	// Func is the name of the function this frame maps to.
	Func string

	// File is the full path to the file on the machine the Go program
	// producing the trace was compiled which contains the source code
	// for the function Func.
	File string

	// Line is the line number within File which maps to PC.
	Line uint64
}

// Event represents a single event in the trace.
type Event struct {
	table *evTable
	ctx   schedCtx
	base  baseEvent
}

// Kind returns the kind of event that this is.
func (e Event) Kind() EventKind {
	return v2Type2Kind[e.base.typ]
}

// Time returns the timestamp of the event.
func (e Event) Time() Time {
	return e.base.time
}

// Goroutine returns the ID of the goroutine this event pertains to.
//
// Note that for goroutine state transitions this always refers to the
// state before the transition. For example, if a goroutine is just
// starting to run on this thread and/or proc, then this will return
// NoGoroutine.
func (e Event) Goroutine() GoID {
	return e.ctx.G
}

// Proc returns the ID of the proc this event event pertains to.
//
// Note that for proc state transitions this always refers to the
// state before the transition. For example, if a proc is just
// starting to run on this thread, then this will return NoProc.
func (e Event) Proc() ProcID {
	return e.ctx.P
}

// Thread returns the ID of the thread this event pertains to.
//
// Note that for thread state transitions this always refers to the
// state before the transition. For example, if a thread is just
// starting to run, then this will return NoThread.
//
// Note: tracking thread state is not currently supported, so this
// will always return a valid thread ID. However thread state transitions
// may be tracked in the future, and callers must be robust to this
// possibility.
func (e Event) Thread() ThreadID {
	return e.ctx.M
}

// HasStack returns true if the iterator from Stack will yield at
// least one frame.
func (e Event) HasStack() bool {
	spec := v2.Specs()[e.base.typ]
	if !spec.HasStack {
		return false
	}
	// Always the last argument if it has a stack, but
	// e.args has already peeled away the timestamp.
	id := stackID(e.base.args[len(spec.Args)-2])
	return id != 0
}

// Stack is an iterator over the frames of a stack trace for the event,
// if it has one.
func (e Event) Stack(yield func(f StackFrame) bool) bool {
	spec := v2.Specs()[e.base.typ]
	if !spec.HasStack {
		return true
	}
	// Always the last argument if it has a stack, but
	// e.args has already peeled away the timestamp.
	id := stackID(e.base.args[len(spec.Args)-2])
	if id == 0 {
		return true
	}
	stk := e.table.stacks[id]
	for _, f := range stk.frames {
		sf := StackFrame{
			PC:   f.pc,
			Func: e.table.strings[f.funcID],
			File: e.table.strings[f.fileID],
			Line: f.line,
		}
		if !yield(sf) {
			return false
		}
	}
	return true
}

// Metric returns details about a Metric event.
//
// Panics if Kind != EventMetric.
func (e Event) Metric() Metric {
	if e.Kind() != EventMetric {
		panic("Metric called on non-Metric event")
	}
	var m Metric
	switch e.base.typ {
	case v2.EvProcsChange:
		m.Name = "/sched/gomaxprocs:threads"
		m.Value = Value{kind: ValueUint64, scalar: e.base.args[0]}
	case v2.EvHeapAlloc:
		m.Name = "/memory/classes/heap/objects:bytes"
		m.Value = Value{kind: ValueUint64, scalar: e.base.args[0]}
	case v2.EvHeapGoal:
		m.Name = "/gc/heap/goal:bytes"
		m.Value = Value{kind: ValueUint64, scalar: e.base.args[0]}
	default:
		panic(fmt.Sprintf("internal error: unexpected event type for Metric kind: %d", e.base.typ))
	}
	return m
}

// Label returns details about a Label event.
//
// Panics if Kind != EventLabel.
func (e Event) Label() Label {
	if e.Kind() != EventLabel {
		panic("Label called on non-Label event")
	}
	if e.base.typ != v2.EvGoLabel {
		panic(fmt.Sprintf("internal error: unexpected event type for Label kind: %d", e.base.typ))
	}
	return Label{
		Label:    e.table.strings[stringID(e.base.args[0])],
		Resource: ResourceGoroutine,
	}
}

// Range returns details about a EventRangeBegin or EventRangeEnd event.
//
// Panics if Kind != EventRangeBegin and Kind != EventRangeEnd.
func (e Event) Range() Range {
	if kind := e.Kind(); kind != EventRangeBegin && kind != EventRangeEnd {
		panic("Range called on non-Range event")
	}
	var r Range
	switch e.base.typ {
	case v2.EvSTWBegin, v2.EvSTWEnd:
		r.Name = "stop-the-world (" + v2.STWReason(e.base.args[0]).String() + ")"
		r.Scope = ResourceGoroutine
	case v2.EvGCBegin, v2.EvGCEnd:
		r.Name = "garbage collection concurrent mark phase"
		r.Scope = ResourceNone
	case v2.EvGCSweepBegin, v2.EvGCSweepEnd:
		r.Name = "garbage collection incremental sweep"
		r.Scope = ResourceGoroutine
	case v2.EvGCMarkAssistBegin, v2.EvGCMarkAssistEnd:
		r.Name = "garbage collection mark phase assist"
		r.Scope = ResourceGoroutine
	default:
		panic(fmt.Sprintf("internal error: unexpected event type for Range kind: %d", e.base.typ))
	}
	return r
}

// RangeAttributes returns attributes for a completed range.
//
// Panics if Kind != EventRangeEnd.
func (e Event) RangeAttributes() []RangeAttribute {
	if e.Kind() != EventRangeEnd {
		panic("Range called on non-Range event")
	}
	if e.base.typ != v2.EvGCSweepEnd {
		return nil
	}
	return []RangeAttribute{
		{
			Name:  "bytes swept",
			Value: Value{kind: ValueUint64, scalar: e.base.args[0]},
		},
		{
			Name:  "bytes reclaimed",
			Value: Value{kind: ValueUint64, scalar: e.base.args[1]},
		},
	}
}

// Task returns details about a TaskBegin or TaskEnd event.
//
// Panics if Kind != EventTaskBegin and Kind != EventTaskEnd.
func (e Event) Task() Task {
	if kind := e.Kind(); kind != EventTaskBegin && kind != EventTaskEnd {
		panic("Task called on non-Task event")
	}
	if e.base.typ != v2.EvUserTaskBegin && e.base.typ != v2.EvUserTaskEnd {
		panic(fmt.Sprintf("internal error: unexpected event type for Task kind: %d", e.base.typ))
	}
	return Task{
		ID:     TaskID(e.base.args[0]),
		Parent: TaskID(e.base.args[1]),
		Type:   e.table.strings[stringID(e.base.args[2])],
	}
}

// Region returns details about a RegionBegin or RegionEnd event.
//
// Panics if Kind != EventRegionBegin and Kind != EventRegionEnd.
func (e Event) Region() Region {
	if kind := e.Kind(); kind != EventRegionBegin && kind != EventRegionEnd {
		panic("Region called on non-Region event")
	}
	if e.base.typ != v2.EvUserRegionBegin && e.base.typ != v2.EvUserRegionEnd {
		panic(fmt.Sprintf("internal error: unexpected event type for Region kind: %d", e.base.typ))
	}
	return Region{
		Task: TaskID(e.base.args[0]),
		Type: e.table.strings[stringID(e.base.args[1])],
	}
}

// Log returns details about a Log event.
//
// Panics if Kind != EventLog.
func (e Event) Log() Log {
	if e.Kind() != EventLog {
		panic("Log called on non-Log event")
	}
	if e.base.typ != v2.EvUserLog {
		panic(fmt.Sprintf("internal error: unexpected event type for Log kind: %d", e.base.typ))
	}
	return Log{
		Task:     TaskID(e.base.args[0]),
		Category: e.table.strings[stringID(e.base.args[1])],
		Message:  e.table.strings[stringID(e.base.args[2])],
	}
}

// StateTransition returns details about a StateTransition event.
//
// Panics if Kind != EventStateTransition.
func (e Event) StateTransition() StateTransition {
	if e.Kind() != EventStateTransition {
		panic("StateTransition called on non-StateTransition event")
	}
	var s StateTransition
	switch e.base.typ {
	case v2.EvProcStart:
		s = procStateTransition(ProcID(e.base.args[0]), ProcIdle, ProcRunning, "")
	case v2.EvProcStop:
		s = procStateTransition(ProcID(e.base.args[0]), ProcRunning, ProcIdle, "")
	case v2.EvProcSteal:
		s = procStateTransition(ProcID(e.base.args[0]), ProcRunning, ProcIdle, "")
	case v2.EvProcStatus:
		// N.B. ordering.advance populates e.base.args[2] here.
		s = procStateTransition(ProcID(e.base.args[0]), ProcState(e.base.args[2]), ProcState(e.base.args[1]), "")
	case v2.EvGoCreate:
		s = goStateTransition(GoID(e.base.args[0]), GoNotExist, GoRunnable, "")
		s.table = e.table
		s.stack = stackID(e.base.args[1])
	case v2.EvGoStart:
		s = goStateTransition(GoID(e.base.args[0]), GoRunnable, GoRunning, "")
	case v2.EvGoDestroy:
		s = goStateTransition(e.ctx.G, GoRunning, GoNotExist, "")
	case v2.EvGoStop:
		s = goStateTransition(e.ctx.G, GoRunning, GoRunnable, v2.GoStopReason(e.base.args[0]).String())
	case v2.EvGoBlock:
		s = goStateTransition(e.ctx.G, GoRunning, GoWaiting, v2.GoBlockReason(e.base.args[0]).String())
	case v2.EvGoUnblock:
		s = goStateTransition(GoID(e.base.args[0]), GoRunning, GoWaiting, "")
	case v2.EvGoSyscallBegin:
		s = goStateTransition(e.ctx.G, GoRunning, GoSyscall, "")
	case v2.EvGoSyscallEnd:
		s = goStateTransition(e.ctx.G, GoSyscall, GoRunning, "")
	case v2.EvGoSyscallEndBlocked:
		s = goStateTransition(e.ctx.G, GoSyscall, GoRunnable, "")
	case v2.EvGoStatus:
		// N.B. ordering.advance populates e.base.args[2] here.
		s = goStateTransition(GoID(e.base.args[0]), GoState(e.base.args[2]), GoState(e.base.args[1]), "")
	default:
		panic(fmt.Sprintf("internal error: unexpected event type for StateTransition kind: %d", e.base.typ))
	}
	return s
}

const evSync = ^event.Type(0)

var v2Type2Kind = [...]EventKind{
	v2.EvCPUSample:           EventStackSample,
	v2.EvProcsChange:         EventMetric,
	v2.EvProcStart:           EventStateTransition,
	v2.EvProcStop:            EventStateTransition,
	v2.EvProcSteal:           EventStateTransition,
	v2.EvProcStatus:          EventStateTransition,
	v2.EvGoCreate:            EventStateTransition,
	v2.EvGoStart:             EventStateTransition,
	v2.EvGoDestroy:           EventStateTransition,
	v2.EvGoStop:              EventStateTransition,
	v2.EvGoBlock:             EventStateTransition,
	v2.EvGoUnblock:           EventStateTransition,
	v2.EvGoSyscallBegin:      EventStateTransition,
	v2.EvGoSyscallEnd:        EventStateTransition,
	v2.EvGoSyscallEndBlocked: EventStateTransition,
	v2.EvGoStatus:            EventStateTransition,
	v2.EvSTWBegin:            EventRangeBegin,
	v2.EvSTWEnd:              EventRangeEnd,
	v2.EvGCBegin:             EventRangeBegin,
	v2.EvGCEnd:               EventRangeEnd,
	v2.EvGCSweepBegin:        EventRangeBegin,
	v2.EvGCSweepEnd:          EventRangeEnd,
	v2.EvGCMarkAssistBegin:   EventRangeBegin,
	v2.EvGCMarkAssistEnd:     EventRangeEnd,
	v2.EvHeapAlloc:           EventMetric,
	v2.EvHeapGoal:            EventMetric,
	v2.EvGoLabel:             EventLabel,
	v2.EvUserTaskBegin:       EventTaskBegin,
	v2.EvUserTaskEnd:         EventTaskEnd,
	v2.EvUserRegionBegin:     EventRegionBegin,
	v2.EvUserRegionEnd:       EventRegionEnd,
	v2.EvUserLog:             EventLog,
	evSync:                   EventSync,
}

func debugPrintEvent(ev Event) {
	print("M=", e.ctx.M, " P=", e.ctx.P, " G=", e.ctx.G)
	if e.base.typ == evSync {
		print("Sync")
	} else {
		spec := v2.Specs()[e.base.typ]
		print(" ", spec.Name, " time=", e.base.time)
		for i, arg := range spec.Args[1:] {
			print(" ", arg, "=", e.base.args[i])
		}
	}
	println()
}
