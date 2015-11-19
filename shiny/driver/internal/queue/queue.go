// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package queue provides an infinitely buffered event queue.
package queue

import "sync"

// Make initializes an Events queue.
func Make() Events {
	return Events{cond: sync.Cond{L: new(sync.Mutex)}}
}

// Events is an ordered infinite queue of events.
type Events struct {
	cond   sync.Cond
	events []interface{}
	done   bool
}

// NextEvent returns the next event in the queue.
func (e *Events) NextEvent() interface{} {
	e.cond.L.Lock()
	defer e.cond.L.Unlock()

	for len(e.events) == 0 {
		if e.done {
			panic("queue: released, no more events to process")
		}
		e.cond.Wait()
	}

	event := e.events[0]
	e.events = e.events[1:]
	return event
}

// Send adds an event to the queue.
// Send returns quickly and will never block waiting for NextEvent.
func (e *Events) Send(event interface{}) {
	e.cond.L.Lock()
	defer e.cond.L.Unlock()

	// TODO(crawshaw): with some song and dance, the periodic allocation
	// of e.events can be avoided in the common case. Leaving as a TODO
	// until I can see it in a profile.
	e.events = append(e.events, event)
	e.cond.Signal()
}

// Release disposes of the pending queue and delivers a final event.
func (e *Events) Release(event interface{}) {
	e.cond.L.Lock()
	defer e.cond.L.Unlock()

	e.events = append(e.events[:0], event)
	e.done = true
	e.cond.Signal()
}
