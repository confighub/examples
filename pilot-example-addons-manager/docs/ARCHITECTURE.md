# Architecture

The app is a small standalone web application:

- The browser UI lives in `public/`.
- The local API server lives in `src/`.
- Fixture data lives in `fixtures/`.
- Tests live in `tests/`.

The server has two data paths:

- `fixture`: read bundled sample data.
- `live`: call `cub` read commands using the user's local ConfigHub session.

The default `auto` mode tries live reads and falls back to fixtures when live
credentials or command access are unavailable.

## Workflow Shape

The app follows the same operator path in live and fixture mode:

1. Map add-ons across Variants.
2. Preview the current Unit configuration.
3. Prepare an approval scope.
4. Keep apply blocked until a governed write path is connected.
5. Show proof tabs and a receipt preview.

The blocked state is intentional. A sample that cannot prove controller and
runtime delivery must not present a live rollout as complete.
