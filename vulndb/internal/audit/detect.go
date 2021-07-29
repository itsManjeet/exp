// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package audit finds vulnerabilities affecting Go packages.
package audit

import (
	"fmt"
	"go/token"
	"sort"

	"golang.org/x/vulndb/osv"
)

// Preamble with types and common functionality used by vulnerability detection mechanisms in detect_*.go files.

// SearchType represents a type of an audit search: call graph, imports, or binary.
type SearchType int

// enum values for SearchType.
const (
	CallGraphSearch SearchType = iota
	ImportsSearch
	BinarySearch
)

// Results contains the information on findings and identified
// vulnerabilities by audit search.
type Results struct {
	SearchMode SearchType

	// Vulnerabilities imported by a target program
	// but not necessarily reachable by any execution.
	Vulnerabilities []osv.Entry

	VulnFindings map[string][]Finding // vuln.ID -> findings
}

// String method for results.
func (r Results) String() string {
	// sort vulnerabilities by their ID but show
	// ones that have some findings earlier.
	sort.Slice(r.Vulnerabilities, func(i, j int) bool {
		vI := r.Vulnerabilities[i]
		hasFindsI := len(r.VulnFindings[vI.ID]) > 0
		vJ := r.Vulnerabilities[j]
		hasFindsJ := len(r.VulnFindings[vJ.ID]) > 0

		return (hasFindsI && !hasFindsJ) || (hasFindsI && hasFindsJ && vI.ID < vJ.ID)
	})

	idToVuln := make(map[string]osv.Entry)
	for _, v := range r.Vulnerabilities {
		idToVuln[v.ID] = v
	}

	rStr := ""
	for id, findings := range r.VulnFindings {
		v := idToVuln[id]
		rStr += fmt.Sprintf("Findings for vulnerability %s (%s):\n\n", v.EcosystemSpecific.URL, v.Package.Name)

		if len(findings) == 0 && r.SearchMode == CallGraphSearch {
			rStr += "package imported, but no vulnerable symbols is reachable\n"
		}
		for _, finding := range findings {
			rStr += finding.String() + "\n"
		}
	}
	return rStr
}

// addFindings adds a findings `f` for vulnerability `v`.
func (r Results) addFinding(v osv.Entry, f Finding) {
	r.VulnFindings[v.ID] = append(r.VulnFindings[v.ID], f)
}

// sort orders findings for each vulnerability based on its
// perceived usefulness to the user.
func (r Results) sort() {
	for _, fs := range r.VulnFindings {
		sort.SliceStable(fs, func(i int, j int) bool { return findingCompare(fs[i], fs[j]) })
	}
}

// Finding represents a finding for the use of a vulnerable symbol or an imported vulnerable package.
// Provides info on symbol location and the trace leading up to the symbol use.
type Finding struct {
	Symbol   string
	Position *token.Position `json:",omitempty"`
	Type     SymbolType
	Trace    []TraceElem

	// Approximate measure for indicating how useful the finding might be to the audit client.
	// The smaller the weight, the more useful is the finding.
	weight int
}

// String method for findings.
func (f Finding) String() string {
	traceStr := traceString(f.Trace)

	var pos string
	if f.Position != nil {
		pos = fmt.Sprintf(" (%s)", f.Position)
	}

	return fmt.Sprintf("Trace:\n%s%s\n%s\n", f.Symbol, pos, traceStr)
}

func traceString(trace []TraceElem) string {
	// traces are typically short, so string builders are not necessary
	traceStr := ""
	for i := len(trace) - 1; i >= 0; i-- {
		traceStr += trace[i].String() + "\n"
	}
	return traceStr
}

// SymbolType represents a type of a symbol use: function, global, or an import statement.
type SymbolType int

// enum values for SymbolType.
const (
	FunctionType SymbolType = iota
	ImportType
	GlobalType
)

// TraceElem represents an entry in the finding trace. Represents a function call or an import statement.
type TraceElem struct {
	Description string
	Position    *token.Position `json:",omitempty"`
}

// String method for trace elements.
func (e TraceElem) String() string {
	if e.Position == nil {
		return fmt.Sprintf("%s", e.Description)
	}
	return fmt.Sprintf("%s (%s)", e.Description, e.Position)
}

// Env encapsulates information for querying if an imported symbol/package is vulnerable:
//  - platform info
//  - package versions
//  - vulnerability db
type Env struct {
	OS          string
	Arch        string
	PkgVersions map[string]string
	Vulns       []*osv.Entry
}

// MarshalText implements the encoding.TextMarshaler interface.
func (s SymbolType) MarshalText() ([]byte, error) {
	var name string
	switch s {
	default:
		name = "unrecognized"
	case FunctionType:
		name = "function"
	case ImportType:
		name = "import"
	case GlobalType:
		name = "global"
	}
	return []byte(name), nil
}

