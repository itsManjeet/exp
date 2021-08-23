package main

import (
	"fmt"
	"sort"
	"strings"

	"golang.org/x/exp/vulndb/internal/audit"
	"golang.org/x/vulndb/osv"
)

func resultsString(r *audit.Results) string {
	var vulns []*osv.Entry
	vulnFindings := make(map[*osv.Entry][]audit.Finding)
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

		rStr += fmt.Sprintf("Findings for vulnerability: %s (of package %s):\n", alias(v), v.Package.Name)
		for _, f := range findings {
			rStr += "  Trace:\n" + indent(findingString(f), "\t") + "\n"
		}
	}

	rStr += fmt.Sprintf("Vulnerabilites with imported package but no observed use:\n")
	for _, v := range r.PackageVulns {
		rStr += fmt.Sprintf("\t%s (of package %s)\n", alias(v), v.Package.Name)
	}

	rStr += "\n"

	rStr += fmt.Sprintf("Vulnerabilites with imported module but no imported package and no observed use:\n")
	for _, v := range r.ModuleVulns {
		rStr += fmt.Sprintf("\t%s (of package %s)\n", alias(v), v.Package.Name)
	}

	return rStr
}

func alias(v *osv.Entry) string {
	if len(v.Aliases) == 0 {
		return v.EcosystemSpecific.URL
	}
	return strings.Join(v.Aliases, ", ")
}

func indent(text string, ind string) string {
	lines := strings.Split(text, "\n")
	var newLines []string
	for _, line := range lines {
		newLines = append(newLines, ind+line)
	}
	return strings.Join(newLines, "\n")
}

func findingString(f audit.Finding) string {
	traceStr := traceString(f.Trace)

	var pos string
	if f.Position != nil {
		pos = fmt.Sprintf(" (%s)", f.Position)
	}

	return fmt.Sprintf("%s%s\n%s", f.Symbol, pos, traceStr)
}

func traceString(trace []audit.TraceElem) string {
	// traces are typically short, so string builders are not necessary
	traceStr := ""
	for i := len(trace) - 1; i >= 0; i-- {
		traceStr += traceElemString(trace[i]) + "\n"
	}
	return traceStr
}

func traceElemString(e audit.TraceElem) string {
	if e.Position == nil {
		return fmt.Sprintf("%s", e.Description)
	}
	return fmt.Sprintf("%s (%s)", e.Description, e.Position)
}
