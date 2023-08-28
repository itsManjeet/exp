// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package go122

import (
	"golang.org/x/exp/trace/internal/event"
)

const (
	EvNone event.Type = iota // unused

	// Structural events.
	EvEventBatch // start of per-M batch of events [generation, M ID, timestamp, batch length]
	EvStacks     // start of a section of the stack table [...EvStack]
	EvStack      // stack table entry [ID, ...{PC, func string ID, file string ID, line #}]
	EvStrings    // start of a section of the string dictionary [...EvString]
	EvString     // string dictionary entry [ID, length, string]
	EvCPUSamples // start of a section of CPU samples [...EvCPUSample]
	EvCPUSample  // CPU profiling sample [timestamp, M ID, P ID, goroutine ID, stack]
	EvFrequency  // timestamp units per sec [freq]

	// Procs.
	EvProcsChange // current value of GOMAXPROCS [timestamp, GOMAXPROCS, stack ID]
	EvProcStart   // start of P [timestamp, P ID, P seq]
	EvProcStop    // stop of P [timestamp]
	EvProcSteal   // P was stolen [timestamp, P ID, P seq, M ID]
	EvProcStatus  // P status at the start of a generation [timestamp, P ID, status]

	// Goroutines.
	EvGoCreate            // goroutine creation [timestamp, new goroutine ID, new stack ID, stack ID]
	EvGoStart             // goroutine starts running [timestamp, goroutine ID, goroutine seq]
	EvGoDestroy           // goroutine ends [timestamp]
	EvGoStop              // goroutine yields its time, but is runnable [timestamp, reason, stack ID]
	EvGoBlock             // goroutine blocks [timestamp, reason, stack ID]
	EvGoUnblock           // goroutine is unblocked [timestamp, goroutine ID, goroutine seq, stack ID]
	EvGoSyscallBegin      // syscall enter [timestamp, stack ID]
	EvGoSyscallEnd        // syscall exit [timestamp]
	EvGoSyscallEndBlocked // syscall exit and it blocked at some point [timestamp]
	EvGoStatus            // goroutine status at the start of a generation [timestamp, goroutine ID, status]

	// STW.
	EvSTWBegin // STW start [timestamp, kind]
	EvSTWEnd   // STW done [timestamp]

	// GC events.
	EvGCActive           // GC active [timestamp, seq]
	EvGCBegin            // GC start [timestamp, seq, stack id]
	EvGCEnd              // GC done [timestamp, seq]
	EvGCSweepActive      // GC sweep active [timestamp, goid]
	EvGCSweepBegin       // GC sweep start [timestamp, stack id]
	EvGCSweepEnd         // GC sweep done [timestamp, swept, reclaimed]
	EvGCMarkAssistActive // GC mark assist active [timestamp, goid]
	EvGCMarkAssistBegin  // GC mark assist start [timestamp, stack]
	EvGCMarkAssistEnd    // GC mark assist done [timestamp]
	EvHeapAlloc          // gcController.heapLive change [timestamp, heap_alloc]
	EvHeapGoal           // gcController.heapGoal() (formerly next_gc) change [timestamp, heap goal in bytes]

	// Annotations.
	EvGoLabel         // apply string label to current running goroutine [timestamp, label string ID]
	EvUserTaskBegin   // trace.NewTask [timestamp, internal task ID, internal parent task ID, name string, stack]
	EvUserTaskEnd     // end of a task [timestamp, internal task ID, stack]
	EvUserRegionBegin // trace.{Start,With}Region [timestamp, internal task ID, name string, stack]
	EvUserRegionEnd   // trace.{End,With}Region [timestamp, internal task ID, name string, stack]
	EvUserLog         // trace.Log [timestamp, internal task ID, key string ID, stack, value string ID]
)

func Specs() []event.Spec {
	return specs[:]
}

