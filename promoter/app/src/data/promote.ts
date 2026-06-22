// Promotion = upgrade a stage's variant-Space units from their upstream units
// (the `cub unit update --patch --upgrade` path, via patchUnit upgrade=true).
//
// This only works when the target variant's units are actually linked upstream
// to the previous stage's chosen variant. We inspect that link topology first
// and never silently fall back to copying data: if the links don't line up,
// promotion is reported as not-promotable with a reason, and the UI disables
// the button.

import { useCallback } from 'react';

import { VariantRef } from './catalog';
import {
  useApplyUnitMutation,
  useLazyListAllUnitsQuery,
  usePatchUnitMutation,
} from '../sdk/confighubapi.gen';

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
  applied: boolean;
}

export interface Promotion {
  inspect: (target: VariantRef, upstream: VariantRef) => Promise<PromotabilityReport>;
  promote: (
    target: VariantRef,
    report: PromotabilityReport,
    changeDesc: string,
    apply: boolean,
  ) => Promise<PromoteResult>;
}

export function usePromotion(): Promotion {
  const [listUnits] = useLazyListAllUnitsQuery();
  const [patchUnit] = usePatchUnitMutation();
  const [applyUnit] = useApplyUnitMutation();

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
      apply: boolean,
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
        const head = ('data' in res && res.data?.HeadRevisionNum) || 0;
        revision = Math.max(revision, head);
      }

      if (apply) {
        for (const unit of report.units) {
          const res = await applyUnit({ spaceId: target.spaceId, unitId: unit.unitId });
          if ('error' in res && res.error) {
            throw new Error(`Apply failed for ${unit.slug}`);
          }
        }
      }

      return { revision, applied: apply };
    },
    [patchUnit, applyUnit],
  );

  return { inspect, promote };
}
