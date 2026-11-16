#!/bin/sh
# Renders the whole piece to renders/mygo-pv.mp4. Pass extra flags through,
# e.g. ./run.sh -stills 130,450 to write those frames as PNGs instead.
set -e
cd "$(dirname "$0")"
exec go run ./pv "$@"
