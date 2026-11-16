#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
exec "${GO_EXE:-go}" run ./tools/newproject "$@"
