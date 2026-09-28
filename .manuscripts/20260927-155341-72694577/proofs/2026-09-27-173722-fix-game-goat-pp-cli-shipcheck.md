# game-goat-pp-cli — Shipcheck (Phase 12)

Run: 20260927-155341-72694577 — structural pass (--no-live-check per user decision; live matrix deferred to Phase 18 once RAWG_API_KEY is exported).

## Umbrella result (final run, shipping binary)

| Leg | Result | Exit |
|---|---|---|
| verify | PASS | 0 |
| validate-narrative | PASS | 0 |
| dogfood | PASS | 0 |
| workflow-verify | PASS | 0 |
| apify-audit | PASS | 0 |
| verify-skill | PASS | 0 |
| scorecard | PASS | 0 |

**Verdict: PASS (7/7 legs), umbrella exit 0.**

## Scores

- Scorecard: 91/100 — Grade A. Domain: Path 10/10, Auth 8/10, Data Pipeline 10/10, Sync 10/10, Type Fidelity 5/5, Dead Code 2/5 (Live API N/A under --no-live-check; mcp_surface_strategy omitted). Novel: Agent Native 10/10, MCP Quality 10/10, Vision 10/10, Workflows 10/10, Insight 10/10, Agent Workflow 9/10.
- verify: 100% pass rate (80/80, 0 critical) — before AND after fixes.
- Novel features built: 5/5 (backlog audit, finishline, radar, retention, moods list) — dogfood-synced.

## Blockers found + fixes

1. **Loop 1 FAIL (validate-narrative)**: the umbrella validated against the staged binary (build/stage/bin/), which was stale from generate time — pre-Phase-11 command tree (no backlog add/tonight/queue). Fixed by rebuilding both staged binaries (game-goat-pp-cli + game-goat-pp-mcp) from current source. Loop 2: 7/7 PASS.
2. **Missing CLI path for user_rating** (found during behavioral sampling): the game_backlog store column supported upserts of user_rating, but backlog add had no --rating flag — the audit feature's finished-vs-added taste mismatch had no input path. Fixed in internal/cli/backlog.go: added --rating (0-5, exit 2 outside range), UserRating wired into GameBacklogInput, and re-add semantics upgraded — plain re-add stays a no-op notice; re-add with explicit --status/--hours/--rating/--notes updates those fields in place (added_at preserved, finished_at follows the status flip). Probed: add finished+4.5 stamps finished_at; plain re-add preserves; --rating=3 re-add updates; --rating=9 exits 2; finishline counts the finished row. Loop 3 (final): 7/7 PASS.

## Behavioral-correctness samples (novel features, local paths)

- backlog audit --json (3 seeded rows): correct stats (unplayed 2/2 active, median shelf days, finished_ratio), [] for empty sections.
- finishline --json: stats + 10 milestones, finished=1 after finished add, projection guards.
- moods list --json: 10 moods with RAWG recipes; moods show cozy: detail + recipe.
- queue --limit=1 (no key): offline degrade with reason "offline pick (no RAWG rating)" + meta.ratings=offline.
- backlog add --rating flow: full add/notice/update/validation matrix above.

RAWG-live behaviors (tonight live picks, suggested tier-gate + fallback join, similar scoring, radar taste profile, retention crowd math, ratings Steam enrichment) are covered by unit tests (budget-fit, tier-gate 401/403 detection, taste ranking, added_by_status math) and dry-run envelopes; live sampling deferred to the Phase 18 matrix per user decision. Steam module coverage 92.4%.

## Before/after

- verify pass rate: 100% (80/80) → 100% (80/80)
- scorecard: 91/100 Grade A → 91/100 Grade A (fixes were correctness, not score-visible)

## Final ship recommendation

**ship** — all ship-threshold conditions met (umbrella 7/7 exit 0; verify PASS 0 critical; dogfood wiring green; verify-skill honest; scorecard 91 ≥ 65; no known functional bugs). Live sampling is the user-deferred Phase 18 matrix gate, which runs before promotion.
