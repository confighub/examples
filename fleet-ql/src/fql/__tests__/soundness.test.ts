// Pushdown soundness: the property the whole compiler rests on. A clause may
// only be pushed to ConfigHub when the server's result is a SUPERSET of the true
// predicate (the always-on client-side residual then narrows to the exact set).
// If a pushed clause were ever STRICTER than our semantics, the server would
// drop rows the residual can never recover — silently wrong results.
//
// These tests pin the partition verified against ConfigHub source
// (libra internal/views + public/core/function/api):
//   SOUND to push  : = < > <= >= , LIKE/ILIKE/~~/!~~ , IN , IS NULL(where) ,
//                    column-vs-column on entity `where` fields
//   UNSOUND (client): ~ ~* !~ !~* (POSIX/RE2 ≠ JS) , negation (!= NOT IN
//                    NOT LIKE) over where_data paths (missing-field/array
//                    existential divergence) , column-vs-column on where_data

import { describe, expect, it } from 'vitest';

import { parse } from '../parser';
import { plan } from '../planner';

const fetchesOf = (q: string) => plan(parse(q)).fetches;
/** Did ANY fetch push something server-side (non-empty FetchSpec)? */
const pushed = (q: string) => fetchesOf(q).some((f) => Object.keys(f).length > 0);

describe('soundness — operators that MUST push down', () => {
  const SOUND: [string, string][] = [
    ['scalar =', "SELECT unit FROM resources WHERE kind = 'Service'"],
    ['scalar >', 'SELECT unit FROM resources WHERE spec.replicas > 1'],
    ['scalar <=', 'SELECT unit FROM resources WHERE spec.replicas <= 3'],
    ['LIKE', "SELECT unit FROM resources WHERE `spec.template.spec.containers.*.image` LIKE '%alpine'"],
    ['ILIKE', "SELECT unit FROM resources WHERE `spec.template.spec.containers.*.image` ILIKE '%ALPINE'"],
    ['IN', "SELECT unit FROM resources WHERE kind IN ('Service', 'Ingress')"],
    ['entity = on units', "SELECT slug FROM units WHERE space = 'prod'"],
    ['IS NULL on entity field', 'SELECT slug FROM units WHERE target IS NULL'],
    ['column-vs-column (entity)', 'SELECT slug FROM units WHERE HeadRevisionNum > LiveRevisionNum'],
  ];
  for (const [name, q] of SOUND) {
    it(`pushes: ${name}`, () => expect(pushed(q)).toBe(true));
  }
});

describe('soundness — operators that MUST stay client-side', () => {
  const UNSOUND: [string, string][] = [
    // Regex dialects diverge (Go RE2 / Postgres POSIX vs JS RegExp).
    ['regex ~', "SELECT unit FROM resources WHERE `spec.template.spec.containers.*.image` ~ ':latest'"],
    ['regex ~*', "SELECT unit FROM resources WHERE `spec.template.spec.containers.*.image` ~* ':LATEST'"],
    ['regex !~', "SELECT unit FROM resources WHERE `spec.template.spec.containers.*.image` !~ ':latest'"],
    ['regex !~*', "SELECT unit FROM resources WHERE `spec.template.spec.containers.*.image` !~* ':latest'"],
    // Negation over a data path: missing-field `!=`→TRUE and array existential.
    ['!= on data path', "SELECT unit FROM resources WHERE `spec.template.spec.containers.*.image` != 'nginx:1.27-alpine'"],
    ['NOT LIKE on data path', "SELECT unit FROM resources WHERE `spec.template.spec.containers.*.image` NOT LIKE 'registry.internal/%'"],
    ['NOT IN on data path', "SELECT unit FROM resources WHERE kind NOT IN ('Service')"],
    // Bracket key with dots/slash can't be expressed in dotted where_data.
    [
      'dotted/slashed annotation key',
      "SELECT unit FROM resources WHERE metadata.annotations['sec-scanner.confighub.com/max-severity'] = 'CRITICAL'",
    ],
  ];
  for (const [name, q] of UNSOUND) {
    it(`client-side only: ${name}`, () => {
      expect(pushed(q)).toBe(false);
      // The residual is always retained, so the result stays exact.
      expect(plan(parse(q)).residual).not.toBeNull();
    });
  }
});

describe('soundness — negation on ENTITY fields is still sound (flat scalars)', () => {
  // Unlike data paths, entity `where` fields are flat scalars with no array /
  // missing-existential divergence, so negation there pushes down.
  it('!= on an entity field pushes', () => {
    expect(pushed("SELECT slug FROM units WHERE toolchain != 'AppConfig/YAML'")).toBe(true);
  });
  it('NOT IN on an entity field pushes', () => {
    expect(pushed("SELECT slug FROM units WHERE space NOT IN ('a', 'b')")).toBe(true);
  });
});

describe('soundness — mixed AND group pushes the sound part, keeps the rest', () => {
  it('pushes the LIKE, drops the regex to residual', () => {
    // path LIKE '%alpine' (sound) AND path ~ ':latest' (regex, client-side)
    const img = '`spec.template.spec.containers.*.image`';
    const f = fetchesOf(
      `SELECT unit FROM resources WHERE ${img} LIKE '%alpine' AND ${img} ~ ':latest'`,
    );
    expect(f).toHaveLength(1);
    expect(f[0].whereData).toBe("spec.template.spec.containers.*.image LIKE '%alpine'");
    // The regex is NOT in the pushed clause (it's in the residual).
    expect(f[0].whereData).not.toContain('~');
  });
});