var specs = [...]event.Spec{
	// "Structural" Events.
	EvEventBatch: event.Spec{
		Name: "EventBatch",
		Args: []string{"gen", "m", "ts", "size"},
	},
	EvStacks: event.Spec{
		Name: "Stacks",
	},
	EvStack: event.Spec{
		Name:    "Stack",
		Args:    []string{"id", "nframes"},
		IsStack: true,
	},
	EvStrings: event.Spec{
		Name: "Strings",
	},
	EvString: event.Spec{
		Name:    "String",
		Args:    []string{"id"},
		HasData: true,
	},
	EvCPUSamples: event.Spec{
		Name: "CPUSamples",
	},
	EvCPUSample: event.Spec{
		Name: "CPUSample",
		Args: []string{"ts", "p", "g", "m", "stack"},
		// N.B. There's clearly a timestamp here, but these Events
		// are special in that they don't appear in the regular
		// M streams.
	},
	EvFrequency: event.Spec{
		Name: "Frequency",
		Args: []string{"freq"},
	},

	// "Timed" Events.
	EvProcsChange: event.Spec{
		Name:         "ProcsChange",
		Args:         []string{"tsdiff", "procs", "stack"},
		IsTimedEvent: true,
		StackIDs:     []int{2},
	},
	EvProcStart: event.Spec{
		Name:         "ProcStart",
		Args:         []string{"tsdiff", "p", "pseq"},
		IsTimedEvent: true,
	},
	EvProcStop: event.Spec{
		Name:         "ProcStop",
		Args:         []string{"tsdiff"},
		IsTimedEvent: true,
	},
	EvProcSteal: event.Spec{
		Name:         "ProcSteal",
		Args:         []string{"tsdiff", "p", "pseq", "m"},
		IsTimedEvent: true,
	},
	EvProcStatus: event.Spec{
		Name:         "ProcStatus",
		Args:         []string{"tsdiff", "p", "status"},
		IsTimedEvent: true,
	},
	EvGoCreate: event.Spec{
		Name:         "GoCreate",
		Args:         []string{"tsdiff", "newg", "newstack", "stack"},
		IsTimedEvent: true,
		StackIDs:     []int{3, 2},
	},
	EvGoStart: event.Spec{
		Name:         "GoStart",
		Args:         []string{"tsdiff", "g", "gseq"},
		IsTimedEvent: true,
	},
	EvGoDestroy: event.Spec{
		Name:         "GoDestroy",
		Args:         []string{"tsdiff"},
		IsTimedEvent: true,
	},
	EvGoStop: event.Spec{
		Name:         "GoStop",
		Args:         []string{"tsdiff", "reason", "stack"},
		IsTimedEvent: true,
		StackIDs:     []int{2},
		StringIDs:    []int{1},
	},
	EvGoBlock: event.Spec{
		Name:         "GoBlock",
		Args:         []string{"tsdiff", "reason", "stack"},
		IsTimedEvent: true,
		StackIDs:     []int{2},
		StringIDs:    []int{1},
	},
	EvGoUnblock: event.Spec{
		Name:         "GoUnblock",
		Args:         []string{"tsdiff", "g", "gseq", "stack"},
		IsTimedEvent: true,
		StackIDs:     []int{3},
	},
	EvGoSyscallBegin: event.Spec{
		Name:         "GoSyscallBegin",
		Args:         []string{"tsdiff", "stack"},
		IsTimedEvent: true,
		StackIDs:     []int{1},
	},
	EvGoSyscallEnd: event.Spec{
		Name:         "GoSyscallEnd",
		Args:         []string{"tsdiff"},
		StartEv:      EvGoSyscallBegin,
		IsTimedEvent: true,
	},
	EvGoSyscallEndBlocked: event.Spec{
		Name:         "GoSyscallEndBlocked",
		Args:         []string{"tsdiff"},
		StartEv:      EvGoSyscallBegin,
		IsTimedEvent: true,
	},
	EvGoStatus: event.Spec{
		Name:         "GoStatus",
		Args:         []string{"tsdiff", "g", "status"},
		IsTimedEvent: true,
	},
	EvSTWBegin: event.Spec{
		Name:         "STWBegin",
		Args:         []string{"tsdiff", "kind", "stack"},
		IsTimedEvent: true,
		StackIDs:     []int{2},
		StringIDs:    []int{1},
	},
	EvSTWEnd: event.Spec{
		Name:         "STWEnd",
		Args:         []string{"tsdiff"},
		StartEv:      EvSTWBegin,
		IsTimedEvent: true,
	},
	EvGCActive: event.Spec{
		Name:         "GCActive",
		Args:         []string{"tsdiff", "seq"},
		IsTimedEvent: true,
		StartEv:      EvGCBegin,
	},
	EvGCBegin: event.Spec{
		Name:         "GCBegin",
		Args:         []string{"tsdiff", "gcseq", "stack"},
		IsTimedEvent: true,
		StackIDs:     []int{2},
	},
	EvGCEnd: event.Spec{
		Name:         "GCEnd",
		Args:         []string{"tsdiff", "gcseq"},
		StartEv:      EvGCBegin,
		IsTimedEvent: true,
	},
	EvGCSweepActive: event.Spec{
		Name:         "GCSweepActive",
		Args:         []string{"tsdiff"},
		IsTimedEvent: true,
	},
	EvGCSweepBegin: event.Spec{
		Name:         "GCSweepBegin",
		Args:         []string{"tsdiff", "stack"},
		IsTimedEvent: true,
		StackIDs:     []int{1},
	},
	EvGCSweepEnd: event.Spec{
		Name:         "GCSweepEnd",
		Args:         []string{"tsdiff", "swept", "reclaimed"},
		StartEv:      EvGCSweepBegin,
		IsTimedEvent: true,
	},
	EvGCMarkAssistActive: event.Spec{
		Name:         "GCMarkAssistActive",
		Args:         []string{"tsdiff", "goid"},
		StartEv:      EvGCMarkAssistBegin,
		IsTimedEvent: true,
	},
	EvGCMarkAssistBegin: event.Spec{
		Name:         "GCMarkAssistBegin",
		Args:         []string{"tsdiff", "stack"},
		IsTimedEvent: true,
		StackIDs:     []int{1},
	},
	EvGCMarkAssistEnd: event.Spec{
		Name:         "GCMarkAssistEnd",
		Args:         []string{"tsdiff"},
		StartEv:      EvGCMarkAssistBegin,
		IsTimedEvent: true,
	},
	EvHeapAlloc: event.Spec{
		Name:         "HeapAlloc",
		Args:         []string{"tsdiff", "heap_alloc"},
		IsTimedEvent: true,
	},
	EvHeapGoal: event.Spec{
		Name:         "HeapGoal",
		Args:         []string{"tsdiff", "heap_goal"},
		IsTimedEvent: true,
	},
	EvGoLabel: event.Spec{
		Name:         "GoLabel",
		Args:         []string{"tsdiff", "label"},
		IsTimedEvent: true,
		StringIDs:    []int{1},
	},
	EvUserTaskBegin: event.Spec{
		Name:         "UserTaskBegin",
		Args:         []string{"tsdiff", "task", "parent_task", "name_string", "stack"},
		IsTimedEvent: true,
		StackIDs:     []int{4},
		StringIDs:    []int{3},
	},
	EvUserTaskEnd: event.Spec{
		Name:         "UserTaskEnd",
		Args:         []string{"tsdiff", "task", "stack"},
		IsTimedEvent: true,
		StackIDs:     []int{2},
	},
	EvUserRegionBegin: event.Spec{
		Name:         "UserRegionBegin",
		Args:         []string{"tsdiff", "task", "name_string", "stack"},
		IsTimedEvent: true,
		StackIDs:     []int{3},
		StringIDs:    []int{2},
	},
	EvUserRegionEnd: event.Spec{
		Name:         "UserRegionEnd",
		Args:         []string{"tsdiff", "task", "name_string", "stack"},
		StartEv:      EvUserRegionBegin,
		IsTimedEvent: true,
		StackIDs:     []int{3},
		StringIDs:    []int{2},
	},
	EvUserLog: event.Spec{
		Name:         "UserLog",
		Args:         []string{"tsdiff", "task", "key_string", "value_string", "stack"},
		IsTimedEvent: true,
		StackIDs:     []int{4},
		StringIDs:    []int{2, 3},
	},
}

type GoStatus uint8

const (
	GoBad GoStatus = iota
	GoRunnable
	GoRunning
	GoSyscall
	GoWaiting
)

type ProcStatus uint8

const (
	ProcBad ProcStatus = iota
	ProcRunning
	ProcIdle

	// ProcSyscall is a status used for validation; it does not appear in the trace.
	ProcSyscall = ^ProcStatus(0)
)
