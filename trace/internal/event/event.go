// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package event

// Type is the common in-memory representation of the low-leve
type Type uint8

// Spec is a specification for an trace event. It contains sufficient information
// to perform basic parsing of any trace event for any version of Go.
type Spec struct {
	// Name is the human-readable name of the trace event.
	Name string

	// Args contains the names of each trace event's argument.
	// Its length determines the number of arguments an event has.
	Args []string

	// StartEv indicates the event type of the corresponding "start"
	// event, if this event is a "end," for a pair of events that
	// represent a time range.
	StartEv Type

	// IsTimedEvent indicates whether this is an event that both
	// appears in the main event stream and is surfaced to the
	// trace reader.
	//
	// Events that are not "timed" are considered "structural"
	// since they either need significant reinterpretation or
	// otherwise aren't actually surfaced by the trace reader.
	IsTimedEvent bool

	// HasData is true if the event has a varint length followed
	// but a number of bytes of data trailing the event.
	HasData bool

	// StringIDs indicates which of the arguments are string IDs.
	StringIDs []int

	// StackIDs indicates which of the arguments are stack IDs.
	//
	// The list is not sorted. The first index always refers to
	// the main stack for the current execution context of the event.
	StackIDs []int

	// IsStack indicates that the event represents a complete
	// stack trace. Specifically, it means that after the arguments
	// there's a varint length, followed by 4*length varints. Each
	// group of 4 represents the PC, file ID, func ID, and line number
	// in that order.
	IsStack bool
}
