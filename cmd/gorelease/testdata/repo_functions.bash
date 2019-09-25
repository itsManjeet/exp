# Copyright 2019 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

function repo_create {
  local name
  if [ $# -eq 0 ]; then
    name=$(basename $(pwd))
  elif [ $# -eq 1 ]; then
    name=$1
  else
    echo "usage: repo_create name" >&2
    return 1
  fi
  rm -rf "$name"
  mkdir "$name"
  cd "$name"
  git init
  go mod init "example.com/$name"
  git add go.mod
  git commit -m 'initial commit'
}

function repo_pack {
  local name
  if [ $# -eq 0 ]; then
    name=$(basename $(pwd))
  elif [ $# -eq 1 ]; then
    name=$1
  else
    echo "usage: repo_pack name" >&2
    return 1
  fi
  cd ..
  rm -f "$name.zip"
  zip --quiet -r "$name.zip" "$name"
  rm -rf "$name"
}
