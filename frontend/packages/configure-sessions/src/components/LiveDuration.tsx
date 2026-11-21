// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Typography} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useNow from '../hooks/useNow';
import formatDuration from '../utils/formatDuration';

export interface LiveDurationProps {
  /** An ISO 8601 timestamp. */
  value: string;
  /** Whether to count down to the timestamp, or up from it. */
  direction: 'until' | 'since';
}

/**
 * The time left until, or gone since, a timestamp, to the minute and kept current. It is its own
 * component so that each refresh re-renders only this line.
 */
export default function LiveDuration({value, direction}: LiveDurationProps): JSX.Element {
  const {t, i18n} = useTranslation('sessions');
  // Shown to the minute, so a tick every few seconds keeps it current without re-rendering every second.
  const now = useNow(10 * 1000);
  const seconds = Math.floor((new Date(value).getTime() - now) / 1000) * (direction === 'until' ? 1 : -1);

  let text: string;
  if (direction === 'until' && seconds <= 0) {
    text = t('details.ended', 'Ended');
  } else if (seconds < 60) {
    text = direction === 'until' ? t('details.endingNow', 'Ending now') : t('details.justNow', 'Just now');
  } else {
    const time = formatDuration(seconds, i18n.language);
    text =
      direction === 'until' ? t('details.endsIn', 'in {{time}}', {time}) : t('details.ago', '{{time}} ago', {time});
  }

  return (
    <Typography variant="caption" color="text.secondary" component="div">
      {text}
    </Typography>
  );
}
