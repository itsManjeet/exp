// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package database

import (
	"math/rand"
	"testing"
)

func TestTreap(t *testing.T) {
	tr := new(treeMap)
	tr.KeyCmp = func(x, y interface{}) int {
		if x.(int) < y.(int) {
			return -1
		}
		if x.(int) > y.(int) {
			return +1
		}
		return 0
	}

	// Test random inserts, with lookups after each.
	const N = 10
	const X = 55
	perm := rand.Perm(N)
	for i := 0; i < N; i++ {
		for j, p := range perm {
			v := tr.Lookup(p)
			if j < i {
				if v == nil {
					t.Fatalf("at #%d: cannot find #%d = %d", i, j, p)
				}
				if v != p^X {
					t.Fatalf("at %d: wrong value for key #%d = %d: have %v want %d", i, j, p, v, p^X)
				}
			} else {
				if v != nil {
					t.Fatalf("at #%d: unexpected value for #%d = %d: have %v", i, j, p, v)
				}
			}
		}
		t.Logf("insert %d = %d", perm[i], perm[i]^X)
		tr.Insert(perm[i], perm[i]^X)
	}

	// Test random deletion order, with lookups after each.
	perm = rand.Perm(N)
	for i := 0; i <= N; i++ {
		for j, p := range perm {
			v := tr.Lookup(p)
			if j >= i {
				if v == nil {
					t.Errorf("at #%d: cannot find #%d = %d", i, j, p)
					tr.Visit(func(k, v interface{}) error {
						t.Logf("  %v => %v", k, v)
						return nil
					})
					t.FailNow()
				}
				if v != p^X {
					t.Fatalf("at %d: wrong value for key #%d = %d: have %v want %d", i, j, p, v, p^X)
				}
			} else {
				if v != nil {
					t.Fatalf("at #%d: unexpected value for #%d = %d: have %v", i, j, p, v)
				}
			}
		}
		if i == N {
			// Tested that all values are gone.
			break
		}
		t.Logf("delete %d", perm[i])
		tr.Delete(perm[i])
	}
}
