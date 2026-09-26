// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {JSX} from 'react';
import normalizeTemplateId from './normalizeTemplateId';
import PlatformBasedApplicationTemplateMetadata from '../config/PlatformBasedApplicationTemplateMetadata';
import TechnologyBasedApplicationTemplateMetadata from '../config/TechnologyBasedApplicationTemplateMetadata';

interface TemplateMetadata {
  icon: JSX.Element;
  displayName: string;
}

/**
 * Gets the template metadata (icon and display name) for a given template ID.
 * Automatically normalizes template IDs by removing the '-embedded' suffix,
 * so 'react-embedded' will match the 'react' template metadata.
 *
 * @param templateId - The template ID (e.g., 'react', 'react-embedded', 'browser')
 * @returns Template metadata with icon and display name, or null if not found
 */
export default function getTemplateMetadata(templateId: string | undefined): TemplateMetadata | null {
  if (!templateId) {
    return null;
  }

  // Normalize the template ID by removing the embedded suffix
  const normalizedId = normalizeTemplateId(templateId);

  if (!normalizedId) {
    return null;
  }

  // Search in technology-based templates
  const techTemplate = TechnologyBasedApplicationTemplateMetadata.find(
    (metadata) => metadata.template.id === normalizedId,
  );

  if (techTemplate) {
    return {
      icon: techTemplate.icon,
      displayName: techTemplate.template.displayName ?? normalizedId,
    };
  }

  // Search in platform-based templates
  const platformTemplate = PlatformBasedApplicationTemplateMetadata.find(
    (metadata) => metadata.template.id === normalizedId,
  );

  if (platformTemplate) {
    return {
      icon: platformTemplate.icon,
      displayName: platformTemplate.template.displayName ?? normalizedId,
    };
  }

  return null;
}
