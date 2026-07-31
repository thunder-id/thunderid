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

/* eslint-disable @typescript-eslint/no-explicit-any */

import {FlowComponentRenderer, AuthCardLayout, useDesign} from '@thunderid/design';
import {
  EmbeddedFlowComponentType,
  EmbeddedFlowEventType,
  SignUp,
  useThunderID,
  type EmbeddedFlowComponent,
} from '@thunderid/react';
import {EMAIL_REGEX} from '@thunderid/utils';
import {Box, Button, Alert, Typography, AlertTitle, CircularProgress} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useEffect, useRef, useState} from 'react';
import {Trans, useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import RouteConfig from '../../configs/RouteConfig';

export default function SignUpBox(): JSX.Element {
  const navigate = useNavigate();
  const {resolveFlowTemplateLiterals: resolve, meta} = useThunderID();
  const {t} = useTranslation();
  const {isDesignEnabled} = useDesign();
  // For React Router navigate() — basename is handled by the router.
  const signInPath = RouteConfig.signIn();
  // For window.location.href and new URL() (via afterSignUpUrl) — React Router basename is
  // bypassed, so an absolute URL with origin + base path must be constructed explicitly.
  // Vite appends a trailing slash to BASE_URL.
  const signInUrl = `${window.location.origin}${import.meta.env.BASE_URL.replace(/\/$/, '')}${signInPath}`;
  // Prefer the application's home URL from flow metadata so the user is returned to the
  // app after sign-up instead of the gate sign-in page. Fall back to the sign-in page if
  // the application URL is not available in the flow metadata.
  const appUrl = meta?.application?.url;
  const afterSignUpUrl = appUrl != null && appUrl !== '' ? appUrl : signInUrl;

  const [localErrors, setLocalErrors] = useState<Record<string, string>>({});
  const [localTouched, setLocalTouched] = useState<Record<string, boolean>>({});
  const componentsRef = useRef<EmbeddedFlowComponent[]>([]);
  const debounceTimers = useRef<Record<string, ReturnType<typeof setTimeout>>>({});
  const formInputsRef = useRef<Record<string, string>>({});

  useEffect(() => {
    const timers = debounceTimers.current;
    return () => {
      Object.values(timers).forEach(clearTimeout);
    };
  }, []);

  const validateFieldFormat = (field: string, value: string): void => {
    const component = componentsRef.current.find(
      (c: EmbeddedFlowComponent) => typeof c.ref === 'string' && c.ref === field,
    );
    if (!component) return;

    let error = '';
    if (
      (component.type as EmbeddedFlowComponentType) === EmbeddedFlowComponentType.EmailInput &&
      value.trim() &&
      !EMAIL_REGEX.test(value)
    ) {
      error = `${t('validations:field.email.invalid', 'Please enter a valid email address.')}`;
    }

    setLocalErrors((prev) => ({...prev, [field]: error}));
    if (error) {
      setLocalTouched((prev) => ({...prev, [field]: true}));
    } else {
      setLocalTouched((prev) => ({...prev, [field]: false}));
    }
  };

  const wrapInputChange = (sdkHandleInputChange: (name: string, value: string) => void) => {
    return (field: string, value: string): void => {
      formInputsRef.current[field] = value;
      sdkHandleInputChange(field, value);

      if (debounceTimers.current[field]) {
        clearTimeout(debounceTimers.current[field]);
      }

      debounceTimers.current[field] = setTimeout(() => {
        validateFieldFormat(field, value);
      }, 600);
    };
  };

  const mergeErrors = (
    sdkErrors: Record<string, string> | undefined,
    sdkTouched: Record<string, boolean> | undefined,
  ): {mergedErrors: Record<string, string>; mergedTouched: Record<string, boolean>} => {
    const mergedErrors = {...localErrors};
    const mergedTouched = {...localTouched};
    if (sdkErrors) {
      Object.entries(sdkErrors).forEach(([key, val]) => {
        if (val) {
          mergedErrors[key] = val;
          mergedTouched[key] = true;
        }
      });
    }
    if (sdkTouched) {
      Object.entries(sdkTouched).forEach(([key, val]) => {
        if (val) mergedTouched[key] = true;
      });
    }
    return {mergedErrors, mergedTouched};
  };

  const collectComponents = (components: EmbeddedFlowComponent[]): void => {
    const fields: EmbeddedFlowComponent[] = [];
    const walk = (comps: EmbeddedFlowComponent[]) => {
      comps.forEach((c: EmbeddedFlowComponent) => {
        if (
          ((c.type as EmbeddedFlowComponentType) === EmbeddedFlowComponentType.TextInput ||
            (c.type as EmbeddedFlowComponentType) === EmbeddedFlowComponentType.PasswordInput ||
            (c.type as EmbeddedFlowComponentType) === EmbeddedFlowComponentType.EmailInput ||
            (c.type as EmbeddedFlowComponentType) === EmbeddedFlowComponentType.PhoneInput ||
            (c.type as EmbeddedFlowComponentType) === EmbeddedFlowComponentType.OtpInput) &&
          c.ref &&
          typeof c.ref === 'string'
        ) {
          fields.push(c);
        }
        if (c.components && Array.isArray(c.components)) walk(c.components);
      });
    };
    walk(components);
    componentsRef.current = fields;
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
      <SignUp afterSignUpUrl={afterSignUpUrl}>
        {({values, fieldErrors, error, touched, handleInputChange, handleSubmit, isLoading, components}: any) => {
          const renderComponents = components as EmbeddedFlowComponent[] | undefined;

          if (!renderComponents) {
            return (
              <>
                <Box sx={{display: 'flex', justifyContent: 'center', p: 3}}>
                  <CircularProgress />
                </Box>
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
            );
          }

          collectComponents(renderComponents);
          const {mergedErrors, mergedTouched} = mergeErrors(
            fieldErrors as Record<string, string> | undefined,
            touched as Record<string, boolean> | undefined,
          );
          const wrappedInputChange = wrapInputChange(handleInputChange as (name: string, value: string) => void);

          return (
            <>
              {error && (
                <Alert severity="error" sx={{mb: 2}}>
                  <AlertTitle>{t('signup:errors.signup.failed.message')}</AlertTitle>
                  {(error as {message?: string}).message ?? t('signup:errors.signup.failed.description')}
                </Alert>
              )}
              {renderComponents.length > 0 ? (
                <Box sx={{display: 'flex', flexDirection: 'column', gap: 2}}>
                  {(isLoading as boolean) && (
                    <Typography sx={{textAlign: 'center'}}>
                      {t('signup:create_account.loading', 'Creating account...')}
                    </Typography>
                  )}
                  {renderComponents.map((component, index) => (
                    <FlowComponentRenderer
                      key={component.id ?? index}
                      component={component}
                      index={index}
                      values={(values as Record<string, string>) ?? {}}
                      touched={mergedTouched}
                      fieldErrors={mergedErrors}
                      isLoading={isLoading as boolean}
                      resolve={resolve}
                      onInputChange={wrappedInputChange}
                      onSubmit={(action, inputs) => {
                        const isTrigger =
                          action.eventType === EmbeddedFlowEventType.Trigger || action.eventType === 'TRIGGER';
                        void (
                          handleSubmit as (
                            a: EmbeddedFlowComponent,
                            i: Record<string, string>,
                            s: boolean,
                          ) => Promise<void>
                        )(action, inputs, isTrigger);
                      }}
                    />
                  ))}
                </Box>
              ) : (
                <Alert severity="error" sx={{mb: 2}}>
                  <AlertTitle>{t('signup:errors.signup.failed.message')}</AlertTitle>
                  {(error as {message?: string})?.message ?? t('signup:errors.signup.failed.description')}
                </Alert>
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
          );
        }}
      </SignUp>
    </AuthCardLayout>
  );
}
