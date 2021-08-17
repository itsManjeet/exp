// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Command govulncheck reports known vulnerabilities filed in a vulnerability database
// (see https://golang.org/design/draft-vulndb) that affect a given package or binary.
//
// It uses static analysis or the binary's symbol table to narrow down reports to only
// those that potentially affect the application.
//
// WARNING WARNING WARNING
//
// govulncheck is still experimental and neither its output or the vulnerability
// database should be relied on to be stable or comprehensive.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/build"
	"os"
	"strings"

	"golang.org/x/exp/vulndb/internal/audit"
	"golang.org/x/tools/go/buildutil"
	"golang.org/x/tools/go/packages"
	"golang.org/x/vulndb/client"
)

var (
	jsonFlag    = flag.Bool("json", false, "")
	importsFlag = flag.Bool("imports", false, "")
	allFlag     = flag.Bool("all", false, "")
	testsFlag   = flag.Bool("tests", false, "")
)

const usage = `govulncheck: identify known vulnerabilities by call graph traversal.

Usage:

	govulncheck [-imports] [-json] [-all] [-tests] [-tags] {package pattern...}

	govulncheck {binary path}

Flags:

	-imports   Perform a broad scan with more false positives, which reports all
	           vulnerabilities found in any transitively imported package, regardless
	           of whether they are reachable.

	-json  	   Print vulnerability findings in JSON format.

	-all       Show all representative findings for each vulnerability. A best effort
		   is made to order findings by relevance. When false [default], show only
		   the most relevant finding.

	-tags	   Comma-separated list of build tags.

	-tests     Boolean flag indicating if test files should be analyzed too.

govulncheck can be used with either one or more package patterns (i.e. golang.org/x/crypto/...
or ./...) or with a single path to a Go binary. In the latter case module and symbol
information will be extracted from the binary in order to detect vulnerable symbols
and the -imports flag is disregarded.

The environment variable GOVULNDB can be set to a comma-separate list of vulnerability
database URLs, with http://, https://, or file:// protocols. Entries from multiple
databases are merged.
`

func init() {
	flag.Var((*buildutil.TagsFlag)(&build.Default.BuildTags), "tags", buildutil.TagsFlagDoc)
}

func main() {
	flag.Usage = func() { fmt.Fprintln(os.Stderr, usage) }
	flag.Parse()

	if len(flag.Args()) == 0 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(1)
	}

	dbs := []string{"https://storage.googleapis.com/go-vulndb"}
	if GOVULNDB := os.Getenv("GOVULNDB"); GOVULNDB != "" {
		dbs = strings.Split(GOVULNDB, ",")
	}
	dbClient, err := client.NewClient(dbs, client.Options{HTTPCache: defaultCache()})
	if err != nil {
		fmt.Fprintf(os.Stderr, "govulncheck: %s\n", err)
		os.Exit(1)
	}

	cfg := &packages.Config{
		Mode:       packages.LoadAllSyntax | packages.NeedModule,
		Tests:      *testsFlag,
		BuildFlags: []string{fmt.Sprintf("-tags=%s", strings.Join(build.Default.BuildTags, ","))},
	}

	r, err := run(cfg, flag.Args(), *importsFlag, dbClient)
	if err != nil {
		fmt.Fprintf(os.Stderr, "govulncheck: %s\n", err)
		os.Exit(1)
	}

	if !*allFlag {
		r = projectToSingleFinding(r)
	}

	writeOut(r, *jsonFlag)
}

func projectToSingleFinding(r *audit.Results) *audit.Results {
	nr := &audit.Results{
		SearchMode:       r.SearchMode,
		UnreachableVulns: r.UnreachableVulns,
	}

	for _, vf := range r.VulnFindings {
		if len(vf.Findings) > 0 {
			nvf := &audit.VulnFindings{Vuln: vf.Vuln, Findings: []audit.Finding{vf.Findings[0]}}
			nr.VulnFindings = append(nr.VulnFindings, nvf)
		}
	}
	return nr
}

func writeOut(r *audit.Results, toJson bool) {
	if !toJson {
		os.Stdout.Write([]byte(r.String()))
		return
	}

	b, err := json.MarshalIndent(r, "", "\t")
	if err != nil {
		fmt.Fprintf(os.Stderr, "govulncheck: %s\n", err)
		os.Exit(1)
	}
	os.Stdout.Write(b)
	os.Stdout.Write([]byte{'\n'})
}

func isFile(path string) bool {
	s, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !s.IsDir()
}

func run(cfg *packages.Config, patterns []string, importsOnly bool, dbClient *client.Client) (*audit.Results, error) {
	if len(patterns) == 1 && isFile(patterns[0]) {
		return audit.VulnerablePackageSymbols(patterns[0], dbClient)
	}

	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		return nil, err
	}
	if packages.PrintErrors(pkgs) > 0 {
		return nil, fmt.Errorf("packages contain errors")
	}

	// Compute the findings.
	if importsOnly {
		return audit.VulnerableImports(pkgs, dbClient)
	}
	return audit.VulnerableSymbols(pkgs, dbClient)
}
