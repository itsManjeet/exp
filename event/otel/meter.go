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

// type meterProvider struct {
// }

// func NewMeterProvider() metric.MeterProvider {
// 	return &meterProvider{}
// }

// func (mp *meterProvider) Meter(instrumentationName string, opts ...metric.MeterOption) metric.Meter {
// 	return metric.WrapMeterImpl(&meterImpl{}, instrumentationName, opts...)
// }

// type meterImpl struct{}

// // RecordBatch atomically records a batch of measurements.
// func (m *meterImpl) RecordBatch(ctx context.Context, labels []attribute.KeyValue, measurement ...Measurement) {

// }

// 	// NewSyncInstrument returns a newly constructed
// 	// synchronous instrument implementation or an error, should
// 	// one occur.
// 	NewSyncInstrument(descriptor Descriptor) (SyncImpl, error)

// 	// NewAsyncInstrument returns a newly constructed
// 	// asynchronous instrument implementation or an error, should
// 	// one occur.
// 	NewAsyncInstrument(
// 		descriptor Descriptor,
// 		runner AsyncRunner,
// 	) (AsyncImpl, error)
// }

type MetricHandler struct {
	metrics map[string]metricFunc // from "namespace name" to otel metric Add/Record function
}

var _ event.MetricHandler = (*MetricHandler)(nil)

type metricFunc struct {
	// exactly one of:
	intFunc   func(context.Context, int64, ...attribute.KeyValue)
	floatFunc func(context.Context, float64, ...attribute.KeyValue)
}

func NewMetricHandler() *MetricHandler {
	return &MetricHandler{
		metrics: map[string]metricFunc{}, // there is no common type for otel metrics
	}
}

func (h *MetricHandler) RegisterInt64Counter(namespace, name string, m metric.Int64Counter) {
	h.registerMetricFunc(namespace, name, metricFunc{m.Add, nil})
}

func (h *MetricHandler) RegisterInt64UpDownCounter(namespace, name string, m metric.Int64UpDownCounter) {
	h.registerMetricFunc(namespace, name, metricFunc{m.Add, nil})
}
func (h *MetricHandler) RegisterInt64ValueRecorder(namespace, name string, m metric.Int64ValueRecorder) {
	h.registerMetricFunc(namespace, name, metricFunc{m.Record, nil})
}

func (h *MetricHandler) RegisterFloat64Counter(namespace, name string, m metric.Float64Counter) {
	h.registerMetricFunc(namespace, name, metricFunc{nil, m.Add})
}

func (h *MetricHandler) RegisterFloat64UpDownCounter(namespace, name string, m metric.Float64UpDownCounter) {
	h.registerMetricFunc(namespace, name, metricFunc{nil, m.Add})
}
func (h *MetricHandler) RegisterFloat64ValueRecorder(namespace, name string, m metric.Float64ValueRecorder) {
	h.registerMetricFunc(namespace, name, metricFunc{nil, m.Record})
}

func (h *MetricHandler) registerMetricFunc(namespace, name string, mf metricFunc) {
	h.metrics[namespace+" "+name] = mf
}

// func metricFuncFromMetric(m interface{}) metricFunc {
// 	switch m := m.(type) {
// 	case metric.Int64Counter:
// 		return metricFunc{m.Add, nil}
// 	case metric.Int64UpDownCounter:
// 		return metricFunc{m.Add, nil}
// 	case metric.Int64ValueRecorder:
// 		return metricFunc{m.Record, nil}
// 	case metric.Float64Counter:
// 		return metricFunc{nil, m.Add}
// 	case metric.Float64UpDownCounter:
// 		return metricFunc{nil, m.Add}
// 	case metric.Float64ValueRecorder:
// 		return metricFunc{nil, m.Record}
// 	default:
// 		return metricFunc{}
// 	}
// }

func (m *MetricHandler) Metric(ctx context.Context, e *event.Event) {
	// Last three labels are namespace, name and value.
	n := len(e.Labels)
	namespace := e.Labels[n-3].Value.String()
	name := e.Labels[n-2].Value.String()
	mf, ok := m.metrics[namespace+" "+name]
	if !ok {
		// Ignore, no error.
		return
	}
	value := e.Labels[n-1].Value
	attrs := labelsToAttributes(e.Labels[:n-3])
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
