#!/usr/bin/env bash
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
tmp_dir="$(mktemp -d /tmp/addon-manager-verify-XXXXXX)"
server_pid=""

cleanup() {
  if [[ -n "${server_pid}" ]]; then
    kill "${server_pid}" 2>/dev/null || true
    wait "${server_pid}" 2>/dev/null || true
  fi
  rm -rf "${tmp_dir}"
}
trap cleanup EXIT

python3 - "${here}/serve.py" "${tmp_dir}/serve.pyc" <<'PY'
import py_compile
import sys
py_compile.compile(sys.argv[1], cfile=sys.argv[2], doraise=True)
PY

forbidden_word="$(printf '%s%s' 'pi' 'lot')"
if grep -RIni --exclude-dir=.git --exclude='verify.sh' "${forbidden_word}" "${here}" >"${tmp_dir}/forbidden-word-hits.txt"; then
  echo "The standalone app must not contain generator-tool wording:" >&2
  cat "${tmp_dir}/forbidden-word-hits.txt" >&2
  exit 1
fi

port="$(python3 - <<'PY'
import socket
sock = socket.socket()
sock.bind(("127.0.0.1", 0))
print(sock.getsockname()[1])
sock.close()
PY
)"

PORT="${port}" python3 "${here}/serve.py" >"${tmp_dir}/server.log" 2>&1 &
server_pid="$!"

python3 - "${port}" <<'PY'
import json
import sys
import time
import urllib.error
import urllib.request

base = f"http://127.0.0.1:{sys.argv[1]}"

def read(path):
    with urllib.request.urlopen(base + path, timeout=5) as response:
        return response.status, response.read().decode("utf-8")

for _ in range(50):
    try:
        status, body = read("/")
        if status == 200 and "Add-on Manager" in body:
            break
    except Exception:
        pass
    time.sleep(0.1)
else:
    raise AssertionError("server did not become ready")

status, body = read("/")
assert status == 200, status
for needle in ("Workflow model", "Add-ons by Variant", "Read-only ConfigHub sample app"):
    assert needle in body, needle
forbidden_word = "pi" + "lot"
assert forbidden_word.lower() not in body.lower(), "app UI must not mention the generator tool"

status, body = read("/api/workflow")
assert status == 200, status
workflow = json.loads(body)
assert workflow["app"] == "Add-on Manager", workflow
assert len(workflow["controls"]) == 5, workflow
assert "variant" in workflow["scope_fields"], workflow
assert "Receipt" in workflow["proof_tabs"], workflow

request = urllib.request.Request(base + "/api/apply", method="POST")
try:
    urllib.request.urlopen(request, timeout=5)
    raise AssertionError("POST unexpectedly succeeded")
except urllib.error.HTTPError as exc:
    assert exc.code == 405, exc.code
    payload = json.loads(exc.read().decode("utf-8"))
    assert "read-only sample app" in payload["error"], payload
PY

echo "addon-manager sample verify: PASS"
