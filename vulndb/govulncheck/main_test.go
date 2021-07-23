// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/exp/vulndb/internal/audit"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/packages/packagestest"
)

// goYamlVuln contains vulnerability info for github.com/go-yaml/yaml package.
var goYamlVuln string = `[{"ID":"GO-2020-0036","Published":"2021-04-14T12:00:00Z","Modified":"2021-04-14T12:00:00Z","Withdrawn":null,"Aliases":["CVE-2019-11254"],"Package":{"Name":"github.com/go-yaml/yaml","Ecosystem":"go"},"Details":"An attacker can craft malicious YAML which will consume significant\nsystem resources when Unmarshalled.\n","Affects":{"Ranges":[{"Type":"SEMVER","Introduced":"","Fixed":"v2.2.8+incompatible"}]},"References":[{"Type":"code review","URL":"https://github.com/go-yaml/yaml/pull/555"},{"Type":"fix","URL":"https://github.com/go-yaml/yaml/commit/53403b58ad1b561927d19068c655246f2db79d48"},{"Type":"misc","URL":"https://bugs.chromium.org/p/oss-fuzz/issues/detail?id=18496"}],"ecosystem_specific":{"Symbols":["yaml_parser_fetch_more_tokens"],"URL":"https://go.googlesource.com/vulndb/+/refs/heads/main/reports/GO-2020-0036.toml"}},{"ID":"GO-2021-0061","Published":"2021-04-14T12:00:00Z","Modified":"2021-04-14T12:00:00Z","Withdrawn":null,"Package":{"Name":"github.com/go-yaml/yaml","Ecosystem":"go"},"Details":"A maliciously crafted input can cause resource exhaustion due to\nalias chasing.\n","Affects":{"Ranges":[{"Type":"SEMVER","Introduced":"","Fixed":"v2.2.3+incompatible"}]},"References":[{"Type":"code review","URL":"https://github.com/go-yaml/yaml/pull/375"},{"Type":"fix","URL":"https://github.com/go-yaml/yaml/commit/bb4e33bf68bf89cad44d386192cbed201f35b241"}],"ecosystem_specific":{"Symbols":["decoder.unmarshal"],"URL":"https://go.googlesource.com/vulndb/+/refs/heads/main/reports/GO-2021-0061.toml"}}]`

// gopkgInYamlVuln contains vulnerability info for gopkg.in/yaml.v2 package.
var gopkgInYamlVuln = `[{"id":"GO-2020-0036","published":"2021-04-14T12:00:00Z","modified":"2021-04-14T12:00:00Z","aliases":["CVE-2019-11254"],"package":{"name":"gopkg.in/yaml.v2","ecosystem":"Go"},"details":"Due to unbounded aliasing, a crafted YAML file can cause consumption\nof significant system resources. If parsing user supplied input, this\nmay be used as a denial of service vector.\n","affects":{"ranges":[{"type":"SEMVER","fixed":"2.2.8"}]},"references":[{"type":"FIX","url":"https://github.com/go-yaml/yaml/pull/555"},{"type":"FIX","url":"https://github.com/go-yaml/yaml/commit/53403b58ad1b561927d19068c655246f2db79d48"},{"type":"WEB","url":"https://bugs.chromium.org/p/oss-fuzz/issues/detail?id=18496"}],"ecosystem_specific":{"symbols":["yaml_parser_fetch_more_tokens"],"url":"https://go.googlesource.com/vulndb/+/refs/heads/master/reports/GO-2020-0036.yaml"}},{"id":"GO-2021-0061","published":"2021-04-14T12:00:00Z","modified":"2021-04-14T12:00:00Z","package":{"name":"gopkg.in/yaml.v2","ecosystem":"Go"},"details":"Due to unbounded alias chasing, a maliciously crafted YAML file\ncan cause the system to consume significant system resources. If\nparsing user input, this may be used as a denial of service vector.\n","affects":{"ranges":[{"type":"SEMVER","fixed":"2.2.3"}]},"references":[{"type":"FIX","url":"https://github.com/go-yaml/yaml/pull/375"},{"type":"FIX","url":"https://github.com/go-yaml/yaml/commit/bb4e33bf68bf89cad44d386192cbed201f35b241"}],"ecosystem_specific":{"symbols":["decoder.unmarshal"],"url":"https://go.googlesource.com/vulndb/+/refs/heads/master/reports/GO-2021-0061.yaml"}}]`

// index for dbs containing some entries for each vuln module.
// The timestamp for module is set to random moment in the past.
var index string = `{
	"github.com/go-yaml/yaml": "2021-01-01T12:00:00.000000000-08:00",
	"gopkg.in/yaml.v2": "2021-01-01T12:00:00.000000000-08:00"
}`

