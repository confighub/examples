#!/usr/bin/env python3
"""Read-only Add-on Manager sample app server."""

from __future__ import annotations

import http.server
import json
import os
from pathlib import Path
import subprocess
import urllib.parse
import urllib.request


ROOT = Path(__file__).resolve().parent
CONFIGHUB_BASE = os.environ.get("CONFIGHUB_BASE", "https://hub.confighub.com").rstrip("/")


WORKFLOW = {
    "app": "Add-on Manager",
    "route": "ConfigHub -> OCI -> Argo -> Kubernetes",
    "scope_fields": [
        "org",
        "space",
        "variant",
        "unit",
        "action",
        "revision",
        "target",
        "strategy",
        "addon",
        "version",
    ],
    "controls": [
        {"id": "map", "label": "Map inventory", "purpose": "Find installed add-ons and versions."},
        {"id": "preview", "label": "Preview impact", "purpose": "Read current ConfigHub Unit data before changing it."},
        {"id": "approve", "label": "Approve scope", "purpose": "Require exact scope before any future mutation."},
        {"id": "apply", "label": "Apply rollout", "purpose": "Reserved for a governed ConfigHub action."},
        {"id": "prove", "label": "Prove result", "purpose": "Collect revision, approval, controller, runtime, and receipt proof."},
    ],
    "proof_tabs": ["Revision", "Approval", "Gate", "Controller", "Runtime", "Receipt"],
    "non_claims": [
        "Preview is not a rollout.",
        "A version proposal is not proof that a controller delivered it.",
        "Runtime proof is unavailable until a real cluster read is wired.",
        "This sample blocks mutation requests.",
    ],
}


def run_cub(args: list[str]) -> subprocess.CompletedProcess[str]:
    return subprocess.run(["cub", *args], capture_output=True, text=True, timeout=45)


def cub_json(args: list[str]) -> dict | list:
    proc = run_cub([*args, "-o", "json"])
    if proc.returncode != 0:
        return {"error": (proc.stderr or proc.stdout).strip()[:500], "cmd": "cub " + " ".join(args)}
    try:
        return json.loads(proc.stdout)
    except json.JSONDecodeError:
        return {"error": "cub returned non-JSON output", "raw": proc.stdout[:500]}


def api_me() -> dict:
    proc = run_cub(["auth", "get-token"])
    token = proc.stdout.strip()
    if proc.returncode != 0 or not token:
        return {"error": "cub auth token unavailable", "detail": (proc.stderr or proc.stdout).strip()[:500]}
    request = urllib.request.Request(
        CONFIGHUB_BASE + "/api/me",
        headers={"Authorization": "Bearer " + token},
    )
    with urllib.request.urlopen(request, timeout=20) as response:
        return {"http": response.status, "body": json.loads(response.read().decode("utf-8"))}


class Handler(http.server.SimpleHTTPRequestHandler):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, directory=str(ROOT), **kwargs)

    def log_message(self, *args):
        return

    def send_json(self, payload, code: int = 200) -> None:
        body = json.dumps(payload, sort_keys=True).encode("utf-8")
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        parsed = urllib.parse.urlparse(self.path)
        query = {key: values[0] for key, values in urllib.parse.parse_qs(parsed.query).items()}
        try:
            if parsed.path == "/api/workflow":
                self.send_json(WORKFLOW)
            elif parsed.path == "/api/me":
                self.send_json(api_me())
            elif parsed.path == "/api/spaces":
                self.send_json(cub_json(["space", "list"]))
            elif parsed.path == "/api/units":
                self.send_json(cub_json(["unit", "list", "--space", query.get("space", "")]))
            elif parsed.path == "/api/unit":
                self.send_json(cub_json(["unit", "get", query.get("unit", ""), "--space", query.get("space", "")]))
            elif parsed.path == "/api/revisions":
                self.send_json(cub_json(["revision", "list", "--space", query.get("space", ""), query.get("unit", "")]))
            elif parsed.path == "/api/unitdata":
                proc = run_cub(["unit", "data", "--space", query.get("space", ""), query.get("unit", "")])
                if proc.returncode == 0:
                    self.send_json({"text": proc.stdout})
                else:
                    self.send_json({"error": (proc.stderr or proc.stdout).strip()[:500]}, 502)
            else:
                super().do_GET()
        except Exception as exc:
            self.send_json({"error": str(exc)[:500]}, 502)

    def do_POST(self):
        self.send_json(
            {"error": "read-only sample app: mutation requires a governed approval and apply path"},
            405,
        )


def main() -> int:
    port = int(os.environ.get("PORT", "5173"))
    print(f"Add-on Manager sample app: http://localhost:{port}")
    print("read-only server; live reads use local cub auth when available")
    http.server.ThreadingHTTPServer(("127.0.0.1", port), Handler).serve_forever()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
