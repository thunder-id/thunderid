// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import validateScimMapping from '../validateScimMapping';

describe('validateScimMapping', () => {
  it('passes with no userName mapped (non-blocking, checked separately)', () => {
    expect(validateScimMapping({email: 'emails'}, {})).toBeNull();
  });

  it('rejects two properties mapped to the same single-valued target', () => {
    expect(validateScimMapping({loginId: 'userName', altEmail: 'userName'}, {})).toEqual({
      type: 'duplicateTarget',
      target: 'userName',
      properties: ['loginId', 'altEmail'],
    });
  });

  it('allows two properties mapped to the same multi-valued target', () => {
    expect(validateScimMapping({workEmail: 'emails', personalEmail: 'emails'}, {})).toBeNull();
  });

  it('rejects two properties both marked primary for the same multi-valued target', () => {
    const meta = {
      workEmail: {type: 'work', primary: true},
      personalEmail: {type: 'home', primary: true},
    };
    expect(validateScimMapping({workEmail: 'emails', personalEmail: 'emails'}, meta)).toEqual({
      type: 'multiplePrimary',
      target: 'emails',
      properties: ['workEmail', 'personalEmail'],
    });
  });

  it('allows two properties mapped to the same multi-valued target with only one primary', () => {
    const meta = {
      workEmail: {type: 'work', primary: true},
      personalEmail: {type: 'home', primary: false},
    };
    expect(validateScimMapping({workEmail: 'emails', personalEmail: 'emails'}, meta)).toBeNull();
  });

  it('ignores unmapped (empty string) entries', () => {
    expect(validateScimMapping({loginId: 'userName', unused: ''}, {})).toBeNull();
  });
});
