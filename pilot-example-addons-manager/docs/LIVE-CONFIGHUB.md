# Live ConfigHub Mode With Browser OAuth

Live mode signs the browser in through a ConfigHub OAuth client and then calls
the ConfigHub API directly from the browser.

Until `cub oauthclient` is released, build `cub` from the ConfigHub PR 4665
feature branch:

```bash
cd /path/to/confighub
git fetch origin pull/4665/head:oauthclient-registration
git checkout oauthclient-registration
make build-cli
```

Register the app against the ConfigHub server:

```bash
bin/cub auth login --server https://pr-4665.testhub.confighub.net
export CONFIGHUB_BASE=https://pr-4665.testhub.confighub.net
export OAUTH_CLIENT_ID=$(bin/cub oauthclient create addon-manager \
  --redirect-uri http://localhost:5173/ -o jq='.ClientID')
```

Start this app:

```bash
CONFIGHUB_BASE=$CONFIGHUB_BASE OAUTH_CLIENT_ID=$OAUTH_CLIENT_ID npm start
```

Check OAuth discovery before calling the browser flow complete:

```bash
CONFIGHUB_BASE=$CONFIGHUB_BASE OAUTH_CLIENT_ID=$OAUTH_CLIENT_ID npm run oauth:smoke
```

Then open `http://localhost:5173/`, choose `Browser OAuth`, and click `Sign in`.

After sign-in, the browser reads:

- `/api/me`
- `/api/space`
- `/api/space/{space_id}/unit`
- `/api/space/{space_id}/unit/{unit_id}/data`
- `/api/space/{space_id}/unit/{unit_id}/revision`

The app does not write to ConfigHub. Apply remains disabled and local POST
requests to approval/apply routes return `405` with a plain explanation.

## Live Binding Gate

Live operation is not complete until the app is bound to real operation objects.
Copy the example binding file:

```bash
cp data/live-bindings.example.json data/live-bindings.json
```

Fill in the ConfigHub object URL, approval object, action endpoint, proof
receipt, and runtime evidence source. Then run:

```bash
npm run binding:check
```

The GUI shows `bindings missing` until that check can pass. The binding file
must also include `action.contract`, which defines the operation name, required
scope fields, required proof layers, and mutation policy. Even after bindings
are present, Apply stays disabled when the action endpoint or runtime evidence
is still marked `blocked:*`.

The current PR-server proof reaches authenticated browser reads, real Variant
inventory, real Unit data, revision history, and approval scope. It does not
claim a live operation until a target, worker, governed action executor,
controller proof, and runtime readback are bound.
