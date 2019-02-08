// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package database_test

import (
	"context"
	"testing"

	"golang.org/x/exp/sumdb/internal/database/dbtest"
	"golang.org/x/exp/sumdb/internal/tkv/tkvtest"
)

func TestDB(t *testing.T) {
	dbtest.TestStorage(t, context.Background(), new(tkvtest.Mem))
}
