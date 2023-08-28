// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package v2

import (
	"golang.org/x/exp/trace/internal/event"
)

const (
	EvNone event.Type = iota // unused

	// Structural Events.
	EvEventBatch // start of per-M batch of Events [batch length, parition ID, M ID, timestamp]
	EvStacks     // start of a section of the stack table [...EvStack]
	EvStack      // stack table entry [ID, length, ...{PC, func string ID, file string ID, line #}]
	EvStrings    // start of a section of the string dictionary [...EvString]
	EvString     // string dictionary entry [ID, length, string]
	EvCPUSamples // start of a section of CPU samples [...traceEvCPUSample]
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

	// Global scheduler Events.
	EvSTWBegin // STW start [timestamp, kind, stack id]
	EvSTWEnd   // STW done [timestamp]

	// GC Events.
	EvGCBegin           // GC start [timestamp, seq, stack id]
	EvGCEnd             // GC done [timestamp, seq]
	EvGCSweepBegin      // GC sweep start [timestamp, stack id]
	EvGCSweepEnd        // GC sweep done [timestamp, swept, reclaimed]
	EvGCMarkAssistBegin // GC mark assist start [timestamp, stack]
	EvGCMarkAssistEnd   // GC mark assist done [timestamp]
	EvHeapAlloc         // gcController.heapLive change [timestamp, heap_alloc]
	EvHeapGoal          // gcController.heapGoal() (formerly next_gc) change [timestamp, heap goal in bytes]

	// Annotations.
	EvGoLabel         // apply string label to current running goroutine [timestamp, label string ID]
	EvUserTaskBegin   // trace.NewTask [timestamp, internal task ID, internal parent task ID, Name string, stack]
	EvUserTaskEnd     // end of a task [timestamp, internal task ID, stack]
	EvUserRegionBegin // trace.{Start,With}Region [timestamp, internal task ID, Name string, stack]
	EvUserRegionEnd   // trace.{End,With}Region [timestamp, internal task ID, Name string, stack]
	EvUserLog         // trace.Log [timestamp, internal task ID, key string ID, value string ID, stack]
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
		HasStack:     true,
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
		HasStack:     true,
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
		HasStack:     true,
	},
	EvGoBlock: event.Spec{
		Name:         "GoBlock",
		Args:         []string{"tsdiff", "reason", "stack"},
		IsTimedEvent: true,
		HasStack:     true,
	},
	EvGoUnblock: event.Spec{
		Name:         "GoUnblock",
		Args:         []string{"tsdiff", "g", "gseq", "stack"},
		IsTimedEvent: true,
		HasStack:     true,
	},
	EvGoSyscallBegin: event.Spec{
		Name:         "GoSyscallBegin",
		Args:         []string{"tsdiff", "stack"},
		IsTimedEvent: true,
		HasStack:     true,
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
		HasStack:     true,
	},
	EvSTWEnd: event.Spec{
		Name:         "STWEnd",
		Args:         []string{"tsdiff"},
		StartEv:      EvSTWBegin,
		IsTimedEvent: true,
	},
	EvGCBegin: event.Spec{
		Name:         "GCBegin",
		Args:         []string{"tsdiff", "gcseq", "stack"},
		IsTimedEvent: true,
		HasStack:     true,
	},
	EvGCEnd: event.Spec{
		Name:         "GCEnd",
		Args:         []string{"tsdiff", "gcseq"},
		StartEv:      EvGCBegin,
		IsTimedEvent: true,
	},
	EvGCSweepBegin: event.Spec{
		Name:         "GCSweepBegin",
		Args:         []string{"tsdiff", "stack"},
		IsTimedEvent: true,
		HasStack:     true,
	},
	EvGCSweepEnd: event.Spec{
		Name:         "GCSweepEnd",
		Args:         []string{"tsdiff", "swept", "reclaimed"},
		StartEv:      EvGCSweepBegin,
		IsTimedEvent: true,
	},
	EvGCMarkAssistBegin: event.Spec{
		Name:         "GCMarkAssistBegin",
		Args:         []string{"tsdiff", "stack"},
		IsTimedEvent: true,
		HasStack:     true,
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
	},
	EvUserTaskBegin: event.Spec{
		Name:         "UserTaskBegin",
		Args:         []string{"tsdiff", "task", "parent_task", "name_string", "stack"},
		IsTimedEvent: true,
		HasStack:     true,
	},
	EvUserTaskEnd: event.Spec{
		Name:         "UserTaskEnd",
		Args:         []string{"tsdiff", "task", "stack"},
		IsTimedEvent: true,
		HasStack:     true,
	},
	EvUserRegionBegin: event.Spec{
		Name:         "UserRegionBegin",
		Args:         []string{"tsdiff", "task", "name_string", "stack"},
		IsTimedEvent: true,
		HasStack:     true,
	},
	EvUserRegionEnd: event.Spec{
		Name:         "UserRegionEnd",
		Args:         []string{"tsdiff", "task", "name_string", "stack"},
		StartEv:      EvUserRegionBegin,
		IsTimedEvent: true,
		HasStack:     true,
	},
	EvUserLog: event.Spec{
		Name:         "UserLog",
		Args:         []string{"tsdiff", "task", "key_string", "value_string", "stack"},
		IsTimedEvent: true,
		HasStack:     true,
	},
}

