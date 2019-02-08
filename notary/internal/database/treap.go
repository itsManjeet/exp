// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package database

import (
	"math/rand"
	"time"
)

// This file provides an ordered map backed by a balanced binary tree.
// The balancing is done by implementing the tree as a treap.
// A treap is a binary tree ordered according to the key order
// but then among the space of possible binary trees respecting those keys,
// it is kept balanced by maintaining a heap ordering on random priorities:
// x.priority <= x.left.priority && x.priority <= x.right.priority.
// See https://en.wikipedia.org/wiki/Treap and
// https://faculty.washington.edu/aragon/pubs/rst89.pdf.
// This implementation is generalized slightly from the one
// in go/src/runtime/mgclarge.go.

// A treeMap is an map from keys to values that
// allows traversal of the entire map in key order.
//
// The caller must initialize KeyCmp to a key comparison function
// that returns -1, 0, or +1 depending on whether
// key1 < key2, key1 == key2, or key1 > key2.
type treeMap struct {
	KeyCmp func(key1, key2 interface{}) int
	root   *treeNode
	rand   rand.Source
}

// A treeNode is a single node in the treeMap.
type treeNode struct {
	priority uint64
	parent   *treeNode
	left     *treeNode
	right    *treeNode
	key      interface{}
	value    interface{}
}

// Lookup returns the value associated with key.
// If key is not present in the map, Lookup returns nil.
func (t *treeMap) Lookup(key interface{}) interface{} {
	x := t.find(key)
	if x == nil {
		return nil
	}
	return x.value
}

// Insert adds the key-value pair to the map.
// If there is already a key with that value in the map,
// Insert overwrites the entry.
func (t *treeMap) Insert(key, value interface{}) {
	x := t.find(key)
	if x != nil {
		x.value = value
		return
	}
	x = &treeNode{key: key, value: value}
	t.insert(x)
}

// Delete removes the key-value pair with the given key from the map.
// If there is no such entry in the map, Delete is a no-op.
func (t *treeMap) Delete(key interface{}) {
	x := t.find(key)
	if x == nil {
		return
	}
	t.remove(x)
}

// DeleteAll removes all key-value pairs from the map.
func (t *treeMap) DeleteAll() {
	t.root = nil
}

// Visit calls f(key, value) for all key-value pairs in the tree.
// The function f must not modify the tree.
// If f returns an error, no more nodes are visited,
// and Visit returns that error.
func (t *treeMap) Visit(f func(key, value interface{}) error) error {
	return t.visitNode(t.root, f)
}

// visitNode calls f(k, v) for all key-value pairs in the subtree rooted at x,
// stopping early if f returns an error.
func (t *treeMap) visitNode(x *treeNode, f func(key, value interface{}) error) error {
	if x == nil {
		return nil
	}
	if err := t.visitNode(x.left, f); err != nil {
		return err
	}
	if err := f(x.key, x.value); err != nil {
		return err
	}
	if err := t.visitNode(x.right, f); err != nil {
		return err
	}
	return nil
}

// find returns the node with the given key.
func (t *treeMap) find(key interface{}) *treeNode {
	x := t.root
	for x != nil {
		switch cmp := t.KeyCmp(x.key, key); {
		case cmp < 0:
			x = x.right
		case cmp > 0:
			x = x.left
		default:
			return x
		}
	}
	return nil
}

// insert inserts the node y into the treeMap.
func (t *treeMap) insert(y *treeNode) {
	if t.rand == nil {
		t.rand = rand.NewSource(time.Now().UnixNano())
	}
	y.priority = uint64(t.rand.Int63())

	// Insert x as leaf.
	px := &t.root
	var last *treeNode
	for x := *px; x != nil; x = *px {
		last = x
		switch t.KeyCmp(x.key, y.key) {
		case -1:
			px = &x.right
		case +1:
			px = &x.left
		default:
			panic("insert already in treeMap")
		}
	}
	y.parent = last
	*px = y

	// Rotate y up into tree according to priority.
	for y.parent != nil && y.parent.priority > y.priority {
		if y.parent.left == y {
			t.rotateRight(y.parent)
		} else {
			if y.parent.right != y {
				panic("treeMap insert finds a broken treeMap")
			}
			t.rotateLeft(y.parent)
		}
	}
}

// remove removes the node x from the treeMap.
func (t *treeMap) remove(x *treeNode) {
	// Rotate x down to be leaf of tree for removal, respecting priorities.
	for x.right != nil || x.left != nil {
		if x.right == nil || x.left != nil && x.left.priority < x.right.priority {
			t.rotateRight(x)
		} else {
			t.rotateLeft(x)
		}
	}

	// Remove x, now a leaf.
	if x.left != nil || x.right != nil {
		panic("treeMap remove")
	}
	if x.parent != nil {
		if x.parent.left == x {
			x.parent.left = nil
		} else {
			x.parent.right = nil
		}
	} else {
		t.root = nil
	}
}

// rotateLeft rotates the tree rooted at node x.
// turning (x a (y b c)) into (y (x a b) c).
func (t *treeMap) rotateLeft(x *treeNode) {
	// p -> (x a (y b c))
	p := x.parent
	a, y := x.left, x.right
	b, c := y.left, y.right

	y.left = x
	x.parent = y
	y.right = c
	if c != nil {
		c.parent = y
	}
	x.left = a
	if a != nil {
		a.parent = x
	}
	x.right = b
	if b != nil {
		b.parent = x
	}

	y.parent = p
	if p == nil {
		t.root = y
	} else if p.left == x {
		p.left = y
	} else {
		if p.right != x {
			panic("treeMap rotateLeft")
		}
		p.right = y
	}
}

// rotateRight rotates the tree rooted at node y.
// turning (y (x a b) c) into (x a (y b c)).
func (t *treeMap) rotateRight(y *treeNode) {
	// p -> (y (x a b) c)
	p := y.parent
	x, c := y.left, y.right
	a, b := x.left, x.right

	x.left = a
	if a != nil {
		a.parent = x
	}
	x.right = y
	y.parent = x
	y.left = b
	if b != nil {
		b.parent = y
	}
	y.right = c
	if c != nil {
		c.parent = y
	}

	x.parent = p
	if p == nil {
		t.root = x
	} else if p.left == y {
		p.left = x
	} else {
		if p.right != y {
			panic("treeMap rotateRight")
		}
		p.right = x
	}
}
