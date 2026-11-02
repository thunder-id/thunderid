// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {MenuItem, Stack, TextField} from '@wso2/oxygen-ui';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {toDurationParts, toSeconds, type DurationParts, type DurationUnit} from '../../utils/duration';

interface DurationDraft {
  unit: DurationUnit;
  text: string;
}

function toDraft(seconds: number): DurationDraft {
  const parts: DurationParts = toDurationParts(seconds);
  return {unit: parts.unit, text: parts.value === 0 ? '' : String(parts.value)};
}

/** The typed number when it is a positive whole number, otherwise undefined. */
function parseWhole(text: string): number | undefined {
  const value = Number(text);
  return text.trim() !== '' && Number.isInteger(value) && value > 0 ? value : undefined;
}

/**
 * Props for {@link DurationField}.
 *
 * @public
 */
export interface DurationFieldProps {
  /** Field label. */
  label: string;
  /** The stored value, in seconds. */
  seconds: number;
  /** Called with the new value in seconds. */
  onChange: (seconds: number) => void;
  /** Helper text shown below the field. */
  helperText?: string;
  /** Blocking message shown instead of the helper text. */
  error?: string;
  /** Whether the field is editable. */
  disabled?: boolean;
  /**
   * When given, the unit list gains an entry meaning zero, labelled with this text. Zero is a real
   * value in this configuration rather than an absent one, and it means different things per field,
   * so the caller names it.
   */
  zeroLabel?: string;
  /**
   * Test id for the number input. The unit select takes the same id with a `-unit` suffix, because
   * a screen can hold several of these fields and "the unit dropdown" is otherwise ambiguous.
   */
  testId?: string;
  /**
   * Renders without visible labels, for a field that sits inside a sentence. Both controls still carry
   * their names as `aria-label`, so neither is unnamed.
   */
  inline?: boolean;
}

/**
 * A duration entered as a number and a unit, stored as seconds. The typed text and unit are kept
 * locally and re-derived (largest exact unit) only when `seconds` changes from outside, such as on
 * load or reset. Only a positive whole number is emitted; an empty box keeps the last valid value and
 * shows it again on blur.
 *
 * @param props - The field's value and labels
 * @returns The rendered field
 *
 * @public
 */
export default function DurationField({
  label,
  seconds,
  onChange,
  helperText = undefined,
  error = undefined,
  disabled = false,
  zeroLabel = undefined,
  testId = undefined,
  inline = false,
}: DurationFieldProps): JSX.Element {
  const {t} = useTranslation();
  const [draft, setDraft] = useState<DurationDraft>(() => toDraft(seconds));
  const [lastSeconds, setLastSeconds] = useState<number>(seconds);
  const isZero: boolean = zeroLabel !== undefined && seconds === 0;

  if (seconds !== lastSeconds) {
    setLastSeconds(seconds);
    setDraft(toDraft(seconds));
  }

  const emit = (next: number): void => {
    setLastSeconds(next);
    onChange(next);
  };

  // The unit reads with the number beside it, so "1 Minute" rather than "1 Minutes".
  const one: boolean = !isZero && parseWhole(draft.text) === 1;
  const unitOptions: {value: string; label: string}[] = [
    {
      value: 'minutes',
      label: one
        ? t('settings:accountLock.units.minute', 'Minute')
        : t('settings:accountLock.units.minutes', 'Minutes'),
    },
    {
      value: 'hours',
      label: one ? t('settings:accountLock.units.hour', 'Hour') : t('settings:accountLock.units.hours', 'Hours'),
    },
    {
      value: 'days',
      label: one ? t('settings:accountLock.units.day', 'Day') : t('settings:accountLock.units.days', 'Days'),
    },
    {
      value: 'seconds',
      label: one
        ? t('settings:accountLock.units.second', 'Second')
        : t('settings:accountLock.units.seconds', 'Seconds'),
    },
  ];
  if (zeroLabel !== undefined) {
    unitOptions.push({value: 'zero', label: zeroLabel});
  }

  const handleUnitChange = (next: string): void => {
    if (next === 'zero') {
      emit(0);
      return;
    }
    // Keep the number shown and re-scale it, so 15 minutes becomes 15 hours.
    const unit = next as DurationUnit;
    const value: number = (isZero ? undefined : parseWhole(draft.text)) ?? 1;
    setDraft({unit, text: String(value)});
    emit(toSeconds(value, unit));
  };

  // An empty or invalid box shows the stored value again once focus leaves.
  const handleBlur = (): void => {
    if (parseWhole(draft.text) === undefined) {
      setDraft(toDraft(seconds));
    }
  };

  // Zero is a deliberate setting, so clearing the box never emits it.
  const handleNumberChange = (raw: string): void => {
    setDraft({...draft, text: raw});
    const value: number | undefined = parseWhole(raw);
    if (value !== undefined) {
      emit(toSeconds(value, draft.unit));
    }
  };

  const unitLabel: string = t('settings:accountLock.units.label', 'Unit');
  // With the number box hidden, the select carries the field's label and error.
  const selectLabel: string = isZero ? label : unitLabel;
  let selectHelperText: string | undefined;
  if (isZero) {
    selectHelperText = inline ? error : (error ?? helperText);
  } else if (!inline) {
    selectHelperText = ' ';
  }

  return (
    <Stack direction="row" spacing={1} useFlexGap alignItems="flex-start" sx={{flexWrap: 'wrap', flexShrink: 0}}>
      {!isZero && (
        <TextField
          label={inline ? undefined : label}
          type="number"
          size="small"
          value={draft.text}
          disabled={disabled}
          error={Boolean(error)}
          helperText={inline ? error : (error ?? helperText)}
          slotProps={{htmlInput: {min: 1, 'aria-label': label, 'data-testid': testId}}}
          onChange={(event) => handleNumberChange(event.target.value)}
          onBlur={handleBlur}
          sx={{width: inline ? 96 : 140, '& .MuiInputBase-root': {height: 40}}}
        />
      )}
      <TextField
        select
        label={inline ? undefined : selectLabel}
        data-testid={testId === undefined ? undefined : `${testId}-unit`}
        size="small"
        value={isZero ? 'zero' : draft.unit}
        disabled={disabled}
        helperText={selectHelperText}
        error={isZero && Boolean(error)}
        onChange={(event) => handleUnitChange(event.target.value)}
        slotProps={{htmlInput: {'aria-label': selectLabel}}}
        sx={{minWidth: isZero ? 220 : 140, '& .MuiInputBase-root': {height: 40}}}
      >
        {unitOptions.map((option) => (
          <MenuItem key={option.value} value={option.value}>
            {option.label}
          </MenuItem>
        ))}
      </TextField>
    </Stack>
  );
}
