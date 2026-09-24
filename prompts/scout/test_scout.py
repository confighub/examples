#!/usr/bin/env python3
"""Regression tests for scout.py. Run: python3 test_scout.py

The last block checks every planted answer in the example fleet at
gitops/argo/intermediate-git-as-database/repo (override with SCOUT_FLEET)."""
import os, sys, subprocess, tempfile, textwrap, json

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import scout


def repo(files):
    d = tempfile.mkdtemp()
    for rel, body in files.items():
        p = os.path.join(d, rel)
        os.makedirs(os.path.dirname(p), exist_ok=True)
        open(p, 'w').write(textwrap.dedent(body))
    return d


def inventory(n=20, extra=None):
    files = {}
    for i in range(n):
        env = 'prod' if i % 2 else 'dev'
        files[f'clusters/c{i:02d}.yaml'] = (
            f"global:\n  name: c{i:02d}\n  env: {env}\n  region: r{i % 4}\n"
            f"  gpu: {'true' if i < 5 else 'false'}\n")
    files.update(extra or {})
    return files


APPSET = """\
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata: {{name: x}}
spec:
  generators:
{gens}
  template:
    spec:
      syncPolicy:
{sync}
"""

def appset(gens, sync='        automated: {selfHeal: true}'):
    return APPSET.format(gens=textwrap.indent(textwrap.dedent(gens), '  '), sync=sync)


FAILS = []
def check(name, cond, got=None):
    print(('ok    ' if cond else 'FAIL  ') + name + ('' if cond else f'   got: {got}'))
    if not cond:
        FAILS.append(name)


# --- version literals: block, flow, trailing comment; apiVersion/appVersion excluded
r = scout.Repo([repo({'a.yaml': """\
    apiVersion: argoproj.io/v1alpha1
    kind: ApplicationSet
    spec:
      generators:
      - values: {version: 1.4.0}
      - values:
          version: 1.4.0   # wave 2
      - values: {version: "1.5.0", other: x}
      appVersion: 9.9.9
      kubeVersion: 1.28
    """})])
q = scout.q34_promotion(r)
check('versions: flow + block + comment counted', q['repeat_max'] == 2, q)
check('versions: two distinct live', q['definitions_multi_version'] == 1, q)

# --- sprawl ledger: one declared value in a directory named for the dimension
files = inventory(extra={'values/environments/prod.yaml': 'a: 1\n'})
r = scout.Repo([repo(files)])
inv = scout.q5_reach(r)['_inv']
lg = scout.q2b_sprawl_ledger(r, inv)
env = [d for d in lg['dimensions'] if d['dimension'] == 'env']
check('ledger: 1-of-2 declared env reported', env and env[0]['undeclared'] == 1, lg['dimensions'])

# --- selectors: boolean labels, DoesNotExist, partial resolution
g = """\
- clusters:
    selector:
      matchLabels: {gpu: true}
"""
r = scout.Repo([repo(inventory(extra={'appsets/a.yaml': appset(g)}))])
q = scout.q5_reach(r)
check('selector: YAML boolean matches', q['reach_max'] == 5, q['reach_max'])

g = """\
- clusters:
    selector:
      matchExpressions:
      - {key: team, operator: DoesNotExist}
      - {key: env, operator: In, values: [prod]}
"""
r = scout.Repo([repo(inventory(extra={'appsets/a.yaml': appset(g)}))])
q = scout.q5_reach(r)
check('selector: DoesNotExist honoured', q['reach_max'] == 10, q['reach_max'])

g = """\
- clusters:
    selector:
      matchLabels: {env: prod}
- git:
    repoURL: x
"""
r = scout.Repo([repo(inventory(extra={'appsets/a.yaml': appset(g)}))])
q = scout.q5_reach(r)
check('selector: partial resolution flagged', q['partially_resolved'] == 1, q)

g = """\
- clusters: {}
"""
r = scout.Repo([repo(inventory(extra={'appsets/a.yaml': appset(g)}))])
q = scout.q5_reach(r)
check('selector: uninterpretable is unresolved, never "all"',
      q['unresolved_definitions'] == 1 and q['instances'] == 0, q)

# --- drift posture: automated: {} is ON; enabled: false is OFF
for sync, want in [('        automated: {}', 'automated, no self-heal'),
                   ('        automated: {enabled: false, selfHeal: true}', 'no automated sync'),
                   ('        automated: {selfHeal: true, prune: true}', 'self-heal + prune'),
                   ('        syncOptions: [CreateNamespace=true]', 'no automated sync')]:
    r = scout.Repo([repo({'a.yaml': appset('- list: {elements: []}\n', sync)})])
    q = scout.q7_drift_tolerance(r)
    check(f'posture: {sync.strip()} -> {want}', q['posture'].get(want) == 1, q['posture'])

# --- value-file refs: only from valueFiles lists; kustomize resources ignored
r = scout.Repo([repo({
    'app.yaml': """\
        kind: Application
        spec:
          source:
            helm:
              valueFiles:
              - values/present.yaml
              - values/missing.yaml
              - values/{{env}}.yaml
        """,
    'values/present.yaml': 'a: 1\n',
    'kustomization.yaml': """\
        resources:
        - not-a-values-file.yaml
        """,
    'mixed.yaml': """\
        kind: Application
        spec:
          source:
            helm:
              valueFiles: [values/present.yaml]
          other:
          - unrelated.yaml
        """})])
