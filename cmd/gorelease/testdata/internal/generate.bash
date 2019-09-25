#!/bin/bash

# Copyright 2019 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

# Repository internal is used to test that changes in internal packages
# are not reported unless they affect the exported API of non-internal packages.

set -euo pipefail
cd "$(dirname "$0")"
source ../repo_functions.bash

repo_create

# v1.0.0: create package p and internal/i.
cat >go.mod <<EOF
module example.com/m

go 1.13
EOF

mkdir p
cat >p/p.go <<EOF
package p

import "example.com/m/internal/i"

type P i.I
EOF

mkdir -p internal/i
cat >internal/i/i.go <<EOF
package i

type I interface{}

func Delete() {}
EOF
git add -A
git commit -m v1.0.0
git tag v1.0.0

# v1.0.1: unreported change: delete unused function in internal/i.
cat >internal/i/i.go <<EOF
package i

type I interface{}
EOF
git commit -am v1.0.1
git tag v1.0.1

# incompatible change: add method to internal/i.
cat >internal/i/i.go <<EOF
package i

type I interface{
  Foo()
}
EOF
git commit -am incompatible
git tag incompatible

repo_pack
