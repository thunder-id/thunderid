/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import type {JSX} from 'react';
import {describe, expect, it} from 'vitest';
import {type ConnectionTypeSummary, type ConnectionVendorMeta} from '../../models/connection';
import buildConnectionCards from '../buildConnectionCards';

const LOGO = 'logo' as unknown as JSX.Element;

const VENDORS: ConnectionVendorMeta[] = [
  {
    key: 'google',
    backendType: 'google',
    displayName: 'Google',
    descriptionKey: 'connections:vendor.google.description',
    logo: LOGO,
    categories: ['social-login'],
    presentation: 'branded',
  },
  {
    key: 'github',
    backendType: 'github',
    displayName: 'GitHub',
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
    key: 'twilio',
    displayName: 'Twilio',
    descriptionKey: 'connections:vendor.twilio.description',
    logo: LOGO,
    categories: ['sms'],
    presentation: 'coming-soon',
    comingSoon: true,
  },
];

describe('buildConnectionCards', () => {
  it('renders a configured branded card that opens its detail page', () => {
    const summaries: ConnectionTypeSummary[] = [{type: 'google', configured: true, instanceCount: 1}];
    const cards = buildConnectionCards(summaries, VENDORS, {});

    const google = cards.find((c) => c.vendorKey === 'google');
    expect(google).toMatchObject({
      status: 'configured',
      comingSoon: false,
      navTarget: '/connections/google',
    });
  });

  it('renders a not-configured branded card that opens the configure wizard', () => {
    const cards = buildConnectionCards([], VENDORS, {});

    const github = cards.find((c) => c.vendorKey === 'github');
    expect(github).toMatchObject({
      status: 'not-configured',
      navTarget: '/connections/github/configure',
    });
  });

  it('renders one card per custom (oidc) instance', () => {
    const summaries: ConnectionTypeSummary[] = [{type: 'oidc', configured: true, instanceCount: 2}];
    const cards = buildConnectionCards(summaries, VENDORS, {
      oidc: [
        {id: 'a1', name: 'Acme Workforce OIDC'},
        {id: 'b2', name: 'EU Citizen Login'},
      ],
    });

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

  it('renders no custom cards when there are no instances', () => {
    const cards = buildConnectionCards([{type: 'oidc', configured: false, instanceCount: 0}], VENDORS, {oidc: []});
    expect(cards.filter((c) => c.vendorKey === 'oidc')).toHaveLength(0);
  });

  it('renders coming-soon vendors as non-navigating cards', () => {
    const cards = buildConnectionCards([], VENDORS, {});

    const twilio = cards.find((c) => c.vendorKey === 'twilio');
    expect(twilio).toMatchObject({
      comingSoon: true,
      navTarget: null,
      status: 'not-configured',
    });
  });

  it('always renders branded + coming-soon cards even with empty summaries', () => {
    const cards = buildConnectionCards([], VENDORS, {});
    const keys = cards.map((c) => c.vendorKey);
    expect(keys).toEqual(['google', 'github', 'twilio']);
  });
});
