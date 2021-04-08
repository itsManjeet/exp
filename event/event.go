// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package event

import (
	"time"
)

// LabelArray is a set of labels inlined into Labels.
// As Labels are often on the stack, storing the first few labels directly can
// avoid an allocation at all for the very common cases of simple events.
// The length needs to be large enough to cope with the majority of events
// but no so large as to cause undue stack pressure.
type LabelArray [3]Label

// Labels is the set of user supplied labels that make up an events data.
// The first static label is normally an indicator of the message type.
type Labels struct {
	Kind    interface{}
	Message string
	Static  LabelArray // inline storage for the first few labels
	Dynamic []Label    // dynamically sized storage for remaining labels
}

// Meta contains the extra information that is added to an event by the system
// before it is delivered to the exporter.
type Meta struct {
	At     time.Time // time at which the event is delivered to the exporter.
	ID     uint64    // unique for this process id of the event
	Parent uint64    // id of the parent event for this event
}

// Event holds the information about an event that occurred.
// It combines the event metadata with the user supplied labels.
type Event struct {
	Labels
	Meta
}
