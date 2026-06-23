// Unit detail: the workload's images and scan verdict, the CVE findings the
// scanner recorded, Apply Gates, an "upgrade image" quick action (server-side
// yq-i, dry-run previewed), the raw YAML, and revision history with rollback.
// The client never re-serializes YAML — edits run in ConfigHub's executor.

import {
  Alert,
  Box,
  Button,
  Chip,
  CircularProgress,
  Container,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
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
  ToggleButton,
  ToggleButtonGroup,
  Typography,
} from '@mui/material';
import { diffLines } from 'diff';
import { useMemo, useState } from 'react';
import { useParams } from 'react-router-dom';
import { parseAllDocuments } from 'yaml';

import { b64decodeUtf8, b64encodeUtf8 } from '../api/encoding';
import { fetchRevisionDataText } from '../api/raw';
import { useSnapshot } from '../fleet/SnapshotContext';
import { compileSetImage } from '../sec/edits';
import { Finding, isStale } from '../sec/model';
import { severityColor, severityLabel } from '../sec/severity';
import {
  Unit,
  useApplyUnitMutation,
  useApproveUnitMutation,
  useGetUnitQuery,
  useInvokeFunctionsMutation,
  useLazyGetUnitQuery,
  useListExtendedRevisionsQuery,
  usePatchUnitMutation,
  useUpdateUnitMutation,
} from '../sdk/confighubapi.gen';

