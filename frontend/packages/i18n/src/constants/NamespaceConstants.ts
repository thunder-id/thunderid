// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * i18n namespace constants for i18n translations.
 *
 * @public
 * @remarks
 * These constants define the translation namespaces used for
 * i18n-related translations. Use these to reference
 * the correct i18n namespace when rendering or processing
 * i18n content.
 *
 * @example
 * // Using in a translation function
 * t(`${NamespaceConstants.CUSTOM_NAMESPACE}:form.title`)
 */
const NamespaceConstants = {
  /**
   * Namespace for custom flow translations (e.g., user-defined or dynamic flows)
   */
  CUSTOM_NAMESPACE: 'custom',
  /**
   * Namespace for home page translations
   */
  HOME: 'home',
  /**
   * Namespace for notification translations. Like the custom namespace, admins may
   * add new keys/values to it from the Console (its defaults are seeded from the
   * server bootstrap resources).
   */
  NOTIFICATION: 'notification',
} as const;

export default NamespaceConstants;
