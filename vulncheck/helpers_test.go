package vulncheck

import (
	"fmt"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/packages/packagestest"
	"golang.org/x/vulndb/osv"
)

type mockClient struct {
	ret map[string][]*osv.Entry
}

func (mc *mockClient) GetByModule(a string) ([]*osv.Entry, error) {
	return mc.ret[a], nil
}

func (mc *mockClient) GetByID(a string) (*osv.Entry, error) {
	return nil, nil
}

var testClient = &mockClient{
	ret: map[string][]*osv.Entry{
		"golang.org/avulnmod": []*osv.Entry{
			{
				ID: "VA",
				Affected: []osv.Affected{{
					Package:           osv.Package{Name: "golang.org/avulnmod/avuln"},
					Ranges:            osv.Affects{{Type: osv.TypeSemver, Events: []osv.RangeEvent{{Introduced: "1.0.0"}, {Fixed: "1.0.4"}, {Introduced: "1.1.2"}}}},
					EcosystemSpecific: osv.EcosystemSpecific{Symbols: []string{"VulnData.Vuln1", "VulnData.Vuln2"}},
				}},
			},
		},
		"golang.org/bvulnmod": []*osv.Entry{
			{
				ID: "VB",
				Affected: []osv.Affected{{
					Package:           osv.Package{Name: "golang.org/bvulnmod/bvuln"},
					Ranges:            osv.Affects{{Type: osv.TypeSemver}},
					EcosystemSpecific: osv.EcosystemSpecific{Symbols: []string{"Vuln"}},
				}},
			},
		},
	},
}

func loadPackages(e *packagestest.Exported, patterns ...string) ([]*packages.Package, error) {
	e.Config.Mode |= packages.NeedModule | packages.NeedName | packages.NeedFiles |
		packages.NeedCompiledGoFiles | packages.NeedImports | packages.NeedTypes |
		packages.NeedTypesSizes | packages.NeedSyntax | packages.NeedTypesInfo | packages.NeedDeps
	return packages.Load(e.Config, patterns...)
}

func moduleVulnerabilitiesToString(mv moduleVulnerabilities) string {
	var s string
	for _, m := range mv {
		s += fmt.Sprintf("mod: %v\n", m.mod)
		for _, v := range m.vulns {
			s += fmt.Sprintf("\t%v\n", v)
		}
	}
	return s
}

func vulnsToString(vulns []*osv.Entry) string {
	var s string
	for _, v := range vulns {
		s += fmt.Sprintf("\t%v\n", v)
	}
	return s
}
