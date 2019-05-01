// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The modgraphviz command translates the output for go mod graph into .dot
// notation, which can then be parsed by graphviz into visual graphs.
//
// Requires graphviz and the graphviz dot cli to be installed.
//
// Usage: GO111MODULE=on go mod graph | modgraphviz [-simple] [-pathsTo] [--colourBuild] | dot -Tpng -o outfile.png
package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
)

var pathsTo = flag.String("pathsTo", "", "Only show the graph of the path(s) to a module. ex: -pathsTo foo.com/bar@1.2.3")
var simple = flag.Bool("simple", false, "Only show the modules without their versions.")
var colourBuild = flag.Bool("colourBuild", false, "Colours green the versions picked by MVS.")

func main() {
	flag.Usage = func() {
		fmt.Println(`modgraphviz [-simple] [-pathsTo]

Example usage:
	GO111MODULE=on go mod graph | \
	modgraphviz [-simple] [-pathsTo] | \
	dot -Tpng -o outfile.png

modgraphviz prints the mod graph in a graphviz format. Graphviz and the graphviz
dot CLI should be installed.

-simple strips module versions and only prints module names.

-pathsTo prints the paths from the root module to the specified module. pathsTo
accepts a string: ex -pathsTo foo.com/bar@1.2.3.

-colourBuild colours the versions picked by MVS as green. Note that when used
with -simple, the minimal build list (sans versions) are already specified,
so every node is green.`)
	}
	flag.Parse()

	var out bytes.Buffer

	g, err := newGraph(os.Stdin, *simple)
	if err != nil {
		log.Fatal(err)
	}

	if _, err := out.Write([]byte("digraph gomodgraph {\n")); err != nil {
		log.Fatal(err)
	}

	if *pathsTo != "" {
		// TODO(deklerk) --colourBuild currently has to be implemented in both
		// print and printPathsTo. Probably this is a good signal that pathsTo
		// should just return a graph that we call print on.
		if err := g.printPathsTo(&out, *pathsTo); err != nil {
			log.Fatal(err)
		}
	} else {
		if err := g.print(&out, *colourBuild); err != nil {
			log.Fatal(err)
		}
	}

	if _, err := out.Write([]byte("}\n")); err != nil {
		log.Fatal(err)
	}

	if _, err := out.WriteTo(os.Stdout); err != nil {
		log.Fatal(err)
	}
}
