package vulncheck

import (
	"fmt"
	"go/build"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"
	"golang.org/x/vulndb/client"
)

// extractModules collects modules in `pkgs` up to uniqueness of
// module path and version.
func extractModules(pkgs []*packages.Package) []*packages.Module {
	modMap := map[string]*packages.Module{}
	modKey := func(mod *packages.Module) string {
		if mod.Replace != nil {
			return fmt.Sprintf("%s@%s", mod.Replace.Path, mod.Replace.Version)
		}
		return fmt.Sprintf("%s@%s", mod.Path, mod.Version)
	}

	seen := map[*packages.Package]bool{}
	var extract func(*packages.Package, map[string]*packages.Module)
	extract = func(pkg *packages.Package, modMap map[string]*packages.Module) {
		if pkg == nil || seen[pkg] {
			return
		}
		if pkg.Module != nil {
			modMap[modKey(pkg.Module)] = pkg.Module
		}
		seen[pkg] = true
		for _, imp := range pkg.Imports {
			extract(imp, modMap)
		}
	}
	for _, pkg := range pkgs {
		extract(pkg, modMap)
	}

	modules := []*packages.Module{}
	for _, mod := range modMap {
		modules = append(modules, mod)
	}
	return modules
}

// fetchVulnerabilities fetches vulnerabilities that affect the supplied modules.
func fetchVulnerabilities(client client.Client, modules []*packages.Module) (moduleVulnerabilities, error) {
	mv := moduleVulnerabilities{}
	for _, mod := range modules {
		modPath := mod.Path
		if mod.Replace != nil {
			modPath = mod.Replace.Path
		}

		// skip loading vulns for local imports
		if isLocal(mod) {
			// TODO: what if client has its own db
			// with local vulns?
			continue
		}
		vulns, err := client.GetByModule(modPath)
		if err != nil {
			return nil, err
		}
		if len(vulns) == 0 {
			continue
		}
		mv = append(mv, modVulns{
			mod:   mod,
			vulns: vulns,
		})
	}
	return mv, nil
}

func isLocal(mod *packages.Module) bool {
	modDir := mod.Dir
	if mod.Replace != nil {
		modDir = mod.Replace.Dir
	}
	return !strings.HasPrefix(modDir, modCacheDirectory())
}

func modCacheDirectory() string {
	var modCacheDir string
	// TODO: define modCacheDir using cmd/go/internal/cfg.GOMODCACHE
	if modCacheDir = os.Getenv("GOMODCACHE"); modCacheDir == "" {
		if modCacheDir = os.Getenv("GOPATH"); modCacheDir == "" {
			modCacheDir = build.Default.GOPATH
		}
		modCacheDir = filepath.Join(modCacheDir, "pkg", "mod")
	}
	return modCacheDir
}
