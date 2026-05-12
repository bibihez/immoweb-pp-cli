---
name: pp-immoweb
description: "Printing Press CLI for Immoweb. Public, anonymous JSON search surface used by the Immoweb.be web app to list real-estate classifieds (houses,..."
author: "bibihez"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - immoweb-pp-cli
    install:
      - kind: go
        bins: [immoweb-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/immoweb/cmd/immoweb-pp-cli
---

# Immoweb — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `immoweb-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install immoweb --cli-only
   ```
2. Verify: `immoweb-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/immoweb/cmd/immoweb-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Public, anonymous JSON search surface used by the Immoweb.be web app to
list real-estate classifieds (houses, apartments, offices, land, garages,
etc.) for sale or rent across Belgium and Luxembourg.

No authentication required. The endpoint backs the `/en/search/...` UI;
sending `Accept: application/json` with a normal browser User-Agent
returns the full result set, paginated 30 per page. Detail pages
(`/classified/{id}`) are HTML with the listing data embedded in
`window.classified = { ... }` and are intentionally out of scope for
this spec — agents that need them should scrape the HTML.

Spec authored from the live endpoint (verified 2026-05-12) and the
`feldeh/immoweb-scraper` reference implementation.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

**en** — Manage en

- `immoweb-pp-cli en` — List classifieds matching a property type, transaction type, and a wide set of optional filters (price, surface,...


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
immoweb-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

No authentication required.

Run `immoweb-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  immoweb-pp-cli en --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
immoweb-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
immoweb-pp-cli feedback --stdin < notes.txt
immoweb-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.immoweb-pp-cli/feedback.jsonl`. They are never POSTed unless `IMMOWEB_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `IMMOWEB_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
immoweb-pp-cli profile save briefing --json
immoweb-pp-cli --profile briefing en
immoweb-pp-cli profile list --json
immoweb-pp-cli profile show briefing
immoweb-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `immoweb-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/immoweb/cmd/immoweb-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add immoweb-pp-mcp -- immoweb-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which immoweb-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   immoweb-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `immoweb-pp-cli <command> --help`.
