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

// ConfigHub's recommended Space labels (see `cub variant create --help`).
// Fleets are selected by these, not by app-specific labels — a Space's
// Environment/Region/Component etc. identify which clusters a change lands on.
const STANDARD_LABELS = ['Component', 'Environment', 'Region', 'Owner', 'Layer', 'Variant'] as const;
type StdLabel = (typeof STANDARD_LABELS)[number];
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

type LabelSelection = Record<StdLabel, string[]>;
const EMPTY_SELECTION: LabelSelection = {
  Component: [],
  Environment: [],
  Region: [],
  Owner: [],
  Layer: [],
  Variant: [],
};

/** Standard-label selection → a server-side `where` over Space labels.
 * Units inherit their cluster's Space labels, so the fleet is selected by
 * `Space.Labels.<L> IN (...)`, ANDed across the chosen dimensions. */
function buildWhere(selection: LabelSelection): string {
  const clauses = STANDARD_LABELS.filter((l) => selection[l].length > 0).map(
    (l) => `Space.Labels.${l} IN (${selection[l].map((v) => `'${v}'`).join(', ')})`,
  );
  return ["ToolchainType = 'Kubernetes/YAML'", ...clauses].join(' AND ');
}

/** True when the Space's labels satisfy every chosen dimension (mirror of buildWhere). */
function matchesSelection(labels: Record<string, string> | undefined, selection: LabelSelection): boolean {
  return STANDARD_LABELS.every(
    (l) => selection[l].length === 0 || selection[l].includes(labels?.[l] ?? ''),
  );
}

export function FleetPage() {
  const { snapshot, isLoading } = useSnapshot();
  const [invokeOrg, invokeState] = useInvokeFunctionsOnOrgMutation();
  const [bulkPatch, bulkPatchState] = useBulkPatchUnitsMutation();

  // Discover the standard label values actually present on in-scope Spaces, so
  // the selector only offers dimensions/values that exist in this org.
  const labelValues = useMemo(() => {
    const m = new Map<StdLabel, Set<string>>(STANDARD_LABELS.map((l) => [l, new Set<string>()]));
    for (const eu of snapshot?.units.values() ?? []) {
      const labels = eu.Space?.Labels ?? {};
      for (const l of STANDARD_LABELS) {
        const v = labels[l];
        if (v !== undefined && v !== '') m.get(l)!.add(v);
      }
    }
    return m;
  }, [snapshot]);
  const activeLabels = useMemo(
    () => STANDARD_LABELS.filter((l) => (labelValues.get(l)?.size ?? 0) > 0),
    [labelValues],
  );

  const [selection, setSelection] = useState<LabelSelection>(EMPTY_SELECTION);
  const [customWhere, setCustomWhere] = useState('');
  const [op, setOp] = useState<'add-verb' | 'remove-verb'>('add-verb');
  const [roleKind, setRoleKind] = useState('ClusterRole');
  const [roleName, setRoleName] = useState('');
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

  // The standard-label selector covers the common case; the custom expression
  // is the escape hatch for anything it can't express.
  const usingCustom = customWhere.trim() !== '';
  const anyLabelSelected = STANDARD_LABELS.some((l) => selection[l].length > 0);
  const where = usingCustom
    ? `ToolchainType = 'Kubernetes/YAML' AND (${customWhere.trim()})`
    : buildWhere(selection);
  const matched = usingCustom
    ? [...snapshot.units.values()]
    : [...snapshot.units.values()].filter((eu) => matchesSelection(eu.Space?.Labels, selection));

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
    // "Behind upstream" comes from the snapshot: a clone is behind when its
    // UpstreamRevisionNum trails the current head of the unit it was cloned
    // from (looked up by UpstreamUnitID). Units whose upstream is out of scope
    // can't be assessed and are left out of the preview.
    const behind = matched.filter((eu) => {
      const upstreamId = eu.Unit?.UpstreamUnitID;
      if (upstreamId === undefined || upstreamId === null) return false;
      const upstream = snapshot.units.get(upstreamId)?.Unit;
      if (upstream?.HeadRevisionNum === undefined) return false;
      return (eu.Unit?.UpstreamRevisionNum ?? 0) < upstream.HeadRevisionNum;
    });
    if (behind.length === 0) {
      setMessage({
        kind: 'info',
        text: 'No matched units are behind their upstream. (A clone can also show no changes when a local override covers the upstream change, or when its upstream is out of scope.)',
      });
      return;
    }
    setChangeDesc(`Propagate upstream changes to ${behind.length} unit(s)`);
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
        {activeLabels.length === 0 && (
          <Typography variant='body2' color='text.secondary'>
            No standard Space labels (Component, Environment, Region, Owner, Layer, Variant) found in
            scope — use the custom selector below.
          </Typography>
        )}
        {activeLabels.map((l) => {
          const values = [...(labelValues.get(l) ?? [])].sort();
          return (
            <FormControl key={l} size='small' sx={{ minWidth: 170 }} disabled={usingCustom}>
              <InputLabel>{l}</InputLabel>
              <Select
                multiple
                label={l}
                value={selection[l]}
                onChange={(e) =>
                  setSelection({
                    ...selection,
                    [l]: typeof e.target.value === 'string' ? e.target.value.split(',') : e.target.value,
                  })
                }
                renderValue={(sel) => (sel as string[]).join(', ')}
              >
                {values.map((v) => (
                  <MenuItem key={v} value={v}>
                    <Checkbox checked={selection[l].includes(v)} size='small' sx={{ py: 0 }} />
                    {v}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
          );
        })}
        {!usingCustom && <Chip label={`${matched.length} unit(s) match`} variant='outlined' />}
      </Stack>
      <TextField
        fullWidth
        size='small'
        label='Custom unit selector (where expression — overrides the label selector)'
        placeholder="e.g. Labels.Team = 'payments' AND Slug LIKE '%-rbac'"
        value={customWhere}
        onChange={(e) => setCustomWhere(e.target.value)}
        sx={{ mb: 1 }}
      />
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
          placeholder='e.g. cluster-admin'
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
          disabled={
            (!usingCustom && !anyLabelSelected) ||
            matched.length === 0 ||
            roleName === '' ||
            invokeState.isLoading
          }
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
        disabled={(!usingCustom && !anyLabelSelected) || matched.length === 0 || bulkPatchState.isLoading}
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
