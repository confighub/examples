// Form-driven RBAC edits that resolve to shared, parameterized set-yq Invocations.
// The client never re-serializes YAML: it parses only to populate the pickers, the
// mutations themselves run in ConfigHub's function executor (preserving comments and
// formatting), and the preview diff comes from a dry run.
//
// Nothing in this panel writes. Each control adds to a list of pending changes, and the
// whole list is reviewed and applied together. Narrowing an over-broad role means removing
// the permissive rule and adding the rules that replace it, and that is one decision — it
// should be one dry run, one diff, and one revision, not a sequence of commits each of
// which leaves the role in a worse state than the last.

import {
  Box,
  Button,
  Chip,
  Divider,
  FormControl,
  IconButton,
  InputLabel,
  MenuItem,
  Paper,
  Select,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  TextField,
  Typography,
} from '@mui/material';
import { useEffect, useMemo, useState } from 'react';
import { parseAllDocuments } from 'yaml';

import {
  CompiledEdit,
  RuleSpec,
  compileAddRule,
  compileAddSubject,
  compileAddVerb,
  compileRemoveRule,
  compileRemoveSubject,
  compileRemoveVerb,
  compileSetRule,
  orderEdits,
} from '../rbac/edits';

interface RoleInfo {
  kind: 'Role' | 'ClusterRole';
  name: string;
  rules: RuleSpec[];
}

interface SubjectInfo {
  kind: string;
  name: string;
  namespace?: string;
}

interface BindingInfo {
  kind: 'RoleBinding' | 'ClusterRoleBinding';
  name: string;
  subjects: SubjectInfo[];
}

interface ParsedUnit {
  roles: RoleInfo[];
  bindings: BindingInfo[];
}

function asStrings(v: unknown): string[] {
  return Array.isArray(v) ? v.filter((x): x is string => typeof x === 'string') : [];
}

function parseUnitYaml(text: string): ParsedUnit {
  const roles: RoleInfo[] = [];
  const bindings: BindingInfo[] = [];
  for (const doc of parseAllDocuments(text)) {
    const obj = doc.toJS() as {
      kind?: string;
      metadata?: { name?: string };
      rules?: unknown;
      subjects?: { kind?: string; name?: string; namespace?: string }[];
    } | null;
    const name = obj?.metadata?.name;
    if (!obj || typeof name !== 'string') continue;
    if (obj.kind === 'Role' || obj.kind === 'ClusterRole') {
      const rules = Array.isArray(obj.rules) ? obj.rules : [];
      roles.push({
        kind: obj.kind,
        name,
        rules: rules.map((r) => {
          const rec = (typeof r === 'object' && r !== null ? r : {}) as Record<string, unknown>;
          return {
            apiGroups: asStrings(rec.apiGroups),
            resources: asStrings(rec.resources),
            verbs: asStrings(rec.verbs),
          };
        }),
      });
    } else if (obj.kind === 'RoleBinding' || obj.kind === 'ClusterRoleBinding') {
      const subjects = (Array.isArray(obj.subjects) ? obj.subjects : [])
        .filter((s) => typeof s.kind === 'string' && typeof s.name === 'string')
        .map((s) => ({ kind: s.kind ?? '', name: s.name ?? '', namespace: s.namespace }));
      bindings.push({ kind: obj.kind, name, subjects });
    }
  }
  return { roles, bindings };
}

const ALL_VERBS = [
  'get',
  'list',
  'watch',
  'create',
  'update',
  'patch',
  'delete',
  'deletecollection',
];

const VERB_PRESETS: { label: string; verbs: string[] }[] = [
  { label: 'read', verbs: ['get', 'list', 'watch'] },
  { label: 'write', verbs: ['create', 'update', 'patch', 'delete'] },
  { label: 'read + patch', verbs: ['get', 'list', 'watch', 'patch'] },
];

/** Comma-separated field text to a token list, preserving the empty core-group entry. */
function parseTokens(text: string, emptyMeansCore: boolean): string[] {
  const trimmed = text.trim();
  if (trimmed === '') return emptyMeansCore ? [''] : [];
  return trimmed
    .split(',')
    .map((t) => t.trim())
    .filter((t) => t !== '');
}

function formatTokens(values: string[]): string {
  // The core API group is the empty string; showing it as an empty field is the same
  // convention Kubernetes manifests use.
  return values.filter((v) => v !== '').join(', ');
}

interface RuleFormProps {
  initial?: RuleSpec;
  onCancel: () => void;
  onSubmit: (rule: RuleSpec) => void;
  submitLabel: string;
}

