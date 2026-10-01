// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {FormControl, FormHelperText, FormLabel, MenuItem, Select, Stack} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import KeyValuePairsField from './KeyValuePairsField';
import MaskedSecretField from './MaskedSecretField';
import {AuthenticationMethods, type AuthenticationMethod} from '../models/authentication-methods';

interface ServiceAuthenticationMethodsProps {
  method: AuthenticationMethod;
  bearerToken: string;
  basicUsername: string;
  basicPassword: string;
  httpHeaders: string;
  hasStoredHTTPHeaders: boolean;
  headersReplacing: boolean;
  hasStoredSecret: boolean;
  replacing: boolean;
  errors?: {
    bearerToken?: string;
    basicUsername?: string;
    basicPassword?: string;
    httpHeaders?: string;
  };
  onChange: (name: string, value: string) => void;
  onHeadersReplacingChange: (replacing: boolean) => void;
  onReplacingChange: (replacing: boolean) => void;
}

export default function ServiceAuthenticationMethods({
  method,
  bearerToken,
  basicUsername,
  basicPassword,
  httpHeaders,
  hasStoredHTTPHeaders,
  headersReplacing,
  hasStoredSecret,
  replacing,
  errors = {},
  onChange,
  onHeadersReplacingChange,
  onReplacingChange,
}: ServiceAuthenticationMethodsProps): JSX.Element {
  const {t} = useTranslation('connections');

  return (
    <Stack direction="column" spacing={2} sx={{maxWidth: 640}}>
      <FormControl fullWidth required>
        <FormLabel htmlFor="connection-field-authenticationScheme">
          {t('form.fields.authScheme.label', 'Authentication method')}
        </FormLabel>
        <Select
          id="connection-field-authenticationScheme"
          value={method}
          onChange={(event) => onChange('authenticationScheme', event.target.value)}
          data-testid="connection-field-select-authenticationScheme"
        >
          <MenuItem value={AuthenticationMethods.NONE}>{t('form.fields.authScheme.none', 'None')}</MenuItem>
          <MenuItem value={AuthenticationMethods.BEARER}>{t('form.fields.authScheme.bearer', 'Bearer token')}</MenuItem>
          <MenuItem value={AuthenticationMethods.BASIC}>
            {t('form.fields.authScheme.basic', 'Basic authentication')}
          </MenuItem>
          <MenuItem value={AuthenticationMethods.API_KEY}>{t('form.fields.authScheme.apiKey', 'API key')}</MenuItem>
        </Select>
        <FormHelperText>
          {t(
            'form.fields.authScheme.hint',
            'Choose how ThunderID authenticates requests sent to this policy decision point.',
          )}
        </FormHelperText>
      </FormControl>

      {method === AuthenticationMethods.BEARER && (
        <MaskedSecretField
          id="connection-field-bearerToken"
          label={t('form.fields.bearerToken.label', 'Bearer token')}
          value={bearerToken}
          onChange={(value) => onChange('bearerToken', value)}
          hasStoredSecret={hasStoredSecret}
          replacing={replacing}
          onReplacingChange={onReplacingChange}
          error={errors.bearerToken}
          hint={t('form.fields.bearerToken.hint', 'Token sent in the Authorization header for each PDP request.')}
        />
      )}

      {method === AuthenticationMethods.BASIC && (
        <Stack direction="column" spacing={1}>
          <MaskedSecretField
            id="connection-field-basicUsername"
            label={t('form.fields.basicUsername.label', 'Username')}
            value={basicUsername}
            onChange={(value) => onChange('basicUsername', value)}
            hasStoredSecret={hasStoredSecret}
            replacing={replacing}
            onReplacingChange={onReplacingChange}
            error={errors.basicUsername}
            hint={t('form.fields.basicUsername.hint', 'Username sent with each PDP request.')}
            required
          />
          <MaskedSecretField
            id="connection-field-basicPassword"
            label={t('form.fields.basicPassword.label', 'Password')}
            value={basicPassword}
            onChange={(value) => onChange('basicPassword', value)}
            hasStoredSecret={hasStoredSecret}
            replacing={replacing}
            onReplacingChange={onReplacingChange}
            error={errors.basicPassword}
            hint={t('form.fields.basicPassword.hint', 'Password sent with each PDP request.')}
            required
          />
        </Stack>
      )}

      {method === AuthenticationMethods.API_KEY && (
        <Stack direction="column" spacing={2}>
          <KeyValuePairsField
            id="connection-field-httpHeaders"
            value={httpHeaders}
            onChange={(value) => onChange('httpHeaders', value)}
            error={errors.httpHeaders}
            maskValues
            hint={t(
              'form.fields.authzenHeaders.hint',
              'Add the API key header and any other headers required by the PDP. Commas are not supported in a name or value.',
            )}
            namePlaceholder="X-API-Key"
            addLabel={t('form.fields.authzenHeaders.add', 'Add header')}
            hasStoredValue={hasStoredHTTPHeaders}
            replacing={headersReplacing}
            onReplacingChange={onHeadersReplacingChange}
            replaceLabel={t('form.secret.update', 'Update')}
            storedHint={t('form.headers.keepHelp', 'Leave unchanged to keep the configured headers.')}
          />
        </Stack>
      )}
    </Stack>
  );
}
