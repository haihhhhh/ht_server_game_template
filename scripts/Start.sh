#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
case "${1:-}" in gateway|account|game|world) ;; *) echo 'Usage: Start.sh gateway|account|game|world [config.json]' >&2; exit 1;; esac
if [ "$#" -ge 2 ]; then exec "bin/$1" -config "$2"; else exec "bin/$1"; fi
