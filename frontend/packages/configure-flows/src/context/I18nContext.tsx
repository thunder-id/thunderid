// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Context} from 'react';
import {createContext} from 'react';
import {PreviewScreenType} from '../models/custom-text-preference';

/**
 * Props interface of {@link I18nContext}
 */
export interface I18nContextProps {
  /**
   * The primary i18n screen for the flow.
   */
  primaryI18nScreen: PreviewScreenType;
  /**
   * Configured i18n text from the branding or default fallback.
   */
  i18nText?: Partial<Record<PreviewScreenType, Record<string, string>>>;
  /**
   * Indicates whether the i18n text is still loading.
   */
  i18nTextLoading?: boolean;
  /**
   * The language of the i18n text.
   */
  language?: string;
  /**
   * Sets the language for the i18n text.
   */
  setLanguage?: (language: string) => void;
  /**
   * Updates an existing i18n key for the specified screen.
   */
  updateI18nKey?: (screenType: string, language: string, i18nText: Record<string, string>) => Promise<boolean>;
  /**
   * Indicates whether the i18n key related operations are in progress.
   */
  isI18nSubmitting?: boolean;
  /**
   * Function to check if a given i18n key is custom.
   */
  isCustomI18nKey?: (key: string, excludePrimaryScreen?: boolean) => boolean;
  /**
   * Supported locales for the custom text preferences.
   */
  supportedLocales?: Record<string, {code: string; name: string; flag: string}>;
}

const I18nContext: Context<I18nContextProps | undefined> = createContext<I18nContextProps | undefined>(undefined);

I18nContext.displayName = 'I18nContext';

export default I18nContext;
