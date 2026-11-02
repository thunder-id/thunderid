// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
 * variant; all others use the REDIRECT_BASED variant.
 *
 * @param templateId - The template ID (e.g., 'react', 'react-embedded')
 * @returns The guide variant key (EMBEDDED or REDIRECT_BASED)
 */
export function getIntegrationGuideVariantKey(templateId: string | undefined | null): string {
  const isEmbedded = templateId?.includes(TemplateConstants.EMBEDDED_SUFFIX) ?? false;

  return isEmbedded ? ApplicationCreateFlowSignInApproach.EMBEDDED : ApplicationCreateFlowSignInApproach.REDIRECT_BASED;
}

/**
 * Gets the integration guide for the variant selected by a template ID.
 *
 * Unlike {@link getIntegrationGuidesForTemplate}, which returns the full guides object,
 * this returns only the guide for the selected variant (EMBEDDED or REDIRECT_BASED), or null
 * when neither variant has content. Use this to decide whether a guide can be rendered.
 *
 * Falls back to the other variant when the selected one isn't authored for this template (e.g. a
 * template whose default sign-in approach is EMBEDDED but which only has REDIRECT_BASED guide content),
 * every technology template should offer a coding-agent prompt regardless of sign-in approach.
 *
 * @param templateId - The template ID (e.g., 'react', 'react-embedded', 'express-embedded')
 * @returns The guide for the selected variant (or the other variant as a fallback), or null if neither exists
 */
export function getIntegrationGuideForTemplate(templateId: string | undefined): IntegrationGuides[string] | null {
  const guides = getIntegrationGuidesForTemplate(templateId);

  if (!guides) {
    return null;
  }

  const selectedVariant = getIntegrationGuideVariantKey(templateId);
  const otherVariant =
    selectedVariant === ApplicationCreateFlowSignInApproach.EMBEDDED
      ? ApplicationCreateFlowSignInApproach.REDIRECT_BASED
      : ApplicationCreateFlowSignInApproach.EMBEDDED;

  return guides[selectedVariant] ?? guides[otherVariant] ?? null;
}
