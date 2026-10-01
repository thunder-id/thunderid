// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import normalizeTemplateId from './normalizeTemplateId';
import PlatformBasedApplicationTemplateMetadata from '../config/PlatformBasedApplicationTemplateMetadata';
import TechnologyBasedApplicationTemplateMetadata from '../config/TechnologyBasedApplicationTemplateMetadata';
import type {ApplicationTemplate} from '../models/application-templates';

/**
 * Gets the field constraints for a given template ID.
 * Automatically normalizes template IDs by removing the '-embedded' suffix,
 * so 'react-embedded' will match the 'react' template constraints.
 *
 * @param templateId - The template ID (e.g., 'react', 'react-embedded', 'browser')
 * @returns Template field constraints, or null if not found
 */
export default function getTemplateFieldConstraints(
  templateId: string | undefined,
): ApplicationTemplate['fieldConstraints'] | null {
  if (!templateId) return null;

  const normalizedId = normalizeTemplateId(templateId);
  if (!normalizedId) return null;

  const techTemplate = TechnologyBasedApplicationTemplateMetadata.find(
    (metadata) => metadata.template.id === normalizedId,
  );
  if (techTemplate) return techTemplate.template.fieldConstraints ?? null;

  const platformTemplate = PlatformBasedApplicationTemplateMetadata.find(
    (metadata) => metadata.template.id === normalizedId,
  );
  if (platformTemplate) return platformTemplate.template.fieldConstraints ?? null;

  return null;
}
