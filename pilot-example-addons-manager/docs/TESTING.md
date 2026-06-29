# Testing

Run all checks:

```bash
npm run verify
```

Run the test suite only:

```bash
npm test
```

The tests use bundled fixture data and do not need ConfigHub credentials.

The verifier also checks that:

- no generated-tool wording appears in the app files;
- no removed runtime files remain;
- the server can start;
- the browser shell loads;
- fixture inventory is available;
- mutation routes stay blocked.
