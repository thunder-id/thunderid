// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import type {Change} from '../../models/deploy';
import {changedFields, isShared} from '../changeSummary';

describe('changedFields', () => {
  it('names the fields the changed lines set, the first few, spelled out', () => {
    expect(
      changedFields([
        {kind: ' ', text: 'name: Orders'},
        {kind: '-', text: '  redirectUris:'},
        {kind: '+', text: '  redirectUris:'},
        {kind: '+', text: '  - clientId: orders'},
        {kind: '-', text: '  tokenLifetime: 300'},
        {kind: '+', text: '  description: new'},
      ]),
    ).toEqual(['Redirect URIs', 'Client ID', 'Token lifetime']);
  });

  it('is empty when no changed line names a field', () => {
    expect(changedFields([{kind: '+', text: '  - https://a.test/cb'}])).toEqual([]);
    expect(changedFields(undefined)).toEqual([]);
  });
});

describe('isShared', () => {
  const change = (resourceType: string): Change => ({key: `${resourceType}/1`, resourceType, change: 'updated'});

  it('keeps applications and agents with the project, everything else with the organization', () => {
    expect(isShared(change('application'))).toBe(false);
    expect(isShared(change('agent'))).toBe(false);
    expect(isShared(change('flow'))).toBe(true);
    expect(isShared(change('theme'))).toBe(true);
    expect(isShared(change('user'))).toBe(true);
  });
});
