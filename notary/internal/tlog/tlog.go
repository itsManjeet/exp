// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package tlog implements a tamper-evident log
// used in the Go module notary.
//
// This package is part of a DRAFT of what the Go module notary will look like.
// Do not assume the details here are final!
//
// This package follows the design of Certificate Transparency (RFC 6962)
// and its proof are compatible with that system.
// See ExampleCertificateTransparency.
//
package tlog

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
)

// A Hash is a hash identifying a log record or tree root.
type Hash [32]byte

// String returns a base64 representation of the hash for printing.
func (h Hash) String() string {
	return base64.StdEncoding.EncodeToString(h[:])
}

// MarshalJSON marshals the hash as a JSON string containing the base64-encoded hash.
func (h Hash) MarshalJSON() ([]byte, error) {
	return []byte(`"` + h.String() + `"`), nil
}

// UnmarshalJSON unmarshals a hash from JSON string containing the a base64-encoded hash.
func (h *Hash) UnmarshalJSON(data []byte) error {
	if len(data) != 1+44+1 || data[0] != '"' || data[len(data)-2] != '=' || data[len(data)-1] != '"' {
		return errors.New("cannot decode hash")
	}
	var tmp [33]byte // base64.Decode insists on slicing past the end of the written data!
	n, err := base64.StdEncoding.Decode(tmp[:], data[1:len(data)-1])
	if err != nil || n != 32 {
		return errors.New("cannot decode hash")
	}
	copy(h[:], tmp[:])
	return nil
}

// ParseHash parses the base64-encoded string form of a hash.
func ParseHash(s string) (Hash, error) {
	data, err := base64.StdEncoding.DecodeString(s)
	if err != nil || len(data) != 32 {
		return Hash{}, fmt.Errorf("malformed hash")
	}
	var h Hash
	copy(h[:], data)
	return h, nil
}

// maxpow2 returns k, the maximum power of 2 smaller than n,
// as well as l = log₂ k (so k = 1<<l).
func maxpow2(n int64) (k int64, l int) {
	l = 0
	for 1<<uint(l+1) < n {
		l++
	}
	return 1 << uint(l), l
}

var zeroPrefix = []byte{0x00}

// RecordHash returns the content hash for the given record data.
func RecordHash(data []byte) Hash {
	// SHA256(0x00 || data)
	// https://tools.ietf.org/html/rfc6962#section-2.1
	h := sha256.New()
	h.Write(zeroPrefix)
	h.Write(data)
	var h1 Hash
	h.Sum(h1[:0])
	return h1
}

// hashNode returns the hash for an interior tree node with the given left and right hashes.
func hashNode(left, right Hash) Hash {
	// SHA256(0x01 || left || right)
	// https://tools.ietf.org/html/rfc6962#section-2.1
	// We use a stack buffer to assemble the hash input
	// to avoid allocating a hash struct with sha256.New.
	var buf [1 + 32 + 32]byte
	buf[0] = 0x01
	copy(buf[1:], left[:])
	copy(buf[1+32:], right[:])
	return sha256.Sum256(buf[:])
}

// For information about the stored hash index ordering,
// see section 3.3 of Crosby and Wallach's paper
// "Efficient Data Structures for Tamper-Evident Logging".
// https://www.usenix.org/legacy/event/sec09/tech/full_papers/crosby.pdf

// StoredHashIndex maps the tree coordinates (level, n)
// to a dense linear ordering that can be used for hash storage.
// Hash storage implementations that store hashes in sequential
// storage can use this function to compute where to read or write
// a given hash.
func StoredHashIndex(level int, n int64) int64 {
	// Level L's n'th hash is written right after level L+1's 2n+1'th hash.
	// Work our way down to the level 0 ordering.
	// We'll add back the orignal level count at the end.
	for l := level; l > 0; l-- {
		n = 2*n + 1
	}

	// Level 0's n'th hash is written at n+n/2+n/4+... (eventually n/2ⁱ hits zero).
	i := int64(0)
	for ; n > 0; n >>= 1 {
		i += n
	}

	return i + int64(level)
}

// StoredHashCount returns the number of stored hashes
// that are expected for a tree with n records.
func StoredHashCount(n int64) int64 {
	if n == 0 {
		return 0
	}
	// The tree will have the hashes up to the last leaf hash.
	numHash := StoredHashIndex(0, n-1) + 1
	// And it will have any hashes for subtrees completed by that leaf.
	for i := uint64(n - 1); i&1 != 0; i >>= 1 {
		numHash++
	}
	return numHash
}

