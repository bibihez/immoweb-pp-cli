# Immoweb CLI

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

Learn more at [Immoweb](https://www.immoweb.be).

## Install

The recommended path installs both the `immoweb-pp-cli` binary and the `pp-immoweb` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install immoweb
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install immoweb --cli-only
```


### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/immoweb/cmd/immoweb-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/immoweb-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-immoweb --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-immoweb --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-immoweb skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-immoweb. The skill defines how its required CLI can be installed.
```

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Verify Setup

```bash
immoweb-pp-cli doctor
```

This checks your configuration.

### 3. Try Your First Command

```bash
immoweb-pp-cli en
```

## Usage

Run `immoweb-pp-cli --help` for the full command reference and flag list.

## Commands

### en

Manage en

- **`immoweb-pp-cli en search-classifieds`** - List classifieds matching a property type, transaction type, and a
wide set of optional filters (price, surface, bedrooms, location,
EPC score, new-build flag, etc.). Returns 30 results per page;
`totalItems` carries the full match count.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
immoweb-pp-cli en

# JSON for scripting and agents
immoweb-pp-cli en --json

# Filter to specific fields
immoweb-pp-cli en --json --select id,name,status

# Dry run — show the request without sending
immoweb-pp-cli en --dry-run

# Agent mode — JSON + compact + no prompts in one flag
immoweb-pp-cli en --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-immoweb -g
```

Then invoke `/pp-immoweb <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


```bash
go install github.com/mvanhorn/printing-press-library/library/other/immoweb/cmd/immoweb-pp-mcp@latest
```

Then register it:

```bash
claude mcp add immoweb immoweb-pp-mcp
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/immoweb-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/other/immoweb/cmd/immoweb-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "immoweb": {
      "command": "immoweb-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
immoweb-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/immoweb-search-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
