// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package event provides an infinitely buffered event queue.
package event

import (
	"sync"

	"golang.org/x/mobile/event/size"
)

// Queue is an infinitely buffered event queue. The zero value is usable, but
// a Queue value must not be copied.
type Queue struct {
	mu        sync.Mutex
	cond      sync.Cond // cond.L is lazily initialized to &Queue.mu.
	events    []interface{}
	sizeEvent size.Event
}

// NextEvent implements the screen.EventQueue interface.
func (q *Queue) NextEvent() interface{} {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.cond.L == nil {
		q.cond.L = &q.mu
	}

	for {
		if len(q.events) > 0 {
			e := q.events[0]
			q.events = q.events[1:]
			if s, ok := e.(size.Event); ok {
				q.sizeEvent = s
			}
			return e
		}

		q.cond.Wait()
	}
}

// Send implements the screen.EventQueue interface.
func (q *Queue) Send(event interface{}) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.cond.L == nil {
		q.cond.L = &q.mu
	}

	q.events = append(q.events, event)
	q.cond.Signal()
}

// LatestSizeEvent implements the screen.EventQueue interface.
func (q *Queue) LatestSizeEvent() size.Event {
	q.mu.Lock()
	defer q.mu.Unlock()

	return q.sizeEvent
}
