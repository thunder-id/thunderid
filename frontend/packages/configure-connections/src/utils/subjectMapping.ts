// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {AuthZENPDPSubjectAttribute, AuthZENPDPSubjectAttributeMapping} from '../models/connection';
import type {SubjectMappingGroup, SubjectRowsState} from '../models/subject-mapping';

export function canonicalSubjectAttributeMappings(
  subjectAttributeMappings: AuthZENPDPSubjectAttributeMapping[] | undefined,
): string {
  return JSON.stringify(subjectAttributeMappings ?? []);
}

export function buildSubjectRows(
  subjectAttributeMappings: AuthZENPDPSubjectAttributeMapping[] | undefined,
  fromSeq: number,
): SubjectRowsState {
  if (subjectAttributeMappings && subjectAttributeMappings.length > 0) {
    let seq = fromSeq;
    const groups = subjectAttributeMappings.map((group) => {
      const rows =
        group.attributes.length > 0
          ? group.attributes.map((attribute) => {
              seq += 1;
              return {key: seq, attribute: attribute.attribute, pdpAttribute: attribute.pdpAttribute ?? ''};
            })
          : [{key: seq + 1, attribute: '', pdpAttribute: ''}];
      seq += group.attributes.length > 0 ? 0 : 1;
      seq += 1;
      return {key: seq, category: '' as const, userType: group.entityType, rows};
    });
    return {groups, seq, syncedAttributeMappings: canonicalSubjectAttributeMappings(subjectAttributeMappings)};
  }
  return {
    groups: [
      {key: fromSeq + 1, category: 'user', userType: '', rows: [{key: fromSeq + 2, attribute: '', pdpAttribute: ''}]},
      {key: fromSeq + 3, category: 'agent', userType: '', rows: [{key: fromSeq + 4, attribute: '', pdpAttribute: ''}]},
    ],
    seq: fromSeq + 4,
    syncedAttributeMappings: canonicalSubjectAttributeMappings(subjectAttributeMappings),
  };
}

export function serializeSubjectAttributeMappings(groups: SubjectMappingGroup[]): AuthZENPDPSubjectAttributeMapping[] {
  return normalizeSubjectAttributeMappings(
    groups
      .filter((group) => group.userType.trim() !== '')
      .map((group) => ({
        entityType: group.userType,
        attributes: group.rows
          .map((row) => ({attribute: row.attribute.trim(), pdpAttribute: row.pdpAttribute.trim()}))
          .filter((row) => row.attribute !== '')
          .map((row) => ({
            attribute: row.attribute,
            ...(row.pdpAttribute !== '' ? {pdpAttribute: row.pdpAttribute} : {}),
          })),
      })),
  );
}

export function normalizeSubjectAttributeMappings(
  mappings: AuthZENPDPSubjectAttributeMapping[] | undefined,
): AuthZENPDPSubjectAttributeMapping[] {
  return (mappings ?? [])
    .map((mapping) => ({
      ...mapping,
      attributes: mapping.attributes.filter((attribute) => attribute.attribute.trim() !== ''),
    }))
    .filter((mapping) => mapping.attributes.length > 0);
}

export function isDuplicateSubjectAttribute(
  attributes: Pick<AuthZENPDPSubjectAttribute, 'attribute'>[],
  attribute: string,
): boolean {
  const normalizedAttribute = attribute.trim();
  return (
    normalizedAttribute !== '' &&
    attributes.filter((candidate) => candidate.attribute.trim() === normalizedAttribute).length > 1
  );
}

export function hasDuplicateSubjectAttributes(mappings: AuthZENPDPSubjectAttributeMapping[] | undefined): boolean {
  return (mappings ?? []).some((mapping) =>
    mapping.attributes.some((attribute) => isDuplicateSubjectAttribute(mapping.attributes, attribute.attribute)),
  );
}
