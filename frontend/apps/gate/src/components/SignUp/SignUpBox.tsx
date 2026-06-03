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

/* eslint-disable @typescript-eslint/no-unsafe-assignment */
/* eslint-disable @typescript-eslint/no-explicit-any */
/* eslint-disable @typescript-eslint/no-unsafe-member-access */
/* eslint-disable @typescript-eslint/no-unsafe-call */

import {FlowComponentRenderer, AuthCardLayout, useDesign} from '@thunderid/design';
import {EmbeddedFlowEventType, SignUp, useThunderID, type EmbeddedFlowComponent} from '@thunderid/react';
import {Box, Button, Alert, Typography, AlertTitle, CircularProgress} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useState} from 'react';
import {Trans, useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import ROUTES from '../../constants/routes';

export default function SignUpBox(): JSX.Element {
  const navigate = useNavigate();
  const {resolveFlowTemplateLiterals: resolve, meta} = useThunderID();
  const {t} = useTranslation();
  const {isDesignEnabled} = useDesign();
  const [flowError, setFlowError] = useState<string | null>(null);

  // For React Router navigate() — basename is handled by the router.
  const signInPath = ROUTES.AUTH.SIGN_IN;
  // For window.location.href and new URL() (via afterSignUpUrl) — React Router basename is
  // bypassed, so an absolute URL with origin + base path must be constructed explicitly.
  // Vite appends a trailing slash to BASE_URL.
  const signInUrl = `${window.location.origin}${import.meta.env.BASE_URL.replace(/\/$/, '')}${signInPath}`;
  // Prefer the application's home URL from flow metadata so the user is returned to the
  // app after sign-up instead of the gate sign-in page. Fall back to the sign-in page if
  // the application URL is not available in the flow metadata.
  const appUrl = meta?.application?.url;
  const afterSignUpUrl = appUrl != null && appUrl !== '' ? appUrl : signInUrl;

  const renderFlowContent = (
    components: EmbeddedFlowComponent[],
    error: any,
    isLoading: boolean,
    values: any,
    touched: any,
    fieldErrors: any,
    handleInputChange: any,
    handleSubmit: any,
  ): JSX.Element | null => {
    if (components.length > 0) {
      return (
        <Box sx={{display: 'flex', flexDirection: 'column', gap: 2}}>
          {isLoading && (
            <Typography sx={{textAlign: 'center'}}>
              {t('signup:create_account.loading', 'Creating account...')}
            </Typography>
          )}
          {components.map((component, index) => (
            <FlowComponentRenderer
              key={component.id ?? index}
              component={component}
              index={index}
              values={values ?? {}}
              touched={touched}
              fieldErrors={fieldErrors}
              isLoading={isLoading}
              resolve={resolve}
              onInputChange={handleInputChange}
              onSubmit={(action, inputs) => {
                setFlowError(null);
                const isTrigger = action.eventType === EmbeddedFlowEventType.Trigger || action.eventType === 'TRIGGER';
                void handleSubmit(action, inputs, isTrigger);
              }}
            />
          ))}
        </Box>
      );
    }
    if (!error) {
      return (
        <Alert severity="error" sx={{mb: 2}}>
          <AlertTitle>{t("Oops, that didn't work")}</AlertTitle>
          {t("We're sorry, we ran into a problem. Please try again!")}
        </Alert>
      );
    }
    return null;
  };

  return (
    <AuthCardLayout
      variant="SignUpBox"
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
      <SignUp
        afterSignUpUrl={afterSignUpUrl}
        onFlowChange={(response: any) => {
          if (response?.failureReason) {
            setFlowError(response.failureReason as string);
          } else {
            setFlowError(null);
          }
        }}
      >
        {({values, fieldErrors, error, touched, handleInputChange, handleSubmit, isLoading, components}: any) => (
          <>
            {!components ? (
              <Box sx={{display: 'flex', justifyContent: 'center', p: 3}}>
                <CircularProgress />
              </Box>
            ) : (
              <>
                {error && (
                  <Alert severity="error" sx={{mb: 2}}>
                    <AlertTitle>{t('signup:errors.signup.failed.message')}</AlertTitle>
                    {error.message ?? t('signup:errors.signup.failed.description')}
                  </Alert>
                )}
                {flowError && (
                  <Alert severity="error" sx={{mb: 2}}>
                    {flowError}
                  </Alert>
                )}

                {renderFlowContent(
                  components as EmbeddedFlowComponent[],
                  error,
                  isLoading as boolean,
                  values,
                  touched,
                  fieldErrors,
                  handleInputChange,
                  handleSubmit,
                )}
              </>
            )}

            <Typography sx={{textAlign: 'center', mt: 3}}>
              <Trans i18nKey="signup:redirect.to.signin">
                Already have an account?
                <Button
                  variant="text"
                  onClick={() => {
                    void navigate(signInPath);
                  }}
                  sx={{
                    p: 0,
                    minWidth: 'auto',
                    textTransform: 'none',
                    color: 'primary.main',
                    textDecoration: 'underline',
                    '&:hover': {
                      textDecoration: 'underline',
                      backgroundColor: 'transparent',
                    },
                  }}
                >
                  Sign in
                </Button>
              </Trans>
            </Typography>
          </>
        )}
      </SignUp>
    </AuthCardLayout>
  );
}
