import fs from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { FIXTURES_URL } from "./config.mjs";
import { buildInventory, buildReceipt, buildUnitDetail, normalizeSpace, normalizeUnit } from "./workflow.mjs";

const FIXTURES_DIR = fileURLToPath(FIXTURES_URL);

async function readJson(relativePath) {
  const text = await fs.readFile(path.join(FIXTURES_DIR, relativePath), "utf8");
  return JSON.parse(text);
}

function unitDataName(space, unit) {
  return `${space}--${unit}.txt`.replace(/[^a-zA-Z0-9_.-]/g, "_");
}

export async function fixtureMe() {
  return readJson("me.json");
}

export async function fixtureSpaces() {
  return readJson("spaces.json");
}

export async function fixtureUnits(space) {
  const units = await readJson("units.json");
  return units[space] || [];
}

export async function fixtureUnit(space, unit) {
  const units = await fixtureUnits(space);
  return units.find((candidate) => normalizeUnit(candidate).slug === unit) || null;
}

export async function fixtureRevisions(space, unit) {
  const revisions = await readJson("revisions.json");
  return revisions[`${space}/${unit}`] || [];
}

export async function fixtureUnitData(space, unit) {
  try {
    return await fs.readFile(path.join(FIXTURES_DIR, "unitdata", unitDataName(space, unit)), "utf8");
  } catch {
    return "";
  }
}

export async function fixtureInventory() {
  const spaces = await fixtureSpaces();
  const unitsBySpace = {};
  for (const item of spaces.items || []) {
    const space = normalizeSpace(item);
    unitsBySpace[space.slug] = await fixtureUnits(space.slug);
  }
  return {
    source: "fixture",
    ...buildInventory(spaces, unitsBySpace),
  };
}

export async function fixtureDetail(spaceSlug, unitSlug) {
  const spaces = await fixtureSpaces();
  const space = (spaces.items || []).map(normalizeSpace).find((candidate) => candidate.slug === spaceSlug);
  const unit = await fixtureUnit(spaceSlug, unitSlug);
  const revisions = await fixtureRevisions(spaceSlug, unitSlug);
  const unitData = await fixtureUnitData(spaceSlug, unitSlug);
  return buildUnitDetail({space, unitRecord: unit, revisions, unitData, source: "fixture"});
}

export async function fixtureReceipt(space, unit) {
  return buildReceipt(await fixtureDetail(space, unit));
}
