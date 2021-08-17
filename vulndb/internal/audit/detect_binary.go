// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package audit

import (
	"fmt"
	"runtime"

	"golang.org/x/exp/vulndb/internal/binscan"
	"golang.org/x/vulndb/osv"
)

// VulnerablePackageSymbols returns a list of vulnerability findings for per-package symbols
// extracted from Go binary at `binPath`. Vulnerabilities are provided by db `client`.
//
// Findings for each vulnerability are sorted by their estimated usefulness to the user and
// do not have an associated trace.
//
// TODO: binPath should ideally be byte array.
func VulnerablePackageSymbols(binPath string, client DbClient) (*Results, error) {
	modules, packageSymbols, err := binscan.ExtractPackagesAndSymbols(binPath)
	if err != nil {
		return nil, err
	}
	results := &Results{SearchMode: BinarySearch}

	modVulns, err := fetchVulnerabilities(client, modules)
	if err != nil {
		return nil, err
	}

	modVulns = modVulns.Filter(runtime.GOOS, runtime.GOARCH)
	if len(modVulns) == 0 {
		return results, nil
	}

	for pkg, symbols := range packageSymbols {
		for _, symbol := range symbols {
			vulns := modVulns.VulnsForSymbol(pkg, symbol)
			for _, v := range vulns {
				results.addFinding(v, Finding{
					Symbol: fmt.Sprintf("%s.%s", pkg, symbol),
					Type:   GlobalType,
				})
			}
		}
	}

	addBinaryUnusedVulns(results, modVulns)
	results.sort()
	return results, nil
}

func addBinaryUnusedVulns(results *Results, modVulns moduleVulnerabilities) {
	allVulns := make(map[string]*osv.Entry)
	for _, v := range modVulns.Vulns() {
		allVulns[v.ID] = v
	}

	vulnsWithFindings := make(map[string]bool)
	for _, vf := range results.VulnFindings {
		vulnsWithFindings[vf.Vuln.ID] = true
	}

	for id, v := range allVulns {
		if vulnsWithFindings[id] {
			continue
		}
		uv := UnreachableVuln{Vuln: v, Type: NotImported}
		results.UnreachableVulns = append(results.UnreachableVulns, uv)
	}
}