/** Author one policy rule: which API groups, which resources, which verbs. */
function RuleForm({ initial, onCancel, onSubmit, submitLabel }: RuleFormProps) {
  const [apiGroups, setApiGroups] = useState(formatTokens(initial?.apiGroups ?? []));
  const [resources, setResources] = useState(formatTokens(initial?.resources ?? []));
  const [verbs, setVerbs] = useState(formatTokens(initial?.verbs ?? []));

  const rule: RuleSpec = {
    apiGroups: parseTokens(apiGroups, true),
    resources: parseTokens(resources, false),
    verbs: parseTokens(verbs, false),
  };
  const valid = rule.resources.length > 0 && rule.verbs.length > 0;

  return (
    <Box sx={{ mt: 1 }}>
      <Stack spacing={1.5}>
        <TextField
          size='small'
          label='API groups'
          value={apiGroups}
          onChange={(e) => setApiGroups(e.target.value)}
          helperText='Comma-separated. Leave empty for the core API group (pods, services, secrets…).'
        />
        <TextField
          size='small'
          label='Resources'
          value={resources}
          onChange={(e) => setResources(e.target.value)}
          helperText='Comma-separated, plural and lowercase, e.g. applications, pods/log.'
        />
        <Box>
          <TextField
            size='small'
            fullWidth
            label='Verbs'
            value={verbs}
            onChange={(e) => setVerbs(e.target.value)}
            helperText='Comma-separated.'
          />
          <Stack direction='row' spacing={0.5} sx={{ mt: 0.5 }} useFlexGap flexWrap='wrap'>
            {VERB_PRESETS.map((p) => (
              <Chip
                key={p.label}
                size='small'
                variant='outlined'
                label={p.label}
                onClick={() => setVerbs(p.verbs.join(', '))}
              />
            ))}
            {ALL_VERBS.map((v) => (
              <Chip
                key={v}
                size='small'
                variant='outlined'
                label={`+${v}`}
                onClick={() =>
                  setVerbs((cur) => {
                    const have = parseTokens(cur, false);
                    return have.includes(v) ? cur : [...have, v].join(', ');
                  })
                }
              />
            ))}
          </Stack>
        </Box>
        <Stack direction='row' spacing={1}>
          <Button size='small' variant='contained' disabled={!valid} onClick={() => onSubmit(rule)}>
            {submitLabel}
          </Button>
          <Button size='small' onClick={onCancel}>
            Cancel
          </Button>
        </Stack>
      </Stack>
    </Box>
  );
}

