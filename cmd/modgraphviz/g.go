package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"

	"golang.org/x/mod/semver"
)

type graph struct {
	edges       []edge
	mvsPicked   []string
	mvsUnpicked []string
}

// convert reads “go mod graph” output from r and returns a graph, recording
// MVS picked and unpicked nodes along the way.
func convert(r io.Reader) (*graph, error) {
	scanner := bufio.NewScanner(r)
	var g graph
	seen := map[string]bool{}
	mvsPicked := map[string]string{} // module name -> module version

	for scanner.Scan() {
		l := scanner.Text()
		if l == "" {
			continue
		}
		parts := strings.Fields(l)
		if len(parts) > 2 {
			return nil, fmt.Errorf("expected 1 or 2 words in line, but got %d: %s", len(parts), l)
		}
		from := parts[0]
		to := parts[1]
		g.edges = append(g.edges, edge{from: from, to: to})

		for _, node := range []string{from, to} {
			if _, ok := seen[node]; ok {
				// Skip over nodes we've already seen.
				continue
			}
			seen[node] = true

			var m, v string
			if i := strings.IndexByte(node, '@'); i >= 0 {
				m, v = node[:i], node[i+1:]
			} else {
				// Root node doesn't have a version.
				continue
			}

			if maxV, ok := mvsPicked[m]; ok {
				if semver.Compare(maxV, v) < 0 {
					// This version is higher - replace it and consign the old
					// max to the unpicked list.
					g.mvsUnpicked = append(g.mvsUnpicked, m+"@"+maxV)
					mvsPicked[m] = v
				} else {
					// Other version is higher - stick this version in the
					// unpicked list.
					g.mvsUnpicked = append(g.mvsUnpicked, node)
				}
			} else {
				mvsPicked[m] = v
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	for m, v := range mvsPicked {
		g.mvsPicked = append(g.mvsPicked, m+"@"+v)
	}

	// Make this function deterministic.
	sort.Strings(g.mvsPicked)

	return &g, nil
}

// edgesAsDOT returns the edges in DOT notation.
func (g *graph) edgesAsDOT() []byte {
	var buf bytes.Buffer
	for _, e := range g.edges {
		fmt.Fprintf(&buf, "\t%q -> %q\n", e.from, e.to)
	}
	return buf.Bytes()
}
