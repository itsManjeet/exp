// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package log_test

import (
	"context"

	"golang.org/x/exp/event/log"
)

func Example_unstructured() {
	ctx := context.Background()
	log.Infof(ctx, "starting the example")
	x := 100
	if x > 50 {
		log.Errorf(ctx, "x is too large: %d", x)
	}
}

func Example_structured() {
	ctx := context.Background()
	x := 100
	if x > 50 {
		log.With("x", x).Errorf(ctx, "x is too large")
	}
}
