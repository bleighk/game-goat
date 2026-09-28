# Post-run live matrix verification (keyed) — 2026-09-27

After run close, the user stored a real RAWG credential and the live matrix was re-run
against the promoted library copy:

- verdict: FAIL 26/245 (pass rate 89.4%) — matrix: 245, passed: 219, failed: 26
- zero auth failures (no 401/403/429) — every network call that could run, ran authenticated
- all 26 failures are invocation-shape artifacts, NOT API or CLI bugs:
  - 20 × kind=help, reason "missing Examples section" — spec-mirror leaf commands
    (creators/developers/games/genres/platforms/publishers/stores/tags read + games
    achievements/additions/development-team/game-series/movies/parent-games/reddit/
    screenshots/stores/suggested/twitch/youtube) whose RAWG spec entries carry no
    example ids, so the generator emitted no Example fields
  - 3 × kind=happy_path + 3 × kind=json_fidelity on backlog audit / finishline /
    moods list — the matrix synthesizes "--json true" (bool flag probed with a
    space-separated value), which these NoArgs commands correctly reject with exit 2
- conclusion: the CLI is live-verified against real RAWG; the remaining failures are
  printing-press retro items (generator: synthesize example ids for id-required
  mirrors; matrix: fix bool-flag probe). Publish gates that read a pass marker stay
  blocked until those land.

Fresh marker (fail, keyed) synced to this archive. Marker: proofs/phase5-acceptance.json
