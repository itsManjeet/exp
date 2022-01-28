package vulncheck

import (
	"container/list"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"

	"golang.org/x/tools/go/packages"
)

// ImportChain is sequence of import paths starting with
// a client package and ending with a package with some
// known vulnerabilities.
type ImportChain []*PkgNode

// ImportChains performs a BFS search of res.RequireGraph for imports of vulnerable
// packages. Search is performed for each vulnerable package in res.Vulns. The search
// starts at a vulnerable package and goes up until reaching an entry package in
// res.ImportGraph.Entries, hence producing an import chain. During the search, a
// package is visited only once to avoid analyzing every possible import chain.
// Hence, not all possible vulnerable import chains are reported.
//
// Note that the resulting map produces an import chain for each Vuln. Thus, a Vuln
// with the same PkgPath will have the same list of identified import chains.
//
// The reported import chains are ordered by how seemingly easy is to understand
// them. Shorter import chains appear earlier in the returned slices.
func ImportChains(res *Result) map[*Vuln][]ImportChain {
	// Group vulns per package.
	vPerPkg := make(map[int][]*Vuln)
	for _, v := range res.Vulns {
		vPerPkg[v.ImportSink] = append(vPerPkg[v.ImportSink], v)
	}

	// Collect chains in parallel for every package path.
	var wg sync.WaitGroup
	var mu sync.Mutex
	chains := make(map[*Vuln][]ImportChain)
	for pkgID, vulns := range vPerPkg {
		pID := pkgID
		vs := vulns
		wg.Add(1)
		go func() {
			pChains := importChains(pID, res)
			mu.Lock()
			for _, v := range vs {
				chains[v] = pChains
			}
			mu.Unlock()
			wg.Done()
		}()
	}
	wg.Wait()
	return chains
}

// importChains finds representative chains of package imports
// leading to vulnerable package identified with vulnSinkID.
func importChains(vulnSinkID int, res *Result) []ImportChain {
	if vulnSinkID == 0 {
		return nil
	}

	// Entry packages, needed for finalizing chains.
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
			// If the next package is an entry, we have
			// a chain to report.
			if entries[imp.ID] {
				chains = append(chains, newC.ImportChain())
			}
			queue.PushBack(newC)
		}
	}
	return chains
}

// importChain models an chain of package imports.
type importChain struct {
	pkg   *PkgNode
	child *importChain
}

// ImportChain converts importChain to ImportChain type.
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

// ImportChains performs a BFS search of res.RequireGraph for imports of vulnerable
// packages. Search is performed for each vulnerable package in res.Vulns. The search
// starts at a vulnerable package and goes up until reaching an entry package in
// res.ImportGraph.Entries, hence producing an import chain. During the search, a
// package is visited only once to avoid analyzing every possible import chain.
// Hence, not all possible vulnerable import chains are reported.
//
// Note that the resulting map produces an import chain for each Vuln. Thus, a Vuln
// with the same PkgPath will have the same list of identified import chains.
//
// The reported import chains are ordered by how seemingly easy is to understand
// them. Shorter import chains appear earlier in the returned slices.

// CallStacks performs a BFS search of res.CallGraph for calls to vulnerable
// symbols. Search is performed for each vulnerable symbol in res.Vulns. The
// search starts at the vulnerable symbol and goes up until reaching an entry
// function or method in res.CallGraph.Entries, hence producing a call stack.
// During the search for a symbol, each function is visited at most once to avoid
// potential exponential explosion. Hence, not all possible call stacks are reported.
//
// The reported call stacks are ordered by how seemingly easy is to understand
// them. In general, shorter call stacks with less dynamic call sites appear
// earlier in the returned call stack slices.
func CallStacks(res *Result) map[*Vuln][]CallStack {
	var wg sync.WaitGroup
	var mu sync.Mutex
	stacksPerVuln := make(map[*Vuln][]CallStack)
	for _, vuln := range res.Vulns {
		v := vuln
		wg.Add(1)
		go func() {
			cs := callStacks(v.CallSink, res)
			// sort call stacks by the estimated value to the user
			sort.SliceStable(cs, func(i int, j int) bool { return stackCompare(cs[i], cs[j]) })
			mu.Lock()
			stacksPerVuln[v] = cs
			mu.Unlock()
			wg.Done()
		}()
	}

	wg.Wait()
	return stacksPerVuln
}