// StoredHashes returns the hashes that must be stored when writing
// record n with the given data. The hashes should be stored starting
// at StoredHashIndex(0, n). The result will have at most 1 + log₂ n hashes,
// but it will average just under two per call for a sequence of calls for n=1..k.
//
// StoredHashes may read up to log n earlier hashes from r
// in order to compute hashes for completed subtrees.
func StoredHashes(n int64, data []byte, r HashReader) ([]Hash, error) {
	// Start with the record hash.
	hashes := []Hash{RecordHash(data)}

	// Add hashes for completed subtrees.
	// Each trailing 1 bit in the binary representation of n completes a subtree.
	level := 0
	for i := n; i&1 == 1; {
		level++
		i >>= 1
		h1, err := r.ReadHash(level-1, 2*i)
		if err != nil {
			return nil, fmt.Errorf("computing new leaf hashes: %v", err)
		}
		h2 := hashes[len(hashes)-1]
		hashes = append(hashes, hashNode(h1, h2))
	}
	return hashes, nil
}

// A HashReader can read hashes for nodes in the log's tree structure.
type HashReader interface {
	// ReadHash returns n'th hash at the given level of the tree.
	// Level 0 is the leaf nodes; level 1 is parents of leaf nodes, and so on.
	ReadHash(level int, n int64) (Hash, error)
}

// TreeHash computes the hash for the root of the tree with n records,
// using the HashReader to obtain previously stored hashes
// (those returned by StoredHashes during the writes of those n records).
// TreeHash makes at most 1 + log₂ n calls to r.ReadHash.
func TreeHash(n int64, r HashReader) (Hash, error) {
	return subTreeHash(0, n, r)
}

// subTreeHash computes the hash for the subtree containing records [lo, hi).
// See https://tools.ietf.org/html/rfc6962#section-2.1
func subTreeHash(lo, hi int64, r HashReader) (Hash, error) {
	// Partition the tree into a left side with 2^level nodes,
	// for as large a level as possible, and a right side with the fringe.
	// The left hash is stored directly and can be read from storage.
	// The right side needs further computation.
	k, level := maxpow2(hi - lo + 1)
	if lo&(k-1) != 0 {
		panic("notary: bad math in treeHash")
	}
	lh, err := r.ReadHash(level, lo>>uint(level))
	if err != nil {
		return Hash{}, err
	}

	// If all the nodes were on the left side, we're done.
	if lo+k == hi {
		return lh, nil
	}

	// Otherwise work out the hash of the right side
	// and combine with the left.
	rh, err := subTreeHash(lo+k, hi, r)
	if err != nil {
		return Hash{}, err
	}
	return hashNode(lh, rh), nil
}

// A RecordProof is a verifiable proof that a particular log root contains a particular record.
// RFC 6962 callls this a “Merkle audit path.”
type RecordProof []Hash

// ProveRecord returns the proof that the tree of size t contains the record with index n.
func ProveRecord(t, n int64, r HashReader) (RecordProof, error) {
	if t < 0 || n < 0 || n >= t {
		return nil, fmt.Errorf("notary: invalid inputs in ProveRecord")
	}
	return leafProof(0, t, n, r)
}

// leafProof constructs the proof that leaf n is contained in the subtree with leaves [lo, hi).
// See https://tools.ietf.org/html/rfc6962#section-2.1.1
func leafProof(lo, hi, n int64, h HashReader) (RecordProof, error) {
	// We must have lo <= n < hi or else the code here has a bug.
	if !(lo <= n && n < hi) {
		panic("notary: bad math in leafProof")
	}

	if lo+1 == hi { // n == lo
		// Reached the leaf node.
		// The verifier knows what the leaf hash is, so we don't need to send it.
		return RecordProof{}, nil
	}

	// Walk down the tree toward n.
	// Record the hash of the path not taken (needed for verifying the proof).
	var p RecordProof
	var th Hash
	k, _ := maxpow2(hi - lo)
	if n < lo+k {
		// n is on left side
		var err error
		p, err = leafProof(lo, lo+k, n, h)
		if err != nil {
			return nil, err
		}
		th, err = subTreeHash(lo+k, hi, h)
		if err != nil {
			return nil, err
		}
	} else {
		// n is on right side
		var err error
		th, err = subTreeHash(lo, lo+k, h)
		if err != nil {
			return nil, err
		}
		p, err = leafProof(lo+k, hi, n, h)
		if err != nil {
			return nil, err
		}
	}
	return append(p, th), nil
}

var errProofFailed = errors.New("invalid transparency proof")

// CheckRecord verifies that p is a valid proof that the tree of size t
// with hash th has an n'th record with hash h.
func CheckRecord(p RecordProof, t int64, th Hash, n int64, h Hash) error {
	if t < 0 || n < 0 || n >= t {
		return fmt.Errorf("notary: invalid inputs in CheckRecord")
	}
	th2, err := runRecordProof(p, 0, t, n, h)
	if err != nil {
		return err
	}
	if th2 == th {
		return nil
	}
	return errProofFailed
}

