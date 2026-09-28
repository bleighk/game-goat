## Customer model

**Marcus, the 412-game library completionist**

- **Today (without this CLI):** Sunday night, Marcus scrolls his Steam library sorted by playtime, with a Reddit "games backlog" thread and a HowLongToBeat tab open. He finished 31 of the 412 games he owns. He gives up and replays something he's already beaten. He cannot answer: which unplayed purchases resemble the games he actually finishes, how close his abandoned saves are to the credits, or how long his backlog really is.
- **Weekly ritual:** Sunday "shelf night" — pick one game for the coming week, update a spreadsheet with columns for started/finished/abandoned and hours logged.
- **Frustration:** His library is a guilt list with no signal — every weekly pick re-litigates "did I waste my money" instead of answering "what will I actually enjoy," and nothing tells him which playthroughs are one session from done.

**Priya, the two-evenings-a-week decider**

- **Today (without this CLI):** Tuesday and Thursday evenings, ~3 hours each after the kids are asleep. Each pick costs 30 minutes of tab juggling: RAWG page for the rating, Steam store page for recent reviews, HowLongToBeat for length, Reddit for "is the ending worth it." She cannot answer: is this a game people finish or quietly abandon, and can she reach a satisfying stopping point in one sitting.
- **Weekly ritual:** Thursday 18:30 "tonight pick" — something absorbing but low-stress, under ~4 hours to a natural arc, on PC.
- **Frustration:** Assembling multi-site ratings by hand for every candidate, and having no way to express "mood" as a filter — her criteria live in her head, not in any search box.

**Devin, the franchise-marathon planner**

- **Today (without this CLI):** Plays series start-to-finish in release order (just finished Yakuza 0, queued Kiwami). Keeps a Notion page with franchise order beside a RAWG game-series page open in a permanent tab, plus an RSS reader for announcements. He cannot answer: which upcoming releases belong to series he's already invested in, and how many hours remain in a franchise.
- **Weekly ritual:** Saturday-morning planning session — confirm the next series entry, check for new entries in franchises he follows.
- **Frustration:** He learns about new entries in his own franchises weeks late from Reddit; nothing connects "series I'm playing" with "what's shipping soon."

## Candidates (pre-cut)

Gate notes: (e) active — `## User Vision` present. (f) inactive — the brief's `## Codebase Intelligence` section is "N/A" (multi-source fusion; no internal queue/worker/event patterns). (d) omitted — first print.

**1. Decision-debt audit** — `backlog audit`
- Weekly ledger over the local store: unplayed %, median shelf-time (added → finished), finished-vs-added genre/tag mismatch, dropped/abandoned entries, edition-duplicate flags, and `--prune` candidate list.
- Persona: Marcus. Source: (a) persona-driven + (e) user vision.
- Long Description: `Use this command for backlog health stats and taste-mismatch analysis. Do NOT use it to pick your next game; use 'queue' for new picks or 'finishline' for in-progress games.`
- Verdict: **KEEP** — passes all checks (local-data command, not a fake API call; mechanical stats).
- Impl: `// pp:data-source local` (rejects `--data-source live`); `hintIfUnsynced(cmd, db, "")` then `hintIfStale`; drain-first — scan backlog rows into structs, `rows.Err()`, close, then resolve genre/tag names in follow-up queries.

**2. Prune suggestions** — `backlog prune`
- Standalone removal recommender: ranks backlog entries by never-touched age + genre-never-finished signals.
- Persona: Marcus. Source: (a). Long Description: `none`.
- Verdict: **KILL (inline)** — monthly purge cadence, not weekly; the removal ranking is one output mode of `backlog audit --prune`.

**3. Taste profile** — `taste`
- Actual-vs-aspirational genre/tag comparison table for finished vs. bought games.
- Persona: Marcus. Source: (a) + (e). Long Description: `none`.
- Verdict: **KILL (inline)** — duplicate query surface; the taste table is `backlog audit`'s core section.

**4. Finishline ranking** — `finishline`
- Ranks in-progress backlog games by estimated hours-to-credits: backlog `hours` ÷ RAWG per-game `playtime` (avg hours).
- Persona: Marcus. Source: (a) persona-driven.
- Long Description: `Use this command to rank in-progress games by how close you are to the credits. Do NOT use it to choose a brand-new game to start; use 'queue' instead. Do NOT use it for overall backlog stats; use 'backlog audit' instead.`
- Verdict: **KEEP** — cross-source join, no LLM/auth/scope issues.
- Impl: `// pp:data-source auto` (live RAWG playtime with local fallback); drain-first local backlog scan, then live detail fetch; `novelAuthHeader` (never `os.Getenv`), check `resp.StatusCode` before decoding.

