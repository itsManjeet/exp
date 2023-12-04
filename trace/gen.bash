#!/usr/bin/env bash
# Copyright 2023 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

# This script copies this directory to golang.org/x/exp/trace.
# Just point it at a Go commit.

set -e

if [ "$#" -ne 1 ]; then
    echo 'mkexp.bash expects one argument: a go.googlesource.com/go commit hash to generate the package from'
	exit 1
fi

# Check out Go.
GODIR=$(mktemp -d)
git -C $GODIR clone https://go.googlesource.com/go
git -C $GODIR/go checkout $1

# Define src and dst.
SRC=$GODIR/go/src/internal/trace/v2
DST=$(dirname $0)

# Copy.
cp -r $SRC/* $DST
rm $DST/mkexp.bash

# Remove the trace_test.go file. This really tests the tracer and is not necessary to bring along.
rm $DST/trace_test.go

# Make some packages internal.
mv $DST/raw $DST/internal/raw
mv $DST/event $DST/internal/event
mv $DST/version $DST/internal/version
mv $DST/testtrace $DST/internal/testtrace

# Move the debug commands out of testdata.
mv $DST/testdata/cmd $DST/cmd

# Fix up import paths.
find $DST -name '*.go' | xargs -- sed -i 's/internal\/trace\/v2/golang.org\/x\/exp\/trace/'
find $DST -name '*.go' | xargs -- sed -i 's/golang.org\/x\/exp\/trace\/raw/golang.org\/x\/exp\/trace\/internal\/raw/'
find $DST -name '*.go' | xargs -- sed -i 's/golang.org\/x\/exp\/trace\/event/golang.org\/x\/exp\/trace\/internal\/event/'
find $DST -name '*.go' | xargs -- sed -i 's/golang.org\/x\/exp\/trace\/event\/go122/golang.org\/x\/exp\/trace\/internal\/event\/go122/'
find $DST -name '*.go' | xargs -- sed -i 's/golang.org\/x\/exp\/trace\/version/golang.org\/x\/exp\/trace\/internal\/version/'
find $DST -name '*.go' | xargs -- sed -i 's/golang.org\/x\/exp\/trace\/testtrace/golang.org\/x\/exp\/trace\/internal\/testtrace/'
find $DST -name '*.go' | xargs -- sed -i 's/internal\/txtar/golang.org\/x\/tools\/txtar/'

# Format the files.
find $DST -name '*.go' | xargs -- gofmt -w -s

# Clean up.
rm -r $GODIR
