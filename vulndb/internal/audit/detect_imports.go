// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package audit

import (
	"container/list"
	"fmt"
	"go/types"
	"runtime"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa/ssautil"
)

// VulnerableImports returns vulnerability findings for packages imported by `pkgs`
// given the vulnerabilities provided by database `client`.
//
// Returns all findings reachable from `pkgs` while analyzing each package only once,
// prefering findings of shorter import traces. For instance, given import chains
//   A -> B -> V
//   A -> D -> B -> V
//   D -> B -> V
// where A and D are top level packages and V is a vulnerable package, VulnerableImports
// can return either
//   A -> B -> V
// or
//   D -> B -> V
// as traces of importing a vulnerable package V.
//
// Findings for each vulnerability are sorted by their estimated usefulness to the user.
func VulnerableImports(pkgs []*packages.Package, client DbClient) (*Results, error) {
	results := &Results{SearchMode: ImportsSearch}

	modules := extractModules(pkgs)
	modVulns, err := fetchVulnerabilities(client, modules)
	if err != nil {
		return nil, err
	}
	modVulns = modVulns.Filter(runtime.GOOS, runtime.GOARCH)
	if len(modVulns) == 0 {
		return results, nil
	}

	prog, ssaPkgs := ssautil.AllPackages(pkgs, 0)
	prog.Build()
	if prog == nil {
		return nil, fmt.Errorf("failed to build internal ssa representation of pkgs")
	}

	seen := make(map[string]bool)
	queue := list.New()
	for _, pkg := range ssaPkgs {
		queue.PushBack(&importChain{pkg: pkg.Pkg})
	}

	for queue.Len() > 0 {
		front := queue.Front()
		c := front.Value.(*importChain)
		queue.Remove(front)

		pkg := c.pkg
		if pkg == nil {
			continue
		}

		if seen[pkg.Path()] {
			continue
		}
		seen[pkg.Path()] = true

		for _, imp := range pkg.Imports() {
			vulns := modVulns.VulnsForPackage(imp.Path())
			for _, v := range vulns {
				results.addFinding(v, Finding{
					Symbol: imp.Path(),
					Type:   ImportType,
					Trace:  c.trace(),
					weight: len(c.trace())})
			}
			queue.PushBack(&importChain{pkg: imp, parent: c})
		}
	}

	results.sort()
	return results, nil
}

// importChain helps doing BFS over package imports while remembering import chains.
type importChain struct {
	pkg    *types.Package
	parent *importChain
}

func (chain *importChain) trace() []TraceElem {
	if chain == nil {
		return nil
	}
	return append(chain.parent.trace(), TraceElem{Description: chain.pkg.Path()})
}
