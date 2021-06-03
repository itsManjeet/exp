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
	"golang.org/x/exp/event/adapter/eventtest"
	"golang.org/x/exp/event/adapter/logfmt"
	"golang.org/x/exp/event/bench"
	"golang.org/x/exp/event/keys"
)

var (
	l1      = keys.Int("l1").Of(1)
	l2      = keys.Int("l2").Of(2)
	l3      = keys.Int("l3").Of(3)
	counter = event.NewCounter("hits")
	gauge   = event.NewFloatGauge("temperature")
	latency = event.NewDuration("latency")
)

func TestPrint(t *testing.T) {
	ctx := context.Background()
	for _, test := range []struct {
		name   string
		events func(context.Context)
		expect string
	}{{
		name:   "simple",
		events: func(ctx context.Context) { event.To(ctx).Log("a message") },
		expect: `time=2020-03-05T14:27:48 msg="a message"
`}, {
		name:   "log 1",
		events: func(ctx context.Context) { event.To(ctx).With(l1).Log("a message") },
		expect: `time=2020-03-05T14:27:48 l1=1 msg="a message"`,
	}, {
		name:   "log 2",
		events: func(ctx context.Context) { event.To(ctx).With(l1).With(l2).Log("a message") },
		expect: `time=2020-03-05T14:27:48 l1=1 l2=2 msg="a message"`,
	}, {
		name:   "log 3",
		events: func(ctx context.Context) { event.To(ctx).With(l1).With(l2).With(l3).Log("a message") },
		expect: `time=2020-03-05T14:27:48 l1=1 l2=2 l3=3 msg="a message"`,
	}, {
		name: "span",
		events: func(ctx context.Context) {
			ctx, end := event.To(ctx).Start("span")
			end()
		},
		expect: `
time=2020-03-05T14:27:48 trace=1 name=span
time=2020-03-05T14:27:49 parent=1 end
`}, {
		name: "span nested",
		events: func(ctx context.Context) {
			ctx, end := event.To(ctx).Start("parent")
			defer end()
			child, end2 := event.To(ctx).Start("child")
			defer end2()
			event.To(child).Log("message")
		},
		expect: `
time=2020-03-05T14:27:48 trace=1 name=parent
time=2020-03-05T14:27:49 parent=1 trace=2 name=child
time=2020-03-05T14:27:50 parent=2 msg=message
time=2020-03-05T14:27:51 parent=2 end
time=2020-03-05T14:27:52 parent=1 end
`}, {
		name:   "counter",
		events: func(ctx context.Context) { event.To(ctx).With(l1).Metric(counter.Record(2)) },
		expect: `time=2020-03-05T14:27:48 l1=1 metricValue=2 metric=Metric("golang.org/x/exp/event_test/hits")`,
	}, {
		name:   "gauge",
		events: func(ctx context.Context) { event.To(ctx).With(l1).Metric(gauge.Record(98.6)) },
		expect: `time=2020-03-05T14:27:48 l1=1 metricValue=98.6 metric=Metric("golang.org/x/exp/event_test/temperature")`,
	}, {
		name: "duration",
		events: func(ctx context.Context) {
			event.To(ctx).With(l1).With(l2).Metric(latency.Record(3 * time.Second))
		},
		expect: `time=2020-03-05T14:27:48 l1=1 l2=2 metricValue=3s metric=Metric("golang.org/x/exp/event_test/latency")`,
	}, {
		name:   "annotate",
		events: func(ctx context.Context) { event.To(ctx).With(l1).Annotate() },
		expect: `time=2020-03-05T14:27:48 l1=1`,
	}, {
		name:   "annotate 2",
		events: func(ctx context.Context) { event.To(ctx).With(l1).With(l2).Annotate() },
		expect: `time=2020-03-05T14:27:48 l1=1 l2=2`,
	}, {
		name: "multiple events",
		events: func(ctx context.Context) {
			b := event.To(ctx)
			b.Clone().With(keys.Int("myInt").Of(6)).Log("my event")
			b.With(keys.String("myString").Of("some string value")).Log("string event")
		},
		expect: `
time=2020-03-05T14:27:48 myInt=6 msg="my event"
time=2020-03-05T14:27:49 myString="some string value" msg="string event"
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
	event.To(ctx).With(keys.Int("myInt").Of(6)).Log("my event")
	event.To(ctx).With(keys.String("myString").Of("some string value")).Log("error event")
	// Output:
	// time=2020-03-05T14:27:48 myInt=6 msg="my event"
	// time=2020-03-05T14:27:49 myString="some string value" msg="error event"
}

func TestLogEventf(t *testing.T) {
	bench.TestBenchmark(t, eventPrint, eventLogf, `
time=2020-03-05T14:27:48 msg="a where A=0"
time=2020-03-05T14:27:49 msg="b where B=\"A value\""
time=2020-03-05T14:27:50 msg="a where A=1"
time=2020-03-05T14:27:51 msg="b where B=\"Some other value\""
time=2020-03-05T14:27:52 msg="a where A=22"
time=2020-03-05T14:27:53 msg="b where B=\"Some other value\""
time=2020-03-05T14:27:54 msg="a where A=333"
time=2020-03-05T14:27:55 msg="b where B=\"\""
time=2020-03-05T14:27:56 msg="a where A=4444"
time=2020-03-05T14:27:57 msg="b where B=\"prime count of values\""
time=2020-03-05T14:27:58 msg="a where A=55555"
time=2020-03-05T14:27:59 msg="b where B=\"V\""
time=2020-03-05T14:28:00 msg="a where A=666666"
time=2020-03-05T14:28:01 msg="b where B=\"A value\""
time=2020-03-05T14:28:02 msg="a where A=7777777"
time=2020-03-05T14:28:03 msg="b where B=\"A value\""
`)
}

func TestLogEvent(t *testing.T) {
	bench.TestBenchmark(t, eventPrint, eventLog, `
time=2020-03-05T14:27:48 A=0 msg=a
time=2020-03-05T14:27:49 B="A value" msg=b
time=2020-03-05T14:27:50 A=1 msg=a
time=2020-03-05T14:27:51 B="Some other value" msg=b
time=2020-03-05T14:27:52 A=22 msg=a
time=2020-03-05T14:27:53 B="Some other value" msg=b
time=2020-03-05T14:27:54 A=333 msg=a
time=2020-03-05T14:27:55 B="" msg=b
time=2020-03-05T14:27:56 A=4444 msg=a
time=2020-03-05T14:27:57 B="prime count of values" msg=b
time=2020-03-05T14:27:58 A=55555 msg=a
time=2020-03-05T14:27:59 B=V msg=b
time=2020-03-05T14:28:00 A=666666 msg=a
time=2020-03-05T14:28:01 B="A value" msg=b
time=2020-03-05T14:28:02 A=7777777 msg=a
time=2020-03-05T14:28:03 B="A value" msg=b
`)
}
