// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Button, FormControlLabel, IconButton, Radio, RadioGroup, Stack, Typography} from '@wso2/oxygen-ui';
import {Plus, Trash2} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import DurationField from './DurationField';
import type {AccountLockCategory} from '../../models/accountAccess';

/** The default first rung, used when there is nothing to escalate from. */
const DEFAULT_RUNG_SECONDS = 300;

/** Fallback copy for the held-lockout keys, matching en-US. An agent has no account recovery. */
const HELD_FALLBACKS: Record<AccountLockCategory, {option: string; endLabel: string; endHint: string}> = {
  user: {
    option: 'Until account recovery or an administrator unlocks',
    endLabel: 'Require account recovery or administrator intervention',
    endHint:
      'Keep the lockout in place until the user completes account recovery or an administrator unlocks the account.',
  },
  agent: {
    option: 'Until an administrator unlocks',
    endLabel: 'Require administrator intervention',
    endHint: 'Keep the lockout in place until an administrator unlocks the agent.',
  },
};

/** Fallback copy for the ordinal plural forms of the position key, matching en-US. */
const POSITION_FALLBACKS = {
  defaultValue_ordinal_one: '{{count}}st',
  defaultValue_ordinal_two: '{{count}}nd',
  defaultValue_ordinal_few: '{{count}}rd',
  defaultValue_ordinal_other: '{{count}}th',
};

/**
 * Props for {@link LockLadderRows}.
 *
 * @public
 */
export interface LockLadderRowsProps {
  /** The identity category, which decides who can end a held lockout. */
  category: AccountLockCategory;
  /** One duration per consecutive lock, in seconds. A trailing `0` holds later locks until unlocked. */
  durations: number[];
  /** Called with the new list. */
  onChange: (durations: number[]) => void;
  /** Blocking message for the list as a whole. */
  error?: string;
  /** Whether the rows are editable. */
  disabled?: boolean;
}

/**
 * How long a lock lasts, and how repeated locks escalate.
 *
 * The rows hold only durations that lift on their own, so another can always be added. What follows
 * the last one is a separate choice: keep repeating it, or hold later locks until an administrator
 * unlocks, which is stored as a trailing `0`. A `0` stored before the last rung is kept and shown in
 * its row, where it can be changed but not chosen again.
 *
 * @param props - The durations and the handler
 * @returns The rendered rows
 *
 * @public
 */
