

## Slice C — local store commands (backlog + queue)

Date: 2026-09-27

### Files

- `internal/store/game_backlog.go` (NEW) — custom local-table migration `game_backlog` (rawg_id PK; title, slug, added_at RFC3339 UTC, status backlog|in-progress|finished|dropped, hours, finished_at, user_rating 1-5, notes; indexes on status and added_at), registered from `migrateExtras` via `s.migrateGameBacklog`. Typed accessors: `UpsertGameBacklog` (insert-or-update, added_at preserved, finished_at reconciled on status transitions), `SetGameBacklogStatus` (flip stamps/clears finished_at), `SetGameBacklogHours`, `GetGameBacklog`, `FindGameBacklogByTitle` (exact case-insensitive, most recent first), `RemoveGameBacklog`, `ListGameBacklog` (status filter + whitelisted sort added|title|hours + limit), `SearchGameBacklog` (LIKE on title/slug/notes with wildcard escaping, rank title > slug > notes).
- `internal/store/game_backlog_test.go` (NEW) — upsert insert/no-duplicate/update, status flip sets + clears + re-stamps finished_at, hours update (+negative rejection, missing-id ok=false), remove, table+index creation, list filter/sort, search ranking with tier-deterministic assertions and LIKE-wildcard literal test. Mirrors candidates_test.go style.
- `internal/store/extras.go` — one hand-edit: `migrateExtras` now calls `s.migrateGameBacklog` (extras.go is the sanctioned hand-edit point per its header comment).
- `internal/cli/backlog.go` (REWRITTEN from scaffold) — backlog group + five subcommands (add/list/remove/show/search) via `addNovelCommandIfAbsent` under the group; `backlog audit` scaffold left untouched (later slice). Shared helpers: `backlogReadStore` (read-only open, missing DB/table = empty state), `backlogNotFoundErr` (typed exit 6), `findBacklogRow` (id or exact title), table cell formatting, `backlogRowsForQueue`.
- `internal/cli/queue.go` (NEW) — top-level `queue` registered via `registerNovelCommand` hook. Tiered picks: in-progress (least hours first) → backlog-status by live RAWG rating (lookups capped at 10 distinct titles, exact-title match) → longest-waiting; finished never picked; offline degrade (no key/unreachable) reranks by wait time with reason "offline pick (no RAWG rating)" + stderr notice + meta.ratings=offline.
- `internal/cli/trending.go` — defensive backlog probe now prefers `game_backlog` (kept older alias probes) so trending's on_backlog join works once Slice C's table exists.

### Decisions

- Idempotent `backlog add`: existing rawg_id → stderr notice "already on backlog since <date>" + `meta.already_present:true` + exit 0, no write (guard checked before upsert; post-upsert inserted=false race reported the same way).
- `backlog add` flags: `--id` skips title resolution; `--id`+`--title` (or positional title) = fully offline add (slug derived, meta.source=local, resolved_by=id-offline); `--id` alone fetches the title live by id; `--year` only valid with title resolution; `--status/--hours/--notes` set at insert.
- Exit palette: no command uses exit 6 elsewhere in the CLI; local backlog misses (remove/show not-found, remove on missing store) are exit 6 with hint "see: game-goat-pp-cli backlog list" — distinct from live RAWG exit 3. Declared via `pp:typed-exit-codes` on add/remove/show.
- MCP annotations: phase-11 has NO local-mutating read-write token (verified against the phase-11 vocabulary: mcp:read-only, mcp:hidden, mcp:write-positionals only) — `backlog add`/`remove` mutate local state and simply OMIT `mcp:read-only` per the phase rule "omit the map for commands that mutate"; list/show/search/queue are read-only true; `backlog add` is `pp:data-source live` (resolves live, writes local), queue is auto (local rows + live ratings).
- List `--limit` default 50 clamped at 500 (stderr notice on clamp); queue `--limit` default 3 bounded 1-10; search `--limit` default 50.
- Human tables: title/status/hours/added columns (date-only added_at for width; full RFC3339 in show/JSON). Empty backlog = friendly empty-state lines (humans) and `[]` (JSON), never an error.
- No TTY prompts on remove; --help documents the re-add undo path (`add --id <id> --title <title>`).
- All five subcommands + queue follow the verify-friendly RunE template exactly (help-only branch, dryRunOK before required-input validation, len(args) checks, no MinimumNArgs/MarkFlagRequired).

