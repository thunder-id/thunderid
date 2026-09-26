// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {ApplicationTemplate} from '../models/application-templates';

/**
 * Whether the template's seeded OAuth2 config uses the authorization_code grant, meaning the
 * created application needs a real redirect URI. Templates often ship a placeholder redirectUris
 * value (e.g. a localhost dev URL) so the app has something to run against out of the box; that
 * placeholder should not be mistaken for "already configured, nothing to ask" — the admin still
 * needs to confirm or replace it with their actual redirect URI.
 */
const isRedirectCapableTemplate = (templateConfig: ApplicationTemplate | null): boolean => {
  return Boolean(
    templateConfig?.defaults?.inboundAuthConfig
      ?.find((config) => config.type === 'oauth2')
      ?.config?.grantTypes?.includes('authorization_code'),
  );
};

export default isRedirectCapableTemplate;
