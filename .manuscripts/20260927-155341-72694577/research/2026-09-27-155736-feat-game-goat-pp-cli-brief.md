# Game Goat CLI Brief

## API Identity
- **Kind:** Fusion CLI (movie-goat / flight-goat family) — multiple sources stitched into one agent-native query surface with a local SQLite store.
- **Domains:** api.rawg.io (spine), www.cheapshark.com/api/1.0 (deals), store.steampowered.com/api + api.steampowered.com (Steam store + Web API), steamspy.com/api.php (playtime/owners), howlongtobeat.com (completion times, reverse-engineered).
- **Users:** PC gamers carrying backlogs and wishlists; agents answering "what should I play tonight"; deal hunters who want price context without an ITAD account.
- **Data profile:** game metadata (ratings, Metacritic, genres/tags/platforms/stores/screenshots), deal prices across ~30 stores, review scores, completion/playtime hours, owned-library + playtime.

## Reachability Risk
- **RAWG (spine): Low.** Official Swagger 2.0 spec (30 paths) at https://api.rawg.io/docs/?format=openapi — fetched 200, cached at /tmp/printing-press-fetch-docs/api.rawg.io-docs-format-openapi-3a5cdaa571ce9a58.json. Auth: free `key` query param required — without it every path 404s (probe evidence: `GET /games?page_size=1` → 404). Free tier: up to 20,000 requests/month (rawg.io/apidocs tier table).
- **Tier gate (critical):** `/games/{id}/suggested` is documented as "available only for business and enterprise API users" — the `similar` command MUST NOT be built on it. Fallback: client-side join over `/games?genres=<g>&tags=<t>&ordering=-rating` (free tier) plus `/games/{id}/game-series` for same-series picks.
- **CheapShark: Low.** Keyless. Contract detail: requests without a descriptive User-Agent get 400 `{"error": "Missing or generic User-Agent header detected..."}`; with a descriptive UA (probe evidence) stores + deals + games endpoints return 200. The generated client must send a descriptive UA (e.g. `game-goat-pp-cli/<version>`).
- **Steam: Low for store surfaces.** Keyless: appdetails (note: response is keyed by an internal id, not the requested appid — parse by scanning values' steam_appid), appreviews (review_score/desc + totals), storesearch (id/name/price), GetNumberOfCurrentPlayers. **Keyed:** IPlayerService/GetOwnedGames → 401 `verify your key= parameter` without a key — library/backlog commands need a free STEAM_API_KEY + public profile. Post-2025 Steam Web API budget: 25 req/s (steam-web CLI documents this).
- **SteamSpy: Low.** Keyless; rate limit 1 req/sec (1 req/60s for `all` pages); data refreshed daily; owners given as ranges; playtime in minutes.
- **HowLongToBeat: Medium.** No official API. Reverse-engineered contract (from ckatzorke/howlongtobeat, 420★, actively maintained): POST https://howlongtobeat.com/api/search with JSON payload (searchTerms, searchType "games", size, searchOptions.games{platform, sortCategory, rangeCategory, rangeYear, playstyle...}), headers content-type: application/json + origin/referer https://howlongtobeat.com + real User-Agent; detail page HTML at /game?id for per-class times. Brittle scraping surface — browser-sniff gate validates before committing.
- **OpenCritic: EXCLUDED (proposed).** api.opencritic.com returns 404s (Cannot GET /, /game/search gone), readthedocs retired. No official API anymore; would require scraping a dead surface. Metacritic signal already arrives via RAWG's `metacritic` field and CheapShark's `metacriticScore` on deal rows. Flag for user confirmation at absorb gate.

## Top Workflows
1. `tonight` — "what do I play tonight" from my library + trending, filtered by --mood/--tags, --platform, --max-hours (HLTB completion time), --under (current best price). One call replaces HLTB + Steam library + Reddit "what should I play" bouncing.
2. `ratings <game>` — RAWG rating + Metacritic + Steam review score (+ player count) in one card; degrades gracefully per missing source.
3. `backlog` — decision-debt analytics: unplayed %, average shelf-time (bought → first played), stale purchases, bought-vs-finished taste mismatch (genres bought vs genres finished).
4. `deals` / `wishlist-watch` — CheapShark current deals joined against local price-history table: historical low per game, price-drop alerts (drop since last snapshot), underpriced-right-now (deal price vs historical low).
5. `similar <game>` / `versus <a> <b>` — recommendations via RAWG genre/tag join (+ game-series) and head-to-head comparison across ratings, playtime, price, platform.

## Table Stakes
- **IsThereAnyDeal** (official API, free key): waitlist, historical lows, per-store price comparison — the incumbent for wishlist watching. ITAD requires an account+key; game-goat does the same reads keyless via CheapShark + a local price-history table.
- **gg.deals / Deku Deals**: curated deal browsing, no official APIs; Deku is Switch-focused.
- **steam-web CLI** (public library, @tmchow): 169 Steam Web API endpoints mirrored, SQLite FTS5 store, library audit / next-achievement commands, 25 req/s throttling — game-goat must complement it (ritual fusion) not mirror it.
- **metacritic CLI** (public library): undocumented internal JSON API (backend.metacritic.com, embedded public apiKey, mcoTypeId=13 games) — ratings parity available if RAWG's metacritic field is insufficient.
- **Depressurizer** (1526★): whole tool built just to categorize/trim Steam libraries — proof the decision-debt pain is real and unserved at the ritual level.
- **shouldiplay** (web, 52★): combines HLTB + OpenCritic to answer "should I play X" — the single-answer desire exists; no CLI/local-store equivalent.
- **HLTB community wrappers** (npm `howlongtobeat`, PyPI): search + detail only, no CLI, no store, no fusion.

## Data Layer
- **Primary entities:** `game` (unified spine: rawg_id + steam_appid + cheapshark_game_id + hltb_game_id cross-reference mapping), `deal` (+ `price_snapshot` history for wishlist-watch), `library_entry` (owned, playtime_forever, playtime_2weeks, last_played, status), `wishlist_entry`, `store`, `genre`/`tag`/`platform`, `beat_time` (HLTB main/extra/completionist).
- **Sync cursor:** RAWG next-page URL; CheapShark deals by store/page + on-sale filters; SteamSpy 1k-entry pages; HLTB detail fetched on demand; Steam library by steamid.
- **FTS/search:** FTS5 over game name/aliases/genres/tags (RAWG's search_precise/search_exact params for live resolution).
- **High-gravity:** game, deal, library_entry, wishlist_entry, price_snapshot. The cross-source ID mapping table is the novel durable asset.

## Codebase Intelligence
- N/A — multi-source fusion; per-source contracts documented in Reachability Risk.

## User Vision
- "A game library isn't a list of purchases. It's a decision-debt ledger: every unplayed game, wishlisted price, and abandoned playthrough is a signal about actual taste vs. aspirational taste, and every price drop is a signal about when to buy." Rung 4-5 commands are built around that insight: `backlog` analytics, `wishlist-watch` price history, `tonight` filtered by mood/time/budget.
- CLI name game-goat, binary game-goat-pp-cli, future skill pp-game-goat. Read-only — no write/purchase/order flows. RAWG_API_KEY provided in environment for live testing. Standard layer: sync, search (FTS5), sql, export, --json/--compact, typed exit codes, --dry-run.

## Source Priority
- **Primary: RAWG** — official Swagger 2.0 spec (30 paths) — auth: free key (RAWG_API_KEY), 20k req/month free tier. Spine: search, details (metacritic, rating, playtime, esrb, platforms, stores, screenshots), genres/tags/platforms/stores reference lists, achievements, game-series.
- **Secondary: CheapShark** — keyless (descriptive UA required) — deals, games lookup, ~30-store reference; deal rows carry metacriticScore + steamRating* fields.
- **Tertiary: Steam Web / SteamSpy** — keyless store surfaces (appdetails/appreviews/storesearch/player-count) + SteamSpy (owners, avg/median playtime, ccu); library+playtime requires free STEAM_API_KEY (401 evidence).
- **Quaternary: HowLongToBeat** — no official API; reverse-engineered POST search + detail HTML; browser-sniff gate decides inclusion.
- **Excluded (pending user confirm): OpenCritic** — API retired.
- **Economics:** all free sources; RAWG free key scoped to RAWG commands; STEAM_API_KEY (free) optional — gates only library/backlog commands; no paid sources anywhere.
- **Inversion risk:** RAWG has the only official spec; none of the other sources publish specs. Do not let spec availability reorder — the user-confirmed priority is RAWG spine. Do NOT build `similar` on the tier-gated /suggested endpoint.

## Product Thesis
- **Name:** game-goat (binary game-goat-pp-cli).
- **Why it should exist:** ITAD holds prices, HLTB holds hours, RAWG holds taste metadata, Steam holds your library — no tool joins them, and none of the incumbents is agent-native or works offline. game-goat is the decision-debt ledger: one local SQLite spine that answers tonight/backlog/deals/similar questions in a single call.

## Build Priorities
1. RAWG spine: search/get/ratings card with graceful multi-source degradation; game resolution with remake-aware ambiguity (movie-goat pattern: "Doom 1993 vs Doom 2016" — --year, meta.ambiguous).
2. `tonight` ritual: RAWG trending + library join, --mood/--tags, --platform, --max-hours (HLTB), --under.
3. `backlog` decision-debt analytics over local store (needs STEAM_API_KEY; graceful skip without).
4. `deals`/`wishlist-watch`: CheapShark + price_snapshot history table (historical low, drops, underpriced-now).
5. `similar` (RAWG genre/tag join + game-series; NOT /suggested) and `versus`.
6. Standard goat layer: sync, search (FTS5), sql, export, --json/--compact/--agent, --select, --dry-run, typed exit codes, doctor, auth (config.toml 0600 + env override: RAWG_API_KEY required, STEAM_API_KEY optional), learnings/teach loop, feedback, --deliver sinks, profiles, MCP server.

## Raw captures
- RAWG OpenAPI spec: /tmp/printing-press-fetch-docs/api.rawg.io-docs-format-openapi-3a5cdaa571ce9a58.json
- RAWG tier table: /tmp/printing-press-fetch-docs/rawg.io-apidocs-ba001f3c29188a74.html
- CheapShark docs: /tmp/printing-press-fetch-docs/apidocs.cheapshark.com-f314914c8cdf7458.html
- SteamSpy docs: /tmp/printing-press-fetch-docs/steamspy.com-api.php-4faddb4122e7bc15.md

---

## Course Correction (2026-09-27, user decision at browser-sniff gate)

The multi-source fusion model (RAWG + CheapShark + Steam library + HowLongToBeat + OpenCritic) is RETIRED. game-goat is movie-goat's sibling for games — a discovery and recommendation CLI, not a deal-hunter or library tracker. flight-goat is not the reference; movie-goat is.

- **Spine (TMDb-equivalent): RAWG only.** Single base_url, free key via RAWG_API_KEY, movie-goat-style auth subcommands. Resources: games search/get, games/suggested, games/similar (heart of the recommender), genres, tags, platforms, stores, developers, lists, discover-style filtering.
- **Optional enrichment (OMDb-equivalent): keyless Steam review-score summary.** Enriches the ratings card; degrades gracefully to RAWG-only.
- **Dropped from this run:** CheapShark (deals/prices), HowLongToBeat (beat times), OpenCritic (retired API). These belong in a post-publish amend, not the first print.
- **Reference implementation:** mvanhorn/printing-press-library library/media-and-entertainment/movie-goat (spec.yaml, README, internal/). Same architecture, same command grammar, same ritual-command philosophy. On conflict, movie-goat wins.
- **Ritual commands:** tonight, ratings, similar/suggested, versus, series (= movie-goat marathon), backlog + queue (= watchlist + queue, SQLite + FTS5). Remake-aware title resolution copied exactly from movie-goat: shared names and remaster/deluxe variants ("Persona 5" vs "Persona 5 Royal") report ambiguity on stderr AND as meta.ambiguous in JSON; pin with --year or id.
- **Thesis (Non-Obvious Insight):** a game library is a decision-debt ledger — every unplayed game is a signal about actual vs. aspirational taste; the recommender's job is closing that gap, not enlarging it.
- **Absorb study targets stay:** steam-web, metacritic, boardgamegeek, IsThereAnyDeal, gg.deals.
- **Read-only scope. No purchase/write flows.**
