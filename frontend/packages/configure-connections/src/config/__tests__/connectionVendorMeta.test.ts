// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import type {ConnectionInstance} from '../../models/connection';
import buildConnectionCards from '../../utils/buildConnectionCards';
import {CONNECTION_VENDOR_META, getAvailableConnectionCategories} from '../connectionVendorMeta';

const OIDC_FEDERATION: ConnectionInstance = {
  id: 'c1',
  name: 'Corp OIDC',
  type: 'oidc',
  categories: ['identity-provider'],
};

const TRUSTED_ISSUER: ConnectionInstance = {
  id: 'c2',
  name: 'Acme Issuer',
  type: 'oidc',
  categories: ['identity-provider'],
  idJagEnabled: true,
};

/** Categories offered as filter chips for the given instances, against the real vendor catalog. */
function categoriesFor(instances: ConnectionInstance[]): string[] {
  return getAvailableConnectionCategories(buildConnectionCards(instances, CONNECTION_VENDOR_META));
}

describe('getAvailableConnectionCategories', () => {
  it('offers only the categories with a branded tile when nothing is configured', () => {
    expect(categoriesFor([])).toEqual(['social-login', 'sms']);
  });

  it('offers enterprise and custom once an OIDC connection exists', () => {
    expect(categoriesFor([OIDC_FEDERATION])).toEqual(['social-login', 'enterprise', 'sms', 'custom']);
  });

  it('offers trusted-idp once a trusted issuer exists', () => {
    expect(categoriesFor([TRUSTED_ISSUER])).toEqual(['social-login', 'sms', 'trusted-idp', 'custom']);
  });

  it('never offers a category that no card belongs to', () => {
    const cards = buildConnectionCards([OIDC_FEDERATION, TRUSTED_ISSUER], CONNECTION_VENDOR_META);

    for (const category of getAvailableConnectionCategories(cards)) {
      expect(cards.some((card) => card.categories.includes(category))).toBe(true);
    }
  });
});
