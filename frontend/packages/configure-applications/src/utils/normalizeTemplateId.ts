// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import TemplateConstants from '../constants/template-constants';

/**
 * Normalizes a template ID by removing the embedded suffix.
 * This allows looking up template metadata for both base and embedded templates.
 *
 * @param templateId - The template ID (e.g., 'react', 'react-embedded')
 * @returns The normalized template ID without the embedded suffix (e.g., 'react')
 *
 * @example
 * ```ts
 * normalizeTemplateId('react-embedded') // Returns: 'react'
 * normalizeTemplateId('react') // Returns: 'react'
 * normalizeTemplateId('nextjs') // Returns: 'nextjs'
 * ```
 */
export default function normalizeTemplateId(templateId: string | undefined): string | undefined {
  if (!templateId) {
    return templateId;
  }

  return templateId.endsWith(TemplateConstants.EMBEDDED_SUFFIX)
    ? templateId.slice(0, -TemplateConstants.EMBEDDED_SUFFIX.length)
    : templateId;
}
