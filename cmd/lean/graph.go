// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"sync"
)

type vertex struct {
	label     string
	sizeBytes int64
	// All vertices recursively dominated by this vertex.
	dominatees []*vertex
}

type edge struct {
	from *vertex
	to   *vertex
	// Number of times that from uses to. (AST parsing)
	numUsages int64
}

// edgeMap is the map of edges in a graph.
type edgeMap map[string]map[string]*edge

func (em edgeMap) contains(from, to *vertex) bool {
	if em == nil {
		em = make(edgeMap)
	}
	if _, ok := em[from.label]; !ok {
		return false
	}
	if _, ok := em[from.label][to.label]; !ok {
		return false
	}
	return true
}

func (em edgeMap) set(from, to *vertex) {
	if em == nil {
		em = make(edgeMap)
	}
	if _, ok := em[from.label]; !ok {
		em[from.label] = make(map[string]*edge)
	}
	em[from.label][to.label] = &edge{from: from, to: to}
}

func (em edgeMap) remove(from, to *vertex) error {
	if em == nil {
		em = make(edgeMap)
	}
	if !em.contains(from, to) {
		return fmt.Errorf("edge (%s, %s) not found", from.label, to.label)
	}
	delete(em[from.label], to.label)
	return nil
}

func (em edgeMap) vertices() []*vertex {
	out := []*vertex{}
	for _, vs := range em {
		for _, v := range vs {
			out = append(out, v.from)
			out = append(out, v.to)
		}
	}
	return out
}

// Returns !(a \ b).
// https://en.wikipedia.org/wiki/Complement_(set_theory)
func (a edgeMap) negativeComplement(b edgeMap) edgeMap {
	out := make(edgeMap)
	for bFrom, bTos := range b {
		for bTo, bEdge := range bTos {
			if _, ok := a[bFrom]; ok {
				if _, ok := a[bFrom][bTo]; ok {
					continue
				}
			}
			if _, ok := out[bFrom]; !ok {
				out[bFrom] = make(map[string]*edge)
			}
			out[bFrom][bTo] = bEdge
		}
	}
	return out
}

type graph struct {
	root string

	mu       sync.Mutex
	vertices map[string]*vertex
	edges    *edgeMap
}

func newGraph(r io.Reader) (*graph, error) {
	g := &graph{vertices: make(map[string]*vertex), edges: &edgeMap{}}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		l := scanner.Text()
		if l == "" {
			continue
		}
		parts := strings.Fields(l)
		if len(parts) != 2 {
			return nil, fmt.Errorf("expected 2 words in line, but got %d: %s", len(parts), l)
		}
		from := parts[0]
		to := parts[1]

		if _, ok := g.vertices[from]; !ok {
			g.vertices[from] = &vertex{label: from}
		}
		if _, ok := g.vertices[to]; !ok {
			g.vertices[to] = &vertex{label: to}
		}

		g.edges.set(g.vertices[from], g.vertices[to])

		// `go mod graph` always presents the root as the first "from" node
		if g.root == "" {
			g.root = from
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return g, nil
}

// copy creates a copy of g.
func (g *graph) copy() *graph {
	g.mu.Lock()
	defer g.mu.Unlock()

	newg := &graph{
		root:     g.root,
		vertices: make(map[string]*vertex),
		edges:    &edgeMap{},
	}
	for k, v := range g.vertices {
		newg.vertices[k] = v
	}

	for _, edges := range *g.edges {
		for _, edge := range edges {
			newg.edges.set(edge.from, edge.to)
		}
	}

	return newg
}

func (g *graph) addEdge(from, to string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, ok := g.vertices[from]; !ok {
		return fmt.Errorf("vertex %s does not exist", from)
	}
	if _, ok := g.vertices[to]; !ok {
		return fmt.Errorf("vertex %s does not exist", to)
	}
	g.edges.set(g.vertices[from], g.vertices[to])
	return nil
}

func (g *graph) removeEdge(from, to string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, ok := g.vertices[from]; !ok {
		return fmt.Errorf("vertex %s does not exist", from)
	}
	if _, ok := g.vertices[to]; !ok {
		return fmt.Errorf("vertex %s does not exist", to)
	}
	return g.edges.remove(g.vertices[from], g.vertices[to])
}

// connected returns the subgraph that is reachable from root.
func (g *graph) connected(root string) edgeMap {
	g.mu.Lock()
	defer g.mu.Unlock()

	sub := edgeMap{}
	seenVertices := make(map[string]struct{})
	var dfs func(from string)
	dfs = func(from string) {
		if _, ok := (*g.edges)[from]; !ok {
			return
		}
		if _, ok := seenVertices[from]; ok {
			return
		}
		seenVertices[from] = struct{}{}
		for _, e := range (*g.edges)[from] {
			sub.set(e.from, e.to)
			dfs(e.to.label)
		}
	}
	dfs(root)
	return sub
}

// hypotheticalCut returns a list of edges and vertices that would be pruned
// from the graph if the given (from, to) edge were cut. These lists may be
// empty but will not be nil.
//
// TODO(deklerk): This is inefficient and should be converted to Lengauer-Tarjan
// https://www.cs.au.dk/~gerth/advising/thesis/henrik-knakkegaard-christensen.pdf.
func (g *graph) hypotheticalCut(from, to string) (edgeMap, []string, error) {
	g.mu.Lock()
	if _, ok := g.vertices[from]; !ok {
		g.mu.Unlock()
		return nil, nil, fmt.Errorf("vertex %s does not exist", from)
	}
	if _, ok := g.vertices[to]; !ok {
		g.mu.Unlock()
		return nil, nil, fmt.Errorf("vertex %s does not exist", to)
	}
	if !g.edges.contains(g.vertices[from], g.vertices[to]) {
		g.mu.Unlock()
		return nil, nil, fmt.Errorf("edge (%s, %s) does not exist", from, to)
	}
	g.mu.Unlock()

	cut := g.copy()
	if err := cut.removeEdge(from, to); err != nil {
		return nil, nil, err
	}

	a := cut.connected(cut.root)
	b := cut.connected(to)

	return a.negativeComplement(b), negativeComplementVertices(a.vertices(), b.vertices()), nil
}

// Returns !(a \ b).
// https://en.wikipedia.org/wiki/Complement_(set_theory)
func negativeComplementVertices(a, b []*vertex) []string {
	amap := make(map[*vertex]struct{})
	for _, v := range a {
		amap[v] = struct{}{}
	}

	out := []string{}
	for _, v := range b {
		if _, ok := amap[v]; ok {
			continue
		}
		out = append(out, v.label)
	}
	return out
}
