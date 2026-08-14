// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {JSX} from 'react';
import {describe, expect, it} from 'vitest';
import {type ConnectionInstance, type ConnectionVendorMeta} from '../../models/connection';
import buildConnectionCards from '../buildConnectionCards';

const LOGO = 'logo' as unknown as JSX.Element;

const VENDORS: ConnectionVendorMeta[] = [
  {
    key: 'google',
    backendType: 'google',
    displayName: 'Google Login',
    descriptionKey: 'connections:vendor.google.description',
    logo: LOGO,
    categories: ['social-login'],
    presentation: 'branded',
  },
  {
    key: 'github',
    backendType: 'github',
    displayName: 'GitHub Login',
    descriptionKey: 'connections:vendor.github.description',
    logo: LOGO,
    categories: ['social-login'],
    presentation: 'branded',
  },
  {
    key: 'oidc',
    backendType: 'oidc',
    displayName: 'Custom OIDC',
    descriptionKey: 'connections:vendor.oidc.description',
    logo: LOGO,
    categories: ['enterprise'],
    presentation: 'custom',
  },
  {
    key: 'sms-gateway',
    backendType: 'sms-gateway',
    displayName: 'SMS Gateway',
    descriptionKey: 'connections:vendor.sms-gateway.description',
    logo: LOGO,
    categories: ['sms', 'custom'],
    presentation: 'custom',
  },
  {
    key: 'twilio',
    displayName: 'Twilio SMS',
    descriptionKey: 'connections:vendor.twilio.description',
    logo: LOGO,
    categories: ['sms'],
    presentation: 'coming-soon',
    comingSoon: true,
  },
];

