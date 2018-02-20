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
	mu       sync.Mutex
	nonempty chan bool     // Buffered; sent with mu locked when the queue is not empty.
	back     []interface{} // FIFO.
	front    []interface{} // LIFO; used only when front is not empty.
}

func (q *Deque) lockAndInit() {
	q.mu.Lock()
	if q.nonempty == nil {
		q.nonempty = make(chan bool, 1)
	}
}

func (q *Deque) signal() {
	select {
	case q.nonempty <- true:
	default:
	}
}

// NextEvent implements the screen.EventDeque interface.
func (q *Deque) NextEvent() interface{} {
	q.lockAndInit()
	defer q.mu.Unlock()

	for len(q.front) == 0 {
		q.mu.Unlock()
		<-q.nonempty
		q.mu.Lock()
	}
	i := len(q.front) - 1
	event := q.front[i]
	q.front[i] = nil
	q.front = q.front[:i]

	if len(q.front) == 0 {
		for n := len(q.back); n > 0; n-- {
			q.front = append(q.front, q.back[n-1])
			q.back[n-1] = nil
		}
		q.back = q.back[:0]
	}
	if len(q.front) != 0 {
		q.signal()
	}
	return event
}

// Send implements the screen.EventDeque interface.
func (q *Deque) Send(event interface{}) {
	q.lockAndInit()
	defer q.mu.Unlock()

	if len(q.front) == 0 && len(q.back) == 0 {
		q.front = append(q.front, event)
	} else {
		q.back = append(q.back, event)
	}
	q.signal()
}

// SendFirst implements the screen.EventDeque interface.
func (q *Deque) SendFirst(event interface{}) {
	q.lockAndInit()
	defer q.mu.Unlock()

	q.front = append(q.front, event)
	q.signal()
}