export default function LockLadderRows({
  category,
  durations,
  onChange,
  error = undefined,
  disabled = false,
}: LockLadderRowsProps): JSX.Element {
  const {t} = useTranslation();
  const endsPermanent: boolean = durations.length > 0 && durations[durations.length - 1] === 0;
  const timed: number[] = endsPermanent ? durations.slice(0, -1) : durations;
  const numbered: boolean = timed.length > 1 || endsPermanent;
  const held = HELD_FALLBACKS[category];
  const untilUnlockedLabel: string = t(`settings:accountLock.ladder.untilUnlocked.${category}`, held.option);

  const emit = (rungs: number[], permanent: boolean = endsPermanent): void =>
    onChange(permanent ? [...rungs, 0] : rungs);

  const rowLabel = (index: number): string => {
    // `position`, not `ordinal`: i18next reserves the latter for plural selection.
    const position: string = t('settings:accountLock.ladder.position', {
      ...POSITION_FALLBACKS,
      count: index + 1,
      ordinal: true,
    });
    return !endsPermanent && index === timed.length - 1
      ? t('settings:accountLock.ladder.rowLast', '{{position}} and later', {position})
      : t('settings:accountLock.ladder.row', '{{position}} lockout', {position});
  };

  /** Doubles the last rung to suggest the next. */
  const nextRungSeconds = (): number => {
    const last: number = timed[timed.length - 1] ?? 0;
    return last > 0 ? last * 2 : DEFAULT_RUNG_SECONDS;
  };

  // A ladder that holds every lock has no timed rung to explain, and the policy summary says so.
  const permanentOnly: boolean = timed.length === 0 && endsPermanent;
  let hint: string | undefined;
  if (timed.length > 1) {
    hint = t('settings:accountLock.ladder.hint', 'Each successive lockout uses the next duration in the list.');
  } else if (!permanentOnly) {
    hint = t('settings:accountLock.ladder.hintSingle', 'How long a lockout lasts before it ends on its own.');
  }

  const endLabel: string = t('settings:accountLock.ladder.end.label', 'After the final duration');

  return (
    <Box>
      <Typography variant="subtitle2" sx={{mb: 0.5}}>
        {t('settings:accountLock.ladder.label', 'Lockout duration')}
      </Typography>
      {hint && (
        <Typography variant="body2" color="text.secondary" sx={{mb: 1.5}}>
          {hint}
        </Typography>
      )}

      <Stack spacing={1}>
        {timed.map((seconds, index) => (
          // The list is positional and its entries are not unique, so the index is the identity.
          // eslint-disable-next-line react/no-array-index-key
          <Stack key={index} direction="row" spacing={1.5} alignItems="center">
            {numbered && (
              <Typography variant="body2" sx={{minWidth: 130}}>
                {rowLabel(index)}
              </Typography>
            )}
            <DurationField
              inline
              label={numbered ? rowLabel(index) : t('settings:accountLock.ladder.label', 'Lockout duration')}
              seconds={seconds}
              disabled={disabled}
              zeroLabel={seconds === 0 ? untilUnlockedLabel : undefined}
              testId={`lock-ladder-row-${index}`}
              onChange={(next) => emit(timed.map((value, i) => (i === index ? next : value)))}
            />
            {numbered && (
              <IconButton
                aria-label={t('settings:accountLock.ladder.remove', 'Remove the {{position}} duration', {
                  position: rowLabel(index),
                })}
                size="small"
                disabled={disabled}
                onClick={() => emit(timed.filter((_, i) => i !== index))}
              >
                <Trash2 size={16} />
              </IconButton>
            )}
          </Stack>
        ))}
      </Stack>

      {error && (
        <Typography variant="body2" color="error" sx={{mt: 1}}>
          {error}
        </Typography>
      )}

      <Button
        variant="text"
        color="primary"
        size="small"
        startIcon={<Plus size={16} />}
        disabled={disabled}
        sx={{mt: 1}}
        onClick={() => emit([...timed, nextRungSeconds()])}
      >
        {timed.length === 1 && !endsPermanent
          ? t('settings:accountLock.ladder.addFirstEscalation', 'Add a longer duration for subsequent lockouts')
          : t('settings:accountLock.ladder.add', 'Add duration')}
      </Button>

      {timed.length > 0 && (
        <Box sx={{mt: 2}}>
          <Typography variant="subtitle2" sx={{mb: 0.5}}>
            {endLabel}
          </Typography>
          <RadioGroup
            aria-label={endLabel}
            value={endsPermanent ? 'untilUnlocked' : 'repeat'}
            onChange={(event) => emit(timed, event.target.value === 'untilUnlocked')}
          >
            <FormControlLabel
              value="repeat"
              disabled={disabled}
              control={<Radio size="small" />}
              label={
                <Box sx={{py: 0.5}}>
                  <Typography variant="body2">
                    {t('settings:accountLock.ladder.end.repeat', 'Continue using the final duration')}
                  </Typography>
                  <Typography variant="body2" color="text.secondary">
                    {t(
                      'settings:accountLock.ladder.end.repeatHint',
                      'Apply the same duration to all subsequent lockouts.',
                    )}
                  </Typography>
                </Box>
              }
            />
            <FormControlLabel
              value="untilUnlocked"
              disabled={disabled}
              control={<Radio size="small" />}
              label={
                <Box sx={{py: 0.5}}>
                  <Typography variant="body2">
                    {t(`settings:accountLock.ladder.end.untilUnlocked.${category}`, held.endLabel)}
                  </Typography>
                  <Typography variant="body2" color="text.secondary">
                    {t(`settings:accountLock.ladder.end.untilUnlockedHint.${category}`, held.endHint)}
                  </Typography>
                </Box>
              }
            />
          </RadioGroup>
        </Box>
      )}
    </Box>
  );
}
