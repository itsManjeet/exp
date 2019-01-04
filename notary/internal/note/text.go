// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Parsing and formatting of Go module notary messages.

package note

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

var errMalformedTree = errors.New("malformed tree note")

var treePrefix = []byte("go notary tree\n")

// FormatTree formats a tree root description with the given tree id and hash.
//
// The hash can be a tlog.Hash (no conversion is needed at the call site);
// using [32]byte here avoids making this package depend on package tlog.
func FormatTree(tid int64, hash [32]byte) []byte {
	return []byte(fmt.Sprintf("go notary tree\n%d\n%s\n", tid, base64.StdEncoding.EncodeToString(hash[:])))
}

// ParseTree parses a tree root description.
func ParseTree(text []byte) (tid int64, hash [32]byte, err error) {
	// The message looks like:
	//
	//	go notary tree
	//	2
	//	nND/nri/U0xuHUrYSy0HtMeal2vzD9V4k/BO79C+QeI=

	if !bytes.HasPrefix(text, treePrefix) || bytes.Count(text, []byte("\n")) != 3 || len(text) > 1000 {
		return 0, [32]byte{}, errMalformedTree
	}

	lines := strings.Split(string(text), "\n")
	tid, err = strconv.ParseInt(lines[1], 10, 64)
	if err != nil || lines[1] != fmt.Sprint(tid) {
		return 0, [32]byte{}, errMalformedTree
	}

	h, err := base64.StdEncoding.DecodeString(lines[2])
	if err != nil || len(h) != 32 {
		return 0, [32]byte{}, errMalformedTree
	}

	copy(hash[:], h)
	return tid, hash, nil
}

var sumPrefix = []byte("go notary sum\n")

// A Sum represents a fragment of a go.sum file describing one module version.
type Sum struct {
	Path      string   // module path
	Version   string   // specific module version (always semver)
	Hash      []string // hashes for main module (one of each hash algorithm)
	GoModHash []string // hashes for go.mod (again one of each hash algorithm, matching Hash)
}

// FormatSum formats a Sum.
func FormatSum(sum Sum) []byte {
	var buf bytes.Buffer

	w := buf.WriteString
	w("go notary sum\n")
	w(sum.Path)
	w(" ")
	w(sum.Version)
	w("\n")
	for _, h := range sum.Hash {
		w(h)
		w("\n")
	}
	for _, h := range sum.GoModHash {
		w("/go.mod ")
		w(h)
		w("\n")
	}
	return buf.Bytes()
}

// ParseGoSum parses a formatted Sum.
func ParseGoSum(text []byte) (Sum, error) {
	// The message looks like:
	//
	//	go notary sum
	//	rsc.io/quote v1.0.0
	//	h1:haUSojyo3j2M9g7CEUFG8Na09dtn7QKxvPGaPVQdGwM=
	//	/go.mod h1:v83Ri/njykPcgJltBc/gEkJTmjTsNgtO1Y7vyIK1CQA=

	if !bytes.HasPrefix(text, sumPrefix) || bytes.Count(text, []byte("\n")) != 4 || text[len(text)-1] != '\n' || len(text) > 10000 {
		return Sum{}, errMalformedTree
	}
	lines := strings.Split(string(text), "\n")

	// Line 0 was "go notary sum".
	// Line 1 is "<module> <version>".
	// Apply very basic sanity checks.
	f := strings.Split(lines[1], " ")
	if len(f) != 2 || f[0] == "" || !strings.HasPrefix(f[1], "v") || strings.Count(f[1], ".") < 2 || f[1][1] < '0' || f[1][1] > '9' {
		return Sum{}, errMalformedTree
	}
	s := Sum{Path: f[0], Version: f[1]}

	// Remaining lines are hashes for entire module and go.mod file.
	// There might be more than one hash if we have multiple hash algorithms in use.
	// (At time of writing there is only "h1".)
	// All hashes begin with "h#:".
	// Some lines have just a hash; others say "/go.mod " at the beginning.
	// Collect them into s.Hash and s.GoModHash.
	have := make(map[string]bool)
	for _, line := range lines {
		i := strings.Index(line, ":")
		if i < 0 || have[line[:i]] {
			return Sum{}, errMalformedTree
		}
		have[line[:i]] = true
		if strings.HasPrefix(line, "h") {
			s.Hash = append(s.Hash, line)
		} else if strings.HasPrefix(line, "/go.mod h") {
			line = line[len("/go.mod "):]
			s.GoModHash = append(s.GoModHash, line)
		}
		if strings.Contains(line, " ") {
			return Sum{}, errMalformedTree
		}
	}

	// We should see the the same number of each kind (s.Hash and s.GoModHash),
	// and with the same kinds of hashes.
	if len(s.Hash) == 0 || len(s.GoModHash) != len(s.Hash) {
		return Sum{}, errMalformedTree
	}
	for k := range have {
		if k[0] == 'h' && !have["/go.mod "+k] || k[0] == '/' && !have[k[len("/go.mod "):]] {
			return Sum{}, errMalformedTree
		}
	}
	sort.Strings(s.Hash)
	sort.Strings(s.GoModHash)

	return s, nil
}
