// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Stack, Tooltip, Typography} from '@wso2/oxygen-ui';
import {Calendar, Clock} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';

export interface TimestampProps {
  value?: string;
  /** Leaves the time out, for a row that has little room. */
  dateOnly?: boolean;
}

const UNITS: [Intl.RelativeTimeFormatUnit, number][] = [
  ['year', 365 * 24 * 3600],
  ['month', 30 * 24 * 3600],
  ['day', 24 * 3600],
  ['hour', 3600],
  ['minute', 60],
];

/** How long ago a moment was, "3 hours ago". */
function relative(date: Date): string {
  const seconds = Math.round((date.getTime() - Date.now()) / 1000);
  const format = new Intl.RelativeTimeFormat(undefined, {numeric: 'auto'});
  for (const [unit, size] of UNITS) {
    if (Math.abs(seconds) >= size) return format.format(Math.round(seconds / size), unit);
  }
  return format.format(seconds, 'second');
}

/**
 * A moment as the deploy path shows one: a calendar and the date, a clock and the time. The full
 * moment, with its time zone, how long ago it was and the same moment in UTC, is a hover away.
 */
export default function Timestamp({value = undefined, dateOnly = false}: TimestampProps): JSX.Element | null {
  const {t} = useTranslation('deploy');
  if (!value) return null;
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return (
      <Typography variant="caption" color="text.secondary">
        {value}
      </Typography>
    );
  }
  const details = (
    <Stack spacing={0.25}>
      <Typography variant="caption" component="span">
        {new Intl.DateTimeFormat(undefined, {dateStyle: 'full', timeStyle: 'long'}).format(date)}
      </Typography>
      <Typography variant="caption" component="span" sx={{opacity: 0.8}}>
        {relative(date)}
      </Typography>
      <Typography variant="caption" component="span" sx={{opacity: 0.8, fontFamily: 'monospace'}}>
        {t('time.utc', '{{time}} UTC', {time: date.toISOString().replace('T', ' ').slice(0, 19)})}
      </Typography>
    </Stack>
  );
  const part = (icon: JSX.Element, text: string): JSX.Element => (
    <Stack direction="row" alignItems="center" spacing={0.5} sx={{color: 'text.secondary'}}>
      <Box sx={{display: 'flex'}} aria-hidden>
        {icon}
      </Box>
      <Typography variant="caption" component="span">
        {text}
      </Typography>
    </Stack>
  );
  return (
    <Tooltip title={details}>
      <Stack direction="row" alignItems="center" spacing={1.25} sx={{width: 'fit-content'}} data-testid="timestamp">
        {part(<Calendar size={12} />, date.toLocaleDateString(undefined, {dateStyle: 'medium'}))}
        {!dateOnly && part(<Clock size={12} />, date.toLocaleTimeString(undefined, {timeStyle: 'short'}))}
      </Stack>
    </Tooltip>
  );
}
