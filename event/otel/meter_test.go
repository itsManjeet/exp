// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package otel

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/otel/exporters/stdout"
	"go.opentelemetry.io/otel/metric"
	"golang.org/x/exp/event"
)

func TestMeter(t *testing.T) {
	ctx := context.Background()
	traceProvider, controller, err := stdout.InstallNewPipeline(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer traceProvider.Shutdown(ctx)
	defer controller.Stop(ctx)

	meter := metric.Must(controller.MeterProvider().Meter("demo"))
	hits := meter.NewInt64Counter("hits")

	h := NewMetricHandler()
	h.RegisterInt64Counter("test", "hits", hits)

	ctx = event.WithExporter(ctx, event.NewExporter(h))
	event.To(ctx).IntMetric("test", "hits", 3)
	event.To(ctx).IntMetric("test", "hits", 5)
	time.Sleep(10 * time.Second)
	// Wait until stdout collector triggers.
	// Observe output on stdout.
	// TODO: fix.
}
