# zero-handoff

Opt-in handoff between harnesses. The v0 core is a local Go CLI. It does not listen on a socket and it does not dial the network.

The name stays `zero-handoff`.

**Z.E.R.O. = Zero-lock-in Entries, Recall & Origin.** No harness lock-in. Entries are append-only. Origin is required. This backronym can change later in the docs.

Other readings: **Zero-Effort Resume & Origin** · **Zone-agnostic Entries, Recall, Origin**.

What the name means: no tie to any harness; zero in the handoff, including zero LLM calls on the save/show path; the first compatible harness is Gitlawb's zero by coincidence.

Scope: [zero-handoff#1](https://github.com/FabioLeitao/zero-handoff/issues/1).

## Install

```bash
go build -o handoff ./cmd/handoff
```

Put that binary on `PATH` as `handoff`. The CLI reads and writes files only.

Store directory: `${HANDOFF_DIR:-${XDG_DATA_HOME:-~/.local/share}/handoff}`. One JSON file per entry. `save` creates a new file and does not rewrite an older one. `HANDOFF_DIR` is the test override.

## Commands

`handoff save` reads a note from `--note` or from stdin. Stdin that is a JSON object supplies `sessionId`, `cwd`, and, when present, `note`, `stopReason`, `turns`, and `error`. A zero `sessionEnd` payload without `note` becomes `session ended` plus those fields.

`handoff show` prints the newest entry. `--harness` and `--cwd` filter on the stored origin. With no match it prints `no handoff entry` and exits 1.

Every entry requires `harness`, `version`, `sessionId`, `cwd`, a UTC `at`, and `resume`. The CLI does not invent `harness`. For `--harness zero` it fills an empty version from `HANDOFF_VERSION` or from `zero --version`, and an empty resume with `zero exec --resume <sessionId>`. The resume string must contain the session id. `show` prints `origin.*` as stored. A later harness does not relabel the entry as its own.

## Redaction

`save` redacts the note before the file is written. Origin fields stay intact so the resume command still names the session.

Redacted shapes: PEM `PRIVATE KEY` blocks, `token=` / `api_key=` / `secret=` / `password=` / `authorization=` assignments, `sk-` (including `sk-ant-` and `sk-or-`), `xai-`, Slack `xox[abprs]-`, JWTs (`eyJ….eyJ….`), `ghp_`, `github_pat_`, `AKIA…`, and `Bearer` tokens.

Left unchanged: ordinary prose, the bare word `token`, filesystem paths, and commit hashes.

## Zero adapter

Copy [`adapters/zero/hooks.project.json`](adapters/zero/hooks.project.json) to `.zero/hooks.json` in a trusted project. zero loads project hooks only when that exact directory is trusted. The hook `id` is required; a file without `id` is dropped and `sessionEnd` does not run.

```json
{
  "id": "handoff-session-end",
  "event": "sessionEnd",
  "command": "handoff",
  "args": ["save", "--harness", "zero"],
  "enabled": true
}
```

`--harness zero` is an argument. The CLI does not infer the harness from `event`.

The plugin at [`adapters/zero/plugin`](adapters/zero/plugin) is self-contained (`schemaVersion` 1). Its `sessionEnd` hook runs `hooks/session-end.sh`, which also passes `--harness zero`. Its tool `recall_handoff` runs `handoff show` and ignores tool arguments. Copy the directory to `<project>/.zero/plugins/zero-handoff` when you want it in one project. `zero plugins add` copies into the user plugin directory; do that only when you mean to install it for every workspace.

`/remind` in zero is the skill [`adapters/zero/plugin/skills/remind/SKILL.md`](adapters/zero/plugin/skills/remind/SKILL.md) and the command file [`adapters/zero/commands/remind.md`](adapters/zero/commands/remind.md). Both tell the agent to call `recall_handoff` and print its stdout unchanged. Typing `/remind` in the zero TUI still starts an agent turn. The deterministic read is the tool command, and on Hermes it is the exec quick command below.

## Sandbox write

Measured on 2026-10-09 with zero `0.9.0+up.3.99721c76.databoar.db9559bb.hdrtimeout.scanignore.acpusage.rmodel.sbxwarn`, a trusted `/tmp` project, and no `sandbox.additionalWriteRoots`. The `sessionEnd` hook runs inside the sandbox. The home directory is read-only there.

Direct mkdir of `~/.local/share/handoff` from the hook:

```text
mkdir: cannot create directory ‘$HOME/.local/share/handoff’: Read-only file system
```

`handoff save` to that default directory from the same hook exited 2:

```text
handoff: mkdir $HOME/.local/share/handoff: read-only file system
```

The same binary with `HANDOFF_DIR` inside the trusted project wrote the file (`INSIDE_SAVE_OK`). The hook itself exited 0 in 31 ms.

zero ignores `sandbox.additionalWriteRoots` in project config. A checked-out repo cannot widen its own sandbox. Grant the store from user config, or pass it for one run:

- User `~/.config/zero/config.json`: `"sandbox": { "additionalWriteRoots": ["<home>/.local/share/handoff"] }`. This v0 did not edit that file.
- One run: `zero exec --add-dir "$HOME/.local/share/handoff"`. The directory has to exist already; the sandbox accepts an existing directory and rejects the filesystem root.

With the directory created and `--add-dir` pointing at it, the real hook `handoff save --harness zero` exited 0 in 40 ms. `handoff show` then printed `origin.harness: zero`, the `sessionId` from that run, and `origin.resume: zero exec --resume <sessionId>`. `zero --version` from inside that hook succeeded, so the stored version is the zero that ran the hook.

## Hermes adapter

[`adapters/hermes/quick_commands.yaml`](adapters/hermes/quick_commands.yaml) is an example for a profile you choose. Do not merge it into the live `~/.hermes/config.yaml`.

```yaml
quick_commands:
  remind:
    type: exec
    command: handoff show
```

Use the interactive TUI. `hermes chat --oneshot` and `-q` initialize the model before slash commands run, so `/remind` never reaches `quick_commands` on that path. In the TUI, `/remind` runs the exec snippet and does not start the model.

Checked on 2026-10-09 with a disposable profile (created with `--no-alias --no-skills`, local `tinyllama:latest`, then deleted). After the entry above existed, interactive `/remind` printed `origin.harness: zero`, the same `sessionId`, and `zero exec --resume <sessionId>`. The transcript did not contain `Initializing agent` or the 64k context refusal. An empty store prints `no handoff entry` (exit 1); that was checked with `handoff show` before the hook wrote the entry.

## Known limits

- `zero exec` in text mode has no session. In the zero fork at `db9559bb`, `shouldUseExecSession` (`internal/cli/exec_sessions.go`) is true only for `stream-json`, `--resume`, `--fork`, or `--init-session-id`. A text `zero exec` still runs `sessionEnd`, with `"sessionId":""`. `handoff save` without `--skip-without-session` then exits 2 (`missing origin fields: sessionId, resume`). With the flag, the CLI prints `handoff: no sessionId; nothing saved` on stderr, exits 0, and writes nothing. It does not invent a session id or a resume command. The zero adapters pass the flag: `adapters/zero/session-end.sh`, `adapters/zero/plugin/hooks/session-end.sh`, and `adapters/zero/hooks.project.json`.
- The resume command stored for a zero session is `zero exec --resume <sessionId>`. The root command `zero` has no `--resume` flag; that flag is on `zero exec`. A TUI session is a regular session (`SessionKind` empty), which `IsResumableKind` accepts, so `zero exec --resume <id>` loads it. Inside the TUI the same session continues with `/resume <id>`.
- `sessionEnd` does not carry a free-form user note. Pass `--note`, set `HANDOFF_NOTE` for the shell adapter, or put `note` in the JSON. Otherwise the note is the synthesized session-ended line.
- A corrupt `*.json` file is skipped with a warning on stderr. One bad file does not hide a valid entry.
- Origin fields are not redacted. A secret placed in `sessionId` or `cwd` would be stored.
- Nothing here installs a user hook, a user plugin, a write root, or a Hermes profile. Those stay opt-in.

License: MIT
