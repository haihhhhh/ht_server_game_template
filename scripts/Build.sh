#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
: "${GO_EXE:=go}"
"$GO_EXE" run ./tools/moduleimports
mkdir -p bin
for target in gateway account game world client; do
  "$GO_EXE" build -trimpath -o "bin/$target" "./cmd/$target"
done
"$GO_EXE" vet ./...
