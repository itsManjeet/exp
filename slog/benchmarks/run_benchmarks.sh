#!/bin/bash -ex

go test -tags nopc -bench . -count 5 > slog.bench
go test            -bench . -count 5 > slog-pc.bench