var NameToType = make(map[string]event.Type)

func init() {
	for i, spec := range specs {
		NameToType[spec.Name] = event.Type(byte(i))
	}
}

// GoStopReason is an enumeration of reasons a goroutine might block.
type GoBlockReason uint8

const (
	GoBlockGeneric GoBlockReason = iota
	GoBlockForever
	GoBlockNet
	GoBlockSelect
	GoBlockCondWait
	GoBlockSync
	GoBlockChanSend
	GoBlockChanRecv
	GoBlockGCMarkAssist
	GoBlockGCSweep
	GoBlockSystemGoroutine
	GoBlockPreempted
	GoBlockDebugCall
	GoBlockUntilGCEnds
	GoBlockSleep
)

// String returns a human-readable form of the block reason.
//
// The string completes the following sentence:
// "The goroutine blocked ____."
func (r GoBlockReason) String() string {
	return goBlockReasonStrings[r]
}

var goBlockReasonStrings = [...]string{
	GoBlockGeneric:         "for an unspecified reason",
	GoBlockForever:         "forever",
	GoBlockNet:             "to wait on the network",
	GoBlockSelect:          "to wait in a select block",
	GoBlockCondWait:        "to wait on a sync.Cond",
	GoBlockSync:            "to wait on a resource from the sync package",
	GoBlockChanSend:        "to wait to send on a channel",
	GoBlockChanRecv:        "to wait to receive from a channel",
	GoBlockGCMarkAssist:    "to wait for GC mark assist work",
	GoBlockGCSweep:         "to wait for GC sweep work",
	GoBlockSystemGoroutine: "for runtime-internal reason",
	GoBlockPreempted:       "by the runtime",
	GoBlockDebugCall:       "for a debug call",
	GoBlockUntilGCEnds:     "to wait until the current GC ends",
	GoBlockSleep:           "to sleep",
}

// GoStopReason is an enumeration of reasons a goroutine might yield.
type GoStopReason uint8

const (
	GoStopGeneric GoStopReason = iota
	GoStopGoSched
	GoStopPreempted
)

// String returns a human-readable form of the yield reason.
//
// The string completes the following sentence:
// "The goroutine stopped ____."
func (r GoStopReason) String() string {
	return goStopReasonStrings[r]
}

var goStopReasonStrings = [...]string{
	GoStopGeneric:   "for an unspecified reason",
	GoStopGoSched:   "voluntarily via runtime.Gosched",
	GoStopPreempted: "involuntarily by the runtime",
}

// STWReason is an enumeration of reasons the world is stopping.
type STWReason uint8

// Reasons to stop-the-world.
//
// Avoid reusing reasons and add new ones instead.
const (
	STWUnknown                     STWReason = iota // "unknown"
	STWGCMarkTerm                                   // "GC mark termination"
	STWGCSweepTerm                                  // "GC sweep termination"
	STWWriteHeapDump                                // "write heap dump"
	STWGoroutineProfile                             // "goroutine profile"
	STWGoroutineProfileCleanup                      // "goroutine profile cleanup"
	STWAllGoroutinesStack                           // "all goroutines stack trace"
	STWReadMemStats                                 // "read mem stats"
	STWAllThreadsSyscall                            // "AllThreadsSyscall"
	STWGOMAXPROCS                                   // "GOMAXPROCS"
	STWStartTrace                                   // "start trace"
	STWStopTrace                                    // "stop trace"
	STWForTestCountPagesInUse                       // "CountPagesInUse (test)"
	STWForTestReadMetricsSlow                       // "ReadMetricsSlow (test)"
	STWForTestReadMemStatsSlow                      // "ReadMemStatsSlow (test)"
	STWForTestPageCachePagesLeaked                  // "PageCachePagesLeaked (test)"
	STWForTestResetDebugLog                         // "ResetDebugLog (test)"
)

func (r STWReason) String() string {
	if int(r) < len(stwReasonStrings) {
		return stwReasonStrings[r]
	}
	return "unknown"
}

var stwReasonStrings = [...]string{
	"unknown",
	"GC mark termination",
	"GC sweep termination",
	"write heap dump",
	"goroutine profile",
	"goroutine profile cleanup",
	"all goroutines stack trace",
	"read mem stats",
	"AllThreadsSyscall",
	"GOMAXPROCS",
	"start trace",
	"stop trace",
	"CountPagesInUse (test)",
	"ReadMetricsSlow (test)",
	"ReadMemStatsSlow (test)",
	"PageCachePagesLeaked (test)",
	"ResetDebugLog (test)",
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
)
