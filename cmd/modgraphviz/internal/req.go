package internal

import (
	"fmt"
	"sort"
	"strings"
)

type ReqsMap map[Version][]Version

func (r ReqsMap) Max(v1, v2 string) string {
	if v1 == "none" || v2 == "" {
		return v2
	}
	if v2 == "none" || v1 == "" {
		return v1
	}
	if v1 < v2 {
		return v2
	}
	return v1
}

func (r ReqsMap) Upgrade(m Version) (Version, error) {
	var u Version
	for k := range r {
		if k.Path == m.Path && u.Version < k.Version && !strings.HasSuffix(k.Version, ".hidden") {
			u = k
		}
	}
	if u.Path == "" {
		return Version{}, fmt.Errorf("missing module: %v", Version{Path: m.Path})
	}
	return u, nil
}

func (r ReqsMap) versions(needlePath string) []string {
	var versions []string
	for from, tos := range r {
		if from.Path == needlePath {
			versions = append(versions, from.Version)
		}
		for _, to := range tos {
			if to.Path == needlePath {
				versions = append(versions, to.Version)
			}
		}
	}
	return versions
}

func (r ReqsMap) Previous(m Version) (Version, error) {
	list := r.versions(m.Path)
	i := sort.Search(len(list), func(i int) bool { return Compare(list[i], m.Version) >= 0 })
	if i > 0 {
		return Version{Path: m.Path, Version: list[i-1]}, nil
	}
	return Version{Path: m.Path, Version: "none"}, nil
}

func (r ReqsMap) Required(m Version) ([]Version, error) {
	rr, ok := r[m]
	if !ok {
		return nil, fmt.Errorf("missing module: %v", m)
	}
	return rr, nil
}
