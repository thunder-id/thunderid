// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import {type ConnectionInstance, ConnectionTypes} from '../../models/connection';
import buildConnectionCards from '../../utils/buildConnectionCards';
import {CONNECTION_VENDOR_META, getAvailableConnectionCategories, VENDOR_META_BY_TYPE} from '../connectionVendorMeta';

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

describe('VENDOR_META_BY_TYPE', () => {
  it('declares the detail-page capabilities supported by AuthZEN PDP connections', () => {
    expect(VENDOR_META_BY_TYPE[ConnectionTypes.AUTHZEN_PDP]).toMatchObject({
      supportsAuthentication: true,
      supportsSubjectMapping: true,
      generalSettingsCardCopy: 'configuration',
    });
  });

  it('presents the HTTP endpoint as a custom connection that re-enters credentials on an origin change and lists its usages', () => {
    expect(VENDOR_META_BY_TYPE[ConnectionTypes.HTTP]).toMatchObject({
      key: 'http',
      displayName: 'Service endpoint',
      descriptionKey: 'connections:vendor.http.description',
      categories: ['custom'],
      presentation: 'custom',
      reentersCredentialsOnOriginChange: true,
      showsUsages: true,
      generalSettingsCardCopy: 'configuration',
    });
    expect(VENDOR_META_BY_TYPE[ConnectionTypes.HTTP]?.supportsAuthentication).toBeUndefined();
    expect(VENDOR_META_BY_TYPE[ConnectionTypes.HTTP]?.supportsSubjectMapping).toBeUndefined();
  });

  it('renders each HTTP endpoint instance as its own custom card', () => {
    const cards = buildConnectionCards(
      [{id: 'h1', name: 'Risk Engine', type: 'http', categories: ['http']}],
      CONNECTION_VENDOR_META,
    );

    expect(cards.find((card) => card.id === 'http:h1')).toMatchObject({
      displayName: 'Risk Engine',
      categories: ['custom'],
      status: 'configured',
    });
  });
});
