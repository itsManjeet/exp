#!/bin/bash

# Copyright 2019 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

set -euo pipefail
cd "$(dirname "$0")"
source ../repo_functions.bash

# Create the repository
repo_create

# abspath: module path is absolute path
cat >go.mod <<EOF
module /home/gopher/go/src/mymod
EOF
git commit -am abspath
git tag abspath

# badmajor: module path has invalid major version
cat >go.mod <<EOF
module example.com/patherrors/v0
EOF
git commit -am badmajor
git tag badmajor

# gopkginsub: gopkgin path in subdirectory
git rm go.mod
mkdir yaml
cat >yaml/go.mod <<EOF
module gopkg.in/yaml.v2
EOF
git add -A
git commit -m gopkginsub
git tag gopkginsub

# pathsub: path does not have subdirectory as a suffix
git rm -r yaml
mkdir x
cat >x/go.mod <<EOF
module example.com/y
EOF
git add -A
git commit -m pathsub
git tag pathsub

# pathsubv2: path does not have subdirectory as a suffix
cat >x/go.mod <<EOF
module example.com/y/v2
EOF
git commit -am pathsubv2
git tag pathsubv2

# example.com/{a,b}: module path is valid but changes
git rm -r x
echo 'module example.com/a' >go.mod
git add go.mod
git commit -m valid-a
git tag v0.0.1
echo 'module example.com/b' >go.mod
git commit -am valid-b
git tag valid-b

# Package the repository
repo_pack
