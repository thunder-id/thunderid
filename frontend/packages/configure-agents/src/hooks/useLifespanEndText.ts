// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useCallback} from 'react';
import {useTranslation} from 'react-i18next';
import formatRelativeTime from '../utils/formatRelativeTime';

/**
 * Returns a function that says when an agent's lifespan ends relative to now, "Expires in 3 months"
 * or "Expired 13 days ago". It returns undefined for an agent that does not expire, or an end that does
 * not parse.
 */
export default function useLifespanEndText(): (validUntil?: string | null, now?: Date) => string | undefined {
  const {t, i18n} = useTranslation();
  // The relative phrase follows the Console's language; without one, the browser's.
  const language: string | undefined = i18n?.language;

  return useCallback(
    (validUntil?: string | null, now: Date = new Date()): string | undefined => {
      if (!validUntil) return undefined;
      const end = new Date(validUntil);
      if (Number.isNaN(end.getTime())) return undefined;
      const relative = formatRelativeTime(end, now, language);

      return end.getTime() > now.getTime()
        ? t('agents:lifespan.expiresIn', 'Expires {{relative}}', {relative})
        : t('agents:lifespan.expiredAgo', 'Expired {{relative}}', {relative});
    },
    [language, t],
  );
}
