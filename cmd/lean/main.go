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
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

var (
	mu            sync.Mutex
	originalGraph *graph
	userGraph     *graph
	shoppingCart  = make(map[string]map[string]struct{})
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

	mu.Lock()
	var err error
	originalGraph, err = newGraph(os.Stdin)
	if err != nil {
		log.Fatal(err)
	}
	userGraph = originalGraph.copy()
	mu.Unlock()

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
		mu.Lock()
		defer mu.Unlock()

		userGraph = originalGraph.copy()
		shoppingCart = make(map[string]map[string]struct{})

		out := make(map[string]interface{})
		out["graph"] = userGraph.edges
		out["shoppingCart"] = shoppingCart
		if err := json.NewEncoder(w).Encode(out); err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	http.HandleFunc("/graph", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		if err := json.NewEncoder(w).Encode(userGraph.connected(userGraph.root)); err != nil {
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

		mu.Lock()
		defer mu.Unlock()

		if r.Method == http.MethodDelete {
			if _, ok := shoppingCart[from]; !ok {
				shoppingCart[from] = make(map[string]struct{})
			}
			shoppingCart[from][to] = struct{}{}
			if err := userGraph.removeEdge(from, to); err != nil {
				log.Println(err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		} else if r.Method == http.MethodPost {
			if _, ok := shoppingCart[from]; ok {
				if _, ok := shoppingCart[from][to]; ok {
					delete(shoppingCart[from], to)
				}
			}
			if err := userGraph.addEdge(from, to); err != nil {
				log.Println(err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		}

		m := make(map[string]interface{})
		m["graph"] = userGraph.connected(userGraph.root)
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
