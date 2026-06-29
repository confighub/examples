import { execFile } from "node:child_process";
import { buildInventory, buildReceipt, buildUnitDetail, normalizeSpace } from "./workflow.mjs";

export function runCub(args, timeout = 45000) {
  return new Promise((resolve) => {
    execFile("cub", args, {timeout}, (error, stdout, stderr) => {
      resolve({
        ok: !error,
        code: error?.code || 0,
        stdout,
        stderr,
        error: error ? String(error.message || error) : "",
        command: `cub ${args.join(" ")}`,
      });
    });
  });
}

export async function cubJson(args) {
  const result = await runCub([...args, "-o", "json"]);
  if (!result.ok) {
    return {error: (result.stderr || result.stdout || result.error).trim(), command: result.command};
  }
  try {
    return JSON.parse(result.stdout);
  } catch {
    return {error: "cub returned non-JSON output", command: result.command, raw: result.stdout.slice(0, 500)};
  }
}

export async function liveMe(config) {
  const token = await runCub(["auth", "get-token"]);
  if (!token.ok || !token.stdout.trim()) {
    return {error: (token.stderr || token.stdout || token.error).trim() || "cub auth token unavailable"};
  }
  const response = await fetch(`${config.configHubBase}/api/me`, {
    headers: {authorization: `Bearer ${token.stdout.trim()}`},
  });
  return {http: response.status, body: await response.json()};
}

export async function liveSpaces() {
  return cubJson(["space", "list"]);
}

export async function liveUnits(space) {
  return cubJson(["unit", "list", "--space", space]);
}

export async function liveUnit(space, unit) {
  return cubJson(["unit", "get", unit, "--space", space]);
}

export async function liveRevisions(space, unit) {
  return cubJson(["revision", "list", "--space", space, unit]);
}

export async function liveUnitData(space, unit) {
  const result = await runCub(["unit", "data", "--space", space, unit]);
  if (!result.ok) {
    return {error: (result.stderr || result.stdout || result.error).trim(), command: result.command};
  }
  return {text: result.stdout};
}

export async function liveInventory() {
  const spaces = await liveSpaces();
  if (spaces.error) return spaces;
  const unitsBySpace = {};
  for (const item of spaces.items || spaces.Items || spaces.Spaces || []) {
    const space = normalizeSpace(item);
    if (!space.slug?.startsWith("helm-")) continue;
    const units = await liveUnits(space.slug);
    unitsBySpace[space.slug] = units.error ? [] : units;
  }
  return {
    source: "live",
    ...buildInventory(spaces, unitsBySpace),
  };
}

export async function liveDetail(spaceSlug, unitSlug) {
  const [spaces, unit, revisions, data] = await Promise.all([
    liveSpaces(),
    liveUnit(spaceSlug, unitSlug),
    liveRevisions(spaceSlug, unitSlug),
    liveUnitData(spaceSlug, unitSlug),
  ]);
  if (unit.error) return unit;
  const space = (spaces.items || spaces.Items || spaces.Spaces || []).map(normalizeSpace).find((candidate) => candidate.slug === spaceSlug);
  return buildUnitDetail({
    space,
    unitRecord: unit.Unit || unit,
    revisions,
    unitData: data.text || "",
    source: "live",
  });
}

export async function liveReceipt(space, unit) {
  return buildReceipt(await liveDetail(space, unit));
}
