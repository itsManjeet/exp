// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tlog

import (
	"fmt"
	"strconv"
	"strings"
)

// A Tile is a description of a transparency log tile.
// A tile of height H at level L offset N lists W consecutive hashes
// at level H*L of the tree starting at offset N*(2**H).
// A complete tile lists 2**H hashes; a partial tile lists fewer.
// Note that a tile represents the entire subtree of height H
// with those hashes as the leaves. The levels above H*L
// can be reconstructed by hashing the leaves.
//
// Each Tile can be encoded as a “tile coordinate path”
// of the form tile/H/L/NNN[.p/W].
// The .p/W suffix is present only for partial tiles, meaning W < 2**H.
// The NNN element is an encoding of N into 3-digit path elements.
// All but the last path element begins with an "x".
// For example,
// Tile{H: 3, L: 4, N: 1234067, W: 1}'s path
// is tile/3/4/x001/x234/067.p/1, and
// Tile{H: 3, L: 4, N: 1234067, W: 8}'s path
// is tile/3/4/x001/x234/067.
// See Tile's Path method and the ParseTilePath function.
type Tile struct {
	H int   // height of tile (1 ≤ H ≤ 30)
	L int   // level in tiling (1 ≤ H ≤ 63)
	N int64 // number within level (unbounded)
	W int   // width of tile (1 ≤ W ≤ 2**H; 2**H is complete tile)
}

// TileForIndex returns the tile of height h ≥ 1
// and least width storing the given hash index.
func TileForIndex(h int, index int64) Tile {
	if h < 1 {
		panic("TileForIndex: invalid height")
	}
	t, _, _ := tileForIndex(h, index)
	return t
}

// tileForIndex returns the tile of height h ≥ 1
// storing the given hash index, which can be
// reconstructed using tileHash(data[start:end]).
func tileForIndex(h int, index int64) (t Tile, start, end int) {
	level, n := SplitStoredHashIndex(index)
	t.H = h
	t.L = level / h
	level -= t.L * h // now level within tile
	t.N = n << uint(level) >> uint(t.H)
	n -= t.N << uint(t.H) >> uint(level) // now n within tile at level
	t.W = int((n + 1) << uint(level))
	return t, int(n<<uint(level)) * HashSize, int((n+1)<<uint(level)) * HashSize
}

// HashFromTile returns the hash at the given storage index,
// provided that t == TileForIndex(t.H, index) or a wider version,
// and data is t's tile data (of length at least t.W*HashSize).
func HashFromTile(t Tile, data []byte, index int64) (Hash, error) {
	if t.H < 1 || t.H > 30 || t.L < 0 || t.L >= 64 || t.W < 1 || t.W > 1<<uint(t.H) {
		return Hash{}, fmt.Errorf("invalid tile %v", t.Path())
	}
	if len(data) < t.W*HashSize {
		return Hash{}, fmt.Errorf("data len %d too short for tile %v", len(data), t.Path())
	}
	t1, start, end := tileForIndex(t.H, index)
	if t.L != t1.L || t.N != t1.N || t.W < t1.W {
		return Hash{}, fmt.Errorf("index %v is in %v not %v", index, t1.Path(), t.Path())
	}
	return tileHash(data[start:end]), nil
}

// tileHash computes the subtree hash corresponding to the (2^K)-1 hashes in data.
func tileHash(data []byte) Hash {
	if len(data) == 0 {
		panic("bad math in tileHash")
	}
	if len(data) == HashSize {
		var h Hash
		copy(h[:], data)
		return h
	}
	n := len(data) / 2
	return hashNode(tileHash(data[:n]), tileHash(data[n:]))
}

