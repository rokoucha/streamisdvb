#!/usr/bin/env bash
set -euo pipefail

version="${1:?usage: build-release.sh VERSION}"
[[ "$version" =~ ^[0-9A-Za-z][0-9A-Za-z._-]*$ ]] || { echo 'Invalid version' >&2; exit 1; }
mkdir -p dist
for arch in amd64 arm64; do
  directory="streamisdvb_linux_${arch}"
  mkdir -p "dist/$directory"
  CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -trimpath -ldflags='-s -w' -o "dist/$directory/streamisdvb" ./cmd/streamisdvb
  tar -C dist -czf "dist/streamisdvb_${version}_linux_${arch}.tar.gz" "$directory"
  rm -r "dist/$directory"
done
(cd dist && shasum -a 256 "streamisdvb_${version}_linux_"*.tar.gz > checksums.txt)
