#!/usr/bin/env python3
"""
scout - seven questions to ask if you are using Git as a database.

Read-only. No network. No credentials. No telemetry. No writes.
Everything it prints is computed from files on disk.

  pip install pyyaml
  python3 scout.py /path/to/config-repo [/path/to/another-repo ...]

Options:
  --resolve   list every layered value reference scout could not match to a
              file, and every templated reference it could not check
  --json      machine-readable output

scout is the reference implementation of the questions. The ten-question
prompt at github.com/confighub/examples/tree/main/prompts asks the same things
of an AI coding agent and explains the answers in your repo's own terms. Where
the two disagree, trust scout and tell us.
"""
import os, re, sys, json, signal, statistics, collections

try:
    import yaml
except ImportError:
    sys.exit("scout needs pyyaml:  pip install pyyaml")

SKIP_DIRS = {'.git', 'node_modules', 'vendor', '.terraform', 'dist', 'build'}

# CI, policy and bot configuration. Question 6 only looks here, so that
# application config containing the string MAX_FOO is not mistaken for a control.
CONTROL_PATHS = (
    '.prow/', '.github/workflows/', '.github/', '.gitlab-ci', 'renovate.json',
    '.renovaterc', 'jenkinsfile', '.tekton/', '.circleci/', 'buildkite',
    '.mergify', '.kodiak', 'dependabot.yml', 'dependabot.yaml',
)

CONTROLS = [
    (r'MAX_[A-Z_]*(CLUSTER|FILE|TARGET|CHANGE|PR|APP)[A-Z_]*',
                                                       'cap on files or targets per change'),
    (r'prConcurrentLimit["\']?\s*:\s*(\d+)',           'cap on concurrent bot pull requests'),
    (r'prHourlyLimit["\']?\s*:\s*(\d+)',               'cap on bot pull requests per hour'),
    (r'(?i)(freeze|blackout|restricted[_ -]?change|rcp)[\w_]*\s*[:=]',
                                                       'calendar freeze window'),
    (r'(?i)parallelism\s*[:=]\s*(\d+)',                'cap on parallel execution'),
    (r'--concurrent[-\w]*[=\s](\d+)',                  'cap on reconcile concurrency'),
    (r'(?i)(rate[_ -]?limit|throttle)\s*[:=]',         'explicit throttle'),
    (r'(?i)manifest-generate-paths',                   'cap on re-render scope'),
]


# ------------------------------------------------------------------ index

class Repo:
    """Walk once, parse once. Everything else reads from here."""

    def __init__(self, roots):
        self.roots = [os.path.abspath(r) for r in roots]
        self.paths, self.text, self.docs = [], {}, {}
        self.exists = set()
        for root in self.roots:
            for d, subs, fs in os.walk(root):
                subs[:] = [s for s in subs if s not in SKIP_DIRS]
                for f in fs:
                    p = os.path.join(d, f)
                    self.paths.append(p)
                    self.exists.add(os.path.relpath(p, root))
                    if f.endswith(('.yaml', '.yml', '.json', '.sh', '.toml')):
                        try:
                            self.text[p] = open(p, errors='ignore').read()
                        except OSError:
                            pass
        for p, t in self.text.items():
            if p.endswith(('.yaml', '.yml')) and len(t) < 4_000_000:
                try:
                    self.docs[p] = [d for d in yaml.safe_load_all(t) if isinstance(d, dict)]
                except Exception:
                    self.docs[p] = []

    def yamls(self):
        return [p for p in self.docs]

    def kind(self, k):
        for p, ds in self.docs.items():
            for d in ds:
                if d.get('kind') == k:
                    yield p, d

    def has(self, rel):
        return rel in self.exists


# ------------------------------------------------------------------ Q1

def q1_render_gap(repo):
    templated = rendered = hydrated = 0
    # plain manifests inside a Kustomize tree are inputs to a build, not output
    kdirs = {os.path.dirname(p) for p in repo.paths
             if os.path.basename(p) in ('kustomization.yaml', 'kustomization.yml',
                                        'Kustomization')}
    def under_kustomize(p):
        d = os.path.dirname(p)
        while True:
            if d in kdirs:
                return True
            if d in repo.roots or d == os.path.dirname(d):
                return False
            d = os.path.dirname(d)
    for p, ds in repo.docs.items():
        t = repo.text.get(p, '')
        if re.search(r'valueFiles|^\s*helm:\s*$|kustomize', t, re.M):
            templated += 1
        for d in ds:
            if d.get('kind') in ('Deployment', 'StatefulSet', 'DaemonSet',
                                 'CronJob', 'Service', 'Ingress') and not under_kustomize(p):
                rendered += 1
            # Argo CD Source Hydrator (beta from v3.5) commits the render to a
            # branch. It stores the output; it does not say how far it reached.
            sp = d.get('spec') or {}
            if d.get('kind') == 'ApplicationSet':
                sp = ((sp.get('template') or {}).get('spec')) or {}
            if isinstance(sp, dict) and sp.get('sourceHydrator'):
                hydrated += 1
    return dict(templated_sources=templated, rendered_manifests=rendered,
                hydrated_definitions=hydrated,
                gap=(rendered == 0 and hydrated == 0 and templated > 0))


