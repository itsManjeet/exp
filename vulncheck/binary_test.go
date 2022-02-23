// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vulncheck

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages/packagestest"
)

// TestCallGraph checks for call graph vuln slicing correctness.
// The inlined test code has the following call graph
//
//          x.X
//        /  |  \
//       /  d.D1 avuln.VulnData.Vuln1
//      /  /  |
//     c.C1  d.internal.Vuln1
//      |
//    avuln.VulnData.Vuln2
//
//         --------------------y.Y-------------------------------
//        /           /              \         \         \       \
//       /           /                \         \         \       \
//      /           /                  \         \         \       \
//    c.C4 c.vulnWrap.V.Vuln1(=nil)   c.C2   bvuln.Vuln   c.C3   c.C3$1
//      |                                       | |
//  y.benign                                    e.E
//
// and this slice
//
//          x.X
//        /  |  \
//       /  d.D1 avuln.VulnData.Vuln1
//      /  /
//     c.C1
//      |
//    avuln.VulnData.Vuln2
//
//     y.Y
//      |
//  bvuln.Vuln
//     | |
//     e.E
// related to avuln.VulnData.{Vuln1, Vuln2} and bvuln.Vuln vulnerabilities.
//
// TODO: we build binary progrmatically? What if the underlying tool chain changes?
func TestBinary(t *testing.T) {
	e := packagestest.Export(t, packagestest.Modules, []packagestest.Module{
		{
			Name: "golang.org/entry",
			Files: map[string]interface{}{
				"main.go": `
			package main

			import (
				"golang.org/cmod/c"
				"golang.org/bmod/bvuln"
			)

			func main() {
				c.C()
				bvuln.NoVuln() // no vuln use
				print("done")
			}
			`,
			}},
		{
			Name: "golang.org/cmod@v1.1.3",
			Files: map[string]interface{}{"c/c.go": `
			package c

			import (
				"golang.org/amod/avuln"
			)

			//go:noinline
			func C() {
				v := avuln.VulnData{}
				v.Vuln1() // vuln use
			}
			`},
		},
		{
			Name: "golang.org/amod@v1.1.3",
			Files: map[string]interface{}{"avuln/avuln.go": `
			package avuln

			type VulnData struct {}

			//go:noinline
			func (v VulnData) Vuln1() {}

			//go:noinline
			func (v VulnData) Vuln2() {}
			`},
		},
		{
			Name: "golang.org/bmod@v0.5.0",
			Files: map[string]interface{}{"bvuln/bvuln.go": `
			package bvuln

			//go:noinline
			func Vuln() {}

			//go:noinline
			func NoVuln() {}
			`},
		},
	})
	defer e.Cleanup()

	// Make sure local vulns can be loaded.
	fetchingInTesting = true

	cmd := exec.Command("go", "build")
	cmd.Dir = e.Config.Dir
	// cmd.Env = e.Config.Env
	cmd.Env = set("GOMODCACHE", path.Join(e.Config.Dir, "../modcache/pkg/mod"), e.Config.Env)
	cmd.Env = set("GOSUMDB", "sum.golang.org", cmd.Env)
	fmt.Printf("%v\n", cmd.Env)
	out, err := cmd.CombinedOutput()
	if err != nil || len(out) > 0 {
		t.Fatalf("failed to build the binary %v %v", err, string(out))
	}

	bin, err := os.Open(path.Join(e.Config.Dir, "entry"))
	if err != nil {
		t.Fatalf("error opening the binary %v", err)
	}
	defer bin.Close()

	cfg := &Config{
		Client:      testClient,
		ImportsOnly: true,
	}
	res, err := Binary(context.Background(), bin, cfg)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("%v\n", res)

	ls := exec.Command("./entry")
	ls.Dir = e.Config.Dir
	lout, err := ls.CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("%v \n %v\n", ls.Dir, string(lout))
}

func set(evar, eval string, env []string) []string {
	var nenv []string
	for _, e := range env {
		parts := strings.Split(e, "=")
		if len(parts) != 2 || parts[0] != evar {
			nenv = append(nenv, e)
			continue
		}
		nenv = append(nenv, evar+"="+eval)
	}
	return nenv
}

func get(evar string, env []string) string {
	for _, e := range env {
		parts := strings.Split(e, "=")
		if len(parts) == 2 && parts[0] == evar {
			return parts[1]
		}
	}
	return ""
}