// callStacks finds representative call stacks
// for vulnerable symbol identified with vulnSinkID.
func callStacks(vulnSinkID int, res *Result) []CallStack {
	if vulnSinkID == 0 {
		return nil
	}

	entries := make(map[int]bool)
	for _, e := range res.Calls.Entries {
		entries[e] = true
	}

	var stacks []CallStack
	seen := make(map[int]bool)

	queue := list.New()
	queue.PushBack(&callChain{f: res.Calls.Functions[vulnSinkID]})

	for queue.Len() > 0 {
		front := queue.Front()
		c := front.Value.(*callChain)
		queue.Remove(front)

		f := c.f
		if seen[f.ID] {
			continue
		}
		seen[f.ID] = true

		for _, cs := range f.CallSites {
			callee := res.Calls.Functions[cs.Parent]
			newS := &callChain{f: callee, call: cs, child: c}
			if entries[callee.ID] {
				stacks = append(stacks, newS.CallStack())
			}

			queue.PushBack(newS)
		}
	}
	return stacks
}

// callChain models an chain of function calls.
type callChain struct {
	call  *CallSite
	f     *FuncNode
	child *callChain
}

// CallStack converts callChain to CallStack type.
func (c *callChain) CallStack() CallStack {
	if c == nil {
		return nil
	}
	return append(CallStack{StackEntry{Function: c.f, Call: c.call}}, c.child.CallStack()...)
}

// weight computes an approximate measure of how easy is to understand the call
// stack when presented to the client as a witness. The smaller the value, the more
// understendeable the stack is. Currently defined as the number of unresolved
// call sites in the stack.
func weight(stack CallStack) int {
	w := 0
	for _, e := range stack {
		if e.Call != nil && !e.Call.Resolved {
			w += 1
		}
	}
	return w
}

// for assesing confidence level of call stacks.
var stdPackages = make(map[string]bool)
var loadStdsOnce sync.Once

func isStdPackage(pkg string) bool {
	loadStdsOnce.Do(func() {
		pkgs, err := packages.Load(nil, "std")
		if err != nil {
			log.Printf("warning: unable to fetch list of std packages, call stack accuracy might be affected: %v", err)
		}

		for _, p := range pkgs {
			stdPackages[p.PkgPath] = true
		}
	})
	return stdPackages[pkg]
}

// confidence computes an approximate measure of whether the stack
// is realizeable in practice. Currently, it equals the number of call
// sites in stack that go through standard libraries. Such call stacks
// have been experimentally shown to often result in false positives.
func confidence(stack CallStack) int {
	c := 0
	for _, e := range stack {
		if isStdPackage(e.Function.PkgPath) {
			c += 1
		}
	}
	return c
}

// stackCompare compares two call stacks in terms of their estimated
// value to the user. Shorter stacks generally come earlier in the ordering.
//
// Two stacks are lexicographically ordered by:
// 1) their estimated level of confidence in being a real call stack,
// 2) their length, and 3) the number of dynamic call sites in the stack.
func stackCompare(s1, s2 CallStack) bool {
	if confidence(s1) != confidence(s2) {
		return confidence(s1) < confidence(s2)
	}

	if len(s1) != len(s2) {
		return len(s1) < len(s2)
	}

	if weight(s1) != weight(s2) {
		return weight(s1) < weight(s2)
	}
	// At this point we just need to make sure the ordering is deterministic.
	// TODO(zpavlinovic): is there a more meaningful additional ordering?
	return stackStrCompare(s1, s2)
}

// stackStrCompare compares string representation of stacks.
func stackStrCompare(s1, s2 CallStack) bool {
	// Creates a unique string representation of a call stack
	// for comparison purposes only.
	stackStr := func(s CallStack) string {
		var sStr []string
		for _, cs := range s {
			if cs.Call == nil {
				sStr = append(sStr, cs.Function.String())
			} else {
				sStr = append(sStr, fmt.Sprintf("%s[%s]", cs.Function.String(), cs.Call.Pos))
			}
		}
		return strings.Join(sStr, "->")
	}
	return strings.Compare(stackStr(s1), stackStr(s2)) <= 0
}
