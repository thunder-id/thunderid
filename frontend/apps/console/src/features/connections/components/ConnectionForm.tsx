/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {FormControl, FormLabel, Stack, TextField, Typography} from '@wso2/oxygen-ui';
import {type JSX, useEffect, useMemo, useState} from 'react';
import {useTranslation} from 'react-i18next';
import MaskedSecretField from './MaskedSecretField';
import ReadOnlyCopyField from './ReadOnlyCopyField';
import {CONNECTION_FORM_FIELDS, type ConnectionFieldDef} from '../config/connectionFormFields';
import type {ConnectionType} from '../models/connection';
import {type ConnectionFormValues, validateConnectionForm} from '../utils/connectionFormMapping';

export interface ConnectionFormSnapshot {
  values: ConnectionFormValues;
  secretReplacing: boolean;
  valid: boolean;
}

interface ConnectionFormProps {
  type: ConnectionType;
  mode: 'create' | 'edit';
  initialValues: ConnectionFormValues;
  /** True when editing a connection whose secret is already stored. */
  hasStoredSecret: boolean;
  vendorDisplayName: string;
  /** External error to show on the name field (e.g. duplicate-name 409). */
  nameError?: string | null;
  /** Render the connection-name field (custom connections only; branded names are fixed). */
  showNameField?: boolean;
  /** Render the redirect URI field (moved to a quick-copy section on the edit page). */
  showRedirectUri?: boolean;
  onChange: (snapshot: ConnectionFormSnapshot) => void;
}

export default function ConnectionForm({
  type,
  mode,
  initialValues,
  hasStoredSecret,
  vendorDisplayName,
  nameError = null,
  showNameField = true,
  showRedirectUri = true,
  onChange,
}: ConnectionFormProps): JSX.Element {
  const {t} = useTranslation('connections');
  const fields: ConnectionFieldDef[] = useMemo(
    () =>
      CONNECTION_FORM_FIELDS[type].filter(
        (field) => (showNameField || field.name !== 'name') && (showRedirectUri || field.name !== 'redirectUri'),
      ),
    [type, showNameField, showRedirectUri],
  );

  const [values, setValues] = useState<ConnectionFormValues>(initialValues);
  const [secretReplacing, setSecretReplacing] = useState(false);
  const [touched, setTouched] = useState<Record<string, boolean>>({});

  const errors: Record<string, string> = useMemo(
    () => validateConnectionForm(values, fields, mode),
    [values, fields, mode],
  );

  useEffect(() => {
    onChange({
      values,
      secretReplacing,
      valid: Object.keys(errors).length === 0,
    });
  }, [values, secretReplacing, errors, onChange]);

  const setField = (name: string, value: string): void => {
    setValues((prev) => ({...prev, [name]: value}));
  };

  const fieldError = (name: string): string | undefined => {
    if (name === 'name' && nameError) {
      return nameError;
    }
    if (touched[name] && errors[name]) {
      return t(errors[name]);
    }
    return undefined;
  };

  return (
    <Stack direction="column" spacing={3} data-testid="connection-form">
      {fields.map((field) => {
        const label: string = t(field.labelKey);

        if (field.kind === 'secret') {
          return (
            <MaskedSecretField
              key={field.name}
              id={`connection-field-${field.name}`}
              label={label}
              value={values[field.name] ?? ''}
              onChange={(value) => setField(field.name, value)}
              hasStoredSecret={hasStoredSecret}
              replacing={secretReplacing}
              onReplacingChange={setSecretReplacing}
              required={mode === 'create' && field.required}
              error={fieldError(field.name)}
              hint={field.hintKey ? t(field.hintKey) : undefined}
            />
          );
        }

        if (field.kind === 'readonly-copy') {
          return (
            <ReadOnlyCopyField
              key={field.name}
              id={`connection-field-${field.name}`}
              label={label}
              value={values[field.name] ?? ''}
              helperText={
                field.name === 'redirectUri'
                  ? t('form.fields.redirectUri.help', {vendor: vendorDisplayName})
                  : undefined
              }
            />
          );
        }

        const error: string | undefined = fieldError(field.name);
        return (
          <FormControl key={field.name} fullWidth required={field.required} error={Boolean(error)}>
            <FormLabel htmlFor={`connection-field-${field.name}`}>
              {label}
              {field.optional && (
                <Typography component="span" variant="caption" color="text.secondary" sx={{ml: 1}}>
                  {t('form.optional')}
                </Typography>
              )}
            </FormLabel>
            <TextField
              id={`connection-field-${field.name}`}
              fullWidth
              value={values[field.name] ?? ''}
              placeholder={field.placeholder}
              error={Boolean(error)}
              helperText={error ?? (field.hintKey ? t(field.hintKey) : undefined)}
              onChange={(e) => setField(field.name, e.target.value)}
              onBlur={() => setTouched((prev) => ({...prev, [field.name]: true}))}
            />
          </FormControl>
        );
      })}
    </Stack>
  );
}
