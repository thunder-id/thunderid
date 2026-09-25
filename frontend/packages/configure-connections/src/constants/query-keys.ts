// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Query key constants for connections feature cache management.
 */
const ConnectionQueryKeys = {
  /**
   * Key for identity providers queries
   */
  IDENTITY_PROVIDERS: 'identity-providers',

  /**
   * Key for SMS provider queries (consumed by useSMSProviders)
   */
  SMS_PROVIDERS: 'sms-providers',

  /**
   * Key for email provider queries (consumed by useEmailProviders)
   */
  EMAIL_PROVIDERS: 'email-providers',

  /**
   * Key for the paginated connection instances list (GET /connections)
   */
  CONNECTIONS: 'connections',

  /**
   * Key for configured instances of a connection type (GET /connections/{type})
   */
  CONNECTION_INSTANCES: 'connection-instances',

  /**
   * Key for a single connection instance (GET /connections/{type}/{id})
   */
  CONNECTION: 'connection',

  /**
   * Key for the resources referencing a connection instance (GET /connections/{type}/{id}/usages)
   */
  CONNECTION_USAGES: 'connection-usages',

  /**
   * Key for a vendor's configurable options (GET /connections/meta)
   */
  CONNECTION_META: 'connection-meta',

  /**
   * Key for a single trusted issuer (GET /connections/oidc/{id})
   */
  TRUSTED_ISSUER: 'trustedIssuer',
} as const;

export default ConnectionQueryKeys;
