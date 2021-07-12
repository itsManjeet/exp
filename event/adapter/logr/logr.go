// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package logr is a logr implementation that uses events.
package logr

import (
	"context"

	"github.com/go-logr/logr"
	"golang.org/x/exp/event"
	"golang.org/x/exp/event/severity"
)

type logger struct {
	ev      *event.Event // cloned, never delivered
	labels  []event.Label
	nameSep string
	name    string
}

// TODO(zchee): support logr.CallDepthLogSink.
var _ logr.LogSink = (*logger)(nil)

// NewLogger returns the new event logger which implements logr interface.
func NewLogger(ctx context.Context, nameSep string) logr.LogSink {
	l := &logger{
		ev:      event.New(ctx, event.LogKind),
		nameSep: nameSep,
	}

	return l
}

// WithName adds a new element to the logger's name.
// Successive calls with WithName continue to append
// suffixes to the logger's name.  It's strongly recommended
// that name segments contain only letters, digits, and hyphens
// (see the package documentation for more information).
func (l *logger) WithName(name string) logr.LogSink {
	l2 := *l
	if l.name == "" {
		l2.name = name
	} else {
		l2.name = l.name + l.nameSep + name
	}
	return &l2
}

// Init receives optional information.
//
// Currently, this method is no-op.
func (l *logger) Init(logr.RuntimeInfo) {}

// Enabled tests whether this Logger is enabled. For example, commandline
// flags might be used to set the logging verbosity and disable some info
// logs.
func (l *logger) Enabled(int) bool {
	return true
}

// Info logs a non-error message with the given key/value pairs as context.
//
// The msg argument should be used to add some constant description to
// the log line.  The key/value pairs can then be used to add additional
// variable information.  The key/value pairs should alternate string
// keys and arbitrary values.
func (l *logger) Info(level int, msg string, keysAndValues ...interface{}) {
	if l.ev == nil {
		return
	}
	l.log(l.ev.Clone(), level, msg, keysAndValues)
}

// Error logs an error, with the given message and key/value pairs as context.
// It functions similarly to calling Info with the "error" named value, but may
// have unique behavior, and should be preferred for logging errors (see the
// package documentations for more information).
//
// The msg field should be used to add context to any underlying error,
// while the err field should be used to attach the actual error that
// triggered this log line, if present.
func (l *logger) Error(err error, msg string, keysAndValues ...interface{}) {
	if l.ev == nil {
		return
	}
	ev := l.ev.Clone()
	ev.Labels = append(ev.Labels, event.Value("error", err))
	l.log(ev, 0, msg, keysAndValues) // 0 means no append severity
}

func (l *logger) log(ev *event.Event, level int, msg string, keysAndValues []interface{}) {
	if level > 0 { // no append when Error method
		ev.Labels = append(ev.Labels, convertVerbosity(level).Label())
	}
	ev.Labels = append(ev.Labels, l.labels...)
	for i := 0; i < len(keysAndValues); i += 2 {
		ev.Labels = append(ev.Labels, newLabel(keysAndValues[i], keysAndValues[i+1]))
	}
	ev.Labels = append(ev.Labels,
		event.String("name", l.name),
		event.String("msg", msg),
	)
	ev.Deliver()
}

// WithValues adds some key-value pairs of context to a logger.
// See Info for documentation on how key/value pairs work.
func (l *logger) WithValues(keysAndValues ...interface{}) logr.LogSink {
	l2 := *l
	if len(keysAndValues) > 0 {
		l2.labels = make([]event.Label, len(l.labels), len(l.labels)+(len(keysAndValues)/2))
		copy(l2.labels, l.labels)
		for i := 0; i < len(keysAndValues); i += 2 {
			l2.labels = append(l2.labels, newLabel(keysAndValues[i], keysAndValues[i+1]))
		}
	}
	return &l2
}

func newLabel(key, value interface{}) event.Label {
	return event.Value(key.(string), value)
}

func convertVerbosity(v int) severity.Level {
	//TODO: this needs to be more complicated, v decreases with increasing severity
	return severity.Level(v)
}
