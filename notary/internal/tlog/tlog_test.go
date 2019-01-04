// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tlog

import (
	"fmt"
	"testing"
)

type testHashStorage []Hash

func (t testHashStorage) ReadHash(level int, n int64) (Hash, error) {
	return t[StoredHashIndex(level, n)], nil
}

func TestTree(t *testing.T) {
	var trees []Hash
	var leafhashes []Hash
	var storage testHashStorage
	for i := int64(0); i < 10; i++ {
		data := []byte(fmt.Sprintf("leaf %d", i))
		hashes, err := StoredHashes(i, data, storage)
		if err != nil {
			t.Fatal(err)
		}
		leafhashes = append(leafhashes, RecordHash(data))
		storage = append(storage, hashes...)
		th, err := TreeHash(i+1, storage)
		if err != nil {
			t.Fatal(err)
		}
		trees = append(trees, th)

		// Check that leaf proofs work, for all trees and leaves so far.
		for j := int64(0); j <= i; j++ {
			p, err := ProveRecord(i+1, j, storage)
			if err != nil {
				t.Fatalf("ProveRecord(%d, %d): %v", i+1, j, err)
			}
			if err := CheckRecord(p, i+1, th, j, leafhashes[j]); err != nil {
				t.Fatalf("CheckRecord(%d, %d): %v", i+1, j, err)
			}
			for k := range p {
				p[k][0] ^= 1
				if err := CheckRecord(p, i+1, th, j, leafhashes[j]); err == nil {
					t.Fatalf("CheckRecord(%d, %d) succeeded with corrupt proof hash #%d!", i+1, j, k)
				}
				p[k][0] ^= 1
			}
		}

		// Check that tree proofs work, for all trees so far.
		for j := int64(0); j <= i; j++ {
			p, err := ProveTree(i+1, j+1, storage)
			if err != nil {
				t.Fatalf("ProveTree(%d, %d): %v", i+1, j+1, err)
			}
			if err := CheckTree(p, i+1, th, j+1, trees[j]); err != nil {
				t.Fatalf("CheckTree(%d, %d): %v [%v]", i+1, j+1, err, p)
			}
			for k := range p {
				p[k][0] ^= 1
				if err := CheckTree(p, i+1, th, j+1, trees[j]); err == nil {
					t.Fatalf("CheckTree(%d, %d) succeeded with corrupt proof hash #%d!", i+1, j+1, k)
				}
				p[k][0] ^= 1
			}
		}
	}
}
