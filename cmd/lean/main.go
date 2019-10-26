// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Linguini (lean) displays "go mod graph" output as an interactable graph. lean
// allows you to see how your dependencies would change if you deleted one or
// more edges.

// While existing tools (e.g. golang.org/x/cmd/digraph) are good at answering
// questions about the actual graph, they're not so helpful with the kind of
// hypothetical questions you have when you're about to undertake a refactoring.
// Linguini can help you see the scope of the work ahead of you.
//
// Usage:
//
//	go mod graph | lean
//	go mod graph | digraph transpose | lean
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type graph map[string]map[string]*edge

type vertex struct {
	label     string
	sizeBytes int64
}

type edge struct {
	dependents []vertex
	// Number of times that from uses to.
	numUsages int64
}

func (e *edge) dependentsSizeBytes() int64 {
	var b int64
	for _, v := range e.dependents {
		b += v.sizeBytes
	}
	return b
}

var (
	vertices      = make(map[string]vertex)
	originalGraph = make(graph)
	malGraph      = make(graph) // malleable graph TODO(deklerk): better name?
	shoppingCart  = make(map[string]map[string]struct{})
	root          string
)

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: go mod graph | lean")
	os.Exit(2)
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("lean: ")

	flag.Usage = usage
	flag.Parse()
	if flag.NArg() != 0 {
		usage()
	}

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		l := scanner.Text()
		if l == "" {
			continue
		}
		parts := strings.Fields(l)
		if len(parts) != 2 {
			log.Fatalf("expected 2 words in line, but got %d: %s", len(parts), l)
		}
		from := parts[0]
		to := parts[1]
		if _, ok := originalGraph[from]; !ok {
			originalGraph[from] = make(map[string]*edge)
		}
		originalGraph[from][to] = &edge{}

		// `go mod graph` always presents the root as the first "from" node
		if root == "" {
			root = from
		}
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	copyGraph(originalGraph, malGraph)

	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		tmpl, err := template.ParseFiles(filepath.Join("static", "index.html"))
		if err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := tmpl.ExecuteTemplate(w, "page", nil); err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	http.HandleFunc("/reset", func(w http.ResponseWriter, r *http.Request) {
		copyGraph(originalGraph, malGraph)
		m := make(map[string]interface{})
		shoppingCart = make(map[string]map[string]struct{})
		m["graph"] = malGraph
		m["shoppingCart"] = shoppingCart
		if err := json.NewEncoder(w).Encode(m); err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	http.HandleFunc("/graph", func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewEncoder(w).Encode(connected(malGraph, root)); err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	http.HandleFunc("/shoppingCart", func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewEncoder(w).Encode(shoppingCart); err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	http.HandleFunc("/edge", func(w http.ResponseWriter, r *http.Request) {
		var in map[string]string
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		from := in["from"]
		to := in["to"]

		if r.Method == http.MethodDelete {
			if _, ok := shoppingCart[from]; !ok {
				shoppingCart[from] = make(map[string]struct{})
			}
			shoppingCart[from][to] = struct{}{}
			delete(malGraph[from], to)
		} else if r.Method == http.MethodPost {
			if _, ok := malGraph[from]; !ok {
				malGraph[from] = make(map[string]*edge)
			}
			malGraph[from][to] = &edge{}
			delete(shoppingCart[from], to)
		}

		m := make(map[string]interface{})
		m["graph"] = connected(malGraph, root)
		m["shoppingCart"] = shoppingCart
		if err := json.NewEncoder(w).Encode(m); err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	log.Println("Listening on :3000")
	log.Fatal(http.ListenAndServe(":3000", nil))
}

// copyGraph copies src into dst.
func copyGraph(src, dst graph) {
	// Empty dst.
	for k := range dst {
		delete(dst, k)
	}
	// Fill dst.
	for k, vs := range src {
		dst[k] = make(map[string]*edge)
		for kk, v := range vs {
			dst[k][kk] = v
		}
	}
}

// connected returns the subgraph that is reachable from root.
func connected(g graph, root string) graph {
	sub := make(graph)
	seen := make(graph)
	var dfs func(from string)
	dfs = func(from string) {
		if _, ok := g[from]; !ok {
			return
		}
		if _, ok := seen[from]; !ok {
			seen[from] = make(map[string]*edge)
		}
		for to := range g[from] {
			if _, ok := seen[from][to]; ok {
				return
			}
			seen[from][to] = &edge{}
			if _, ok := sub[from]; !ok {
				sub[from] = make(map[string]*edge)
			}
			sub[from][to] = &edge{}
			dfs(to)
		}
	}
	dfs(root)
	return sub
}
