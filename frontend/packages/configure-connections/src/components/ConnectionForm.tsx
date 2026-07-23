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

import {
  Box,
  Collapse,
  Divider,
  FormControl,
  FormControlLabel,
  FormLabel,
  Stack,
  Switch,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import {type JSX, type ReactNode, useMemo, useState} from 'react';
import {Trans, useTranslation} from 'react-i18next';
import MaskedSecretField from './MaskedSecretField';
import ReadOnlyCopyField from './ReadOnlyCopyField';
import {fieldsForMode, type ConnectionFieldDef} from '../config/connectionFormFields';
import type {ConnectionType} from '../models/connection';
import {type ConnectionFormValues, validateConnectionForm} from '../utils/connectionFormMapping';

interface ConnectionFormProps {
  type: ConnectionType;
  mode: 'create' | 'edit';
  /** Full field values to display (baseline merged with any edits). */
  values: ConnectionFormValues;
  /** Whether the user has chosen to replace the stored secret. */
  secretReplacing: boolean;
  /** True when editing a connection whose secret is already stored. */
  hasStoredSecret: boolean;
  vendorDisplayName: string;
  /** External error to show on the name field (e.g. duplicate-name 409). */
  nameError?: string | null;
  /** Render the connection-name field (custom connections only; branded names are fixed). */
  showNameField?: boolean;
  onFieldChange: (name: string, value: string) => void;
  onSecretReplacingChange: (replacing: boolean) => void;
}

export default function ConnectionForm({
  type,
  mode,
  values,
  secretReplacing,
  hasStoredSecret,
  vendorDisplayName,
  nameError = null,
  showNameField = true,
  onFieldChange,
  onSecretReplacingChange,
}: ConnectionFormProps): JSX.Element {
  const {t} = useTranslation('connections');
  const fields: ConnectionFieldDef[] = useMemo(
    () => fieldsForMode(type, mode).filter((field) => showNameField || field.name !== 'name'),
    [type, mode, showNameField],
  );

  const [touched, setTouched] = useState<Record<string, boolean>>({});

  const errors: Record<string, string> = useMemo(
    () => validateConnectionForm(values, fields, mode),
    [values, fields, mode],
  );

  const setField = (name: string, value: string): void => {
    onFieldChange(name, value);
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

  const isRequiredNow = (field: ConnectionFieldDef): boolean => {
    const requiredWhen: string | undefined = field.requiredWhen;
    return Boolean(field.required) || (requiredWhen !== undefined && values[requiredWhen] === 'true');
  };

  // Render a hint, resolving inline <code> markup in the translation to a styled code element.
  const renderHint = (hintKey: string): ReactNode => (
    <Trans
      t={t}
      i18nKey={hintKey}
      components={{
        code: (
          <Box
            component="code"
            sx={{
              fontFamily: 'monospace',
              fontSize: '0.85em',
              color: 'primary.main',
              bgcolor: 'action.selected',
              borderRadius: 0.5,
              px: 0.5,
            }}
          />
        ),
      }}
    />
  );

  return (
    <Stack direction="column" spacing={3} data-testid="connection-form">
      {fields.map((field) => {
        const label: string = t(field.labelKey);
        const visible: boolean = !field.revealedBy || values[field.revealedBy] === 'true';

        let fieldContent: ReactNode;

        if (field.kind === 'switch') {
          fieldContent = (
            <Box>
              <FormControlLabel
                control={
                  <Switch
                    checked={values[field.name] === 'true'}
                    onChange={(e) => setField(field.name, e.target.checked ? 'true' : 'false')}
                    slotProps={{input: {'aria-label': label, role: 'switch'}}}
                  />
                }
                label={<Typography variant="subtitle2">{label}</Typography>}
              />
              {field.hintKey && (
                <Typography variant="caption" color="text.secondary" sx={{display: 'block', ml: '52px'}}>
                  {t(field.hintKey)}
                </Typography>
              )}
            </Box>
          );
        } else if (field.kind === 'secret') {
          fieldContent = (
            <MaskedSecretField
              id={`connection-field-${field.name}`}
              label={label}
              value={values[field.name] ?? ''}
              onChange={(value) => setField(field.name, value)}
              hasStoredSecret={hasStoredSecret}
              replacing={secretReplacing}
              onReplacingChange={onSecretReplacingChange}
              required={mode === 'create' && field.required}
              error={fieldError(field.name)}
              hint={field.hintKey ? t(field.hintKey) : undefined}
            />
          );
        } else if (field.kind === 'readonly-copy') {
          fieldContent = (
            <ReadOnlyCopyField
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
        } else {
          const error: string | undefined = fieldError(field.name);
          const required: boolean = isRequiredNow(field);
          fieldContent = (
            <FormControl fullWidth required={required} error={Boolean(error)}>
              <FormLabel htmlFor={`connection-field-${field.name}`}>
                {label}
                {field.optional && !required && (
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
                helperText={error ?? (field.hintKey ? renderHint(field.hintKey) : undefined)}
                onChange={(e) => setField(field.name, e.target.value)}
                onBlur={() => setTouched((prev) => ({...prev, [field.name]: true}))}
              />
            </FormControl>
          );
        }

        return (
          <Box key={field.name}>
            {field.section && (
              <Box>
                <Divider sx={{mt: 3, mb: 2}} />
                <Typography variant="subtitle2" component="h3">
                  {t(field.section)}
                </Typography>
              </Box>
            )}
            {field.revealedBy ? (
              <Collapse in={visible} timeout="auto" unmountOnExit>
                <Box sx={{mt: 3}}>{fieldContent}</Box>
              </Collapse>
            ) : (
              fieldContent
            )}
          </Box>
        );
      })}
    </Stack>
  );
}
