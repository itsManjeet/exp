#!/bin/bash

# Copyright 2019 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

# Repository first is used to test the first tag for a major version.

set -euo pipefail
cd "$(dirname "$0")"
source ../repo_functions.bash

repo_create

# v0, v1
cat >go.mod <<EOF
module example.com/first

go 1.13
EOF
echo 'package p' >p.go
git add -A
git commit -m v0
git tag v0

# v0_err
echo 'package ?' >p.go
git commit -am v0_err
git tag v0_err

# v2
git checkout v0 p.go
cat >go.mod <<EOF
module example.com/first/v2

go 1.13
EOF
git commit -am v2
git tag v2

# v2_err
echo 'package ?' >p.go
git commit -am v2_err
git tag v2_err

repo_pack
