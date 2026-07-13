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

import {GitHub, Google, KeyRound, MessageSquare, Send, ShieldCheck} from '@wso2/oxygen-ui-icons-react';
import {CONNECTION_CATEGORIES} from '../constants/connection-categories';
import {type ConnectionCategory, ConnectionTypes, type ConnectionVendorMeta} from '../models/connection';

/**
 * Frontend-owned catalog of every connection vendor the console presents.
 *
 * The backend `/connections` API only knows `google`/`github`/`oidc`/`oauth`; this map adds all
 * presentation (logo, name, categories) plus the coming-soon placeholder vendors that are
 * not yet wired to an API.
 */
export const CONNECTION_VENDOR_META: ConnectionVendorMeta[] = [
  {
    key: 'google',
    backendType: ConnectionTypes.GOOGLE,
    displayName: 'Google',
    descriptionKey: 'connections:vendor.google.description',
    logo: <Google />,
    categories: ['social-login'],
    presentation: 'branded',
    supportsAttributeMapping: true,
  },
  {
    key: 'github',
    backendType: ConnectionTypes.GITHUB,
    displayName: 'GitHub',
    descriptionKey: 'connections:vendor.github.description',
    logo: <GitHub />,
    categories: ['social-login'],
    presentation: 'branded',
    supportsAttributeMapping: true,
  },
  {
    key: 'oidc',
    backendType: ConnectionTypes.OIDC,
    displayName: 'OpenID Connect',
    descriptionKey: 'connections:vendor.oidc.description',
    logo: <ShieldCheck />,
    categories: ['enterprise'],
    presentation: 'custom',
    supportsAttributeMapping: true,
  },
  {
    key: 'oauth',
    backendType: ConnectionTypes.OAUTH,
    displayName: 'OAuth 2.0',
    descriptionKey: 'connections:vendor.oauth.description',
    logo: <KeyRound />,
    categories: ['enterprise'],
    presentation: 'custom',
  },
  {
    key: 'twilio',
    backendType: ConnectionTypes.TWILIO,
    displayName: 'Twilio',
    descriptionKey: 'connections:vendor.twilio.description',
    logo: <MessageSquare />,
    categories: ['sms'],
    presentation: 'branded',
  },
  {
    key: 'vonage',
    backendType: ConnectionTypes.VONAGE,
    displayName: 'Vonage',
    descriptionKey: 'connections:vendor.vonage.description',
    logo: <Send />,
    categories: ['sms'],
    presentation: 'branded',
  },
];

/**
 * Categories actually represented by the vendor catalog, in display order. Drives the listing
 * filter chips so categories with no connections (e.g. Email, CRM) are not shown.
 */
export const AVAILABLE_CONNECTION_CATEGORIES: ConnectionCategory[] = CONNECTION_CATEGORIES.filter((category) =>
  CONNECTION_VENDOR_META.some((vendor) => vendor.categories.includes(category)),
);

/**
 * Vendor meta keyed by backend connection type (for the wired vendors only).
 */
export const VENDOR_META_BY_TYPE = Object.fromEntries(
  CONNECTION_VENDOR_META.filter((v) => v.backendType).map((v) => [v.backendType as string, v]),
);

/**
 * Look up vendor meta by its map key.
 */
export const getVendorMetaByKey = (key: string): ConnectionVendorMeta | undefined =>
  CONNECTION_VENDOR_META.find((v) => v.key === key);
