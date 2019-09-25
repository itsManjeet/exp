#!/bin/bash

# Copyright 2019 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

# Repository tidy contains a module that is not tidy.

set -euo pipefail
cd "$(dirname "$0")"
source ../repo_functions.bash

repo_create

# v0.0.1: no requirements, no go.sum.
cat >go.mod <<EOF
module example.com/tidy

go 1.13
EOF
echo 'package tidy' >tidy.go
git add go.mod tidy.go
git commit -m tidy
git tag v0.0.1

# missing go statement.
echo 'module example.com/tidy' >go.mod
git commit -am missing_go
git tag missing_go

# missing requirement
git checkout v0.0.1
cat >tidy.go <<EOF
package tidy

import _ "rsc.io/quote"
EOF
git commit -am missing_req
git tag missing_req

# extra req
git checkout v0.0.1
cat >go.mod <<EOF
module example.com/tidy

go 1.13

require rsc.io/quote v1.5.2
EOF
cat >go.sum <<EOF
golang.org/x/text v0.0.0-20170915032832-14c0d48ead0c/go.mod h1:NqM8EUOU14njkJ3fqMW+pc6Ldnwhi/IjpwHt7yyuwOQ=
rsc.io/quote v1.5.2/go.mod h1:LzX7hefJvL54yjefDEDHNONDjII0t9xZLPXsUe+TKr0=
rsc.io/sampler v1.3.0/go.mod h1:T1hPZKmBbMNahiBKFy5HrXp6adAjACjK9JXDnKaTXpA=
EOF
git add -A
git commit -m extra_req
git tag extra_req

# no sum
git checkout v0.0.1
cat >go.mod <<EOF
module example.com/tidy

go 1.13

require rsc.io/quote v1.5.2
EOF
cat >tidy.go <<EOF
package tidy

import _ "rsc.io/quote"
EOF
git add -A
git commit -m no_sum
git tag no_sum

# empty sum
echo -n >go.sum
git add go.sum
git commit -m empty_sum
git tag empty_sum

repo_pack
