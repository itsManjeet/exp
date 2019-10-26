// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Linguini (lean) displays “go mod graph” output as an interactable graph. lean
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

var (
	originalGraph = make(map[string]map[string]struct{})
	graph         = make(map[string]map[string]struct{})
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
			originalGraph[from] = make(map[string]struct{})
		}
		originalGraph[from][to] = struct{}{}

		// `go mod graph` always presents the root as the first "from" node
		if root == "" {
			root = from
		}
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	copyGraph(originalGraph, graph)

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
		copyGraph(originalGraph, graph)
		m := make(map[string]interface{})
		shoppingCart = make(map[string]map[string]struct{})
		m["graph"] = graph
		m["shoppingCart"] = shoppingCart
		if err := json.NewEncoder(w).Encode(m); err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	http.HandleFunc("/graph", func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewEncoder(w).Encode(connected(graph, root)); err != nil {
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
			delete(graph[from], to)
		} else if r.Method == http.MethodPost {
			if _, ok := graph[from]; !ok {
				graph[from] = make(map[string]struct{})
			}
			graph[from][to] = struct{}{}
			delete(shoppingCart[from], to)
		}

		m := make(map[string]interface{})
		m["graph"] = connected(graph, root)
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

func copyGraph(src, dst map[string]map[string]struct{}) {
	// Empty dst.
	for k := range dst {
		delete(dst, k)
	}
	// Fill dst.
	for k, vs := range src {
		dst[k] = make(map[string]struct{})
		for kk, v := range vs {
			dst[k][kk] = v
		}
	}
}

// connected returns the subgraph that is reachable from root.
func connected(g map[string]map[string]struct{}, root string) map[string]map[string]struct{} {
	sub := make(map[string]map[string]struct{})
	seen := make(map[string]map[string]struct{})
	var dfs func(from string)
	dfs = func(from string) {
		if _, ok := g[from]; !ok {
			return
		}
		if _, ok := seen[from]; !ok {
			seen[from] = make(map[string]struct{})
		}
		for to := range g[from] {
			if _, ok := seen[from][to]; ok {
				return
			}
			seen[from][to] = struct{}{}
			if _, ok := sub[from]; !ok {
				sub[from] = make(map[string]struct{})
			}
			sub[from][to] = struct{}{}
			dfs(to)
		}
	}
	dfs(root)
	return sub
}
