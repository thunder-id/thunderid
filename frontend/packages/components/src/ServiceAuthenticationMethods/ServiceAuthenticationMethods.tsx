// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Button, FormControl, FormLabel, MenuItem, Select, Stack, TextField} from '@wso2/oxygen-ui';
import {RotateCcw} from '@wso2/oxygen-ui-icons-react';
import {type JSX, useEffect, useRef} from 'react';
import {useTranslation} from 'react-i18next';
import {AuthenticationMethods, type AuthenticationMethod} from './authentication-methods';
import {
  getAPIKeyHeaderProblems,
  type APIKeyHeaderProblem,
  type ServiceAuthenticationErrors,
} from './outboundAuthentication';
import KeyValuePairsField, {type KeyValueRowError} from '../KeyValuePairsField/KeyValuePairsField';
import MaskedSecretField from '../MaskedSecretField/MaskedSecretField';

export interface ServiceAuthenticationMethodsProps {
  method: AuthenticationMethod;
  bearerToken: string;
  basicUsername: string;
  basicPassword: string;
  httpHeaders: string;
  hasStoredHTTPHeaders: boolean;
  /** The stored API key headers, restored when API key is chosen again. */
  storedHTTPHeaders: string;
  hasStoredSecret: boolean;
  replacing: boolean;
  errors?: ServiceAuthenticationErrors;
  onChange: (name: string, value: string) => void;
  onReplacingChange: (replacing: boolean) => void;
  /** Shows the authentication without letting it be changed. */
  disabled?: boolean;
}

/**
 * Lets an administrator choose how ThunderID authenticates to an HTTP service: none, a bearer
 * token, Basic authentication, or API key headers. Stored credentials stay masked until replaced.
 */

