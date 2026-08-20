// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  Box,
  Collapse,
  Divider,
  FormControl,
  FormControlLabel,
  FormHelperText,
  FormLabel,
  MenuItem,
  Select,
  Stack,
  Switch,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import {type JSX, type ReactNode, useMemo, useState} from 'react';
import {Trans, useTranslation} from 'react-i18next';
import KeyValuePairsField from './KeyValuePairsField';
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
  /** Show the stored configuration without offering to change it. */
  isReadOnly?: boolean;
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
  isReadOnly = false,
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
                    disabled={isReadOnly}
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
              disabled={isReadOnly}
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
        } else if (field.kind === 'key-value') {
          fieldContent = (
            <KeyValuePairsField
              id={`connection-field-${field.name}`}
              label={label}
              value={values[field.name] ?? ''}
              onChange={(next) => setField(field.name, next)}
              hint={field.hintKey ? renderHint(field.hintKey) : undefined}
              namePlaceholder={field.placeholder}
              addLabel={field.addLabelKey ? t(field.addLabelKey) : t('form.keyValue.add')}
            />
          );
        } else if (field.kind === 'select') {
          const error: string | undefined = fieldError(field.name);
          fieldContent = (
            <FormControl fullWidth required={isRequiredNow(field)} error={Boolean(error)}>
              <FormLabel htmlFor={`connection-field-${field.name}`}>{label}</FormLabel>
              <Select
                id={`connection-field-${field.name}`}
                value={values[field.name] ?? ''}
                onChange={(e) => setField(field.name, e.target.value)}
                data-testid={`connection-field-select-${field.name}`}
              >
                {(field.options ?? []).map((option) => (
                  <MenuItem key={option.value} value={option.value}>
                    {option.label}
                  </MenuItem>
                ))}
              </Select>
              {error ? (
                <FormHelperText>{error}</FormHelperText>
              ) : (
                field.hintKey && <FormHelperText>{t(field.hintKey)}</FormHelperText>
              )}
            </FormControl>
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
              <FormLabel htmlFor={`connection-field-${field.name}`}>{label}</FormLabel>
              <TextField
                disabled={isReadOnly}
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
