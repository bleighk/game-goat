# game-goat UAT session close — 2026-09-27

Handoff for continuing enhancements in a future session. Everything below is the durable state.

## Final state at close
- **CLI:** `/home/brad/printing-press/library/game-goat` — binary rebuilt in place (root + `build/stage/bin/`), authenticated, live-verified
- **Git restore point:** `c24fa6e` "post-print UAT hardening (F-U1..F-U16)" — 286 files, first commit of this dir. Note: `/home/brad/.git` exists as an empty zero-commit scaffolding repo (harmless; the library repo is nested and takes precedence)
- **UAT ledger:** `proofs/2026-09-27-uat-findings.md` — **16 findings, 16 FIXED** (F-U1..F-U16; F-U13's entry was reconstructed at close after the fix had landed without a ledger write)
- **Gates at close:** `go build` / `go vet` / `go test ./...` all green (every package); doctor 5/5 OK; RAWG key verified working, never in source (exact-value scan clean)
- **Press-side retro** (saved locally only, NOT filed to GitHub): `proofs/20260927-092504-retro-game-goat-pp-cli.md` — 5 work units incl. P1 skip-marker fingerprint parity

## IMPORTANT — acceptance marker is stale
The keyed live-matrix acceptance marker + 219/245 proof **predate the UAT fix pass** (~30 code changes across `internal/cli`). Before any publish attempt: re-run the live matrix (`dogfood --live --write-acceptance`) so the marker reflects current code. Known matrix failures that are press-side, not CLI bugs: ~20 help "missing Examples" (RAWG spec lacks example ids), ~6 `--json true` probe (press bool-flag bug) — see retro WU-2 and the keyed matrix proof.

## Untested surfaces (next UAT round)
- `gg sync` + `--data-source local` (offline path, cache tables already populated by browse)
- MCP server surface (`game-goat-pp-cli mcp` / `cmd/game-goat-pp-mcp`)
- `--deliver file:/webhook:` sinks; teach/recall learning loop

## Enhancement backlog (nits, logged in ledger, none blocking)
- versus: per-side year pinning (`--year-a/--year-b` or id-as-title) — F-U11 design gap
- suggested/similar studio tier: rating-sample floor (11-rating oddities) — F-U14 nit
- similar/suggested human mode: per-row reason repeats the same tag — F-U12 nit
- finishline: all milestones say "next"; sub-day months_active projection guard — round-3 nits
- studio timeline ascending; radar unrated rows as "-"; backlog add dry-run could show resolved game; games-get 404 hint says "list"; tonight PLAYTIME empty for in-progress lead

## How to continue
1. Edit `/home/brad/printing-press/library/game-goat/internal/cli/...` (hand-authored files are regen-merge safe: `generate --force` preserves them)
2. Verify: `go build -o build/stage/bin/game-goat-pp-cli ./cmd/game-goat-pp-cli && cp build/stage/bin/game-goat-pp-cli ./game-goat-pp-cli && go vet ./... && go test ./...` + live probes
3. Log findings as F-U<n> in the ledger; commit in the library repo
4. Publish path (when ready): `/printing-press-publish game-goat` — but re-run the live matrix first (see stale-marker note)

## Where things live
- Ledger, retro, this handoff: `/home/brad/printing-press/manuscripts/game-goat/20260927-155341-72694577/proofs/`
- CLI source + git: `/home/brad/printing-press/library/game-goat`
- Credentials (never committed): `~/.local/share/game-goat-pp-cli/credentials.toml`
- Local store: `~/.local/share/game-goat-pp-cli/data.db` (your celeste in-progress + hollow-knight backlog rows)
