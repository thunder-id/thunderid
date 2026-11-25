// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {ServiceAuthenticationMethods} from '@thunderid/components';
import {useConfig} from '@thunderid/contexts';
import {Alert, Box, FormControlLabel, Stack, Switch, TextField, Typography} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {AuthorizationDetailTypeForm} from '../../hooks/useAuthorizationDetailTypeForm';
import {modifyingParties, processorEndpointProblem} from '../../utils/authorization-detail-schema';
import {flattenFields} from '../../utils/schema-composer';

export interface ProcessorSectionProps {
  form: AuthorizationDetailTypeForm;
  readOnly?: boolean;
}

/**
 * Edits the processor of an authorization details type: the endpoint of the system that approves or
 * refuses each request, and how ThunderID authenticates to it.
 */
export default function ProcessorSection({form, readOnly = false}: ProcessorSectionProps): JSX.Element {
  const {t} = useTranslation();
  const {config} = useConfig();
  const endpointServerError = form.saveErrorTarget === 'endpoint' ? form.saveError : null;

  const endpointProblem = form.endpointError ? processorEndpointProblem(form.endpoint.trim()) : null;
  // The fields the processor sets or changes, named in the alert shown while there is no processor.
  const processorFieldPaths = form.processorMissing
    ? flattenFields(form.activeFields ?? [])
        .filter(({field}) => field.policy.suppliedBy === 'processor' || modifyingParties(field).includes('processor'))
        .map(({path}) => path)
        .filter(Boolean)
    : [];

  let endpointHelper: string;
  if (endpointServerError) endpointHelper = endpointServerError;
  else if (endpointProblem === 'scheme')
    endpointHelper = t(
      'authorizationDetailTypes:processor.endpointErrors.scheme',
      'Use HTTPS. HTTP is accepted only for a loopback address, such as localhost or 127.0.0.1.',
    );
  else if (endpointProblem === 'credentials')
    endpointHelper = t(
      'authorizationDetailTypes:processor.endpointErrors.credentials',
      'Remove the user name and password from the URL.',
    );
  else if (endpointProblem === 'port')
    endpointHelper = t('authorizationDetailTypes:processor.endpointErrors.port', 'Enter a port from 1 to 65535.');
  else if (endpointProblem)
    endpointHelper = t(
      'authorizationDetailTypes:processor.endpointErrors.invalid',
      'Enter a full URL, such as https://api.example.com/authorize.',
    );
  else if (form.endpointMissing)
    endpointHelper = t(
      'authorizationDetailTypes:processor.endpointRequired',
      'Enter the URL of your external processor.',
    );
  else
    endpointHelper = t(
      'authorizationDetailTypes:processor.endpointHint',
      'An HTTPS URL. HTTP is accepted only for a loopback address, for development.',
    );

  return (
    <Stack spacing={2}>
      <Box>
        <FormControlLabel
          control={
            <Switch
              checked={form.processorEnabled}
              disabled={readOnly}
              onChange={(e) => form.setProcessorEnabled(e.target.checked)}
            />
          }
          label={t('authorizationDetailTypes:processor.enable', 'Use an external processor')}
        />
        <Typography variant="body2" color="text.secondary" sx={{mt: 0.5}}>
          {t(
            'authorizationDetailTypes:processor.explanation',
            '{{productName}} sends each request of this type to the external processor before the user is asked. The processor approves or refuses it, and fills in or adjusts the fields you allow it to. Without a processor, requests are checked only against the fields and their rules.',
            {productName: config.brand.product_name},
          )}
        </Typography>
      </Box>
      {form.processorMissing && (
        <Alert severity="error">
          {processorFieldPaths.length > 0
            ? t(
                'authorizationDetailTypes:processor.missingFields',
                'Some fields are set or changed by the external processor: {{fields}}. Add a processor, or change those fields.',
                {fields: processorFieldPaths.join(', ')},
              )
            : t(
                'authorizationDetailTypes:processor.missing',
                'Some fields are set or changed by the external processor. Add a processor, or change those fields.',
              )}
        </Alert>
      )}
      {form.processorEnabled && (
        <Box sx={{display: 'grid', gridTemplateColumns: {xs: '1fr', md: '2fr 1fr'}, gap: 2}}>
          <TextField
            labelPlacement="top"
            label={t('authorizationDetailTypes:processor.endpoint', 'Endpoint URL')}
            required={!readOnly}
            fullWidth
            data-save-error-target="endpoint"
            id="authz-type-endpoint"
            size="small"
            value={form.endpoint}
            disabled={readOnly}
            error={form.endpointError || form.endpointMissing || Boolean(endpointServerError)}
            placeholder={t(
              'authorizationDetailTypes:processor.endpointPlaceholder',
              'e.g., https://api.example.com/authorize',
            )}
            onChange={(e) => form.setEndpoint(e.target.value)}
            helperText={endpointHelper}
          />
        </Box>
      )}
      {form.processorEnabled && (
        <Stack spacing={2} data-save-error-target="authentication">
          {!readOnly &&
            form.authenticationState.requiresFreshCredentials &&
            !form.authenticationState.hasRequiredAuthenticationCredential && (
              <Alert severity="warning">
                {t(
                  'authorizationDetailTypes:processor.endpointChanged',
                  'The endpoint changed. Enter the credentials again before saving.',
                )}
              </Alert>
            )}
          <ServiceAuthenticationMethods
            method={form.authenticationState.selectedAuthenticationScheme}
            bearerToken={form.authentication.bearerToken}
            basicUsername={form.authentication.basicUsername}
            basicPassword={form.authentication.basicPassword}
            httpHeaders={form.authentication.httpHeaders}
            hasStoredHTTPHeaders={form.authenticationState.hasStoredHTTPHeaders}
            storedHTTPHeaders={form.authenticationState.storedHTTPHeaders}
            hasStoredSecret={form.authenticationState.hasStoredCredentialForSelectedScheme}
            replacing={form.secretReplacing}
            errors={form.authenticationErrors}
            disabled={readOnly}
            onChange={form.setAuthenticationValue}
            onReplacingChange={form.setSecretReplacing}
          />
        </Stack>
      )}
    </Stack>
  );
}
