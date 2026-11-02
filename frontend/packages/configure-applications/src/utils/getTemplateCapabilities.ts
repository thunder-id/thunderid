// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import normalizeTemplateId from './normalizeTemplateId';
import PlatformBasedApplicationTemplateMetadata from '../config/PlatformBasedApplicationTemplateMetadata';
import TechnologyBasedApplicationTemplateMetadata from '../config/TechnologyBasedApplicationTemplateMetadata';
import type {ApplicationTemplate} from '../models/application-templates';

/**
 * Gets the capability flags for a given template ID.
 * Automatically normalizes template IDs by removing the '-embedded' suffix, so 'mobile-embedded'
 * resolves to the 'mobile' template's capabilities.
 *
 * @param templateId - The template ID (e.g., 'mobile', 'mobile-embedded', 'browser')
 * @returns Template capability flags, or null if not found
 */
export default function getTemplateCapabilities(
  templateId: string | undefined,
): ApplicationTemplate['capabilities'] | null {
  if (!templateId) return null;

  const normalizedId = normalizeTemplateId(templateId);
  if (!normalizedId) return null;

  const techTemplate = TechnologyBasedApplicationTemplateMetadata.find(
    (metadata) => metadata.template.id === normalizedId,
  );
  if (techTemplate) return techTemplate.template.capabilities ?? null;

  const platformTemplate = PlatformBasedApplicationTemplateMetadata.find(
    (metadata) => metadata.template.id === normalizedId,
  );
  if (platformTemplate) return platformTemplate.template.capabilities ?? null;

  return null;
}