var vulns = map[string]string{
	"github.com/go-yaml/yaml.json": goYamlVuln,
	"gopkg.in/yaml.v2.json":        gopkgInYamlVuln,
}

// finding abstraction of Finding, for test purposes.
type finding struct {
	symbol   string
	traceLen int
}

func testFindings(finds []audit.Finding) []finding {
	var fs []finding
	for _, f := range finds {
		fs = append(fs, finding{symbol: f.Symbol, traceLen: len(f.Trace)})
	}
	return fs
}

func subset(finds1, finds2 []finding) bool {
	fs2 := make(map[finding]bool)
	for _, f := range finds2 {
		fs2[f] = true
	}

	for _, f := range finds1 {
		if !fs2[f] {
			return false
		}
	}
	return true
}

func allFindings(r *audit.Results) []audit.Finding {
	var findings []audit.Finding
	for _, v := range r.Vulnerabilities {
		for _, f := range r.VulnFindings[v.ID] {
			findings = append(findings, f)
		}
	}
	return findings
}

// staticCallStacks returns true if `findings` is an empty
// list or each finding has call stacks involving exclusively
// static call sites.
func staticCallStacks(findings []audit.Finding) bool {
	for _, f := range findings {
		for _, te := range f.Trace {
			if strings.Contains(te.Description, "approx. resolved") {
				return false
			}
		}
	}
	return true
}

// TestStaticCallStackFindings checks if govulncheck finds uses
// of test vulnerabilities in a real world package. Each use is
// expected to involve static call stacks exclusively.
func TestStaticCallStackFindings(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	e := packagestest.Export(t, packagestest.Modules, []packagestest.Module{
		{
			Name: "foo",
		},
	})
	defer e.Cleanup()

	hashiVaultOkta := "github.com/hashicorp/vault/builtin/credential/okta"

	// Go get hashicorp-vault okta package v1.6.3.
	env := envUpdate(e.Config.Env, "GOPROXY", "https://proxy.golang.org,direct")
	if out, err := execCmd(e.Config.Dir, env, "go", "get", hashiVaultOkta+"@v1.6.3"); err != nil {
		t.Logf("failed to get %s: %s", hashiVaultOkta+"@v1.6.3", out)
		t.Fatal(err)
	}

	// run govulncheck.
	cfg := &packages.Config{
		Mode:  packages.LoadAllSyntax | packages.NeedModule,
		Tests: false,
		Dir:   e.Config.Dir,
	}

	// Create a local filesystem db.
	dbPath := filepath.Join(e.Config.Dir, "db")
	addToLocalDb(dbPath, "index.json", index)
	// Create a local server db.
	sMux := http.NewServeMux()
	s := http.Server{Addr: ":8080", Handler: sMux}
	go func() { s.ListenAndServe() }()
	defer func() { s.Shutdown(context.Background()) }()
	addToServerDb(sMux, "index.json", index)

	for _, test := range []struct {
		source string
		// list of packages whose vulns should be addded to source
		toAdd []string
		want  []finding
	}{
		// test local db with gopkgInYaml but without goYaml, which should result in no findings.
		{source: "file://" + dbPath, want: nil,
			toAdd: []string{"gopkg.in/yaml.v2.json"}},
		// add yaml to the local db, which should produce 2 findings.
		{source: "file://" + dbPath, toAdd: []string{"github.com/go-yaml/yaml.json"},
			want: []finding{
				{"github.com/go-yaml/yaml.decoder.unmarshal", 6},
				{"github.com/go-yaml/yaml.yaml_parser_fetch_more_tokens", 12}},
		},
		// repeat the similar experiment with a server db.
		{source: "http://localhost:8080", toAdd: []string{"gopkg.in/yaml.v2.json"}, want: nil},
		{source: "http://localhost:8080", toAdd: []string{"github.com/go-yaml/yaml.json"},
			want: []finding{
				{"github.com/go-yaml/yaml.decoder.unmarshal", 6},
				{"github.com/go-yaml/yaml.yaml_parser_fetch_more_tokens", 12}},
		},
	} {
		for _, add := range test.toAdd {
			if strings.HasPrefix(test.source, "file://") {
				addToLocalDb(dbPath, add, vulns[add])
			} else {
				addToServerDb(sMux, add, vulns[add])
			}
		}

		r, err := run(cfg, []string{hashiVaultOkta}, false, []string{test.source})
		if err != nil {
			t.Fatal(err)
		}

		findings := allFindings(r)
		if len(findings) != 0 && !staticCallStacks(findings) {
			t.Errorf("want all static traces; got some dynamic ones")
		}

		if fs := testFindings(findings); !subset(test.want, fs) {
			t.Errorf("want %v subset of findings; got %v", test.want, fs)
		}
	}
}

