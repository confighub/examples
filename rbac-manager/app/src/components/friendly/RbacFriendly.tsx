// Friendly (form-like) renderings of the RBAC resource kinds, enriched with
// cluster context where available: roleRef resolution status, binding scope,
// reverse ServiceAccount lookups, and aggregation expansion. Falls back to
// YAML for unknown kinds.

import { Alert, Box, Chip, Typography } from '@mui/material';
import { stringify } from 'yaml';

import { ClusterRbac, Subject, subjectKey } from '../../rbac/model';
import {
  effectiveRules,
  isBuiltinRoleName,
  resolveRoleRef,
} from '../../rbac/semantics';
import {
  EmptyHint,
  FieldList,
  MiniTable,
  Section,
  TokenList,
  VerbChips,
} from './helpers';

type Doc = Record<string, unknown>;

function asRecord(v: unknown): Doc | undefined {
  return typeof v === 'object' && v !== null && !Array.isArray(v) ? (v as Doc) : undefined;
}

function asArray(v: unknown): unknown[] {
  return Array.isArray(v) ? v : [];
}

function asString(v: unknown): string {
  return typeof v === 'string' ? v : '';
}

function asStringArray(v: unknown): string[] {
  return asArray(v).filter((x): x is string => typeof x === 'string');
}

function MetadataSection({ doc }: { doc: Doc }) {
  const metadata = asRecord(doc.metadata);
  const labels = asRecord(metadata?.labels) ?? {};
  const labelEntries = Object.entries(labels);
  return (
    <Section title='Metadata'>
      <FieldList
        fields={[
          { label: 'Name', value: asString(metadata?.name) },
          { label: 'Namespace', value: asString(metadata?.namespace) },
          {
            label: 'Labels',
            value:
              labelEntries.length > 0 ? (
                <Box>
                  {labelEntries.map(([k, v]) => (
                    <Chip key={k} size='small' variant='outlined' label={`${k}=${String(v)}`} sx={{ mr: 0.5, mb: 0.5 }} />
                  ))}
                </Box>
              ) : undefined,
          },
        ]}
      />
    </Section>
  );
}

function RulesSection({ doc, title }: { doc: Doc; title: string }) {
  const rules = asArray(doc.rules);
  if (rules.length === 0) {
    return (
      <Section title={title}>
        <EmptyHint text='This role grants no permissions of its own.' />
      </Section>
    );
  }
  return (
    <Section title={title}>
      <MiniTable
        columns={['#', 'API groups', 'Resources', 'Verbs', 'Restricted to names']}
        rows={rules.map((r, i) => {
          const rule = asRecord(r);
          const resources = asStringArray(rule?.resources);
          const nonResource = asStringArray(rule?.nonResourceURLs);
          return [
            String(i),
            nonResource.length > 0 ? (
              <EmptyHint text='(non-resource URLs)' />
            ) : (
              <TokenList values={asStringArray(rule?.apiGroups)} core />
            ),
            <TokenList values={resources.length > 0 ? resources : nonResource} />,
            <VerbChips verbs={asStringArray(rule?.verbs)} />,
            asStringArray(rule?.resourceNames).join(', ') || '(any)',
          ];
        })}
      />
      <Typography variant='caption' color='text.secondary'>
        Rule numbers match the Quick edit panel.
      </Typography>
    </Section>
  );
}

function RoleView({ doc, cluster }: { doc: Doc; cluster?: ClusterRbac }) {
  const kind = asString(doc.kind);
  const name = asString(asRecord(doc.metadata)?.name);
  const aggregation = asRecord(doc.aggregationRule);
  const selectors = asArray(aggregation?.clusterRoleSelectors);

  // Expand aggregation through the engine when cluster context is available.
  const engineRole = cluster?.roles.find(
    (r) => r.kind === kind && r.name === name,
  );
  const aggregated =
    selectors.length > 0 && engineRole && cluster
      ? effectiveRules(engineRole, cluster)
      : null;

  return (
    <>
      <MetadataSection doc={doc} />
      {selectors.length > 0 && (
        <Section title='Aggregation'>
          <Alert severity='info' sx={{ mb: 1 }}>
            This ClusterRole aggregates other ClusterRoles by label
            {aggregated !== null &&
              ` — effective rules below include ${aggregated.length} aggregated rule(s)`}
            .
          </Alert>
          {selectors.map((s, i) => (
            <Chip
              key={i}
              size='small'
              variant='outlined'
              label={Object.entries(asRecord(asRecord(s)?.matchLabels) ?? {})
                .map(([k, v]) => `${k}=${String(v)}`)
                .join(', ')}
              sx={{ mr: 0.5 }}
            />
          ))}
        </Section>
      )}
      <RulesSection doc={doc} title={selectors.length > 0 ? 'Own rules' : 'Rules'} />
      {aggregated !== null && aggregated.length > 0 && (
        <Section title='Effective rules (including aggregation)'>
          <MiniTable
            columns={['API groups', 'Resources', 'Verbs']}
            rows={aggregated.map((r) => [
              <TokenList values={r.apiGroups} core />,
              <TokenList values={r.resources} />,
              <VerbChips verbs={r.verbs} />,
            ])}
          />
        </Section>
      )}
    </>
  );
}

