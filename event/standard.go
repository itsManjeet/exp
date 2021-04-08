// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package event

import (
	"context"
)

type standardKind string

const (
	// LogKind is a Labels kind that indicates a log event.
	LogKind = standardKind("log")

	// StartKind is a Labels kind that indicates a span start event.
	StartKind = standardKind("start")

	// EndKind is a Labels kind that indicates a span end event.
	EndKind = standardKind("end")

	// MetricKind is a Labels kind that indicates a metric record event.
	MetricKind = standardKind("metric")

	// RecordKind is a Labels kind that reports label values at a point in time.
	RecordKind = standardKind("record")
)

// ErrKey is a key used to add error values to label lists.
type ErrKey struct{}

// Log takes a message and a label list and combines them into a single event
// before delivering them to the exporter.
func Log(ctx context.Context, message string, labels ...Label) {
	Export(ctx, Labels{Kind: LogKind, Message: message, Dynamic: labels})
}

// Log1 takes a message and one label delivers a log event to the exporter.
// It is a customized version of Log that is faster and does no allocation.
func Log1(ctx context.Context, message string, t1 Label) {
	Export(ctx, Labels{Kind: LogKind, Message: message, Static: LabelArray{t1}})
}

// Log2 takes a message and two labels and delivers a log event to the exporter.
// It is a customized version of Print that is faster and does no allocation.
func Log2(ctx context.Context, message string, t1 Label, t2 Label) {
	Export(ctx, Labels{Kind: LogKind, Message: message, Static: LabelArray{t1, t2}})
}

// Error takes a message and a label list and combines them into a single event
// before delivering them to the exporter. It captures the error in the
// delivered event as the first label using ErrKey.
func Error(ctx context.Context, message string, err error, labels ...Label) {
	Export(ctx, Labels{
		Kind:    LogKind,
		Message: message,
		Static:  LabelArray{OfValue(ErrKey{}, err)},
		Dynamic: labels,
	})
}

// Start sends a span start event with the supplied label list to the exporter.
// It also returns a context that holds span, you should always call End on
// the context.
func Start(ctx context.Context, name string, labels ...Label) context.Context {
	return withSpan(ctx, Labels{Kind: StartKind, Message: name, Dynamic: labels})
}

// Start1 sends a span start event with the supplied label list to the exporter.
// It also returns a function that will end the span, which should normally be
// deferred.
func Start1(ctx context.Context, name string, t1 Label) context.Context {
	return withSpan(ctx, Labels{Kind: StartKind, Message: name, Static: LabelArray{t1}})
}

// Start2 sends a span start event with the supplied label list to the exporter.
// It also returns a function that will end the span, which should normally be
// deferred.
func Start2(ctx context.Context, name string, t1, t2 Label) context.Context {
	return withSpan(ctx, Labels{Kind: StartKind, Message: name, Static: LabelArray{t1, t2}})
}

// withSpan returns a context with a new span.
// It has no effect if the context does not have an exporter set.
// It has a shortcut that returns quickly if WithExporter has never been called.
func withSpan(ctx context.Context, labels Labels) context.Context {
	v := get(ctx)
	if v.exporter == nil {
		return ctx
	}
	v.parent = deliver(ctx, v.exporter, v.parent, labels)
	return context.WithValue(ctx, contextKey{}, v)
}

// End sends a span end event to the exporter.
func End(ctx context.Context) {
	Export(ctx, Labels{Kind: EndKind})
}

// Metric sends a metric event to the exporter with the supplied labels.
func Metric(ctx context.Context, labels ...Label) {
	Export(ctx, Labels{Kind: MetricKind, Dynamic: labels})
}

// Metric1 sends a metric event to the exporter with the supplied labels.
func Metric1(ctx context.Context, t1 Label) {
	Export(ctx, Labels{Kind: MetricKind, Static: LabelArray{t1}})
}

// Metric2 sends a metric event to the exporter with the supplied labels.
func Metric2(ctx context.Context, t1, t2 Label) {
	Export(ctx, Labels{Kind: MetricKind, Static: LabelArray{t1, t2}})
}

// Record sends a record event to the exporter with the supplied labels.
func Record(ctx context.Context, labels ...Label) {
	Export(ctx, Labels{Kind: RecordKind, Dynamic: labels})
}

// Record1 sends a record event to the exporter with the supplied labels.
func Record1(ctx context.Context, t1 Label) {
	Export(ctx, Labels{Kind: RecordKind, Static: LabelArray{t1}})
}

// Record2 sends a record event to the exporter with the supplied labels.
func Record2(ctx context.Context, t1, t2 Label) {
	Export(ctx, Labels{Kind: RecordKind, Static: LabelArray{t1, t2}})
}

func (k ErrKey) Name() string { return "error" }

func (k ErrKey) Print(p Printer, l Label) {
	p.String(l.UnpackValue().(error).Error())
}

func (k ErrKey) From(l Label) (error, bool) {
	if l.Key() != k {
		return nil, false
	}
	err, _ := l.UnpackValue().(error)
	return err, true
}
