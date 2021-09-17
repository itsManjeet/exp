package audit

import (
	"fmt"
	"go/token"
	"log"

	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
)

type CallGraph struct {
	Funcs   map[uint]*FuncNode
	Entries []*FuncNode
}

type FuncNode struct {
	ID        uint
	Name      string
	RecvType  string // full type path of the receiver object, if any
	PkgPath   string
	Pos       *token.Position
	CallSites []CallSite
}

type CallSite struct {
	Parent   uint
	Name     string
	RecvType string
	Pos      *token.Position
	Resolved bool
	Edges    []Edge
}

type Edge struct {
	ID uint
}

var id uint = 0

func freshId() uint {
	i := id
	id += 1
	return i
}

func createGraph(slice cgSlice, entries []*ssa.Function, cg *callgraph.Graph) *CallGraph {
	fMap := make(map[*ssa.Function][]ssa.CallInstruction)
	for cs := range slice {
		f := cs.Parent()
		fMap[f] = append(fMap[f], cs)
	}

	graph := &CallGraph{
		Funcs: make(map[uint]*FuncNode),
	}

	funcToNode := make(map[*ssa.Function]*FuncNode)
	for f, css := range fMap {
		fNode := getOrCreateNode(f, funcToNode)
		graph.Funcs[fNode.ID] = fNode
		for _, cs := range css {
			callSite := createCallSite(cs, fNode)
			for _, callee := range siteCallees(cs, cg) {
				cNode := getOrCreateNode(callee, funcToNode)
				callSite.Edges = append(callSite.Edges, Edge{ID: cNode.ID})
			}
			fNode.CallSites = append(fNode.CallSites, callSite)
		}
	}

	for _, e := range entries {
		eNode := getOrCreateNode(e, funcToNode)
		graph.Entries = append(graph.Entries, eNode)
	}

	return graph
}

func createCallSite(call ssa.CallInstruction, fNode *FuncNode) CallSite {
	cs := CallSite{
		Parent:   fNode.ID,
		Resolved: !unresolved(call),
		Pos:      instrPosition(call),
	}

	cs.Name = call.Common().Value.Name()
	if !cs.Resolved {
		cs.RecvType = typeString(call.Common().Value.Type())
		if call.Common().Method != nil {
			cs.Name = call.Common().Method.Name()
		}
	} else {
		if f, ok := call.Common().Value.(*ssa.Function); ok {
			if rec := f.Signature.Recv(); rec != nil {
				cs.RecvType = typeString(rec.Type())
			}
		}
	}

	return cs
}

func getOrCreateNode(f *ssa.Function, funcToNode map[*ssa.Function]*FuncNode) *FuncNode {
	if fNode, ok := funcToNode[f]; ok {
		return fNode
	}
	fNode := &FuncNode{
		ID:      freshId(),
		Name:    dbFuncName(f),
		PkgPath: pkgPath(f),
		Pos:     funcPosition(f),
	}

	if rec := f.Signature.Recv(); rec != nil {
		fNode.RecvType = typeString(rec.Type())
	}
	funcToNode[f] = fNode
	return fNode
}

type gCallChain struct {
	// nil for entry points of the chain.
	call   *CallSite
	f      *FuncNode
	parent *gCallChain
}

func (chain *gCallChain) trace() []TraceElem {
	if chain == nil {
		return nil
	}

	var pos *token.Position
	desc := fmt.Sprintf("%s.%s(...)", chain.f.PkgPath, chain.f.Name)
	if chain.call != nil {
		pos = chain.call.Pos
		if !chain.call.Resolved {
			// In case of a statically unresolved call site, communicate to the client
			// that this was approximatelly resolved to chain.f.
			desc = fmt.Sprintf("%s(...) [approx. resolved to %s.%s]", gCallName(chain.call, chain.parent.f), chain.f.PkgPath, chain.f.Name)
		}
	} else {
		// No call information means the function is an entry point.
		pos = chain.f.Pos
	}

	return append(chain.parent.trace(), TraceElem{Description: desc, Position: pos})
}

func (chain *gCallChain) weight() int {
	if chain == nil || chain.call == nil {
		return 0
	}

	callWeight := 0
	if !chain.call.Resolved {
		callWeight = 1
	}
	return callWeight + chain.parent.weight()
}

func gIsStdPackage(pkgPath string) bool {
	loadStdsOnce.Do(func() {
		pkgs, err := packages.Load(nil, "std")
		if err != nil {
			log.Printf("warning: unable to fetch list of std packages, ordering of findings might be affected: %v", err)
		}

		for _, p := range pkgs {
			stdPackages[p.PkgPath] = true
		}
	})
	return stdPackages[pkgPath]
}

func (chain *gCallChain) confidence() int {
	if chain == nil || chain.call == nil {
		return 0
	}

	callConfidence := 0
	if gIsStdPackage(chain.parent.f.PkgPath) {
		callConfidence = 1
	}
	return callConfidence + chain.parent.confidence()
}
