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
		events: func(ctx context.Context) { event.To(ctx).Log("a message") },
		expect: `time="2020/03/05 14:27:48" msg="a message"
`}, {
		name:   "log 1",
		events: func(ctx context.Context) { event.To(ctx).Log("a message", l1) },
		expect: `time="2020/03/05 14:27:48" l1=1 msg="a message"`,
	}, {
		name:   "log 2",
		events: func(ctx context.Context) { event.To(ctx).Log("a message", l1, l2) },
		expect: `time="2020/03/05 14:27:48" l1=1 l2=2 msg="a message"`,
	}, {
		name:   "log 3",
		events: func(ctx context.Context) { event.To(ctx).Log("a message", l1, l2, l3) },
		expect: `time="2020/03/05 14:27:48" l1=1 l2=2 l3=3 msg="a message"`,
	}, {
		name: "span",
		events: func(ctx context.Context) {
			ctx, eb := event.To(ctx).Start("span")
			eb.End()
		},
		expect: `
time="2020/03/05 14:27:48" trace=1 name=span
time="2020/03/05 14:27:49" parent=1 end
`}, {
		name: "span nested",
		events: func(ctx context.Context) {
			ctx, eb := event.To(ctx).Start("parent")
			defer eb.End()
			child, eb2 := event.To(ctx).Start("child")
			defer eb2.End()
			event.To(child).Log("message")
		},
		expect: `
time="2020/03/05 14:27:48" trace=1 name=parent
time="2020/03/05 14:27:49" parent=1 trace=2 name=child
time="2020/03/05 14:27:50" parent=2 msg=message
time="2020/03/05 14:27:51" parent=2 end
time="2020/03/05 14:27:52" parent=1 end
`}, {
		name:   "counter",
		events: func(ctx context.Context) { event.To(ctx).Metric(counter.Record(2), l1) },
		expect: `time="2020/03/05 14:27:48" in="golang.org/x/exp/event_test" l1=1 metricValue=2 metric="Metric(\"golang.org/x/exp/event_test/hits\")"`,
	}, {
		name:   "gauge",
		events: func(ctx context.Context) { event.To(ctx).Metric(gauge.Record(98.6), l1) },
		expect: `time="2020/03/05 14:27:48" in="golang.org/x/exp/event_test" l1=1 metricValue=98.6 metric="Metric(\"golang.org/x/exp/event_test/temperature\")"`,
	}, {
		name: "duration",
		events: func(ctx context.Context) {
			event.To(ctx).Metric(latency.Record(3*time.Second), l1, l2)
		},
		expect: `time="2020/03/05 14:27:48" in="golang.org/x/exp/event_test" l1=1 l2=2 metricValue=3s metric="Metric(\"golang.org/x/exp/event_test/latency\")"`,
	}, {
		name:   "annotate",
		events: func(ctx context.Context) { event.To(ctx).Annotate(l1) },
		expect: `time="2020/03/05 14:27:48" l1=1`,
	}, {
		name:   "annotate 2",
		events: func(ctx context.Context) { event.To(ctx).Annotate(l1, l2) },
		expect: `time="2020/03/05 14:27:48" l1=1 l2=2`,
	}, {
		name: "multiple events",
		events: func(ctx context.Context) {
			t := event.To(ctx)
			t.Log("my event", keys.Int("myInt").Of(6))
			t.Log("string event", keys.String("myString").Of("some string value"))
		},
		expect: `
time="2020/03/05 14:27:48" myInt=6 msg="my event"
time="2020/03/05 14:27:49" myString="some string value" msg="string event"
`}} {
		t.Run(test.name, func(t *testing.T) {
			buf := &strings.Builder{}
			ctx := event.WithExporter(ctx, event.NewExporter(logfmt.NewHandler(buf), eventtest.ExporterOptions()))
			test.events(ctx)
			got := strings.TrimSpace(buf.String())
			expect := strings.TrimSpace(test.expect)
			if got != expect {
				t.Errorf("\ngot:    %s\nexpect: %s", got, expect)
			}
		})
	}
}

func ExampleLog() {
	ctx := event.WithExporter(context.Background(), event.NewExporter(logfmt.NewHandler(os.Stdout), eventtest.ExporterOptions()))
	event.To(ctx).Log("my event", keys.Int("myInt").Of(6))
	event.To(ctx).Log("error event", keys.String("myString").Of("some string value"))
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
