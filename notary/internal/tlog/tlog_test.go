// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tlog

import (
	"bytes"
	"fmt"
	"testing"
)

type testHashStorage []Hash

func (t testHashStorage) ReadHash(level int, n int64) (Hash, error) {
	return t[StoredHashIndex(level, n)], nil
}

func (t testHashStorage) ReadHashes(index []int64) ([]Hash, error) {
	// It's not required by HashReader that indexes be in increasing order,
	// but check that the functions we are testing only ever ask for
	// indexes in increasing order.
	for i := 1; i < len(index); i++ {
		if index[i-1] >= index[i] {
			panic("indexes out of order")
		}
	}

	out := make([]Hash, len(index))
	for i, x := range index {
		out[i] = t[x]
	}
	return out, nil
}

func TestTree(t *testing.T) {
	var trees []Hash
	var leafhashes []Hash
	var storage testHashStorage
	tiles := make(map[Tile][]byte)
	const testH = 2
	for i := int64(0); i < 100; i++ {
		data := []byte(fmt.Sprintf("leaf %d", i))
		hashes, err := StoredHashes(i, data, storage)
		if err != nil {
			t.Fatal(err)
		}
		leafhashes = append(leafhashes, RecordHash(data))
		oldStorage := len(storage)
		storage = append(storage, hashes...)
		if count := StoredHashCount(i + 1); count != int64(len(storage)) {
			t.Errorf("StoredHashCount(%d) = %d, have %d StoredHashes", i+1, count, len(storage))
		}
		th, err := TreeHash(i+1, storage)
		if err != nil {
			t.Fatal(err)
		}

		for _, tile := range NewTiles(testH, i, i+1) {
			data, err := ReadTileData(tile, storage)
			if err != nil {
				t.Fatal(err)
			}
			old := Tile{H: tile.H, L: tile.L, N: tile.N, W: tile.W - 1}
			oldData := tiles[old]
			if len(oldData) != len(data)-HashSize || !bytes.Equal(oldData, data[:len(oldData)]) {
				t.Fatalf("tile %v not extending earlier tile %v", tile.Path(), old.Path())
			}
			tiles[tile] = data
		}
		for _, tile := range NewTiles(testH, 0, i+1) {
			data, err := ReadTileData(tile, storage)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(tiles[tile], data) {
				t.Fatalf("mismatch at %+v", tile)
			}
		}
		for _, tile := range NewTiles(testH, i/2, i+1) {
			data, err := ReadTileData(tile, storage)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(tiles[tile], data) {
				t.Fatalf("mismatch at %+v", tile)
			}
		}

		// Check that all the new hashes are readable from their tiles.
		for j := oldStorage; j < len(storage); j++ {
			tile := TileForIndex(testH, int64(j))
			data, ok := tiles[tile]
			if !ok {
				t.Log(NewTiles(testH, 0, i+1))
				t.Fatalf("TileForIndex(%d, %d) = %v, not yet stored (i=%d, stored %d)", testH, j, tile.Path(), i, len(storage))
				continue
			}
			h, err := HashFromTile(tile, data, int64(j))
			if err != nil {
				t.Fatal(err)
			}
			if h != storage[j] {
				t.Errorf("HashFromTile(%v, %d) = %v, want %v", tile.Path(), int64(j), h, storage[j])
			}
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

func TestSplitStoredHashIndex(t *testing.T) {
	for l := 0; l < 10; l++ {
		for n := int64(0); n < 100; n++ {
			x := StoredHashIndex(l, n)
			l1, n1 := SplitStoredHashIndex(x)
			if l1 != l || n1 != n {
				t.Fatalf("StoredHashIndex(%d, %d) = %d, but SplitStoredHashIndex(%d) = %d, %d", l, n, x, x, l1, n1)
			}
		}
	}
}