/** Rules of the selected role, each removable or replaceable. */
function RuleEditor({ role, onPending }: { role: RoleInfo; onPending: (edit: CompiledEdit) => void }) {
  const [editing, setEditing] = useState<number | 'new' | null>(null);

  // Selecting a different role invalidates any open form's rule index.
  useEffect(() => setEditing(null), [role.kind, role.name]);

  return (
    <Box>
      <Table size='small'>
        <TableHead>
          <TableRow>
            <TableCell>#</TableCell>
            <TableCell>API groups</TableCell>
            <TableCell>Resources</TableCell>
            <TableCell>Verbs</TableCell>
            <TableCell />
          </TableRow>
        </TableHead>
        <TableBody>
          {role.rules.map((rule, i) => (
            <TableRow key={i}>
              <TableCell>{i}</TableCell>
              <TableCell>{rule.apiGroups.map((g) => (g === '' ? 'core' : g)).join(', ')}</TableCell>
              <TableCell>{rule.resources.join(', ')}</TableCell>
              <TableCell>{rule.verbs.join(', ')}</TableCell>
              <TableCell align='right'>
                <Button size='small' onClick={() => setEditing(i)}>
                  Replace…
                </Button>
                <Button
                  size='small'
                  color='warning'
                  onClick={() => onPending(compileRemoveRule(role.kind, role.name, i))}
                >
                  Remove…
                </Button>
              </TableCell>
            </TableRow>
          ))}
          {role.rules.length === 0 && (
            <TableRow>
              <TableCell colSpan={5}>
                <Typography variant='body2' color='text.secondary'>
                  This role grants no permissions of its own.
                </Typography>
              </TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>

      {editing === null ? (
        <Button size='small' sx={{ mt: 1 }} onClick={() => setEditing('new')}>
          + Add rule
        </Button>
      ) : (
        <RuleForm
          // Remount when the target rule changes: the form seeds its fields on mount.
          key={`${role.kind}/${role.name}#${editing}`}
          initial={typeof editing === 'number' ? role.rules[editing] : undefined}
          submitLabel='Add to pending changes'
          onCancel={() => setEditing(null)}
          onSubmit={(rule) => {
            onPending(
              typeof editing === 'number'
                ? compileSetRule(role.kind, role.name, editing, rule)
                : compileAddRule(role.kind, role.name, rule),
            );
            setEditing(null);
          }}
        />
      )}
    </Box>
  );
}

/** Subjects of the selected binding, plus a form to add one. */
function SubjectEditor({
  binding,
  onPending,
}: {
  binding: BindingInfo;
  onPending: (edit: CompiledEdit) => void;
}) {
  const [subjectKind, setSubjectKind] = useState('Group');
  const [subjectName, setSubjectName] = useState('');
  const [subjectNs, setSubjectNs] = useState('');

  return (
    <Box>
      <Table size='small'>
        <TableHead>
          <TableRow>
            <TableCell>Kind</TableCell>
            <TableCell>Name</TableCell>
            <TableCell>Namespace</TableCell>
            <TableCell />
          </TableRow>
        </TableHead>
        <TableBody>
          {binding.subjects.map((s, i) => (
            <TableRow key={`${s.kind}:${s.name}:${i}`}>
              <TableCell>{s.kind}</TableCell>
              <TableCell>{s.name}</TableCell>
              <TableCell>{s.namespace ?? ''}</TableCell>
              <TableCell align='right'>
                <Button
                  size='small'
                  color='warning'
                  onClick={() =>
                    onPending(
                      compileRemoveSubject(
                        binding.kind,
                        binding.name,
                        s.kind,
                        s.name,
                        s.namespace,
                      ),
                    )
                  }
                >
                  Remove…
                </Button>
              </TableCell>
            </TableRow>
          ))}
          {binding.subjects.length === 0 && (
            <TableRow>
              <TableCell colSpan={4}>
                <Typography variant='body2' color='text.secondary'>
                  No subjects — this binding grants nothing.
                </Typography>
              </TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>

      <Stack direction='row' spacing={1} sx={{ mt: 1 }} alignItems='flex-start'>
        <FormControl size='small' sx={{ minWidth: 150 }}>
          <InputLabel>Subject kind</InputLabel>
          <Select
            label='Subject kind'
            value={subjectKind}
            onChange={(e) => setSubjectKind(e.target.value)}
          >
            {['Group', 'User', 'ServiceAccount'].map((k) => (
              <MenuItem key={k} value={k}>
                {k}
              </MenuItem>
            ))}
          </Select>
        </FormControl>
        <TextField
          size='small'
          label='Subject name'
          value={subjectName}
          onChange={(e) => setSubjectName(e.target.value)}
        />
        {subjectKind === 'ServiceAccount' && (
          <TextField
            size='small'
            label='Namespace'
            value={subjectNs}
            onChange={(e) => setSubjectNs(e.target.value)}
          />
        )}
        <Button
          size='small'
          disabled={subjectName.trim() === ''}
          onClick={() => {
            onPending(
              compileAddSubject(
                binding.kind,
                binding.name,
                subjectKind,
                subjectName.trim(),
                subjectKind === 'ServiceAccount' ? subjectNs.trim() : undefined,
              ),
            );
            setSubjectName('');
          }}
        >
          Add subject
        </Button>
      </Stack>
    </Box>
  );
}

/** Add or remove a single verb on one rule, without rewriting the rest of it. */
function VerbEditor({ role, onPending }: { role: RoleInfo; onPending: (edit: CompiledEdit) => void }) {
  const [ruleIdx, setRuleIdx] = useState(0);
  const [verb, setVerb] = useState('delete');
  useEffect(() => setRuleIdx(0), [role.kind, role.name]);
  if (role.rules.length === 0) return null;
  const idx = Math.min(ruleIdx, role.rules.length - 1);

  return (
    <Stack direction='row' spacing={1} alignItems='center' sx={{ mt: 1 }}>
      <FormControl size='small' sx={{ minWidth: 90 }}>
        <InputLabel>Rule</InputLabel>
        <Select label='Rule' value={idx} onChange={(e) => setRuleIdx(Number(e.target.value))}>
          {role.rules.map((_, i) => (
            <MenuItem key={i} value={i}>
              {i}
            </MenuItem>
          ))}
        </Select>
      </FormControl>
      <FormControl size='small' sx={{ minWidth: 150 }}>
        <InputLabel>Verb</InputLabel>
        <Select label='Verb' value={verb} onChange={(e) => setVerb(e.target.value)}>
          {ALL_VERBS.map((v) => (
            <MenuItem key={v} value={v}>
              {v}
            </MenuItem>
          ))}
        </Select>
      </FormControl>
      <Button size='small' onClick={() => onPending(compileAddVerb(role.kind, role.name, idx, verb))}>
        Add verb
      </Button>
      <Button
        size='small'
        color='warning'
        onClick={() => onPending(compileRemoveVerb(role.kind, role.name, idx, verb))}
      >
        Remove verb
      </Button>
    </Stack>
  );
}

export interface StructuredEditProps {
  /** Current unit YAML (for pickers only — never written back from here). */
  yamlText: string;
  /** Kind and name of the resource to open on, from the deep link that led here. */
  focus?: { kind: string; name: string };
  /** Review, then commit, the pending edits as one batch. */
  onReview: (edits: CompiledEdit[]) => void;
}

/** Quick edits: pick the changes, we resolve them to parameterized Invocations. */
export function StructuredEdit({ yamlText, focus, onReview }: StructuredEditProps) {
  const parsed = useMemo(() => parseUnitYaml(yamlText), [yamlText]);
  const targets = useMemo(
    () => [
      ...parsed.roles.map((r) => ({ key: `${r.kind}/${r.name}`, role: r, binding: undefined })),
      ...parsed.bindings.map((b) => ({ key: `${b.kind}/${b.name}`, role: undefined, binding: b })),
    ],
    [parsed],
  );

  const [selectedKey, setSelectedKey] = useState('');
  const [pending, setPending] = useState<CompiledEdit[]>([]);

  // Open on the resource the caller linked to, falling back to the first one. Depends on
  // `targets` because the Unit's configuration is fetched after the first render.
  useEffect(() => {
    if (targets.length === 0) return;
    const wanted =
      focus === undefined
        ? undefined
        : targets.find(
            (t) =>
              (t.role?.kind ?? t.binding?.kind) === focus.kind &&
              (t.role?.name ?? t.binding?.name) === focus.name,
          );
    setSelectedKey((cur) =>
      wanted !== undefined ? wanted.key : targets.some((t) => t.key === cur) ? cur : targets[0].key,
    );
  }, [targets, focus?.kind, focus?.name]);

  const selected = targets.find((t) => t.key === selectedKey);
  const addPending = (edit: CompiledEdit) => setPending((cur) => [...cur, edit]);

  if (targets.length === 0) return null;

  return (
    <Paper variant='outlined' sx={{ p: 2, mb: 2 }}>
      <Typography variant='h6'>Quick edit</Typography>
      <Typography variant='body2' color='text.secondary' sx={{ mb: 2 }}>
        Nothing here is written yet. Each change is collected below, and the whole set is
        reviewed and applied together as one revision.
      </Typography>

      <FormControl size='small' sx={{ minWidth: 320 }}>
        <InputLabel>Resource</InputLabel>
        <Select
          label='Resource'
          value={selected ? selectedKey : ''}
          onChange={(e) => setSelectedKey(e.target.value)}
        >
          {targets.map((t) => (
            <MenuItem key={t.key} value={t.key}>
              {t.key}
            </MenuItem>
          ))}
        </Select>
      </FormControl>

      {selected?.role !== undefined && (
        <Box sx={{ mt: 2 }}>
          <Typography variant='subtitle2'>Rules</Typography>
          <RuleEditor role={selected.role} onPending={addPending} />
          <Typography variant='subtitle2' sx={{ mt: 2 }}>
            Single verb
          </Typography>
          <VerbEditor role={selected.role} onPending={addPending} />
        </Box>
      )}
      {selected?.binding !== undefined && (
        <Box sx={{ mt: 2 }}>
          <Typography variant='subtitle2'>Subjects</Typography>
          <SubjectEditor binding={selected.binding} onPending={addPending} />
        </Box>
      )}

      {pending.length > 0 && (
        <>
          <Divider sx={{ my: 2 }} />
          <Stack direction='row' spacing={2} alignItems='center'>
            <Typography variant='subtitle2'>Pending changes ({pending.length})</Typography>
            <Button size='small' variant='contained' onClick={() => onReview(pending)}>
              Review changes…
            </Button>
            <Button size='small' onClick={() => setPending([])}>
              Discard all
            </Button>
          </Stack>
          <Typography variant='caption' color='text.secondary'>
            Applied in this order, as one revision. Rule removals run last so the rule
            numbers above keep meaning what they say.
          </Typography>
          <Stack spacing={0.5} sx={{ mt: 1 }}>
            {orderEdits(pending).map((edit, i) => (
              <Stack key={`${edit.slug}:${i}`} direction='row' spacing={1} alignItems='center'>
                <Chip size='small' label={i + 1} />
                <Typography variant='body2' sx={{ flexGrow: 1 }}>
                  {edit.summary}
                </Typography>
                <IconButton
                  size='small'
                  aria-label='discard this change'
                  onClick={() => setPending((cur) => cur.filter((e) => e !== edit))}
                >
                  ✕
                </IconButton>
              </Stack>
            ))}
          </Stack>
        </>
      )}
    </Paper>
  );
}