function DiffView({ before, after }: { before: string; after: string }) {
  const parts = useMemo(() => diffLines(before, after), [before, after]);
  return (
    <Box component='pre' sx={{ fontSize: 13, m: 0, overflow: 'auto' }}>
      {parts.map((part, i) => (
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
  );
}

interface ChangeDialogProps {
  open: boolean;
  title: string;
  preview?: React.ReactNode;
  busy: boolean;
  onCancel: () => void;
  onConfirm: (changeDescription: string) => void;
}

/** Every write goes through here: a change description is required. */
function ChangeDialog({ open, title, preview, busy, onCancel, onConfirm }: ChangeDialogProps) {
  const [desc, setDesc] = useState('');
  return (
    <Dialog open={open} onClose={onCancel} maxWidth='md' fullWidth>
      <DialogTitle>{title}</DialogTitle>
      <DialogContent>
        {preview}
        <TextField
          autoFocus
          fullWidth
          size='small'
          label='Change description (required)'
          value={desc}
          onChange={(e) => setDesc(e.target.value)}
          sx={{ mt: 2 }}
        />
      </DialogContent>
      <DialogActions>
        <Button onClick={onCancel}>Cancel</Button>
        <Button
          variant='contained'
          disabled={desc.trim() === '' || busy}
          onClick={() => {
            onConfirm(desc.trim());
            setDesc('');
          }}
        >
          {busy ? 'Saving…' : 'Save'}
        </Button>
      </DialogActions>
    </Dialog>
  );
}

interface ContainerRef {
  name: string;
  image: string;
}

function parseContainers(text: string): ContainerRef[] {
  const out: ContainerRef[] = [];
  try {
    for (const doc of parseAllDocuments(text)) {
      const obj = doc.toJS() as {
        kind?: string;
        spec?: { template?: { spec?: { containers?: { name?: string; image?: string }[] } } };
      } | null;
      if (obj?.kind !== 'Deployment') continue;
      for (const c of obj.spec?.template?.spec?.containers ?? []) {
        if (typeof c.name === 'string') out.push({ name: c.name, image: c.image ?? '' });
      }
    }
  } catch {
    // leave empty
  }
  return out;
}

/** Pick a container and a new image; compile it to a yq-i and preview it. */
function ImageUpgrade({
  containers,
  onPreview,
}: {
  containers: ContainerRef[];
  onPreview: (expr: string) => void;
}) {
  const [idx, setIdx] = useState(0);
  const [image, setImage] = useState('');
  const c = containers[idx];
  const compiled = c && image.trim() !== '' ? compileSetImage(c.name, image.trim()) : null;
  if (containers.length === 0) return null;
  return (
    <Paper variant='outlined' sx={{ p: 2, mb: 3 }}>
      <Typography variant='subtitle1' gutterBottom>
        Upgrade image (runs server-side, format-preserving)
      </Typography>
      <Stack direction='row' spacing={2} useFlexGap flexWrap='wrap' alignItems='center'>
        <FormControl size='small' sx={{ minWidth: 160 }}>
          <InputLabel>Container</InputLabel>
          <Select label='Container' value={idx} onChange={(e) => setIdx(Number(e.target.value))}>
            {containers.map((ct, i) => (
              <MenuItem key={ct.name} value={i}>
                {ct.name}
              </MenuItem>
            ))}
          </Select>
        </FormControl>
        <TextField
          size='small'
          label='New image'
          placeholder={c?.image}
          value={image}
          onChange={(e) => setImage(e.target.value)}
          sx={{ minWidth: 280 }}
        />
        <Button variant='outlined' disabled={!compiled} onClick={() => compiled && onPreview(compiled.expr)}>
          Preview change
        </Button>
      </Stack>
      <Typography variant='caption' color='text.secondary' sx={{ display: 'block', mt: 1 }}>
        Current: <Box component='code'>{c?.image}</Box>. After saving, re-run{' '}
        <Box component='code'>secscan scan-fleet --write-back</Box> to refresh the verdict.
      </Typography>
    </Paper>
  );
}

function FindingsTable({ findings }: { findings: Finding[] }) {
  if (findings.length === 0) return null;
  return (
    <>
      <Typography variant='h6' gutterBottom>
        Findings ({findings.length})
      </Typography>
      <Table size='small' sx={{ mb: 4 }}>
        <TableHead>
          <TableRow>
            <TableCell>Severity</TableCell>
            <TableCell>Score</TableCell>
            <TableCell>CVE</TableCell>
            <TableCell>Package</TableCell>
            <TableCell>Installed</TableCell>
            <TableCell>Fixed</TableCell>
          </TableRow>
        </TableHead>
        <TableBody>
          {findings.map((f, i) => (
            <TableRow key={`${f.advisory}-${f.package}-${i}`} hover>
              <TableCell>
                <Chip size='small' color={severityColor(f.severity)} label={severityLabel(f.severity)} />
              </TableCell>
              <TableCell>{f.cvss_score ? f.cvss_score.toFixed(1) : '—'}</TableCell>
              <TableCell>{f.advisory}</TableCell>
              <TableCell>{f.package}</TableCell>
              <TableCell>
                <Box component='code' sx={{ fontSize: 12 }}>
                  {f.version}
                </Box>
              </TableCell>
              <TableCell>
                <Box component='code' sx={{ fontSize: 12 }}>
                  {f.fixed_version ?? '—'}
                </Box>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </>
  );
}

export function UnitPage() {
  const { spaceId = '', unitId = '' } = useParams();
  const { data: extended, isLoading, isError, refetch } = useGetUnitQuery({ spaceId, unitId });
  const revisions = useListExtendedRevisionsQuery({ spaceId, unitId });
  const [fetchLatest] = useLazyGetUnitQuery();
  const [updateUnit, updateState] = useUpdateUnitMutation();
  const [patchUnit, patchState] = usePatchUnitMutation();
  const [invokeFunctions, invokeState] = useInvokeFunctionsMutation();
  const [applyUnit, applyState] = useApplyUnitMutation();
  const [approveUnit, approveState] = useApproveUnitMutation();

  const unit = extended?.Unit;
  const originalText = useMemo(() => b64decodeUtf8(unit?.Data ?? ''), [unit?.Data]);
  const [editedText, setEditedText] = useState<string | null>(null);
  const text = editedText ?? originalText;
  const dirty = editedText !== null && editedText !== originalText;
  const [viewMode, setViewMode] = useState<'security' | 'yaml'>('security');

  const containers = useMemo(() => parseContainers(originalText), [originalText]);

  const { snapshot } = useSnapshot();
  const workload = snapshot?.byUnit.get(unitId);
  const stale = workload ? isStale(workload, snapshot?.cvedb ?? null) : false;

  const [saveOpen, setSaveOpen] = useState(false);
  const [restoreTarget, setRestoreTarget] = useState<number | null>(null);
  const [diffOpen, setDiffOpen] = useState<{ num: number; before: string } | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [actionInfo, setActionInfo] = useState<string | null>(null);
  const [pendingEdit, setPendingEdit] = useState<{ expr: string; after: string } | null>(null);

  if (isError) {
    return (
      <Container sx={{ mt: 4 }}>
        <Alert severity='error'>Unit not found.</Alert>
      </Container>
    );
  }
  if (isLoading || !unit) {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', mt: 12 }}>
        <CircularProgress />
      </Box>
    );
  }

  const gates = Object.keys(unit.ApplyGates ?? {});
  const warnings = Object.keys(unit.ApplyWarnings ?? {});

  const save = async (changeDescription: string) => {
    setActionError(null);
    const latest = await fetchLatest({ spaceId, unitId });
    const payload = {
      ...latest.data?.Unit,
      Data: b64encodeUtf8(text),
      LastChangeDescription: changeDescription,
    } as unknown as Unit;
    const result = await updateUnit({ spaceId, unitId, unit: payload });
    setSaveOpen(false);
    if ('error' in result && result.error) {
      setActionError('Save failed — the guardrail triggers may have rejected the change.');
      return;
    }
    setEditedText(null);
    void refetch();
    void revisions.refetch();
  };

  const restore = async (changeDescription: string) => {
    if (restoreTarget === null) return;
    setActionError(null);
    const result = await patchUnit({
      spaceId,
      unitId,
      restore: String(restoreTarget),
      body: { LastChangeDescription: changeDescription },
    });
    setRestoreTarget(null);
    if ('error' in result && result.error) {
      setActionError('Restore failed.');
      return;
    }
    setEditedText(null);
    void refetch();
    void revisions.refetch();
  };

  const showDiff = async (revisionId: string, num: number) => {
    try {
      setDiffOpen({ num, before: await fetchRevisionDataText(spaceId, unitId, revisionId) });
    } catch {
      setActionError(`Failed to fetch revision ${num} data.`);
    }
  };

  const yqInvocation = (expr: string) => ({
    FunctionInvocations: [
      { FunctionName: 'yq-i', Arguments: [{ ParameterName: 'yq-expression', Value: expr }] },
    ],
  });

  const unitWhere = `UnitID = '${unitId}'`;

  const previewEdit = async (expr: string) => {
    setActionError(null);
    setActionInfo(null);
    const result = await invokeFunctions({
      spaceId,
      where: unitWhere,
      dryRun: 'true',
      functionInvocationsRequest: yqInvocation(expr),
    });
    if ('error' in result && result.error) {
      setActionError('Dry run failed.');
      return;
    }
    const response = (result.data ?? [])[0];
    if (!response?.Success) {
      setActionError('Dry run did not succeed — check the image reference.');
      return;
    }
    const after = response.ConfigData ? b64decodeUtf8(response.ConfigData) : originalText;
    if (after === originalText) {
      setActionInfo('No change: the image is already set to that value.');
      return;
    }
    setPendingEdit({ expr, after });
  };

  const commitEdit = async (changeDescription: string) => {
    if (pendingEdit === null) return;
    setActionError(null);
    const result = await invokeFunctions({
      spaceId,
      where: unitWhere,
      functionInvocationsRequest: { ...yqInvocation(pendingEdit.expr), ChangeDescription: changeDescription },
    });
    setPendingEdit(null);
    if ('error' in result && result.error) {
      setActionError('Edit failed.');
      return;
    }
    setEditedText(null);
    setActionInfo('Image updated. Re-run the scanner (scan-fleet --write-back) to refresh the CVE verdict.');
    void refetch();
    void revisions.refetch();
  };

  const apply = async () => {
    setActionError(null);
    setActionInfo(null);
    const result = await applyUnit({ spaceId, unitId });
    if ('error' in result && result.error) {
      setActionError('Apply rejected — check Apply Gates and Target.');
      return;
    }
    setActionInfo('Apply submitted.');
    void refetch();
  };

  const approve = async () => {
    setActionError(null);
    setActionInfo(null);
    const result = await approveUnit({ spaceId, unitId });
    if ('error' in result && result.error) {
      setActionError('Approve failed — you may lack Approve permission.');
      return;
    }
    setActionInfo('Approved. Gates re-evaluate asynchronously; refresh in a moment.');
    void refetch();
  };

  const sev = workload?.maxSeverity ?? 'UNKNOWN';

  return (
    <Container maxWidth='lg' sx={{ mt: 3 }}>
      <Stack direction='row' spacing={1.5} alignItems='center' sx={{ mb: 2 }} useFlexGap flexWrap='wrap'>
        <Typography variant='h5'>{unit.Slug}</Typography>
        <Chip size='small' color={severityColor(sev)} label={severityLabel(sev)} />
        <Chip size='small' label={`head rev ${unit.HeadRevisionNum ?? '?'}`} />
        {gates.map((g) => (
          <Chip key={g} size='small' color='error' label={g} />
        ))}
        {warnings.map((w) => (
          <Chip key={w} size='small' color='warning' variant='outlined' label={w} />
        ))}
        <Box sx={{ flexGrow: 1 }} />
        {gates.some((g) => g.endsWith('/vet-approvedby')) && (
          <Button variant='contained' color='success' disabled={approveState.isLoading} onClick={() => void approve()}>
            Approve
          </Button>
        )}
        <Button
          variant='outlined'
          disabled={!unit.TargetID || gates.length > 0 || applyState.isLoading}
          title={
            !unit.TargetID
              ? 'No Target bound — this is a paper cluster'
              : gates.length > 0
                ? 'Blocked by Apply Gates'
                : 'Apply head revision to the Target'
          }
          onClick={() => void apply()}
        >
          Apply
        </Button>
      </Stack>

      {actionError !== null && (
        <Alert severity='error' sx={{ mb: 2 }} onClose={() => setActionError(null)}>
          {actionError}
        </Alert>
      )}
      {actionInfo !== null && (
        <Alert severity='info' sx={{ mb: 2 }} onClose={() => setActionInfo(null)}>
          {actionInfo}
        </Alert>
      )}

      <ToggleButtonGroup
        size='small'
        exclusive
        value={viewMode}
        onChange={(_, v: 'security' | 'yaml' | null) => v !== null && setViewMode(v)}
        sx={{ mb: 2 }}
      >
        <ToggleButton value='security'>Security</ToggleButton>
        <ToggleButton value='yaml'>YAML{dirty ? ' (edited)' : ''}</ToggleButton>
      </ToggleButtonGroup>

      {viewMode === 'security' ? (
        <>
          <Paper variant='outlined' sx={{ p: 2, mb: 3 }}>
            <Typography variant='subtitle1' gutterBottom>
              Images & scan verdict
            </Typography>
            <Stack spacing={1}>
              {(workload?.images ?? containers.map((c) => c.image)).map((img) => (
                <Box key={img} component='code' sx={{ fontSize: 13 }}>
                  {img}
                </Box>
              ))}
              <Typography variant='body2' color='text.secondary'>
                {workload?.scanned
                  ? `Scanned: ${severityLabel(sev)}, ${workload.cveCount} CVE(s).`
                  : 'Not scanned yet — run secscan scan-fleet --write-back to record a verdict.'}
              </Typography>
              {workload?.scanned && (
                <Typography variant='caption' color='text.secondary'>
                  Scanned {workload.scannedAt || '?'} against CVE DB {workload.cvedbVersion || '?'}.
                  {stale && (
                    <Chip
                      size='small'
                      color='warning'
                      variant='outlined'
                      label='stale — re-scan'
                      title={`current CVE DB is ${snapshot?.cvedb?.version ?? 'newer'}`}
                      sx={{ ml: 1 }}
                    />
                  )}
                </Typography>
              )}
            </Stack>
          </Paper>

          <ImageUpgrade containers={containers} onPreview={(expr) => void previewEdit(expr)} />

          <FindingsTable findings={workload?.findings ?? []} />
        </>
      ) : (
        <>
          <TextField
            fullWidth
            multiline
            minRows={16}
            maxRows={32}
            value={text}
            onChange={(e) => setEditedText(e.target.value)}
            slotProps={{ input: { sx: { fontFamily: 'monospace', fontSize: 13 } } }}
          />
          <Stack direction='row' spacing={2} sx={{ mt: 1, mb: 4 }}>
            <Button variant='contained' disabled={!dirty} onClick={() => setSaveOpen(true)}>
              Save…
            </Button>
            <Button disabled={!dirty} onClick={() => setEditedText(null)}>
              Discard edits
            </Button>
          </Stack>
        </>
      )}

      <Typography variant='h6' gutterBottom>
        Revisions
      </Typography>
      <Table size='small'>
        <TableHead>
          <TableRow>
            <TableCell>Rev</TableCell>
            <TableCell>When</TableCell>
            <TableCell>By</TableCell>
            <TableCell>Description</TableCell>
            <TableCell />
          </TableRow>
        </TableHead>
        <TableBody>
          {(revisions.data ?? []).map((er) => {
            const rev = er.Revision;
            if (!rev) return null;
            const num = rev.RevisionNum ?? 0;
            const isHead = num === unit.HeadRevisionNum;
            return (
              <TableRow key={rev.RevisionID} hover>
                <TableCell>
                  {num} {isHead && <Chip size='small' label='head' />}
                </TableCell>
                <TableCell>{rev.CreatedAt?.replace('T', ' ').slice(0, 19)}</TableCell>
                <TableCell>{er.User?.Username ?? er.User?.ExternalID ?? ''}</TableCell>
                <TableCell>{rev.Description}</TableCell>
                <TableCell>
                  <Stack direction='row' spacing={1}>
                    {!isHead && rev.RevisionID !== undefined && (
                      <Button size='small' onClick={() => void showDiff(rev.RevisionID!, num)}>
                        Diff vs current
                      </Button>
                    )}
                    {!isHead && (
                      <Button size='small' color='warning' onClick={() => setRestoreTarget(num)}>
                        Restore…
                      </Button>
                    )}
                  </Stack>
                </TableCell>
              </TableRow>
            );
          })}
        </TableBody>
      </Table>

      <ChangeDialog
        open={saveOpen}
        title={`Save ${unit.Slug}`}
        preview={<DiffView before={originalText} after={text} />}
        busy={updateState.isLoading}
        onCancel={() => setSaveOpen(false)}
        onConfirm={(d) => void save(d)}
      />
      <ChangeDialog
        open={pendingEdit !== null}
        title='Commit image upgrade (runs server-side)'
        preview={pendingEdit && <DiffView before={originalText} after={pendingEdit.after} />}
        busy={invokeState.isLoading}
        onCancel={() => setPendingEdit(null)}
        onConfirm={(d) => void commitEdit(d)}
      />
      <ChangeDialog
        open={restoreTarget !== null}
        title={`Restore ${unit.Slug} to revision ${restoreTarget ?? ''}`}
        preview={
          <Typography variant='body2' color='text.secondary'>
            Moves head to a new revision with the content of revision {restoreTarget}. Nothing is lost — this is
            itself a recorded change.
          </Typography>
        }
        busy={patchState.isLoading}
        onCancel={() => setRestoreTarget(null)}
        onConfirm={(d) => void restore(d)}
      />
      <Dialog open={diffOpen !== null} onClose={() => setDiffOpen(null)} maxWidth='md' fullWidth>
        <DialogTitle>Revision {diffOpen?.num} → current</DialogTitle>
        <DialogContent>{diffOpen && <DiffView before={diffOpen.before} after={originalText} />}</DialogContent>
        <DialogActions>
          <Button onClick={() => setDiffOpen(null)}>Close</Button>
        </DialogActions>
      </Dialog>
    </Container>
  );
}