**5. Franchise radar** — `radar`
- Upcoming releases joined against franchises already present in backlog/finished lists: local `game_series` ids × `/games?dates=<future range>`.
- Persona: Devin. Source: (c) cross-entity local query + (a).
- Long Description: `Use this command for upcoming releases in franchises you already play. Do NOT use it for a general upcoming-releases calendar; use 'games upcoming' instead.`
- Verdict: **KEEP** — local join the absorbed generic `games upcoming` cannot do.
- Impl: `// pp:data-source auto`; drain-first (scan backlog/series ids → close → live `/games?dates=` fetch).

**6. Crowd retention** — `retention <game>`
- Community completion verdict from RAWG's unique `added_by_status`: beaten/dropped/playing/yet percentages + an "aspirational trap" flag (high added, low beaten, high dropped).
- Persona: Priya. Source: (b) service-specific content pattern.
- Long Description: `Use this command for the community's completion and drop verdict on one game. Do NOT use it for RAWG/Metacritic/Steam rating scores; use 'ratings' instead.`
- Verdict: **KEEP** — mechanical ratio math over one real endpoint; verifiable.
- Impl: `// pp:data-source auto`; `novelAuthHeader` seam, status-code check before JSON decode.

**7. Mood vocabulary** — `moods list` / `moods show <mood>`
- Curated mood → genre/tag clusters (cozy, gripping, mindless, heavy-story…) resolved to verified RAWG tag ids, powering `tonight --mood`.
- Persona: Priya. Source: (b) + (a).
- Long Description: `Use this command to discover the mood names 'tonight --mood' accepts. Do NOT use it to search games by tag; use 'discover' or the raw tag ids from 'tags list' instead.`
- Verdict: **KEEP** — mechanical static-reference command.
- Impl: `// pp:novel-static-reference` + `// pp:data-source computed`; tag-id resolution validates against synced tags (local hint on `tags`).

**8. Edition-duplicate detector** — `dupes`
- Finds backlog entries that are variants of the same base game (base + Royal + Strikers) via parent-games/additions join.
- Persona: Marcus. Source: (c). Long Description: `none`.
- Verdict: **KILL (inline)** — occasional maintenance query; surfaces as a flag inside `backlog audit`'s scan.

**9. Live player count** — `players <game>`
- Steam GetNumberOfCurrentPlayers for a game (keyless).
- Persona: Priya. Source: (b). Long Description: `none`.
- Verdict: **KILL (inline)** — external-service/scope check: course correction narrowed keyless Steam enrichment to the review-score summary only.

**10. Development-team credits** — `credits <game>`
- Per-game dev team with roles from `/games/{id}/development_team`.
- Persona: Devin. Source: (b). Long Description: `none`.
- Verdict: **KILL (inline)** — endpoint mirror with no join; monthly cadence at best; `studio` already serves the people-following ritual.

**11. Hidden gems** — `gems`
- Preset feed: high rating, low added-count (cult classics).
- Persona: Priya/Devin. Source: (b). Long Description: `none`.
- Verdict: **KILL (inline)** — thin preset over absorbed `discover` (`ordering=-rating` + added cap); ships as a discover recipe.

**12. Steam library import** — `import steam`
- Bulk-seed backlog from owned Steam library/wishlist.
- Persona: Marcus. Source: (a). Long Description: `none`.
- Verdict: **KILL (inline)** — auth gap: manifest note restricts this print to keyless scope; steam-web's library surfaces (STEAM_API_KEY) explicitly not absorbed.

**13. Nightly briefing** — `briefing`
- One-call digest: tonight pick + queue top + audit stats.
- Persona: Priya. Source: (a) + (e). Long Description: `none`.
- Verdict: **KILL (inline)** — scope creep: pure aggregation of survivor outputs with no new data; `--select`/recipes compose the same.

**14. Person filmography** — `worked-on <person>`
- Local join across synced `development_team` rows: every game person X worked on.
- Persona: Devin. Source: (c). Long Description: `none`.
- Verdict: **KILL (inline)** — requires syncing per-game development_team for every cached game to answer a monthly question; sync cost dwarfs value.

