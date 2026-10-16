// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {MenuItem, Select, Stack, TextField} from '@wso2/oxygen-ui';
import {type JSX, useState} from 'react';
import {useTranslation} from 'react-i18next';

/**
 * Common separators for a delimited string claim (e.g. a space-delimited OAuth `scope` claim, or a
 * comma-joined list). The backend splits on whatever string is configured, so these are presets for
 * discoverability, not an enforced set — CUSTOM_DELIMITER reveals a free-text field for anything else.
 */
const DELIMITER_PRESETS: {value: string; labelKey: string; labelFallback: string}[] = [
  {
    value: '',
    labelKey: 'authorizationRules.delimiter.preset.none',
    labelFallback: 'No delimiter (single value)',
  },
  {value: ' ', labelKey: 'authorizationRules.delimiter.preset.space', labelFallback: 'Space'},
  {value: ',', labelKey: 'authorizationRules.delimiter.preset.comma', labelFallback: 'Comma ( , )'},
  {value: ';', labelKey: 'authorizationRules.delimiter.preset.semicolon', labelFallback: 'Semicolon ( ; )'},
  {value: '|', labelKey: 'authorizationRules.delimiter.preset.pipe', labelFallback: 'Pipe ( | )'},
];
const CUSTOM_DELIMITER = '__custom__';

export default function DelimiterField({
  value,
  onChange,
}: {
  value: string;
  onChange: (value: string) => void;
}): JSX.Element {
  const {t} = useTranslation('connections');
  const isPreset = (candidate: string): boolean => DELIMITER_PRESETS.some((preset) => preset.value === candidate);
  const [customMode, setCustomMode] = useState<boolean>(!isPreset(value));

  return (
    <Stack direction="column" spacing={1}>
      <Select
        value={customMode ? CUSTOM_DELIMITER : value}
        onChange={(e) => {
          const next = e.target.value;
          if (next === CUSTOM_DELIMITER) {
            setCustomMode(true);
          } else {
            setCustomMode(false);
            onChange(next);
          }
        }}
        inputProps={{'aria-label': t('authorizationRules.delimiter.label')}}
      >
        {DELIMITER_PRESETS.map((preset) => (
          <MenuItem key={preset.value} value={preset.value}>
            {t(preset.labelKey, preset.labelFallback)}
          </MenuItem>
        ))}
        <MenuItem value={CUSTOM_DELIMITER}>{t('authorizationRules.delimiter.preset.custom', 'Custom…')}</MenuItem>
      </Select>
      {customMode && (
        <TextField
          placeholder={t('authorizationRules.delimiter.custom.placeholder')}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          inputProps={{'aria-label': t('authorizationRules.delimiter.custom.label')}}
        />
      )}
    </Stack>
  );
}
