// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {I18N_KEY_PATTERN, isI18nTemplatePattern} from '@thunderid/utils';
import {
  Box,
  Divider,
  FormControl,
  FormHelperText,
  FormLabel,
  MenuItem,
  Select,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import {type JSX, type ReactNode, useCallback, useMemo} from 'react';
import {useTranslation} from 'react-i18next';
import MaskedSecretField from './MaskedSecretField';
import type {OutboundAuthField, OutboundAuthMethod} from '../models/connection';
import {AUTH_TYPE_FIELD, authPropertyField, AUTH_TYPE_NONE} from '../utils/connectionFormMapping';

interface AuthenticationSectionProps {
  /** The methods the vendor supports, as the server described them. */
  methods: OutboundAuthMethod[];
  /** Flat form values, including the namespaced authentication entries. */
  values: Record<string, string>;
  mode: 'create' | 'edit';
  /** Whether the user has chosen to replace the stored secret. */
  secretReplacing: boolean;
  /** True when editing a connection whose secret is already stored. */
  hasStoredSecret: boolean;
  /** Render the divider and section title. Off when a surrounding card already titles it. */
  showHeading?: boolean;
  onFieldChange: (name: string, value: string) => void;
  onSecretReplacingChange: (replacing: boolean) => void;
}

/**
 * Renders the authentication method selector and the fields of the selected method, entirely
 * from the metadata the server returned. Nothing here enumerates methods or field names, so a
 * method added server-side appears without a console change.
 *
 * Renders nothing when the vendor advertises no methods, which is how a provider whose
 * credentials are a fixed vendor contract (Twilio, Vonage) opts out.
 */
export default function AuthenticationSection({
  methods,
  values,
  mode,
  secretReplacing,
  hasStoredSecret,
  showHeading = true,
  onFieldChange,
  onSecretReplacingChange,
}: AuthenticationSectionProps): JSX.Element | null {
  const {t} = useTranslation('connections');

  /**
   * Resolves a server-supplied label, which is either plain text or an i18n template pattern.
   * A pattern whose key has no locale entry falls back to the raw name, so a method added
   * server-side is still identifiable rather than rendering blank.
   */
  const labelFor = useCallback(
    (displayName: string, fallback: string): string => {
      if (!displayName.trim()) {
        return fallback;
      }
      if (!isI18nTemplatePattern(displayName)) {
        return displayName;
      }
      const key: string | undefined = I18N_KEY_PATTERN.exec(displayName.trim())?.[1];
      if (!key) {
        return fallback;
      }
      const resolved: string = t(key);
      // i18next echoes the key back when the translation is missing, stripping the namespace.
      const keyWithoutNamespace: string = key.includes(':') ? (key.split(':').pop() ?? key) : key;
      if (!resolved || resolved === key || resolved === keyWithoutNamespace) {
        return fallback;
      }
      return resolved;
    },
    [t],
  );

  const selectedType: string = values[AUTH_TYPE_FIELD] || AUTH_TYPE_NONE;
  const selected: OutboundAuthMethod | undefined = useMemo(
    () => methods.find((method) => method.type === selectedType),
    [methods, selectedType],
  );

  if (methods.length === 0) {
    return null;
  }

  const renderField = (field: OutboundAuthField): ReactNode => {
    const name: string = authPropertyField(field.name);
    const label: string = labelFor(field.displayName, field.name);
    const value: string = values[name] ?? '';

    if (field.credential) {
      return (
        <MaskedSecretField
          id={`connection-field-${name}`}
          label={label}
          value={value}
          onChange={(next) => onFieldChange(name, next)}
          hasStoredSecret={hasStoredSecret}
          replacing={secretReplacing}
          onReplacingChange={onSecretReplacingChange}
          required={mode === 'create' && Boolean(field.required)}
        />
      );
    }

    if (field.enum && field.enum.length > 0) {
      return (
        <FormControl fullWidth required={Boolean(field.required)}>
          <FormLabel htmlFor={`connection-field-${name}`}>{label}</FormLabel>
          <Select
            id={`connection-field-${name}`}
            value={value || field.enum[0]}
            onChange={(e) => onFieldChange(name, String(e.target.value))}
          >
            {field.enum.map((choice) => (
              <MenuItem key={choice} value={choice}>
                {choice}
              </MenuItem>
            ))}
          </Select>
        </FormControl>
      );
    }

    if (field.type !== 'string') {
      // An unrecognized field type renders nothing rather than guessing at a widget, matching
      // how the schema-driven user attribute form degrades.
      return null;
    }

    return (
      <FormControl fullWidth required={Boolean(field.required)}>
        <FormLabel htmlFor={`connection-field-${name}`}>{label}</FormLabel>
        <TextField
          id={`connection-field-${name}`}
          value={value}
          onChange={(e) => onFieldChange(name, e.target.value)}
          fullWidth
        />
      </FormControl>
    );
  };

  return (
    <Box data-testid="connection-authentication-section">
      {showHeading && (
        <>
          <Divider sx={{mb: 2, mt: 3}} />
          <Typography variant="subtitle2" component="h3">
            {t('form.sections.authentication')}
          </Typography>
        </>
      )}

      <Stack direction="column" spacing={3} sx={{mt: showHeading ? 3 : 0}}>
        <FormControl fullWidth>
          <FormLabel htmlFor="connection-field-authentication-type">
            {t('form.fields.authenticationType.label')}
          </FormLabel>
          <Select
            id="connection-field-authentication-type"
            value={selectedType}
            onChange={(e) => onFieldChange(AUTH_TYPE_FIELD, String(e.target.value))}
          >
            {methods.map((method) => (
              <MenuItem key={method.type} value={method.type}>
                {labelFor(method.displayName, method.type)}
              </MenuItem>
            ))}
          </Select>
          <FormHelperText>{t('form.fields.authenticationType.hint')}</FormHelperText>
        </FormControl>

        {selected?.properties?.map((field) => (
          <Box key={field.name}>{renderField(field)}</Box>
        ))}
      </Stack>
    </Box>
  );
}
