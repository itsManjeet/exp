// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package elogr is a logr implementation that uses events.
package elogr

import (
	"time"

	"github.com/go-logr/logr"
	"golang.org/x/exp/event"
	"golang.org/x/exp/log-adapters/internal"
)

type logger struct {
	exporter  event.Exporter
	nameSep   string
	name      string
	labels    []event.Label
	verbosity int
}

var _ logr.Logger = (*logger)(nil)

func NewLogger(e event.Exporter, nameSep string) logr.Logger {
	return &logger{
		exporter: e,
		nameSep:  nameSep,
	}
}

// WithName adds a new element to the logger's name.
// Successive calls with WithName continue to append
// suffixes to the logger's name.  It's strongly recommended
// that name segments contain only letters, digits, and hyphens
// (see the package documentation for more information).
func (l *logger) WithName(name string) logr.Logger {
	l2 := *l
	if l.name == "" {
		l2.name = name
	} else {
		l2.name = l.name + l.nameSep + name
	}
	return &l2
}

// V returns an Logger value for a specific verbosity level, relative to
// this Logger.  In other words, V values are additive.  V higher verbosity
// level means a log message is less important.  It's illegal to pass a log
// level less than zero.
func (l *logger) V(level int) logr.Logger {
	l2 := *l
	l2.verbosity += level
	return &l2
}

// Enabled tests whether this Logger is enabled.  For example, commandline
// flags might be used to set the logging verbosity and disable some info
// logs.
func (l *logger) Enabled() bool {
	return true
}

// addLabels creates a new []event.Label with the given labels followed by the
// labels constructed from keysAndValues.
func addLabels(labels []event.Label, keysAndValues []interface{}) []event.Label {
	ls := make([]event.Label, len(labels)+len(keysAndValues)/2)
	n := copy(ls, labels)
	j := 0
	for i := n; i < len(ls); i++ {
		ls[i] = newLabel(keysAndValues[j], keysAndValues[j+1])
		j += 2
	}
	return ls
}

// Info logs a non-error message with the given key/value pairs as context.
//
// The msg argument should be used to add some constant description to
// the log line.  The key/value pairs can then be used to add additional
// variable information.  The key/value pairs should alternate string
// keys and arbitrary values.
func (l *logger) Info(msg string, keysAndValues ...interface{}) {
	l.deliver(msg, event.Label{}, keysAndValues)
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
	l.deliver(msg, event.OfValue(event.ErrorKey{}, err), keysAndValues)
}

func (l *logger) deliver(msg string, lab event.Label, keysAndValues []interface{}) {
	ev := internal.GetLogEvent()
	defer internal.PutLogEvent(ev)
	ev.At = time.Now()
	ev.Message = msg
	ev.Static[0] = internal.LevelKey.Of(l.verbosity) // TODO: Convert verbosity to level.
	ev.Static[1] = internal.NameKey.Of(l.name)
	ev.Dynamic = addLabels(l.labels, keysAndValues)
	// TODO: add at least one more static label.
	if lab.Valid() {
		ev.Dynamic = append(ev.Dynamic, lab)
	}
	l.exporter.Export(ev)
}

// WithValues adds some key-value pairs of context to a logger.
// See Info for documentation on how key/value pairs work.
func (l *logger) WithValues(keysAndValues ...interface{}) logr.Logger {
	l2 := *l
	l2.labels = addLabels(l.labels, keysAndValues)
	return &l2
}

func newLabel(key, value interface{}) event.Label {
	return internal.StringKey(key.(string)).Of(value)
}
