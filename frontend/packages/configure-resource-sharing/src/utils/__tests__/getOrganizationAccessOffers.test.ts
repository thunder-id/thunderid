// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import getOrganizationAccessOffers from '../getOrganizationAccessOffers';

describe('getOrganizationAccessOffers', () => {
  it('offers no deployment-wide choice for an unshared resource', () => {
    expect(getOrganizationAccessOffers(undefined)).toEqual({everyUnit: false, everyTree: false});
  });

  it('offers every organization unit while the saved policy holds it', () => {
    expect(getOrganizationAccessOffers({id: 'p1', stage: 'share', version: 1, allUnits: {}})).toEqual({
      everyUnit: true,
      everyTree: false,
    });
  });

  it('offers every tree while the saved policy reaches every other tree', () => {
    expect(getOrganizationAccessOffers({id: 'p1', stage: 'share', version: 1, otherTreeRoots: {mode: 'all'}})).toEqual({
      everyUnit: false,
      everyTree: true,
    });
  });

  it('does not offer every tree for named trees', () => {
    expect(
      getOrganizationAccessOffers({
        id: 'p1',
        stage: 'share',
        version: 1,
        otherTreeRoots: {mode: 'selected', units: [{ouId: 'northwind'}]},
      }).everyTree,
    ).toBe(false);
  });
});
