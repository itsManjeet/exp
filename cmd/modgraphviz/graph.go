package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"golang.org/x/exp/cmd/modgraphviz/internal"
)

type graph struct {
	root         *vertex
	vertices     map[string]*vertex
	mvsBuildList []internal.Version
}

type vertex struct {
	name  string
	edges []*vertex
}

// newGraph builds a graph from a Reader that has data formatted in the format:
// A B
// Where A has a directed edge to B.
//
// When simple is true, versions are stripped and ignored.
func newGraph(in io.Reader, simple bool) (*graph, error) {
	vertexMap := map[string]*vertex{}
	var root string
	var rootVersion internal.Version
	var buildList []internal.Version
	buildReq := make(internal.ReqsMap)
	paths := map[string]bool{}

	r := bufio.NewScanner(in)
	for {
		if !r.Scan() {
			if r.Err() != nil {
				return nil, r.Err()
			}
			break
		}

		parts := strings.Fields(r.Text())
		if r.Text() == "" {
			// Ignore empty lines.
			continue
		}
		if len(parts) != 2 {
			return nil, fmt.Errorf("couldn't decipher %s", r.Text())
		}

		fromName := parts[0]
		if simple {
			fromName = strings.Split(fromName, "@")[0]
		}

		fromNameParts := strings.Split(fromName, "@")
		var fromVersion internal.Version
		fromVersion.Path = fromNameParts[0]
		if len(fromNameParts) > 1 {
			fromVersion.Version = fromNameParts[1]
		}

		if root == "" {
			root = fromName
			rootVersion = fromVersion
		}

		toName := parts[1]
		if simple {
			toName = strings.Split(toName, "@")[0]
		}

		toNameParts := strings.Split(toName, "@")
		toVersion := internal.Version{Path: toNameParts[0], Version: toNameParts[1]}

		buildList = append(buildList, fromVersion)
		paths[fromVersion.Path] = true
		buildList = append(buildList, toVersion)
		paths[toVersion.Path] = true
		if req, ok := buildReq[fromVersion]; ok {
			req = append(req, toVersion)
		} else {
			buildReq[fromVersion] = []internal.Version{toVersion}
		}
		if _, ok := buildReq[toVersion]; !ok {
			buildReq[toVersion] = []internal.Version{}
		}

		fromVertex, fromVertexFound := vertexMap[fromName]
		if !fromVertexFound {
			// fromVertex is nil - we couldn't find it. This could happen if
			// the list is unordered: B->C, A->B.
			fromVertex = &vertex{name: fromName, edges: []*vertex{}}
			vertexMap[fromVertex.name] = fromVertex
		}

		toVertex, toVertexFound := vertexMap[toName]
		if !toVertexFound {
			toVertex = &vertex{name: toName, edges: []*vertex{}}
			vertexMap[toVertex.name] = toVertex
		}

		fromVertex.edges = append(fromVertex.edges, toVertex)
	}

	mvsList, err := internal.Req(rootVersion, buildList, keysAsList(paths), buildReq)
	if err != nil {
		return nil, err
	}

	return &graph{
		root:         vertexMap[root],
		vertices:     vertexMap,
		mvsBuildList: mvsList,
	}, nil
}

func keysAsList(in map[string]bool) []string {
	var out []string
	for k := range in {
		out = append(out, k)
	}
	return out
}

// print prints the graph.
func (g *graph) print(out io.Writer, colourBuild bool) error {
	if err := g.printDFS(out, map[string]bool{}, g.root); err != nil {
		return err
	}
	if colourBuild {
		return g.printColours(out)
	}
	return nil
}

// printDFS traverses the graph depth first, printing each edge.
func (g *graph) printDFS(out io.Writer, visited map[string]bool, cursor *vertex) error {
	if _, ok := visited[cursor.name]; ok {
		return nil // Stop if we've already visited this vertex.
	}
	visited[cursor.name] = true
	for _, conn := range cursor.edges {
		if _, err := fmt.Fprintf(out, "\t%q -> %q\n", cursor.name, conn.name); err != nil {
			return err
		}
		if err := g.printDFS(out, visited, conn); err != nil {
			return err
		}
	}

	return nil
}

func (g *graph) printColours(out io.Writer) error {
	for _, v := range g.mvsBuildList {
		var reconstructedName = v.Path
		if v.Version != "" {
			reconstructedName += "@" + v.Version
		}
		if _, err := fmt.Fprintf(out, "\t%q [style = filled, fillcolor = green]\n", reconstructedName); err != nil {
			return err
		}
	}
	return nil
}

type breadcrumb struct {
	*vertex             // Current vertex.
	from    *breadcrumb // The vertex we traveled from to get here.
}

// printPathsTo prints the paths from g.root to the needle.
func (g *graph) printPathsTo(out io.Writer, needle string) error {
	if _, ok := g.vertices[needle]; !ok {
		return fmt.Errorf("%q does not exist in dependency graph", needle)
	}
	return g.printPathToDFS(out, map[string]bool{}, map[string]bool{}, &breadcrumb{vertex: g.root}, needle)
}

// printPathToDFS traverses the graph depth first, finding all the paths to
// needle and printing them (avoiding duplicate edges).
func (g *graph) printPathToDFS(out io.Writer, printed, visited map[string]bool, cursor *breadcrumb, needle string) error {
	if cursor.name == needle {
		// We've found the needle! Now let's build the path up for it.
		var path []string
		cursor := cursor
		for {
			// We've reached the top!
			if cursor.from == nil {
				break
			}

			s := fmt.Sprintf("\t%q -> %q\n", cursor.from.name, cursor.name)

			// If we've already backtracked and printed this edge, there's no
			// point going higher: all the path up from here will be captured
			// already.
			if _, ok := printed[s]; ok {
				break
			}

			printed[s] = true
			path = append(path, s)

			// We don't need to write the from field since we won't be
			// backtracking from our backtracking.
			cursor = cursor.from
		}

		// Print the path in reverse. This is not strictly required for
		// graphviz, but it does make the output more comprehensible.
		for i := len(path) - 1; i >= 0; i-- {
			if _, err := fmt.Fprintf(out, path[i]); err != nil {
				return err
			}
		}
	}

	if _, ok := visited[cursor.name]; ok {
		return nil // Stop if we've already visited this vertex.
	}
	visited[cursor.name] = true
	for _, v := range cursor.edges {
		if err := g.printPathToDFS(out, printed, visited, &breadcrumb{vertex: v, from: cursor}, needle); err != nil {
			return err
		}
	}
	return nil
}
