/**
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
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

import {useConfig} from '@thunderid/contexts';
import {useDesign, FlowComponentRenderer, AuthCardLayout} from '@thunderid/design';
import {useLogger} from '@thunderid/logger/react';
import {AcceptInvite, useThunderID, type EmbeddedFlowComponent} from '@thunderid/react';
import {Box, Alert, AlertTitle, Typography, CircularProgress} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useState} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate, useSearchParams} from 'react-router';
import ROUTES from '../../constants/routes';

export interface FlowChangeResponse {
  flowStatus?: string;
  assertion?: string;
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
    const result = navigate(ROUTES.AUTH.SIGN_IN);
    if (result instanceof Promise) {
      result.catch(() => null);
    }
  };

  /**
   * Posts the completed flow assertion to the callback endpoint.
   * Requires authId (from URL params) and assertion (from flow completion).
   * callbackType is optional — the backend defaults to authorization_code when absent.
   *
   * Response handling is generic:
   *   - redirect_uri present → redirect the browser (e.g. auth code flow)
   *   - redirect_uri absent  → no redirect; the flow's completion components display the outcome
   */
  const handleFlowCallback = async (authId: string, assertion: string, callbackType?: string): Promise<void> => {
    try {
      const body: Record<string, string> = {authId, assertion};
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
          if (messageKey) {
            const translated: string = t(messageKey);
            if (translated !== messageKey) {
              setFlowError(translated);

              return;
            }
          }
          const fallback: string | undefined =
            response?.error?.message?.defaultValue ?? response?.error?.description?.defaultValue;
          setFlowError(fallback ?? null);

          if (response.flowStatus === 'COMPLETE' && response.assertion) {
            const authId = searchParams.get('auth_req_id') ?? searchParams.get('authId');
            const {assertion} = response;
            const callbackType = response.data?.additionalData?.callbackType;

            if (authId && assertion) {
              void handleFlowCallback(authId, assertion, callbackType);
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
