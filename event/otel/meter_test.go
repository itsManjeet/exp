// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package otel_test

import (
	"bytes"
	"context"
	"encoding/json"
	"sort"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"go.opentelemetry.io/otel/exporters/stdout"
	"go.opentelemetry.io/otel/metric"
	"golang.org/x/exp/event"
	"golang.org/x/exp/event/otel"
)

func TestMeter(t *testing.T) {
	ctx := context.Background()
	var buf bytes.Buffer
	traceProvider, controller, err := stdout.InstallNewPipeline([]stdout.Option{
		stdout.WithPrettyPrint(),
		stdout.WithWriter(&buf),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	traceProvider.Shutdown(ctx) // Not used.

	meter := metric.Must(controller.MeterProvider().Meter("demo"))
	newRecordFunc := func(m event.Metric) otel.RecordFunc {
		return otel.StandardNewRecordFunc(meter, m)
	}
	h := otel.NewMetricHandler(event.NopHandler{}, newRecordFunc)
	ctx = event.WithExporter(ctx, event.NewExporter(h, nil))
	recordMetrics(ctx)

	controller.Stop(ctx) // Flushes output to buf.

	var got []otelMetricRecord
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	sort.Slice(got, func(i, j int) bool { return got[i].Name < got[j].Name })
	want := []otelMetricRecord{
		{
			Name: "golang.org/x/exp/event/otel_test/hits{service.name=unknown_service:otel.test,telemetry.sdk.language=go,telemetry.sdk.name=opentelemetry,telemetry.sdk.version=0.20.0,instrumentation.name=demo}",
			Sum:  8,
		},
		{
			Name:  "golang.org/x/exp/event/otel_test/latency{service.name=unknown_service:otel.test,telemetry.sdk.language=go,telemetry.sdk.name=opentelemetry,telemetry.sdk.version=0.20.0,instrumentation.name=demo}",
			Min:   5e6,
			Max:   10e6,
			Sum:   15e6,
			Count: 2,
		},
		{
			Name: "golang.org/x/exp/event/otel_test/temp{service.name=unknown_service:otel.test,telemetry.sdk.language=go,telemetry.sdk.name=opentelemetry,telemetry.sdk.version=0.20.0,instrumentation.name=demo}",
			Sum:  -20,
		},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("mismatch (-want, +got):\n%s", diff)
	}
}

type otelMetricRecord struct {
	Name          string
	Min, Max, Sum float64
	Count         int
}

func recordMetrics(ctx context.Context) {
	c := event.NewCounter("hits", "")
	g := event.NewFloatGauge("temp", "")
	d := event.NewDuration("latency", "")

	event.To(ctx).Metric(c.Record(8))
	event.To(ctx).Metric(g.Record(-20))
	event.To(ctx).Metric(d.Record(5 * time.Millisecond))
	event.To(ctx).Metric(d.Record(10 * time.Millisecond))
}
