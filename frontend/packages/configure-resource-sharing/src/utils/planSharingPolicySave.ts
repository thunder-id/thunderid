// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {hasNoSharingSections} from './draftToSharingSections';
import type {SharingPolicySavePlan} from '../models/organization-access';
import type {SharingPolicy, SharingPolicySections} from '../models/sharing-policy';

type SectionName = keyof SharingPolicySections;

type SectionKind = 'none' | 'selected' | 'blanket';

const SECTION_NAMES: SectionName[] = ['allUnits', 'childUnits', 'otherTreeRoots'];

function kindOf(sections: SharingPolicySections, name: SectionName): SectionKind {
  const section = sections[name];
  if (!section) return 'none';
  if (name === 'allUnits') return 'blanket';
  return (section as {mode: string}).mode === 'all' ? 'blanket' : 'selected';
}

function sectionKey(sections: SharingPolicySections, name: SectionName): string {
  const section = sections[name] as
    | {mode?: string; excludedOuIds?: string[]; units?: {ouId: string; reach?: string}[]}
    | undefined;
  if (!section) return '';
  const excluded = [...(section.excludedOuIds ?? [])].sort().join(',');
  const units = (section.units ?? [])
    .map((unit) => `${unit.ouId}:${unit.reach ?? ''}`)
    .sort()
    .join(',');
  return `${section.mode ?? ''}|${excluded}|${units}`;
}

/**
 * Reports whether two sets of sections reach the same organization units in the same way, ignoring
 * order.
 */
export function haveSameSharingSections(a: SharingPolicySections, b: SharingPolicySections): boolean {
  return SECTION_NAMES.every((name) => sectionKey(a, name) === sectionKey(b, name));
}

/**
 * Reports whether a policy holding `current` can become `next` through an edit.
 *
 * Each section is judged on its own. A section that reaches a whole group keeps that reach, so an
 * edit may only rewrite its exclusions. A section naming units may add or drop them, and may appear
 * or go away. Adding, dropping or converting a section that reaches a whole group is a different
 * policy.
 */
export function canEditSharingSectionsInPlace(current: SharingPolicySections, next: SharingPolicySections): boolean {
  return SECTION_NAMES.every((name) => {
    const was = kindOf(current, name);
    const becomes = kindOf(next, name);
    return was === becomes || (was !== 'blanket' && becomes !== 'blanket');
  });
}

/**
 * Decides what saving a set of sections does to a resource's owner-issued sharing policy.
 *
 * @param current - The policy as last read, or `undefined` when the resource is not shared
 * @param sections - The sections the resource should have after the save
 * @returns The requests the save takes
 */
export default function planSharingPolicySave(
  current: SharingPolicy | undefined,
  sections: SharingPolicySections,
): SharingPolicySavePlan {
  if (hasNoSharingSections(sections)) {
    return current ? {type: 'delete', policyId: current.id} : {type: 'none'};
  }
  if (!current) {
    return {type: 'create', sections};
  }
  if (haveSameSharingSections(current, sections)) {
    return {type: 'none'};
  }
  if (canEditSharingSectionsInPlace(current, sections)) {
    return {type: 'update', policyId: current.id, version: current.version, sections};
  }
  return {type: 'replace', policyId: current.id, sections};
}
