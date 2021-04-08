// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package event_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"golang.org/x/exp/event"
	"golang.org/x/exp/event/eventtest"
	"golang.org/x/exp/event/keys"
)

var (
	l1 = keys.NewInt("l1", "l1").Of(1)
	l2 = keys.NewInt("l2", "l2").Of(2)
	l3 = keys.NewInt("l3", "l3").Of(3)
)

type captureHandler struct {
	printer event.Printer
	buf     strings.Builder
}

func (e *captureHandler) Handle(ev *event.Event) {
	e.printer.Handle(ev)
}

func TestPrint(t *testing.T) {
	ctx := context.Background()
	for _, test := range []struct {
		name   string
		events func(context.Context)
		expect string
	}{{
		name:   "simple",
		events: func(ctx context.Context) { event.To(ctx).Log("a message") },
		expect: `
2020/03/05 14:27:48 [log:1] a message
`}, {
		name:   "log 1",
		events: func(ctx context.Context) { event.To(ctx).With(l1).Log("a message") },
		expect: `
2020/03/05 14:27:48 [log:1] a message
	l1=1
`}, {
		name:   "simple",
		events: func(ctx context.Context) { event.To(ctx).With(l1).With(l2).Log("a message") },
		expect: `
2020/03/05 14:27:48 [log:1] a message
	l1=1
	l2=2
`}, {
		name:   "simple",
		events: func(ctx context.Context) { event.To(ctx).With(l1).With(l2).With(l3).Log("a message") },
		expect: `
2020/03/05 14:27:48 [log:1] a message
	l1=1
	l2=2
	l3=3
`}, {
		name: "span",
		events: func(ctx context.Context) {
			ctx = event.Start(ctx, "span")
			event.To(ctx).End()
		},
		expect: `
2020/03/05 14:27:48 [start:1] span
2020/03/05 14:27:49 [end:2:1]
`}, {
		name: "span nested",
		events: func(ctx context.Context) {
			ctx = event.Start(ctx, "parent")
			defer func() { event.To(ctx).End() }()
			child := event.Start(ctx, "child")
			defer func() { event.To(child).End() }()
			event.To(child).Log("message")
		},
		expect: `
2020/03/05 14:27:48 [start:1] parent
2020/03/05 14:27:49 [start:2:1] child
2020/03/05 14:27:50 [log:3:2] message
2020/03/05 14:27:51 [end:4:2]
2020/03/05 14:27:52 [end:5:1]
`}, {
		name:   "metric",
		events: func(ctx context.Context) { event.To(ctx).With(l1).Metric() },
		expect: `
2020/03/05 14:27:48 [metric:1]
	l1=1
`}, {
		name:   "metric 2",
		events: func(ctx context.Context) { event.To(ctx).With(l1).With(l2).Metric() },
		expect: `
2020/03/05 14:27:48 [metric:1]
	l1=1
	l2=2
`}, {
		name:   "annotate",
		events: func(ctx context.Context) { event.To(ctx).With(l1).Annotate() },
		expect: `
2020/03/05 14:27:48 [annotate:1]
	l1=1
`}, {
		name:   "annotate 2",
		events: func(ctx context.Context) { event.To(ctx).With(l1).With(l2).Annotate() },
		expect: `
2020/03/05 14:27:48 [annotate:1]
	l1=1
	l2=2
`}, {
		name: "multiple events",
		events: func(ctx context.Context) {
			anInt := keys.NewInt("myInt", "an integer")
			aString := keys.NewString("myString", "a string")
			b := event.To(ctx)
			b.Clone().With(anInt.Of(6)).Log("my event")
			b.With(aString.Of("some string value")).Log("string event")
		},
		expect: `
2020/03/05 14:27:48 [log:1] my event
	myInt=6
2020/03/05 14:27:49 [log:2] string event
	myString="some string value"
`}} {
		e := &captureHandler{}
		e.printer = event.NewPrinter(&e.buf)
		ctx := event.WithExporter(ctx, event.NewExporter(eventtest.Rewriter(e)))
		test.events(ctx)
		got := strings.TrimSpace(e.buf.String())
		expect := strings.TrimSpace(test.expect)
		if got != expect {
			t.Errorf("%s failed\ngot   : %q\nexpect: %q", test.name, got, expect)
		}
	}
}

func ExampleLog() {
	ctx := event.WithExporter(context.Background(),
		event.NewExporter(eventtest.Rewriter(event.NewPrinter(os.Stdout))))
	anInt := keys.NewInt("myInt", "an integer")
	aString := keys.NewString("myString", "a string")
	event.To(ctx).With(anInt.Of(6)).Log("my event")
	event.To(ctx).With(aString.Of("some string value")).Log("error event")
	// Output:
	// 2020/03/05 14:27:48 [log:1] my event
	// 	myInt=6
	// 2020/03/05 14:27:49 [log:2] error event
	// 	myString="some string value"
}
