// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package otel

import (
	"bytes"
	"context"
	"encoding/json"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"go.opentelemetry.io/otel/exporters/stdout"
	"go.opentelemetry.io/otel/metric"
	"golang.org/x/exp/event"
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
	hits := meter.NewInt64Counter("hits")
	latency := meter.NewFloat64ValueRecorder("latency")

	h := NewMetricHandler()
	h.RegisterInt64Counter("test", "hits", hits)
	h.RegisterFloat64ValueRecorder("test", "latency", latency)

	ctx = event.WithExporter(ctx, event.NewExporter(h))
	event.To(ctx).IntMetric("test", "hits", 3)
	event.To(ctx).FloatMetric("test", "latency", 7.5)
	event.To(ctx).IntMetric("test", "unregistered", 2)
	event.To(ctx).FloatMetric("test", "latency", 1.5)
	event.To(ctx).IntMetric("test", "hits", 5)

	controller.Stop(ctx) // Flushes output to buf.
	var got []otelMetricRecord
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	sort.Slice(got, func(i, j int) bool { return got[i].Name < got[j].Name })
	want := []otelMetricRecord{
		{
			Name: "hits{service.name=unknown_service:otel.test,telemetry.sdk.language=go,telemetry.sdk.name=opentelemetry,telemetry.sdk.version=0.20.0,instrumentation.name=demo}",
			Sum:  8,
		},
		{
			Name:  "latency{service.name=unknown_service:otel.test,telemetry.sdk.language=go,telemetry.sdk.name=opentelemetry,telemetry.sdk.version=0.20.0,instrumentation.name=demo}",
			Min:   1.5,
			Max:   7.5,
			Sum:   9,
			Count: 2,
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
