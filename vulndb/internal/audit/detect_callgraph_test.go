// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package audit

import (
	"go/token"
	"reflect"
	"testing"
)

func TestSymbolVulnDetectionVTA(t *testing.T) {
	pkgs, client := testContext(t)
	results, err := VulnerableSymbols(pkgs, client)
	if err != nil {
		t.Fatal(err)
	}

	if results.SearchMode != CallGraphSearch {
		t.Errorf("want call graph search mode; got %v", results.SearchMode)
	}

	// There should be four call chains reported with VTA-VTA version, in the following order,
	// for vuln.VG and vuln.VulnData.Vuln vulnerabilities:
	//  vuln.VG:
	//   T:T1() -> vuln.VG                                     [use of global at line 4]
	//   T:T2() -> vuln.Vuln() [approx.resolved] -> vuln.VG    [use of global at vuln.go:4]
	//  vuln.VulnData.Vuln:
	//   T:T1() -> A:A1() -> vuln.VulnData.Vuln()              [call at A.go:14]
	//   T:T1() -> vuln.VulnData.Vuln() [approx. resolved]     [call at testdata.go:13]
	// Without VTA-VTA, we would alse have the following false positive:
	//   T:T2() -> vuln.VulnData.Vuln() [approx. resolved]     [call at testdata.go:26]
	for _, test := range []struct {
		vulnId   string
		findings []Finding
	}{
		{vulnId: "V1", findings: []Finding{
			{
				Symbol: "thirdparty.org/vulnerabilities/vuln.VulnData.Vuln",
				Trace: []TraceElem{
					{Description: "command-line-arguments.T1(...)", Position: &token.Position{Line: 11, Filename: "T.go"}},
					{Description: "a.org/A.A1(...)", Position: &token.Position{Line: 14, Filename: "T.go"}}},
				Type:     FunctionType,
				Position: &token.Position{Line: 15, Filename: "A.go"},
				weight:   0,
			},
			{
				Symbol: "thirdparty.org/vulnerabilities/vuln.VulnData.Vuln",
				Trace: []TraceElem{
					{Description: "command-line-arguments.T1(...)", Position: &token.Position{Line: 11, Filename: "T.go"}},
					{Description: "a.org/A.I.Vuln(...) [approx. resolved to (thirdparty.org/vulnerabilities/vuln.VulnData).Vuln]", Position: &token.Position{Line: 14, Filename: "T.go"}}},
				Type:     FunctionType,
				Position: &token.Position{Line: 14, Filename: "T.go"},
				weight:   1,
			},
		}},
		{vulnId: "V2", findings: []Finding{
			{
				Symbol: "thirdparty.org/vulnerabilities/vuln.VG",
				Trace: []TraceElem{
					{Description: "command-line-arguments.T1(...)", Position: &token.Position{Line: 11, Filename: "T.go"}},
				},
				Type:     GlobalType,
				Position: &token.Position{Line: 5, Filename: "vuln.go"},
				weight:   0,
			},
			{
				Symbol: "thirdparty.org/vulnerabilities/vuln.VG",
				Trace: []TraceElem{
					{Description: "command-line-arguments.T2(...)", Position: &token.Position{Line: 20, Filename: "T.go"}},
					{Description: "command-line-arguments.t0(...) [approx. resolved to thirdparty.org/vulnerabilities/vuln.Vuln]", Position: &token.Position{Line: 22, Filename: "T.go"}},
				},
				Type:     GlobalType,
				Position: &token.Position{Line: 5, Filename: "vuln.go"},
				weight:   1,
			},
		}},
	} {
		got := projectFindings(vulnFindings(results, test.vulnId))
		if !reflect.DeepEqual(test.findings, got) {
			t.Errorf("want %v findings (projected); got %v", test.findings, got)
		}
	}
}
