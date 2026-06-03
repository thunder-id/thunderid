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

import {useConfig} from '@thunderid/contexts';
import {useDesign, FlowComponentRenderer, AuthCardLayout} from '@thunderid/design';
import {useLogger} from '@thunderid/logger/react';
import {AcceptInvite, useThunderID, type EmbeddedFlowComponent} from '@thunderid/react';
import {Box, Alert, Typography, AlertTitle, CircularProgress} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useState} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import ROUTES from '../../constants/routes';

export default function AcceptInviteBox(): JSX.Element {
  const navigate = useNavigate();
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
        onFlowChange={(response: {failureReason?: string}) => {
          setFlowError(response?.failureReason ?? null);
        }}
      >
        {({
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
          // Validating token
          if (isValidatingToken) {
            return (
              <Box sx={{display: 'flex', flexDirection: 'column', alignItems: 'center', p: 3, gap: 2}}>
                <CircularProgress />
                <Typography>{t('invite:validating', 'Validating your invite link...')}</Typography>
              </Box>
            );
          }

          // Invalid token
          if (isTokenInvalid) {
            return (
              <Alert severity="error">
                <AlertTitle>{t('invite:errors.invalid.title', 'Unable to verify invite')}</AlertTitle>
                {t('invite:errors.invalid.description', 'This invite link is invalid or has expired.')}
              </Alert>
            );
          }

          // Loading
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
