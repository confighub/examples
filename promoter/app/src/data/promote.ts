// Promotion = upgrade a stage's variant-Space units from their upstream units
// (the `cub unit update --patch --upgrade` path, via patchUnit upgrade=true).
//
// This only works when the target variant's units are actually linked upstream
// to the previous stage's chosen variant. We inspect that link topology first
// and never silently fall back to copying data: if the links don't line up,
// promotion is reported as not-promotable with a reason, and the UI disables
// the button.
//
// Promoting changes desired state in ConfigHub; it delivers nothing. Delivery is
// publishing an immutable Release of the Space to its OCI Release Target, which a
// cluster-side controller then pulls. The two are separate steps here because they
// have different subjects: an upgrade acts on the Units linked upstream, while a
// Release captures the Space's whole EffectiveReleaseSet — every Unit assigned to
// the Space's Release Target, promoted or not. The dialog states both counts rather
// than letting a promotion approval quietly become a wider publish.

import { useCallback } from 'react';

import {
  useLazyGetSpaceQuery,
  useLazyListAllTargetsQuery,
  useLazyListAllUnitsQuery,
  usePatchUnitMutation,
  usePublishReleaseMutation,
} from '@confighub/rtk-query';

import { VariantRef } from './catalog';

export interface UnitPromotability {
  unitId: string;
  slug: string;
  headRevisionNum: number;
  upstreamUnitId?: string;
  /** True when this unit is linked to the expected upstream variant. */
  ok: boolean;
  reason?: string;
}

export interface PromotabilityReport {
  /** True when there is at least one unit and every unit is ok. */
  promotable: boolean;
  units: UnitPromotability[];
  summary: string;
}

export interface PromoteResult {
  /** Highest head revision across the upgraded units (for status display). */
  revision: number;
}

/** One member of the Space's EffectiveReleaseSet. */
export interface ReleaseMember {
  unitId: string;
  slug: string;
  /** Gate keys blocking publication. A Release with any gate present is refused. */
  gates: string[];
  /** True when this Unit is also one of the Units the promotion upgrades. */
  promoted: boolean;
}

/**
 * Whether the target Space can be published, and — the part that matters for
 * approval — exactly what publishing would capture.
 */
export interface ReleaseReadiness {
  publishable: boolean;
  /** Why not, when publishable is false. */
  reason?: string;
  /** The Space's ReleaseTargetID, when it has one. */
  targetId?: string;
  targetSlug?: string;
  providerType?: string;
  /** Every Unit a Release of this Space would bundle. */
  members: ReleaseMember[];
  /** Members that the promotion does not touch — the widened scope. */
  alsoCaptured: ReleaseMember[];
  /** Members whose ApplyGates block publication. */
  gated: ReleaseMember[];
}

export interface PublishResult {
  releaseId: string;
  releaseNum: number;
  unitCount: number;
}

export interface Promotion {
  inspect: (target: VariantRef, upstream: VariantRef) => Promise<PromotabilityReport>;
  promote: (
    target: VariantRef,
    report: PromotabilityReport,
    changeDesc: string,
  ) => Promise<PromoteResult>;
  /** Read-only: what a Release of this Space would capture, and whether it can be cut. */
  inspectRelease: (
    target: VariantRef,
    report: PromotabilityReport | null,
  ) => Promise<ReleaseReadiness>;
  publish: (target: VariantRef) => Promise<PublishResult>;
}

