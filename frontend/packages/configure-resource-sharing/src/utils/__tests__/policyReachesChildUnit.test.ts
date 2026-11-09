// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import type {SharingPolicySections} from '../../models/sharing-policy';
import policyReachesChildUnit from '../policyReachesChildUnit';

describe('policyReachesChildUnit', () => {
  it.each<[string, SharingPolicySections, boolean]>([
    ['every unit', {allUnits: {}}, true],
    ['every unit but the child', {allUnits: {excludedOuIds: ['retail']}}, false],
    ['every child unit', {childUnits: {mode: 'all'}}, true],
    ['every child unit but the child', {childUnits: {mode: 'all', excludedOuIds: ['retail']}}, false],
    ['every child unit but another', {childUnits: {mode: 'all', excludedOuIds: ['wholesale']}}, true],
    ['the named child', {childUnits: {mode: 'selected', units: [{ouId: 'retail', reach: 'unit'}]}}, true],
    ['the named child and below', {childUnits: {mode: 'selected', units: [{ouId: 'retail', reach: 'subtree'}]}}, true],
    ['another named child', {childUnits: {mode: 'selected', units: [{ouId: 'wholesale', reach: 'subtree'}]}}, false],
    ['other trees only', {otherTreeRoots: {mode: 'selected', units: [{ouId: 'retail'}]}}, false],
    ['no section', {}, false],
  ])('%s', (_name, policy, expected) => {
    expect(policyReachesChildUnit(policy, 'retail')).toBe(expected);
  });
});
