// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package export

import (
	"sync"

	"golang.org/x/exp/event"
)

// LogWriter returns an Exporter that logs events to the supplied printer.
// If onlyErrors is true it does not log any event that did not have an
// associated error.
// It ignores all telemetry other than log events.
func LogWriter(p event.Printer, onlyErrors bool) event.Exporter {
	lw := &logWriter{printer: p, onlyErrors: onlyErrors}
	return lw
}

type logWriter struct {
	mu         sync.Mutex
	printer    event.Printer
	onlyErrors bool
}

func (w *logWriter) Export(ev *event.Event) {
	switch ev.Kind {
	case event.LogKind:
		if w.onlyErrors && !ev.Find(event.ErrorKey{}).Valid() {
			return
		}
		w.mu.Lock()
		defer w.mu.Unlock()
		w.printer.Event(ev)
	}
}
