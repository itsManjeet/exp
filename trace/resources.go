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

// String returns a human-readable representation of a GoState.
//
// The format of the returned string is for debugging purposes and is subject to change.
func (s GoState) String() string {
	switch s {
	case GoUndetermined:
		return "Undetermined"
	case GoNotExist:
		return "NotExist"
	case GoRunnable:
		return "Runnable"
	case GoRunning:
		return "Running"
	case GoWaiting:
		return "Waiting"
	case GoSyscall:
		return "Syscall"
	}
	return "Bad"
}

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

// String returns a human-readable representation of a ProcState.
//
// The format of the returned string is for debugging purposes and is subject to change.
func (s ProcState) String() string {
	switch s {
	case ProcUndetermined:
		return "Undetermined"
	case ProcNotExist:
		return "NotExist"
	case ProcRunning:
		return "Running"
	case ProcIdle:
		return "Idle"
	}
	return "Bad"
}

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

// String returns a human-readable representation of a ResourceKind.
//
// The format of the returned string is for debugging purposes and is subject to change.
func (r ResourceKind) String() string {
	switch r {
	case ResourceNone:
		return "None"
	case ResourceGoroutine:
		return "Goroutine"
	case ResourceProc:
		return "Proc"
	case ResourceThread:
		return "Thread"
	}
	return "Bad"
}

// StateTransition provides details about a StateTransition event.
type StateTransition struct {
	// Resource is the kind of resource this state transition is for.
	Resource ResourceKind

	// Reason is a human-readable reason for the state transition.
	Reason string

	// Stack is the stack trace of the resource making the state transition.
	//
	// This is distinct from the result (Event).Stack because it pertains to
	// the transitioning resource, not any of the ones executing the event
	// this StateTransition came from.
	//
	// An example of this difference is the NotExist -> Runnable transition for
	// goroutines, which indicates goroutine creation. In this particular case,
	// a Stack here would refer to the starting stack of the new goroutine, and
	// an (Event).Stack would refer to the stack trace of whoever created the
	// goroutine.
	Stack Stack

	// The actual transition data. Stored in a neutral form so that
	// we don't need fields for every kind of resource.
	id       int64
	oldState uint8
	newState uint8
}

func goStateTransition(id GoID, from, to GoState) StateTransition {
	return StateTransition{
		Resource: ResourceGoroutine,
		id:       int64(id),
		oldState: uint8(from),
		newState: uint8(to),
	}
}

func procStateTransition(id ProcID, from, to ProcState) StateTransition {
	return StateTransition{
		Resource: ResourceProc,
		id:       int64(id),
		oldState: uint8(from),
		newState: uint8(to),
	}
}

// Goroutine returns the state transition for a goroutine.
//
// Transitions to and from GoRunning and GoSyscall are special in that
// they change the future execution context. In other words, future events
// on the same thread will feature the same goroutine until it stops running.
//
// Panics if d.Resource is not ResourceGoroutine.
func (d StateTransition) Goroutine() (id GoID, from, to GoState) {
	if d.Resource != ResourceGoroutine {
		panic("Goroutine called on non-Goroutine state transition")
	}
	return GoID(d.id), GoState(d.oldState), GoState(d.newState)
}

// Proc returns the state transition for a proc.
//
// Transitions to and from ProcRunning is special in that they change
// the future execution context. In other words, future events on the
// same thread will feature the same goroutine until it stops running.
//
// Panics if d.Resource is not ResourceProc.
func (d StateTransition) Proc() (id ProcID, from, to ProcState) {
	if d.Resource != ResourceProc {
		panic("Proc called on non-Proc state transition")
	}
	return ProcID(d.id), ProcState(d.oldState), ProcState(d.newState)
}
