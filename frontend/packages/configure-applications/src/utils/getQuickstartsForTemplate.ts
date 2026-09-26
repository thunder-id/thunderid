// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import normalizeTemplateId from './normalizeTemplateId';
import PlatformBasedApplicationTemplateMetadata from '../config/PlatformBasedApplicationTemplateMetadata';
import TechnologyBasedApplicationTemplateMetadata from '../config/TechnologyBasedApplicationTemplateMetadata';
import type {QuickstartLink} from '../models/application-templates';

/**
 * Gets the quickstart guides (hosted docs, and StackBlitz sample where runnable) for a given
 * template ID. Most templates have exactly one; templates that cover more than one platform SDK
 * (e.g. the generic Mobile template covering iOS, Android, and Flutter) return one entry per
 * platform.
 * @param templateId - The template ID (e.g., 'react', 'react-embedded', 'nextjs', 'mobile')
 * @returns The quickstart guides, or null when this template has none
 */
export default function getQuickstartsForTemplate(templateId: string | undefined): QuickstartLink[] | null {
  if (!templateId) {
    return null;
  }

  // Normalize the template ID to handle embedded variants (e.g., 'react-embedded' -> 'react')
  const normalizedTemplateId = normalizeTemplateId(templateId) ?? templateId;

  const techTemplate = TechnologyBasedApplicationTemplateMetadata.find(
    (metadata) => metadata.template.id === normalizedTemplateId,
  );

  if (techTemplate?.template.quickstarts) {
    return techTemplate.template.quickstarts;
  }

  const platformTemplate = PlatformBasedApplicationTemplateMetadata.find(
    (metadata) => metadata.template.id === normalizedTemplateId,
  );

  if (platformTemplate?.template.quickstarts) {
    return platformTemplate.template.quickstarts;
  }

  return null;
}
