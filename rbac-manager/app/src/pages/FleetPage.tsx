// Fleet operations: one structured edit applied across a selector of
// clusters in a single server-side bulk invocation, and base→downstream
// upgrade propagation. The selector compiles to a ConfigHub `where` clause —
// the server resolves membership, the app never loops over clusters.

import {
  Alert,
  Box,
  Button,
  Checkbox,
  Chip,
  CircularProgress,
  Container,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  FormControl,
  FormControlLabel,
  InputLabel,
  MenuItem,
  Select,
  Stack,
  TextField,
  Typography,
} from '@mui/material';
import { diffLines } from 'diff';
import { useMemo, useState } from 'react';

import { b64decodeUtf8 } from '../api/encoding';
import { fetchUnitDataText } from '../api/raw';
import { useSnapshot } from '../fleet/SnapshotContext';
import { CompiledEdit, compileAddVerb, compileRemoveVerb } from '../rbac/edits';
import {
  useBulkPatchUnitsMutation,
  useInvokeFunctionsOnOrgMutation,
} from '../sdk/confighubapi.gen';

const ENVS = ['dev', 'staging', 'prod'];
const VERBS = ['get', 'list', 'watch', 'create', 'update', 'patch', 'delete', 'deletecollection'];

interface UnitDiff {
  label: string;
  before: string;
  after: string;
}

function MultiDiff({ diffs }: { diffs: UnitDiff[] }) {
  return (
    <Stack spacing={2}>
      {diffs.map((d) => (
        <Box key={d.label}>
          <Typography variant='subtitle2'>{d.label}</Typography>
          <Box component='pre' sx={{ fontSize: 12, m: 0, overflow: 'auto', maxHeight: 220 }}>
            {diffLines(d.before, d.after).map((part, i) => (
              <Box
                key={i}
                component='span'
                sx={{
                  display: 'block',
                  whiteSpace: 'pre-wrap',
                  bgcolor: part.added ? '#e6ffec' : part.removed ? '#ffebe9' : 'transparent',
                }}
              >
                {part.value.replace(/\n$/, '')}
              </Box>
            ))}
          </Box>
        </Box>
      ))}
    </Stack>
  );
}

/** env multi-select + persona select → a server-side where clause. */
function buildWhere(persona: string, envs: string[]): string {
  const envList = envs.map((e) => `'${e}'`).join(', ');
  return `Labels.app = 'rbac-manager' AND Labels.persona = '${persona}' AND Labels.env IN (${envList})`;
}

