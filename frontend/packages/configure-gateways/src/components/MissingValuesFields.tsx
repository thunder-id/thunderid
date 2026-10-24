// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Chip, Stack, TextField, Typography} from '@wso2/oxygen-ui';
import {CircleAlert, CircleCheck} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {ValueReference} from '../models/deploy';
import {fieldKey, type MissingField} from '../utils/missingValues';

export interface MissingValuesFieldsProps {
  fields: MissingField[];
  values: Record<string, string>;
  onChange: (field: MissingField, value: string) => void;
  disabled?: boolean;
}

const WARNING = 'var(--oxygen-palette-warning-mainChannel)';

/**
 * An input for each value the gateway lacks: text for a variable, a password field for a secret. The
 * values sit together in a gently marked panel that counts how many are left, and each empty field is
 * outlined until it is filled.
 */
export default function MissingValuesFields({
  fields,
  values,
  onChange,
  disabled = false,
}: MissingValuesFieldsProps): JSX.Element {
  const {t} = useTranslation('deploy');
  const filled = fields.filter((field: MissingField) => (values[fieldKey(field)] ?? '').trim() !== '').length;
  const done = filled === fields.length;
  return (
    <Stack
      spacing={1.5}
      data-testid="missing-values"
      sx={{
        p: 2,
        borderRadius: 2,
        border: 1,
        borderColor: done ? 'divider' : `rgba(${WARNING} / 0.45)`,
        borderLeft: 4,
        borderLeftColor: done ? 'success.main' : 'warning.main',
        bgcolor: done ? 'transparent' : `rgba(${WARNING} / 0.06)`,
        transition: 'background-color 0.2s, border-color 0.2s',
      }}
    >
      <Stack direction="row" alignItems="center" spacing={1}>
        <Box sx={{display: 'flex', color: done ? 'success.main' : 'warning.main'}}>
          {done ? <CircleCheck size={18} /> : <CircleAlert size={18} />}
        </Box>
        <Typography variant="subtitle2" sx={{flex: 1}}>
          {done ? t('missing.allSet', 'All values set') : t('missing.title', 'Values this gateway needs')}
        </Typography>
        <Chip
          size="small"
          variant="outlined"
          color={done ? 'success' : 'warning'}
          label={t('missing.progress', '{{filled}} of {{total}} set', {filled, total: fields.length})}
          sx={{height: 20, fontSize: 11}}
        />
      </Stack>
      {fields.map((field: MissingField) => {
        const id = `missing-${field.kind}-${field.name}`;
        const where = field.references
          .map((reference: ValueReference) => [reference.resourceName, reference.field].filter(Boolean).join(' · '))
          .filter(Boolean)
          .join(', ');
        const empty = (values[fieldKey(field)] ?? '').trim() === '';
        return (
          <Box key={fieldKey(field)} data-empty={empty}>
            <Typography
              component="label"
              htmlFor={id}
              variant="caption"
              sx={{fontFamily: 'monospace', display: 'flex', alignItems: 'center', gap: 0.75, mb: 0.5}}
            >
              <Box
                component="span"
                aria-hidden
                sx={{display: 'inline-flex', color: empty ? 'warning.main' : 'success.main'}}
              >
                {empty ? <CircleAlert size={12} /> : <CircleCheck size={12} />}
              </Box>
              {field.name}
              {where && (
                <Typography component="span" variant="caption" color="text.secondary" sx={{fontFamily: 'inherit'}}>
                  {where}
                </Typography>
              )}
              {empty && (
                <Typography component="span" variant="caption" sx={{ml: 'auto', color: 'warning.main'}}>
                  {t('missing.needed', 'Needed')}
                </Typography>
              )}
            </Typography>
            <TextField
              id={id}
              size="small"
              fullWidth
              type={field.kind === 'secret' ? 'password' : 'text'}
              multiline={field.list}
              minRows={field.list ? 2 : undefined}
              autoComplete={field.kind === 'secret' ? 'new-password' : 'off'}
              value={values[fieldKey(field)] ?? ''}
              onChange={(event) => onChange(field, event.target.value)}
              disabled={disabled}
              placeholder={
                field.kind === 'secret'
                  ? t('missing.secretPlaceholder', 'Secret value')
                  : t('missing.variablePlaceholder', 'Variable value')
              }
              helperText={field.list ? t('missing.listHint', 'One per line.') : undefined}
              sx={{
                '& .MuiOutlinedInput-root': {bgcolor: 'background.paper'},
                ...(empty
                  ? {
                      '& .MuiOutlinedInput-root:not(.Mui-focused) .MuiOutlinedInput-notchedOutline': {
                        borderColor: `rgba(${WARNING} / 0.6)`,
                      },
                    }
                  : {}),
              }}
            />
          </Box>
        );
      })}
    </Stack>
  );
}
