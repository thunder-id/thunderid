// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import {hasDuplicateSubjectAttributes, normalizeSubjectAttributeMappings} from '../subjectMapping';

describe('normalizeSubjectAttributeMappings', () => {
  it('omits mappings that release no attributes', () => {
    expect(
      normalizeSubjectAttributeMappings([
        {entityType: 'employee', attributes: [{attribute: 'email'}]},
        {entityType: 'assistant', attributes: []},
      ]),
    ).toEqual([{entityType: 'employee', attributes: [{attribute: 'email'}]}]);
  });
});

describe('hasDuplicateSubjectAttributes', () => {
  it('detects duplicate attributes within one entity type', () => {
    expect(
      hasDuplicateSubjectAttributes([
        {entityType: 'employee', attributes: [{attribute: 'email'}, {attribute: 'email'}]},
      ]),
    ).toBe(true);
  });

  it('allows the same attribute across different entity types', () => {
    expect(
      hasDuplicateSubjectAttributes([
        {entityType: 'employee', attributes: [{attribute: 'email'}]},
        {entityType: 'customer', attributes: [{attribute: 'email'}]},
      ]),
    ).toBe(false);
  });
});