export default function ServiceAuthenticationMethods({
  method,
  bearerToken,
  basicUsername,
  basicPassword,
  httpHeaders,
  hasStoredHTTPHeaders,
  storedHTTPHeaders,
  hasStoredSecret,
  replacing,
  errors = {},
  onChange,
  onReplacingChange,
  disabled = false,
}: ServiceAuthenticationMethodsProps): JSX.Element {
  const {t} = useTranslation('common');
  const usernameRef = useRef<HTMLInputElement>(null);
  const updateRef = useRef<HTMLButtonElement>(null);
  // What to focus once Update or Cancel of the Basic credentials has rendered.
  const pendingFocus = useRef<'username' | 'update' | null>(null);
  // The Basic credentials as they were before Update, restored by Cancel.
  const basicBeforeUpdate = useRef({username: basicUsername, password: basicPassword});

  useEffect(() => {
    if (pendingFocus.current === 'username') usernameRef.current?.focus();
    if (pendingFocus.current === 'update') updateRef.current?.focus();
    pendingFocus.current = null;
  }, [replacing]);

  const headerProblemMessage = (problem: APIKeyHeaderProblem): string => {
    switch (problem) {
      case 'nameRequired':
        return t('outboundAuthentication.apiKeyHeaders.errors.nameRequired', 'Enter a header name.');
      case 'invalidName':
        return t(
          'outboundAuthentication.apiKeyHeaders.errors.invalidName',
          "A header name can contain only letters, digits and these characters: ! # $ % & ' * + - . ^ _ ` | ~",
        );
      case 'reservedName':
        return t(
          'outboundAuthentication.apiKeyHeaders.errors.reservedName',
          'This header is reserved. Use another name.',
        );
      case 'duplicateName':
        return t(
          'outboundAuthentication.apiKeyHeaders.errors.duplicateName',
          'This header is already added. Header names are not case sensitive.',
        );
      case 'valueRequired':
        return t('outboundAuthentication.apiKeyHeaders.errors.valueRequired', 'Enter a value.');
      default:
        return t('outboundAuthentication.apiKeyHeaders.errors.valueReentry', "Enter this header's value again.");
    }
  };

  // A wrong header is shown as soon as it is entered; a missing part only once errors are shown.
  const headerRowErrors = (
    method === AuthenticationMethods.API_KEY
      ? getAPIKeyHeaderProblems(httpHeaders, hasStoredHTTPHeaders ? storedHTTPHeaders : '')
      : []
  ).map((problem): KeyValueRowError | undefined =>
    !problem || (!errors.httpHeaders && (problem === 'nameRequired' || problem === 'valueRequired'))
      ? undefined
      : {
          part: problem === 'valueRequired' || problem === 'valueReentry' ? 'value' : 'name',
          message: headerProblemMessage(problem),
        },
  );

  const startBasicUpdate = (): void => {
    basicBeforeUpdate.current = {username: basicUsername, password: basicPassword};
    pendingFocus.current = 'username';
    onReplacingChange(true);
  };

  const cancelBasicUpdate = (): void => {
    onChange('basicUsername', basicBeforeUpdate.current.username);
    onChange('basicPassword', basicBeforeUpdate.current.password);
    pendingFocus.current = 'update';
    onReplacingChange(false);
  };

  return (
    <Stack direction="column" spacing={2} sx={{maxWidth: 640}}>
      <FormControl fullWidth required>
        <FormLabel id="connection-field-authenticationScheme-label" htmlFor="connection-field-authenticationScheme">
          {t('outboundAuthentication.method.label', 'Authentication method')}
        </FormLabel>
        <Select
          id="connection-field-authenticationScheme"
          labelId="connection-field-authenticationScheme-label"
          value={method}
          disabled={disabled}
          onChange={(event) => {
            onChange('authenticationScheme', event.target.value);
            // Choosing API key again shows the stored headers, as the other methods show theirs.
            if (event.target.value === AuthenticationMethods.API_KEY && storedHTTPHeaders) {
              onChange('httpHeaders', storedHTTPHeaders);
            }
          }}
          data-testid="connection-field-select-authenticationScheme"
        >
          <MenuItem value={AuthenticationMethods.NONE}>{t('outboundAuthentication.method.none', 'None')}</MenuItem>
          <MenuItem value={AuthenticationMethods.BEARER}>
            {t('outboundAuthentication.method.bearer', 'Bearer token')}
          </MenuItem>
          <MenuItem value={AuthenticationMethods.BASIC}>
            {t('outboundAuthentication.method.basic', 'Basic authentication')}
          </MenuItem>
          <MenuItem value={AuthenticationMethods.API_KEY}>
            {t('outboundAuthentication.method.apiKey', 'API key')}
          </MenuItem>
        </Select>
      </FormControl>

      {method === AuthenticationMethods.BEARER && (
        <MaskedSecretField
          id="connection-field-bearerToken"
          label={t('outboundAuthentication.bearerToken.label', 'Bearer token')}
          value={bearerToken}
          onChange={(value) => onChange('bearerToken', value)}
          hasStoredSecret={hasStoredSecret}
          replacing={replacing}
          onReplacingChange={onReplacingChange}
          disabled={disabled}
          error={errors.bearerToken}
          required
          hint={t(
            'outboundAuthentication.bearerToken.hint',
            'Token sent in the Authorization header with each request.',
          )}
        />
      )}

      {method === AuthenticationMethods.BASIC && (
        <Box
          data-testid="basic-credentials-fields"
          sx={{
            display: 'grid',
            gridTemplateColumns: {
              xs: '1fr',
              sm: hasStoredSecret && !disabled ? 'repeat(2, minmax(0, 1fr)) auto' : 'repeat(2, minmax(0, 1fr))',
            },
            gap: 2,
          }}
        >
          <TextField
            labelPlacement="top"
            label={t('outboundAuthentication.basicUsername.label', 'Username')}
            required
            id="connection-field-basicUsername"
            inputRef={usernameRef}
            fullWidth
            value={basicUsername}
            disabled={disabled || (hasStoredSecret && !replacing)}
            onChange={(event) => onChange('basicUsername', event.target.value)}
            error={Boolean(errors.basicUsername)}
            helperText={
              errors.basicUsername ??
              (hasStoredSecret && !replacing
                ? undefined
                : t('outboundAuthentication.basicUsername.hint', 'Username sent with each request.'))
            }
          />
          <MaskedSecretField
            id="connection-field-basicPassword"
            label={t('outboundAuthentication.basicPassword.label', 'Password')}
            value={basicPassword}
            onChange={(value) => onChange('basicPassword', value)}
            hasStoredSecret={hasStoredSecret}
            replacing={replacing}
            onReplacingChange={onReplacingChange}
            showStoredControls={false}
            disabled={disabled}
            error={errors.basicPassword}
            hint={t('outboundAuthentication.basicPassword.hint', 'Password sent with each request.')}
            required
          />
          {hasStoredSecret && !replacing && !disabled && (
            <Button
              variant="outlined"
              size="small"
              startIcon={<RotateCcw size={16} />}
              onClick={startBasicUpdate}
              ref={updateRef}
              sx={{alignSelf: 'end', justifySelf: {xs: 'end', sm: 'start'}, whiteSpace: 'nowrap'}}
            >
              {t('outboundAuthentication.update', 'Update')}
            </Button>
          )}
          {hasStoredSecret && replacing && !disabled && (
            <Button
              variant="text"
              size="small"
              onClick={cancelBasicUpdate}
              data-testid="basic-credentials-cancel"
              sx={{alignSelf: 'start', mt: 3.5, justifySelf: {xs: 'end', sm: 'start'}, whiteSpace: 'nowrap'}}
            >
              {t('outboundAuthentication.cancel', 'Cancel')}
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
            rowErrors={headerRowErrors}
            maskValues
            hint={t(
              'outboundAuthentication.apiKeyHeaders.hint',
              'Add the API key header and any other headers the service requires. Commas are not supported in a name or value.',
            )}
            namePlaceholder="X-API-Key"
            addLabel={t('outboundAuthentication.apiKeyHeaders.add', 'Add header')}
            hasStoredValues={hasStoredHTTPHeaders}
            disabled={disabled}
          />
        </Stack>
      )}
    </Stack>
  );
}
