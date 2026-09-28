# game-goat-pp-cli — UAT Findings Ledger

Live user acceptance testing, 2026-09-27. User-driven; one fix pass at the end unless a finding blocks further testing.

**FIX PASS COMPLETE — 2026-09-27 (post-UAT).** All 8 findings fixed in one pass, 24 edits across 15 files (internal/cli). Build clean, `go vet` clean, full `go test ./...` green. Every fix verified live against RAWG (evidence per finding below; verification also exercised in herdr pane w8:pC). Per-finding status lines updated in place.

## F-U1 — trending and popular return identical results (confirmed)
- **Found:** user observed `trending` and `games popular` present the same games.
- **Verified:** both send the identical RAWG query `ordering=-added` (same page size). Only differences: trending adds the backlog `on_backlog` column and is cache-eligible; popular is live-only. RAWG `-added` is an all-time counter, so both lists are static (GTA V, Witcher 3, Portal 2...).
- **Verdict:** redundancy, not intended. trending's Short promises "right now" momentum the query cannot deliver. Design intent in the absorb manifest (movie-goat trending heritage) was momentum, not all-time totals.
- **Evidence:** source read of internal/cli/trending.go + games_popular.go (identical fetchGamesResults params); live side-by-side run returned identical top-5.
- **Proposed fix:** trending adds a release-date window (default last 90 days, `--days N` override) to the -added query, keeps backlog join. popular stays all-time. One standalone hand-authored file (regen-safe). Proven feasible live: `discover --dates <90d window> --ordering -added` returned a genuinely different list (Valheim, Marvel's Wolverine, Dawnwalker...).
- **Status:** FIXED. trending now adds a `dates` window (default 90 days, `--days N` override) to the -added query; Short/Long updated to the momentum framing. Verified: trending top-5 = Valheim, Wolverine, Dawnwalker (recent momentum) vs popular top-5 = GTA V, Witcher 3, Portal 2 (all-time). Fully differentiated.

## F-U2 — remake ambiguity signal never fires on live RAWG data (confirmed, feature-dead)
- **Found:** user ran `ratings "resident evil 4"`; no stderr warning, no `meta.ambiguous`, despite RAWG returning both "Resident Evil 4" (2023) and "Resident Evil 4 (2005)".
- **Reproduced across three pairs:** resident evil 4, god of war (the normalizer's own doc-comment example), doom — zero warnings in all three.
- **Root cause:** `normalizeGameTitle` (games_search.go) only lowercases + collapses whitespace + trims edge punctuation ` 	:;,.-–—`. It does NOT strip parenthetical suffixes. RAWG disambiguates same-named releases by suffixing one of them: "DOOM" vs "DOOM (2016)", "Resident Evil 4" vs "Resident Evil 4 (2005)". Normalized keys therefore never match across a remake pair → `resolveExactTitleMatches` returns exactly 1 `exact` entry → `writeAmbiguousNotice` never called, `meta.ambiguous` never set. The signal can only fire if RAWG returns two games with literally identical names, which its naming convention avoids.
- **Blast radius:** every title-resolving command shares `resolveTitleForMultiSource`/`resolveExactTitleMatches` (games_search.go:175): ratings, versus, series, backlog add, similar, suggested. The headline "remake-aware ambiguity signal" is dead everywhere.
- **Evidence:** source read (games_search.go normalizeGameTitle + resolveExactTitleMatches + writeAmbiguousNotice); live probes of three remake pairs with stderr captured empty; search results showing RAWG's "(YEAR)" suffix convention.
- **Non-finding checked:** `metacritic: null` on RE4 2023 ratings card is faithful — RAWG's own detail record (games get 795632) returns metacritic null. Correct behavior, not logged as a bug.
- **Proposed fix:** strip trailing parenthetical suffixes in `normalizeGameTitle` before comparison (e.g. remove `\s*\([^)]*\)\s*$`; it is a comparison-only key — ranking/grouping in rankExactTitleFirst/ambiguousCandidates/resolveExactTitleMatches — display names unaffected). One edit in games_search.go lifts all six commands; --year pin and filterByReleaseYear already work correctly downstream of grouping. Watch over-grouping on edition labels ("(25th anniversary)", "(Game of the Year Edition)") — acceptable: an informative notice, still pinnable; scope the strip to parentheticals containing a 4-digit year or edition keywords if it proves noisy.
- **Status:** FIXED. normalizeGameTitle strips a trailing "(YYYY)" parenthetical before comparison (display names untouched; only exact 4-digit-year parentheticals, so "(Raul Fernandes)" stays distinct). Verified: ratings "resident evil 4" and "doom" both emit the ambiguity notice + meta.ambiguous; bonus natural case "backlog add Hades" fired on two Hades releases unprompted. Lifts all six title-resolving commands.

## F-U3 — series mis-anchors franchise shorthand to an obscure same-named game (confirmed)
- **Found:** user ran `series "zelda"` — got "no game-series or parent-games data for 'zelda'" and an empty result, exit 0.
- **Root cause:** series anchors to a single resolved game; "zelda" exactly matches an obscure game literally titled "Zelda" on RAWG (not the Nintendo franchise), which has no series links. The anchor (meta.anchor) was invisible in human mode, the stderr note was title-centric (read like "RAWG has no Zelda data" — false), and the recovery hint pointed at games search instead of suggesting a better anchor. Franchise shorthand — the most natural input for a play-order command — deterministically landed on a junk anchor. Compounded by F-U2 (no ambiguity net for silent mis-resolution).
- **Evidence:** live repro (series "zelda" empty, exit 0) vs series "the legend of zelda: breath of the wild" returning the real 10-game series; source read of series.go resolution flow.
- **Proposed fix:** on an empty first fetch, re-anchor once to the best-rated game in the title's search results with an explicit stderr note; empty human output names the anchor and suggests a franchise-entry title.
- **Status:** FIXED. Verified: `series "zelda"` now prints 'note: no series data on "Zelda" (2023); re-anchored to best-rated match "The Legend of Zelda: Skyward Sword" (2011)' and returns the full 10-game Zelda series in play order with next_unplayed set.

## F-U4 — suggested/similar fallback-join returns the same all-time popular list regardless of seed (confirmed)
- **Found:** user reported `suggested "megabonk"` and `suggested "the witcher 3"` gave the same list.
- **Reproduced:** exact-title runs of both seeds return 9 of 10 identical games (GTA V, Portal, Tomb Raider, L4D2, RDR2, Skyrim, HL2, BioShock Infinite, Borderlands 2). Megabonk = Action/Casual/Indie; Witcher 3 = Action/RPG. The only differences are the seed itself and ordering.
- **Root cause:** RAWG's suggested endpoint requires business tier (401/403); the CLI's documented fallback is a shared-genre join ordered by `-added` (all-time add counter). Every all-time-popular game carries the broad "Action" genre, so the top-N-by-added-with-genre-overlap filter collapses to the same popularity canon for ANY seed whose genres include Action. Seed specificity beyond the genre filter is zero. meta note discloses the fallback but not that ordering is seed-independent.
- **Verdict:** recommendation quality bug, not an auth/correctness bug — exit 0, honest meta.data_origin="fallback-join", correct note. But "recommendations" that don't vary with the seed fail the feature's purpose.
- **Blast radius:** suggested + similar (shared resolver/fallback).
- **Proposed fix:** in the fallback join, order by `-rating` instead of `-added`, rank results by count of shared genres (and tags if available) so overlap-weighting is seed-sensitive, and prefer exact genre-set matches over single-genre overlap. Alternatively weigh: score = shared_genres/rating — anything but all-time-added. Also consider making the fallback note seed-aware ("ordered by community rating within shared genres").
- **Status:** FIXED. Fallback join now orders by -rating and locally ranks by shared-genre count (scoreSimilarity — the same helper similar.go already used); reasons carry the overlap ("shares 2 genres with Megabonk (Indie, Action)"). Verified: suggested megabonk returns indie/casual/action overlap games (Sonic Triple Trouble 16-Bit, Bike Baron, Superfighters Deluxe) — completely different from Witcher 3's list (Mass Effect Trilogy, Bloodborne, Persona 5 Royal). Seed-sensitive now.

## F-U5 — partial-title inputs hard-fail instead of resolving (confirmed)
- **Found:** `suggested "the witcher 3"` exits 3: "no game titled 'the witcher 3' in the top RAWG search results; nearby matches: The Witcher 3: Wild Hunt, ..." — while the exact title is the unambiguous #1 search hit and an obvious intent match.
- **Root cause:** `resolveExactTitleMatches` keeps only normalized EXACT-title matches. "the witcher 3" ≠ "the witcher 3: wild hunt" under any case/punctuation fold, so resolution fails before ranking. The nearby-hint does helpfully list the intended title first, but the command refuses what every human means.
- **Blast radius:** all six title-resolving commands (ratings, versus, series, backlog add, suggested, similar) — same resolver family as F-U2/F-U3. This is the opposite failure mode from F-U2: hard fail where users meant something obvious, vs silent wrong anchor where RAWG named oddly.
- **Proposed fix:** when exact matches = 0, fall back to the top-ranked search hit with an explicit stderr notice ("resolved 'the witcher 3' → 'The Witcher 3: Wild Hunt' (2015); use the full title or id to be sure"), honoring --year if passed. Exact matches keep current behavior. This also gives F-U3's series command a sane re-anchor path.
- **Status:** FIXED. When no normalized exact match exists, the resolver falls back to the top-ranked search hit with an explicit stderr notice ("resolved 'the witcher 3' to 'The Witcher 3: Wild Hunt' (2015); no exact title match — use the full title or a RAWG id to pin"). Applies to ratings/versus/series/suggested/similar/backlog add via resolveTitleForMultiSource; games get keeps its strict informative error by design. Verified: suggested "the witcher 3" exit 0 with notice.

---

## F-U6 — bare invocation of NoArgs commands prints help instead of running (confirmed systemic; CLI's own examples contradict it)
- **Found:** via herdr-pane UAT: `backlog list` bare printed full help while 2 rows sat in the store. Scoped further: `queue`, `finishline`, `trending`, `moods list`, and `tonight` bare ALL print help; `analytics` bare runs. Exit 0 in every case.
- **Root cause:** the `len(args)==0 && cmd.Flags().NFlag()==0 → cmd.Help()` guard is applied to NoArgs read commands where bare invocation is the documented default. Self-contradictions observed: queue/finishline/moods list/backlog list each list bare invocation as their FIRST Example; tonight's Long explicitly documents a no-flag surprise-me default ("With no --mood, a surprise-me mood is picked"); the guard blocks exactly that. analytics lacking the guard proves the pattern is inconsistent across siblings, not a deliberate policy.
- **Impact:** a user typing the command from its own help examples gets help, not results — the single most-discoverable invocation is broken for 6 commands.
- **Evidence:** pane transcript w8:pC T6/T9/T11/T13/T14/T17 (help-blocked) vs T12 (runs); each command's own Examples/Long text captured in the same transcript.
- **Proposed fix:** remove the no-flag guard from NoArgs read commands with safe defaults (backlog list, queue, finishline, trending, moods list, tonight); keep it only for commands that genuinely need input (e.g. tonight without any budget default could keep it — but tonight HAS a --time default of 60). One-line guard removal per command, 6 files.
- **Status:** FIXED. Guard removed from 12 NoArgs commands with safe defaults: backlog list, backlog audit, queue, finishline, trending, moods list, tonight, games popular, games top-rated, games upcoming, discover, radar. Positional-requiring commands keep the guard. Verified: all 12 produce real output bare (zero help-blocks).

## F-U7 — analytics human-mode table misaligned + misleading "not synced" hint (confirmed cosmetic)
- **Found:** `analytics` bare ran, printed "hint: local store has not been synced yet. Run 'game-goat-pp-cli sync' before trusting local results." — then a misaligned table: dashes row spans only the first column; "games   65" row does not align under the Count header.
- **Root cause:** table renderer width/alignment bug (dashes and column padding computed differently); the sync hint fires off the sync-mirror state while the cache tables (populated by browse) hold the counts being printed — the hint's blanket "not been synced" contradicts the data shown.
- **Proposed fix:** fix the analytics table renderer alignment; scope the sync hint to commands that actually read the sync mirror, or reword ("sync mirror empty — browse cache shown").
- **Status:** FIXED. Summary table renders with sorted keys and dynamic-width aligned columns; the "not synced" hint is suppressed whenever the status table has data (fires only when there is nothing to show). Verified: "Resource Type  Count" aligned with two-space padding, creators/games rows aligned, no misleading hint.

## F-U8 — backlog add renders different column orders across invocations (confirmed cosmetic)
- **Found:** first-ever add (Hollow Knight) rendered `TITLE STATUS ADDED HOURS`; second add (Celeste) rendered `TITLE STATUS HOURS ADDED`. Same command, two runs, different column order — two render paths (likely store-creation vs append).
- **Proposed fix:** unify the post-add table renderer on one column set/order.
- **Status:** FIXED. Root cause was deeper than first logged: prioritizeFields (helpers.go) ordered same-tier columns by Go map iteration — randomized per process, so every printAutoTable table had nondeterministic column order (backlog was just where it was noticed). Within-tier order is now alphabetical (deterministic). Verified: backlog add + backlog list render TITLE STATUS ADDED HOURS consistently across processes.

## F-U9 — series list incomplete (pagination) and reads unsorted in human mode (confirmed, post-fix-pass UAT)
- **Found:** user ran `gg series` post-F-U3 fix: "its list is not sorted, nor is it complete."
- **Diagnosis (two independent defects):**
  - **Incomplete — pagination:** fetchSeriesGames made one unpaginated GET to /games/{id}/game-series. RAWG's default page_size for that endpoint is 10, so any franchise larger than 10 silently truncated to page 1 — for the re-anchored Zelda query that meant only the modern era (2009-2023); the classics (Ocarina, Majora's Mask, Wind Waker, Twilight Princess, the 1986 original) were on page 2. RAWG holds 26 games for that anchor; the CLI showed 10. Additionally the anchor game itself never appears (the endpoint returns only *other* games), so Skyward Sword was absent from its own franchise list.
  - **Reads unsorted — display:** entries WERE date-sorted, but printAutoTable orders columns by its tier heuristic (NAME first, ORDER third+), so the sorted-by-date list visually scanned as a jumble.
- **Evidence:** raw RAWG probe (game-series count=26, page_size default 10; full 40/page list contains the complete 1986-2023 franchise); pane repro showing 10-game JSON + human table.
- **Fix (series.go):** (1) fetchSeriesEndpoint drains the endpoint at page_size=40 following pages (up to 5) with id-dedupe; parent-games fallback gets the same treatment. (2) The anchor game is appended to the results (it is a franchise entry). (3) Human table hand-rendered with fixed column order ORDER NAME YEAR RATING PLAYTIME BACKLOG so play order leads. (4) --limit default 20→50. (5) Re-anchor search window 5→8 (best-rated franchise entry more likely in-window).
- **Verified:** pane run shows 27 entries 1986→2023 (full franchise incl. anchor at #19, Skyward Sword), ORDER first, next_unplayed #1 The Legend of Zelda (1986). Regressions: yakuza 15 games sorted 2005→2020; partial title "the witcher 3" resolves with notice + anchor included; RE4 2005 via game-series. Build/vet/test green.
- **Status:** FIXED (same-day follow-up to F-U3).

## F-U10 — tonight's session filter passes RAWG playtime=0 (unknown) games (confirmed, round 3)
- **Found:** `tonight --time 15` recommended "Keep on Mining!" (RAWG playtime 0) inside a 15-minute window. Playtime 0 on RAWG means "no average recorded," not "very short" — an unknown duration can't be verified to fit the session, which is tonight's entire promise.
- **Root cause:** fitsTimeBudget treats playtime 0 as trivially fitting every budget (`0*60 <= anything`); filterByPlaytimeBand's zero-bounds-are-unbounded rule let 0h games through band checks too. The old behavior was even unit-tested as intended ("zero playtime always fits") — the contract itself was wrong.
- **Fix (tonight.go):** both filters now reject playtime <= 0; TestFitsTimeBudget updated to the new contract (two explicit unknown-playtime cases, with comment citing F-U10). The in-progress backlog lead is unaffected (exempt by design — "pick up where you left off").
- **Verified:** `tonight --time 15` now returns only the Celeste in-progress lead (no unmeasured recipe picks); cozy 90-min regression still returns real picks (A Castle Full of Cats 2h, Hidden Cats in Paris 2h). Build/vet/test green.
- **Status:** FIXED.

## F-U11 — versus compares a game against itself; shared --year cannot express remake pairs (confirmed, round 3)
- **Found:** `versus "resident evil 4" "resident evil 4"` resolved both sides to RE4 2023 and rendered an all-tie self-comparison (no guard). Related: versus has ONE --year flag applied to both titles, so comparing a remake pair (2005 original vs 2023 remake) is impossible even with distinct intents.
- **Fix (versus.go):** post-resolution guard — both sides resolving to the same RAWG id returns a clean usage error ("both titles resolved to the same game... nothing to compare; pass two different titles"), exit 2. meta.ambiguous duplication (identical candidate list from both sides) disappears with the guard.
- **Not fixed (design gap, noted):** per-side year pinning (e.g. --year-a/--year-b, or accepting RAWG ids as titles) would unlock remake-pair comparisons — a feature change, not a bug patch; flagged for a future amend.
- **Verified:** self-comparison exits 2 with the message; distinct pair (celeste vs hollow knight) still renders. Build/vet/test green.
- **Status:** FIXED (guard); remake-pair pinning logged as enhancement backlog.

---

## UAT round 3 results — 2026-09-27 (groups E/F/G + fixes F-U10/F-U11)

### Passes
| # | Test | Result |
|---|------|--------|
| E4 | tonight --seed 42 twice / seed 7 | Deterministic: 42→challenging both runs; 7→puzzle |
| E6 | tonight --mood bogus | Clean error listing all 10 valid moods, exit 2 (verified without pipe) |
| E8 | retention "Elden Ring" | Full TTD stats: 8219 added, 14.1% beaten, verdict "aspirational-trap" — exit 0 |
| E9 | games achievements games-read 9767 | 39 achievements via the generated two-level shape, exit 0 |
| E10 | unknown subcommand / unknown flag | Both exit 2 with actionable errors |
| E11 | --csv / --quiet machine modes | CSV headers+rows correct; quiet one-value-per-line correct |
| F1-F3 | backlog add lifecycle (Stardew Valley disposable row) | add → idempotent re-add notice → in-place update with --status finished --hours 90 (finished_at set) |
| F4 | finishline with finished signal | finished_pct 33.3%, finished_per_month 1, months_to_zero 2 — projection engages |
| F5 | queue excludes finished | Stardew absent; Celeste lead + Hollow Knight tiers correct |
| F6 | backlog show / search | Full row detail incl. hours/finished_at; search matches by slug |
| F8-F9 | backlog remove + restore | {"removed": true}; store back to exactly celeste + hollow-knight |
| G1 | studio team-cherry | Resolves developer with notice; timeline w/ backlog flag + metacritic; Silksong 2025 present |
| G2 | radar --limit 5 | Taste inferred from backlog (Indie/Action 2/3), GTA VI flagged, reasons carry genre counts |
| G3 | moods show cozy | Recipe detail + both actionable commands |
| G4 | finishline human | Clean card; correct no-signal message |
| G5 | backlog audit human | Clean decision-debt summary |

### Nits (not fixed, cosmetic)
- finishline milestones: every row reads "next" (only the first unreached one truly is) — pre-existing, still open.
- studio timeline rows newest-first; ascending would read more like a timeline.
- radar RATING cell empty for unrated upcoming releases (consistent, but a dash would read better).
- Generated two-level subcommand shape (games achievements games-read <id>) is awkward but generator-standard.
- finishline months_active is sub-day for a same-day backlog, making finished_per_month wildly optimistic — mathematically consistent with documented semantics, worth a "too-young-to-project" guard someday.

### Store state after round
Untouched user rows only (celeste in-progress, hollow-knight backlog). All disposable test rows removed.

## F-U12 — similar and suggested converge on identical output; both irrelevant for the seed (confirmed, follow-up UAT)
- **Found:** user ran `similar "megabonk"` and `suggested "megabonk"`: (1) outputs effectively the same list; (2) the recommendations felt irrelevant.
- **Root cause 1 — convergence:** the F-U4 fix gave suggested's tier-gated fallback the same scoring helper (scoreSimilarity) and the same -rating genre join as similar. On a free RAWG key the suggested endpoint is ALWAYS 401-gated, so both commands shipped the identical algorithm — measured 8/10 identical results on megabonk.
- **Root cause 2 — genre-only matching is too coarse:** megabonk is a 2025 indie survivor-like; its RAWG genres are Action/Casual/Indie, which pull The Last of Us, Mass Effect Trilogy, Witcher 3. The mechanics signal lives in RAWG TAGS (Roguelite, Bullet Hell, Loot, Action Roguelike) — unused by both commands.
- **API mechanics established by probe:** (a) RAWG's comma-separated `tags=` is a UNION, not intersection — multi-tag queries collapse to the broadest tag (loot,tps,action-roguelike returned the TPS list exactly); (b) raw rarity ordering picks trappings over identity (Loot 2937 / TPS 4195 are "rarer" than Roguelite 8822 but not the game's identity); (c) `-rating` ordering in niche tags surfaces obscure rating outliers (Action Roguelike: 2 spam games) while `-added` surfaces the niche canon (Roguelite → Hades, Dead Cells, Vampire Survivors).
- **Fix (similar.go + rawgNamedRef):** similar is now a mechanics match. Seed's gameplay tags are noise-filtered (Steam/audio/platform metadata), narrowed to a curated mechanics vocabulary (~45 loop names: roguelite, bullet hell, metroidvania, open world...), each candidate tag's -added top-20 neighborhood is probed, and the winning tag is the one whose neighborhood CO-OCCURS most with the other tags' neighborhoods — the seed's identity cluster, discovered from data rather than guessed (megabonk: hack-and-slash lone trapping loses to the roguelike+roguelite+bullet-hell cluster). Reasons name the defining tag; the genre join remains only as fill/no-tag fallback. suggested deliberately keeps the broader genre-join fallback, so the two commands now answer different questions (mechanics vs genre) — measured overlap after fix: 0/10. rawgNamedRef gained games_count (RAWG's per-tag census) to power the rarity ordering.
- **Implementation bugs hit and fixed:** shared-seen dedupe zeroed the co-occurrence signal (probe sets must be independent); the first union-ladder design collapsed to the broadest tag.
- **Verified:** similar megabonk → Hades, Dead Cells, Darkest Dungeon, Isaac, FTL, Enter the Gungeon, Vampire Survivors, Spelunky (reason "Roguelite") — the survivor-like canon, exactly the complaint's target. witcher 3 → RDR2, Skyrim, Cyberpunk 2077, Fallout 4 (Open World). hollow knight → Ori, Dead Cells + RAWG's looser metroidvania entries (Arkham, Control — RAWG's own tagging). Human table renders; build/vet/test green.
- **Cosmetic nit (open):** the per-row reason repeats the same defining tag on every row in human mode; meta.note carries it too — consider showing it once.
- **Status:** FIXED.

## F-U13 — suggested leads with tiny-sample 4.7s; studio signal unused (confirmed, follow-up UAT)
- **Found:** user's pasted `suggested "megabonk"` human output — the leading rows carried 4.7 ratings backed by 6-7 ratings (Sonic Triple Trouble 16-Bit fan game, Bike Baron, Superfighters Deluxe) while genuinely comparable picks sat in the tail. (This entry was reconstructed at session close — the fix landed but the ledger entry was never written at the time; caught by the close-out count check.)
- **Root cause:** (1) RAWG's `-rating` ordering surfaces obscure titles whose handful of fans rated them 4.7 — the genre-join pool ranked that noise first; (2) the fallback used no studio signal even though RAWG exposes the studio on the detail record (probe: `/games/{id}/development-team` returns PEOPLE, not studios; the detail record's `developers` field is the studio).
- **Fix (suggested.go):** same-studio tier from `seed.Developers` (`/games?developers=<id>`), `ratings_count` surfaced on every result and shown in human mode as "(N)", `minSuggestedRatingSample=20` confidence floor (relaxed only when the confident set is empty), ranking by genre overlap → metacritic → sample size → rating. Long/usage text updated.
- **Verified:** megabonk → confident list led by TLOU Remastered (2858 ratings), Witcher 3 (7282); hollow knight → Team Cherry Silksong studio tier; witcher 3 → full CD PROJEKT RED catalog with sample counts.
- **Status:** FIXED. (Later subsumed by F-U15's three-tier redesign, which kept the studio tier, floor, and ranking; F-U14 additionally tightened the floor to non-empty confident sets.)

## F-U14 — obscure exact-title hits hijack franchise queries: "halo" resolves to a 3-add itch RPG (confirmed, follow-up UAT)
- **Found:** user ran `suggested "halo"` — top recommendation Persona 5 Royal. Irrelevant on its face.
- **Root cause — seed mis-resolution, not the pool:** RAWG's #1 search hit for "halo" is an obscure indie RPG literally named "Halo" (id 609130, added=3, genre RPG). The resolver's exact-title rule binds to it; the genre join then faithfully recommends RPGs — Persona 5 was correct output for the wrong seed. The real Halo games (Reach, Infinite, ODST) sit below the junk hit in RAWG's relevance ordering. Same pattern as F-U3 ("zelda" -> junk "Zelda"), now amplified: any franchise name that an obscure game shares exactly will hijack resolution across ALL six title-resolving commands.
- **Signal probed:** RAWG's library-add count is the community-recognition signal — obscure "Halo" added=3 vs Halo Infinite 7888 (2600x); junk "Zelda" added=10 vs Zelda II 433; real classics (DOOM 1993: 1736, Celeste: thousands) sit far above any obscure floor, so confident exact matches never get overridden.
- **Fix (games_search.go + ratings.go + suggested.go):** (1) new pure helpers — bestKnownGame, isFranchiseContinuation (word/colon boundary; "halo" -> "halo infinite" yes, "halo's adventure" no), franchiseOverride (fires only when the single exact hit is below added=100 AND a continuation clears 10x dominance plus a 200 floor). (2) resolveTitleForMultiSource: the override resolves franchise shorthand to the dominant franchise entry with an explicit pin-able stderr notice; the F-U5 partial-title fallback now picks the best-known hit (max added) instead of RAWG's first. (3) suggested's confidence floor no longer relaxes when a non-empty confident set exists (F-U13's megabonk output still led with 6-7-rating 4.7s because the floor only applied at full list size).
- **Verified:** `suggested "halo"` -> notice -> Halo Infinite seed -> **343 Industries studio tier: MCC, Halo 4, Halo 5, Halo 2 Anniversary, Halo Wars, Spartan Assault/Strike** — Persona 5 gone; human table renders with sample counts. `ratings "halo"` -> Halo Infinite card + notice. `series "zelda"` -> resolves directly to Zelda II (no longer needs the empty re-anchor) -> full franchise. Regressions hold: megabonk (obscure exact but no continuation — unchanged, now confident-only list: TLOU Remastered 2858, Witcher 3 7282 lead), witcher 3 / hollow knight studio tiers, doom ambiguity signal (1993 vs 2016), backlog add. New unit tests: TestIsFranchiseContinuation, TestFranchiseOverride (dominance, confident-exact, no-continuation cases), TestBestKnownGame. Full suite green; build/vet clean.
- **Cosmetic nits (open):** the same-studio tier does not apply the rating-sample floor (Halo Campaign Evolved 11 ratings, Silksong Soundtrack Bundle 0 ratings appear) — defensible for a strong-signal tier, worth a floor someday.
- **Status:** FIXED.

## F-U15 — suggested's genre-join fallback recommends the broad-genre canon regardless of seed (confirmed; the megabonk irrelevance, post F-U13/F-U14)
- **Found:** user: `suggested "megabonk"` still returns "completely irrelevant" titles (The Last of Us, Witcher 3...). Distinct from F-U14 (halo = mis-resolution, fixed): megabonk resolves correctly, yet the POOL is irrelevant.
- **Root cause — the genre join is structurally too coarse:** with no studio data and the suggested endpoint business-gated, the fallback's only tier was the shared-genre join. Megabonk's genres (Action/Casual/Indie) make "Action" the join key, and every AAA blockbuster carries Action — so the most-popular-Action canon is the mathematically correct output of the wrong model. Confidence floors and overlap ranking only reorder that canon; they cannot make "Action" specific. The design kept the genre join for similar-vs-suggested differentiation (F-U12), a trade the user has now rejected twice: relevance beats differentiation.
- **Fix (suggested.go + similar.go):** suggested's fallback is now three tiers: (1) same-studio (unchanged), (2) NEW mechanics-tag tier — the seed's defining gameplay tag via mechanicsClusterNeighborhood, the identity-cluster probe EXTRACTED from similar into a shared helper (independent -added top-20 probes per mechanics tag, co-occurrence-scored winner), confidence-filtered and ranked by the suggested scoring (shared-genre overlap → metacritic → sample size → rating), (3) shared-genre join ONLY when earlier tiers run short (was: always). Reasons carry the signal ("plays like Megabonk — roguelite"); meta.note names the tiers that actually fired; Long/usage text updated. similar's RunE now calls the shared helper (behavior verified unchanged).
- **Verified live:** suggested megabonk → **Vampire Survivors (803 ratings), Hades (2123), Dead Cells (1754), Crypt of the NecroDancer, Rogue Legacy, Isaac** — reason "plays like Megabonk — roguelite", note "fallback: mechanics-tag matches (roguelite)". suggested hollow knight → Team Cherry (Silksong) then FEZ/Dead Cells/Guacamelee ("metroidvania") — tiers compose. suggested halo / witcher 3 → studio tier fills, tag tier skipped, unchanged output (notes now correctly say "same-studio games" only). similar megabonk regression → identical roguelite canon after the helper extraction. Build/vet/full-suite green.
- **Differentiation note:** suggested and similar now share the tag signal by design — suggested stays distinct via the studio tier, confidence ranking (similar orders by the niche canon), and reasons. Overlap between the two on tag-only seeds (megabonk) is now accepted as correct: both commands describe the same game's identity; their difference is framing (canon order vs confidence order), not relevance.
- **Status:** FIXED.

## F-U16 — data-scarce seeds get genre-canon suggestions with no disclosure (confirmed; guildrun)
- **Found:** user ran `suggested "guildrun"` — Shovel Knight, Deltarune, "I don't agree with these suggested results."
- **Diagnosis — not an algorithm bug, a disclosure gap:** Guildrun on RAWG is an unreleased indie with added=1, genres=[Indie], ZERO tags, ZERO developers. Studio tier: nothing to fetch. Mechanics tier: no tags to cluster. The genre join is the only signal RAWG offers, and Shovel Knight / Deltarune genuinely are its confidence-ranked top. The picks are the honest ceiling of the data — but the CLI presented them as if they were seed-specific matches, which reads as broken.
- **Fix (suggested.go + similar.go; strengthened after advisor review — first pass disclosed only):** weak rows are now HIDDEN, not just labeled. When the genre join was the only usable tier AND the seed has no gameplay tags and no studio, any row sharing ONLY a broad genre (Indie, Action, Adventure) is dropped — the popularity canon is never presented as suggestions for a seed it has no signal about. The JSON note names the hiding ("weak single-broad-genre matches are hidden (N hidden)") and human mode prints the stderr notice; anything that survives shares a specific genre and earns its place. The same hide rule covers similar's genre-only path. sharesOnlyBroadGenre is a pure helper with TestSharesOnlyBroadGenre covering the boundary (only-broad shared = weak; two shared or a specific genre = kept).
- **Verified:** guildrun → 0 results in BOTH suggested (3 rows hidden, notice + note) and similar (note; empty list) — the Shovel Knight/Deltarune output is gone. Rich seeds unchanged: suggested megabonk (Vampire Survivors first) / hollow knight (Silksong) / halo (Halo 2 Anniversary) / witcher 3 (Blood and Wine), similar megabonk (Hades) / hollow knight (Control) / witcher 3 (GTA V) all full. Also on advisor request: full `go test ./...` green (all packages), similar hollow-knight refactor regression holds (metroidvania canon unchanged), F-U14 halo fix intact post-F-U15. Build/vet green.
- **Design note:** for a 1-add unreleased indie, "top games in its genre" is the best any RAWG-derived engine can honestly say; the fix makes the CLI say it instead of implying more.
- **Status:** FIXED.

---

## UAT round results — executed via herdr pane w8:pC, 2026-09-27

### Passes (no action needed)
| # | Test | Result |
|---|------|--------|
| T1 | versus celeste hollow knight | Clean 5-dimension card (rating/count/metacritic/playtime/steam), correct resolution, verdict 2-2-1, backlog flags accurate, exit 0 |
| T2 | games get 99999999 | Exit 3 (correct per README taxonomy: 3=not-found), clean error, no crash. NOTE: my earlier tour message said "exit 5" — that was my error, the CLI is right |
| T3 | backlog add --dry-run | "would run backlog add; no changes made", exit 0, store untouched |
| T4 | backlog add Hollow Knight | Correct RAWG id 9767, status backlog, mini table |
| T5 | backlog add Celeste --status in-progress | Correct id 22121, in-progress |
| T7 | backlog add zzzqqq | Clean exit 3 not-found with actionable hint, no crash, no write |
| T8 | backlog list --limit 50 | Both rows correct, newest first |
| T10 | tonight --mood cozy --time 90 | Celeste leads "pick up where you left off" (in-progress lead works); cozy picks fit 90-min budget (2h games); HK correctly excluded by playtime filter (~7h > 1.5x budget) |
| T12 | analytics bare | Runs (see F-U7 for rendering), counts correct (65 games from browse cache, 10 creators) |
| T15 | queue --limit 3 | Tier logic perfect: Celeste "already started — pick up where you left off", Hollow Knight "highest-rated on your backlog", live ratings present |
| T16 | finishline --json | finished_pct 0, months_to_zero 0 with documented not-enough-signal semantics; milestone math correct (HK crosses 10-50%, Celeste 60-100%, oldest-active-first ordering right) |

### Nits (cosmetic, not numbered as findings)
- backlog add --dry-run message is generic ("would run backlog add") — could show the resolved game.
- games get 404 hint suggests "Run the 'list' command" — meaningless for a game-id lookup; 'games search' is the actionable hint.
- versus shows leader on "4.4 vs 4.4" — leader computed on unrounded values, display rounds to a tie; show one more decimal or "tie (4.44 vs 4.36)".
- finishline milestones all carry "status": "next" — only the first unreached milestone is truly next.
- tonight PLAYTIME column empty for the in-progress lead row.

### Methodology note
Tests ran in herdr pane w8:pC (split from the pi pane, no-focus). Two marker echoes containing parens caused benign bash syntax errors in the pane (T13/T14); test results unaffected. Pane left alive at prompt for further manual testing.

---

*(append new findings above the line as they surface)*
