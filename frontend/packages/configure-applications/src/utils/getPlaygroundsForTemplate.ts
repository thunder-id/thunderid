// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import normalizeTemplateId from './normalizeTemplateId';
import PlatformBasedApplicationTemplateMetadata from '../config/PlatformBasedApplicationTemplateMetadata';
import TechnologyBasedApplicationTemplateMetadata from '../config/TechnologyBasedApplicationTemplateMetadata';
import type {PlaygroundLink} from '../models/application-templates';

/**
 * Gets the runnable playgrounds (e.g. StackBlitz samples) for a given template ID. Most templates
 * have exactly one, or none for platforms with no runnable environment (e.g. native mobile).
 * @param templateId - The template ID (e.g., 'react', 'react-embedded', 'nextjs', 'mobile')
 * @returns The playgrounds, or null when this template has none
 */
export default function getPlaygroundsForTemplate(templateId: string | undefined): PlaygroundLink[] | null {
  if (!templateId) {
    return null;
  }

  // Normalize the template ID to handle embedded variants (e.g., 'react-embedded' -> 'react')
  const normalizedTemplateId = normalizeTemplateId(templateId) ?? templateId;

  const techTemplate = TechnologyBasedApplicationTemplateMetadata.find(
    (metadata) => metadata.template.id === normalizedTemplateId,
  );

  if (techTemplate?.template.playgrounds) {
    return techTemplate.template.playgrounds;
  }

  const platformTemplate = PlatformBasedApplicationTemplateMetadata.find(
    (metadata) => metadata.template.id === normalizedTemplateId,
  );

  if (platformTemplate?.template.playgrounds) {
    return platformTemplate.template.playgrounds;
  }

  return null;
}
