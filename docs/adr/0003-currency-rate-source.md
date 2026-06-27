# ADR 0003: Currency rates from open.er-api.com, fawazahmed0 as fallback

**Status**: Accepted

## Context

The launcher's unit/currency conversion needs exchange rates, ideally with no API key and
cached locally. Two no-key options were evaluated in depth, including the developers behind
them:

- **open.er-api.com** — the free open tier of ExchangeRate-API, operated by **AYR Tech (Pty)
  Ltd**, a registered South African company running a paid currency API since 2010. ~160 fiat
  currencies, daily updates, and the response includes `time_next_update_unix` (a precise
  freshness signal). Single origin host; free tier requests attribution and is revocable at
  the vendor's discretion.
- **fawazahmed0/exchange-api** — a solo open-source maintainer's project served as static JSON
  over the jsDelivr CDN. 200+ currencies plus crypto/metals, no rate limits, a mirror fallback
  URL. No legal entity / SLA; URL paths have shifted historically (currency-api → exchange-api),
  and rates would silently go stale if the maintainer steps away.

Crypto coverage (fawazahmed0's main functional edge) is **not needed**.

## Decision

Use **open.er-api.com as the primary** source and **fawazahmed0/exchange-api as the fallback**.
Fetch rates relative to a single base (USD) and derive cross-rates locally, cache to
`~/.cache/blf/currency.json`, and on fetch failure fall back to **stale cache** rather than
showing nothing. The cache TTL is driven by open.er-api's `time_next_update_unix` (falling
back to a flat 12h).

Rationale: with crypto out of scope, the deciding factor is **longevity and accountability** —
a 15-year-old company with a stable endpoint and an explicit freshness signal is the safer
institutional bet and avoids babysitting community URL changes. fawazahmed0's CDN-static model
is an excellent backstop precisely when the primary host is unavailable.

## Alternatives considered

- **fawazahmed0 primary / open.er-api fallback** — the original lean (static CDN = fastest,
  on-theme). Reversed after due-diligence weighted vendor accountability over crypto coverage,
  which isn't needed.
- **frankfurter.app** — no key but only ~30 currencies (ECB, weekday-only); too narrow even as
  a fallback.
