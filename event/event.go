// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package event

import (
	"context"
	"time"
)

// Event holds the information about an event that occurred.
// It combines the event metadata with the user supplied labels.
type Event struct {
	ID     uint64    // only set for trace events, a unique id per exporter for the trace.
	Parent uint64    // id of the parent event for this event
	At     time.Time // time at which the event is delivered to the exporter.
	Labels []Label
}

// Handler is a the type for something that handles events as they occur.
type Handler interface {
	// Handle is called with all events delivered by the exporter.
	Handle(context.Context, *Event) context.Context
}

// WithExporter returns a context with the exporter attached.
// The exporter is called synchronously from the event call site, so it should
// return quickly so as not to hold up user code.
func WithExporter(ctx context.Context, e *Exporter) context.Context {
	return newContext(ctx, e, 0)
}

// SetDefaultExporter sets an exporter that is used if no exporter can be
// found on the context.
func SetDefaultExporter(e *Exporter) {
	setDefaultExporter(e)
}
