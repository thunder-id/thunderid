// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

const DOCUMENTATION_LINK_KEY_PATTERN = /^\{\{(.+)\}\}$/;

/**
 * Resolves a template field that may hold either a literal URL or a `{{documentation.links key}}`
 * reference (e.g. '{{applications.templates.react.docs}}'), matching the `{{productName}}`/
 * `{{clientId}}` placeholder convention already used elsewhere in these templates, so the docs
 * site stays the source of truth. A value that isn't wrapped in `{{ }}` is used as-is, which keeps
 * not-yet-migrated templates working unchanged. A `{{ }}`-wrapped reference that isn't configured
 * resolves to undefined rather than the raw placeholder, so callers can hide the dependent UI.
 *
 * Used for {@link QuickstartLink.docsUrl}, {@link PlaygroundLink.url}, and
 * {@link IntegrationGuide.docsUrl}.
 *
 * @public
 */
export default function resolveTemplateLink(
  value: string | undefined,
  getDocumentationLink: (key: string) => string | undefined,
): string | undefined {
  if (!value) {
    return undefined;
  }
  const keyMatch = DOCUMENTATION_LINK_KEY_PATTERN.exec(value);
  if (!keyMatch) {
    return value;
  }
  return getDocumentationLink(keyMatch[1]);
}
