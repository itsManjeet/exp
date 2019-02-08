// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package spanner

import (
	"context"
	"flag"
	"testing"

	"golang.org/x/exp/sumdb/internal/tkv/tkvtest"
)

var testInstance = flag.String("spanner", "", "test spanner instance (projects/xxx/instances/yyy)")

func TestSpanner(t *testing.T) {
	// Test basic spanner operations
	// (exercising interface wrapper, not spanner itself).

	if *testInstance == "" {
		t.Skip("no test instance given in -spanner flag")
	}

	ctx := context.Background()
	DeleteTestStorage(ctx, *testInstance+"/databases/test_spandb")
	s, err := CreateStorage(ctx, *testInstance+"/databases/test_spandb")
	if err != nil {
		t.Fatal(err)
	}
	defer DeleteTestStorage(ctx, *testInstance+"/databases/test_spandb")

	tkvtest.TestStorage(t, ctx, s)
}