// matchingVulns returns `vulns` matching `os`, `arch`, and `version`.
func matchingVulns(os, arch, version string, vulns []*osv.Entry) []*osv.Entry {
	var matches []*osv.Entry
	for _, vuln := range vulns {
		if matchesPlatformAndVersion(os, arch, version, vuln) {
			matches = append(matches, vuln)
		}
	}
	return matches
}

// matchesPlatformAndVersion checks if `os`, `arch`, and `version` match the vulnerability `vuln`.
func matchesPlatformAndVersion(os, arch, version string, vuln *osv.Entry) bool {
	return matchesPlatform(os, vuln.EcosystemSpecific.GOOS) && matchesPlatform(arch, vuln.EcosystemSpecific.GOARCH) && vuln.Affects.AffectsSemver(version)
}

// matchesPlatform checks if `platform`, typically os or system architecture,
// matches `platforms`. Empty `platforms` is also a match.
func matchesPlatform(platform string, platforms []string) bool {
	if len(platforms) == 0 {
		return true
	}

	for _, p := range platforms {
		if platform == p {
			return true
		}
	}
	return false
}

// pkgVulnerabilities map for fast lookup on vulnerable packages.
// Maps package paths to their vulnerabilities.
type pkgVulnerabilities map[string][]*osv.Entry

// createPkgVulns creates a fast package-vulnerability look-up map for `vulns`.
func createPkgVulns(vulns []*osv.Entry) pkgVulnerabilities {
	pkgVulns := make(pkgVulnerabilities)
	for _, vuln := range vulns {
		pkgVulns[vuln.Package.Name] = append(pkgVulns[vuln.Package.Name], vuln)
	}
	return pkgVulns
}

// vulnerabilities returns a list of vulnerabilities that deem `pkgPath` vulnerable at `version` as well
// as `arch` architecture and `os` operating system. Assumes version strings in `pkgVulns` are well-formed;
// otherwise, the correctness of the results is not guaranteed.
func (pkgVulns pkgVulnerabilities) vulnerabilities(pkgPath, version, arch, os string) []*osv.Entry {
	vulns, ok := pkgVulns[pkgPath]
	if !ok {
		return nil
	}
	return matchingVulns(os, arch, version, vulns)
}

func queryPkgVulns(pkgPath string, env Env, pkgVulns pkgVulnerabilities) []*osv.Entry {
	version, ok := env.PkgVersions[pkgPath]
	if !ok {
		return nil
	}
	return pkgVulns.vulnerabilities(pkgPath, version, env.Arch, env.OS)
}

// symVulnerabilities map for fast lookup on vulnerable symbols.
// Maps package paths to symbols to their vulnerabilities.
type symVulnerabilities map[string]map[string][]*osv.Entry

// Represents any symbol. Used to model vulnerabilities in
// symVulnerabilties that define every symbol as vulnerable.
const symWildCard = "*"

// createSymVulns creates a fast symbol-vulnerability look-up map for `vulns`.
func createSymVulns(vulns []*osv.Entry) symVulnerabilities {
	symVulns := make(symVulnerabilities)
	for _, vuln := range vulns {
		if len(vuln.EcosystemSpecific.Symbols) == 0 {
			// If vuln.Symbols is empty, every symbol is vulnerable.
			symVulns.add(symWildCard, vuln)
		} else {
			for _, sym := range vuln.EcosystemSpecific.Symbols {
				symVulns.add(sym, vuln)
			}
		}
	}
	return symVulns
}

func (symVulns symVulnerabilities) add(symbol string, v *osv.Entry) {
	syms := symVulns[v.Package.Name]
	if syms == nil {
		syms = make(map[string][]*osv.Entry)
		symVulns[v.Package.Name] = syms
	}
	syms[symbol] = append(syms[symbol], v)
}

// vulnerabilities returns a list of vulnerabilities that deem `symbol` from package `pkgPath` vulnerable at
// `version`, architecture `arch`, and operating system `os`. Assumes version strings in `symVulns` are well-formed;
// otherwise, the correctness of the results is not guaranteed.
func (symVulns symVulnerabilities) vulnerabilities(symbol, pkgPath, version, arch, os string) []*osv.Entry {
	pkgVulns, ok := symVulns[pkgPath]
	if !ok {
		return nil
	}

	var vulns []*osv.Entry
	vulns = append(vulns, pkgVulns[symbol]...)
	vulns = append(vulns, pkgVulns[symWildCard]...)
	if len(vulns) == 0 {
		return nil
	}

	return matchingVulns(os, arch, version, vulns)
}

func querySymbolVulns(symbol, pkgPath string, symVulns symVulnerabilities, env Env) []*osv.Entry {
	version, ok := env.PkgVersions[pkgPath]
	if !ok {
		return nil
	}
	return symVulns.vulnerabilities(symbol, pkgPath, version, env.Arch, env.OS)
}