export function usePromotion(): Promotion {
  const [listUnits] = useLazyListAllUnitsQuery();
  const [patchUnit] = usePatchUnitMutation();
  const [getSpace] = useLazyGetSpaceQuery();
  const [listTargets] = useLazyListAllTargetsQuery();
  const [publishRelease] = usePublishReleaseMutation();

  const inspect = useCallback(
    async (target: VariantRef, upstream: VariantRef): Promise<PromotabilityReport> => {
      // A unit's UpstreamSpaceID is not denormalized in list responses, so we
      // resolve the link by checking whether its UpstreamUnitID is one of the
      // units in the upstream variant's Space.
      const [targetResult, upstreamResult] = await Promise.all([
        listUnits({
          where: `SpaceID = '${target.spaceId}'`,
          select: 'UnitID,Slug,UpstreamUnitID,HeadRevisionNum',
        }),
        listUnits({ where: `SpaceID = '${upstream.spaceId}'`, select: 'UnitID' }),
      ]);
      if (targetResult.error || targetResult.data === undefined || upstreamResult.error) {
        return { promotable: false, units: [], summary: 'Failed to read units.' };
      }
      const upstreamUnitIds = new Set(
        (upstreamResult.data ?? []).map((eu) => eu.Unit?.UnitID).filter(Boolean) as string[],
      );

      const units: UnitPromotability[] = targetResult.data.flatMap((eu) => {
        const u = eu.Unit;
        if (!u?.UnitID || !u.Slug) return [];
        const hasLink = Boolean(u.UpstreamUnitID);
        const linkedToUpstream = hasLink && upstreamUnitIds.has(u.UpstreamUnitID as string);
        let ok = false;
        let reason: string | undefined;
        if (!hasLink) {
          reason = 'no upstream link';
        } else if (!linkedToUpstream) {
          reason = `upstream unit is not in ${upstream.spaceSlug}`;
        } else {
          ok = true;
        }
        return [
          {
            unitId: u.UnitID,
            slug: u.Slug,
            headRevisionNum: u.HeadRevisionNum ?? 0,
            upstreamUnitId: u.UpstreamUnitID,
            ok,
            reason,
          },
        ];
      });

      if (units.length === 0) {
        return { promotable: false, units, summary: `No units in ${target.spaceSlug}.` };
      }
      const blocked = units.filter((u) => !u.ok);
      if (blocked.length > 0) {
        return {
          promotable: false,
          units,
          summary: `${blocked.length} of ${units.length} unit(s) not linked to ${upstream.spaceSlug}.`,
        };
      }
      return {
        promotable: true,
        units,
        summary: `${units.length} unit(s) ready to upgrade from ${upstream.spaceSlug}.`,
      };
    },
    [listUnits],
  );

  const promote = useCallback(
    async (
      target: VariantRef,
      report: PromotabilityReport,
      changeDesc: string,
    ): Promise<PromoteResult> => {
      if (!report.promotable) throw new Error('Not promotable');

      let revision = 0;
      for (const unit of report.units) {
        const res = await patchUnit({
          spaceId: target.spaceId,
          unitId: unit.unitId,
          upgrade: true,
          body: { LastChangeDescription: changeDesc },
        });
        if ('error' in res && res.error) {
          throw new Error(`Upgrade failed for ${unit.slug}`);
        }
        // A write answers with what it did, not with the entity: the Unit is nested.
        const head = ('data' in res && res.data?.Unit?.HeadRevisionNum) || 0;
        revision = Math.max(revision, head);
      }

      return { revision };
    },
    [patchUnit],
  );

  const inspectRelease = useCallback(
    async (target: VariantRef, report: PromotabilityReport | null): Promise<ReleaseReadiness> => {
      const empty = { members: [], alsoCaptured: [], gated: [] };

      // A Release is bound to the Space's Release Target. Without one there is
      // nowhere to publish to, and the Target must be an OCI registry: that is the
      // only delivery provider a Release supports.
      const spaceResult = await getSpace({ spaceId: target.spaceId });
      if (spaceResult.error || !spaceResult.data) {
        return { publishable: false, reason: 'Failed to read the Space.', ...empty };
      }
      const targetId = spaceResult.data.Space?.ReleaseTargetID;
      if (!targetId) {
        return {
          publishable: false,
          reason: `${target.spaceSlug} has no Release Target. Bind one with \`cub space update\` before publishing.`,
          ...empty,
        };
      }

      // A Space's Release Target usually lives in another Space — one OCI Target is
      // commonly shared by every variant Space that publishes to it — so this is looked
      // up across Spaces by id rather than inside the variant's own Space.
      const targetResult = await listTargets({ where: `TargetID = '${targetId}'` });
      const found = (targetResult.data ?? [])[0]?.Target;
      const providerType = found?.ProviderType;
      const targetSlug = found?.Slug;
      if (targetResult.error || !found) {
        return { publishable: false, reason: 'Failed to read the Release Target.', targetId, ...empty };
      }
      if (providerType !== 'OCI') {
        return {
          publishable: false,
          reason: `Release Target ${targetSlug ?? targetId} is ${providerType ?? 'of an unknown provider'}; publishing requires an OCI Target.`,
          targetId,
          targetSlug,
          providerType,
          ...empty,
        };
      }

      // The EffectiveReleaseSet: every Unit in the Space assigned to that Target.
      // Not the promoted Units, and not every Unit in the Space.
      const unitsResult = await listUnits({
        where: `SpaceID = '${target.spaceId}'`,
        select: 'UnitID,Slug,TargetID,ApplyGates',
      });
      if (unitsResult.error || unitsResult.data === undefined) {
        return { publishable: false, reason: 'Failed to read units.', targetId, targetSlug, providerType, ...empty };
      }

      const promotedIds = new Set((report?.units ?? []).map((u) => u.unitId));
      const members: ReleaseMember[] = unitsResult.data.flatMap((eu) => {
        const u = eu.Unit;
        if (!u?.UnitID || u.TargetID !== targetId) return [];
        return [
          {
            unitId: u.UnitID,
            slug: u.Slug ?? u.UnitID,
            gates: Object.keys(u.ApplyGates ?? {}),
            promoted: promotedIds.has(u.UnitID),
          },
        ];
      });

      const gated = members.filter((m) => m.gates.length > 0);
      const alsoCaptured = members.filter((m) => !m.promoted);

      if (members.length === 0) {
        return {
          publishable: false,
          reason: `No Unit in ${target.spaceSlug} is assigned to Release Target ${targetSlug ?? targetId}, so a Release would be empty.`,
          targetId,
          targetSlug,
          providerType,
          members,
          alsoCaptured,
          gated,
        };
      }
      if (gated.length > 0) {
        return {
          publishable: false,
          reason: `${gated.length} of ${members.length} Unit(s) have an Apply Gate set; publishing is refused until they are cleared.`,
          targetId,
          targetSlug,
          providerType,
          members,
          alsoCaptured,
          gated,
        };
      }
      return { publishable: true, targetId, targetSlug, providerType, members, alsoCaptured, gated };
    },
    [getSpace, listTargets, listUnits],
  );

  const publish = useCallback(
    async (target: VariantRef): Promise<PublishResult> => {
      // No TagID: every Unit is bundled at its head, and the server creates a
      // release-<num> Tag on each bundled Revision as the Release's own TagID.
      const res = await publishRelease({ spaceId: target.spaceId, releasePublishRequest: {} });
      if ('error' in res && res.error) {
        throw new Error(`Publishing a Release of ${target.spaceSlug} failed.`);
      }
      const release = 'data' in res ? res.data : undefined;
      if (!release?.ReleaseID) {
        throw new Error('Publish returned no Release.');
      }
      return {
        releaseId: release.ReleaseID,
        releaseNum: release.ReleaseNum ?? 0,
        unitCount: release.UnitCount ?? 0,
      };
    },
    [publishRelease],
  );

  return { inspect, promote, inspectRelease, publish };
}