# ------------------------------------------------------------------ Q2

def _value_file_refs(docs):
    """Every string listed under a valueFiles / valuesFiles key, at any depth."""
    out = []
    def walk(x):
        if isinstance(x, dict):
            for k, v in x.items():
                if k in ('valueFiles', 'valuesFiles') and isinstance(v, list):
                    out.extend(str(i) for i in v if isinstance(i, (str, int)))
                else:
                    walk(v)
        elif isinstance(x, list):
            for i in x:
                walk(i)
    for d in docs:
        walk(d)
    return out


def q2_keys_in_paths(repo, want_resolve=False):
    schemes, seen = [], set()
    for root in repo.roots:
        for d, subs, fs in os.walk(root):
            subs[:] = [s for s in subs if s not in SKIP_DIRS]
            names = [f.rsplit('.', 1)[0] for f in fs if f.endswith(('.yaml', '.yml'))]
            if len(names) < 10:
                continue
            for delim in ('-', '_'):
                parts = [n.split(delim) for n in names]
                widths = collections.Counter(len(p) for p in parts)
                w, cnt = widths.most_common(1)[0]
                if w < 2 or cnt / len(names) < 0.9:
                    continue
                card = [len({p[i] for p in parts if len(p) == w}) for i in range(w)]
                if min(card[1:]) >= len(names) * 0.5:
                    continue
                sig = (w, delim, tuple(sorted(card)[:2]))
                schemes.append(dict(dir=os.path.relpath(d, root), rows=len(names),
                                    fields=w, delim=delim, cardinalities=card,
                                    dup=sig in seen))
                seen.add(sig)
                break
    schemes.sort(key=lambda s: -s['rows'])

    # rename cost, sampled
    cost = None
    if schemes:
        top = schemes[0]
        d = next((os.path.join(r, top['dir']) for r in repo.roots
                  if os.path.isdir(os.path.join(r, top['dir']))), None)
        if d:
            rows = [f.rsplit('.', 1)[0] for f in os.listdir(d)
                    if f.endswith(('.yaml', '.yml'))][:30]
            basenames = [(os.path.basename(p), os.path.dirname(p)) for p in repo.paths]
            hits = [sum(1 for b, dd in basenames if r in b or r in dd) for r in rows]
            if hits:
                cost = dict(median=statistics.median(hits), max=max(hits))

    # silent resolution: layered value refs, and which are literal enough to check
    ignore_on = layered = 0
    templated_refs, literal_refs = set(), set()
    for p, t in repo.text.items():
        if 'valueFiles' not in t and 'valuesFiles' not in t:
            continue
        layered += 1
        if re.search(r'ignoreMissing(Value|Values)Files:\s*true', t):
            ignore_on += 1
        for ref in _value_file_refs(repo.docs.get(p, [])):
            (templated_refs if '{{' in ref else literal_refs).add(ref)

    # literal refs: does any file in the scanned roots match? Matching is by
    # path suffix after stripping $ref/ prefixes and ./ ../ segments, so it is
    # lenient: it can call a ref resolved when the real file is elsewhere, never
    # the reverse. Missing counts are therefore a lower bound.
    rels = set()
    for root in repo.roots:
        rels |= {os.path.relpath(p, root) for p in repo.paths
                 if p.startswith(root + os.sep)}
    def found(ref):
        r = re.sub(r'^\$[\w-]+/', '', ref)
        parts = [x for x in r.split('/') if x not in ('', '.', '..')]
        if not parts:
            return True
        tail = '/'.join(parts)
        return any(x == tail or x.endswith('/' + tail) for x in rels)
    missing = sorted(r for r in literal_refs if not found(r))

    return dict(schemes=schemes[:6], distinct_schemes=len(seen), rename_cost=cost,
                sources_with_layers=layered, ignore_missing=ignore_on,
                ignore_pct=(ignore_on * 100 // layered) if layered else 0,
                literal_refs=len(literal_refs), templated_refs=len(templated_refs),
                literal_missing=len(missing),
                missing=missing[:(999 if want_resolve else 6)],
                uncheckable=sorted(templated_refs)[:(999 if want_resolve else 6)])


# ------------------------------------------------------------------ Q2b
# Sprawl ledger: dimensions declared as directories of enum files, compared
# against the values actually present across the discovered target inventory.

def q2b_sprawl_ledger(repo, inv):
    """A directory whose yaml filenames (or subdirectory names) match the set of
    values some target label takes is an enum declaration. Report declared vs live."""
    if not inv:
        return dict(dimensions=[], note="no target inventory discovered")

    # candidate label keys: low cardinality, present on most targets
    n = len(inv)
    keys = collections.Counter()
    for lab in inv.values():
        keys.update(lab.keys())
    cand = {}
    for k, seen in keys.items():
        if seen < n * 0.8:
            continue
        vals = collections.Counter(str(l.get(k, '')) for l in inv.values())
        vals.pop('', None)
        if 2 <= len(vals) <= 60 and len(vals) < n * 0.5:
            cand[k] = vals
    if not cand:
        return dict(dimensions=[], note="no low-cardinality target labels found")

    # candidate declaration dirs: small dirs of yaml files or subdirs
    decls = []
    for root in repo.roots:
        for d, subs, fs in os.walk(root):
            if any(x in d for x in SKIP_DIRS):
                continue
            names = {f.rsplit('.', 1)[0] for f in fs if f.endswith(('.yaml', '.yml'))}
            names |= set(subs)
            names -= {'values', 'global', 'all', 'defaults', 'README', 'Chart', 'ci'}
            if 1 <= len(names) <= 60:
                decls.append((os.path.relpath(d, root), names))

    out = []
    for k, vals in cand.items():
        live = set(vals)
        for rel, names in decls:
            hit = names & live
            # one declared value is enough when the directory is named for the
            # dimension (providers/ for provider): that is the worst case, not noise
            stem = rel.rstrip('/').split('/')[-1].lower().rstrip('s')
            named = len(stem) >= 3 and (stem.startswith(k.lower())
                                        or k.lower().startswith(stem))
            if len(hit) < (1 if named else 2):
                continue
            # the directory must be mostly explained by this dimension
            if len(hit) / max(len(names), 1) < 0.75:
                continue
            undeclared = {v: vals[v] for v in live - names}
            out.append(dict(dimension=k, directory=rel,
                            values_live=len(live), declared=len(hit),
                            undeclared=len(undeclared),
                            targets_affected=sum(undeclared.values()),
                            worst=sorted(undeclared.items(), key=lambda x: -x[1])[:5]))
    # keep, per dimension, the most complete and the most incomplete declaration
    bydim = collections.defaultdict(list)
    for r in out:
        bydim[r['dimension']].append(r)
    keep = []
    for k, rs in bydim.items():
        rs.sort(key=lambda r: r['targets_affected'])
        keep.append(rs[0])
        if len(rs) > 1 and rs[-1]['targets_affected'] > rs[0]['targets_affected']:
            keep.append(rs[-1])
    keep.sort(key=lambda r: -r['targets_affected'])
    out = keep
    multi = {k: len(v) for k, v in bydim.items() if len(v) > 1}

    # the escape hatch: per-target override files
    hatch = 0
    hatch_dirs = 0
    for root in repo.roots:
        for d, subs, fs in os.walk(root):
            if any(x in d for x in SKIP_DIRS):
                continue
            hits = sum(1 for f in fs
                       if f.endswith(('.yaml', '.yml')) and f.rsplit('.', 1)[0] in inv)
            if hits >= 5:
                hatch += hits
                hatch_dirs += 1
    return dict(dimensions=out, per_target_overrides=hatch,
                override_dirs=hatch_dirs, declared_in_many_places=multi, note=None)


# ------------------------------------------------------------------ Q3 / Q4

VER = re.compile(r'(?<![\w.])(?:version|targetRevision|tag)\s*:\s*["\']?([0-9][\w.\-]*)["\']?'
                 r'\s*(?=$|[,}\n]|#)', re.M)

def q34_promotion(repo):
    repeats, skew = [], []
    DEF_KINDS = ('ApplicationSet', 'Application', 'HelmRelease', 'Kustomization')
    for p, t in repo.text.items():
        if not p.endswith(('.yaml', '.yml')):
            continue
        if not any(f'kind: {k}' in t for k in DEF_KINDS):
            continue
        vs = VER.findall(t)
        if not vs:
            continue
        c = collections.Counter(vs)
        top = c.most_common(1)[0]
        if top[1] > 1:
            repeats.append((top[1], os.path.relpath(p, repo.roots[0])))
        if len(c) > 1:
            skew.append((len(c), dict(c), os.path.relpath(p, repo.roots[0])))
    repeats.sort(reverse=True)
    skew.sort(key=lambda x: -x[0])
    r = [x[0] for x in repeats]
    return dict(definitions_repeating=len(repeats),
                repeat_median=statistics.median(r) if r else 0,
                repeat_max=max(r) if r else 0,
                worst_repeat=repeats[0][1] if repeats else None,
                definitions_multi_version=len(skew),
                worst_skew=[(n, p) for n, _, p in skew[:5]])


# ------------------------------------------------------------------ Q5

def q5_reach(repo):
    """Resolve ApplicationSet cluster selectors against a discovered target
    inventory. Reports what it could NOT resolve rather than guessing."""
    inv = {}
    for root in repo.roots:
        for d, subs, fs in os.walk(root):
            subs[:] = [s for s in subs if s not in SKIP_DIRS]
            ys = [f for f in fs if f.endswith(('.yaml', '.yml'))]
            if len(ys) < 20:
                continue
            cand = {}
            for f in ys:
                for doc in repo.docs.get(os.path.join(d, f), [])[:1]:
                    g = doc.get('global', doc)
                    if isinstance(g, dict):
                        lab = {k: (str(v).lower() if isinstance(v, bool) else str(v))
                               for k, v in g.items() if isinstance(v, (str, int, bool))}
                        if len(lab) >= 3:
                            cand[f.rsplit('.', 1)[0]] = lab
            if len(cand) > len(inv):
                inv = cand

    def norm(v):
        return str(v).lower() if isinstance(v, bool) else str(v)

    def match(lab, sel):
        for k, v in (sel.get('matchLabels') or {}).items():
            if '/' in k:           # infrastructure label, not a targeting label
                continue
            if lab.get(k) != norm(v):
                return False
        for e in (sel.get('matchExpressions') or []):
            k, op = e.get('key'), e.get('operator')
            vals = [norm(x) for x in (e.get('values') or [])]
            cur = lab.get(k)
            if op == 'In' and cur not in vals:
                return False
            if op == 'NotIn' and cur in vals:
                return False
            if op == 'Exists' and cur is None:
                return False
            if op == 'DoesNotExist' and cur is not None:
                return False
        return True

    def collect(gens, depth=0):
        """Returns (targets, unresolved_generator_count)."""
        s, unres = set(), 0
        for g in gens or []:
            if not isinstance(g, dict):
                continue
            if 'clusters' in g:
                sel = (g['clusters'] or {}).get('selector') or {}
                keys = set((sel.get('matchLabels') or {})) | {
                    e.get('key') for e in (sel.get('matchExpressions') or [])}
                keys = {k for k in keys if k and '/' not in k}
                if not keys:
                    unres += 1            # selector we cannot interpret: do NOT
                    continue              # fall back to "matches everything"
                s |= {k for k, l in inv.items() if match(l, sel)}
            for nest in ('merge', 'matrix'):
                if isinstance(g.get(nest), dict):
                    t, u = collect(g[nest].get('generators'), depth + 1)
                    s |= t
                    unres += u
            if any(k in g for k in ('git', 'list', 'scmProvider', 'pullRequest')):
                unres += 1
        return s, unres

    fan, unresolved, partial, residual = [], 0, 0, 0
    for p, d in repo.kind('ApplicationSet'):
        gens = (d.get('spec') or {}).get('generators')
        t, u = collect(gens)
        unresolved += 1 if u and not t else 0
        partial += 1 if u and t else 0
        if 'NotIn' in json.dumps(gens or []):
            residual += 1
        if t:
            fan.append((len(t), os.path.relpath(p, repo.roots[0])))
    fan.sort(reverse=True)
    v = [x[0] for x in fan] or [0]

    # reach by declared re-render scope, if the annotation is present
    scope = collections.Counter()
    for p, d in repo.kind('ApplicationSet'):
        ann = (((d.get('spec') or {}).get('template') or {}).get('metadata') or {}).get('annotations') or {}
        mgp = str(ann.get('argocd.argoproj.io/manifest-generate-paths', ''))
        t, _ = collect((d.get('spec') or {}).get('generators'))
        for seg in [s for s in mgp.split(';') if s and '{{' not in s]:
            scope[seg.strip()] += len(t)

    return dict(_inv=inv, targets=len(inv), definitions=len(fan) + unresolved,
                resolved_definitions=len(fan), unresolved_definitions=unresolved,
                partially_resolved=partial,
                instances=sum(v), reach_median=statistics.median(v),
                reach_max=max(v), residual_selectors=residual,
                widest=fan[:3],
                reach_by_path=scope.most_common(5))


# ------------------------------------------------------------------ Q6

def q6_controls(repo):
    found, where = collections.Counter(), {}
    for p, t in repo.text.items():
        low = p.lower()
        if not any(c in low for c in CONTROL_PATHS):
            continue
        for rx, name in CONTROLS:
            m = re.search(rx, t)
            if m:
                found[name] += 1
                where.setdefault(name, (os.path.relpath(p, repo.roots[0]),
                                        m.group(0).strip()[:60]))
    return dict(count=len(found), detail=dict(found), where=where)


# ------------------------------------------------------------------ Q7
# Drift tolerance. Drift itself is not visible from a repo: it is the difference
# between the repo and the running system. The *tolerance* for it is declared
# here, and that splits the same way variety does — declared, or merely allowed.

def q7_drift_tolerance(repo):
    posture = collections.Counter()
    declared = collections.Counter()
    fields = collections.Counter()
    kinds = collections.Counter()
    n = 0
    for p, d in list(repo.kind('ApplicationSet')) + list(repo.kind('Application')):
        sp = d.get('spec') or {}
        if d.get('kind') == 'ApplicationSet':
            sp = ((sp.get('template') or {}).get('spec')) or {}
        sy = sp.get('syncPolicy') or {}
        on = 'automated' in sy and sy.get('automated') is not False
        a = sy.get('automated') or {}
        if isinstance(a, dict) and a.get('enabled') is False:
            on = False
        n += 1
        opts = [str(o) for o in (sy.get('syncOptions') or [])]
        if not on:
            posture['no automated sync'] += 1
        elif not a.get('selfHeal'):
            posture['automated, no self-heal'] += 1
        elif not a.get('prune'):
            posture['self-heal, no prune'] += 1
        else:
            posture['self-heal + prune'] += 1
        for o in opts:
            if o.startswith(('Prune=', 'RespectIgnoreDifferences=', 'ServerSideApply=')):
                declared[o] += 1
        idf = sp.get('ignoreDifferences') or []
        if idf:
            declared['ignoreDifferences block'] += 1
        for e in idf:
            if e.get('kind'):
                kinds[e['kind']] += 1
            if e.get('managedFieldsManagers'):
                declared['managedFieldsManagers'] += 1
            for jp in (e.get('jsonPointers') or []) + (e.get('jqPathExpressions') or []):
                fields[str(jp)] += 1

    # Flux equivalent
    flux = collections.Counter()
    for p, d in list(repo.kind('Kustomization')) + list(repo.kind('HelmRelease')):
        sp = d.get('spec') or {}
        if 'prune' in sp:
            flux['prune=%s' % sp['prune']] += 1
        if (sp.get('force')):
            flux['force=true'] += 1
    return dict(definitions=n, posture=dict(posture), declared=dict(declared),
                top_fields=fields.most_common(6), top_kinds=kinds.most_common(6),
                flux=dict(flux))


# ------------------------------------------------------------------ Q7b
# Observed data in an intent store. If facts about the running system are
# written back into the repo, the repo holds two kinds of truth. What stops a
# person editing the observed kind by hand?

OBSERVED_DIRS = re.compile(r'(^|/)(generated|observed|discovered|inventory|'
                           r'status|live|state|snapshots?|exported)(/|$)', re.I)
OBSERVED_MARK = re.compile(r'(?i)(do not edit|auto-?generated|generated by|'
                           r'this file is (managed|maintained) by)')

def q7b_observed_data(repo):
    dirs = collections.Counter()
    marked = 0
    for p in repo.paths:
        rel = None
        for root in repo.roots:
            if p.startswith(root + os.sep):
                rel = os.path.relpath(p, root)
        if rel is None:
            continue
        m = OBSERVED_DIRS.search(os.path.dirname(rel) + '/')
        head = repo.text.get(p, '')[:400]
        if m:
            key = os.path.dirname(rel)[:m.end() - 1] if m.end() else rel
            dirs[key.rstrip('/')] += 1
        if OBSERVED_MARK.search(head):
            marked += 1
    # guards: does any CI / policy / hook file mention each directory?
    ci = {p: t for p, t in repo.text.items()
          if any(c in p.lower() for c in CONTROL_PATHS)
          or os.path.basename(p) in ('CODEOWNERS', '.pre-commit-config.yaml')}
    for root in repo.roots:
        co = os.path.join(root, '.github', 'CODEOWNERS')
        if os.path.exists(co):
            ci[co] = open(co, errors='ignore').read()
    out = []
    for d, n in dirs.most_common(8):
        name = d.split('/')[-1]
        guards = sorted({os.path.relpath(p, repo.roots[0])
                         for p, t in ci.items() if d in t or f'{name}/' in t})
        out.append(dict(dir=d, files=n, guarded_by=guards[:3]))
    return dict(dirs=out, marked_files=marked)


# ------------------------------------------------------------------ Q7c
# Dependency pinning. A chart that depends on a version range, with no lock
# file committed, resolves at render time. Half a rollout can render against
# one version and the other half against the next.

RANGE = re.compile(r'[\^~<>*|]|\bx\b|\.x(\.|$)|\s-\s')

def q7c_dependency_pinning(repo):
    charts, ranged, locked = 0, [], 0
    ignored = False
    for root in repo.roots:
        gi = os.path.join(root, '.gitignore')
        if os.path.exists(gi) and re.search(r'(?m)^\s*/?(\*\*/)?Chart\.lock\s*$',
                                             open(gi, errors='ignore').read()):
            ignored = True
    for p, ds in repo.docs.items():
        if os.path.basename(p) != 'Chart.yaml' or not ds:
            continue
        deps = ds[0].get('dependencies') or []
        if not deps:
            continue
        charts += 1
        if os.path.exists(os.path.join(os.path.dirname(p), 'Chart.lock')):
            locked += 1
        for dep in deps:
            v = str((dep or {}).get('version', ''))
            if not v or RANGE.search(v):
                ranged.append((os.path.relpath(p, repo.roots[0]),
                               (dep or {}).get('name'), v or '(none)'))
    # Flux HelmRelease / Argo helm sources with a range as the chart version
    src_ranged = []
    for p, d in list(repo.kind('HelmRelease')):
        v = str(((((d.get('spec') or {}).get('chart') or {}).get('spec') or {})
                 .get('version', '')))
        if not v or RANGE.search(v):
            src_ranged.append((os.path.relpath(p, repo.roots[0]), v or '(none: latest)'))
    for p, t in repo.text.items():
        if 'chart:' not in t or 'targetRevision' not in t:
            continue
        for m in re.finditer(r'targetRevision\s*:\s*(?:"([^"]+)"|\'([^\']+)\'|(\S+))', t):
            v = next(g for g in m.groups() if g)
            if RANGE.search(v) and '{{' not in v:
                src_ranged.append((os.path.relpath(p, repo.roots[0]), v))
    return dict(charts_with_deps=charts, charts_locked=locked,
                lock_gitignored=ignored, ranged_deps=len(ranged),
                ranged_examples=ranged[:5], ranged_sources=len(src_ranged),
                ranged_source_examples=src_ranged[:5])


# ------------------------------------------------------------------ output

W = 76
def hr(c='-'): print(c * W)

def report(res, resolve=False):
    hr('=')
    print("  SEVEN QUESTIONS TO ASK IF YOU ARE USING GIT AS A DATABASE")
    hr('=')
    print("  Read-only. Nothing was sent anywhere. These are questions, not")
    print("  findings: several answers will turn out to be deliberate and correct.")
    print("  The exercise is whether you can find out quickly.")
    print()
    print("    1.  Inputs were approved, but outputs are deployed.")
    print("    2.  Your primary key is a filename.")
    print("    3.  Promotion by find-and-replace.")
    print("    4.  Is my change in progress, or abandoned?")
    print("    5.  One value was edited. How many values changed?")
    print("    6.  Safer, or just rarer?")
    print("    7.  Allowed to differ, or just differing?")

    r = res['q1']
    print(); hr(); print("Q1  Are you approving inputs, but deploying outputs?"); hr()
    print(f"    templated sources ................. {r['templated_sources']}")
    print(f"    rendered manifests in repo ........ {r['rendered_manifests']}")
    print(f"    definitions using Source Hydrator . {r['hydrated_definitions']}")
    if r['hydrated_definitions']:
        print("\n    Hydrated definitions store their render on a branch, so the output")
        print("    exists. How far one input change reaches is still not recorded: see Q5.")
    if r['gap']:
        print("\n    Reviewers see inputs. Clusters receive outputs. Nothing here")
        print("    shows the second. How does a reviewer know what a values change")
        print("    will actually produce?")

    r = res['q2']
    print(); hr(); print("Q2  Is your primary key a filename?"); hr()
    if r['schemes']:
        s = r['schemes'][0]
        print(f"    largest path-encoded key .......... {s['fields']} fields, "
              f"{s['rows']} rows  ({s['dir']}/)")
        print(f"    field cardinalities ............... {s['cardinalities']}")
        print(f"    distinct key schemes in repo ...... {r['distinct_schemes']}")
    if r['rename_cost']:
        print(f"    files touched to rename one row ... median {r['rename_cost']['median']}, "
              f"max {r['rename_cost']['max']}")
    print(f"    sources with layered values ....... {r['sources_with_layers']}")
    print(f"    ...ignoring missing files ......... {r['ignore_missing']}  ({r['ignore_pct']}%)")
    print(f"    layer refs: literal / templated .... {r['literal_refs']} / {r['templated_refs']}")
    print(f"    literal refs matching no file ..... {r['literal_missing']}  (lower bound)")
    for ref in r['missing']:
        print(f"        {ref}")
    if r['templated_refs']:
        print("    templated refs cannot be resolved without rendering. examples:")
        for ref in r['uncheckable']:
            print(f"        {ref}")
    if r['ignore_pct'] > 50:
        print("\n    A typo, a deleted file and a deliberate default all render the")
        print("    same and emit no signal. Templated refs cannot be checked from")
        print("    disk. Which of yours currently resolve to nothing?")

    lg = res['q2b']
    if lg['dimensions']:
        bydim = {}
        for d in lg['dimensions']:
            b = bydim.setdefault(d['dimension'], d)
            if d['declared'] > b['declared']:
                bydim[d['dimension']] = d
        multi = lg.get('declared_in_many_places') or {}
        print()
        print("    SPRAWL LEDGER — variation in use vs variation declared")
        print(f"    {'dimension':<20}{'values in use':>14}{'most complete':>15}"
              f"{'declared in':>13}")
        for k, d in sorted(bydim.items(), key=lambda x: -x[1]['values_live']):
            n = multi.get(k, 1)
            print(f"    {k:<20}{d['values_live']:>14}"
                  f"{d['declared']:>10} ({d['directory'].split('/')[-1] or d['directory']})"
                  f"{n:>7} place{'s' if n > 1 else ''}")
        for k, d in sorted(bydim.items(), key=lambda x: -x[1]['values_live']):
            if d['undeclared']:
                w = ', '.join(f"{v} ({n})" for v, n in d['worst'])
                print(f"        {k}: even the most complete declaration has no entry for {w}")
        if multi:
            worst = max(multi.items(), key=lambda x: x[1])
            print()
            print(f"    '{worst[0]}' is declared in {worst[1]} separate places, each with a")
            print("    different subset of the values actually in use. Nothing reconciles them.")
        if lg['per_target_overrides']:
            print(f"\n    per-target override files: {lg['per_target_overrides']} "
                  f"across {lg['override_dirs']} directories")
        print()
        print("    Targets genuinely differ, and variety is legitimate. Sprawl is variety")
        print("    nobody declared. A dimension declared in many places with different")
        print("    subsets, or with values in use that no declaration mentions, is not")
        print("    necessarily wrong: some of those charts only ever apply to a subset.")
        print("    The question is whether anyone decided that, or whether adding a value")
        print("    simply succeeded and nothing ever asked.")

    r = res['q34']
    print(); hr(); print("Q3  Is your promotion a find-and-replace?"); hr()
    print(f"    definitions repeating one version . {r['definitions_repeating']}")
    print(f"    repeats per definition ............ median {r['repeat_median']}, "
          f"max {r['repeat_max']}")
    if r['worst_repeat']:
        print(f"    worst ............................. {r['worst_repeat']}")
    print("\n    A promotion is a state transition. Here it is a text edit repeated")
    print("    once per target group, with nothing recording that it happened.")

    print(); hr(); print("Q4  Can you tell 'in progress' from 'abandoned'?"); hr()
    print(f"    definitions holding 2+ versions ... {r['definitions_multi_version']}")
    for n, p in r['worst_skew']:
        print(f"        {n} versions live   {p}")
    print("\n    A rollout 84% done and one abandoned last year look identical here.")
    print("    No start, no target, no completion criterion. For each of the above:")
    print("    in progress, or stopped?")

    r = res['q5']
    print(); hr(); print("Q5  If you edit just one value, can you say exactly how many");
    print("    values will be changed?"); hr()
    print(f"    deploy targets discovered ......... {r['targets']}")
    print(f"    definitions ....................... {r['definitions']}")
    print(f"    ...scout could not resolve ........ {r['unresolved_definitions']}")
    print(f"    ...resolved only in part .......... {r['partially_resolved']}  (reach is a lower bound)")
    print(f"    resolved instances ................ {r['instances']}")
    print(f"    reach per definition .............. median {r['reach_median']}, "
          f"max {r['reach_max']}")
    if r['reach_by_path']:
        print("    reach by changed path:")
        for path, n in r['reach_by_path']:
            print(f"        {n:>8}  instances re-render when {path} changes")
    if r['unresolved_definitions']:
        print(f"\n    scout could not work out what {r['unresolved_definitions']} of your "
              f"definitions target.")
        print("    It refuses to guess. Your review process has the same problem and")
        print("    does not say so.")

    r = res['q6']
    print(); hr(); print("Q6  Do your controls make changes safer, or just rarer?"); hr()
    print(f"    quota-shaped controls found ....... {r['count']}")
    for k in r['detail']:
        loc, snip = r['where'][k]
        print(f"        {k}")
        print(f"            {loc}:  {snip}")
    print("\n    scout only inspects CI, policy and bot config. Rollout waves,")
    print("    patch cadence and release calendars live elsewhere: add them yourself.")
    if r['count'] >= 2:
        print("\n    A quota limits how often you change things. A check limits what a")
        print("    change may do. Which of the above is a check?")


    r = res['q7']
    if r['definitions']:
        print(); hr(); print("Q7  Allowed to differ, or just differing?"); hr()
        print("    Drift itself is not visible from a repository: it is the difference")
        print("    between this repo and the running system. What IS here is the")
        print("    tolerance — and that splits into declared and merely allowed.")
        print()
        order = ['self-heal + prune', 'self-heal, no prune',
                 'automated, no self-heal', 'no automated sync']
        meaning = {'self-heal + prune': 'edits revert, deletions removed',
                   'self-heal, no prune': 'edits revert, deletions linger',
                   'automated, no self-heal': 'manual edits persist',
                   'no automated sync': 'everything persists until someone syncs'}
        print(f"    {'reconciliation posture':<28}{'definitions':>12}   effect")
        for k in order:
            v = r['posture'].get(k, 0)
            if v:
                print(f"    {k:<28}{v:>12}   {meaning[k]}")
        if r['declared']:
            print("\n    declared tolerance:")
            for k, v in sorted(r['declared'].items(), key=lambda x: -x[1]):
                print(f"        {v:>6}  {k}")
        if r['top_fields']:
            print("    most-declared fields:")
            for f, v in r['top_fields']:
                print(f"        {v:>6}  {f}")
        if r['flux']:
            print("    flux:", ', '.join(f"{k}: {v}" for k, v in r['flux'].items()))
        sh = r['posture'].get('self-heal, no prune', 0)
        pf = r['declared'].get('Prune=false', 0)
        if sh:
            print()
            print(f"    {sh} definitions self-heal but do not prune. {pf} of them say")
            print("    Prune=false explicitly, which is a decision. For the rest, is a")
            print("    resource deleted from desired state meant to stay on the cluster,")
            print("    or did nobody decide? Nothing here records which.")
        print()
        print("    Tolerance is not drift. A repo can declare perfect discipline and")
        print("    still have targets diverged. Confirming that needs the cluster.")

    r = res['q7b']
    print(); hr(); print("Q7b Where does observed data land, and what stops hand edits?"); hr()
    if r['dirs']:
        for d in r['dirs']:
            g = ', '.join(d['guarded_by']) if d['guarded_by'] else 'NOT OBSERVED'
            print(f"    {d['dir'] + '/':<38}{d['files']:>6} files   guard: {g}")
    else:
        print("    no directories with observed-data names found")
    print(f"    files marked generated / do-not-edit  {r['marked_files']}")
    print("\n    A guard is any CI, policy or ownership file that names the directory.")
    print("    scout cannot tell what the guard does. Does it block a hand edit, or")
    print("    only review one?")

    r = res['q7c']
    print(); hr(); print("Q7c Do your charts pin their dependencies?"); hr()
    print(f"    charts with dependencies .......... {r['charts_with_deps']}")
    print(f"    ...with Chart.lock committed ...... {r['charts_locked']}")
    print(f"    Chart.lock in .gitignore .......... {'yes' if r['lock_gitignored'] else 'no'}")
    print(f"    dependencies given as a range ..... {r['ranged_deps']}")
    for p, n, v in r['ranged_examples']:
        print(f"        {n} {v}   {p}")
    print(f"    chart sources given as a range .... {r['ranged_sources']}")
    for p, v in r['ranged_source_examples']:
        print(f"        {v}   {p}")
    if r['ranged_deps'] and r['charts_locked'] < r['charts_with_deps']:
        print("\n    A range with no committed lock resolves at render time. If a new")
        print("    version is published mid-rollout, which clusters got which? This may")
        print("    be deliberate, with the pin held elsewhere (a mirror, a registry gate).")
        print("    Is it written down?")

    print(); hr('=')
    print("  Cost per change scales with fleet size. Change rate is rising.")
    print("  If every control you have is a quota, the ceiling comes to you.")
    hr('=')


def main(argv):
    if hasattr(signal, 'SIGPIPE'):          # exit quietly when piped to head
        signal.signal(signal.SIGPIPE, signal.SIG_DFL)
    resolve = '--resolve' in argv
    as_json = '--json' in argv
    roots = [a for a in argv if not a.startswith('--')]
    if not roots:
        sys.exit(__doc__)
    repo = Repo(roots)
    q5 = q5_reach(repo)
    res = dict(q1=q1_render_gap(repo),
               q2=q2_keys_in_paths(repo, resolve),
               q2b=q2b_sprawl_ledger(repo, q5.pop('_inv', {})),
               q34=q34_promotion(repo),
               q5=q5,
               q6=q6_controls(repo),
               q7=q7_drift_tolerance(repo),
               q7b=q7b_observed_data(repo),
               q7c=q7c_dependency_pinning(repo))
    if as_json:
        print(json.dumps(res, indent=2, default=str))
    else:
        report(res, resolve)


if __name__ == '__main__':
    main(sys.argv[1:])
