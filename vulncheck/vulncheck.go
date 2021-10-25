// Package vulncheck detects uses of known vulnerabilities
// in Go binaries and source code.
package vulncheck

import (
	"go/token"

	"golang.org/x/vulndb/client"
	"golang.org/x/vulndb/osv"
)

// Config is used for configuring vulncheck algorithms.
type Config struct {
	// If true, analyze import chains only. Otherwise, analyze call chains too.
	ImportsOnly bool
	// Database client for querying vulnerability data.
	Client client.Client
}

// Result contains information on which vulnerabilities are potentially affecting
// user code and how are they affecting them via call graph, package imports graph,
// and module requires graph.
type Result struct {
	// Call graph whose roots are program entry functions/methods and sinks are
	// vulnerable functions/methods. Empty when Config.ImportsOnly=true or when
	// no vulnerable symbols are reachable via program call graph.
	Calls *CallGraph
	// Package imports graph whose roots are entry user packages and sinks are
	// the packages with some vulnerable symbols.
	Imports *ImportGraph
	// Module graph whose roots are entry user modules and sinks are modules
	// with some vulnerable packages.
	Requires *RequireGraph

	// Detected vulnerabilities and their place in the above graphs. Only
	// vulnerabilities whose symbols are reachable in Calls, or whose packages
	// are imported in Imports, or whose modules are required in Requires, have
	// an entry in Vulns.
	Vulns []*Vuln
}

// Vuln provides information on how a vulnerability is affecting user code by
// connecting it to the Result.{Calls,Imports,Requires} graph. Vulnerabilities
// detected in Go binaries do not have a place in the Result graphs.
type Vuln struct {
	// The next four fields identify a vulnerability. Note that *osv.Entry
	// describes potentially multiple symbols from multiple packages.
	OSV     *osv.Entry
	Symbol  string
	PkgPath string
	ModPath string

	// ID of the sink node in Calls graph corresponding to the use of Symbol.
	// ID is not available (denoted with 0) in binary mode, or if Symbol is
	// not reachable, or if Config.ImportsOnly=true.
	CallSink int
	// ID of the sink node in the Imports graph corresponding to the import of
	// PkgPath. ID is not available (denoted with 0) in binary mode or if PkgPath
	// is not imported.
	ImportSink int
	// IDs of the sink node in Requires graph corresponding to the require statement
	// of ModPath. ID is not available (denoted with 0) in binary mode.
	RequireSink int
}

// CallGraph whose sinks are vulnerable functions and sources are entry points of user
// packages. CallGraph is backwards directed, i.e., from a function node to the place
// where the function is called.
type CallGraph struct {
	Funcs   map[int]*FuncNode // all func nodes as a map: func node id -> func node
	Entries []*FuncNode       // subset of Funcs representing vulncheck entry points
}

type FuncNode struct {
	ID        int
	Name      string
	RecvType  string // receiver object type, if any
	PkgPath   string
	Pos       *token.Position
	CallSites []*CallSite // set of call sites where this function is called
}

type CallSite struct {
	Parent   int    // ID of the enclosing function where the call is made
	Name     string // name of the function (variable) being called
	RecvType string // full path of the receiver object type, if any
	Pos      *token.Position
	Resolved bool // true for static call, false otherwise
}

// RequireGraph models part of module requires graph where sinks are modules with
// some known vulnerabilities and sources are modules of user entry packages.
// RequireGraph is backwards directed, i.e., from a module to the set of modules
// it is required by.
type RequireGraph struct {
	Modules map[int]*ModNode // module node id -> module node
	Entries []*ModNode
}

type ModNode struct {
	Path       string
	Version    string
	Replace    *ModNode
	RequiredBy []int // IDs of the modules requiring this module
}

// ImportGraph models part of package import graph where sinks are packages with
// some known vulnerabilities and sources are user specified packages.
// ImportGraph is backwards directed, i.e., from a package to the set of packages
// importing it.
type ImportGraph struct {
	Pkgs    map[int]*PkgNode // package node id -> package node
	Entries []*PkgNode
}

type PkgNode struct {
	Name       string // as it appears in the source code
	Path       string
	Module     int   // ID of the corresponding module (node) in Requires graph
	ImportedBy []int // IDs of packages directly importing this package
}
