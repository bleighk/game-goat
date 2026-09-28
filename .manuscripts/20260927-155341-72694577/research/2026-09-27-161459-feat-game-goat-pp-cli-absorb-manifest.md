# game-goat-pp-cli — Absorb Manifest (Step 1.5d)

## Absorb Manifest

### Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 1 | Search games by title/keyword | movie-goat movies search; mcp-rawg search_games; npm rawg wrappers | game-goat-pp-cli games search | FTS5 offline mirror via sync, --json/--select, typed exit codes |
| 2 | Full game detail (ratings, platforms, stores) | mcp-rawg get_game | game-goat-pp-cli games get | remake-aware ambiguity signal, --dry-run, offline cache |
| 3 | Genre list | mcp-rawg list_genres | game-goat-pp-cli genres list (generated endpoint) | offline sync; ids feed discover filters |
| 4 | Platform list | mcp-rawg list_platforms | game-goat-pp-cli platforms list (generated endpoint) | offline sync |
| 5 | Tag list | RAWG spec /tags | game-goat-pp-cli tags list (generated endpoint) | offline sync |
| 6 | Store list | RAWG spec /stores | game-goat-pp-cli stores list (generated endpoint) | offline sync |
| 7 | Developer browse | movie-goat people; RAWG /developers | (generated endpoint) developers list + developers read | offline sync |
| 8 | Creator list | RAWG /creators | game-goat-pp-cli creators list (generated endpoint) | offline sync |
| 9 | Popular browse | movie-goat movies popular | game-goat-pp-cli games popular (RAWG /games ordering=-added) | offline sync |
| 10 | Top-rated browse | movie-goat movies top-rated | game-goat-pp-cli games top-rated (RAWG /games ordering=-rating) | offline sync |
| 11 | Upcoming releases | movie-goat movies upcoming | game-goat-pp-cli games upcoming (RAWG /games dates range) | offline sync |
| 12 | Discover-by-filter (genre, dates, rating, platform) | movie-goat discover; metacritic finder list-filters | game-goat-pp-cli discover (RAWG /games 20+ filters) | richer facets than metacritic, --agent output |
| 15 | Multi-source ratings card | movie-goat ratings (TMDb+OMDb) | game-goat-pp-cli ratings | RAWG + Metacritic (via RAWG field) + keyless Steam review summary; graceful degradation |
| 16 | Franchise planner | movie-goat marathon (collections); boardgamegeek family; RAWG game-series | game-goat-pp-cli series | franchise order, total playtime, suggested entry points |
| 21 | Remake-aware title resolution | movie-goat ("Sabrina" 1954 vs 1995) | (behavior in game-goat-pp-cli ratings) | ambiguity notice on stderr AND meta.ambiguous in JSON; pin with --year or id; also applied in series, similar, and retention |
| 22 | Auth subcommands | movie-goat auth | game-goat-pp-cli auth set-token | RAWG_API_KEY env wins over config |
| 24 | Similar games | RAWG suggested/similar endpoints | game-goat-pp-cli similar | tiered free-tier join (same studio capped, defining gameplay tag, confidence-floored genre join); absorbs row 23 suggested; per-row tier + reason |
| 25 | Steam review-score enrichment | steam-web keyless appreviews (live-probed) | (behavior in game-goat-pp-cli ratings) | Steam review summary column; keyless, optional, degrades to RAWG-only |
| 26 | DLC/edition resolution | RAWG additions/parent-games | game-goat-pp-cli games additions (generated endpoint) | feeds edition disambiguation |

### Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|--------------------------|------------------|
| 4 | Crowd retention | retention <game> | hand-code | RAWG's unique added_by_status counts become beaten/dropped/playing/yet percentages plus an aspirational-trap verdict; RAWG exposes no aggregation endpoint, and generic clients return only raw counts. | Use this command for the community's completion and drop verdict on one game. Do NOT use it for RAWG/Metacritic/Steam rating scores; use 'ratings' instead. |

### Manifest notes — source cuts (confirmed 2026-09-27; revision re-approved by user at Phase 3 entry)

- Revision (user-approved): CUT row 9 Community lists — RAWG public API has no /lists endpoint (live-probe 404 on /api/lists and /api/lists/games; website-only feature; post-publish amend if ever exposed). Rows 10-27 renumbered 9-26. Rows 7/19(20)/21(22)/22 normalized to gate-parseable command paths; no other feature changed.

