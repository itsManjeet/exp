// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package audit finds vulnerabilities affecting Go packages.
package audit

import (
	"fmt"
	"go/token"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
	"golang.org/x/vulndb/osv"
)

// Preamble with types and common functionality used by vulnerability detection mechanisms in detect_*.go files.

// DbClient interface for loading vulnerabilities for
// a list of import paths.
type DbClient interface {
	Get([]string) ([]*osv.Entry, error)
}

// SearchType represents a type of an audit search: call graph, imports, or binary.
type SearchType int

// enum values for SearchType.
const (
	CallGraphSearch SearchType = iota
	ImportsSearch
	BinarySearch
)

// Results contains the information on findings and identified vulnerabilities by audit search.
type Results struct {
	SearchMode SearchType
	// TODO: identify vulnerability with <ID, package, symbol>?

	// Information on vulnerabilities that are not exercised but whose
	// corresponding modules were referenced by the client code.
	UnreachableVulns []UnreachableVuln
	VulnFindings     []*VulnFindings // vulnerability -> findings
}

// TODO: improve result format
func (r *Results) String() string {
	var vulns []*osv.Entry
	vulnFindings := make(map[*osv.Entry][]Finding)
	for _, vf := range r.VulnFindings {
		vulns = append(vulns, vf.Vuln)
		vulnFindings[vf.Vuln] = vf.Findings
	}
	sort.Slice(vulns, func(i, j int) bool { return vulns[i].ID < vulns[j].ID })

	rStr := ""
	for _, v := range vulns {
		findings := vulnFindings[v]
		if len(findings) == 0 {
			// TODO: add messages for such cases too?
			continue
		}

		rStr += fmt.Sprintf("Findings for vulnerability: %s:\n\n", alias(v))
		for _, finding := range findings {
			rStr += finding.String() + "\n"
		}
	}

	rStr += fmt.Sprintf("Vulnerabilites not exercised in the code but applicable to modules (transitively) used by the code:\n")
	for _, uv := range r.UnreachableVulns {
		rStr += fmt.Sprintf("\t%s (%s)\n", alias(uv.Vuln), uv.Type.String())
	}

	return rStr
}

func alias(v *osv.Entry) string {
	if len(v.Aliases) == 0 {
		return v.EcosystemSpecific.URL
	}
	return strings.Join(v.Aliases, ", ")
}

// addFindings adds a findings `f` for vulnerability `v`.
func (r *Results) addFinding(v *osv.Entry, f Finding) {
	for _, vf := range r.VulnFindings {
		if vf.Vuln == v {
			vf.Findings = append(vf.Findings, f)
			return
		}
	}
	r.VulnFindings = append(r.VulnFindings, &VulnFindings{Vuln: v, Findings: []Finding{f}})
}

// sort orders findings for each vulnerability based on its
// perceived usefulness to the user.
func (r *Results) sort() {
	for _, vf := range r.VulnFindings {
		fs := vf.Findings
		sort.SliceStable(fs, func(i int, j int) bool { return findingCompare(&fs[i], &fs[j]) })
	}
}

// UnreachableVuln encodes why a vulnerability
// has not been exercised in the code.
type UnreachableVuln struct {
	Type UnreachableType
	Vuln *osv.Entry
}

// UnreachableType provides information on why a vulnerability was not exercised in
// the client code although its module is imported by the client.
type UnreachableType int

// enum values for UnreachableType.
const (
	// package of the vulnerability is not imported
	NotImported UnreachableType = iota
	// package of the vulnerability imported, but vulnerability is not reachable
	Unreachable
)

func (u UnreachableType) String() string {
	switch u {
	case NotImported:
		return "package not imported"
	case Unreachable:
		return "symbols not reachable"
	default:
		return "unknown"
	}
}

// VulnFindings encapsulates findings for a vulnerability.
type VulnFindings struct {
	Vuln     *osv.Entry
	Findings []Finding
}

