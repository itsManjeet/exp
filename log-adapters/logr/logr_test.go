// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package elogr

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/exp/event"
	"golang.org/x/exp/log-adapters/internal"
)

func TestInfo(t *testing.T) {
	te := &internal.TestExporter{}
	log := NewLogger(te, "/").WithName("n").V(3)
	log = log.WithName("m")
	log.Info("mess", "traceID", 17, "resource", "R")
	want := &event.Event{
		Kind:    event.LogKind,
		Message: "mess",
		Static:  [2]event.Label{internal.LevelKey.Of(3), internal.NameKey.Of("n/m")},
		Dynamic: []event.Label{
			internal.StringKey("traceID").Of(17),
			internal.StringKey("resource").Of("R"),
		},
	}
	te.Got.At = want.At
	if diff := cmp.Diff(want, te.Got, internal.CmpOptions...); diff != "" {
		t.Errorf("mismatch (-want, +got):\n%s", diff)
	}
}
