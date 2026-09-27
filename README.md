# Game Goat CLI

**Look up any game and find what to play next - RAWG search, ratings, franchise order, and tag-matched recommendations with remake-aware title resolution, built for agents.**

## Install

The recommended path installs both the `game-goat-pp-cli` binary and the `pp-game-goat` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install game-goat
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install game-goat --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install game-goat --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install game-goat --agent claude-code
npx -y @mvanhorn/printing-press-library install game-goat --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/game-goat/cmd/game-goat-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/game-goat-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install game-goat --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-game-goat --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-game-goat --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install game-goat --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/game-goat-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `RAWG_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/game-goat/cmd/game-goat-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "game-goat": {
      "command": "game-goat-pp-mcp",
      "env": {
        "RAWG_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

```bash
game-goat-pp-cli auth set-token

game-goat-pp-cli games search "hollow knight" --json

game-goat-pp-cli ratings "hollow knight" --json

game-goat-pp-cli similar "hollow knight" --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.
- **`similar`** — Games like <title>: the seed's own studio first, then its defining gameplay tag (roguelite, metroidvania) found by tag-neighborhood co-occurrence, then a confidence-floored genre join. Every row carries its tier and a reason.
- **`retention`** — Community completion and drop verdict for one game from RAWG added_by_status counts (needs RAWG_API_KEY).

## Recipes

### Games like one you loved

```bash
game-goat-pp-cli similar "Megabonk" --json --select results.name,results.tier,results.reason
```

### Play a franchise in order

```bash
game-goat-pp-cli series "zelda" --json
```

### Pin a remake when titles collide

```bash
game-goat-pp-cli ratings "resident evil 4" --year 2023 --json
```

### Will people actually finish it?

```bash
game-goat-pp-cli retention "elden ring" --json
```

## Usage

Run `game-goat-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `GAME_GOAT_CONFIG_DIR`, `GAME_GOAT_DATA_DIR`, `GAME_GOAT_STATE_DIR`, or `GAME_GOAT_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `GAME_GOAT_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export GAME_GOAT_HOME=/srv/game-goat
game-goat-pp-cli doctor
```

Under `GAME_GOAT_HOME=/srv/game-goat`, the four dirs resolve to `/srv/game-goat/config`, `/srv/game-goat/data`, `/srv/game-goat/state`, and `/srv/game-goat/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "game-goat": {
      "command": "game-goat-pp-mcp",
      "env": {
        "GAME_GOAT_HOME": "/srv/game-goat"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `GAME_GOAT_DATA_DIR` overrides an explicit `--home` for that kind. Use `GAME_GOAT_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `GAME_GOAT_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `game-goat-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### creator-roles

Manage creator roles

- **`game-goat-pp-cli creator-roles`** - Get a list of creator positions (jobs).

### creators

Manage creators

- **`game-goat-pp-cli creators list`** - Get a list of game creators.
- **`game-goat-pp-cli creators read`** - Get details of the creator.

### developers

Manage developers

- **`game-goat-pp-cli developers list`** - Get a list of game developers.
- **`game-goat-pp-cli developers read`** - Get details of the developer.

### games

Manage games

- **`game-goat-pp-cli games list`** - Get a list of games.
- **`game-goat-pp-cli games read`** - Get details of the game.

### genres

Manage genres

- **`game-goat-pp-cli genres list`** - Get a list of video game genres.
- **`game-goat-pp-cli genres read`** - Get details of the genre.

### platforms

Manage platforms

- **`game-goat-pp-cli platforms list`** - Get a list of video game platforms.
- **`game-goat-pp-cli platforms lists-parents-list`** - For instance, for PS2 and PS4 the “parent platform” is PlayStation.
- **`game-goat-pp-cli platforms read`** - Get details of the platform.

### publishers

Manage publishers

- **`game-goat-pp-cli publishers list`** - Get a list of video game publishers.
- **`game-goat-pp-cli publishers read`** - Get details of the publisher.

### stores

Manage stores

- **`game-goat-pp-cli stores list`** - Get a list of video game storefronts.
- **`game-goat-pp-cli stores read`** - Get details of the store.

### tags

Manage tags

- **`game-goat-pp-cli tags list`** - Get a list of tags.
- **`game-goat-pp-cli tags read`** - Get details of the tag.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`game-goat-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`game-goat-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`game-goat-pp-cli learnings list`** - Inspect taught rows
- **`game-goat-pp-cli learnings forget <query>`** - Undo a teach
- **`game-goat-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`game-goat-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`game-goat-pp-cli teach-pattern`** - Install a query/resource template up front
- **`game-goat-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `GAME_GOAT_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `game-goat-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
game-goat-pp-cli creator-roles

# JSON for scripting and agents
game-goat-pp-cli creator-roles --json
# Filter to specific fields
game-goat-pp-cli creator-roles --json --select id,name,slug

# Dry run — show the request without sending
game-goat-pp-cli creator-roles --dry-run

# Agent mode — JSON + compact + no prompts in one flag
game-goat-pp-cli creator-roles --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `6` local store miss, `7` rate limited, `10` config error.

## Health Check

```bash
game-goat-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `game-goat-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/game-goat-pp-cli/config.toml`; `--home`, `GAME_GOAT_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `RAWG_API_KEY` | per_call | Yes for live RAWG commands | Set to your API credential. Local search and analytics over synced data work without it. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `game-goat-pp-cli doctor` reports `agentcookie: detected (managing credentials)` and `auth status` labels the credential source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `game-goat-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $RAWG_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**movie-goat-pp-cli**](https://github.com/mvanhorn/printing-press-library)
- [**mcp-rawg (pipeworx-io)**](https://github.com/pipeworx-io/mcp-rawg)
- [**Depressurizer**](https://github.com/Depressurizer/Depressurizer)
- [**shouldiplay**](https://github.com/rileydnorris/shouldiplay)
- [**BaranDev/videogames-mcp-server**](https://github.com/BaranDev/videogames-mcp-server)
- [**rawg-api-js / rawg-python SDKs**](https://www.npmjs.com/package/rawg-api-js)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