// Finding represents a finding for the use of a vulnerable symbol or an imported vulnerable package.
// Provides info on symbol location and the trace leading up to the symbol use.
type Finding struct {
	Symbol   string
	Position *token.Position `json:",omitempty"`
	Type     SymbolType
	Trace    []TraceElem

	// Approximate measure for indicating how understandable the finding is to the client.
	// The smaller the weight, the more understandable is the finding.
	weight int

	// Approximate measure for indicating confidence in finding being a true positive. The
	// smaller the value, the bigger the confidence.
	confidence int
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

type modVulns struct {
	mod   *packages.Module
	vulns []*osv.Entry
}

type moduleVulnerabilities []modVulns

func matchesPlatform(os, arch string, e osv.GoSpecific) bool {
	matchesOS := len(e.GOOS) == 0
	matchesArch := len(e.GOARCH) == 0
	for _, o := range e.GOOS {
		if os == o {
			matchesOS = true
			break
		}
	}
	for _, a := range e.GOARCH {
		if arch == a {
			matchesArch = true
			break
		}
	}
	return matchesOS && matchesArch
}

func (mv moduleVulnerabilities) Filter(os, arch string) moduleVulnerabilities {
	var filteredMod moduleVulnerabilities
	for _, mod := range mv {
		module := mod.mod
		modVersion := module.Version
		if module.Replace != nil {
			modVersion = module.Replace.Version
		}
		// TODO: if modVersion == "", try vcs to get the version?
		var filteredVulns []*osv.Entry
		for _, v := range mod.vulns {
			// A module version is affected if
			//  - it is included in one of the affected version ranges
			//  - and module version is not ""
			//  The latter means the module version is not available, so
			//  we don't want to spam users with potential false alarms.
			//  TODO: issue warning for "" cases above?
			affectsVersion := modVersion != "" && v.Affects.AffectsSemver(modVersion)
			if affectsVersion && matchesPlatform(os, arch, v.EcosystemSpecific) {
				filteredVulns = append(filteredVulns, v)
			}
		}
		filteredMod = append(filteredMod, modVulns{
			mod:   module,
			vulns: filteredVulns,
		})
	}
	return filteredMod
}

func (mv moduleVulnerabilities) Num() int {
	var num int
	for _, m := range mv {
		num += len(m.vulns)
	}
	return num
}

// VulnsForPackage returns the vulnerabilities for the module which is the most
// specific prefix of importPath, or nil if there is no matching module with
// vulnerabilities.
func (mv moduleVulnerabilities) VulnsForPackage(importPath string) []*osv.Entry {
	var mostSpecificMod *modVulns
	for _, mod := range mv {
		md := mod
		if strings.HasPrefix(importPath, md.mod.Path) {
			if mostSpecificMod == nil || len(mostSpecificMod.mod.Path) < len(md.mod.Path) {
				mostSpecificMod = &md
			}
		}
	}

	if mostSpecificMod == nil {
		return nil
	}

	if mostSpecificMod.mod.Replace != nil {
		importPath = fmt.Sprintf("%s%s", mostSpecificMod.mod.Replace.Path, strings.TrimPrefix(importPath, mostSpecificMod.mod.Path))
	}
	vulns := mostSpecificMod.vulns
	packageVulns := []*osv.Entry{}
	for _, v := range vulns {
		if v.Package.Name == importPath {
			packageVulns = append(packageVulns, v)
		}
	}
	return packageVulns
}

// VulnsForSymbol returns vulnerabilites for `symbol` in `mv.VulnsForPackage(importPath)`.
func (mv moduleVulnerabilities) VulnsForSymbol(importPath, symbol string) []*osv.Entry {
	vulns := mv.VulnsForPackage(importPath)
	if vulns == nil {
		return nil
	}

	symbolVulns := []*osv.Entry{}
	for _, v := range vulns {
		if len(v.EcosystemSpecific.Symbols) == 0 {
			symbolVulns = append(symbolVulns, v)
			continue
		}
		for _, s := range v.EcosystemSpecific.Symbols {
			if s == symbol {
				symbolVulns = append(symbolVulns, v)
				break
			}
		}
	}
	return symbolVulns
}

// Vulns returns vulnerabilities for all modules in `mv`.
func (mv moduleVulnerabilities) Vulns() []*osv.Entry {
	var vulns []*osv.Entry
	seen := make(map[string]bool)
	for _, mv := range mv {
		for _, v := range mv.vulns {
			if !seen[v.ID] {
				vulns = append(vulns, v)
				seen[v.ID] = true
			}
		}
	}
	return vulns
}