// TestDynamicCallStackFindings checks that govulncheck finds
// uses of test vulnerabilities in a real world package. Each
// use is expected to involve a dynamic call stack.
func TestDynamicCallStackFindings(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	e := packagestest.Export(t, packagestest.Modules, []packagestest.Module{
		{
			Name: "foo",
		},
	})
	defer e.Cleanup()

	goAuthServer := "github.com/RichardKnop/go-oauth2-server"

	// Go get go-oauth package v1.0.4.
	env := envUpdate(e.Config.Env, "GOPROXY", "https://proxy.golang.org,direct")
	if out, err := execCmd(e.Config.Dir, env, "go", "get", goAuthServer+"@v1.0.4"); err != nil {
		t.Logf("failed to get %s: %s", goAuthServer+"@v1.0.4", out)
		t.Fatal(err)
	}

	// run govulncheck.
	cfg := &packages.Config{
		Mode:  packages.LoadAllSyntax | packages.NeedModule,
		Tests: false,
		Dir:   e.Config.Dir,
	}

	// Create a local filesystem db.
	dbPath := filepath.Join(e.Config.Dir, "db")
	addToLocalDb(dbPath, "index.json", index)
	// Create a local server db.
	sMux := http.NewServeMux()
	s := http.Server{Addr: ":8080", Handler: sMux}
	go func() { s.ListenAndServe() }()
	defer func() { s.Shutdown(context.Background()) }()
	addToServerDb(sMux, "index.json", index)

	for _, test := range []struct {
		source string
		// list of packages whose vulns should be addded to source
		toAdd []string
		want  []finding
	}{
		// Test local db without gopkinYaml but with goYaml, which should result in no findings.
		// This is the flip side of the TestHashicorpVault.
		{source: "file://" + dbPath, want: nil,
			toAdd: []string{"github.com/go-yaml/yaml.json"}},
		// add yaml to the local db, which should produce 2 findings.
		{source: "file://" + dbPath, toAdd: []string{"gopkg.in/yaml.v2.json"},
			want: []finding{
				{"gopkg.in/yaml.v2.decoder.unmarshal", 10},
				{"gopkg.in/yaml.v2.yaml_parser_fetch_more_tokens", 16}},
		},
		// repeat the similar experiment with a server db.
		{source: "http://localhost:8080", toAdd: []string{"github.com/go-yaml/yaml.json"}, want: nil},
		{source: "http://localhost:8080", toAdd: []string{"gopkg.in/yaml.v2.json"},
			want: []finding{
				{"gopkg.in/yaml.v2.decoder.unmarshal", 10},
				{"gopkg.in/yaml.v2.yaml_parser_fetch_more_tokens", 16}},
		},
	} {
		for _, add := range test.toAdd {
			if strings.HasPrefix(test.source, "file://") {
				addToLocalDb(dbPath, add, vulns[add])
			} else {
				addToServerDb(sMux, add, vulns[add])
			}
		}

		r, err := run(cfg, []string{goAuthServer}, false, []string{test.source})
		if err != nil {
			t.Fatal(err)
		}

		findings := allFindings(r)
		if len(findings) != 0 && staticCallStacks(findings) {
			t.Errorf("want all dynamic traces; got some static ones")
		}

		if fs := testFindings(findings); !subset(test.want, fs) {
			t.Errorf("want %v subset of findings; got %v", test.want, fs)
		}
	}
}

// addToLocalDb adds vuln for package p to local db at path db.
func addToLocalDb(db, p, vuln string) error {
	if err := os.MkdirAll(filepath.Join(db, filepath.Dir(p)), fs.ModePerm); err != nil {
		return err
	}

	f, err := os.Create(filepath.Join(db, p))
	if err != nil {
		return err
	}
	defer f.Close()

	f.Write([]byte(vuln))
	return nil
}

// addToServerDb adds vuln for package p to localhost server identified by its handler.
func addToServerDb(handler *http.ServeMux, p, vuln string) {
	handler.HandleFunc("/"+p, func(w http.ResponseWriter, req *http.Request) { fmt.Fprint(w, vuln) })
}

// envUpdate updates an environment e by setting the key to value.
func envUpdate(e []string, key, value string) []string {
	var nenv []string
	for _, kv := range e {
		if strings.HasPrefix(kv, key+"=") {
			nenv = append(nenv, key+"="+value)
		} else {
			nenv = append(nenv, kv)
		}
	}
	return nenv
}

// execCmd runs the command name with arg in dir location with the env environment.
func execCmd(dir string, env []string, name string, arg ...string) ([]byte, error) {
	cmd := exec.Command(name, arg...)
	cmd.Dir = dir
	cmd.Env = env
	return cmd.CombinedOutput()
}
