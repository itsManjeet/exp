// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package event_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"golang.org/x/exp/event"
	"golang.org/x/exp/event/keys"
)

func TestPrint(t *testing.T) {
	ctx := context.Background()
	for _, test := range []struct {
		name   string
		events func(context.Context)
		expect string
	}{{
		name:   "simple",
		events: func(ctx context.Context) { event.To(ctx).Log("simple") },
		expect: `
2020/03/05 14:27:48 [1] simple
`}, {
		name:   "log 1",
		events: func(ctx context.Context) { event.To(ctx).With(l1).Log("log") },
		expect: `
2020/03/05 14:27:48 [1] log
	l1=1
`}, {
		name:   "simple",
		events: func(ctx context.Context) { event.To(ctx).With(l1).With(l2).Log("log") },
		expect: `
2020/03/05 14:27:48 [1] log
	l1=1
	l2=2
`}, {
		name:   "simple",
		events: func(ctx context.Context) { event.To(ctx).With(l1).With(l2).With(l3).Log("log") },
		expect: `
2020/03/05 14:27:48 [1] log
	l1=1
	l2=2
	l3=3
`}, {
		name: "span",
		events: func(ctx context.Context) {
			ctx = event.To(ctx).Start("span")
			event.To(ctx).End()
		},
		expect: `
2020/03/05 14:27:48 [1] start "span"
2020/03/05 14:27:48 [2:1] end
`}, {
		name:   "span 1",
		events: func(ctx context.Context) { event.To(ctx).With(l1).Start("span") },
		expect: `
2020/03/05 14:27:48 [1] start "span"
	l1=1
`}, {
		name:   "span 2",
		events: func(ctx context.Context) { event.To(ctx).With(l1).With(l2).Start("span") },
		expect: `
2020/03/05 14:27:48 [1] start "span"
	l1=1
	l2=2
`}, {
		name: "span nested",
		events: func(ctx context.Context) {
			ctx = event.To(ctx).Start("parent")
			defer func() { event.To(ctx).End() }()
			child := event.To(ctx).Start("child")
			defer func() { event.To(child).End() }()
			event.To(child).Log("message")
		},
		expect: `
2020/03/05 14:27:48 [1] start "parent"
2020/03/05 14:27:48 [2:1] start "child"
2020/03/05 14:27:48 [3:2] message
2020/03/05 14:27:48 [4:2] end
2020/03/05 14:27:48 [5:1] end
`}, {
		name:   "metric",
		events: func(ctx context.Context) { event.To(ctx).With(l1).Metric() },
		expect: `
2020/03/05 14:27:48 [1] metric
	l1=1
`}, {
		name:   "metric 2",
		events: func(ctx context.Context) { event.To(ctx).With(l1).With(l2).Metric() },
		expect: `
2020/03/05 14:27:48 [1] metric
	l1=1
	l2=2
`}, {
		name:   "annotate",
		events: func(ctx context.Context) { event.To(ctx).With(l1).Annotate() },
		expect: `
2020/03/05 14:27:48 [1] annotate
	l1=1
`}, {
		name:   "annotate 2",
		events: func(ctx context.Context) { event.To(ctx).With(l1).With(l2).Annotate() },
		expect: `
2020/03/05 14:27:48 [1] annotate
	l1=1
	l2=2
`}, {
		name: "multiple events",
		events: func(ctx context.Context) {
			anInt := keys.NewInt("myInt", "an integer")
			aString := keys.NewString("myString", "a string")
			event.To(ctx).With(anInt.Of(6)).Log("my event")
			event.To(ctx).Error(errors.New("an error")).With(aString.Of("some string value")).Log("error event")
		},
		expect: `
2020/03/05 14:27:48 [1] my event
	myInt=6
2020/03/05 14:27:48 [2] error event: an error
	myString="some string value"
`}} {
		exporter := exporter{
			ids: make(map[uint64]uint64),
		}
		exporter.printer = event.NewPrinter(&exporter.buf)
		ctx := event.WithExporter(ctx, &exporter)
		test.events(ctx)
		got := strings.TrimSpace(exporter.buf.String())
		expect := strings.TrimSpace(test.expect)
		if got != expect {
			t.Errorf("%s failed\ngot   : %q\nexpect: %q", test.name, got, expect)
		}
	}
}

var (
	l1 = keys.NewInt("l1", "l1").Of(1)
	l2 = keys.NewInt("l2", "l2").Of(2)
	l3 = keys.NewInt("l3", "l3").Of(3)
)

type exporter struct {
	buf     strings.Builder
	printer event.Printer
	nextID  uint64
	ids     map[uint64]uint64
}

func (e *exporter) Export(ev *event.Event) {
	// rewrite the time to normalize it
	copy := *ev
	copy.At, _ = time.Parse(time.RFC3339Nano, "2020-03-05T14:27:48Z")
	// remap the parent id if present
	if copy.Parent != 0 {
		copy.Parent = e.ids[copy.Parent]
	}
	// rewrite the id to be per exporter rather than per process
	e.nextID++
	e.ids[copy.ID] = e.nextID
	copy.ID = e.nextID
	// and print the event
	e.printer.Event(&copy)
}
