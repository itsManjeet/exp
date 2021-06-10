// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !disable_events

package event_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"golang.org/x/exp/event"
	"golang.org/x/exp/event/adapter/logfmt"
	"golang.org/x/exp/event/eventtest"
	"golang.org/x/exp/event/keys"
)

var (
	l1      = keys.Int("l1").Of(1)
	l2      = keys.Int("l2").Of(2)
	l3      = keys.Int("l3").Of(3)
	counter = event.NewCounter("hits", "cache hits")
	gauge   = event.NewFloatGauge("temperature", "CPU board temperature in Celsius")
	latency = event.NewDuration("latency", "how long it took")
)

func TestPrint(t *testing.T) {
	ctx := context.Background()
	for _, test := range []struct {
		name   string
		events func(context.Context)
		expect string
	}{{
		name:   "simple",
		events: func(ctx context.Context) { event.Log(ctx, "a message") },
		expect: `time="2020/03/05 14:27:48" msg="a message"
`}, {
		name:   "log 1",
		events: func(ctx context.Context) { event.LogB(ctx, "a message").Label(l1).Send() },
		expect: `time="2020/03/05 14:27:48" l1=1 msg="a message"`,
	}, {
		name:   "log 2",
		events: func(ctx context.Context) { event.LogB(ctx, "a message").Label(l1).Label(l2).Send() },
		expect: `time="2020/03/05 14:27:48" l1=1 l2=2 msg="a message"`,
	}, {
		name:   "log 3",
		events: func(ctx context.Context) { event.LogB(ctx, "a message").Label(l1).Label(l2).Label(l3).Send() },
		expect: `time="2020/03/05 14:27:48" l1=1 l2=2 l3=3 msg="a message"`,
	}, {
		name: "span",
		events: func(ctx context.Context) {
			ctx, eb := event.Start(ctx, "span")
			eb.Send()
		},
		expect: `
time="2020/03/05 14:27:48" trace=1 name=span
time="2020/03/05 14:27:49" parent=1 name=span end
`}, {
		name: "span nested",
		events: func(ctx context.Context) {
			ctx, eb := event.Start(ctx, "parent")
			defer eb.Send()
			child, eb2 := event.Start(ctx, "child")
			defer eb2.Send()
			event.Log(child, "message")
		},
		expect: `
time="2020/03/05 14:27:48" trace=1 name=parent
time="2020/03/05 14:27:49" parent=1 trace=2 name=child
time="2020/03/05 14:27:50" parent=2 msg=message
time="2020/03/05 14:27:51" parent=2 name=child end
time="2020/03/05 14:27:52" parent=1 name=parent end
`}, {
		name:   "counter",
		events: func(ctx context.Context) { counter.RecordB(ctx, 2).Label(l1).Send() },
		expect: `time="2020/03/05 14:27:48" metricValue=2 metric=Metric("golang.org/x/exp/event_test/hits") l1=1`,
	}, {
		name:   "gauge",
		events: func(ctx context.Context) { gauge.RecordB(ctx, 98.6).Label(l1).Send() },
		expect: `time="2020/03/05 14:27:48" metricValue=98.6 metric=Metric("golang.org/x/exp/event_test/temperature") l1=1`,
	}, {
		name: "duration",
		events: func(ctx context.Context) {
			latency.RecordB(ctx, 3*time.Second).Label(l1).Label(l2).Send()
		},
		expect: `time="2020/03/05 14:27:48" metricValue=3s metric=Metric("golang.org/x/exp/event_test/latency") l1=1 l2=2`,
	}, {
		name:   "annotate",
		events: func(ctx context.Context) { event.Annotate(ctx, l1) },
		expect: `time="2020/03/05 14:27:48" l1=1`,
	}, {
		name:   "annotate 2",
		events: func(ctx context.Context) { event.To(ctx).Label(l1).Label(l2).Send() },
		expect: `time="2020/03/05 14:27:48" l1=1 l2=2`,
	}, {
		name: "multiple events",
		events: func(ctx context.Context) {
			t := event.To(ctx)
			p := event.Prototype{}.As(event.LogKind)
			t.With(p).Int("myInt", 6).Message("my event").Send()
			t.With(p).String("myString", "some string value").Message("string event").Send()
		},
		expect: `
time="2020/03/05 14:27:48" myInt=6 msg="my event"
time="2020/03/05 14:27:49" myString="some string value" msg="string event"
`}} {
		buf := &strings.Builder{}
		ctx := event.WithExporter(ctx, event.NewExporter(logfmt.NewHandler(buf), eventtest.ExporterOptions()))
		test.events(ctx)
		got := strings.TrimSpace(buf.String())
		expect := strings.TrimSpace(test.expect)
		if got != expect {
			t.Errorf("%s failed\ngot   : %s\nexpect: %s", test.name, got, expect)
		}
	}
}

func ExampleLog() {
	ctx := event.WithExporter(context.Background(), event.NewExporter(logfmt.NewHandler(os.Stdout), eventtest.ExporterOptions()))
	event.LogB(ctx, "my event").Label(keys.Int("myInt").Of(6)).Send()
	event.LogB(ctx, "error event").String("myString", "some string value").Send()
	// Output:
	// time="2020/03/05 14:27:48" myInt=6 msg="my event"
	// time="2020/03/05 14:27:49" myString="some string value" msg="error event"
}

func TestLogEventf(t *testing.T) {
	eventtest.TestBenchmark(t, eventPrint, eventLogf, eventtest.LogfOutput)
}

func TestLogEvent(t *testing.T) {
	eventtest.TestBenchmark(t, eventPrint, eventLog, eventtest.LogfmtOutput)
}
