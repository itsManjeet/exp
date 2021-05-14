// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package event

import (
	"context"
	"time"
)

// A Metric represents a kind of recorded measurement.
type Metric interface {
	Name() string
	Namespace() string
	Description() string
}

type MetricOptions struct {
	// A string that should be common for all metrics of an application or
	// service. Defaults to the import path of the package calling
	// the metric construction function (NewCounter, etc.).
	Namespace string

	// Optional description of the metric.
	Description string

	// TODO: deal with units. Follow otel, or define Go types for common units.
	// We don't need a time unit because we'll use time.Duration, and the only
	// other unit otel currently defines (besides dimensionless) is bytes.
}

type metricCommon struct {
	name string
	opts MetricOptions
}

func newMetricCommon(name string, opts *MetricOptions) *metricCommon {
	c := &metricCommon{name: name}
	if opts != nil {
		c.opts = *opts
	}
	if c.opts.Namespace == "" {
		c.opts.Namespace = scanStack().Space
	}
	return c
}

func (c *metricCommon) Name() string {
	return c.name
}

func (c *metricCommon) Namespace() string {
	return c.opts.Namespace
}

func (c *metricCommon) Description() string {
	return c.opts.Description
}

// A Counter is a metric that counts something cumulatively.
type Counter struct {
	*metricCommon
}

// NewCounter creates a counter with the given name.
func NewCounter(name string, opts *MetricOptions) *Counter {
	return &Counter{newMetricCommon(name, opts)}
}

// Record delivers a metric event with the given metric, value and labels to the
// exporter in the context.
func (c *Counter) Record(ctx context.Context, v int64, labels ...Label) {
	ev := New(ctx, MetricKind)
	if ev != nil {
		record(ev, c, Int64(string(MetricVal), v))
		ev.Labels = append(ev.Labels, labels...)
		ev.Deliver()
	}
}

// A FloatGauge records a single floating-point value that may go up or down.
// TODO(generics): Gauge[T]
type FloatGauge struct {
	*metricCommon
}

// NewFloatGauge creates a new FloatGauge with the given name.
func NewFloatGauge(name string, opts *MetricOptions) *FloatGauge {
	return &FloatGauge{newMetricCommon(name, opts)}
}

// Record converts its argument into a Value and returns a MetricValue with the
// receiver and the value.
func (g *FloatGauge) Record(ctx context.Context, v float64, labels ...Label) {
	ev := New(ctx, MetricKind)
	if ev != nil {
		record(ev, g, Float64(string(MetricVal), v))
		ev.Labels = append(ev.Labels, labels...)
		ev.Deliver()
	}
}

// A DurationDistribution records a distribution of durations.
// TODO(generics): Distribution[T]
type DurationDistribution struct {
	*metricCommon
}

// NewDuration creates a new Duration with the given name.
func NewDuration(name string, opts *MetricOptions) *DurationDistribution {
	return &DurationDistribution{newMetricCommon(name, opts)}
}

// Record converts its argument into a Value and returns a MetricValue with the
// receiver and the value.
func (d *DurationDistribution) Record(ctx context.Context, v time.Duration, labels ...Label) {
	ev := New(ctx, MetricKind)
	if ev != nil {
		record(ev, d, Duration(string(MetricVal), v))
		ev.Labels = append(ev.Labels, labels...)
		ev.Deliver()
	}
}

// An IntDistribution records a distribution of int64s.
type IntDistribution struct {
	*metricCommon
}

// NewIntDistribution creates a new IntDistribution with the given name.
func NewIntDistribution(name string, opts *MetricOptions) *IntDistribution {
	return &IntDistribution{newMetricCommon(name, opts)}
}

// Record converts its argument into a Value and returns a MetricValue with the
// receiver and the value.
func (d *IntDistribution) Record(ctx context.Context, v int64, labels ...Label) {
	ev := New(ctx, MetricKind)
	if ev != nil {
		record(ev, d, Int64(string(MetricVal), v))
		ev.Labels = append(ev.Labels, labels...)
		ev.Deliver()
	}
}

func record(ev *Event, m Metric, l Label) {
	ev.Labels = append(ev.Labels, l, MetricKey.Of(m))
}
