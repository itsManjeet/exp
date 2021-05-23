// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !disable_events

package event

import (
	"context"
)

type Builder struct {
	namespace string
	labels    []Label
}

func NewBuilder(namespace string) *Builder {
	return &Builder{namespace: namespace}
}

func (b *Builder) AddLabels(labels ...Label) {
	b.labels = append(b.labels, labels...)
}

func (b *Builder) To(ctx context.Context) *Event {
	exporter, parent := fromContext(ctx)
	if exporter == nil {
		return nil
	}
	return newEvent(ctx, b.namespace, exporter, parent, b.labels)
}

func (b *Builder) Trace(ctx context.Context) *Event {
	exporter, parent := fromContext(ctx)
	labels := b.labels
	if exporter == nil {
		labels = nil
	}
	// Even if exporter is nl, we still need an event to hold the context to
	// return from Event.Start.
	return newEvent(ctx, b.namespace, exporter, parent, labels)
}
