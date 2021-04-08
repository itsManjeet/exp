// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ocagent_test

import (
	"context"
	"testing"

	"golang.org/x/exp/event"
)

func TestEncodeMetric(t *testing.T) {
	t.Skip("TODO: all things metrics")
	exporter := newTestExporter()
	const prefix = testNodeStr + `
	"metrics":[`
	const suffix = `]}`
	tests := []struct {
		name string
		run  func(ctx context.Context)
		want string
	}{
		{
			name: "HistogramFloat64, HistogramInt64",
			run: func(ctx context.Context) {
				//TODO:event.Label(ctx, keyMethod.Of("godoc.ServeHTTP"))
				event.Metric(ctx, latencyMs.Of(96.58))
				//TODO:event.Label(ctx, keys.Err.Of(errors.New("panic: fatal signal")))
				event.Metric(ctx, bytesIn.Of(97e2))
			},
			want: prefix + `
			{
				"metric_descriptor": {
					"name": "latency_ms",
					"description": "The latency of calls in milliseconds",
					"type": 6,
					"label_keys": [
						{
							"key": "method"
						},
						{
							"key": "route"
						}
					]
				},
				"timeseries": [
					{
						"start_timestamp": "1970-01-01T00:00:00Z",
						"points": [
							{
								"timestamp": "1970-01-01T00:00:40Z",
								"distributionValue": {
									"count": 1,
									"sum": 96.58,
									"bucket_options": {
										"explicit": {
											"bounds": [
												0,
												5,
												10,
												25,
												50
											]
										}
									},
									"buckets": [
										{},
										{},
										{},
										{},
										{}
									]
								}
							}
						]
					}
				]
			},
			{
				"metric_descriptor": {
					"name": "latency_ms",
					"description": "The latency of calls in milliseconds",
					"type": 6,
					"label_keys": [
						{
							"key": "method"
						},
						{
							"key": "route"
						}
					]
				},
				"timeseries": [
					{
						"start_timestamp": "1970-01-01T00:00:00Z",
						"points": [
							{
								"timestamp": "1970-01-01T00:00:40Z",
								"distributionValue": {
									"count": 1,
									"sum": 9700,
									"bucket_options": {
										"explicit": {
											"bounds": [
												0,
												10,
												50,
												100,
												500,
												1000,
												2000
											]
										}
									},
									"buckets": [
										{},
										{},
										{},
										{},
										{},
										{},
										{}
									]
								}
							}
						]
					}
				]
			}` + suffix,
		},
	}

	ctx := event.WithExporter(context.Background(), exporter)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.run(ctx)
			got := exporter.Output("/v1/metrics")
			checkJSON(t, got, []byte(tt.want))
		})
	}
}
