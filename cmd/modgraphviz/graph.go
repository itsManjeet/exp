package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

type graph struct {
	root  *node
	nodes map[string]*node
}

type node struct {
	name        string
	connections []*node
	cameFrom    *node
}

func newGraph(in io.Reader) (*graph, error) {
	nodeMap := map[string]*node{}
	var rootDep string
	r := bufio.NewScanner(in)
	for {
		if !r.Scan() {
			if r.Err() != nil {
				return nil, r.Err()
			}
			break
		}

		parts := strings.Fields(r.Text())
		if len(parts) != 2 {
			// TODO should probably be a panic? why wouldn't there be 2 parts?
			continue
		}

		fromName := parts[0]
		if rootDep == "" {
			rootDep = fromName
		}

		toName := parts[1]
		fromNode, fromNodeFound := nodeMap[fromName]
		if !fromNodeFound {
			// fromNode is nil - we couldn't find it
			fromNode = &node{name: fromName, connections: []*node{}}
			nodeMap[fromNode.name] = fromNode
		}

		toNode, toNodeFound := nodeMap[toName]
		if !toNodeFound {
			toNode = &node{name: toName, connections: []*node{}}
			nodeMap[toNode.name] = toNode
		}

		fromNode.connections = append(fromNode.connections, toNode)
	}

	return &graph{
		root:  nodeMap[rootDep],
		nodes: map[string]*node{},
	}, nil
}

func (g *graph) print(out io.Writer, visitedNodes map[string]bool, currentNode *node) error {
	if visitedNodes == nil {
		visitedNodes = map[string]bool{}
	}
	if currentNode == nil {
		currentNode = g.root
	}

	if _, ok := visitedNodes[currentNode.name]; ok {
		return nil // if we have already visited this currentNode stop
	}
	visitedNodes[currentNode.name] = true
	for _, conn := range currentNode.connections {
		if _, err := fmt.Fprintf(out, "\t%q -> %q\n", currentNode.name, conn.name); err != nil {
			return err
		}
		if err := g.print(out, visitedNodes, conn); err != nil {
			return err
		}
	}

	return nil
}

func (g *graph) printPathTo(out io.Writer, printedNodes, visitedNodes map[string]bool, currentNode *node, needle string) error {
	if _, ok := g.nodes[needle]; !ok {
		return fmt.Errorf("%q does not exist in dependency graph", needle)
	}
	if visitedNodes == nil {
		visitedNodes = map[string]bool{}
	}
	if printedNodes == nil {
		printedNodes = map[string]bool{}
	}
	if currentNode == nil {
		currentNode = g.root
	}

	if currentNode.name == needle {
		var path []string
		cursor := currentNode
		for {
			if cursor.cameFrom == nil {
				break
			} else {
				s := fmt.Sprintf("\t%q -> %q\n", cursor.cameFrom.name, cursor.name)
				if _, ok := printedNodes[s]; ok {
					break
				}

				printedNodes[s] = true
				path = append(path, s)
				cursor = cursor.cameFrom

			}
		}
		for i := len(path) - 1; i >= 0; i-- {
			if _, err := fmt.Fprintf(out, path[i]); err != nil {
				return err
			}
		}
	}

	if _, ok := visitedNodes[currentNode.name]; ok {
		return nil // if we have already visited this currentNode stop
	}
	visitedNodes[currentNode.name] = true
	for _, conn := range currentNode.connections {
		conn.cameFrom = currentNode
		if err := g.printPathTo(out, printedNodes, visitedNodes, conn, needle); err != nil {
			return err
		}
	}
	return nil
}
