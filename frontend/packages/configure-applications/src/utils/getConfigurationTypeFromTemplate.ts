// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {ApplicationCreateFlowConfiguration} from '../models/application-create-flow';
import type {ApplicationTemplate} from '../models/application-templates';

/**
 * Detect configuration type from template.
 * This is a data-driven approach that relies on the template definition.
 *
 * Returns:
 * - 'deeplink' for Mobile apps (need deep link or universal link)
 * - 'url' for Browser/Server apps (need application URL and callback URL)
 * - 'none' for Backend apps or templates with pre-filled redirectUris
 */
const getConfigurationTypeFromTemplate = (
  templateConfig: ApplicationTemplate | null,
): ApplicationCreateFlowConfiguration => {
  if (!templateConfig) return ApplicationCreateFlowConfiguration.NONE;

  const oauthConfig = templateConfig.defaults?.inboundAuthConfig?.find((config) => config.type === 'oauth2')?.config;

  // If redirectUris is already populated, no configuration needed
  if (oauthConfig?.redirectUris && oauthConfig.redirectUris.length > 0) {
    return ApplicationCreateFlowConfiguration.NONE;
  }

  // If redirectUris is undefined, null, or empty array, determine what type of configuration is needed
  // TODO: Remove this once https://github.com/thunder-id/thunderid/pulls/924 is merged.
  const templateName = templateConfig.defaults?.name?.toLowerCase() ?? '';

  if (templateName.includes('mobile') || templateName.includes('wallet')) {
    return ApplicationCreateFlowConfiguration.DEEPLINK;
  }

  // Browser and Server apps need URL
  if (templateName.includes('browser') || templateName.includes('server')) {
    return ApplicationCreateFlowConfiguration.URL;
  }

  // Backend doesn't need configuration (client_credentials flow)
  if (templateName.includes('backend')) {
    return ApplicationCreateFlowConfiguration.NONE;
  }

  // Default to URL for other cases
  return ApplicationCreateFlowConfiguration.URL;
};

export default getConfigurationTypeFromTemplate;
