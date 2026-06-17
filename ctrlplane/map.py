#!/usr/bin/env python3
"""Map a Ctrlplane "System" bundle onto a ConfigHub governed-app plan.

READ-ONLY. This never touches ConfigHub, a cluster, or any live infra. It reads
a Ctrlplane declarative bundle (System / Deployment / Environment / Resource /
JobAgent / Policy) and emits a proposed ConfigHub plan:

    System       -> app (naming root)
    Environment  -> Space (one per environment)
    Deployment   -> Unit (Deployment x Environment)
    Resource     -> Target (bound to the Space whose env selector matches it)
    JobAgent     -> delivery strategy (e.g. argocd -> confighub-oci-argo)
    Policy       -> ConfigHub approval gate / verification note

Modes:
    --mode explain        human-readable plan (default)
    --mode json           machine-readable View Packet (stable fields)
    --mode cub-commands   the `cub` commands that WOULD create the plan (printed)

Dependency: PyYAML (`pip install pyyaml`). The script exits with a clear message
if it is missing.
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path

try:
    import yaml
except ImportError:  # pragma: no cover - dependency guard
    sys.stderr.write(
        "error: PyYAML is required. Install it with `pip install pyyaml`.\n"
    )
    sys.exit(2)


# --------------------------------------------------------------------------- #
# helpers
# --------------------------------------------------------------------------- #
def slugify(value: str) -> str:
    value = (value or "").strip().lower()
    value = re.sub(r"[^a-z0-9]+", "-", value)
    return value.strip("-")


# Minimal evaluator for the Ctrlplane resourceSelector subset we support:
#   resource.metadata["key"] == "value"
#   resource.metadata['key'] == 'value'
_SELECTOR_RE = re.compile(
    r"""resource\.metadata\[\s*['"](?P<key>[^'"]+)['"]\s*\]\s*==\s*['"](?P<val>[^'"]+)['"]"""
)


def selector_matches(selector: str, resource_metadata: dict) -> bool | None:
    """Return True/False if we can evaluate the selector, else None (unknown)."""
    if not selector:
        return None
    m = _SELECTOR_RE.search(selector)
    if not m:
        return None
    return str(resource_metadata.get(m.group("key"))) == m.group("val")


# JobAgent type -> ConfigHub delivery strategy recommendation.
_STRATEGY_BY_AGENT = {
    "argocd": "confighub-oci-argo",
    "argo": "confighub-oci-argo",
    "flux": "confighub-oci-flux",
    "kubernetes": "confighub-direct-apply",
    "github": "ci-dispatch (ConfigHub OCI->Argo recommended instead)",
    "terraform-cloud": "out-of-band (infra provisioning, not app delivery)",
}


def load_bundle(paths: list[Path]) -> list[dict]:
    docs: list[dict] = []
    for path in paths:
        text = path.read_text()
        for doc in yaml.safe_load_all(text):
            if isinstance(doc, dict) and doc.get("type"):
                docs.append(doc)
    return docs


def collect_sources(source: str) -> list[Path]:
    p = Path(source)
    if p.is_dir():
        return sorted(
            list(p.glob("*.yaml")) + list(p.glob("*.yml"))
        )
    if p.is_file():
        return [p]
    raise FileNotFoundError(f"no such file or directory: {source}")


# --------------------------------------------------------------------------- #
# core mapping
# --------------------------------------------------------------------------- #
def build_plan(docs: list[dict]) -> dict:
    systems = [d for d in docs if d.get("type") == "System"]
    deployments = [d for d in docs if d.get("type") == "Deployment"]
    environments = [d for d in docs if d.get("type") == "Environment"]
    resources = [d for d in docs if d.get("type") == "Resource"]
    job_agents = [d for d in docs if d.get("type") == "JobAgent"]
    policies = [d for d in docs if d.get("type") == "Policy"]

    warnings: list[str] = []
    if not systems:
        warnings.append("no System document found; using 'app' as the naming root")
    if not deployments:
        warnings.append("no Deployment document found; no Units can be proposed")
    if not environments:
        warnings.append("no Environment document found; no Spaces can be proposed")

    app_name = systems[0]["name"] if systems else "app"
    app_slug = slugify(app_name)

    # delivery strategy from the (first) job agent
    if job_agents:
        agent_type = (job_agents[0].get("agent") or {}).get("type", "")
        delivery_strategy = _STRATEGY_BY_AGENT.get(
            agent_type, f"unknown agent type '{agent_type}' (manual mapping needed)"
        )
    else:
        delivery_strategy = "unspecified (no JobAgent; choose confighub-oci-argo)"
        warnings.append("no JobAgent document; defaulting delivery to manual choice")

    # Variant model: a base/upstream Space holds the shared Deployment
    # definition; each Environment gets a downstream variant linked via
    # --upstream-unit. Promotion = `cub unit update <variant> --upgrade`.
    base_space_slug = f"{app_slug}-base"
    spaces = []
    targets = []
    units = []
    release_targets = []
    unbound_resources = list(resources)

    # base space + base (upstream) units, one per Deployment
    spaces.append(
        {
            "slug": base_space_slug,
            "role": "base",
            "from_environment": None,
            "requires_approval": False,
            "resource_count": 0,
        }
    )
    base_unit_ref: dict[str, str] = {}
    for dep in deployments:
        dep_slug = slugify(dep["name"])
        ref = f"{base_space_slug}/{dep_slug}"
        base_unit_ref[dep_slug] = ref
        units.append(
            {
                "ref": ref,
                "slug": dep_slug,
                "space": base_space_slug,
                "role": "base",
                "upstream": None,
                "from_deployment": dep["name"],
                "image": (dep.get("config") or {}).get("image"),
            }
        )

    for env in environments:
        env_name = env["name"]
        env_slug = slugify(env_name)
        space_slug = f"{app_slug}-{env_slug}"
        requires_approval = str(
            (env.get("metadata") or {}).get("requires-approval", "")
        ).lower() == "true"

        # match resources to this environment via the selector
        env_resources = []
        for res in resources:
            verdict = selector_matches(
                env.get("resourceSelector", ""), res.get("metadata") or {}
            )
            if verdict is True:
                env_resources.append(res)
                if res in unbound_resources:
                    unbound_resources.remove(res)
            elif verdict is None:
                warnings.append(
                    f"could not evaluate selector for environment '{env_name}'; "
                    f"resources need manual binding"
                )

        spaces.append(
            {
                "slug": space_slug,
                "role": "environment",
                "from_environment": env_name,
                "requires_approval": requires_approval,
                "resource_count": len(env_resources),
            }
        )

        # targets: one per resource in this environment's space
        for res in env_resources:
            target_slug = slugify(res.get("name") or res.get("identifier"))
            targets.append(
                {
                    "slug": target_slug,
                    "space": space_slug,
                    "from_resource": res.get("identifier"),
                    "kind": res.get("kind"),
                    "region": (res.get("metadata") or {}).get("region"),
                }
            )

        # units: one variant per deployment in this environment's space,
        # linked to the base unit upstream
        for dep in deployments:
            dep_slug = slugify(dep["name"])
            unit_ref = f"{space_slug}/{dep_slug}"
            units.append(
                {
                    "ref": unit_ref,
                    "slug": dep_slug,
                    "space": space_slug,
                    "role": "variant",
                    "upstream": base_unit_ref.get(dep_slug),
                    "from_deployment": dep["name"],
                    "image": (dep.get("config") or {}).get("image"),
                }
            )
            # release targets = Deployment x Environment x Resource
            for res in env_resources:
                release_targets.append(
                    f"{dep_slug} -> {space_slug} -> {slugify(res.get('name'))}"
                )

    for res in unbound_resources:
        warnings.append(
            f"resource '{res.get('identifier')}' matched no environment selector; "
            f"needs manual Target binding"
        )

    # policies -> gates
    gates = []
    for pol in policies:
        for rule in pol.get("rules", []):
            if "approval" in rule:
                gates.append(
                    {
                        "from_policy": pol["name"],
                        "confighub": "approval gate (ApplyGate / Trigger)",
                        "detail": f"required approvals: {rule['approval'].get('required', 1)}",
                    }
                )
            if "verification" in rule:
                v = rule["verification"]
                gates.append(
                    {
                        "from_policy": pol["name"],
                        "confighub": "post-apply verification (out of ConfigHub core; "
                        "wire via cub-scout receipt + external check)",
                        "detail": f"{v.get('provider')}: {v.get('query')}",
                    }
                )

    # Structural seams that are TRUE of the model, independent of this bundle.
    mapping_notes = [
        "Variant model: the base Space holds the shared Deployment definition; "
        "each Environment is a downstream variant linked via --upstream-unit. "
        "Supply the rendered manifest ONCE on the base Unit (via import-from-helm "
        "/ import-from-kustomize or a YAML file); variants inherit it and carry "
        "only their environment-local overrides. A Ctrlplane Deployment carries "
        "an image reference, not a manifest, so the base manifest is yours to add.",
        "Promotion = `cub unit update --space <env> <unit> --upgrade` (pull from "
        "the upstream base). This is the ConfigHub-native parallel to Ctrlplane "
        "promoting a Version across environments. CAVEAT: --upgrade can silently "
        "under-propagate list/nested fields when leaf vs base list shapes differ "
        "— diff the variant after upgrading; do not assume it landed.",
        "Ctrlplane Policy 'verification' (Datadog/Prometheus/HTTP) has no "
        "ConfigHub-core equivalent. Wire it as a post-apply external check that "
        "feeds the promotion gate (cub-scout receipt + the metric check).",
        "Ctrlplane environment progression and gradual-rollout TIMING is "
        "orchestration ConfigHub does not schedule. Keep Ctrlplane (or another "
        "promotion driver) for sequencing; ConfigHub governs and proves each step.",
    ]

    return {
        "example_name": "ctrlplane-on-confighub",
        "source": "Ctrlplane System bundle",
        "mutates": False,
        "mutates_confighub": False,
        "mutates_live_infra": False,
        "app": app_slug,
        "app_display_name": app_name,
        "delivery_strategy": delivery_strategy,
        "spaces": [s["slug"] for s in spaces],
        "units": [u["ref"] for u in units],
        "targets": [f"{t['space']}/{t['slug']}" for t in targets],
        "plan": {
            "spaces": spaces,
            "units": units,
            "targets": targets,
            "gates": gates,
            "release_targets_preview": release_targets,
        },
        "mapping_notes": mapping_notes,
        "warnings": sorted(set(warnings)),
    }


# --------------------------------------------------------------------------- #
# renderers
# --------------------------------------------------------------------------- #
def render_explain(plan: dict) -> str:
    lines = []
    lines.append("Ctrlplane -> ConfigHub mapping plan (READ-ONLY, nothing is created)")
    lines.append("=" * 66)
    lines.append("")
    lines.append(f"App (from System):   {plan['app_display_name']}  ->  {plan['app']}")
    lines.append(f"Delivery strategy:   {plan['delivery_strategy']}")
    lines.append("")
    lines.append("Conceptual model")
    lines.append("----------------")
    lines.append("  Ctrlplane                 ConfigHub")
    lines.append("  ---------                 ---------")
    lines.append("  System            ->      app (naming root)")
    lines.append("  Deployment        ->      base Unit (upstream, in <app>-base)")
    lines.append("  Environment       ->      Space + downstream Unit variant")
    lines.append("  Resource          ->      Target")
    lines.append("  JobAgent          ->      delivery strategy")
    lines.append("  Policy            ->      approval gate / verification")
    lines.append("  (promotion)       ->      cub unit update <variant> --upgrade")
    lines.append("")
    lines.append(f"Spaces to create ({len(plan['plan']['spaces'])})")
    lines.append("-" * 20)
    for s in plan["plan"]["spaces"]:
        if s["role"] == "base":
            lines.append(f"  - {s['slug']}  (base / upstream — shared definition)")
        else:
            approval = "  [requires approval]" if s["requires_approval"] else ""
            lines.append(
                f"  - {s['slug']}  (from env '{s['from_environment']}', "
                f"{s['resource_count']} target(s)){approval}"
            )
    lines.append("")
    lines.append(f"Units to create ({len(plan['plan']['units'])})")
    lines.append("-" * 20)
    for u in plan["plan"]["units"]:
        img = f"  image={u['image']}" if u.get("image") else ""
        if u["role"] == "base":
            lines.append(f"  - {u['ref']}  [base]{img}")
        else:
            lines.append(f"  - {u['ref']}  [variant -> upstream {u['upstream']}]")
    lines.append("")
    lines.append(f"Targets to bind ({len(plan['plan']['targets'])})")
    lines.append("-" * 20)
    for t in plan["plan"]["targets"]:
        region = f"  ({t['region']})" if t.get("region") else ""
        lines.append(f"  - {t['space']}/{t['slug']}  [{t['kind']}]{region}")
    lines.append("")
    lines.append(f"Gates (from Policies) ({len(plan['plan']['gates'])})")
    lines.append("-" * 20)
    for g in plan["plan"]["gates"]:
        lines.append(f"  - {g['from_policy']}: {g['confighub']}")
        lines.append(f"      {g['detail']}")
    lines.append("")
    lines.append("Mapping seams (structural — read these)")
    lines.append("-" * 38)
    for n in plan.get("mapping_notes", []):
        lines.append(f"  * {n}")
    if plan["warnings"]:
        lines.append("")
        lines.append("Warnings / manual follow-ups")
        lines.append("-" * 28)
        for w in plan["warnings"]:
            lines.append(f"  ! {w}")
    lines.append("")
    lines.append("This plan is READ-ONLY. To see the cub commands it implies:")
    lines.append("  ./setup.sh --cub-commands")
    return "\n".join(lines)


def render_cub_commands(plan: dict) -> str:
    lines = []
    lines.append("#!/usr/bin/env bash")
    lines.append("# Generated by map.py from a Ctrlplane System bundle.")
    lines.append("# These are the ConfigHub commands the plan implies. Review before running.")
    lines.append("# Nothing here runs automatically; ./setup.sh --apply executes them.")
    lines.append("set -euo pipefail")
    lines.append("")
    for s in plan["plan"]["spaces"]:
        suffix = "  # base / upstream" if s["role"] == "base" else ""
        lines.append(f"cub space create {s['slug']}{suffix}")
    lines.append("")
    lines.append("# base (upstream) units — supply the rendered manifest ONCE here")
    for u in plan["plan"]["units"]:
        if u["role"] != "base":
            continue
        lines.append(
            f"cub unit create --space {u['space']} {u['slug']} "
            f"path/to/{u['slug']}.manifest.yaml "
            f"# from Deployment '{u['from_deployment']}' (image {u.get('image')}); "
            f"Ctrlplane carries only an image ref"
        )
    lines.append("")
    lines.append("# environment variants — cloned from the upstream base, inherit its config")
    for u in plan["plan"]["units"]:
        if u["role"] != "variant":
            continue
        lines.append(
            f"cub unit create --space {u['space']} {u['slug']} "
            f"--upstream-unit {u['upstream']} "
            f"# variant of {u['upstream']}; add only env-local overrides"
        )
    lines.append("")
    for t in plan["plan"]["targets"]:
        lines.append(
            f"# target {t['space']}/{t['slug']} [{t['kind']}] -> bind via "
            f"`cub target create` once the worker/cluster is connected"
        )
    lines.append("")
    lines.append("# promotion: pull base changes down into a variant (the Ctrlplane 'promote' step)")
    for u in plan["plan"]["units"]:
        if u["role"] != "variant":
            continue
        lines.append(
            f"# cub unit update --space {u['space']} {u['slug']} --upgrade  "
            f"# then DIFF — --upgrade can under-propagate list/nested fields"
        )
    lines.append("")
    lines.append(f"# delivery strategy: {plan['delivery_strategy']}")
    for g in plan["plan"]["gates"]:
        lines.append(f"# gate from policy '{g['from_policy']}': {g['confighub']} ({g['detail']})")
    return "\n".join(lines)


# --------------------------------------------------------------------------- #
# main
# --------------------------------------------------------------------------- #
def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--source",
        default=str(Path(__file__).resolve().parent / "systems"),
        help="Ctrlplane System file or directory (default: ./systems)",
    )
    parser.add_argument(
        "--mode",
        choices=["explain", "json", "cub-commands"],
        default="explain",
    )
    args = parser.parse_args(argv)

    try:
        sources = collect_sources(args.source)
    except FileNotFoundError as exc:
        sys.stderr.write(f"error: {exc}\n")
        return 2
    if not sources:
        sys.stderr.write(f"error: no .yaml/.yml files under {args.source}\n")
        return 2

    docs = load_bundle(sources)
    plan = build_plan(docs)

    if args.mode == "json":
        print(json.dumps(plan, indent=2))
    elif args.mode == "cub-commands":
        print(render_cub_commands(plan))
    else:
        print(render_explain(plan))
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
