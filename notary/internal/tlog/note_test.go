// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tlog

import "testing"

func TestFormatTree(t *testing.T) {
	n := int64(123456789012)
	h := RecordHash([]byte("hello world"))
	golden := "go notary tree\n123456789012\nTszzRgjTG6xce+z2AG31kAXYKBgQVtCSCE40HmuwBb0=\n"
	b := FormatTree(Tree{n, h})
	if string(b) != golden {
		t.Errorf("FormatTree(...) = %q, want %q", b, golden)
	}
}

func TestParseTree(t *testing.T) {
	in := "go notary tree\n123456789012\nTszzRgjTG6xce+z2AG31kAXYKBgQVtCSCE40HmuwBb0=\n"
	goldH := RecordHash([]byte("hello world"))
	goldN := int64(123456789012)
	tree, err := ParseTree([]byte(in))
	if tree.N != goldN || tree.Hash != goldH || err != nil {
		t.Fatalf("ParseTree(...) = Tree{%d, %v}, %v, want Tree{%d, %v}, nil", tree.N, tree.Hash, err, goldN, goldH)
	}

	// Check junk on end is ignored.
	tree, err = ParseTree([]byte(in + "JOE"))
	if tree.N != goldN || tree.Hash != goldH || err != nil {
		t.Fatalf("ParseTree(...+JOE) = Tree{%d, %v}, %v, want Tree{%d, %v}, nil", tree.N, tree.Hash, err, goldN, goldH)
	}
}
