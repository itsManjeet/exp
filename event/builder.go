// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !disable_events

package event

import (
	"context"
)

// A Builder holds a namespace and labels for events that it builds.
// Builders are intended to be long-lived; they are relatively expensive
// compared to using event.To directly.
type Builder struct {
	Namespace string
	Labels    []Label
}

func NewBuilder(labels ...Label) *Builder {
	b := &Builder{Labels: labels}
	b.Namespace = importPath(3, nil)
	return b
}

func (b *Builder) AddLabel(l Label) {
	b.Labels = append(b.Labels, l)
}

func (b *Builder) To(ctx context.Context) Target {
	t := To(ctx)
	t.builder = b
	return t
}

func (b *Builder) Clone() *Builder {
	return &Builder{
		Namespace: b.Namespace,
		Labels:    b.Labels[:len(b.Labels):len(b.Labels)],
	}
}
