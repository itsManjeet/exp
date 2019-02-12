// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tlog

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// A Tree is a tree description signed by a notary.
type Tree struct {
	N    int64
	Hash Hash
}

// FormatTree formats a tree description for inclusion in a note.
func FormatTree(tree Tree) []byte {
	return []byte(fmt.Sprintf("go notary tree\n%d\n%s\n", tree.N, tree.Hash))
}

var errMalformedTree = errors.New("malformed tree note")
var treePrefix = []byte("go notary tree\n")

// ParseTree parses a tree root description.
func ParseTree(text []byte) (tree Tree, err error) {
	// The message looks like:
	//
	//	go notary tree
	//	2
	//	nND/nri/U0xuHUrYSy0HtMeal2vzD9V4k/BO79C+QeI=
	//
	// For forwards compatibility, extra text lines after the encoding are ignored.

	if !bytes.HasPrefix(text, treePrefix) || bytes.Count(text, []byte("\n")) != 3 || len(text) > 1000 {
		return Tree{}, errMalformedTree
	}

	lines := strings.Split(string(text), "\n")
	if len(lines) < 3 {
		return Tree{}, errMalformedTree
	}

	n, err := strconv.ParseInt(lines[1], 10, 64)
	if err != nil || n < 0 || lines[1] != strconv.FormatInt(n, 10) {
		return Tree{}, errMalformedTree
	}

	h, err := base64.StdEncoding.DecodeString(lines[2])
	if err != nil || len(h) != HashSize {
		return Tree{}, errMalformedTree
	}

	var hash Hash
	copy(hash[:], h)
	return Tree{n, hash}, nil
}
