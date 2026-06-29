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

Then open `http://localhost:5173/`, choose `Browser OAuth`, and click `Sign in`.

After sign-in, the browser reads:

- `/api/me`
- `/api/space`
- `/api/space/{space_id}/unit`
- `/api/space/{space_id}/unit/{unit_id}/data`
- `/api/space/{space_id}/unit/{unit_id}/revision`

The app does not write to ConfigHub. Apply remains disabled and local POST
requests to approval/apply routes return `405` with a plain explanation.