// NewTiles returns the coordinates of the tiles of height h ≥ 1
// that must be published when publishing from a tree of
// size newTreeSize to replace a tree of size oldTreeSize.
// (No tiles need to be published for a tree of size zero.)
func NewTiles(h int, oldTreeSize, newTreeSize int64) []Tile {
	if h < 1 {
		panic(fmt.Sprintf("NewTiles: invalid height %d", h))
	}
	H := uint(h)
	var tiles []Tile
	for level := uint(0); newTreeSize>>(H*level) > 0; level++ {
		oldN := oldTreeSize >> (H * level)
		newN := newTreeSize >> (H * level)
		for n := oldN >> H; n < newN>>H; n++ {
			tiles = append(tiles, Tile{H: h, L: int(level), N: n, W: 1 << H})
		}
		n := newN >> H
		maxW := int(newN - n<<H)
		minW := 1
		if oldN > n<<H {
			minW = int(oldN - n<<H)
		}
		for w := minW; w <= maxW; w++ {
			tiles = append(tiles, Tile{H: h, L: int(level), N: n, W: w})
		}
	}
	return tiles
}

// ReadTileData reads the hashes for tile t from r
// and returns the corresponding tile data.
func ReadTileData(t Tile, r HashReader) ([]byte, error) {
	size := t.W
	if size == 0 {
		size = 1 << uint(t.H)
	}
	start := t.N << uint(t.H)
	indexes := make([]int64, size)
	for i := 0; i < size; i++ {
		indexes[i] = StoredHashIndex(t.H*t.L, start+int64(i))
	}

	hashes, err := r.ReadHashes(indexes)
	if err != nil {
		return nil, err
	}
	if len(hashes) != len(indexes) {
		return nil, fmt.Errorf("notary: ReadHashes(%d indexes) = %d hashes", len(indexes), len(hashes))
	}

	tile := make([]byte, size*HashSize)
	for i := 0; i < size; i++ {
		copy(tile[i*HashSize:], hashes[i][:])
	}
	return tile, nil
}

// To limit the size of any particular directory listing,
// we encode the (possibly very large) number N
// by encoding three digits at a time.
// For example, 123456789 encodes as x123/x456/789.
// Each directory has less than 2000 children in the N encoding,
// and each N encoding can be itself or the .p partial form,
// so there are less than 4000 entries in any one directory.
const pathBase = 1000

// Path returns a tile coordinate path describing t.
func (t Tile) Path() string {
	n := t.N
	nStr := fmt.Sprintf("%03d", n%pathBase)
	for n >= pathBase {
		n /= pathBase
		nStr = fmt.Sprintf("x%03d/%s", n%pathBase+pathBase, nStr)
	}
	pStr := ""
	if t.W != 1<<uint(t.H) {
		pStr = fmt.Sprintf(".p/%d", t.W)
	}
	return fmt.Sprintf("tile/%d/%d/%s%s", t.H, t.L, nStr, pStr)
}

// ParseTilePath parses a tile coordinate path.
func ParseTilePath(path string) (Tile, error) {
	f := strings.Split(path, "/")
	if len(f) < 4 || f[0] != "tile" {
		return Tile{}, &badPathError{path}
	}
	h, err1 := strconv.Atoi(f[1])
	l, err2 := strconv.Atoi(f[2])
	if err1 != nil || err2 != nil || h < 1 || l < 0 || h > 30 {
		return Tile{}, &badPathError{path}
	}
	w := 1 << uint(h)
	if dotP := f[len(f)-2]; strings.HasSuffix(dotP, ".p") {
		ww, err := strconv.Atoi(f[len(f)-1])
		if err != nil || ww <= 0 || ww >= w {
			return Tile{}, &badPathError{path}
		}
		f = f[:len(f)-1]
		f[len(f)-1] = dotP[:len(dotP)-2]
	}
	f = f[3:]
	n := int64(0)
	for _, s := range f {
		nn, err := strconv.Atoi(strings.TrimPrefix(s, "x"))
		if err != nil || nn < 0 || nn >= pathBase {
			return Tile{}, &badPathError{path}
		}
		n = n*pathBase + int64(nn)
	}
	t := Tile{H: h, L: l, N: n, W: w}
	if path != t.Path() {
		return Tile{}, &badPathError{path}
	}
	return t, nil
}

type badPathError struct {
	path string
}

func (e *badPathError) Error() string {
	return fmt.Sprintf("malformed tile path %q", e.path)
}
