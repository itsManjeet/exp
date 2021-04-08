// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package metric aggregates events into metrics that can be exported.
package metric

import (
	"sync"
	"time"

	"golang.org/x/exp/event"
	"golang.org/x/exp/event/keys"
)

var Entries = keys.New("metric_entries", "The set of metrics calculated for an event")

type Config struct {
	subscribers map[interface{}][]subscriber
}

type subscriber func(time.Time, event.Event, event.Label) Data

type exporter struct {
	mu     sync.Mutex
	config Config
}

func (e *Config) subscribe(key event.Key, s subscriber) {
	if e.subscribers == nil {
		e.subscribers = make(map[interface{}][]subscriber)
	}
	e.subscribers[key] = append(e.subscribers[key], s)
}

func (e *Config) Exporter() event.Exporter {
}

func (e *exporter) Export(ev event.Event) {
	if ev.Kind != event.MetricKind {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	var metrics []Data

	for index := 0; ev.Valid(index); index++ {
		l := ev.Label(index)
		if !l.Valid() {
			continue
		}
		id := l.Key()
		if list := e.subscribers[id]; len(list) > 0 {
			for _, s := range list {
				metrics = append(metrics, s(ev.At, ev, l))
			}
		}
	}
}
