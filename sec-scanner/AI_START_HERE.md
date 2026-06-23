# AI Guide: sec-scanner

Container image CVEs managed as data: a unified CVE database (GitHub Advisory DB
+ CVE List V5 + OSV, normalized), a custom scanner that digs into images, scan
verdicts written back onto ConfigHub Units, and Apply Gates that block the
vulnerable.

## CRITICAL: Demo Pacing

Run ONE stage at a time. After each stage, STOP and wait for the human before
continuing. Do not run ahead, even if the next command is obvious. The human
may want to inspect the GUI or the database between stages.

## Suggested Prompt

> Walk me through the sec-scanner example stage by stage. Pause after each
> stage so I can look around. I'm authenticated with cub. (No Docker needed —
> the scanner pulls image layers from the registry directly.)

## Stage 1: Stand up the CVE database

```bash
./cvedb/build.sh                 # builds secscan, creates cvedb/cve.db, imports OSV CVE data
# (optional, if you have the sqlite3 CLI) peek at what landed:
sqlite3 cvedb/cve.db "SELECT severity, count(*) FROM advisory GROUP BY severity ORDER BY 2 DESC;"
```

Expected: a few thousand advisories across CRITICAL/HIGH/MEDIUM/LOW. This is
real data — the three GitHub/OSV sources normalized to one schema by
`secscan import`, in a single SQLite file accessed through a pure-Go driver (no
server, no Python, no `sqlite3` binary). Set `SEC_SCANNER_OFFLINE=1` to load
curated fixtures instead (no downloads).

**PAUSE.** Wait for the human.

## Stage 2: Dig into one image

```bash
# secscan was already built by build.sh in Stage 1 (it is the importer too)
./scanner/secscan scan nginx:1.16-alpine
./scanner/secscan scan nginx:1.27-alpine    # a current image, for contrast
```

Expected: the old image reports CRITICAL with a table of `(package, version,
fixed, CVE)`; the current image reports NONE. The scanner pulled the image,
read its apk database out of the exported filesystem, and matched each package
against the cvedb — no Trivy involved. `--json` emits the structured report.

**PAUSE.** Wait for the human.

## Stage 3: Seed the fleet and gate it

```bash
./demo-setup.sh --explain        # preview first — mutates nothing
./demo-setup.sh                  # seed Spaces/Units, then scan + write back + gate
./demo-verify.sh                 # layout + gate matrix + scan write-back + cvedb
```

Expected: `All checks passed.` Re-running demo-setup.sh is safe; it skips
existing entities and re-scans.

GUI gap: there is no single fleet-level view of "which images run where and
what's vulnerable" — that is what a sec-scanner console webapp would add on top
of this layout (as rbac-manager's webapp does for RBAC).

GUI feature ask: a fleet security dashboard that summarizes, across a
label-selected set of Spaces, each Unit's image, its scanned max severity, the
CVE count, and whether it is currently gated — with a drill-down to the
per-package findings the scanner recorded.

**PAUSE.** Wait for the human.

## Stage 4: The fleet image inventory

The complaint this answers: scanning one image is easy; knowing which clusters
run a vulnerable image is the hard part.

```bash
# inventory/scan-fleet talk to the ConfigHub REST API directly (like the web
# app), so give the scanner a token first:
export CONFIGHUB_URL="https://hub.confighub.com" CONFIGHUB_TOKEN="$(cub auth get-token)"

# every image, every cluster, one query
./scanner/secscan inventory --space "sec-demo-*"

# which Units did the scanner mark CRITICAL? (verdict stored as data)
cub unit list --space "*" --where "Annotations.'sec-scanner.confighub.com/max-severity' = 'CRITICAL'"
```

Expected: the inventory lists each Unit's image; the query returns the dev
`legacy-frontend` and `legacy-api` Units the scanner flagged.

**PAUSE.** Wait for the human.

## Stage 5: Guardrails are gates, not advice

```bash
# the years-old image is blocked from ever being applied
cub unit get legacy-frontend --space sec-demo-dev -o jq=".Unit.ApplyGates"

# so is the :latest image (static check, no scan needed)
cub unit get unpinned-web --space sec-demo-dev -o jq=".Unit.ApplyGates"

# prod changes carry an approval gate out of the box
cub unit get frontend --space sec-demo-prod -o jq=".Unit.ApplyGates"
```

Expected gates: `sec-demo-policy/no-critical-cves/vet-celexpr`,
`sec-demo-policy/no-latest-tag/vet-celexpr`, and
`sec-demo-policy/require-approval/vet-approvedby` respectively. The clean
workloads on current images carry NO gate.

**PAUSE.** Wait for the human.

## Stage 6: Fix forward, re-scan, gate clears

```bash
# upgrade the legacy frontend to a current image (server-side, comment-preserving)
cub function do --space sec-demo-dev --unit legacy-frontend \
  --change-desc "Upgrade legacy-frontend off the vulnerable image" \
  -- set-image nginx nginx:1.27-alpine

# re-scan + write back; the CRITICAL annotation flips to NONE and the gate lifts
./scanner/secscan scan-fleet --space "sec-demo-dev" --write-back
cub unit get legacy-frontend --space sec-demo-dev -o jq=".Unit.ApplyGates"
```

Expected: after the upgrade and re-scan, `legacy-frontend` is no longer gated by
no-critical-cves — the fix and the verdict are the same versioned object.

**PAUSE.** Wait for the human.

## Stage 7: A new CVE drops — know what to re-scan

New CVEs are published constantly. Each scan records the CVE DB version it ran
against (`cvedb-version`), so after you re-import the database you can tell
exactly which Units are judged by a stale snapshot — without blindly re-scanning
the whole fleet.

```bash
# re-import bumps the DB version (a new import_log row)
./scanner/secscan import --osv-zip Alpine:v3.9

# every Unit scanned before the re-import is now flagged
./scanner/secscan stale --space "sec-demo-*"

# bring them current again (also refreshes the cvedb-status Unit)
./scanner/secscan scan-fleet --space "sec-demo-*" --write-back --status-space sec-demo-policy
./scanner/secscan stale --space "sec-demo-*"     # clean
```

Expected: after the re-import every workload reports "scanned against older DB";
after the re-scan, `stale` is clean. The console shows the same via a **stale**
count on the Dashboard and a `stale` chip in the Fleet view (it reads the
`cvedb-status` Unit, so it needs no database access of its own).

**PAUSE.** Wait for the human.

## Stage 8: The console (optional)

A static React SPA renders all of the above from the ConfigHub API — no extra
backend.

```bash
cd app && npm install
CONFIGHUB_URL=http://localhost:9090 npm run dev    # then open http://localhost:5180
```

Paste a token from `cub auth get-token` when prompted (dev mode). The Dashboard
shows the fleet severity rollup; Fleet is the image inventory; Findings lists
every CVE; a Unit page offers an in-app **Upgrade image** action (the same
server-side `yq-i` mutation as Stage 6). The console reads image refs + the
scanner's verdict annotations — it computes nothing itself.

**PAUSE.** Wait for the human.

## Cleanup

No cleanup script by design (demo data is meant to persist). To tear down:

```bash
for s in sec-demo-dev sec-demo-staging sec-demo-prod sec-demo-base sec-demo-policy; do
  cub space delete "$s" --recursive
done
rm -f cvedb/cve.db                                   # drop the CVE DB (just a file)
```
