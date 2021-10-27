package vulncheck

import (
	"fmt"
	"testing"

	"golang.org/x/tools/go/packages/packagestest"
)

func TestImportsOnly(t *testing.T) {
	e := packagestest.Export(t, packagestest.Modules, []packagestest.Module{
		{
			Name: "golang.org/x",
			Files: map[string]interface{}{"x/x.go": `
			package x

			import "golang.org/avulnmod/avuln"

			func X() {
				avuln.VulnData{}.Vuln1()
			}
			`},
		},
		{
			Name: "golang.org/y",
			Files: map[string]interface{}{"y/y.go": `
			package y

			import (
				"golang.org/avulnmod/avuln"
				"golang.org/z"
			)

			func Y() {
				avuln.VulnData{}.Vuln2()
				z.Z()
			}
			`},
		},
		{
			Name: "golang.org/z",
			Files: map[string]interface{}{"z/z.go": `
			package z

			func Z() {}
			`},
		},
		{
			Name: "golang.org/avulnmod@v1.0.1",
			Files: map[string]interface{}{"avuln/avuln.go": `
			package avuln

			import "golang.org/w"

			type VulnData struct {}
			func (v VulnData) Vuln1() { w.W() }
			func (v VulnData) Vuln2() {}
			`},
		},
		{
			Name: "golang.org/bvulnmod@v0.5.0",
			Files: map[string]interface{}{"bvuln/bvuln.go": `
			package bvuln

			func Vuln() {}
			`},
		},
		{
			Name: "golang.org/w",
			Files: map[string]interface{}{"w/w.go": `
			package w

			import "golang.org/bvulnmod/bvuln"

			func W() { bvuln.Vuln() }
			`},
		},
	})
	defer e.Cleanup()

	pkgs, err := loadPackages(e, "x", "y")
	if err != nil {
		t.Fatal(err)
	}

	cfg := &Config{
		Client:      testClient,
		ImportsOnly: true,
	}
	result, err := Source(pkgs, cfg)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(result.Vulns)
}
