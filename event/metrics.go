// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package event

// MetricKind represents the kind of a Metric.
type MetricKind int

const (
	// A Counter is a metric that always increases, usually by 1.
	Counter MetricKind = iota
	// A Gauge is a metric that may go up or down.
	Gauge
	// A Distribution is a metric for which a summary of values is tracked.
	Distribution
)

type Metric struct {
	kind            MetricKind
	namespace, name string
}

func NewMetric(kind MetricKind, namespace, name string) *Metric {
	return &Metric{
		kind:      kind,
		namespace: namespace,
		name:      name,
	}
}

func (m *Metric) Kind() MetricKind {
	return m.kind
}

func (m *Metric) Namespace() string {
	return m.namespace
}

func (m *Metric) Name() string {
	return m.name
}
