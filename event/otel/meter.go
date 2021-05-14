// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package otel

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"golang.org/x/exp/event"
)

// MetricHandler is an event.Handler for OpenTelemetry metrics.
// To use, first create with NewMetricHandler. Then register the metrics
// of interest with the RegisterXXX methods.
type MetricHandler struct {
	delegate      event.Handler // for non-Metric methods
	newRecordFunc NewRecordFunc
	mu            sync.Mutex
	recordFuncs   map[[2]string]RecordFunc
}

var _ event.Handler = (*MetricHandler)(nil)

type NewRecordFunc func(event.Metric) RecordFunc

type RecordFunc func(context.Context, event.Value, []attribute.KeyValue)

// NewMetricHandler creates a new MetricHandler.
// It implements the Metric method itself, and delegates
// the other methods to h.
func NewMetricHandler(h event.Handler, newRecordFunc NewRecordFunc) *MetricHandler {
	return &MetricHandler{
		delegate:      h,
		newRecordFunc: newRecordFunc,
		recordFuncs:   map[[2]string]RecordFunc{},
	}
}

func (m *MetricHandler) Log(c context.Context, e *event.Event)      { m.delegate.Log(c, e) }
func (m *MetricHandler) Annotate(c context.Context, e *event.Event) { m.delegate.Annotate(c, e) }
func (m *MetricHandler) Start(c context.Context, e *event.Event) context.Context {
	return m.delegate.Start(c, e)
}
func (m *MetricHandler) End(c context.Context, e *event.Event) { m.delegate.End(c, e) }

func (m *MetricHandler) Metric(ctx context.Context, e *event.Event) {
	// Get the otel instrument corresponding to the event's MetricDescriptor,
	// or create a new one.
	mi, _ := event.MetricKey.Find(e)
	em := mi.(event.Metric)
	desc := em.Descriptor()
	var rf RecordFunc
	key := [2]string{desc.Namespace(), desc.Name()}
	m.mu.Lock()
	rf = m.recordFuncs[key]
	if rf == nil {
		rf = m.newRecordFunc(em)
		m.recordFuncs[key] = rf
	}
	m.mu.Unlock()

	val, _ := event.MetricVal.Find(e)
	rf(ctx, val, labelsToAttributes(e.Labels))
}

func StandardNewRecordFunc(meter metric.MeterMust, m event.Metric) RecordFunc {
	desc := m.Descriptor()
	name := desc.Namespace() + "/" + desc.Name()
	switch m.(type) {
	case *event.Counter:
		c := meter.NewInt64Counter(name)
		return func(ctx context.Context, v event.Value, attrs []attribute.KeyValue) {
			c.Add(ctx, int64(v.Uint64()), attrs...)
		}

	case *event.FloatGauge:
		g := meter.NewFloat64UpDownCounter(name)
		return func(ctx context.Context, v event.Value, attrs []attribute.KeyValue) {
			g.Add(ctx, v.Float64(), attrs...)
		}

	case *event.Duration:
		r := meter.NewInt64ValueRecorder(name)
		return func(ctx context.Context, v event.Value, attrs []attribute.KeyValue) {
			r.Record(ctx, v.Duration().Nanoseconds(), attrs...)
		}

	default:
		return nil
	}
}

func labelsToAttributes(ls []event.Label) []attribute.KeyValue {
	var attrs []attribute.KeyValue
	for _, l := range ls {
		if l.Name == string(event.MetricKey) || l.Name == string(event.MetricVal) {
			continue
		}
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
