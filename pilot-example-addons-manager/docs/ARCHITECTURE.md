# Architecture

The app is a small standalone web application:

- The browser UI lives in `public/`.
- The local API server lives in `src/`.
- Fixture data lives in `fixtures/`.
- Tests live in `tests/`.

The app has two data paths:

- `fixture`: read bundled sample data from the local server.
- `browser OAuth`: sign in through the registered browser client and call the
  ConfigHub API directly from the browser.

The local server only serves static assets, fixture endpoints, and app
configuration. It does not proxy live ConfigHub API reads.

## Workflow Shape

The app follows the same operator path in live and fixture mode:

1. Map add-ons across Variants.
2. Preview the current Unit configuration.
3. Prepare an approval scope.
4. Keep apply blocked until a governed write path is connected.
5. Show proof tabs and a receipt preview.

The blocked state is intentional. A sample that cannot prove controller and
runtime delivery must not present a live rollout as complete.
