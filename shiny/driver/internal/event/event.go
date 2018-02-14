// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package event provides an infinitely buffered double-ended queue of events.
package event // import "golang.org/x/exp/shiny/driver/internal/event"

import (
	"sync"
)

// Deque is an infinitely buffered double-ended queue of events. The zero value
// is usable, but a Deque value must not be copied.
type Deque struct {
	mu    sync.Mutex
	head  chan interface{} // Buffered with length 1; empty or contains the head of the queue.
	front []interface{}    // LIFO events after the head.
	back  []interface{}    // FIFO events after front; used only when front is non-empty.
}

func (q *Deque) lockAndInit() {
	q.mu.Lock()
	if q.head == nil {
		q.head = make(chan interface{}, 1)
	}
}

// NextEvent implements the screen.EventDeque interface.
func (q *Deque) NextEvent() (event interface{}) {
	// If we could guarantee that q.head is always initialized, we could simplify
	// this block (and avoid some contention on q.mu) as:
	// 	event := <-q.head
	// 	q.mu.Lock()
	// 	defer q.mu.Unlock()
	q.lockAndInit()
	defer q.mu.Unlock()
	select {
	case event = <-q.head:
	default:
		q.mu.Unlock()
		event = <-q.head
		q.mu.Lock()
	}

	if len(q.front) == 0 {
		return event
	}
	i := len(q.front) - 1
	select {
	case q.head <- q.front[i]:
	default:
		return event
	}
	q.front[i] = nil
	q.front = q.front[:i]

	if len(q.front) == 0 {
		for n := len(q.back); n > 0; n-- {
			q.front = append(q.front, q.back[n-1])
			q.back[n-1] = nil
		}
		q.back = q.back[:0]
	}
	return event
}

// Send implements the screen.EventDeque interface.
func (q *Deque) Send(event interface{}) {
	q.lockAndInit()
	defer q.mu.Unlock()

	// The head event must be stored in the head channel.
	if len(q.front) == 0 && len(q.back) == 0 {
		select {
		case q.head <- event:
		default:
			// Maintain the invariant that q.back is unused when q.front is empty.
			// That avoids unnecessary copying in the steady state and somewhat
			// simplifies NextEvent.
			q.front = append(q.front, event)
		}
		return
	}

	q.back = append(q.back, event)
}

// SendFirst implements the screen.EventDeque interface.
func (q *Deque) SendFirst(event interface{}) {
	q.lockAndInit()
	defer q.mu.Unlock()

	// Move the previous head, if any, to the front slice.
	select {
	case prev := <-q.head:
		q.front = append(q.front, prev)
	default:
	}

	q.head <- event
}
