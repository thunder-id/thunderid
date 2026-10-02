// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useLogger} from '@thunderid/logger/react';
import {Alert, Box, Button, FormControl, FormLabel, Stack, TextField, Typography} from '@wso2/oxygen-ui';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useCreateGatewaySecret from '../api/useCreateGatewaySecret';
import useCreateGatewayVariable from '../api/useCreateGatewayVariable';
import {GATEWAY_VALUE_MAX_LENGTH} from '../constants/gateway-values';
import useGatewayErrorTranslator from '../hooks/useGatewayErrorTranslator';
import type {MissingValues} from '../models/gateway';
import getGatewayValuesErrorMessage from '../utils/getGatewayValuesErrorMessage';
import {validateGatewayValueName, validateGatewayValueValue} from '../utils/validateGatewayValue';

export interface SetMissingValuesFormProps {
  gatewayId: string;
  missing: MissingValues;
}

type Kind = 'variable' | 'secret';

interface Field {
  kind: Kind;
  name: string;
}

const keyOf = (field: Field): string => `${field.kind}:${field.name}`;

/**
 * One input for each variable and secret a version needs that the gateway lacks, saved to the
 * gateway's store. Saving refreshes the check that found them missing, so a name drops out of the
 * form once the gateway holds it.
 */
export default function SetMissingValuesForm({gatewayId, missing}: SetMissingValuesFormProps): JSX.Element {
  const {t} = useTranslation();
  const tForErrors = useGatewayErrorTranslator();
  const logger = useLogger('SetMissingValuesForm');
  const createVariable = useCreateGatewayVariable();
  const createSecret = useCreateGatewaySecret();
  const [values, setValues] = useState<Record<string, string>>({});
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState<Error | null>(null);

  const fields: Field[] = [
    ...(missing.secrets ?? []).map((name: string): Field => ({kind: 'secret', name})),
    ...(missing.variables ?? []).map((name: string): Field => ({kind: 'variable', name})),
  ];

  const problemOf = (field: Field): string | undefined => {
    if (validateGatewayValueName(field.name)) {
      return t('gateways:missing.form.invalidName', 'This name cannot be stored on the gateway.');
    }
    if (validateGatewayValueValue(values[keyOf(field)] ?? '') === 'tooLong') {
      return t('gateways:values.form.value.tooLong', 'A value can be at most {{max}} characters.', {
        max: GATEWAY_VALUE_MAX_LENGTH,
      });
    }
    return undefined;
  };

  const filled = fields.filter((field: Field) => (values[keyOf(field)] ?? '') !== '');
  const canSave = filled.length > 0 && filled.every((field: Field) => !problemOf(field)) && !isSaving;

  const handleChange = (field: Field, next: string): void => {
    setError(null);
    setValues((prev) => ({...prev, [keyOf(field)]: next}));
  };

  const handleSave = (): void => {
    if (!canSave) return;
    setIsSaving(true);
    setError(null);

    // One at a time, so a failure stops at the value it names and keeps the ones not yet sent.
    filled
      .reduce(
        (previous: Promise<void>, field: Field) =>
          previous.then(async (): Promise<void> => {
            const data = {name: field.name, value: values[keyOf(field)]};
            if (field.kind === 'secret') {
              await createSecret.mutateAsync({gatewayId, data});
            } else {
              await createVariable.mutateAsync({gatewayId, data});
            }
            setValues((prev) => Object.fromEntries(Object.entries(prev).filter(([key]) => key !== keyOf(field))));
          }),
        Promise.resolve(),
      )
      .catch((err: unknown) => {
        logger.error('Failed to set missing gateway values', {error: err});
        setError(err instanceof Error ? err : new Error(String(err)));
      })
      .finally(() => {
        setIsSaving(false);
      });
  };

  return (
    <Box sx={{p: 2, border: 1, borderColor: 'divider', borderRadius: 1}}>
      <Stack spacing={2}>
        <Typography variant="body2" color="text.secondary">
          {t(
            'gateways:missing.form.description',
            'Set the missing values on the gateway. Secret values are stored on the gateway and never shown again.',
          )}
        </Typography>
        {fields.map((field: Field) => {
          const id = `missing-${field.kind}-${field.name}-input`;
          const problem = problemOf(field);
          return (
            <FormControl key={keyOf(field)} fullWidth>
              <FormLabel htmlFor={id} sx={{fontFamily: 'monospace'}}>
                {field.name}
              </FormLabel>
              <TextField
                id={id}
                fullWidth
                size="small"
                type={field.kind === 'secret' ? 'password' : 'text'}
                autoComplete={field.kind === 'secret' ? 'new-password' : 'off'}
                value={values[keyOf(field)] ?? ''}
                disabled={isSaving || validateGatewayValueName(field.name) !== undefined}
                onChange={(e) => handleChange(field, e.target.value)}
                placeholder={
                  field.kind === 'secret'
                    ? t('gateways:missing.form.secretPlaceholder', 'Secret value')
                    : t('gateways:missing.form.variablePlaceholder', 'Variable value')
                }
                error={Boolean(problem)}
                helperText={problem}
              />
            </FormControl>
          );
        })}
        {error && (
          <Alert severity="error">
            {getGatewayValuesErrorMessage(
              error,
              tForErrors,
              'missing.form.error',
              'The values could not be saved. Please try again.',
            )}
          </Alert>
        )}
        <Box>
          <Button variant="outlined" onClick={handleSave} disabled={!canSave}>
            {isSaving ? t('common:status.saving', 'Saving...') : t('gateways:missing.form.submit', 'Save values')}
          </Button>
        </Box>
      </Stack>
    </Box>
  );
}