function BindingView({ doc, cluster }: { doc: Doc; cluster?: ClusterRbac }) {
  const kind = asString(doc.kind);
  const name = asString(asRecord(doc.metadata)?.name);
  const namespace = asString(asRecord(doc.metadata)?.namespace);
  const roleRef = asRecord(doc.roleRef);
  const refKind = asString(roleRef?.kind);
  const refName = asString(roleRef?.name);
  const subjects = asArray(doc.subjects);

  // Resolution status, when we have cluster context.
  const engineBinding = cluster?.bindings.find((b) => b.kind === kind && b.name === name);
  let resolution: 'resolved' | 'builtin' | 'missing' | 'unknown' = 'unknown';
  let resolvedUnit: string | undefined;
  if (engineBinding && cluster) {
    const role = resolveRoleRef(engineBinding, cluster);
    if (role) {
      resolution = 'resolved';
      resolvedUnit = role.origin.unitSlug;
    } else {
      resolution = isBuiltinRoleName(refName) ? 'builtin' : 'missing';
    }
  }

  return (
    <>
      <MetadataSection doc={doc} />
      <Section title='Grants role'>
        <FieldList
          fields={[
            {
              label: 'Role',
              value: (
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                  <span>
                    {refKind}/{refName}
                  </span>
                  {resolution === 'resolved' && (
                    <Chip size='small' color='success' variant='outlined' label={`defined in unit ${resolvedUnit}`} />
                  )}
                  {resolution === 'builtin' && (
                    <Chip size='small' variant='outlined' label='Kubernetes builtin' />
                  )}
                  {resolution === 'missing' && (
                    <Chip size='small' color='error' label='role not found — orphaned binding' />
                  )}
                </Box>
              ),
            },
            {
              label: 'Scope',
              value:
                kind === 'ClusterRoleBinding'
                  ? 'cluster-wide (all namespaces)'
                  : `namespace "${namespace}" only`,
            },
          ]}
        />
        {refName === 'cluster-admin' && (
          <Alert severity='error' sx={{ mt: 1 }}>
            This is a standing superuser grant.
          </Alert>
        )}
      </Section>
      <Section title={`Subjects (${subjects.length})`}>
        {subjects.length === 0 ? (
          <EmptyHint text='No subjects — this binding grants nothing.' />
        ) : (
          <MiniTable
            columns={['Kind', 'Name', 'Namespace']}
            rows={subjects.map((s) => {
              const subject = asRecord(s);
              return [
                <Chip size='small' variant='outlined' label={asString(subject?.kind)} />,
                asString(subject?.name),
                asString(subject?.namespace),
              ];
            })}
          />
        )}
      </Section>
    </>
  );
}

function ServiceAccountView({ doc, cluster }: { doc: Doc; cluster?: ClusterRbac }) {
  const name = asString(asRecord(doc.metadata)?.name);
  const namespace = asString(asRecord(doc.metadata)?.namespace) || 'default';
  const sa: Subject = { kind: 'ServiceAccount', name, namespace };

  const boundBy = (cluster?.bindings ?? []).filter((b) =>
    b.subjects.some((s) => subjectKey(s) === subjectKey(sa)),
  );

  return (
    <>
      <MetadataSection doc={doc} />
      <Section title='Bound by'>
        {boundBy.length === 0 ? (
          <EmptyHint text='No bindings reference this ServiceAccount in this cluster — possibly unused.' />
        ) : (
          <MiniTable
            columns={['Binding', 'Grants role', 'Scope']}
            rows={boundBy.map((b) => [
              `${b.kind}/${b.name}`,
              `${b.roleRef.kind}/${b.roleRef.name}`,
              b.kind === 'ClusterRoleBinding' ? 'cluster-wide' : `namespace "${b.namespace}"`,
            ])}
          />
        )}
      </Section>
    </>
  );
}

function GenericView({ doc }: { doc: Doc }) {
  return (
    <Box component='pre' sx={{ bgcolor: 'grey.100', p: 1.5, borderRadius: 1, overflow: 'auto', fontSize: 13 }}>
      {stringify(doc)}
    </Box>
  );
}

export interface FriendlyResourceProps {
  doc: unknown;
  /** Cluster context for resolution/enrichment; omit when unavailable. */
  cluster?: ClusterRbac;
}

/** Dispatch a parsed resource document to its friendly view. */
export function FriendlyResource({ doc, cluster }: FriendlyResourceProps) {
  const rec = asRecord(doc);
  if (!rec) return <EmptyHint text='Unparseable resource.' />;
  switch (asString(rec.kind)) {
    case 'Role':
    case 'ClusterRole':
      return <RoleView doc={rec} cluster={cluster} />;
    case 'RoleBinding':
    case 'ClusterRoleBinding':
      return <BindingView doc={rec} cluster={cluster} />;
    case 'ServiceAccount':
      return <ServiceAccountView doc={rec} cluster={cluster} />;
    default:
      return <GenericView doc={rec} />;
  }
}
