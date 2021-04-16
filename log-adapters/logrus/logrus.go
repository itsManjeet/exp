// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package elogrus provides a logrus Formatter for events.
// To use for the global logger:
//   logrus.SetFormatter(elogrus.NewFormatter(exporter))
//   logrus.SetOutput(io.Discard)
// and for a Logger instance:
//   logger.SetFormatter(elogrus.NewFormatter(exporter))
//   loggee.SetOutput(io.Discard)
package elogrus

import (
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/event"
	"golang.org/x/exp/log-adapters/internal"
)

type formatter struct {
	exporter event.Exporter
}

func NewFormatter(e event.Exporter) logrus.Formatter {
	return &formatter{exporter: e}
}

var _ logrus.Formatter = (*formatter)(nil)

// Logrus first calls the Formatter to get a []byte, then writes that to the
// output. That doesn't work for events, so we subvert it by having the
// Formatter export the event (and thereby write it). That is why the logrus
// Output io.Writer should be set to io.Discard.
func (f *formatter) Format(e *logrus.Entry) ([]byte, error) {
	ev := internal.GetLogEvent()
	defer internal.PutLogEvent(ev)
	ev.At = e.Time
	ev.Message = e.Message
	ev.Static = [2]event.Label{internal.LevelKey.Of(int(e.Level))} // TODO: convert level
	ev.Dynamic = make([]event.Label, 0, len(e.Data))
	for k, v := range e.Data {
		ev.Dynamic = append(ev.Dynamic, internal.StringKey(k).Of(v))
	}
	// Use the exporter in the context, if there is one.
	var exporter event.Exporter
	if e.Context != nil {
		exporter, _ = event.FromContext(e.Context)
	}
	if exporter == nil {
		exporter = f.exporter
	}
	exporter.Export(ev)
	return nil, nil
}