export function FleetPage() {
  const { snapshot, isLoading } = useSnapshot();
  const [invokeOrg, invokeState] = useInvokeFunctionsOnOrgMutation();
  const [bulkPatch, bulkPatchState] = useBulkPatchUnitsMutation();

  const personas = useMemo(() => {
    const set = new Set<string>();
    for (const eu of snapshot?.units.values() ?? []) {
      const p = eu.Unit?.Labels?.persona;
      if (p !== undefined) set.add(p);
    }
    return [...set].sort();
  }, [snapshot]);

  const [persona, setPersona] = useState('developer');
  const [envs, setEnvs] = useState<string[]>(['dev', 'staging']);
  const [op, setOp] = useState<'add-verb' | 'remove-verb'>('add-verb');
  const [roleKind, setRoleKind] = useState('ClusterRole');
  const [roleName, setRoleName] = useState('rbac-manager-developer');
  const [ruleIdx, setRuleIdx] = useState(0);
  const [verb, setVerb] = useState('deletecollection');

  const [pending, setPending] = useState<{
    edit: CompiledEdit;
    where: string;
    diffs: UnitDiff[];
  } | null>(null);
  const [upgradePreview, setUpgradePreview] = useState<{ where: string; slugs: string[] } | null>(
    null,
  );
  const [changeDesc, setChangeDesc] = useState('');
  const [message, setMessage] = useState<{ kind: 'success' | 'error' | 'info'; text: string } | null>(
    null,
  );

  if (isLoading || !snapshot) {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', mt: 12 }}>
        <CircularProgress />
      </Box>
    );
  }

  const where = buildWhere(persona, envs);
  const matched = [...snapshot.units.values()].filter(
    (eu) =>
      eu.Unit?.Labels?.persona === persona &&
      envs.includes(eu.Unit?.Labels?.env ?? '') ,
  );

  const previewBulkEdit = async () => {
    setMessage(null);
    const edit =
      op === 'add-verb'
        ? compileAddVerb(roleKind, roleName, ruleIdx, verb)
        : compileRemoveVerb(roleKind, roleName, ruleIdx, verb);
    const result = await invokeOrg({
      where,
      dryRun: 'true',
      functionInvocationsRequest: {
        FunctionInvocations: [
          { FunctionName: 'yq-i', Arguments: [{ ParameterName: 'yq-expression', Value: edit.expr }] },
        ],
      },
    });
    if ('error' in result && result.error) {
      setMessage({ kind: 'error', text: 'Dry run failed.' });
      return;
    }
    const diffs: UnitDiff[] = [];
    for (const r of result.data ?? []) {
      if (!r.Success || !r.HasNewMutations || !r.UnitID || !r.SpaceID) continue;
      let before = '';
      try {
        before = await fetchUnitDataText(r.SpaceID, r.UnitID);
      } catch {
        // Fall through with an empty before; the diff degrades, not the flow.
      }
      diffs.push({
        label: `${r.SpaceSlug}/${r.UnitSlug}`,
        before,
        after: r.ConfigData !== undefined && r.ConfigData !== '' ? b64decodeUtf8(r.ConfigData) : '',
      });
    }
    if (diffs.length === 0) {
      setMessage({ kind: 'info', text: 'No changes: the edit is already in effect everywhere it matches.' });
      return;
    }
    setChangeDesc(edit.summary + ` across ${diffs.length} cluster(s)`);
    setPending({ edit, where, diffs });
  };

  const commitBulkEdit = async () => {
    if (pending === null) return;
    const result = await invokeOrg({
      where: pending.where,
      functionInvocationsRequest: {
        FunctionInvocations: [
          {
            FunctionName: 'yq-i',
            Arguments: [{ ParameterName: 'yq-expression', Value: pending.edit.expr }],
          },
        ],
        ChangeDescription: changeDesc,
      },
    });
    setPending(null);
    if ('error' in result && result.error) {
      setMessage({ kind: 'error', text: 'Bulk edit failed.' });
      return;
    }
    setMessage({
      kind: 'success',
      text: `Edit committed to ${(result.data ?? []).filter((r) => r.Success).length} unit(s). Refresh the snapshot to see it.`,
    });
  };

  const previewUpgrade = () => {
    setMessage(null);
    // "Behind upstream" comes from the snapshot: the clone's
    // UpstreamRevisionNum vs the base persona's current head. (A bulk
    // dry-run returns every matched unit whether or not it would change.)
    const baseHead = [...snapshot.units.values()].find(
      (eu) =>
        eu.Unit?.Labels?.persona === persona && eu.Space?.Labels?.role === 'base',
    )?.Unit?.HeadRevisionNum;
    const behind = matched.filter(
      (eu) =>
        baseHead !== undefined && (eu.Unit?.UpstreamRevisionNum ?? baseHead) < baseHead,
    );
    if (behind.length === 0) {
      setMessage({
        kind: 'info',
        text: 'All matched units are already based on the latest base revision. (A unit can also show no changes when a local override covers the upstream change.)',
      });
      return;
    }
    setChangeDesc(`Propagate base ${persona} changes to ${envs.join(', ')}`);
    setUpgradePreview({
      where,
      slugs: behind.map((eu) => `${eu.Space?.Slug}/${eu.Unit?.Slug}`),
    });
  };

  const commitUpgrade = async () => {
    if (upgradePreview === null) return;
    const result = await bulkPatch({
      where: upgradePreview.where,
      upgrade: true,
      body: { LastChangeDescription: changeDesc },
    });
    setUpgradePreview(null);
    if ('error' in result && result.error) {
      setMessage({ kind: 'error', text: 'Upgrade failed.' });
      return;
    }
    setMessage({ kind: 'success', text: 'Upgrade applied. Local divergence was preserved.' });
  };

  return (
    <Container maxWidth='lg' sx={{ mt: 3 }}>
      <Typography variant='h5' gutterBottom>
        Fleet operations
      </Typography>
      <Typography variant='body2' color='text.secondary' sx={{ mb: 2 }}>
        One change, a selector of clusters, one server-side operation. The selector compiles to a
        ConfigHub <code>where</code> clause; nothing here loops over clusters.
      </Typography>

      <Stack direction='row' spacing={2} useFlexGap flexWrap='wrap' alignItems='center' sx={{ mb: 1 }}>
        <FormControl size='small' sx={{ minWidth: 160 }}>
          <InputLabel>Persona</InputLabel>
          <Select
            label='Persona'
            value={persona}
            onChange={(e) => {
              setPersona(e.target.value);
              setRoleName(`rbac-manager-${e.target.value}`);
            }}
          >
            {personas.map((p) => (
              <MenuItem key={p} value={p}>
                {p}
              </MenuItem>
            ))}
          </Select>
        </FormControl>
        {ENVS.map((e) => (
          <FormControlLabel
            key={e}
            control={
              <Checkbox
                checked={envs.includes(e)}
                onChange={(ev) =>
                  setEnvs(ev.target.checked ? [...envs, e] : envs.filter((x) => x !== e))
                }
              />
            }
            label={e}
          />
        ))}
        <Chip label={`${matched.length} unit(s) match`} variant='outlined' />
      </Stack>
      <Box component='code' sx={{ display: 'block', fontSize: 12, color: 'text.secondary', mb: 3 }}>
        {where}
      </Box>

      {message !== null && (
        <Alert severity={message.kind} sx={{ mb: 2 }} onClose={() => setMessage(null)}>
          {message.text}
        </Alert>
      )}

      <Typography variant='h6' gutterBottom>
        Bulk edit
      </Typography>
      <Stack direction='row' spacing={2} useFlexGap flexWrap='wrap' sx={{ mb: 4 }}>
        <FormControl size='small' sx={{ minWidth: 150 }}>
          <InputLabel>Operation</InputLabel>
          <Select
            label='Operation'
            value={op}
            onChange={(e) => setOp(e.target.value as 'add-verb' | 'remove-verb')}
          >
            <MenuItem value='add-verb'>Add verb</MenuItem>
            <MenuItem value='remove-verb'>Remove verb</MenuItem>
          </Select>
        </FormControl>
        <FormControl size='small' sx={{ minWidth: 140 }}>
          <InputLabel>Role kind</InputLabel>
          <Select label='Role kind' value={roleKind} onChange={(e) => setRoleKind(e.target.value)}>
            <MenuItem value='ClusterRole'>ClusterRole</MenuItem>
            <MenuItem value='Role'>Role</MenuItem>
          </Select>
        </FormControl>
        <TextField
          size='small'
          label='Role name'
          value={roleName}
          onChange={(e) => setRoleName(e.target.value)}
        />
        <TextField
          size='small'
          type='number'
          label='Rule #'
          value={ruleIdx}
          onChange={(e) => setRuleIdx(Math.max(0, Number(e.target.value)))}
          sx={{ width: 90 }}
        />
        <FormControl size='small' sx={{ minWidth: 150 }}>
          <InputLabel>Verb</InputLabel>
          <Select label='Verb' value={verb} onChange={(e) => setVerb(e.target.value)}>
            {VERBS.map((v) => (
              <MenuItem key={v} value={v}>
                {v}
              </MenuItem>
            ))}
          </Select>
        </FormControl>
        <Button
          variant='contained'
          disabled={envs.length === 0 || roleName === '' || invokeState.isLoading}
          onClick={() => void previewBulkEdit()}
        >
          {invokeState.isLoading ? 'Previewing…' : 'Preview across fleet'}
        </Button>
      </Stack>

      <Divider sx={{ mb: 3 }} />

      <Typography variant='h6' gutterBottom>
        Propagate base changes
      </Typography>
      <Typography variant='body2' color='text.secondary' sx={{ mb: 1 }}>
        Upgrades the selected downstream units to the latest base revision. Intentional local
        divergence is preserved (override-aware merge).
      </Typography>
      <Button
        variant='outlined'
        disabled={envs.length === 0 || bulkPatchState.isLoading}
        onClick={() => void previewUpgrade()}
      >
        {bulkPatchState.isLoading ? 'Checking…' : 'Preview upgrade'}
      </Button>

      {/* Bulk edit commit dialog */}
      <Dialog open={pending !== null} onClose={() => setPending(null)} maxWidth='md' fullWidth>
        <DialogTitle>Commit fleet edit ({pending?.diffs.length} unit(s))</DialogTitle>
        <DialogContent>
          {pending && <MultiDiff diffs={pending.diffs} />}
          <TextField
            fullWidth
            size='small'
            label='Change description (required)'
            value={changeDesc}
            onChange={(e) => setChangeDesc(e.target.value)}
            sx={{ mt: 2 }}
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setPending(null)}>Cancel</Button>
          <Button
            variant='contained'
            disabled={changeDesc.trim() === '' || invokeState.isLoading}
            onClick={() => void commitBulkEdit()}
          >
            Commit
          </Button>
        </DialogActions>
      </Dialog>

      {/* Upgrade commit dialog */}
      <Dialog open={upgradePreview !== null} onClose={() => setUpgradePreview(null)} maxWidth='sm' fullWidth>
        <DialogTitle>Upgrade {upgradePreview?.slugs.length} unit(s) from base</DialogTitle>
        <DialogContent>
          <Typography variant='body2' sx={{ mb: 1 }}>
            Units to upgrade: {upgradePreview?.slugs.join(', ')}
          </Typography>
          <TextField
            fullWidth
            size='small'
            label='Change description (required)'
            value={changeDesc}
            onChange={(e) => setChangeDesc(e.target.value)}
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setUpgradePreview(null)}>Cancel</Button>
          <Button
            variant='contained'
            disabled={changeDesc.trim() === '' || bulkPatchState.isLoading}
            onClick={() => void commitUpgrade()}
          >
            Upgrade
          </Button>
        </DialogActions>
      </Dialog>
    </Container>
  );
}
