// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ocagent_test

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"testing"
	"time"

	"golang.org/x/exp/event"
	"golang.org/x/exp/event/export/metric"
	"golang.org/x/exp/event/export/ocagent"
	"golang.org/x/exp/event/keys"
)

const testNodeStr = `{
	"node":{
		"identifier":{
			"host_name":"tester",
			"pid":1,
			"start_timestamp":"1970-01-01T00:00:00Z"
		},
		"library_info":{
			"language":4,
			"exporter_version":"0.0.1",
			"core_library_version":"x/tools"
		},
		"service_info":{
			"name":"ocagent-tests"
		}
	},`

var (
	keyDB     = keys.NewString("db", "the database name")
	keyMethod = keys.NewString("method", "a metric grouping key")
	keyRoute  = keys.NewString("route", "another metric grouping key")

	key1DB = keys.NewString("1_db", "A test string key")

	key2aAge      = keys.NewFloat64("2a_age", "A test float64 key")
	key2bTTL      = keys.NewFloat32("2b_ttl", "A test float32 key")
	key2cExpiryMS = keys.NewFloat64("2c_expiry_ms", "A test float64 key")

	key3aRetry = keys.NewBoolean("3a_retry", "A test boolean key")
	key3bStale = keys.NewBoolean("3b_stale", "Another test boolean key")

	key4aMax      = keys.NewInt("4a_max", "A test int key")
	key4bOpcode   = keys.NewInt8("4b_opcode", "A test int8 key")
	key4cBase     = keys.NewInt16("4c_base", "A test int16 key")
	key4eChecksum = keys.NewInt32("4e_checksum", "A test int32 key")
	key4fMode     = keys.NewInt64("4f_mode", "A test int64 key")

	key5aMin     = keys.NewUInt("5a_min", "A test uint key")
	key5bMix     = keys.NewUInt8("5b_mix", "A test uint8 key")
	key5cPort    = keys.NewUInt16("5c_port", "A test uint16 key")
	key5dMinHops = keys.NewUInt32("5d_min_hops", "A test uint32 key")
	key5eMaxHops = keys.NewUInt64("5e_max_hops", "A test uint64 key")

	recursiveCalls = keys.NewInt64("recursive_calls", "Number of recursive calls")
	bytesIn        = keys.NewInt64("bytes_in", "Number of bytes in")           //, unit.Bytes)
	latencyMs      = keys.NewFloat64("latency", "The latency in milliseconds") //, unit.Milliseconds)

	metricLatency = metric.HistogramFloat64{
		Name:        "latency_ms",
		Description: "The latency of calls in milliseconds",
		Keys:        []event.Key{keyMethod, keyRoute},
		Buckets:     []float64{0, 5, 10, 25, 50},
	}

	metricBytesIn = metric.HistogramInt64{
		Name:        "latency_ms",
		Description: "The latency of calls in milliseconds",
		Keys:        []event.Key{keyMethod, keyRoute},
		Buckets:     []int64{0, 10, 50, 100, 500, 1000, 2000},
	}

	metricRecursiveCalls = metric.Scalar{
		Name:        "latency_ms",
		Description: "The latency of calls in milliseconds",
		Keys:        []event.Key{keyMethod, keyRoute},
	}
)

type testExporter struct {
	ocagent   *ocagent.Exporter
	start     time.Time
	end       time.Time
	at        time.Time
	lastTrace uint64
	lastSpan  uint64
	sent      fakeSender
}

func newTestExporter() *testExporter {
	exporter := &testExporter{}
	cfg := ocagent.Config{
		Host:        "tester",
		Process:     1,
		Service:     "ocagent-tests",
		Client:      &http.Client{Transport: &exporter.sent},
		IDGenerator: exporter,
	}
	cfg.Start, _ = time.Parse(time.RFC3339Nano, "1970-01-01T00:00:00Z")
	exporter.start, _ = time.Parse(time.RFC3339Nano, "1970-01-01T00:00:30Z")
	exporter.at, _ = time.Parse(time.RFC3339Nano, "1970-01-01T00:00:40Z")
	exporter.end, _ = time.Parse(time.RFC3339Nano, "1970-01-01T00:00:50Z")
	exporter.ocagent = ocagent.Connect(&cfg)

	metrics := metric.Config{}
	metricLatency.Record(&metrics, latencyMs)
	metricBytesIn.Record(&metrics, bytesIn)
	metricRecursiveCalls.SumInt64(&metrics, recursiveCalls)

	return exporter
}

func (e *testExporter) Export(ev event.Event) {
	switch ev.Kind {
	case event.StartKind:
		ev.At = e.start
	case event.EndKind:
		ev.At = e.end
	default:
		ev.At = e.at
	}
	e.ocagent.Export(ctx, ev)
}

func (e *testExporter) TraceID(ev event.Event) []byte {
	e.lastTrace++
	id := make([]byte, 16)
	binary.LittleEndian.PutUint64(id, e.lastTrace)
	return id
}

func (e *testExporter) SpanID(ev event.Event) []byte {
	e.lastSpan++
	id := make([]byte, 8)
	binary.LittleEndian.PutUint64(id, e.lastSpan)
	return id
}

func (e *testExporter) Output(route string) []byte {
	e.ocagent.Flush()
	return e.sent.get(route)
}

func checkJSON(t *testing.T, got, want []byte) {
	// compare the compact form, to allow for formatting differences
	g := &bytes.Buffer{}
	if err := json.Compact(g, got); err != nil {
		t.Fatal(err)
	}
	w := &bytes.Buffer{}
	if err := json.Compact(w, want); err != nil {
		t.Fatal(err)
	}
	if g.String() != w.String() {
		t.Fatalf("Got:\n%s\nWant:\n%s", g, w)
	}
}

type fakeSender struct {
	mu   sync.Mutex
	data map[string][]byte
}

func (s *fakeSender) get(route string) []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, found := s.data[route]
	if found {
		delete(s.data, route)
	}
	return data
}

func (s *fakeSender) RoundTrip(req *http.Request) (*http.Response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data == nil {
		s.data = make(map[string][]byte)
	}
	data, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	path := req.URL.EscapedPath()
	if _, found := s.data[path]; found {
		return nil, fmt.Errorf("duplicate delivery to %v", path)
	}
	s.data[path] = data
	return &http.Response{
		Status:     "200 OK",
		StatusCode: 200,
		Proto:      "HTTP/1.0",
		ProtoMajor: 1,
		ProtoMinor: 0,
	}, nil
}
