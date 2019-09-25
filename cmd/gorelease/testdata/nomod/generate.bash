#!/bin/bash

# Copyright 2019 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

# Repository nomod is used to test situations where no go.mod is present.

set -euo pipefail
cd "$(dirname "$0")"
source ../repo_functions.bash

repo_create

# v0.0.1: packages p and sub/q with no go.mod.
rm go.mod
mkdir -p p
echo 'package p' >p/p.go
git add -A
git commit -m v0.0.1
git tag v0.0.1

# v0.0.2: add root go.mod
cat >go.mod <<EOF
module example.com/nomod

go 1.13
EOF
git add go.mod
git commit -m v0.0.2
git tag v0.0.2

repo_pack
