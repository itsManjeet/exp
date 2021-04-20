// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package elogrus provides a logrus Formatter for events.
// To use for the global logger:
//   logrus.SetFormatter(elogrus.NewFormatter(exporter))
//   logrus.SetOutput(io.Discard)
// and for a Logger instance:
//   logger.SetFormatter(elogrus.NewFormatter(exporter))
//   logger.SetOutput(io.Discard)
//
// If you call elogging.SetExporter, then you can pass nil
// for the exporter above and it will use the global one.
package elogrus

import (
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/elogging"
	"golang.org/x/exp/elogging/internal"
	"golang.org/x/exp/event"
	"golang.org/x/exp/event/keys"
)

type formatter struct {
	exporter *event.Exporter
}

func NewFormatter(e *event.Exporter) logrus.Formatter {
	if e == nil {
		e = elogging.Exporter()
	}
	return &formatter{exporter: e}
}

var _ logrus.Formatter = (*formatter)(nil)

// Logrus first calls the Formatter to get a []byte, then writes that to the
// output. That doesn't work for events, so we subvert it by having the
// Formatter export the event (and thereby write it). That is why the logrus
// Output io.Writer should be set to io.Discard.
func (f *formatter) Format(e *logrus.Entry) ([]byte, error) {
	var b *event.Builder
	if e.Context != nil {
		b = event.To(e.Context)
	}
	if b == nil {
		b = f.exporter.Builder()
	}
	b.Event.At = e.Time
	b.With(internal.LevelKey.Of(int(e.Level))) // TODO: convert level
	for k, v := range e.Data {
		b.With(keys.Value(k).Of(v))
	}
	b.Log(e.Message)
	return nil, nil
}
