// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package egokit provides a go-kit logger for events.
package egokit

import (
	"time"

	"github.com/go-kit/kit/log"
	"golang.org/x/exp/event"
	"golang.org/x/exp/log-adapters/internal"
)

type logger struct {
	exporter event.Exporter
}

func NewLogger(e event.Exporter) log.Logger {
	return &logger{exporter: e}
}

func (l *logger) Log(keyvals ...interface{}) error {
	ev := internal.GetLogEvent()
	defer internal.PutLogEvent(ev)
	ev.At = time.Now()
	if nd := len(keyvals)/2 - len(ev.Static); nd > 0 {
		// This will leave an extra slot if one of the keys is the message.
		ev.Dynamic = make([]event.Label, nd)
	}
	n := 0
	add := func(key string, value interface{}) {
		l := internal.StringKey(key).Of(value)
		if n < len(ev.Static) {
			ev.Static[n] = l
		} else {
			ev.Dynamic[n-len(ev.Static)] = l
		}
		n++
	}

	for i := 0; i < len(keyvals); i += 2 {
		key := keyvals[i].(string)
		value := keyvals[i+1]
		if key == "msg" || key == "message" {
			ev.Message = value.(string)
		} else {
			add(key, value)
		}
	}
	l.exporter.Export(ev)
	return nil
}
