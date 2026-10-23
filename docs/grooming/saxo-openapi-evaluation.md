# Grooming: Saxo OpenAPI as alternate broker

Evaluation against copilot requirements (official Saxo developer docs only). Current execution target remains **Bybit EU spot demo** — see [bybit-eu-demo.md](../execution/bybit-eu-demo.md).

## Requirements

| Requirement | Verdict | Notes |
|-------------|---------|--------|
| Fully automated API trading | **Yes** (SIM); **Partial** (LIVE) | REST + WebSocket; `ManualOrder: false` for unattended orders. LIVE needs funded Saxo account, separate app credentials, approval. **1 order/s/session** documented rate limit. |
| Demo with artificial balance | **Yes** (limits) | SIM ~**$100k** simulated value; `PUT …/accounts/{AccountKey}/reset` on trial accounts. SIM gaps: reporting, some market data. **20-day** simulation account lifetime (renewable). Default SIM often **FX-only** until [openapisupport@saxobank.com](mailto:openapisupport@saxobank.com) enables more asset classes. |
| Sign-up without ID/KYC | **Partial** / **Unclear** | Developer portal + simulation signup documented as **email/form**; **no ID step** on those pages. **Live** trading implies full client onboarding (KYC not detailed on developer portal). IB Signups API documents identity documents — partner flow, not retail dev SIM. |

## Fit for this product (3h crypto swing)

| Topic | Fit |
|-------|-----|
| API automation | Strong on SIM |
| “Spot” crypto | **Partial** — CryptoFX (`FxSpot`), not on-chain spot; **no crypto/crypto** crosses |
| 24/7 weekends | **Poor** — CryptoFX **weekdays only** ([Crypto FX in OpenAPI](https://www.developer.saxo/openapi/learn/crypto-fx-in-openapi)) |
| vs Bybit EU demo | Saxo: richer multi-asset, OAuth, SIM reset API. Bybit: native spot, 24/7, existing stack; EU demo lacks `wallet-balance` route (transaction-log fallback). |

## Environments (reference)

| | SIM | LIVE |
|---|-----|------|
| REST | `https://gateway.saxobank.com/sim/openapi` | `https://gateway.saxobank.com/openapi` |
| Auth | `https://sim.logonvalidation.net` | `https://live.logonvalidation.net` |
| Portal token | 24h SIM-only quick token | Full OAuth only |

[Environments](https://www.developer.saxo/openapi/learn/environments) · [Security / OAuth](https://www.developer.saxo/openapi/learn/security) · [Order placement](https://www.developer.saxo/openapi/learn/order-placement)

## Recommendation

- **Do not replace Bybit** for the current PoC without accepting weekend gaps and CryptoFX semantics.
- **Optional spike:** SIM developer account → simulation trading account → request CryptoFX on SIM → OAuth app → BTCEUR test orders with `ManualOrder: false`.
- **Live Saxo:** only after funded Saxo Group account + LIVE OpenAPI approval; retail Germany eligibility not spelled out in developer docs.

## Doc gaps (explicit)

Retail live KYC steps, Germany-specific eligibility, and whether SIM signup ever collects ID are **not** stated on the developer pages used for this evaluation.