**15. Backlog-years budget** — `backlog years --hours-per-week N`
- "Your backlog is 2.3 years at 6 hrs/week" projection from summed RAWG playtimes.
- Persona: Marcus. Source: (c). Long Description: `none`.
- Verdict: **KILL (inline)** — run-once novelty; becomes a summary line in `backlog audit`.

**16. Free-text vibe search** — `vibe <query>`
- Match games to a natural-language mood description ("something cozy but tense").
- Persona: Priya. Source: (b). Long Description: `none`.
- Verdict: **KILL (inline)** — LLM dependency (semantic grouping); mechanically reframed as `moods` (candidate 7) + `discover`.

## Survivors and kills

### Survivors

**1. Decision-debt audit (`backlog audit`)**
1. Weekly use: Yes — Marcus's Sunday shelf-night runs it before every pick; it is the ritual's scoreboard.
2. Wrapper vs leverage: Not a wrapper — pure local SQLite joins; no single RAWG call computes "my backlog's taste mismatch."
3. Transcendence proof: Local SQLite cross-entity join (backlog status/hours/dates × synced genre/tag rows).
4. Sibling kill: `taste` — same queries, narrower view; `prune`, `dupes`, and `budget` folded in as flags/stat lines.
5. Buildability: `hand-code` — SQLite joins, drain-first scans, custom stats output (~100–150 LoC + root.go wiring).
6. Long-description validity: References `queue` (absorbed, ships) and `finishline` (survives) — both valid.

**2. Finishline ranking (`finishline`)**
1. Weekly use: Yes — Marcus's Sunday pick starts with "what am I closest to finishing"; Priya uses it mid-series.
2. Wrapper vs leverage: Not a wrapper — joins local `hours` against RAWG `playtime`; no endpoint ranks your in-progress saves.
3. Transcendence proof: Cross-source join (local playtime log × live RAWG avg playtime) + decision-debt ranking.
4. Sibling kill: `backlog prune` — both rank backlog rows by debt signals; prune ranks removals (monthly purge), finishline ranks completions (weekly close-the-loop ritual per the User Vision).
5. Buildability: `hand-code` — live playtime fetch + local join + ratio math.
6. Long-description validity: References `queue` (absorbed, ships) and `backlog audit` (survives) — valid.

**3. Franchise radar (`radar`)**
1. Weekly use: Yes — Devin's Saturday planning session includes "anything new in my franchises?"
2. Wrapper vs leverage: Not a wrapper — the join over game-series of local entries against a future-dates query is the product; `games upcoming` alone is generic.
3. Transcendence proof: Cross-entity local join (game_series × backlog/finished) + franchise content pattern.
4. Sibling kill: `gems` — both are "lens" feeds over `/games`; gems is a static discover preset, radar is personalized by local data.
5. Buildability: `hand-code` — drain-first local series scan + live `/games?dates=` fetch.
6. Long-description validity: References `games upcoming` (absorbed, ships) — valid.

