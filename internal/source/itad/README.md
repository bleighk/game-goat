# internal/source/itad — keyed IsThereAnyDeal price client

Sibling source-client module for game-goat-pp-cli's price commands
(`price-history`, `prices`). Unlike the keyless Steam source, ITAD is
**API-keyed**: every endpoint needs a personal key, sent as the
`ITAD-API-Key` header. Endpoints are modeled on the published OpenAPI spec
(https://docs.isthereanydeal.com/openapi.json):

| Endpoint | Purpose | Response shape |
|---|---|---|
| `GET /games/search/v1?title=<t>&results=<n>` | title → game id | array of `{id,slug,title,type,mature}` |
| `POST /games/prices/v3?country=<CC>&capacity=<n>&deals=<bool>` | current deals + all/1y/3m lows | array of `{id,historyLow:{all,y1,m3},deals:[{shop,price,regular,cut,storeLow,timestamp,expiry,url}]}` |
| `GET /games/history/v2?id=<uuid>&country=<CC>&since=<ts>` | dated price-change log | array of `{timestamp,shop,deal:{price,regular,cut}}` |
| `GET /games/info/v2?id=<uuid>` | id → game title (bare-id resolution) | object `{id,slug,title,type,mature,...}` |

## Currency localisation

Every price endpoint takes a `country` query parameter (ISO 3166-1 alpha-2).
ITAD returns each amount in that storefront region's currency, so `country`
IS the currency control: the CLI exposes it as `--country`, defaulting from
`ITAD_COUNTRY` then `US`. The selected country and the resolved currency are
surfaced in the command meta.

## API surface

- `itad.go` — typed client: `Search`, `Info`, `Prices`, `History`,
  `ResolveGame` (exact-title, game-typed ranking with ambiguity candidates),
  `SortDealsByPrice`, and sentinel errors `ErrMissingAPIKey` /
  `ErrGameNotFound` plus `IsAuthError` / `IsNotFound` status mapping.
- `doer.go` — `adaptiveDoer`: the same token-bucket + `AdaptiveLimiter`
  pacing + 429/5xx backoff policy as the Steam source, extended with POST for the
  prices endpoint. Exhausted 429 retries surface as `*cliutil.RateLimitError`.
- `config.go` — `Config{APIKey, Country, RateLimit}`. Defaults: US storefront,
  3 req/sec sustained, burst 5.

## Auth

A missing key fails fast with `ErrMissingAPIKey` before any HTTP call; the
commands translate that (and HTTP 401/403) into a code-4 auth error naming both
`export ITAD_API_KEY="..."` and the one-time `auth set-token --provider itad`
store, plus the key-registration URL. Resolution order is the env override
first, then the stored `itad_api_key` in `credentials.toml`.

## Tests

`itad_test.go` covers search parsing + key header, the prices POST body and
`country` passthrough, history parsing, the missing-key short-circuit, exact vs
ambiguous title resolution, the 429 retry-once path, 403 classification, and
cheapest-first deal sorting — all against `httptest` servers, no live calls.
