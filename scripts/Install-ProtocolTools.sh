#!/bin/sh
set -eu
if [ "$#" -ne 1 ] || [ -z "$1" ]; then
    echo '用法：Install-ProtocolTools.sh "protoc 文件路径"' >&2
    exit 1
fi
case "$1" in
    /*) protoc_source=$1 ;;
    *) protoc_source="$PWD/$1" ;;
esac
cd "$(dirname "$0")/.."
exec "${GO_EXE:-go}" run github.com/haihhhhh/ht_server_frame/tools/protocol install -protoc-source "$protoc_source"