q = scout.q2_keys_in_paths(r)
check('refs: literal/templated split', (q['literal_refs'], q['templated_refs']) == (2, 1), q)
check('refs: missing file found, list-only', q['missing'] == ['values/missing.yaml'], q['missing'])

# --- render gap: plain manifests in a kustomize tree are sources
r = scout.Repo([repo({
    'base/kustomization.yaml': 'apiVersion: kustomize.config.k8s.io/v1beta1\nresources: [dep.yaml]\n',
    'base/dep.yaml': 'kind: Deployment\nmetadata: {name: a}\n',
    'rendered/prod/dep.yaml': 'kind: Deployment\nmetadata: {name: a}\n'})])
q = scout.q1_render_gap(r)
check('render: kustomize base not counted as rendered', q['rendered_manifests'] == 1, q)
r = scout.Repo([repo({
    'kustomization.yaml': 'resources: [dep.yaml]\n',
    'dep.yaml': 'kind: Deployment\nmetadata: {name: a}\n'})])
q = scout.q1_render_gap(r)
check('render: root-level kustomization respected', q['rendered_manifests'] == 0, q)

# --- controls: MAX_RETRIES is not a change cap
r = scout.Repo([repo({'.github/workflows/ci.yaml': 'env:\n  MAX_RETRIES: 3\n  MAX_CLUSTERS: 5\n'})])
q = scout.q6_controls(r)
check('controls: MAX_CLUSTERS matched', q['where'].get('cap on files or targets per change', ('', ''))[1] == 'MAX_CLUSTERS', q)
r = scout.Repo([repo({'.github/workflows/ci.yaml': 'env:\n  MAX_RETRIES: 3\n'})])
check('controls: MAX_RETRIES ignored', scout.q6_controls(r)['count'] == 0)

# --- pinning: quoted ranges kept whole; HelmRelease without version floats
r = scout.Repo([repo({
    'a.yaml': 'kind: Application\nspec:\n  source:\n    chart: x\n    targetRevision: ">=1.0.0 <2.0.0"\n',
    'hr.yaml': 'kind: HelmRelease\nspec:\n  chart:\n    spec:\n      chart: y\n',
    'c/Chart.yaml': 'name: c\ndependencies:\n- {name: p, version: "1.2.3"}\n'})])
q = scout.q7c_dependency_pinning(r)
vals = sorted(v for _, v in q['ranged_source_examples'])
check('pinning: quoted range whole', '>=1.0.0 <2.0.0' in vals, vals)
check('pinning: HelmRelease without version flagged', '(none: latest)' in vals, vals)
check('pinning: exact dependency not flagged', q['ranged_deps'] == 0, q)

# --- root prefix collision: /x/a must not claim files under /x/ab
base = tempfile.mkdtemp()
for sub, body in (('a', 'k: 1\n'), ('ab', 'k: 2\n')):
    os.makedirs(os.path.join(base, sub))
    open(os.path.join(base, sub, 'f.yaml'), 'w').write(body)
r = scout.Repo([os.path.join(base, 'a')])
check('roots: no prefix collision', len(r.paths) == 1, r.paths)

# --- fleet-small: every planted answer
FLEET = os.environ.get('SCOUT_FLEET', os.path.join(
    HERE, '..', '..', 'gitops', 'argo', 'intermediate-git-as-database', 'repo'))
out = json.loads(subprocess.run([sys.executable, os.path.join(HERE, 'scout.py'),
                                 FLEET, '--json'],
                                capture_output=True, text=True, check=True).stdout)
check('fleet Q1: 1 templated, 0 rendered',
      (out['q1']['templated_sources'], out['q1']['rendered_manifests']) == (1, 0), out['q1'])
check('fleet Q2: rename touches 2 files', out['q2']['rename_cost']['median'] == 2, out['q2']['rename_cost'])
check('fleet Q2: 1 silent missing layer', out['q2']['missing'] == ['$values/values/org/acme.yaml'], out['q2']['missing'])
led = {d['dimension']: d['undeclared'] for d in out['q2b']['dimensions']}
check('fleet ledger: org 2, provider 1, env 0 undeclared',
      (led.get('org'), led.get('provider'), led.get('env')) == (2, 1, 0), led)
check('fleet Q3: version repeated 4x', out['q34']['repeat_max'] == 4, out['q34'])
check('fleet Q4: 3 versions live', out['q34']['worst_skew'][0][0] == 3, out['q34'])
rbp = dict(out['q5']['reach_by_path'])
check('fleet Q5: 24 vs 1',
      (rbp.get('/values/global.yaml'), rbp.get('/values/clusters/acme-prod-use1.yaml')) == (24, 1), rbp)
check('fleet Q6: 2 quotas', out['q6']['count'] == 2, out['q6'])
check('fleet Q7: 1 self-heal no prune', out['q7']['posture'].get('self-heal, no prune') == 1, out['q7'])
check('fleet Q7b: generated/ guarded', out['q7b']['dirs'][0]['guarded_by'] != [], out['q7b'])
check('fleet Q7c: unpinned, lock ignored',
      out['q7c']['ranged_deps'] == 1 and out['q7c']['lock_gitignored'], out['q7c'])

print(f"\n{len(FAILS)} failed" if FAILS else "\nall passed")
sys.exit(1 if FAILS else 0)
