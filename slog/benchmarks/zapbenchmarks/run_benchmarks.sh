#!/bin/bash -e

GO=go
if [[ $1 != '' ]]; then
  GO=$1
fi

cd $(dirname $0)

set -x

# Run all benchmarks a few times and capture to a file.
$GO test -bench . -count 5 > zap.bench

# Rename the package in the output to fool benchstat into comparing
# these benchmarks with the ones in the parent directory.
sed -i -e 's?^pkg: .*$?pkg: golang.org/x/exp/slog/benchmarks?' zap.bench
