#!/bin/bash

set -euo pipefail

cd "$(dirname "$0")"

pkgs=(
  cmd/go/internal/base
  cmd/go/internal/cfg
  cmd/go/internal/lockedfile
  cmd/go/internal/lockedfile/internal/filelock
  cmd/go/internal/modfile
  cmd/go/internal/modfetch/codehost
  cmd/go/internal/module
  cmd/go/internal/par
  cmd/go/internal/semver
  cmd/go/internal/str
  cmd/internal/objabi
  internal/cfg
  internal/lazyregexp
  internal/testenv
)
goroot=$(go env GOROOT)
to_base_pkg=golang.org/x/exp/cmd/gorelease/internal
to_base_dir=./internal

function to_pkg {
  local pkg=$1
  if [ "$pkg" = internal/cfg ]; then
    echo "$to_base_pkg/stdcfg"
  else
    echo "$to_base_pkg/$(basename "$pkg")"
  fi
}

function to_dir {
  local pkg=$1
  if [ "$pkg" = internal/cfg ]; then
    echo "$to_base_dir/stdcfg"
  else
    echo "$to_base_dir/$(basename "$pkg")"
  fi
}

edits=()
for pkg in "${pkgs[@]}"; do
  edits+=(-e "s,\"$pkg\",\"$(to_pkg "$pkg")\",")
done

for pkg in "${pkgs[@]}"; do
  from_dir=$goroot/src/$pkg
  to_pkg=$(to_pkg "$pkg")
  to_dir=$(to_dir "$pkg")
  rm -rf "$to_dir"
  mkdir -p "$to_dir"

  for f in "$from_dir"/*; do
    if [[ -f $f && $f =~ \.go$ ]]; then
      sed "${edits[@]}" <"$f" >"$to_dir/$(basename "$f")"
    elif [[ -f $f || -d $f && $f =~ /testdata$ ]]; then
      cp -r "$f" "$to_dir/$(basename "$f")"
      chmod u+w "$to_dir/$(basename "$f")"
    fi
  done
  go fmt "$to_pkg" >/dev/null
done

patches=($(ls -1 *.patch | sort))
for p in "${patches[@]}"; do
  patch --silent -Np1 -i "$p"
done
