// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useDesign, FlowComponentRenderer, AuthCardLayout} from '@thunderid/design';
import {useTemplateLiteralResolver} from '@thunderid/hooks';
import {SignIn, useThunderID, type EmbeddedFlowComponent} from '@thunderid/react';
import {TemplateLiteralType} from '@thunderid/utils';
import {Box, Alert, CircularProgress, Link, Typography} from '@wso2/oxygen-ui';
import {useEffect, useRef, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useFlowForm from '../../hooks/useFlowForm';
import FlowExpired from '../FlowExpired/FlowExpired';

interface ClearInputsOnNewStepProps {
  components: EmbeddedFlowComponent[];
  hasFieldErrors: boolean;
  onNewStep: () => void;
}

/**
 * Clears the form when the flow moves to a new step. A response with field errors shows the same step
 * again, so the values stay for the user to correct.
 */
function ClearInputsOnNewStep({components, hasFieldErrors, onNewStep}: ClearInputsOnNewStepProps): null {
  const hasFieldErrorsRef = useRef(hasFieldErrors);

  // The SDK clears field errors when a submission starts, before the next step arrives, so only a
  // change of components marks a new step. The ref carries the latest value into that effect.
  useEffect(() => {
    hasFieldErrorsRef.current = hasFieldErrors;
  });

  useEffect(() => {
    if (!hasFieldErrorsRef.current) {
      onNewStep();
    }
  }, [components, onNewStep]);

  return null;
}

export default function SignInBox(): JSX.Element {
  const {resolve, resolveAll} = useTemplateLiteralResolver();
  const {t} = useTranslation();
  const {isDesignEnabled} = useDesign();
  const {meta} = useThunderID();
  const {formInputs, fieldErrors, touched, collectInputComponents, validateForm, updateInput, clearInputs} =
    useFlowForm(resolve);
  const [submittedInputs, setSubmittedInputs] = useState<Record<string, string>>({});

  return (
    <AuthCardLayout
      variant="SignInBox"
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
      <SignIn afterFlowExpiredUrl={meta?.application?.url}>
        {({
          onSubmit,
          isLoading,
          isFlowExpired,
          isFlowRestarted,
          flowExpiryRedirect,
          components,
          error,
          fieldErrors: serverFieldErrors,
          meta: flowMeta,
          additionalData,
        }) =>
          isFlowExpired ? (
            <FlowExpired applicationName={flowMeta?.application?.name} redirect={flowExpiryRedirect} />
          ) : isLoading && !components?.length ? (
            <Box sx={{display: 'flex', justifyContent: 'center', p: 3}}>
              <CircularProgress />
            </Box>
          ) : (
            <>
              {/* Held back while a submission is in flight. An expired consent prompt auto-submits, and
                  the SDK raises its expiry error against the request already on the wire, which would
                  otherwise flash an error the user cannot act on. Errors worth showing outlive the
                  request, since the flow clears them as each submission starts. */}
              {isFlowRestarted && (
                <Alert severity="info" sx={{mb: 2}}>
                  {t('auth:flowExpired.restarted', 'Your session expired, so we started again.')}
                </Alert>
              )}
              {error && !isLoading && (
                <Alert severity="error" sx={{mb: 2}}>
                  {error.message ??
                    t(
                      'signin:errors.signin.failed.description',
                      'We are sorry, something has gone wrong here. Please try again.',
                    )}
                </Alert>
              )}
              <ClearInputsOnNewStep
                components={components}
                hasFieldErrors={Object.keys(serverFieldErrors).length > 0}
                onNewStep={clearInputs}
              />
              {(() => {
                const renderComponents = components && components.length > 0 ? components : [];

                collectInputComponents(renderComponents);

                // A server error stays on its field until the user changes the value that was refused.
                const visibleFieldErrors: Record<string, string> = {...fieldErrors};
                const visibleTouched: Record<string, boolean> = {...touched};
                Object.entries(serverFieldErrors).forEach(([ref, message]) => {
                  if ((formInputs[ref] ?? '') === (submittedInputs[ref] ?? '')) {
                    visibleFieldErrors[ref] = message;
                    visibleTouched[ref] = true;
                  }
                });

                if (renderComponents.length > 0) {
                  return (
                    <Box sx={{display: 'flex', flexDirection: 'column', gap: 2}}>
                      {renderComponents.map((component: EmbeddedFlowComponent, index: number) => (
                        <FlowComponentRenderer
                          key={component.id ?? index}
                          component={component}
                          index={index}
                          values={formInputs}
                          touched={visibleTouched}
                          fieldErrors={visibleFieldErrors}
                          isLoading={isLoading}
                          additionalData={additionalData}
                          resolve={(template) =>
                            resolveAll(template, {
                              [TemplateLiteralType.TRANSLATION]: t,
                              [TemplateLiteralType.META]: (path: string) => {
                                const keys = path.split('.');
                                const value: unknown = keys.reduce<unknown>((acc: unknown, key: string): unknown => {
                                  if (acc == null || typeof acc !== 'object') return acc;
                                  const record = acc as Record<string, unknown>;
                                  return record[key] ?? record[key.replace(/([A-Z])/g, '_$1').toLowerCase()];
                                }, flowMeta as unknown);

                                return (value as string | undefined) ?? `{{meta(${path})}}`;
                              },
                            })
                          }
                          onInputChange={updateInput}
                          onValidate={validateForm}
                          onSubmit={(action, inputs) => {
                            setSubmittedInputs(inputs);
                            void onSubmit({inputs, action: action.id});
                          }}
                        />
                      ))}
                    </Box>
                  );
                }

                // Terminal failure with nothing to render. A spinner here would look like loading,
                // so offer the way back to the application instead.
                if (error) {
                  const applicationUrl = flowMeta?.application?.url;
                  if (!applicationUrl) {
                    // No application URL to link back to, so at least tell the user what to do.
                    return (
                      <Typography sx={{textAlign: 'center', p: 1}}>
                        {t(
                          'signin:errors.signin.returnToApplicationUnavailable',
                          'Please return to the application and try again.',
                        )}
                      </Typography>
                    );
                  }
                  return (
                    <Typography sx={{textAlign: 'center', p: 1}}>
                      <Link href={applicationUrl}>
                        {t('signin:errors.signin.returnToApplication', 'Return to application')}
                      </Link>
                    </Typography>
                  );
                }

                return (
                  <Box sx={{display: 'flex', justifyContent: 'center', p: 3}}>
                    <CircularProgress />
                  </Box>
                );
              })()}
            </>
          )
        }
      </SignIn>
    </AuthCardLayout>
  );
}
