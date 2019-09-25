#!/bin/bash

# Copyright 2019 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

# Repository "errors" has the following packages and versions:
#
# * package "fixed" has type errors in v0.1.0, fixed in v0.2.0.
# * package "deleted" has type errors in v0.1.0, deleted in v0.2.0.
# * package "broken" is correct in v0.1.0, has type errors in v0.2.0.
# * package "added" doesn't exist in v0.1.0, has type errors in v0.2.0.

set -euo pipefail
cd "$(dirname "$0")"
source ../repo_functions.bash

# Create the repository.
repo_create

# v0.1.0
mkdir fixed
cat >fixed/fixed.go <<EOF
package fixed

const X int = "not an int"
EOF

mkdir deleted
cat >deleted/deleted.go <<EOF
package deleted

const X int = "not an int"
EOF

mkdir broken
cat >broken/broken.go <<EOF
package broken

const X int = 12
EOF

git add -A
git commit -m 'v0.1.0'
git tag v0.1.0

# v0.2.0
cat >fixed/fixed.go <<EOF
package fixed

const X int = 12
EOF

git rm -r deleted

cat >broken/broken.go <<EOF
package broken

const X int = "no longer an int"
EOF

mkdir added
cat >added/added.go <<EOF
package added

const X int = "not an int"
EOF

git add -A
git commit -m 'v0.2.0'
git tag v0.2.0

# Package the repository
repo_pack
