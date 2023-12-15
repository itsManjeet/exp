// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build go1.22

package trace_test

import (
	"bytes"
	"runtime/trace"
	"testing"

	"golang.org/x/exp/trace/internal/testtrace"

	. "golang.org/x/exp/trace"
)

func TestFlightRecorderSmoke(t *testing.T) {
	if trace.IsEnabled() {
		t.Skip("cannot run flight recorder tests when tracing is enabled")
	}
	fr := NewFlightRecorder()
	testFlightRecorder(t, fr, func(snapshot func()) {
		snapshot()
	})
}

func TestFlightRecorderStartStop(t *testing.T) {
	if trace.IsEnabled() {
		t.Skip("cannot run flight recorder tests when tracing is enabled")
	}
	fr := NewFlightRecorder()
	for i := 0; i < 5; i++ {
		testFlightRecorder(t, fr, func(snapshot func()) {
			snapshot()
		})
	}
}

type flightRecorderTestFunc func(snapshot func())

func testFlightRecorder(t *testing.T, fr *FlightRecorder, f flightRecorderTestFunc) {
	// Start the flight recorder and immediately snapshot whatever we have.
	if err := fr.Start(); err != nil {
		t.Fatalf("unexpected error on Start: %v", err)
	}

	// Set up snapshot callback.
	var buf bytes.Buffer
	callback := func() {
		n, err := fr.WriteTo(&buf)
		if err != nil {
			fr.Stop()
			t.Fatalf("unexpected failure during flight recording: %v", err)
		}
		if n < 16 {
			t.Fatalf("expected a trace size of at least 16 bytes, got %d", n)
		}
	}

	// Call the test function.
	f(callback)

	// Stop the flight recorder.
	if err := fr.Stop(); err != nil {
		t.Fatalf("unexpected error on Stop: %v", err)
	}

	// Parse the trace to make sure it's not broken.
	testReader(t, &buf, testtrace.ExpectSuccess())
}
