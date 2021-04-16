// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package egokit provides a go-kit logger for events.
package egokit

import (
	"fmt"

	"github.com/go-kit/kit/log"
	"golang.org/x/exp/event"
	"golang.org/x/exp/event/keys"
)

type logger struct {
	exporter *event.Exporter
}

func NewLogger(e *event.Exporter) log.Logger {
	return &logger{exporter: e}
}

func (l *logger) Log(keyvals ...interface{}) error {
	b := l.exporter.Builder()
	var msg string
	for i := 0; i < len(keyvals); i += 2 {
		key := keyvals[i].(string)
		value := keyvals[i+1]
		if key == "msg" || key == "message" {
			msg = fmt.Sprint(value)
		} else {
			b.With(keys.Value(key).Of(value))
		}
	}
	b.Log(msg)
	return nil
}
