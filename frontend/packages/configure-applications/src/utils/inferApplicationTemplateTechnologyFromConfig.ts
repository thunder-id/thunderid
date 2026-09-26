// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {TechnologyApplicationTemplate} from '../models/application-templates';
import {OAuth2GrantTypes} from '@thunderid/configure-applications';
import type {OAuth2Config} from '@thunderid/configure-applications';

/**
 * Infers the application template technology type from an OAuth2 configuration.
 *
 * This function analyzes OAuth2 configuration properties to determine the most
 * appropriate application template technology. It uses patterns such as client
 * type (public/confidential) and grant types to make the inference.
 *
 * @param config - The OAuth2 configuration to analyze, or null if no config exists.
 * @returns The inferred technology-based application template type.
 *
 * @remarks
 * The inference logic:
 * - Public clients → REACT (typical for SPAs)
 * - Confidential clients with authorization code → NEXTJS (typical for server-side apps)
 * - No config or other patterns → OTHER (fallback)
 *
 * @example
 * ```typescript
 * // Public client (SPA)
 * const spaConfig = { publicClient: true, grantTypes: ['authorization_code'] };
 * inferApplicationTemplateTechnologyFromConfig(spaConfig);
 * // Returns: 'REACT'
 *
 * // Confidential client (SSR)
 * const ssrConfig = { publicClient: false, grantTypes: ['authorization_code'] };
 * inferApplicationTemplateTechnologyFromConfig(ssrConfig);
 * // Returns: 'NEXTJS'
 *
 * // No config
 * inferApplicationTemplateTechnologyFromConfig(null);
 * // Returns: 'OTHER'
 * ```
 */
export default function inferApplicationTemplateTechnologyFromConfig(
  config: OAuth2Config | null,
): TechnologyApplicationTemplate {
  if (!config) return TechnologyApplicationTemplate.OTHER;

  if (config.publicClient) {
    return TechnologyApplicationTemplate.REACT;
  }

  if (config.grantTypes.includes(OAuth2GrantTypes.AUTHORIZATION_CODE)) {
    return TechnologyApplicationTemplate.NEXTJS;
  }

  return TechnologyApplicationTemplate.OTHER;
}
