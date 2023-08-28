// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package trace

// ThreadID is the runtime-internal M structure's ID. This is unique
// for each OS thread.
type ThreadID int64

// NoThread indicates that the relevant events don't correspond to any
// thread in particular.
const NoThread = ThreadID(-1)

// ProcID is the runtime-internal G structure's id field. This is unique
// for each P.
type ProcID int64

// NoProc indicates that the relevant events don't correspond to any
// P in particular.
const NoProc = ProcID(-1)

// GoID is the runtime-internal G structure's goid field. This is unique
// for each goroutine.
type GoID int64

// NoGoroutine indicates that the relevant events don't correspond to any
// goroutine in particular.
const NoGoroutine = GoID(-1)

// GoState represents the state of a goroutine.
//
// New GoStates may be added in the future. Users of this type must be robust
// to that possibility.
type GoState uint8

const (
	GoUndetermined GoState = iota // No information is known about the goroutine.
	GoNotExist                    // Goroutine does not exist.
	GoRunnable                    // Goroutine is runnable but not running.
	GoRunning                     // Goroutine is running.
	GoWaiting                     // Goroutine is waiting on something to happen.
	GoSyscall                     // Goroutine is in a system call.
)

// ProcState represents the state of a proc.
//
// New ProcStates may be added in the future. Users of this type must be robust
// to that possibility.
type ProcState uint8

const (
	ProcUndetermined ProcState = iota // No information is known about the proc.
	ProcNotExist                      // Proc does not exist.
	ProcRunning                       // Proc is running.
	ProcIdle                          // Proc is idle.
)

// ResourceKind indicates a kind of resource that has a state machine.
//
// New ResourceKinds may be added in the future. Users of this type must be robust
// to that possibility.
type ResourceKind uint8

const (
	ResourceNone      ResourceKind = iota // No resource.
	ResourceGoroutine                     // Goroutine.
	ResourceProc                          // Proc.
	ResourceThread                        // Thread.
)

// StateTransition provides details about a StateTransition event.
type StateTransition struct {
	table    *evTable
	reason   string
	stack    stackID
	id       int64
	kind     ResourceKind
	oldState uint8
	newState uint8
}

func goStateTransition(id GoID, from, to GoState, reason string) StateTransition {
	return StateTransition{
		reason:   reason,
		id:       int64(id),
		kind:     ResourceGoroutine,
		oldState: uint8(from),
		newState: uint8(to),
	}
}

func procStateTransition(id ProcID, from, to ProcState, reason string) StateTransition {
	return StateTransition{
		reason:   reason,
		id:       int64(id),
		kind:     ResourceProc,
		oldState: uint8(from),
		newState: uint8(to),
	}
}

// Resource returns the kind of resource this state transition is for.
func (d StateTransition) Resource() ResourceKind {
	return d.kind
}

// Reason returns a human-readable reason for the state transition.
func (d StateTransition) Reason() string {
	return d.reason
}

// HasStack returns true if the Stack iterator will yield at least one frame.
func (d StateTransition) HasStack() bool {
	return d.stack != 0
}

// Stack is an iterator over the stack trace of the resource transitioning,
// if one exists.
//
// Note: this is *not* the same as Event.Stack, which is the stack trace
// for event itself. For example, goroutine creation always happens on
// an existing goroutine. In that case, Event.Stack would return the stack
// of that existing goroutine at the point of creation. This method would
// instead return the starting stack of the new goroutine (its transition
// from NotExist to Runnable).
func (d StateTransition) Stack(yield func(f StackFrame) bool) bool {
	if d.stack == 0 {
		return true
	}
	stk := d.table.stacks[d.stack]
	for _, f := range stk.frames {
		sf := StackFrame{
			PC:   f.pc,
			Func: d.table.strings[f.funcID],
			File: d.table.strings[f.fileID],
			Line: f.line,
		}
		if !yield(sf) {
			return false
		}
	}
	return true
}

// Goroutine returns the state transition for a goroutine.
//
// Panics if d.Resource is not ResourceGoroutine.
func (d StateTransition) Goroutine() (id GoID, from, to GoState) {
	if d.kind != ResourceGoroutine {
		panic("Goroutine called on non-Goroutine state transition")
	}
	return GoID(d.id), GoState(d.oldState), GoState(d.newState)
}

// Proc returns the state transition for a proc.
//
// Panics if d.Resource is not ResourceProc.
func (d StateTransition) Proc() (id ProcID, from, to ProcState) {
	if d.kind != ResourceProc {
		panic("Proc called on non-Proc state transition")
	}
	return ProcID(d.id), ProcState(d.oldState), ProcState(d.newState)
}
