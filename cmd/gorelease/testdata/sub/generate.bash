#!/bin/bash

# Copyright 2019 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

# Repository sub has multiple modules in subdirectories at different
# major versions.
#
# Multiple modules may be present in any given commit, but each tag only
# refers to one commit. Each tag has previous tags for the same module
# as its ancestors in linear sequence.
#
#   example.com/sub - root module, no versions
#   example.com/sub/v2 - v2.0.X (subdirectory), v2.1.X (root directory)
#   example.com/sub/nest - nested module with version v1.0.X
#   example.com/sub/nest/v2 - v2.1.X (subdirectory), v2.2.X (root directory)

set -euo pipefail
cd "$(dirname "$0")"
source ../repo_functions.bash

# Create the repository
repo_create

# Create the base modules. Files are empty, so expect syntax errors.
mkdir -p v2 nest/v2
touch sub.go v2/sub.go nest/nest.go nest/v2/nest.go
(cd v2 && go mod init example.com/sub/v2)
(cd nest && go mod init example.com/sub/nest)
(cd nest/v2 && go mod init example.com/sub/nest/v2)
git add -A
git commit -m base
git checkout --detach master

# example.com/sub/v2
cat >v2/sub.go <<EOF
package sub

func A() int { return 1 }
EOF
git commit -am 'example.com/sub/v2 v2.0.0'
git tag v2.0.0

cat >v2/sub.go <<EOF
package sub

func A() int { return 2 }
EOF
git commit -am 'example.com/sub/v2 v2.0.1'
git tag v2.0.1

git mv -f v2/go.mod go.mod
git mv -f v2/sub.go sub.go
rmdir v2
cat >sub.go <<EOF
package sub

func A() int { return 2 }
func B() int { return 0 }
EOF
git commit -am 'example.com/sub/v2 v2.1.0'
git tag v2.1.0

cat >sub.go <<EOF
package sub

func A() int { return 2 }
func B() int { return 1 }
EOF
git commit -am 'example.com/sub/v2 v2.1.1'
git tag v2.1.1

# example.com/sub/nest
git checkout --detach master
cat >nest/nest.go <<EOF
package nest

func A() int { return 1 }
EOF
git commit -am 'example.com/sub/nest v1.0.0'
git tag nest/v1.0.0
cat >nest/nest.go <<EOF
package nest

func A() int { return 2 }
EOF
git commit -am 'example.com/sub/nest v1.0.1'
git tag nest/v1.0.1

# example.com/sub/nest/v2
cat >nest/v2/nest.go <<EOF
package nest

func B() int { return 2 }
EOF
git commit -am 'example.com/sub/nest/v2 v2.0.0'
git tag nest/v2.0.0

git mv -f nest/v2/go.mod nest/go.mod
git mv -f nest/v2/nest.go nest/nest.go
rmdir nest/v2
cat >nest/nest.go <<EOF
package nest

func B() int { return 3 }
EOF
git commit -am 'example.com/sub/nest/v2 v2.0.1'
git tag nest/v2.0.1

# Package the repository
repo_pack
