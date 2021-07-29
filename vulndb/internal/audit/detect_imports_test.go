// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package audit

import (
	"reflect"
	"sort"
	"testing"

	"golang.org/x/vulndb/osv"
)

func TestImportedPackageVulnDetection(t *testing.T) {
	pkgs, env := testProgAndEnv(t)
	results := VulnerableImports(pkgs, env)

	if results.SearchMode != ImportsSearch {
		t.Errorf("want import search mode; got %v", results.SearchMode)
	}

	// There should be two chains reported in the following order
	// for two of the thirdparty.org test vulnerabilities:
	//   T -> vuln
	//   T -> A -> vuln
	for _, test := range []struct {
		vulnId   string
		findings []Finding
	}{
		{vulnId: "TP-1", findings: []Finding{
			{
				Symbol: "thirdparty.org/vulnerabilities/vuln",
				Trace:  []TraceElem{{Description: "command-line-arguments"}},
				Type:   ImportType,
				weight: 1,
			},
			{
				Symbol: "thirdparty.org/vulnerabilities/vuln",
				Trace:  []TraceElem{{Description: "command-line-arguments"}, {Description: "a.org/A"}},
				Type:   ImportType,
				weight: 2,
			},
		}},
		{vulnId: "TP-3", findings: []Finding{
			{
				Symbol: "thirdparty.org/vulnerabilities/vuln",
				Trace:  []TraceElem{{Description: "command-line-arguments"}},
				Type:   ImportType,
				weight: 1,
			},
			{
				Symbol: "thirdparty.org/vulnerabilities/vuln",
				Trace:  []TraceElem{{Description: "command-line-arguments"}, {Description: "a.org/A"}},
				Type:   ImportType,
				weight: 2,
			},
		}},
	} {
		got := projectFindings(results.Findings(osv.Entry{ID: test.vulnId}))
		sort.SliceStable(got, func(i int, j int) bool { return FindingCompare(got[i], got[j]) })
		if !reflect.DeepEqual(test.findings, got) {
			t.Errorf("want %v findings (projected); got %v", test.findings, got)
		}
	}
}
