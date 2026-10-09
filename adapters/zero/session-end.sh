#!/bin/sh
# Forwards zero's sessionEnd JSON on stdin to `handoff save`.
# hooks.json must pass --harness zero; this script does too, so the CLI
# does not invent the origin harness.
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
