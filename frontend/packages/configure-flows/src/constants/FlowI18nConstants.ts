// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {I18nDefaultConstants, NamespaceConstants} from '@thunderid/i18n';

/**
 * i18n namespace constants for flow feature translations.
 *
 * @public
 * @remarks
 * These constants define the translation namespaces used for
 * flow-related UI and runtime translations. Use these to reference
 * the correct i18n namespace when rendering or processing flow content.
 *
 * @example
 * // Using in a translation function
 * t(`${FlowI18nConstants.CUSTOM_TRANSLATIONS_NAMESPACE}:form.title`)
 */
const FlowI18nConstants = {
  /**
   * Namespace for custom flow translations (e.g., user-defined or dynamic flows)
   */
  CUSTOM_TRANSLATIONS_NAMESPACE: NamespaceConstants.CUSTOM_NAMESPACE,
  /**
   * Namespaces for built-in flow translations (e.g., sign-in, sign-up, onboarding flows)
   */
  FLOW_TRANSLATIONS_NAMESPACES: [],
  /**
   * Namespace for default flow translations (e.g., built-in or system flows)
   */
  DEFAULT_LANGUAGE: I18nDefaultConstants.FALLBACK_LANGUAGE,
} as const;

export default FlowI18nConstants;
