# sec-scanner app

Static SPA (React + TypeScript + Vite + MUI) over ConfigHub's published API — a
security console for the fleet. All durable state lives in ConfigHub: the image
references are config data, the gate-signal verdict is on the workload Unit
(`sec-scanner.confighub.com/max-severity`, `…/cve-count`), and the full per-CVE
findings are read from each Space's `AppConfig/YAML` `sec-scan-record` Unit (a
multi-document YAML, one document per workload). The browser holds only the
session and UI state and computes nothing — it reads the scanner's result.

Pages:

- **Dashboard** — fleet severity rollup, gated/unscanned counts, worst workloads.
- **Fleet** — the image inventory: every workload, its image(s), scan verdict,
  and gate state, filterable by severity / free text.
- **Findings** — every CVE across the fleet, flattened, with links to NVD/GHSA.
- **Unit** — one workload: images, verdict, findings, Apply Gates, an
  **Upgrade image** action (server-side `yq-i`, dry-run previewed), the raw
  YAML, and revision history with rollback.

## Development

```bash
npm install
npm run dev          # http://localhost:5180, proxies /api and /auth to ConfigHub
```

The dev proxy targets `https://hub.confighub.com` by default; override with
`CONFIGHUB_URL=http://localhost:9090 npm run dev` for a local server.

Authentication in dev: paste a token from `cub auth get-token` when prompted
(kept in sessionStorage). In a same-origin deployment the standard ConfigHub
session-cookie login is used automatically and no token is needed.

Seed demo data and scan it first so the console has something to show:

```bash
../demo-setup.sh        # seeds the fleet, loads the cvedb, scans + writes back
```

Without a scan, workloads show as **unscanned**; run
`../scanner/secscan scan-fleet --space "sec-demo-*" --write-back`.

## Build & checks

```bash
npm run lint         # tsc --noEmit
npm run build        # typecheck + production bundle in dist/
```

## SDK client

`src/sdk/confighubapi.gen.ts` and `src/sdk/validation.gen.ts` are vendored from
the [ConfigHub SDK](https://github.com/confighub/sdk) (MIT); refresh with
`npm run vendor-sdk`. `src/sdk/confighubapi.ts` is a deliberate fork of the
SDK's base client — see the header comment there and `src/sdk/VENDORED_FROM.md`.
