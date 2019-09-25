#!/bin/bash

# Copyright 2019 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

# Repository basic has the same series of changes on three major versions:
# v0, v1, and v2:
#
#   vX.0.1 - initial commit: simple package
#   vX.1.0 - compatible change: add a function and a package
#   vX.1.1 - internal change: function returns different value
#   vX.1.2 - incompatible change: delete a function and a package

set -euo pipefail
cd "$(dirname "$0")"
source ../repo_functions.bash

# Create the repository
repo_create

for major in v0 v1 v2; do
  # Clear any previous changes.
  rm -rf a b

  # Set mod path correctly for v2
  if [ "$major" = v2 ]; then
    go mod edit -module="$(go list -m)"/v2
  fi

  # vX.0.1: simple package
  mkdir a
  cat >a/a.go <<EOF
package a

func A() int { return 0 }
EOF
  git add -A
  git commit -m 'simple package'
  git tag $major.0.1

  # vX.1.0: compatible change: add a function and a package
  cat >a/a.go <<EOF
package a

func A() int { return 0 }
func A2() int { return 2 }
EOF
  mkdir b
  cat >b/b.go <<EOF
package b

func B() int { return 3 }
EOF
  git add -A
  git commit -m 'compatible change'
  git tag $major.1.0

  # vX.1.1: internal change: function returns different value
  cat >a/a.go <<EOF
package a

func A() int { return 1 }
func A2() int { return 2 }
EOF
  git add -A
  git commit -m 'internal change'
  git tag $major.1.1

  # vX.1.2: incompatible change: delete a function and a package
  git rm -r b
  cat >a/a.go <<EOF
package a

func A() int { return 1 }
EOF
  git add -A
  git commit -m 'incompatible change'
  git tag $major.1.2
done

# Package the repository
repo_pack
