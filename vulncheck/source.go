package vulncheck

import "golang.org/x/tools/go/packages"

func Source(pkgs []*packages.Package, cfg *Config) (*Result, error) {
	if !cfg.ImportsOnly {
		panic("call graph feature is currently unsupported")
	}

	_, err := fetchVulnerabilities(cfg.Client, extractModules(pkgs))
	if err != nil {
		return nil, err
	}
	return nil, nil
}
