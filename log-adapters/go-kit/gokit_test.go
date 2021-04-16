// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package egokit

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/exp/event"
	"golang.org/x/exp/log-adapters/internal"
)

func Test(t *testing.T) {
	te := &internal.TestExporter{}
	log := NewLogger(te)
	log.Log("msg", "mess", "level", 1, "name", "n/m", "traceID", 17, "resource", "R")
	want := &event.Event{
		Kind:    event.LogKind,
		Message: "mess",
		Static:  [2]event.Label{internal.StringKey("level").Of(1), internal.StringKey("name").Of("n/m")},
		Dynamic: []event.Label{
			internal.StringKey("traceID").Of(17),
			internal.StringKey("resource").Of("R"),
			{}, // extra slot, since one key was "msg" and ended up in Event.Message.
		},
	}
	te.Got.At = want.At
	if diff := cmp.Diff(want, te.Got, internal.CmpOptions...); diff != "" {
		t.Errorf("mismatch (-want, +got):\n%s", diff)
	}
}
