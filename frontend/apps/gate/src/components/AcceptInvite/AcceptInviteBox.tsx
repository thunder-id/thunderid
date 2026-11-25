// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useConfig} from '@thunderid/contexts';
import {useDesign, FlowComponentRenderer, AuthCardLayout} from '@thunderid/design';
import {useLogger} from '@thunderid/logger/react';
import {AcceptInvite, useThunderID, type EmbeddedFlowComponent} from '@thunderid/react';
import {Box, Alert, AlertTitle, Typography, CircularProgress} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useState} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate, useSearchParams} from 'react-router';
import RouteConfig from '../../configs/RouteConfig';

export interface FlowChangeResponse {
  flowStatus?: string;
  assertion?: string;
  errorAssertion?: string;
  data?: {additionalData?: Record<string, string>};
  error?: {
    code?: string;
    message?: {key?: string; defaultValue?: string};
    description?: {key?: string; defaultValue?: string};
  };
}

export default function AcceptInviteBox(): JSX.Element {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const {resolveFlowTemplateLiterals} = useThunderID();
  const {t} = useTranslation();
  const {getServerUrl} = useConfig();
  const logger = useLogger('AcceptInviteBox');

  const {isDesignEnabled} = useDesign();
  const [flowError, setFlowError] = useState<string | null>(null);

  const baseUrl = getServerUrl() ?? (import.meta.env.VITE_THUNDER_BASE_URL as string);

  const handleGoToSignIn = () => {
    const result = navigate(RouteConfig.signIn());
    if (result instanceof Promise) {
      result.catch(() => null);
    }
  };

  /**
   * Posts a flow outcome to the callback endpoint: a completed flow's assertion (success) or a signed
   * error assertion (terminal failure). Both go in the same field; the backend tells them apart from
   * the assertion's own claims. Requires authId (from URL params). callbackType is optional; the
   * backend defaults to authorization_code when absent.
   *
   * Response handling is generic:
   *   - redirect_uri present: redirect the browser (e.g. auth code flow, or an error redirect)
   *   - redirect_uri absent: no redirect; the flow's completion components display the outcome
   */
  const handleFlowCallback = async (authId: string, assertion: string, callbackType?: string): Promise<void> => {
    try {
      const body: Record<string, string> = {assertion, authId};
      if (callbackType) {
        body.type = callbackType;
      }

      const resp = await fetch(`${baseUrl}/oauth2/auth/callback`, {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        credentials: 'include',
        body: JSON.stringify(body),
      });

      if (!resp.ok) {
        logger.error('Flow callback returned non-OK status', {status: resp.status});
        setFlowError(t('invite:errors.callback.failed', 'Failed to complete callback. Please try again.'));
        return;
      }

      const result = (await resp.json()) as {redirect_uri?: string};

      if (result.redirect_uri) {
        window.location.href = result.redirect_uri;
      }
      // No redirect_uri: the flow's own completion components already display the outcome.
    } catch (err) {
      logger.error('Flow callback error:', err instanceof Error ? err : undefined);
      setFlowError(t('invite:errors.callback.failed', 'Failed to complete callback. Please try again.'));
    }
  };

  return (
    <AuthCardLayout
      logo={{
        src: {
          light: `${import.meta.env.BASE_URL}/assets/images/logo.svg`,
          dark: `${import.meta.env.BASE_URL}/assets/images/logo-inverted.svg`,
        },
        alt: {light: '', dark: ''},
      }}
      showLogo={!isDesignEnabled}
      logoDisplay={!isDesignEnabled ? {xs: 'flex', md: 'none'} : {display: 'none'}}
    >
      <AcceptInvite
        baseUrl={baseUrl}
        onGoToSignIn={handleGoToSignIn}
        onError={(error: Error) => {
          logger.error('Invite acceptance error:', error);
        }}
        onFlowChange={(response: FlowChangeResponse) => {
          const messageKey: string | undefined = response?.error?.message?.key;
          const translated: string | undefined = messageKey ? t(messageKey) : undefined;

          if (translated && translated !== messageKey) {
            setFlowError(translated);
          } else {
            const fallback: string | undefined =
              response?.error?.message?.defaultValue ?? response?.error?.description?.defaultValue;
            setFlowError(fallback ?? null);
          }

          // Relay the terminal flow outcome to the waiting OAuth request: the assertion on success, or
          // the signed error assertion on failure (auth code: error redirect; CIBA: request denied).
          // flowStatus selects which field to read, since both are relayed in the same request field.
          let assertion: string | undefined;

          if (response.flowStatus === 'COMPLETE') {
            assertion = response.assertion;
          } else if (response.flowStatus === 'ERROR') {
            assertion = response.errorAssertion;
          }

          if (assertion) {
            const authId = searchParams.get('auth_req_id') ?? searchParams.get('authId');

            if (authId) {
              void handleFlowCallback(authId, assertion, response.data?.additionalData?.callbackType);
            }
          }
        }}
      >
        {({
          additionalData,
          fieldErrors,
          error,
          touched,
          isLoading,
          components,
          values,
          handleInputChange,
          handleSubmit,
          isValidatingToken,
          isTokenInvalid,
        }) => {
          if (isValidatingToken) {
            return (
              <Box sx={{display: 'flex', flexDirection: 'column', alignItems: 'center', p: 3, gap: 2}}>
                <CircularProgress />
                <Typography>{t('invite:validating', 'Validating your invite link...')}</Typography>
              </Box>
            );
          }

          if (isTokenInvalid) {
            return (
              <Alert severity="error">
                <AlertTitle>{t('invite:errors.invalid.title', 'Unable to verify invite')}</AlertTitle>
                {t('invite:errors.invalid.description', 'This invite link is invalid or has expired.')}
              </Alert>
            );
          }

          if (isLoading && !components?.length) {
            return (
              <Box sx={{display: 'flex', justifyContent: 'center', p: 3}}>
                <CircularProgress />
              </Box>
            );
          }

          return (
            <>
              {(flowError ?? error) && (
                <Alert severity="error" sx={{mb: 2}}>
                  <AlertTitle>{t('invite:errors.failed.title', 'Error')}</AlertTitle>
                  {flowError ?? error?.message ?? t('invite:errors.failed.description', 'An error occurred.')}
                </Alert>
              )}
              {additionalData?.bindingMessage && (
                <Alert severity="info" icon={false} sx={{mb: 2}} data-testid="invite-binding-message">
                  <AlertTitle>{t('invite:bindingMessage.title', 'Request message')}</AlertTitle>
                  {additionalData.bindingMessage}
                </Alert>
              )}
              {components?.length > 0 && (
                <Box sx={{display: 'flex', flexDirection: 'column', gap: 2}}>
                  {(components as EmbeddedFlowComponent[]).map((component, index) => (
                    <FlowComponentRenderer
                      key={component.id ?? index}
                      component={component}
                      index={index}
                      values={values ?? {}}
                      touched={touched}
                      fieldErrors={fieldErrors}
                      isLoading={isLoading}
                      additionalData={additionalData}
                      resolve={resolveFlowTemplateLiterals}
                      onInputChange={handleInputChange}
                      onSubmit={(action, inputs) => {
                        void handleSubmit(action, inputs);
                      }}
                    />
                  ))}
                </Box>
              )}
            </>
          );
        }}
      </AcceptInvite>
    </AuthCardLayout>
  );
}
