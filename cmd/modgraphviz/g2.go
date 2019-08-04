package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"
)

type graph2 struct {
	vertices map[string]nodeset // Vertex -> successor
}

func convert2(r io.Reader) (*graph2, error) {
	scanner := bufio.NewScanner(r)
	g := &graph2{
		vertices: make(map[string]nodeset),
	}

	for scanner.Scan() {
		l := scanner.Text()
		if l == "" {
			continue
		}
		parts := strings.Fields(l)
		if len(parts) == 1 {
			node := parts[0]
			if _, ok := g.vertices[node]; !ok {
				g.vertices[node] = make(nodeset)
			}
			continue
		}
		if len(parts) > 2 {
			return nil, fmt.Errorf("expected 1 or 2 words in line, but got %d: %s", len(parts), l)
		}
		from := parts[0]
		to := parts[1]
		if _, ok := g.vertices[from]; !ok {
			g.vertices[from] = make(nodeset)
		}
		if _, ok := g.vertices[to]; !ok {
			g.vertices[to] = make(nodeset)
		}

		g.vertices[from][to] = true
	}
	return g, nil
}

func mustConvert2(r io.Reader) *graph2 {
	if g, err := convert2(r); err != nil {
		panic(err)
	} else {
		return g
	}
}

func (g *graph2) clone() *graph2 {
	n := graph2{
		vertices: make(map[string]nodeset),
	}
	for v, succs := range g.vertices {
		n.vertices[v] = make(nodeset)
		for succ := range succs {
			n.vertices[v][succ] = true
		}
	}
	return &n
}

func (g *graph2) printVertices() string {
	outBuf := bytes.Buffer{}
	for v := range g.vertices {
		outBuf.Write([]byte(v + "\n"))
	}
	outLines := strings.Split(outBuf.String(), "\n")
	sort.Strings(outLines)
	return strings.Join(outLines, "\n")
}

func (g *graph2) allSubgraphs(root string, minVerticesCut int) (*graph2, []*graph2, error) {
	rootGraph := g.clone()

	var cuts []edge
	seen := make(nodeset)
	var cut1 func(label string) (*graph2, *graph2, bool, error)
	cut1 = func(label string) (*graph2, *graph2, bool, error) {
		seen[label] = true
		for succ := range g.vertices[label] {
			if seen[succ] {
				continue
			}
			graphWithCut := rootGraph.clone()
			delete(graphWithCut.vertices[label], succ)

			if !graphWithCut.isConnected(root) {
				// Cutting this edge partitions the graph.

				// Gather the root and cut graph.
				newRoot, err := formGraph(graphWithCut, label)
				if err != nil {
					return nil, nil, false, err
				}
				cutGraph, err := formGraph(graphWithCut, succ)
				if err != nil {
					return nil, nil, false, err
				}

				// If the cut graph is not big enough to be meaningful, discard
				// and continue.
				if len(cutGraph.vertices) < minVerticesCut {
					continue
				}

				// Record the cut.
				cuts = append(cuts, edge{from: label, to: succ})

				// Return the now-smaller root graph and the cut graphs.
				return newRoot, cutGraph, true, nil
			}

			if newRoot, cutGraph, ok, err := cut1(succ); err != nil {
				return nil, nil, false, err
			} else if ok {
				return newRoot, cutGraph, ok, nil
			}
		}
		return rootGraph, nil, false, nil
	}

	var subgraphs []*graph2

	for {
		var cutGraph *graph2
		var ok bool
		var err error
		rootGraph, cutGraph, ok, err = cut1(root)
		if err != nil {
			return nil, nil, err
		}
		if !ok {
			// Add the cut edges back to rootGraph.
			for _, e := range cuts {
				if _, ok := rootGraph.vertices[e.from]; !ok {
					rootGraph.vertices[e.from] = make(nodeset)
				}
				rootGraph.vertices[e.from][e.to] = true
			}

			return rootGraph, subgraphs, nil
		}
		subgraphs = append(subgraphs, cutGraph)
	}
}

func (g *graph2) isConnected(root string) bool {
	seen := make(nodeset)
	var visit func(label string)
	visit = func(label string) {
		seen[label] = true
		for succ := range g.vertices[label] {
			if seen[succ] {
				continue
			}
			visit(succ)
		}
	}
	// Start at root. Since this is a digraph, starting at a leaf node would
	// always result in isConnected=true.
	visit(root)
	return len(seen) == len(g.vertices)
}

func formGraph(orig *graph2, root string) (*graph2, error) {
	in := bytes.Buffer{}
	seen := make(nodeset)
	var visit func(label string)
	visit = func(label string) {
		seen[label] = true
		for succ := range orig.vertices[label] {
			if seen[succ] {
				continue
			}
			in.Write([]byte(fmt.Sprintf("%s %s\n", label, succ)))
			visit(succ)
		}
	}
	visit(root)
	if len(seen) == 1 {
		// Only one item - the root.
		return convert2(bytes.NewBufferString(root))
	}
	return convert2(&in)
}
