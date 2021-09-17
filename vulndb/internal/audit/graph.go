package audit

import (
	"fmt"

	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/ssa"
)

func vulnSlice(funcs map[*ssa.Function]bool, entries []*ssa.Function, cg *callgraph.Graph, modVulns ModuleVulnerabilities) {
	fcs := forwardCallSites(entries, cg)
	sinks := vulnSinks(funcs, modVulns)
	fmt.Printf("Total number of sinks: %v and entries: %v\n", len(sinks), len(entries))

	allSinksSlice := computeSlice(sinks, fcs, cg)
	cSize, fSize, eSize := sliceSizes(allSinksSlice, entries)
	fmt.Printf("AllSinks slice size in # of callsites: %v   # of functions: %v   # of app. entries: %v\n", cSize, fSize, eSize)

	sinkSlices := make(map[*ssa.Function]map[ssa.CallInstruction]bool)
	for sink := range sinks {
		sinkSlice := computeSlice(map[*ssa.Function]bool{sink: true}, fcs, cg)
		cSize, fSize, eSize := sliceSizes(sinkSlice, entries)
		fmt.Printf("\tsink %v slice size in # of callsites: %v   # of functions: %v   # of app. entries: %v\n", sink, cSize, fSize, eSize)
		sinkSlices[sink] = sinkSlice
	}

	copies := make(map[ssa.CallInstruction]int)
	for _, slice := range sinkSlices {
		for cs := range slice {
			copies[cs] += 1
		}
	}

	copySum := 0
	for _, cnt := range copies {
		copySum += cnt - 1
	}
	fmt.Printf("Total number of potentially copied callsites: %v\n\n", copySum)
}

func computeSlice(sinks map[*ssa.Function]bool, fslice map[ssa.CallInstruction]bool, cg *callgraph.Graph) map[ssa.CallInstruction]bool {
	bcs := backwardCallSites(sinks, cg)

	m := make(map[ssa.CallInstruction]bool)
	for c := range fslice {
		if bcs[c] {
			m[c] = true
		}
	}
	return m
}

func sliceSizes(slice map[ssa.CallInstruction]bool, entries []*ssa.Function) (int, int, int) {
	f := make(map[*ssa.Function]bool)
	for cs := range slice {
		f[cs.Parent()] = true
	}

	entryCnt := 0
	for _, e := range entries {
		if f[e] {
			entryCnt += 1
		}
	}
	return len(slice), len(f), entryCnt
}

func forwardCallSites(sources []*ssa.Function, cg *callgraph.Graph) map[ssa.CallInstruction]bool {
	m := make(map[*ssa.Function]bool)
	callSites := make(map[ssa.CallInstruction]bool)
	for _, s := range sources {
		forwardCall(s, cg, m, callSites)
	}
	return callSites
}

func forwardCall(f *ssa.Function, cg *callgraph.Graph, seen map[*ssa.Function]bool, callSites map[ssa.CallInstruction]bool) {
	if _, ok := seen[f]; ok {
		return
	}
	seen[f] = true
	var buf [10]*ssa.Value // avoid alloc in common case
	for _, b := range f.Blocks {
		for _, instr := range b.Instrs {
			switch i := instr.(type) {
			case ssa.CallInstruction:
				callSites[i] = true
				for _, c := range siteCallees(i, cg) {
					forwardCall(c, cg, seen, callSites)
				}
			default:
				for _, op := range i.Operands(buf[:0]) {
					if fn, ok := (*op).(*ssa.Function); ok {
						forwardCall(fn, cg, seen, callSites)
					}
				}
			}
		}
	}
}

func backwardCallSites(sinks map[*ssa.Function]bool, cg *callgraph.Graph) map[ssa.CallInstruction]bool {
	callSites := make(map[ssa.CallInstruction]bool)
	seen := make(map[*ssa.Function]bool)
	for {
		new := make(map[*ssa.Function]bool)
		for f := range sinks {
			if _, ok := seen[f]; ok {
				continue
			}
			seen[f] = true

			node := cg.Nodes[f]
			if node == nil {
				continue
			}

			for _, edge := range node.In {
				new[edge.Caller.Func] = true
				callSites[edge.Site] = true
			}
		}

		if len(new) == 0 {
			break
		}
		sinks = new
	}
	return callSites
}

func vulnSinks(slice map[*ssa.Function]bool, modVulns ModuleVulnerabilities) map[*ssa.Function]bool {
	vps := vulnSymbols(modVulns)
	sinks := make(map[*ssa.Function]bool)
	for f := range slice {
		fname := dbFuncName(f)
		if f.Pkg == nil || f.Pkg.Pkg == nil {
			continue
		}
		pkgPath := f.Pkg.Pkg.Path()
		fs, ok := vps[pkgPath]
		if !ok {
			continue
		}
		if _, ok := fs["*"]; ok {
			sinks[f] = true
		}

		if _, ok := fs[fname]; ok {
			sinks[f] = true
		}
	}

	return sinks
}

func vulnSymbols(modVulns ModuleVulnerabilities) map[string]map[string]bool {
	m := make(map[string]map[string]bool)
	for _, v := range modVulns.Vulns() {
		for _, a := range v.Affected {
			fs, ok := m[a.Package.Name]
			if !ok {
				fs = make(map[string]bool)
				m[a.Package.Name] = fs
			}

			if len(a.EcosystemSpecific.Symbols) == 0 {
				fs["*"] = true
			}

			for _, f := range a.EcosystemSpecific.Symbols {
				fs[f] = true
			}
		}
	}
	return m
}
