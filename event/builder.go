// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !disable_events

package event

import (
	"context"
	"runtime"
	"strings"
)

type Builder struct {
	Namespace string
	labels    []Label
}

func NewBuilder(namespace string) *Builder {
	if namespace == "" {
		var pcs [1]uintptr
		n := runtime.Callers(2 /* caller of NewBuilder */, pcs[:])
		frames := runtime.CallersFrames(pcs[:n])
		frame, _ := frames.Next()
		// Function is the fully-qualified function name. The name itself may
		// have dots (for a closure, for instance), but it can't have slashes.
		// So the package path ends at the first dot after the last slash.
		i := strings.LastIndexByte(frame.Function, '/')
		if i < 0 {
			i = 0
		}
		end := strings.IndexByte(frame.Function[i:], '.')
		if end >= 0 {
			end += i
		} else {
			end = len(frame.Function)
		}
		namespace = frame.Function[:end]
	}
	return &Builder{Namespace: namespace}
}

func (b *Builder) AddLabels(labels ...Label) {
	b.labels = append(b.labels, labels...)
}

func (b *Builder) To(ctx context.Context) *Event {
	exporter, parent := fromContext(ctx)
	if exporter == nil {
		return nil
	}
	return newEvent(ctx, b.Namespace, exporter, parent, b.labels)
}

func (b *Builder) Trace(ctx context.Context) *Event {
	exporter, parent := fromContext(ctx)
	labels := b.labels
	if exporter == nil {
		labels = nil
	}
	// Even if exporter is nl, we still need an event to hold the context to
	// return from Event.Start.
	return newEvent(ctx, b.Namespace, exporter, parent, labels)
}
