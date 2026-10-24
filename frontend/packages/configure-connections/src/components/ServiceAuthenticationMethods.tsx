// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Button, FormControl, FormLabel, MenuItem, Select, Stack, TextField} from '@wso2/oxygen-ui';
import {RotateCcw} from '@wso2/oxygen-ui-icons-react';
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
  hasStoredSecret: boolean;
  replacing: boolean;
  errors?: {
    bearerToken?: string;
    basicUsername?: string;
    basicPassword?: string;
    httpHeaders?: string;
  };
  onChange: (name: string, value: string) => void;
  onReplacingChange: (replacing: boolean) => void;
}

export default function ServiceAuthenticationMethods({
  method,
  bearerToken,
  basicUsername,
  basicPassword,
  httpHeaders,
  hasStoredHTTPHeaders,
  hasStoredSecret,
  replacing,
  errors = {},
  onChange,
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
        <Box
          data-testid="basic-credentials-fields"
          sx={{
            display: 'grid',
            gridTemplateColumns: {
              xs: '1fr',
              sm: hasStoredSecret && !replacing ? 'repeat(2, minmax(0, 1fr)) auto' : 'repeat(2, minmax(0, 1fr))',
            },
            gap: 2,
          }}
        >
          <FormControl fullWidth required error={Boolean(errors.basicUsername)}>
            <FormLabel htmlFor="connection-field-basicUsername">
              {t('form.fields.basicUsername.label', 'Username')}
            </FormLabel>
            <TextField
              id="connection-field-basicUsername"
              fullWidth
              value={basicUsername}
              disabled={hasStoredSecret && !replacing}
              onChange={(event) => onChange('basicUsername', event.target.value)}
              error={Boolean(errors.basicUsername)}
              helperText={
                errors.basicUsername ??
                (hasStoredSecret && !replacing
                  ? undefined
                  : t('form.fields.basicUsername.hint', 'Username sent with each PDP request.'))
              }
            />
          </FormControl>
          <MaskedSecretField
            id="connection-field-basicPassword"
            label={t('form.fields.basicPassword.label', 'Password')}
            value={basicPassword}
            onChange={(value) => onChange('basicPassword', value)}
            hasStoredSecret={hasStoredSecret}
            replacing={replacing}
            onReplacingChange={onReplacingChange}
            showStoredControls={false}
            error={errors.basicPassword}
            hint={t('form.fields.basicPassword.hint', 'Password sent with each PDP request.')}
            required
          />
          {hasStoredSecret && !replacing && (
            <Button
              variant="outlined"
              size="small"
              startIcon={<RotateCcw size={16} />}
              onClick={() => onReplacingChange(true)}
              sx={{alignSelf: 'end', justifySelf: {xs: 'end', sm: 'start'}, whiteSpace: 'nowrap'}}
            >
              {t('form.secret.update', 'Update')}
            </Button>
          )}
        </Box>
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
            hasStoredValues={hasStoredHTTPHeaders}
          />
        </Stack>
      )}
    </Stack>
  );
}
