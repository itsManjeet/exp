// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package trace

import (
	"encoding/binary"
	"fmt"

	"golang.org/x/exp/trace/internal/event"
	v2 "golang.org/x/exp/trace/internal/v2"
)

// maxArgs is the maximum number of arguments for "plain" events,
// i.e. anything that could reasonably be represented as a Base.
const maxArgs = 5

// baseEvent is the basic unprocessed event. This serves as a common
// fundamental data structure across.
type baseEvent struct {
	typ  event.Type
	time Time
	args [maxArgs - 1]uint64
}

// readBaseEvent reads out the raw event data from b
// into e. It does not try to interpret the arguments
// but it does validate that the event is a regular
// event with a timestamp (vs. a structural event).
func readBaseEvent(b []byte, e *baseEvent) (int, timestamp, error) {
	// Get the event type.
	typ := event.Type(b[0])
	specs := v2.Specs()
	if int(typ) > len(specs) {
		return 0, 0, fmt.Errorf("found invalid event type: %v", typ)
	}
	e.typ = typ

	// Get spec.
	spec := &specs[typ]
	if len(spec.Args) == 0 || !spec.IsTimedEvent {
		return 0, 0, fmt.Errorf("found event without a timestamp: type=%v", typ)
	}
	n := 1

	// Read timestamp diff.
	ts, nb := binary.Uvarint(b[n:])
	n += nb

	// Read the rest of the arguments.
	for i := 0; i < len(spec.Args)-1; i++ {
		arg, nb := binary.Uvarint(b[n:])
		e.args[i] = arg
		n += nb
	}
	return n, timestamp(ts), nil
}
