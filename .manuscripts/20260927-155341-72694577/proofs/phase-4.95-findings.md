# Phase 4.95 — Local Code Review findings (game-goat-pp-cli)

## Autofix summary
13 findings autofixed in-place across 3 review rounds (8 in round 1, 4 in round 2, plus 4 round-3 residuals fixed in place after the final panel per the press 1–3-file rule — build/vet/test re-verified green after every round). No surface-to-user findings.

## Review path chosen
Direct subagent dispatch — correctness, security, maintainability, data-migrations, reliability reviewers in parallel, re-run as full panels per round (rounds 2 and 3 re-ran all five).

## Convergence outcome
Findings cleared. Round 3 returned PASS from 4 of 5 reviewers; the fifth (correctness) plus two maintainability residuals (all low/medium, one-line-mechanical: raw `serr` exit-code classification in suggested.go, retention double-fetch, resolver routing for radar/queue taste loops, hint-construction dedup) were fixed in place after the final panel without a round-4 re-review, respecting the 3-round cap. Build, vet, and the full test suite pass.

## Notable round-1 fixes (high-signal)
- **HIGH steam wire-shape bug:** `query_summary` decoded a nonexistent `score` field, so every Steam review score silently read 0 (ratings cards, versus, bridge). Now decodes `review_score` (0–10) × 10 to the documented 0–100 scale; test fixtures corrected to the real wire shape.
- finishline months-to-zero projection counted dropped rows as remaining work.
- radar taste-profile attributed a fuzzy first-hit game's genres; now exact-title-gated.
- tonight could list the resume-lead game twice.
- parseGameID validated id-parsing consolidated at 6 call sites; UpsertGameBacklog gained negative-hours/rating-range guards; silently-dropped backlog errors in radar/suggested/similar now warn on stderr; MCP command-mirror + .printing-press.json descriptions realigned to research.json; title-resolution deduplicated into shared resolveExactTitleMatches + nearbyMatchHint.

## Template-shape retro candidates
1. `internal/cli/data_source.go:988-1045 vs 1165-1220` — medium — two near-identical ~45-line local-list param canonicalization loops in one file; a pagination-semantics change must be applied twice. Fix in the generator template: extract one shared parse helper for both call sites.
2. `internal/mcp/tools.go:1412-1418` — low — `handleContextResult` serves six empty `{"topic":"","insight":""}` playbook rows as filler to every MCP context call. Fix in the generator: populate from the playbook store or omit the key when empty.
3. `internal/mcp/tools.go:897-915 vs 1155-1175` — low — `mcpSearchEnvelope`/`mcpSQLEnvelope` duplicate store-status warning/next_step/empty-store branching including identical literals. Fix in the generator: share one envelope builder.
4. `cmd/game-goat-pp-mcp/main.go:86` — medium — `http.Server` for `--transport http` sets no ReadHeaderTimeout/IdleTimeout (slowloris exposure, gosec G114). Fix in the generator template: add `ReadHeaderTimeout: 10s, IdleTimeout: 120s`.
5. `internal/client/client.go:1304` — low — binary-stream reads use unbounded `io.ReadAll` after the whole-call Timeout is deliberately dropped. Fix in the generator: wrap in `io.LimitReader` with a documented cap.
6. `internal/client/client.go:1304-1307` — medium — mid-body transport failures (reset/truncated body after headers) return immediately, bypassing the retry loop for replayable idempotent GETs. Fix in the generator: mirror the Do()-error retry branch in the ReadAll/decode error paths.
7. (pre-existing, carried from build/shipcheck) `internal/cli/helpers.go` — 3 dead generator-owned helpers; `moods list` dogfood depth-heuristic mismatch — already known WARNs.

## Surface-to-user findings
None.
