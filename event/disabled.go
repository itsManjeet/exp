// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build disable_events

package event

import (
	"context"
	"time"
)

type Builder struct{}

type Exporter struct {
	Now func() time.Time
}

func NewExporter(h interface{}) *Exporter { return &Exporter{} }

func To(ctx context.Context) Builder                        { return Builder{} }
func (b Builder) With(label Label) Builder                  { return b }
func (b Builder) WithAll(labels ...Label) Builder           { return b }
func (b Builder) Log(message string)                        {}
func (b Builder) Logf(template string, args ...interface{}) {}
func (b Builder) Metric()                                   {}
func (b Builder) Annotate()                                 {}
func (b Builder) Start(string)                              {}
func (b Builder) End()                                      {}

func newContext(ctx context.Context, exporter *Exporter, parent uint64) context.Context {
	return ctx
}

func setDefaultExporter(e *Exporter) {}
