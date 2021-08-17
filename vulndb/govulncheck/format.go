package main

import (
	"fmt"
	"sort"
	"strings"

	"golang.org/x/exp/vulndb/internal/audit"
)

func resultsString(r *audit.Results) string {
	sort.Slice(r.Vulnerabilities, func(i, j int) bool { return r.Vulnerabilities[i].ID < r.Vulnerabilities[j].ID })

	rStr := ""
	for _, v := range r.Vulnerabilities {
		findings := r.VulnFindings[v.ID]
		if len(findings) == 0 {
			// TODO: add messages for such cases too?
			continue
		}

		var alias string
		if len(v.Aliases) == 0 {
			alias = v.EcosystemSpecific.URL
		} else {
			alias = strings.Join(v.Aliases, ", ")
		}
		rStr += fmt.Sprintf("Findings for vulnerability: %s (of package %s):\n", alias, v.Package.Name)

		for _, f := range findings {
			rStr += "  Trace:\n" + indent(findingString(f), "\t") + "\n"
		}
	}
	return rStr
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