// runRecordProof runs the proof p that leaf n is contained in the subtree with leaves [lo, hi).
// Running the proof means constructing and returning the implied hash of that
// subtree.
func runRecordProof(p RecordProof, lo, hi, n int64, leafHash Hash) (Hash, error) {
	// We must have lo <= n < hi or else the code here has a bug.
	if !(lo <= n && n < hi) {
		panic("notary: bad math in runRecordProof")
	}

	if lo+1 == hi { // m == lo
		// Reached the leaf node.
		// The proof must not have any unnecessary hashes.
		if len(p) != 0 {
			return Hash{}, errProofFailed
		}
		return leafHash, nil
	}

	if len(p) == 0 {
		return Hash{}, errProofFailed
	}

	k, _ := maxpow2(hi - lo)
	if n < lo+k {
		th, err := runRecordProof(p[:len(p)-1], lo, lo+k, n, leafHash)
		if err != nil {
			return Hash{}, err
		}
		return hashNode(th, p[len(p)-1]), nil
	} else {
		th, err := runRecordProof(p[:len(p)-1], lo+k, hi, n, leafHash)
		if err != nil {
			return Hash{}, err
		}
		return hashNode(p[len(p)-1], th), nil
	}
}

// A TreeProof is a verifiable proof that a particular log tree contains
// as a prefix all records present in an earlier tree.
type TreeProof []Hash

// ProveTree returns the proof that the tree of size t contains
// as a prefix all the records from the tree of smaller size n.
func ProveTree(t, n int64, h HashReader) (TreeProof, error) {
	if t < 1 || n < 1 || n > t {
		return nil, fmt.Errorf("notary: invalid inputs in ProveTree")
	}
	return treeProof(0, t, n, h)
}

// treeProof constructs the sub-proof related to the subtree containing records [lo, hi).
// See https://tools.ietf.org/html/rfc6962#section-2.1.2.
func treeProof(lo, hi, n int64, h HashReader) (TreeProof, error) {
	// We must have lo < n <= hi or else the code here has a bug.
	if !(lo < n && n <= hi) {
		panic("notary: bad math in treeProof")
	}

	// Reached common ground.
	if n == hi {
		if lo == 0 {
			// This subtree corresponds exactly to the old tree.
			// The verifier knows that hash, so we don't need to send it.
			return TreeProof{}, nil
		}
		th, err := subTreeHash(lo, hi, h)
		if err != nil {
			return nil, err
		}
		return TreeProof{th}, nil
	}

	// Interior node for the proof.
	// Decide whether to walk down the left or right side.
	k, _ := maxpow2(hi - lo)
	var p TreeProof
	var th Hash
	if n <= lo+k {
		// m is on left side
		var err error
		p, err = treeProof(lo, lo+k, n, h)
		if err != nil {
			return nil, err
		}
		th, err = subTreeHash(lo+k, hi, h)
		if err != nil {
			return nil, err
		}
	} else {
		// m is on right side
		var err error
		th, err = subTreeHash(lo, lo+k, h)
		if err != nil {
			return nil, err
		}
		p, err = treeProof(lo+k, hi, n, h)
		if err != nil {
			return nil, err
		}
	}
	return append(p, th), nil
}

// CheckTree verifies that p is a valid proof that the tree of size t with hash th
// contains as a prefix the tree of size n with hash h.
func CheckTree(p TreeProof, t int64, th Hash, n int64, h Hash) error {
	if t < 1 || n < 1 || n > t {
		return fmt.Errorf("notary: invalid inputs in CheckTree")
	}
	h2, th2, err := runTreeProof(p, 0, t, n, h)
	if err != nil {
		return err
	}
	if th2 == th && h2 == h {
		return nil
	}
	return errProofFailed
}

// runTreeProof runs the sub-proof p related to the subtree containing records [lo, hi),
// where old is the hash of the old tree with n records.
// Running the proof means constructing and returning the implied hashes of that
// subtree in both the old and new tree.
func runTreeProof(p TreeProof, lo, hi, n int64, old Hash) (Hash, Hash, error) {
	// We must have lo < n <= hi or else the code here has a bug.
	if !(lo < n && n <= hi) {
		panic("notary: bad math in runTreeProof")
	}

	// Reached common ground.
	if n == hi {
		if lo == 0 {
			if len(p) != 0 {
				return Hash{}, Hash{}, errProofFailed
			}
			return old, old, nil
		}
		if len(p) != 1 {
			return Hash{}, Hash{}, errProofFailed
		}
		return p[0], p[0], nil
	}

	if len(p) == 0 {
		return Hash{}, Hash{}, errProofFailed
	}

	// Interior node for the proof.
	k, _ := maxpow2(hi - lo)
	if n <= lo+k {
		oh, th, err := runTreeProof(p[:len(p)-1], lo, lo+k, n, old)
		if err != nil {
			return Hash{}, Hash{}, err
		}
		return oh, hashNode(th, p[len(p)-1]), nil
	} else {
		oh, th, err := runTreeProof(p[:len(p)-1], lo+k, hi, n, old)
		if err != nil {
			return Hash{}, Hash{}, err
		}
		return hashNode(p[len(p)-1], oh), hashNode(p[len(p)-1], th), nil
	}
}
