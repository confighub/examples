# Initiatives: The AI ↔ GUI Bridge

This guide shows how an AI agent and the ConfigHub GUI work together on Initiatives — the same data, the same entities, two different ways to interact with them. An initiative created by the AI appears on the GUI kanban. An initiative created in the GUI can be picked up and executed by the AI. Neither surface is privileged.

## What you'll need

- A ConfigHub login: `cub auth login`
- This example set up: `./setup.sh` (creates 18 units + 5 initiatives)
- The `confighub-ai-demo` repo cloned alongside this one (for the AI skills)

## What an Initiative is

An Initiative is a tracked compliance or remediation campaign across multiple Kubernetes units. In ConfigHub, it's built from three entities:

- **A View** — the campaign itself (name, priority, status, deadline, check results)
- **A Filter** — which units the campaign applies to (by app, owner, label, or query)
- **A Trigger** — the policy check to run (e.g., "every Deployment must have a liveness probe")

The same three entities power both the GUI kanban board and the AI skill chain. There's no separate "AI initiatives" database — just initiatives.

## The 5 demo initiatives

After running `./setup.sh`, you have:

| Initiative | Priority | Status | What it checks |
|---|---|---|---|
| Liveness and Readiness Probes | HIGH | in_progress | Every container has health checks |
| Image Registry Restriction | HIGH | in_progress | Images come from approved registries |
| Run-As-NonRoot Enforcement | MEDIUM | in_progress | Containers don't run as root |
| Resource Limits Enforcement | MEDIUM | draft | Every container has memory/CPU limits |
| Disallow Host Ports | LOW | completed | No containers bind host ports |

Each applies to a subset of the 18 units (filtered by App or AppOwner label).

---

## Direction 1: AI creates and runs an initiative

You're in a Claude session. You say:

```
"Create an initiative to enforce resource limits on all website units"
```

Here's what happens step by step:

### Step 1: AI creates the Filter

```
/cub-fix "create a filter for website units"
  → cub filter create --space initiatives-demo website-limits-filter Unit \
      --where "Labels.App = 'website'"
  → Filter created: website-limits-filter
  → matches: website-web, website-cms, website-postgres
```

### Step 2: AI creates the Initiative (View)

```
  → cub view create --space initiatives-demo \
      --label initiative=true \
      --label initiative-priority=HIGH \
      --label initiative-status=in_progress \
      --annotation initiative-description="Enforce resource limits on all website containers" \
      --annotation initiative-deadline=2026-04-30 \
      "enforce-website-limits" "website-limits-filter"
  → Initiative created: enforce-website-limits
  → 🔗 Trust: https://hub.confighub.com/views/enforce-website-limits
```

At this point, the initiative card appears on the GUI kanban. Anyone with space access can see it — the AI session doesn't need to be open.

### Step 3: AI scans the matched units

```
  → cub unit list --space initiatives-demo --filter website-limits-filter
  → 3 units matched: website-web, website-cms, website-postgres

  → for each unit: cub-scan --confighub --space initiatives-demo --unit <unit>
  → website-web: 2 findings (missing resources.requests.memory, missing resources.limits.cpu)
  → website-cms: 1 finding (missing resources.limits.cpu)
  → website-postgres: 0 findings ✓
```

### Step 4: AI fixes the violations

```
  → cub run set-resource-limits --space initiatives-demo --unit website-web \
      --container-name web --cpu-limit 500m --memory-limit 256Mi
  → CH-WRITE: resource limits set on website-web (rev 2 → 3)

  → cub run set-resource-limits --space initiatives-demo --unit website-cms \
      --container-name cms --cpu-limit 500m
  → CH-WRITE: resource limits set on website-cms (rev 2 → 3)

  → approve + apply each unit
```

### Step 5: AI updates the initiative's check summary

```
  → cub view update --space initiatives-demo enforce-website-limits \
      --annotation initiative-check-summary='{"passing":3,"failing":0,"total":3,"checkedAt":"2026-04-17T..."}'
  → cub view update --space initiatives-demo enforce-website-limits \
      --label initiative-status=completed \
      --annotation initiative-completed-at=2026-04-17T12:00:00Z
  → Initiative completed ✓
  → 🔗 Trust: https://hub.confighub.com/views/enforce-website-limits
```

The GUI kanban card now shows "completed" with a green badge. The check summary shows 3/3 passing. The user clicks the trust URL and sees the full initiative in the browser — without ever having opened the GUI during the AI session.

### Step 6: AI offers to share

```
/cub-share
  → *ConfigHub Governed Session* — `20260417T120000Z-website-limits`
  → Last governed write: set-resource-limits on website-cms
  → Events: 15 | Pending approvals: 0 | Approved: 3
  → Trust: https://hub.confighub.com/views/enforce-website-limits
  → Approve: ./cub-approve
```

---

## Direction 2: GUI initiative picked up by the AI

You (or a colleague) created an initiative in the GUI. Now you want the AI to run it.

