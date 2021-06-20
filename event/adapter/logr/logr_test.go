// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !disable_events

package logr_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/exp/event"
	elogr "golang.org/x/exp/event/adapter/logr"
	"golang.org/x/exp/event/eventtest"
	"golang.org/x/exp/event/keys"
	"golang.org/x/exp/event/severity"
)

func TestInfo(t *testing.T) {
	ctx, th := eventtest.NewCapture()
	log := elogr.NewLogger(ctx, "/").WithName("n").V(int(severity.DebugLevel))
	log = log.WithName("m")
	log.Info("mess", "traceID", 17, "resource", "R")
	want := []event.Event{{
		At:        eventtest.InitialTime,
		Kind:      event.LogKind,
		Message:   "mess",
		Name:      "n/m",
		Namespace: "golang.org/x/exp/event/adapter/logr",
		Labels: []event.Label{
			severity.Debug,
			keys.Value("traceID").Of(17),
			keys.Value("resource").Of("R"),
		},
	}}
	if diff := cmp.Diff(want, th.Got); diff != "" {
		t.Errorf("mismatch (-want, +got):\n%s", diff)
	}
}
