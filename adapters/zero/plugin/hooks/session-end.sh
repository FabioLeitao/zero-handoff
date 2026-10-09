#!/bin/sh
# Self-contained sessionEnd adapter. `zero plugins add` copies this directory
# on its own, so the script must not refer to a path outside the plugin.
# hooks.json and this script both pass --harness zero. The CLI does not invent it.
set -eu

if ! command -v handoff >/dev/null 2>&1; then
	echo "handoff: command not found on PATH" >&2
	exit 127
fi

set -- save --harness zero --skip-without-session
if [ -n "${HANDOFF_VERSION:-}" ]; then
	set -- "$@" --version "$HANDOFF_VERSION"
fi
if [ -n "${HANDOFF_NOTE:-}" ]; then
	set -- "$@" --note "$HANDOFF_NOTE"
fi
exec handoff "$@"
