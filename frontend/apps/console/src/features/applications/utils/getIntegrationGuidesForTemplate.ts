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

import normalizeTemplateId from './normalizeTemplateId';
import PlatformBasedApplicationTemplateMetadata from '../config/PlatformBasedApplicationTemplateMetadata';
import TechnologyBasedApplicationTemplateMetadata from '../config/TechnologyBasedApplicationTemplateMetadata';
import TemplateConstants from '../constants/template-constants';
import {ApplicationCreateFlowSignInApproach} from '../models/application-create-flow';
import type {IntegrationGuides} from '../models/application-templates';

/**
 * Gets the integration guides for a given template ID
 * @param templateId - The template ID (e.g., 'react', 'react-embedded', 'nextjs', 'browser')
 * @returns Integration guides object, or null if not found
 */
export default function getIntegrationGuidesForTemplate(templateId: string | undefined): IntegrationGuides | null {
  if (!templateId) {
    return null;
  }

  // Normalize the template ID to handle embedded variants (e.g., 'react-embedded' -> 'react')
  const normalizedTemplateId = normalizeTemplateId(templateId) ?? templateId;

  // Search in technology-based templates
  const techTemplate = TechnologyBasedApplicationTemplateMetadata.find(
    (metadata) => metadata.template.id === normalizedTemplateId,
  );

  if (techTemplate?.template.integrationGuides) {
    return techTemplate.template.integrationGuides;
  }

  // Search in platform-based templates
  const platformTemplate = PlatformBasedApplicationTemplateMetadata.find(
    (metadata) => metadata.template.id === normalizedTemplateId,
  );

  if (platformTemplate?.template.integrationGuides) {
    return platformTemplate.template.integrationGuides;
  }

  return null;
}

/**
 * Resolves the integration guide variant key for a template ID.
 *
 * Templates with the '-embedded' suffix (e.g., 'react-embedded') use the EMBEDDED
 * variant; all others use the INBUILT variant.
 *
 * @param templateId - The template ID (e.g., 'react', 'react-embedded')
 * @returns The guide variant key (EMBEDDED or INBUILT)
 */
export function getIntegrationGuideVariantKey(templateId: string | undefined | null): string {
  const isEmbedded = templateId?.includes(TemplateConstants.EMBEDDED_SUFFIX) ?? false;

  return isEmbedded ? ApplicationCreateFlowSignInApproach.EMBEDDED : ApplicationCreateFlowSignInApproach.INBUILT;
}

/**
 * Gets the integration guide for the variant selected by a template ID.
 *
 * Unlike {@link getIntegrationGuidesForTemplate}, which returns the full guides object,
 * this returns only the guide for the selected variant (EMBEDDED or INBUILT), or null
 * when that variant has no content. Use this to decide whether a guide can be rendered.
 *
 * @param templateId - The template ID (e.g., 'react', 'react-embedded', 'express-embedded')
 * @returns The guide for the selected variant, or null if not found
 */
export function getIntegrationGuideForTemplate(templateId: string | undefined): IntegrationGuides[string] | null {
  const guides = getIntegrationGuidesForTemplate(templateId);

  if (!guides) {
    return null;
  }

  return guides[getIntegrationGuideVariantKey(templateId)] ?? null;
}
