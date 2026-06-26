# Vendored SDK client

`confighubapi.gen.ts` and `validation.gen.ts` are vendored verbatim from the
ConfigHub SDK (MIT):

- Repo: https://github.com/confighub/sdk
- Path: `core/openapi/rtkqueryclient/`
- Vendored: 2026-06-26 (ref: main)

Do not edit them; refresh with `npm run vendor-sdk` (see `scripts/vendor-sdk.sh`).

`confighubapi.ts` is intentionally **forked** (not vendored): it is the base
client the generated file injects endpoints into, adapted for this app's auth
modes and routes. Review it against the upstream file when re-vendoring.
