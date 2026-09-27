# internal/source/steam — keyless Steam storefront client

Sibling source-client module for game-goat-pp-cli's multi-source commands
(`ratings`, `versus`). Steam enrichment is **keyless**: no credential, no auth
header, no `os.Getenv` — the three endpoints this client calls are public
storefront surfaces, live-probed during research:

| Endpoint | Purpose | Response shape |
|---|---|---|
| `GET /api/storesearch/?term=<title>&cc=us&l=en` | title → appid | `{"total":n,"items":[{"type":"app","name":…,"id":<appid>}]}` |
| `GET /appreviews/<appid>?json=1&num_per_page=0&language=all&purchase_type=all` | review rollup | `{"success":1,"query_summary":{"review_score_desc":…,"score":…}}` |
| `GET /api/appdetails?appids=<appid>&cc=us&l=en` | price (optional) | `{"<appid>":{"success":true,"data":{…}}}` — decoded via `json.RawMessage` to tolerate the dynamic top-level key |

## API surface

- `steam.go` — typed client: `ResolveAppID`, `ReviewSummary`, `AppDetails`,
  plus sentinel errors `ErrAppNotFound` / `ErrReviewsUnavailable` so callers
  degrade on typed errors instead of string matching.
- `cli_bridge.go` — `SteamReviewForTitle(ctx, title) (*SteamReview, error)`:
  the thin wrapper `internal/cli` commands call; resolves the title, fetches
  the review summary, and opportunistically attaches price (a price failure
  degrades to `Price: nil`, never fails the review).
- `config.go` — `Config{RateLimit}` only. Keyless by design: no credential
  fields. Defaults: 3 req/sec sustained, burst 5.
- `cross_domain.go` — `adaptiveDoer`: the shared generic policy (token-bucket
  burst + `cliutil.AdaptiveLimiter` pacing + 429 backoff) that a future
  sibling source can reuse instead of re-implementing it.

## Rate-limit and retry policy

- Pacing: `cliutil.AdaptiveLimiter` at 3 req/sec with a 5-request burst
  allowance; the adaptive limiter halves the rate on 429 and ramps after
  successes.
- Retries: at most **one** retry per HTTP call, only on 429/5xx, never on
  other 4xx. A 429 backoff honors `Retry-After` (default 2s) capped at 10s.
- Exhausted 429 retries surface as `*cliutil.RateLimitError` —
  empty-on-throttle is indistinguishable from "no data exists", so the
  typed error keeps downstream commands honest.
- HTTP client: `net/http` with a 10s timeout, User-Agent
  `game-goat-pp-cli/0.1.0 (+printing-press)`, context-aware via `boundCtx`
  at every command call site.

## Why there is no `mcp_server.go`

Phase 11's MCP exposure contract says the MCP binary mirrors the Cobra tree
at startup (`internal/mcp/cobratree/`), and command annotations
(`mcp:read-only`) control tool hints — a per-module MCP server file is not
  part of the sibling source-client layout. Every command that consumes this
module (`ratings`, `versus`) is a top-level Cobra command and is therefore
exposed on the MCP surface automatically. No `mcp_server.go` stub is needed
here.

## Tests

`steam_test.go` covers storesearch parsing (incl. empty → `ErrAppNotFound`),
appreviews parsing (incl. `success:false` → `ErrReviewsUnavailable`), the
429 backoff-then-retry path (attempt-counted, real sleeps kept
milliseconds), 5xx retry-once, non-retryable 4xx, appdetails dynamic-key
parsing, and the bridge end-to-end including price degradation.