**4. Crowd retention (`retention <game>`)**
1. Weekly use: Yes — Priya runs it on 2–3 candidates per Thursday pick; Marcus runs it before adding (aspirational-trap check).
2. Wrapper vs leverage: Not a wrapper — `/games/{id}` returns raw `added_by_status` counts; retention computes beaten/drop percentages and the trap verdict no generic client computes, and RAWG has no aggregation endpoint for it.
3. Transcendence proof: Service-specific content pattern (RAWG's unique community-status field) + agent-shaped verdict output.
4. Sibling kill: `players` — both are crowd-signal features; players is live concurrents (out of course-corrected enrichment scope), retention is the completion/drop signal that fits the decision-debt thesis.
5. Buildability: `hand-code` — one detail call via `novelAuthHeader`, ratio math, structured output.
6. Long-description validity: References `ratings` (absorbed, ships) — valid.

**5. Mood vocabulary (`moods list` / `moods show <mood>`)**
1. Weekly use: Yes — Priya consults the vocabulary every Thursday when running `tonight --mood`; she cannot name the clusters from memory.
2. Wrapper vs leverage: Not a wrapper — no API call at all; a curated static-reference taxonomy resolving to verified RAWG tag ids.
3. Transcendence proof: Serves the service-specific ritual pattern — `tonight --mood` is unusable without a discoverable vocabulary.
4. Sibling kill: `vibe` — killed by LLM dependency; `moods` is its mechanical reframe (curated clusters instead of semantic matching).
5. Buildability: `hand-code` — static-reference table + tag-id resolution against synced tags.
6. Long-description validity: References `tonight`, `discover`, `tags list` (all absorbed, ship) — valid.

| # | Feature | Command | Score | Persona | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|---------|--------------|--------------|----------|------------------|
| 1 | Decision-debt audit | backlog audit | 10/10 | Marcus, backlog-guilty completionist | hand-code | Joins local backlog status/hours/added-date rows with synced genre/tag data (drain-first) to compute unplayed %, shelf-time, finished-vs-added taste mismatch, edition-duplicate flags, and prune candidates with no external dependencies | User Vision's decision-debt ledger thesis; Depressurizer (1526★) proves whole-tool demand for library triage; steam-web's library-audit commands show the analytics appetite | Use this command for backlog health stats and taste-mismatch analysis. Do NOT use it to pick your next game; use 'queue' for new picks or 'finishline' for in-progress games. |
| 2 | Finishline ranking | finishline | 8/10 | Marcus; Priya mid-series | hand-code | Joins local backlog hours/status against /games/{id} playtime to rank in-progress games by estimated hours-to-credits with no external dependencies | User Vision: "every abandoned playthrough is a signal"; brief Top Workflow 3 (decision-debt analytics); Depressurizer backlog pain | Use this command to rank in-progress games by how close you are to the credits. Do NOT use it to choose a brand-new game to start; use 'queue' instead. Do NOT use it for overall backlog stats; use 'backlog audit' instead. |
| 3 | Franchise radar | radar | 8/10 | Devin, franchise-marathon planner | hand-code | Joins local game-series rows of backlog/finished entries against /games?dates=<future range> to list upcoming releases in franchises you already play with no external dependencies | Brief Top Workflows 1/5 (tonight + series/similar); User Vision taste-closing thesis; RAWG /games/{id}/game-series in the spec | Use this command for upcoming releases in franchises you already play. Do NOT use it for a general upcoming-releases calendar; use 'games upcoming' instead. |
| 4 | Crowd retention | retention \<game\> | 9/10 | Priya, two-evening decider; Marcus pre-add | hand-code | Computes beaten/dropped/playing/yet percentages and an aspirational-trap flag from /games/{id} added_by_status counts with no external dependencies | shouldiplay (52★) proves single-verdict "should I play X" demand; User Vision's actual-vs-aspirational taste signal; RAWG spec's unique added_by_status field | Use this command for the community's completion and drop verdict on one game. Do NOT use it for RAWG/Metacritic/Steam rating scores; use 'ratings' instead. |
| 5 | Mood vocabulary | moods list / moods show \<mood\> | 7/10 | Priya, two-evening decider | hand-code | Uses a curated static mood→genre/tag map (pp:novel-static-reference) resolved against synced RAWG tag ids to expand tonight's --mood vocabulary with no external dependencies | Brief Top Workflow 1 (--mood/--tags filters on tonight); User Vision "tonight filtered by mood/time" | Use this command to discover the mood names 'tonight --mood' accepts. Do NOT use it to search games by tag; use 'discover' or the raw tag ids from 'tags list' instead. |

### Killed candidates

| Feature | Kill reason | Closest-surviving-sibling |
|---------|-------------|---------------------------|
| backlog prune | Monthly purge cadence, not weekly; removal candidates are one output mode of `backlog audit --prune` rather than a standalone command | backlog audit |
| taste | Duplicate query surface — the finished-vs-added genre/tag comparison is `backlog audit`'s core section | backlog audit |
| dupes | Occasional maintenance query; edition duplicates surface as a flag inside `backlog audit`'s scan instead | backlog audit |
| budget | Run-once projection novelty; becomes a summary line (total hours, months at N hrs/week) in `backlog audit` | backlog audit |
| vibe | LLM dependency — free-text mood matching needs semantic grouping; `moods` is the mechanical reframe | moods |
| players | Course correction narrowed keyless Steam enrichment to the review-score summary; live concurrents belong to a post-publish amend | retention |
| credits | Monthly-cadence endpoint mirror of /games/{id}/development_team with no join or local value; `studio` already serves the people-following ritual | studio |
| worked-on | Requires syncing per-game development_team rows for every cached game to answer a monthly filmography question; sync cost dwarfs weekly value | studio |
| gems | Thin preset over absorbed `discover` (ordering=-rating with an added-count cap); ships as a discover recipe, not a command | radar |
| import | Auth gap — this print is keyless-only per the absorb manifest note; Steam library reads need STEAM_API_KEY and are explicitly excluded | backlog (add) |
| briefing | Pure aggregation of audit/tonight/queue outputs with no new data; `--select` and recipes compose the same digest | tonight |