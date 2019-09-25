#!/bin/bash

# Copyright 2019 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

# Repository detectbase is used test automatic detection of the base version.

set -euo pipefail
cd "$(dirname "$0")"
source ../repo_functions.bash

repo_create

# Create the base modules. No versions are reachable from here.
mkdir sub
(cd sub; go mod init example.com/detectbase/sub)
git add -A
git commit -m 'initial commit'
git tag none

# Create a sequence of commits where one version is visible.
git checkout --detach master
git commit --allow-empty -m 'one'
git tag v1.0.0
git tag sub/v1.0.0
git tag one
git commit --allow-empty -m 'one-after'
git tag one-after

# Create a sequence of commits where multiple versions are visible.
git checkout --detach master
git commit --allow-empty -m 'two-1'
git tag v1.1.0
git tag sub/v1.1.0
git checkout --detach master
git commit --allow-empty -m 'two-2'
git tag v1.1.1
git tag sub/v1.1.1
git merge -m 'two' v1.1.0
git tag two

repo_pack
