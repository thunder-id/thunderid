// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {AppLanguages} from '@thunderid/system-apps';
import {useEffect} from 'react';
import {useTranslation} from 'react-i18next';

/**
 * Where the language an admin picks is kept, so the app opens in it next time.
 */
const LANGUAGE_STORAGE_KEY = 'thunderid-admin-language';

const readStoredLanguage = (): string | null => {
  try {
    return window.localStorage.getItem(LANGUAGE_STORAGE_KEY);
  } catch {
    return null;
  }
};

/**
 * Keeps the language an admin picks for their next visit.
 */
export const storeAppLanguage = (language: string): void => {
  try {
    window.localStorage.setItem(LANGUAGE_STORAGE_KEY, language);
  } catch {
    // The choice then lasts only until the page reloads.
  }
};

/**
 * Applies the Admin App's languages: the language the admin picked, when the menu is on and it is
 * still offered, else the configured default. Without a configured default the app keeps its own.
 */
export default function useAppLanguage(languages: AppLanguages | undefined): void {
  const {i18n} = useTranslation();
  const offered: string[] = languages?.offered ?? [];
  const stored: string | null = languages?.menu ? readStoredLanguage() : null;
  const language: string | undefined = stored && offered.includes(stored) ? stored : languages?.default;

  useEffect(() => {
    if (language && i18n.language !== language) {
      void i18n.changeLanguage(language);
    }
    // Applied when the configuration loads; the menu changes the language itself afterwards.
  }, [i18n, language]);
}
