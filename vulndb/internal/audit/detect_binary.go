// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package audit

import (
	"fmt"
)

// VulnerablePackageSymbols returns a list of vulnerability findings for per-package symbols
// in packageSymbols, given the vulnerability and platform info captured in env.
//
// Returned Findings only have Symbol, Type, and Vulns fields set.
//
// Findings for each vulnerability are sorted by estimated usefulness to the user.
func VulnerablePackageSymbols(packageSymbols map[string][]string, env Env) Results {
	results := Results{
		SearchMode:      BinarySearch,
		Vulnerabilities: serialize(env.Vulns),
		VulnFindings:    make(map[string][]Finding),
	}
	if len(env.Vulns) == 0 {
		return results
	}

	symVulns := createSymVulns(env.Vulns)

	for pkg, symbols := range packageSymbols {
		for _, symbol := range symbols {
			vulns := querySymbolVulns(symbol, pkg, symVulns, env)
			for _, v := range serialize(vulns) {
				results.addFinding(v, Finding{
					Symbol: fmt.Sprintf("%s.%s", pkg, symbol),
					Type:   GlobalType,
				})
			}
		}
	}

	results.sort()
	return results
}
