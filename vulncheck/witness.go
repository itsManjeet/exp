package vulncheck

import "container/list"

// ImportChain is sequence of import paths starting with
// a client package and ending with a package with some
// known vulnerabilities.
type ImportChain []*PkgNode

// ImportChains performs a BFS search of res.RequireGraph for imports of vulnerable
// packages. Search is performed for each vulnerable symbol in res.Vulns, i.e.,
// its package. The search starts at a vulnerable package and goes up until
// reaching an element in res.ImportGraph.Entries, hence producing an import chain.
// During the search for a vulnerable package, a package is visited only once to
// avoid exponential explosion. Hence, not all possible import chains are reported.
// Note that the resulting map produces an import chain for each Vuln. Thus, a Vuln
// with the same PkgPath will have the same list of identified import chains.
//
// The reported import chains are ordered by how seemingly easy is to understand
// them. Shorter import chains appear earlier in the returned import chains slices.
func ImportChains(res *Result) map[*Vuln][]ImportChain {
	// Collect vulns per package (sink ID)
	vPerPkg := make(map[int][]*Vuln)
	for _, v := range res.Vulns {
		vPerPkg[v.ImportSink] = append(vPerPkg[v.ImportSink], v)
	}

	chainsPerPkg := make(map[int]chan []ImportChain)
	for pkg := range vPerPkg {
		p := pkg
		chainsPerPkg[p] = make(chan []ImportChain)
		go func() {
			chainsPerPkg[p] <- importChains(p, res)
		}()
	}

	chains := make(map[*Vuln][]ImportChain)
	for pkg, ch := range chainsPerPkg {
		out := <-ch
		for _, v := range vPerPkg[pkg] {
			chains[v] = out
		}
	}
	return chains
}

// importChains finds representative chains of package imports
// for vulnerabiity identified with vulnSinkID.
func importChains(vulnSinkID int, res *Result) []ImportChain {
	if vulnSinkID == 0 {
		return nil
	}

	entries := make(map[int]bool)
	for _, e := range res.Imports.Entries {
		entries[e] = true
	}

	var chains []ImportChain
	seen := make(map[int]bool)

	queue := list.New()
	queue.PushBack(&importChain{pkg: res.Imports.Packages[vulnSinkID]})

	for queue.Len() > 0 {
		front := queue.Front()
		c := front.Value.(*importChain)
		queue.Remove(front)

		pkg := c.pkg
		if seen[pkg.ID] {
			continue
		}
		seen[pkg.ID] = true

		for _, impBy := range pkg.ImportedBy {
			imp := res.Imports.Packages[impBy]
			newC := &importChain{pkg: imp, child: c}
			if entries[imp.ID] {
				chains = append(chains, newC.ImportChain())
			}

			queue.PushBack(newC)
		}
	}
	return chains
}

// importChain models an chain of package imports. Used during
// the search as alternative to ImportChain to improve performance,
// i.e., to avoid cost of slice resizing.
type importChain struct {
	pkg   *PkgNode
	child *importChain
}

// ImportChain converts importChain to ImportChain (type).
func (r *importChain) ImportChain() ImportChain {
	if r == nil {
		return nil
	}
	return append([]*PkgNode{r.pkg}, r.child.ImportChain()...)
}

// CallStack models a trace of function calls starting
// with a client function or method and ending with a
// call to a vulnerable symbol.
type CallStack []StackEntry

// StackEntry models an element of a call stack.
type StackEntry struct {
	// Function provides information on the function whose frame is on the stack.
	Function *FuncNode

	// Call provides information on the call site inducing this stack frame.
	// nil when the frame represents an entry point of the stack.
	Call *CallSite
}
