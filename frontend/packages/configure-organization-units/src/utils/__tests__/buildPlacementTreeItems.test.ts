// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import type {OrganizationUnit} from '../../models/organization-unit';
import buildPlacementTreeItems from '../buildPlacementTreeItems';

const system: OrganizationUnit = {id: 'system-ou', handle: 'default', name: 'Default', parent: null, isSystem: true};
const underSystem: OrganizationUnit = {id: 'ou-eu', handle: 'eu', name: 'EU', parent: 'system-ou'};
const root: OrganizationUnit = {id: 'ou-retail', handle: 'retail', name: 'Retail', parent: null};

describe('buildPlacementTreeItems', () => {
  it('sets the units placed under the system unit apart from the top ones', () => {
    const {topItems, underSystem: nested} = buildPlacementTreeItems([system, underSystem, root]);

    expect(topItems.map((item) => item.id)).toEqual(['system-ou', 'ou-retail']);
    expect(nested.map((item) => item.id)).toEqual(['ou-eu']);
  });

  it('places the units of a later page under the system unit it is given', () => {
    const {topItems, underSystem: nested} = buildPlacementTreeItems([underSystem, root], 'system-ou');

    expect(topItems.map((item) => item.id)).toEqual(['ou-retail']);
    expect(nested.map((item) => item.id)).toEqual(['ou-eu']);
  });

  it('keeps every unit at the top when the listing has no system unit', () => {
    const {topItems, underSystem: nested} = buildPlacementTreeItems([underSystem, root]);

    expect(topItems.map((item) => item.id)).toEqual(['ou-eu', 'ou-retail']);
    expect(nested).toEqual([]);
  });
});
