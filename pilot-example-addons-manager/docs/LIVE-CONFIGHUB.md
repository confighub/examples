# Live ConfigHub Mode

Live mode uses the local `cub` session for read-only calls.

Check access:

```bash
cub auth status
cub space list -o json
```

Start with live reads required:

```bash
DATA_MODE=live npm start
```

Start with live reads preferred and fixture fallback enabled:

```bash
DATA_MODE=auto npm start
```

The app reads:

- `/api/me`
- spaces whose names start with `helm-`
- Units in those spaces
- Unit data
- revision history

The app does not write to ConfigHub. POST requests to approval and apply routes
return `405` with a plain explanation.
