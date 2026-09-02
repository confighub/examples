// One RBAC resource, rendered the same way wherever it is reached from — the Explorer
// table, a Finding, or anywhere else that has a FleetResource. Findings and inventory
// rows are two ways of arriving at the same object, so they share the panel rather than
// each growing their own.
//
// Two renderings: the friendly view over the resource's JSON projection, and the
// resource's original YAML, which is read on demand because the fleet snapshot
// deliberately does not carry it.

import {
  Alert,
  Box,
  Button,
  CircularProgress,
  Drawer,
  Stack,
  ToggleButton,
  ToggleButtonGroup,
  Typography,
} from '@mui/material';
import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { stringify } from 'yaml';

import { getResourceRaw, stripCommentKeys } from '@confighub/examples-webkit/api';
import type { FleetResource } from '@confighub/examples-webkit/rbac';

import { clusterContextFor } from '../fleet/enrichment';
import { useSnapshot } from '../fleet/snapshot';
import { FriendlyResource } from './friendly/RbacFriendly';

/** Kind and name of a resource document, for titles and edit deep links. */
export function resourceIdentity(doc: unknown): { kind: string; name: string; namespace: string } {
  const rec = doc as { kind?: string; metadata?: { name?: string; namespace?: string } } | null;
  return {
    kind: typeof rec?.kind === 'string' ? rec.kind : '',
    name: rec?.metadata?.name ?? '',
    namespace: rec?.metadata?.namespace ?? '',
  };
}

/**
 * Link into the Unit page with this resource preselected in the quick-edit panel. The
 * kind and name travel in the query string rather than a resource id so the link still
 * means something after an edit replaces the resource row.
 */
export function editLink(resource: FleetResource): string {
  const { kind, name } = resourceIdentity(resource.doc);
  const params = new URLSearchParams({ kind, name });
  return `/unit/${resource.origin.spaceId}/${resource.origin.unitId}?${params.toString()}`;
}

/** The resource's original YAML, fetched when the YAML tab is first opened. */
function RawYaml({ resource }: { resource: FleetResource }) {
  const { spaceId, unitId, resourceId } = resource.origin;
  const [text, setText] = useState<string | null>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    if (resourceId === undefined) return;
    let cancelled = false;
    setText(null);
    setFailed(false);
    getResourceRaw(spaceId, unitId, resourceId)
      .then((t) => {
        if (!cancelled) setText(t);
      })
      .catch(() => {
        if (!cancelled) setFailed(true);
      });
    return () => {
      cancelled = true;
    };
  }, [spaceId, unitId, resourceId]);

  // Without a resource id, or when the read fails, the JSON projection still renders the
  // configuration — only the authored comments and formatting are lost.
  const fallback = stringify(stripCommentKeys(resource.doc));
  if (resourceId !== undefined && text === null && !failed) {
    return <CircularProgress size={20} />;
  }
  return (
    <Box
      component='pre'
      sx={{ bgcolor: 'grey.100', p: 1.5, borderRadius: 1, overflow: 'auto', fontSize: 13 }}
    >
      {text ?? fallback}
    </Box>
  );
}

export interface ResourceDetailProps {
  /** The resource to show; null closes the panel. */
  resource: FleetResource | null;
  onClose: () => void;
  /** Rendered above the resource, e.g. the Finding that led here. */
  context?: JSX.Element;
}

/** Side panel showing one RBAC resource, with a route into editing it. */
export function ResourceDetail({ resource, onClose, context }: ResourceDetailProps) {
  const { snapshot } = useSnapshot();
  const [viewMode, setViewMode] = useState<'friendly' | 'yaml'>('friendly');

  return (
    <Drawer anchor='right' open={resource !== null} onClose={onClose}>
      {resource && (
        <Box sx={{ width: 560, p: 2 }}>
          <Typography variant='h6' gutterBottom>
            {resourceIdentity(resource.doc).kind} {resourceIdentity(resource.doc).name}
          </Typography>
          <Typography variant='body2' color='text.secondary'>
            {resource.origin.cluster} · unit{' '}
            <Link to={`/unit/${resource.origin.spaceId}/${resource.origin.unitId}`}>
              {resource.origin.unitSlug}
            </Link>{' '}
            · space {resource.origin.space}
          </Typography>
          {resource.origin.canonical === true && (
            <Alert severity='info' sx={{ mt: 1 }}>
              A base definition — nothing deploys here. Fixing it here fixes every cluster
              it is cloned into; the clusters also report it separately, until they are
              upgraded from base.
            </Alert>
          )}
          {context}
          <Stack direction='row' spacing={1} alignItems='center' sx={{ my: 2 }}>
            <ToggleButtonGroup
              size='small'
              exclusive
              value={viewMode}
              onChange={(_, v: 'friendly' | 'yaml' | null) => v !== null && setViewMode(v)}
            >
              <ToggleButton value='friendly'>Friendly</ToggleButton>
              <ToggleButton value='yaml'>YAML</ToggleButton>
            </ToggleButtonGroup>
            <Box sx={{ flexGrow: 1 }} />
            <Button size='small' variant='outlined' component={Link} to={editLink(resource)}>
              Edit this resource
            </Button>
          </Stack>
          {viewMode === 'friendly' ? (
            <FriendlyResource
              doc={resource.doc}
              cluster={clusterContextFor(
                snapshot,
                resource.origin.cluster,
                resource.origin.unitId,
              )}
              editHref={editLink(resource)}
            />
          ) : (
            /* Read-only rendering; writes never round-trip through this. */
            <RawYaml resource={resource} />
          )}
        </Box>
      )}
    </Drawer>
  );
}
