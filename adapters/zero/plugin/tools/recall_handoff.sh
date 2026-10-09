#!/bin/sh
# Deterministic recall. Tool arguments arrive on stdin and are not a note.
set -eu
if ! command -v handoff >/dev/null 2>&1; then
	echo "handoff: command not found on PATH" >&2
	exit 127
fi
exec handoff show </dev/null
