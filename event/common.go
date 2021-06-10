// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package event

import (
	"context"
)

const (
	MetricKey      = interfaceKey("metric")
	MetricVal      = valueKey("metricValue")
	DurationMetric = interfaceKey("durationMetric")
)

type Kind int

const (
	unknownKind = Kind(iota)

	LogKind
	MetricKind
	TraceKind
)

type (
	valueKey     string
	interfaceKey string
)

//TODO: we could add labels ...Label and it would be free if not used...
func Log(ctx context.Context, msg string) {
	To(ctx).As(LogKind).Message(msg).Send()
}

func LogB(ctx context.Context, msg string) Builder {
	return To(ctx).As(LogKind).Message(msg)
}

func LogWith(ctx context.Context, example Prototype, msg string) {
	To(ctx).With(example).As(LogKind).Message(msg).Send()
}

func Annotate(ctx context.Context, label Label) {
	To(ctx).As(LogKind).Label(label).Send()
}

func Start(ctx context.Context, name string) (context.Context, Builder) {
	startB := To(ctx).As(TraceKind)
	if !startB.Active() {
		return ctx, Builder{}
	}
	startB.data.Event.Name = name
	endB := startB.start()
	return endB.ctx, endB
}

func (k valueKey) Of(v Value) Label {
	return Label{Name: string(k), Value: v}
}

func (k valueKey) Matches(ev *Event) bool {
	_, found := k.Find(ev)
	return found
}

func (k valueKey) Find(ev *Event) (Value, bool) {
	return lookupValue(string(k), ev.Labels)
}

func (k interfaceKey) Of(v interface{}) Label {
	return Label{Name: string(k), Value: ValueOf(v)}
}

func (k interfaceKey) Matches(ev *Event) bool {
	_, found := k.Find(ev)
	return found
}

func (k interfaceKey) Find(ev *Event) (interface{}, bool) {
	v, ok := lookupValue(string(k), ev.Labels)
	if !ok {
		return nil, false
	}
	return v.Interface(), true

}

func lookupValue(name string, labels []Label) (Value, bool) {
	for i := len(labels) - 1; i >= 0; i-- {
		if labels[i].Name == name {
			return labels[i].Value, true
		}
	}
	return Value{}, false
}
