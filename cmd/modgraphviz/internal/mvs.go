package internal

import "sort"

// A Version is defined by a module path and version pair.
type Version struct {
	Path string

	// Version is usually a semantic version in canonical form.
	// There are two exceptions to this general rule.
	// First, the top-level target of a build has no specific version
	// and uses Version = "".
	// Second, during MVS calculations the version "none" is used
	// to represent the decision to take no version of a given module.
	Version string `json:",omitempty"`
}

// A Reqs is the requirement graph on which Minimal Version Selection (MVS) operates.
//
// The version strings are opaque except for the special version "none"
// (see the documentation for module.Version). In particular, MVS does not
// assume that the version strings are semantic versions; instead, the Max method
// gives access to the comparison operation.
//
// It must be safe to call methods on a Reqs from multiple goroutines simultaneously.
// Because a Reqs may read the underlying graph from the network on demand,
// the MVS algorithms parallelize the traversal to overlap network delays.
type Reqs interface {
	// Required returns the module versions explicitly required by m itself.
	// The caller must not modify the returned list.
	Required(m Version) ([]Version, error)

	// Max returns the maximum of v1 and v2 (it returns either v1 or v2).
	//
	// For all versions v, Max(v, "none") must be v,
	// and for the tanget passed as the first argument to MVS functions,
	// Max(target, v) must be target.
	//
	// Note that v1 < v2 can be written Max(v1, v2) != v1
	// and similarly v1 <= v2 can be written Max(v1, v2) == v2.
	Max(v1, v2 string) string

	// Upgrade returns the upgraded version of m,
	// for use during an UpgradeAll operation.
	// If m should be kept as is, Upgrade returns m.
	// If m is not yet used in the build, then m.Version will be "none".
	// More typically, m.Version will be the version required
	// by some other module in the build.
	//
	// If no module version is available for the given path,
	// Upgrade returns a non-nil error.
	// TODO(rsc): Upgrade must be able to return errors,
	// but should "no latest version" just return m instead?
	Upgrade(m Version) (Version, error)

	// Previous returns the version of m.Path immediately prior to m.Version,
	// or "none" if no such version is known.
	Previous(m Version) (Version, error)
}

// Req returns the minimal requirement list for the target module
// that results in the given build list, with the constraint that all
// module paths listed in base must appear in the returned list.
func Req(target Version, list []Version, base []string, reqs Reqs) ([]Version, error) {
	// Note: Not running in parallel because we assume
	// that list came from a previous operation that paged
	// in all the requirements, so there's no I/O to overlap now.

	// Compute postorder, cache requirements.
	var postorder []Version
	reqCache := map[Version][]Version{}
	reqCache[target] = nil
	var walk func(Version) error
	walk = func(m Version) error {
		_, ok := reqCache[m]
		if ok {
			return nil
		}
		required, err := reqs.Required(m)
		if err != nil {
			return err
		}
		reqCache[m] = required
		for _, m1 := range required {
			if err := walk(m1); err != nil {
				return err
			}
		}
		postorder = append(postorder, m)
		return nil
	}
	for _, m := range list {
		if err := walk(m); err != nil {
			return nil, err
		}
	}

	// Walk modules in reverse post-order, only adding those not implied already.
	have := map[string]string{}
	walk = func(m Version) error {
		if v, ok := have[m.Path]; ok && reqs.Max(m.Version, v) == v {
			return nil
		}
		have[m.Path] = m.Version
		for _, m1 := range reqCache[m] {
			walk(m1)
		}
		return nil
	}
	max := map[string]string{}
	for _, m := range list {
		if v, ok := max[m.Path]; ok {
			max[m.Path] = reqs.Max(m.Version, v)
		} else {
			max[m.Path] = m.Version
		}
	}
	// First walk the base modules that must be listed.
	var min []Version
	for _, path := range base {
		m := Version{Path: path, Version: max[path]}
		min = append(min, m)
		walk(m)
	}
	// Now the reverse postorder to bring in anything else.
	for i := len(postorder) - 1; i >= 0; i-- {
		m := postorder[i]
		if max[m.Path] != m.Version {
			// Older version.
			continue
		}
		if have[m.Path] != m.Version {
			min = append(min, m)
			walk(m)
		}
	}
	sort.Slice(min, func(i, j int) bool {
		return min[i].Path < min[j].Path
	})
	return min, nil
}