### Acceptance results (raw)

1. `./game-goat-pp-cli backlog --help` / `backlog add --help` / `queue --help` -> exit 0 with real synopses and Example blocks. **PASS**
2. `--dry-run --json` on add/list/remove/show/search/queue -> `{"dry_run":true,"action":"<cmd>","would":"run <cmd>; no changes made"}` exit 0. **PASS**
3. `backlog add --id 12345 --title "Offline Test Game" --hours 2.5 --notes "from a friend" --json` (fresh HOME, no network, no RAWG key) -> exit 0, row with hours=2.5, notes, added_at RFC3339 UTC, resolved_by=id-offline. **PASS**
4. Re-add same id -> `already_present:true`, same added_at, exit 0, stderr notice, no duplicate row. **PASS**
5. `backlog list --json` -> the row; `backlog show "offline test game" --json` (case-insensitive title) -> full row; `backlog search "Offline" --json` and notes substring "from a friend" -> row; `--status`/`--sort` filters verified. **PASS**
6. `backlog remove "Offline Test Game" --json` -> `{removed:true,rawg_id,title,row}`; `backlog remove "Not A Real Game"` -> exit 6 with "see: game-goat-pp-cli backlog list". **PASS**
7. Usage errors exit 2 with JSON usage envelope: `--status bogus`, `--sort bogus`, `--hours -1`, `--year 20`, `--title` without `--id`, queue `--limit 0`/`--limit 99`/positional arg. **PASS**
8. `queue` offline (no RAWG key): in-progress rows first by hours asc (1h before 3h), then backlog-status with reason "offline pick (no RAWG rating)", stderr notice, meta.ratings=offline; finished rows never picked; empty backlog -> friendly empty state / `[]` exit 0. **PASS**
9. `go test ./internal/store/ -run GameBacklog` -> ok (9 focused tests); `go test ./...` -> exit 0 all packages; `go vet ./...` -> clean; `gofmt -l internal/` -> empty. **PASS**

Build gates: `go build ./...` exit 0; binary rebuilt via `go build -o game-goat-pp-cli ./cmd/game-goat-pp-cli`; probe artifacts (tmp-probe dirs, err.txt) removed.

### Deferred

- Live (non-key) queue rating lookups, `backlog add` by-title live resolution, and trending's on_backlog join against populated game_backlog are left to the Phase 4 dogfood matrix (needs RAWG_API_KEY in the run environment); offline paths exercised fully above.
- `backlog audit` remains a scaffold (pp:novel-scaffold) for its own slice; queue's dropped-tier picks surface last as "waiting longest since <added-at>" so revisits are possible.
- pp:happy-args intentionally absent on `backlog show`/`remove`: their happy path depends on a populated store (verify fixtures run on a fresh HOME), while their not-found exit 6 is already declared via pp:typed-exit-codes; dogfood matrix coverage covers the seeded flows.

## Completion gate — Phase 11 (2026-09-27)
- Absorbed-row leaf walk: 23/23 PASS (games search/get/popular/top-rated/upcoming, genres/platforms/tags/stores/creators list, discover, trending, tonight, ratings, series, versus, studio, backlog add, queue, auth set-token, suggested, similar, games additions).
- Transcendence: backlog audit, finishline, radar, retention, moods list + hand-added moods show — all resolve via --help; unit tests extended per feature (budget-fit, tier-gate 401/403 detection, taste ranking, retention math, milestone projection).
- research.json novel_features: 5/5 commands resolve; slice D report carries per-invocation probes; live paths deferred to the live dogfood matrix (no RAWG_API_KEY in run env by design).
- Build gates: go build ./... PASS, go vet ./... PASS, go test ./... 15 packages ok, gofmt -l internal/ clean.
- Manifest revision applied at Phase 11 entry with user re-approval (lists row cut — RAWG public API has no /lists endpoint; 4 path normalizations).
\n## Post-shipcheck fix — --rating CLI path\n- backlog add gained --rating (0-5) + update-on-re-add semantics (explicit status/hours/rating/notes merge in place; plain re-add stays a no-op). Probed add/notice/update/exit-2/finishline; shipcheck loop 3 PASS 7/7 on the fixed binary.\n
