// Form-driven RBAC edits that resolve to shared, parameterized set-yq
// Invocations. The client never re-serializes YAML: it parses only to populate
// the pickers, the mutation itself runs in ConfigHub's function executor
// (preserving comments and formatting), and the preview diff comes from a dry run.

import {
  Box,
  Button,
  FormControl,
  InputLabel,
  MenuItem,
  Paper,
  Select,
  Stack,
  TextField,
  Typography,
} from '@mui/material';
import { useMemo, useState } from 'react';
import { parseAllDocuments } from 'yaml';

import {
  CompiledEdit,
  compileAddSubject,
  compileAddVerb,
  compileRemoveSubject,
  compileRemoveVerb,
} from '../rbac/edits';

interface RoleInfo {
  kind: 'Role' | 'ClusterRole';
  name: string;
  /** One human summary per rule, indexed by rule position. */
  ruleSummaries: string[];
  verbsPerRule: string[][];
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

function parseUnitYaml(text: string): ParsedUnit {
  const roles: RoleInfo[] = [];
  const bindings: BindingInfo[] = [];
  for (const doc of parseAllDocuments(text)) {
    const obj = doc.toJS() as {
      kind?: string;
      metadata?: { name?: string };
      rules?: { verbs?: string[]; resources?: string[]; apiGroups?: string[] }[];
      subjects?: { kind?: string; name?: string; namespace?: string }[];
    } | null;
    const name = obj?.metadata?.name;
    if (!obj || typeof name !== 'string') continue;
    if (obj.kind === 'Role' || obj.kind === 'ClusterRole') {
      const rules = Array.isArray(obj.rules) ? obj.rules : [];
      roles.push({
        kind: obj.kind,
        name,
        ruleSummaries: rules.map(
          (r, i) =>
            `rule ${i}: [${(r.verbs ?? []).join(',')}] on [${(r.resources ?? []).join(',')}]`,
        ),
        verbsPerRule: rules.map((r) => r.verbs ?? []),
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

type Operation = 'add-verb' | 'remove-verb' | 'add-subject' | 'remove-subject';

export interface StructuredEditProps {
  /** Current unit YAML (for pickers only — never written back from here). */
  yamlText: string;
  /** Preview then commit the structured edit via its parameterized Invocation. */
  onPreview: (edit: CompiledEdit) => void;
}

/** Quick edits: pick the change, we resolve it to a parameterized Invocation. */
export function StructuredEdit({ yamlText, onPreview }: StructuredEditProps) {
  const parsed = useMemo(() => parseUnitYaml(yamlText), [yamlText]);
  const [op, setOp] = useState<Operation>('add-verb');
  const [roleIdx, setRoleIdx] = useState(0);
  const [ruleIdx, setRuleIdx] = useState(0);
  const [verb, setVerb] = useState('delete');
  const [bindingIdx, setBindingIdx] = useState(0);
  const [subjectKind, setSubjectKind] = useState('Group');
  const [subjectName, setSubjectName] = useState('');
  const [subjectNs, setSubjectNs] = useState('');
  const [removeSubjectIdx, setRemoveSubjectIdx] = useState(0);

  const role = parsed.roles[roleIdx];
  const binding = parsed.bindings[bindingIdx];

  // For remove-verb the choices are the rule's current verbs; keep the
  // selection valid when the operation, role, or rule changes.
  const removableVerbs = role?.verbsPerRule[ruleIdx] ?? [];
  const effectiveVerb =
    op === 'remove-verb' && !removableVerbs.includes(verb) ? (removableVerbs[0] ?? '') : verb;
  const removableSubject =
    binding?.subjects[removeSubjectIdx < (binding?.subjects.length ?? 0) ? removeSubjectIdx : 0];

  const compile = (): CompiledEdit | null => {
    if (op === 'add-verb' || op === 'remove-verb') {
      if (!role || role.ruleSummaries.length === 0) return null;
      if (op === 'remove-verb' && effectiveVerb === '') return null;
      return op === 'add-verb'
        ? compileAddVerb(role.kind, role.name, ruleIdx, verb)
        : compileRemoveVerb(role.kind, role.name, ruleIdx, effectiveVerb);
    }
    if (op === 'remove-subject') {
      if (!binding || !removableSubject) return null;
      return compileRemoveSubject(
        binding.kind,
        binding.name,
        removableSubject.kind,
        removableSubject.name,
        removableSubject.namespace,
      );
    }
    if (!binding || subjectName.trim() === '') return null;
    return compileAddSubject(
      binding.kind,
      binding.name,
      subjectKind,
      subjectName.trim(),
      subjectNs.trim(),
    );
  };

  const compiled = compile();

  return (
    <Paper variant='outlined' sx={{ p: 2, mb: 3 }}>
      <Typography variant='subtitle1' gutterBottom>
        Quick edit (runs server-side, format-preserving)
      </Typography>
      <Stack direction='row' spacing={2} useFlexGap flexWrap='wrap'>
        <FormControl size='small' sx={{ minWidth: 160 }}>
          <InputLabel>Operation</InputLabel>
          <Select label='Operation' value={op} onChange={(e) => setOp(e.target.value as Operation)}>
            <MenuItem value='add-verb'>Add verb</MenuItem>
            <MenuItem value='remove-verb'>Remove verb</MenuItem>
            <MenuItem value='add-subject'>Add subject</MenuItem>
            <MenuItem value='remove-subject'>Remove subject</MenuItem>
          </Select>
        </FormControl>

        {(op === 'add-verb' || op === 'remove-verb') && (
          <>
            <FormControl size='small' sx={{ minWidth: 220 }}>
              <InputLabel>Role</InputLabel>
              <Select
                label='Role'
                value={parsed.roles.length > 0 ? roleIdx : ''}
                onChange={(e) => {
                  setRoleIdx(Number(e.target.value));
                  setRuleIdx(0);
                }}
              >
                {parsed.roles.map((r, i) => (
                  <MenuItem key={i} value={i}>
                    {r.kind}/{r.name}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
            <FormControl size='small' sx={{ minWidth: 280 }}>
              <InputLabel>Rule</InputLabel>
              <Select
                label='Rule'
                value={role !== undefined && role.ruleSummaries.length > 0 ? ruleIdx : ''}
                onChange={(e) => setRuleIdx(Number(e.target.value))}
              >
                {(role?.ruleSummaries ?? []).map((s, i) => (
                  <MenuItem key={i} value={i}>
                    {s}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
            <FormControl size='small' sx={{ minWidth: 140 }}>
              <InputLabel>Verb</InputLabel>
              <Select
                label='Verb'
                value={effectiveVerb}
                onChange={(e) => setVerb(e.target.value)}
              >
                {(op === 'remove-verb' ? removableVerbs : ALL_VERBS).map((v) => (
                  <MenuItem key={v} value={v}>
                    {v}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
          </>
        )}

        {(op === 'add-subject' || op === 'remove-subject') && (
          <FormControl size='small' sx={{ minWidth: 260 }}>
            <InputLabel>Binding</InputLabel>
            <Select
              label='Binding'
              value={parsed.bindings.length > 0 ? bindingIdx : ''}
              onChange={(e) => {
                setBindingIdx(Number(e.target.value));
                setRemoveSubjectIdx(0);
              }}
            >
              {parsed.bindings.map((b, i) => (
                <MenuItem key={i} value={i}>
                  {b.kind}/{b.name}
                </MenuItem>
              ))}
            </Select>
          </FormControl>
        )}

        {op === 'remove-subject' && (
          <FormControl size='small' sx={{ minWidth: 260 }}>
            <InputLabel>Subject</InputLabel>
            <Select
              label='Subject'
              value={binding !== undefined && binding.subjects.length > 0 ? Math.min(removeSubjectIdx, binding.subjects.length - 1) : ''}
              onChange={(e) => setRemoveSubjectIdx(Number(e.target.value))}
            >
              {(binding?.subjects ?? []).map((s, i) => (
                <MenuItem key={i} value={i}>
                  {s.kind}:{s.namespace !== undefined ? `${s.namespace}/` : ''}
                  {s.name}
                </MenuItem>
              ))}
            </Select>
          </FormControl>
        )}

        {op === 'add-subject' && (
          <>
            <FormControl size='small' sx={{ minWidth: 160 }}>
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
          </>
        )}

        <Button
          variant='outlined'
          disabled={compiled === null}
          onClick={() => compiled && onPreview(compiled)}
        >
          Preview change
        </Button>
      </Stack>
      {compiled && (
        <Box component='code' sx={{ display: 'block', mt: 1, fontSize: 12, color: 'text.secondary' }}>
          {compiled.slug}(
          {Object.entries(compiled.params)
            .map(([k, v]) => `${k}=${v}`)
            .join(', ')}
          )
        </Box>
      )}
    </Paper>
  );
}
