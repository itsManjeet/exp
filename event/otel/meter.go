// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package otel

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"golang.org/x/exp/event"
)

// MetricHandler is an event.MetricHandler for OpenTelemetry metrics.
// To use, first create with NewMetricHandler. Then register the metrics
// of interest with the RegisterXXX methods.
type MetricHandler struct {
	metrics map[metricName]metricFunc
}

var _ event.MetricHandler = (*MetricHandler)(nil)

type metricName struct {
	namespace, name string
}

type metricFunc struct {
	// exactly one of:
	intFunc   func(context.Context, int64, ...attribute.KeyValue)
	floatFunc func(context.Context, float64, ...attribute.KeyValue)
}

func NewMetricHandler() *MetricHandler {
	return &MetricHandler{metrics: map[metricName]metricFunc{}}
}

func (h *MetricHandler) RegisterInt64Counter(namespace, name string, m metric.Int64Counter) {
	h.metrics[metricName{namespace, name}] = metricFunc{m.Add, nil}
}

func (h *MetricHandler) RegisterInt64UpDownCounter(namespace, name string, m metric.Int64UpDownCounter) {
	h.metrics[metricName{namespace, name}] = metricFunc{m.Add, nil}
}

func (h *MetricHandler) RegisterInt64ValueRecorder(namespace, name string, m metric.Int64ValueRecorder) {
	h.metrics[metricName{namespace, name}] = metricFunc{m.Record, nil}
}

func (h *MetricHandler) RegisterFloat64Counter(namespace, name string, m metric.Float64Counter) {
	h.metrics[metricName{namespace, name}] = metricFunc{nil, m.Add}
}

func (h *MetricHandler) RegisterFloat64UpDownCounter(namespace, name string, m metric.Float64UpDownCounter) {
	h.metrics[metricName{namespace, name}] = metricFunc{nil, m.Add}
}
func (h *MetricHandler) RegisterFloat64ValueRecorder(namespace, name string, m metric.Float64ValueRecorder) {
	h.metrics[metricName{namespace, name}] = metricFunc{nil, m.Record}
}

func (m *MetricHandler) Metric(ctx context.Context, e *event.Event) {
	// Message is namespace. Last label is name and value.
	n := len(e.Labels)
	mf, found := m.metrics[metricName{e.Message, e.Labels[n-1].Name}]
	if !found {
		// Ignore, no error.
		return
	}
	value := e.Labels[n-1].Value
	attrs := labelsToAttributes(e.Labels[:n-1])
	if mf.intFunc != nil {
		mf.intFunc(ctx, value.Int64(), attrs...)
	} else {
		mf.floatFunc(ctx, value.Float64(), attrs...)
	}
}

func labelsToAttributes(ls []event.Label) []attribute.KeyValue {
	var attrs []attribute.KeyValue
	for _, l := range ls {
		attrs = append(attrs, labelToAttribute(l))
	}
	return attrs
}

func labelToAttribute(l event.Label) attribute.KeyValue {
	switch {
	case l.Value.IsString():
		return attribute.String(l.Name, l.Value.String())
	case l.Value.IsInt64():
		return attribute.Int64(l.Name, l.Value.Int64())
	case l.Value.IsFloat64():
		return attribute.Float64(l.Name, l.Value.Float64())
	case l.Value.IsBool():
		return attribute.Bool(l.Name, l.Value.Bool())
	default: // including uint64
		return attribute.Any(l.Name, l.Value.Interface())
	}
}
