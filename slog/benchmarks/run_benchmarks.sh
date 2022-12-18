#!/bin/bash -e

GO=go
if [[ $1 != '' ]]; then
  GO=$1
fi

cd $(dirname $0)

set -x

$GO test -tags nopc -bench . -count 5 > slog.bench
$GO test            -bench . -count 5 > slog-pc.bench