### In the GUI

1. Open ConfigHub → Initiatives
2. See "Image Registry Restriction" (HIGH, in_progress)
3. Copy the URL: `https://hub.confighub.com/views/image-registry-restriction`

### In the AI

Paste the URL into the chat:

```
https://hub.confighub.com/views/image-registry-restriction
```

The AI recognises it's a ConfigHub URL and acts on it:

```
  → This is initiative "Image Registry Restriction" in space initiatives-demo
  → Priority: HIGH | Status: in_progress
  → Description: All container images must come from approved registries
  → Filter: AppOwner = Product (matches 7 units)
  → Check summary: 4 passing, 3 failing

  I can:
  - Run the image registry check on all matched units
  - Fix the 3 failing units (change images to approved registry)
  - Update the initiative progress
  - Show me more detail on what's failing

  What would you like to do?
```

You say: "fix the 3 failing units"

```
  → Checking which units fail the image registry policy...
  → eshop-postgres: image postgres:15 (not from approved registry)
  → eshop-redis: image redis:7 (not from approved registry)
  → docs-search: image elasticsearch:8 (not from approved registry)

  → Step 1: Fix eshop-postgres
  → cub run set-container-image --space initiatives-demo --unit eshop-postgres \
      --container-name postgres --container-image registry.confighub.io/postgres:15
  → CH-WRITE: image changed on eshop-postgres (rev 2 → 3)
  → 🔗 Trust: https://hub.confighub.com/spaces/initiatives-demo/units/eshop-postgres

  Step 1 complete. Continue to step 2? (y/n)
```

You say yes. The AI fixes all 3, updates the initiative, and the GUI kanban card reflects the new state — 7/7 passing.

---

## How it works under the hood

### The View is the integration point

The AI and the GUI both read and write the same View entity. The View carries:

```
Labels:
  initiative: "true"
  initiative-priority: "HIGH"
  initiative-status: "in_progress" → "completed"

Annotations:
  initiative-description: "All container images must come from approved registries"
  initiative-deadline: "2026-04-30"
  initiative-check-summary: '{"passing":7,"failing":0,"total":7}'
  initiative-completed-at: "2026-04-17T12:00:00Z"
```

The AI writes these with `cub view create` and `cub view update`. The GUI reads them and renders the kanban card. Same data, same entity, no synchronisation needed.

### The Filter selects units

Each initiative's Filter is a `--where` clause that selects units by label:

```bash
cub unit list --space initiatives-demo --filter image-registry-restriction
```

The AI uses the same filter to find which units to check and fix. The GUI uses it to show the matched units on the initiative detail page.

### The Trigger runs the policy

When a worker is connected, the Trigger runs `vet-kyverno` against the matched units. The AI can run the same check directly:

```bash
cub run vet-celexpr --space initiatives-demo --unit <unit> --validation-expr '<policy>'
```

Or use `cub-scan` for the authority-grade catalog scan.

---

## Commands quick reference

```
# List all initiatives
cub view list --space initiatives-demo --where "Labels.initiative = 'true'"

# Get initiative detail
cub view get --space initiatives-demo --json <initiative-slug>

# List matched units
cub unit list --space initiatives-demo --filter <filter-slug>

# Create an initiative
cub view create --space initiatives-demo \
  --label initiative=true \
  --label initiative-priority=HIGH \
  --label initiative-status=in_progress \
  --annotation initiative-description="..." \
  "<slug>" "<filter-slug>"

# Update initiative progress
cub view update --space initiatives-demo <slug> \
  --annotation initiative-check-summary='{"passing":N,"failing":M,"total":T}'

# Complete an initiative
cub view update --space initiatives-demo <slug> \
  --label initiative-status=completed \
  --annotation initiative-completed-at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
```

## Try it yourself

1. Set up the demo: `./setup.sh`
2. Open Claude in the `confighub-ai-demo` repo: `cd ../confighub-ai-demo && claude`
3. Try Direction 1: `"Create an initiative to pin all :latest image tags in initiatives-demo"`
4. Try Direction 2: paste `https://hub.confighub.com/views/liveness-and-readiness-probes`
5. Try the GUI sim: `./scripts/cub-gui-sim session`
6. Clean up: `./cleanup.sh`

## Related

- [README.md](./README.md) — setup and data model
- [AI_START_HERE.md](./AI_START_HERE.md) — CLI-only demo stages
- [confighub-ai-demo/docs/AI_AND_GUI_INTEGRATION.md](../../confighub-ai-demo/docs/AI_AND_GUI_INTEGRATION.md) — the architecture
- [confighub-ai-demo/skills/ai-gui-bridge/SKILL.md](../../confighub-ai-demo/skills/ai-gui-bridge/SKILL.md) — the skill
- [confighub-ai-demo/skills/create-initiative/SKILL.md](../../confighub-ai-demo/skills/create-initiative/SKILL.md) — the initiative skill