describe('buildConnectionCards', () => {
  it('renders one card per instance of a branded vendor, titled by instance name', () => {
    const instances: ConnectionInstance[] = [
      {id: 'g1', name: 'Corp Google', type: 'google', categories: ['identity-provider']},
      {id: 'g2', name: 'Test Google', type: 'google', categories: ['identity-provider']},
    ];
    const cards = buildConnectionCards(instances, VENDORS);

    const googleCards = cards.filter((c) => c.vendorKey === 'google');
    expect(googleCards).toHaveLength(2);
    expect(googleCards[0]).toMatchObject({
      id: 'google:g1',
      displayName: 'Corp Google',
      status: 'configured',
      comingSoon: false,
      navTarget: '/connections/google/g1',
    });
    expect(googleCards[1]).toMatchObject({
      id: 'google:g2',
      displayName: 'Test Google',
      navTarget: '/connections/google/g2',
    });
  });

  it('renders a not-configured branded card that opens the configure wizard', () => {
    const cards = buildConnectionCards([], VENDORS);

    const github = cards.find((c) => c.vendorKey === 'github');
    expect(github).toMatchObject({
      displayName: 'GitHub Login',
      status: 'not-configured',
      navTarget: '/connections/github/configure',
    });
  });

  it('renders one card per custom (oidc) instance', () => {
    const instances: ConnectionInstance[] = [
      {id: 'a1', name: 'Acme Workforce OIDC', type: 'oidc', categories: ['identity-provider']},
      {id: 'b2', name: 'EU Citizen Login', type: 'oidc', categories: ['identity-provider']},
    ];
    const cards = buildConnectionCards(instances, VENDORS);

    const oidcCards = cards.filter((c) => c.vendorKey === 'oidc');
    expect(oidcCards).toHaveLength(2);
    expect(oidcCards[0]).toMatchObject({
      id: 'oidc:a1',
      displayName: 'Acme Workforce OIDC',
      status: 'configured',
      navTarget: '/connections/oidc/a1',
    });
    expect(oidcCards[1].navTarget).toBe('/connections/oidc/b2');
  });

  it('renders one card per SMS gateway instance', () => {
    const instances: ConnectionInstance[] = [
      {id: 's1', name: 'Prod SMS Gateway', type: 'sms-gateway', categories: ['sms-provider']},
    ];
    const cards = buildConnectionCards(instances, VENDORS);

    expect(cards.filter((c) => c.vendorKey === 'sms-gateway')).toHaveLength(1);
    expect(cards.find((c) => c.vendorKey === 'sms-gateway')).toMatchObject({
      id: 'sms-gateway:s1',
      displayName: 'Prod SMS Gateway',
      status: 'configured',
      navTarget: '/connections/sms-gateway/s1',
    });
  });

  it('renders no custom cards when there are no instances', () => {
    const cards = buildConnectionCards([], VENDORS);
    expect(cards.filter((c) => c.vendorKey === 'oidc')).toHaveLength(0);
  });

  it('ignores instances whose type has no vendor meta', () => {
    const instances: ConnectionInstance[] = [
      {id: 's1', name: 'Custom Gateway', type: 'vonage', categories: ['sms-provider']},
    ];
    const cards = buildConnectionCards(instances, VENDORS);
    expect(cards.some((c) => c.displayName === 'Custom Gateway')).toBe(false);
  });

  it('renders coming-soon vendors as non-navigating cards', () => {
    const cards = buildConnectionCards([], VENDORS);

    const twilio = cards.find((c) => c.vendorKey === 'twilio');
    expect(twilio).toMatchObject({
      comingSoon: true,
      navTarget: null,
      status: 'not-configured',
    });
  });

  it('renders branded wizard tiles + coming-soon cards even with no instances', () => {
    const cards = buildConnectionCards([], VENDORS);
    const keys = cards.map((c) => c.vendorKey);
    expect(keys).toEqual(['google', 'github', 'twilio']);
  });

  it('renders an OIDC instance with idJagEnabled as a trusted-idp card, not a plain oidc card', () => {
    const instances: ConnectionInstance[] = [
      {id: 't1', name: 'Acme Okta', type: 'oidc', categories: ['identity-provider'], idJagEnabled: true},
    ];
    const cards = buildConnectionCards(instances, VENDORS);

    expect(cards.filter((c) => c.vendorKey === 'oidc')).toHaveLength(0);
    const trustedIdpCard = cards.find((c) => c.vendorKey === 'trusted-idp');
    expect(trustedIdpCard).toMatchObject({
      id: 'trusted-idp:t1',
      displayName: 'Acme Okta',
      status: 'configured',
      comingSoon: false,
      navTarget: '/trusted-issuers/t1',
      categories: ['trusted-idp', 'custom'],
    });
  });

  it('renders an OIDC instance with idJagEnabled: false as a trusted-idp card', () => {
    const instances: ConnectionInstance[] = [
      {id: 't2', name: 'Disabled Trust', type: 'oidc', categories: ['identity-provider'], idJagEnabled: false},
    ];
    const cards = buildConnectionCards(instances, VENDORS);

    const trustedIdpCard = cards.find((c) => c.vendorKey === 'trusted-idp');
    expect(trustedIdpCard).toMatchObject({navTarget: '/trusted-issuers/t2'});
  });

  it('renders a plain OIDC instance without idJagEnabled as the normal oidc card, unchanged', () => {
    const instances: ConnectionInstance[] = [
      {id: 'f1', name: 'Federation OIDC', type: 'oidc', categories: ['identity-provider']},
    ];
    const cards = buildConnectionCards(instances, VENDORS);

    expect(cards.some((c) => c.vendorKey === 'trusted-idp')).toBe(false);
    const oidcCard = cards.find((c) => c.vendorKey === 'oidc');
    expect(oidcCard).toMatchObject({
      displayName: 'Federation OIDC',
      navTarget: '/connections/oidc/f1',
    });
  });

  it('renders a non-OIDC instance with idJagEnabled as its normal vendor card, not a trusted-idp card', () => {
    const instances: ConnectionInstance[] = [
      {id: 'g1', name: 'Corp Google', type: 'google', categories: ['identity-provider'], idJagEnabled: true},
    ];
    const cards = buildConnectionCards(instances, VENDORS);

    expect(cards.some((c) => c.vendorKey === 'trusted-idp')).toBe(false);
    const googleCard = cards.find((c) => c.vendorKey === 'google');
    expect(googleCard).toMatchObject({
      id: 'google:g1',
      displayName: 'Corp Google',
      navTarget: '/connections/google/g1',
    });
  });
});
