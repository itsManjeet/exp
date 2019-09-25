#!/bin/bash

# Copyright 2019 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

set -euo pipefail
cd "$(dirname "$0")"
source ../repo_functions.bash

# Create the repository.
repo_create

# Create a file without committing it.
cat >untracked.go <<EOF
package untracked
EOF

# Package the repository.
repo_pack
