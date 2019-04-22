package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

type graph struct {
	root     *vertex
	vertices map[string]bool
}

type vertex struct {
	name  string
	edges map[string]*vertex
}

func newVertex(name string) *vertex {
	return &vertex{name: name, edges: map[string]*vertex{}}
}

// newGraph builds a graph from a Reader that has data formatted in the format:
//
// A B
//
// Where A has a directed edge to B.
func newGraph(in io.Reader) (*graph, error) {
	vertexMap := map[string]*vertex{}
	var root string
	r := bufio.NewScanner(in)
	for r.Scan() {
		parts := strings.Fields(r.Text())
		if r.Text() == "" {
			// Ignore empty lines.
			continue
		}
		if len(parts) != 2 {
			return nil, fmt.Errorf("couldn't decipher %s", r.Text())
		}

		fromName := parts[0]
		if root == "" {
			root = fromName
		}

		toName := parts[1]
		fromVertex, ok := vertexMap[fromName]
		if !ok {
			// fromVertex is nil - we couldn't find it. This could happen if
			// the list is unordered: B->C, A->B.
			fromVertex = newVertex(fromName)
			vertexMap[fromVertex.name] = fromVertex
		}

		toVertex, ok := vertexMap[toName]
		if !ok {
			toVertex = newVertex(toName)
			vertexMap[toVertex.name] = toVertex
		}

		fromVertex.edges[toVertex.name] = toVertex
	}
	if r.Err() != nil {
		return nil, r.Err()
	}

	vertices := map[string]bool{}
	for k := range vertexMap {
		vertices[k] = true
	}

	return &graph{
		root:     vertexMap[root],
		vertices: vertices,
	}, nil
}

// print prints the graph.
func (g *graph) print(out io.Writer) error {
	return g.printDFS(out, map[string]bool{}, g.root)
}

// printDFS traverses the graph depth first, printing each edge.
func (g *graph) printDFS(out io.Writer, visited map[string]bool, cursor *vertex) error {
	if visited[cursor.name] {
		return nil // Stop if we've already visited this vertex.
	}
	visited[cursor.name] = true
	for _, edge := range cursor.edges {
		if _, err := fmt.Fprintf(out, "\t%q -> %q\n", cursor.name, edge.name); err != nil {
			return err
		}
		if err := g.printDFS(out, visited, edge); err != nil {
			return err
		}
	}

	return nil
}

type breadcrumb struct {
	*vertex             // Current vertex.
	from    *breadcrumb // The vertex we traveled from to get here.
}

// hasCycle reports whether the breadcrumb has a cycle in its chain.
func (b *breadcrumb) hasCycle() bool {
	cursor := &breadcrumb{vertex: b.vertex, from: b.from}
	seen := map[string]bool{cursor.name: true}
	// TODO: we could replace this with map lookups.
	for {
		cursor = cursor.from
		if cursor == nil {
			return false
		}
		if seen[cursor.name] {
			return true
		}
	}
}

// to builds a new graph containing only the paths from g.root to the needle.
func (g *graph) to(needle string) (*graph, error) {
	if _, ok := g.vertices[needle]; !ok {
		return nil, fmt.Errorf("%q does not exist in dependency graph", needle)
	}

	newRoot := newVertex(g.root.name)
	vertexMap := map[string]*vertex{newRoot.name: newRoot}
	q := []*breadcrumb{{vertex: g.root}}

	// Traverse the graph BFS.
	for len(q) > 0 {
		cursor := q[0]
		q = q[1:]

		// If this cursor has already been to this node, we can stop.
		if cursor.hasCycle() {
			continue
		}

		if cursor.name == needle {
			// Last element should always be the connecting piece.
			var path []*vertex

			// Climb up the cursor chain until we get to the root.
			// Note: it's not enough to reach another node that's in the graph
			// 		 already, since this path may contain novel routes earlier
			//		 in the path.
			// TODO: we could probably store the path as we build the cursor
			//		 rather than generating it.
			// TODO: we might be able to avoid going all the way to the root by
			//		 DFS recursively building paths from one connected piece of
			//		 the new graph to any other part of the new graph. (instead
			//		 of strictly looking for the needle) Not sure if this would
			//		 have better time complexity/performance, though.
			for crumbCursor := cursor; crumbCursor != nil; crumbCursor = crumbCursor.from {
				path = append(path, crumbCursor.vertex)
			}

			// Now that we have the full path, let's add the entire path to the
			// new graph, skipping vertices that already exist in the new graph.
			for i := len(path) - 1; i > 0; i-- {
				var ok bool
				var from *vertex

				from, ok = vertexMap[path[i].name]
				if !ok {
					from = newVertex(path[i].name)
					vertexMap[from.name] = from
				}
				var to *vertex
				to, ok = vertexMap[path[i-1].name]
				if !ok {
					to = newVertex(path[i-1].name)
					vertexMap[to.name] = to
				}

				if _, ok := from.edges[to.name]; !ok {
					from.edges[to.name] = to
				}
			}
		}

		// Add nodes to the queue (BFS).
		for _, edge := range cursor.edges {
			q = append(q, &breadcrumb{
				vertex: edge,
				from:   cursor,
			})
		}
	}

	vertices := map[string]bool{}
	for k := range vertexMap {
		vertices[k] = true
	}

	return &graph{root: newRoot, vertices: vertices}, nil
}
