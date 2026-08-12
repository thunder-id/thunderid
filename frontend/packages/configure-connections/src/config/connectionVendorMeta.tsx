// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {GithubIcon, GoogleIcon, ResourceAvatar} from '@thunderid/components';
import {MessageSquare, Send} from '@wso2/oxygen-ui-icons-react';
import {CONNECTION_CATEGORIES} from '../constants/connection-categories';
import ConnectionConstants from '../constants/connection-constants';
import {
  type ConnectionCardModel,
  type ConnectionCategory,
  ConnectionTypes,
  type ConnectionVendorMeta,
} from '../models/connection';

const AVATAR_SIZE = 48;

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
    logo: <ResourceAvatar transparent variant="rounded" size={AVATAR_SIZE} fallback={<GoogleIcon size={34} />} />,
    categories: ['social-login'],
    presentation: 'branded',
    supportsAttributeMapping: true,
    createHintKey: 'connections:configure.hint.google',
  },
  {
    key: 'github',
    backendType: ConnectionTypes.GITHUB,
    displayName: 'GitHub',
    descriptionKey: 'connections:vendor.github.description',
    logo: <ResourceAvatar transparent variant="rounded" size={AVATAR_SIZE} fallback={<GithubIcon size={34} />} />,
    categories: ['social-login'],
    presentation: 'branded',
    supportsAttributeMapping: true,
    createHintKey: 'connections:configure.hint.github',
  },
  {
    key: 'oidc',
    backendType: ConnectionTypes.OIDC,
    displayName: 'OpenID Connect',
    descriptionKey: 'connections:vendor.oidc.description',
    logo: (
      <ResourceAvatar
        transparent
        variant="rounded"
        size={AVATAR_SIZE}
        fallback={ConnectionConstants.OIDC_AVATAR_FALLBACK}
      />
    ),
    categories: ['enterprise', 'custom'],
    presentation: 'custom',
    supportsAttributeMapping: true,
  },
  {
    key: 'oauth',
    backendType: ConnectionTypes.OAUTH,
    displayName: 'OAuth 2',
    descriptionKey: 'connections:vendor.oauth.description',
    logo: (
      <ResourceAvatar
        transparent
        variant="rounded"
        size={AVATAR_SIZE}
        fallback={ConnectionConstants.OAUTH_AVATAR_FALLBACK}
      />
    ),
    categories: ['enterprise', 'custom'],
    presentation: 'custom',
    supportsAttributeMapping: true,
  },
  {
    key: 'twilio',
    backendType: ConnectionTypes.TWILIO,
    displayName: 'Twilio',
    descriptionKey: 'connections:vendor.twilio.description',
    // Twilio's brand mark isn't cleared for use yet — keep the generic icon until that's sorted.
    logo: <ResourceAvatar transparent variant="rounded" size={AVATAR_SIZE} fallback={<MessageSquare size={28} />} />,
    categories: ['sms'],
    presentation: 'branded',
  },
  {
    key: 'vonage',
    backendType: ConnectionTypes.VONAGE,
    displayName: 'Vonage',
    descriptionKey: 'connections:vendor.vonage.description',
    // Vonage's brand mark isn't cleared for use yet — keep the generic icon until that's sorted.
    logo: <ResourceAvatar transparent variant="rounded" size={AVATAR_SIZE} fallback={<Send size={28} />} />,
    categories: ['sms'],
    presentation: 'branded',
  },
  {
    key: 'sms-gateway',
    backendType: ConnectionTypes.SMS_GATEWAY,
    displayName: 'SMS Gateway',
    descriptionKey: 'connections:vendor.sms-gateway.description',
    logo: (
      <ResourceAvatar
        transparent
        variant="rounded"
        size={AVATAR_SIZE}
        fallback={ConnectionConstants.SMS_GATEWAY_AVATAR_FALLBACK}
      />
    ),
    categories: ['sms', 'custom'],
    presentation: 'custom',
  },
];

/**
 * Categories that at least one of the given listing cards belongs to, in display order. Drives the
 * listing filter chips: a category with no card (e.g. Email, or Enterprise before any OIDC connection
 * is created) would render an empty grid, so its chip is not shown at all.
 *
 * Derived from the built cards rather than from `CONNECTION_VENDOR_META` because the catalog alone
 * cannot say which categories are populated — `presentation: 'custom'` vendors and trusted issuers
 * only produce cards once instances exist.
 */
export const getAvailableConnectionCategories = (cards: ConnectionCardModel[]): ConnectionCategory[] =>
  CONNECTION_CATEGORIES.filter((category) => cards.some((card) => card.categories.includes(category)));

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