- IsThereAnyDeal + gg.deals: studied, zero absorbed — deal/price scope excluded by the course correction; belongs in a post-publish amend.
- CheapShark, HowLongToBeat, OpenCritic: dropped from this run entirely (course correction).
- steam-web: keyless review-score enrichment only (ratings column); auth-required library/achievement/friends surfaces explicitly out of scope for this print.
- movie-goat collaborators: no RAWG-native parallel; evaluated, not carried over.

### Movie-goat command mapping (user confirmation requested)

| movie-goat | game-goat-pp-cli | Change |
|-----------|------------------|--------|
| movies search / movies get | games search / games get | renamed to RAWG domain, remake-aware ambiguity |
| discover | discover | same command, RAWG's 20+ filters |
| trending | trending | same, joined against local backlog |
| tonight | tonight | same, --mood powered by new moods vocabulary |
| ratings (TMDb + OMDb) | ratings | RAWG + Metacritic + keyless Steam review summary |
| marathon | series | RAWG game-series spine; franchise order + total playtime |
| versus | versus | same, playtime/platforms columns |
| career | studio | developer timeline |
| watchlist | backlog | decision-debt fields: added date, status, hours, finished |
| queue | queue | backlog + suggested + similar derivation |
| auth | auth | RAWG_API_KEY env override |
| (new) | suggested / similar | RAWG-native recommenders, backlog-flagged |
| (new) | backlog audit / finishline / radar / retention / moods | 5 transcendence features (hand-code) |

### Scope cut (2026-09-27, post-UAT, user decision)

game-goat is a lookup-and-recommend CLI for agents; the local backlog is dropped. Rows removed from the tables above (numbering preserved, gaps intentional):

- | 13 | Trending now | movie-goat trending; boardgamegeek hot | game-goat-pp-cli trending | join against local backlog |
- | 14 | Tonight picker | movie-goat tonight | game-goat-pp-cli tonight | trending + backlog; --mood/--tags --platform --max-hours --min-rating |
- | 17 | Head-to-head compare | movie-goat versus | game-goat-pp-cli versus | ratings, genres, playtime, platforms side-by-side |
- | 18 | Studio timeline | movie-goat career | game-goat-pp-cli studio | a developer's games over time, chronological with ratings |
- | 19 | Local SQLite list + FTS5 | movie-goat watchlist; boardgamegeek collection | game-goat-pp-cli backlog add | decision-debt fields: added date, status, hours, finished |
- | 20 | Recommendation queue from saved list | movie-goat queue | game-goat-pp-cli queue | next-play picks derived from backlog + suggested + similar |
- | 23 | Suggested-games recommender | RAWG /games/{id}/suggested | game-goat-pp-cli suggested | flags rows already on the backlog |
- | 1 | Decision-debt audit | backlog audit | hand-code | Local SQLite join of backlog status/hours/added-dates × synced genres/tags computes unplayed %, median shelf-time, finished-vs-added taste mismatch, edition duplicates, and prune candidates — no single RAWG call or existing tool (Depressurizer needs Steam auth) can audit a backlog keylessly. | Use this command for backlog health stats and taste-mismatch analysis. Do NOT use it to pick your next game; use 'queue' for new picks or 'finishline' for in-progress games. |
- | 2 | Finishline ranking | finishline | hand-code | Cross-source join of locally logged hours ÷ live RAWG per-game playtime ranks in-progress saves by estimated hours-to-credits; no RAWG endpoint ranks your own playthroughs. | Use this command to rank in-progress games by how close you are to the credits. Do NOT use it to choose a brand-new game to start; use 'queue' instead. Do NOT use it for overall backlog stats; use 'backlog audit' instead. |
- | 3 | Franchise radar | radar | hand-code | Local game_series ids of backlog/finished entries joined against a live /games future-dates query — a personalized upcoming feed impossible from any single RAWG call. | Use this command for upcoming releases in franchises you already play. Do NOT use it for a general upcoming-releases calendar; use 'games upcoming' instead. |
- | 5 | Mood vocabulary | moods list / moods show <mood> | hand-code | Curated static mood→genre/tag taxonomy resolved against synced RAWG tag ids; makes tonight --mood usable with a discoverable vocabulary — no RAWG surface provides one. | Use this command to discover the mood names 'tonight --mood' accepts. Do NOT use it to search games by tag; use 'discover' or the raw tag ids from 'tags list' instead. |

- `suggested` merged into `similar` (RAWG /games/{id}/suggested is business-tier; both were free-tier joins). `trending`, `versus`, `studio` cut as redundant with `games popular`/`discover`, two `ratings` calls, and `discover --developers`. `tonight`, `queue`, `backlog`, `backlog audit`, `finishline`, `radar`, `moods` cut with the backlog.
- research.json novel_features now: `similar`, `retention`.
