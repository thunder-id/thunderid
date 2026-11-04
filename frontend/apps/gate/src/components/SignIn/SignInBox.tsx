// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useDesign, FlowComponentRenderer, AuthCardLayout} from '@thunderid/design';
import {useTemplateLiteralResolver} from '@thunderid/hooks';
import {SignIn, type EmbeddedFlowComponent} from '@thunderid/react';
import {TemplateLiteralType} from '@thunderid/utils';
import {Box, Alert, CircularProgress, Link, Typography} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useFlowForm from '../../hooks/useFlowForm';

export default function SignInBox(): JSX.Element {
  const {resolve, resolveAll} = useTemplateLiteralResolver();
  const {t} = useTranslation();
  const {isDesignEnabled} = useDesign();
  const {formInputs, fieldErrors, touched, collectInputComponents, validateForm, updateInput, clearInputs} =
    useFlowForm(resolve);

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
      <SignIn>
        {({onSubmit, isLoading, components, error, meta: flowMeta, additionalData}) =>
          isLoading && !components?.length ? (
            <Box sx={{display: 'flex', justifyContent: 'center', p: 3}}>
              <CircularProgress />
            </Box>
          ) : (
            <>
              {/* Held back while a submission is in flight. An expired consent prompt auto-submits, and
                  the SDK raises its expiry error against the request already on the wire, which would
                  otherwise flash an error the user cannot act on. Errors worth showing outlive the
                  request, since the flow clears them as each submission starts. */}
              {error && !isLoading && (
                <Alert severity="error" sx={{mb: 2}}>
                  {error.message ??
                    t(
                      'signin:errors.signin.failed.description',
                      'We are sorry, something has gone wrong here. Please try again.',
                    )}
                </Alert>
              )}
              {(() => {
                const renderComponents = components && components.length > 0 ? components : [];

                collectInputComponents(renderComponents);
                if (renderComponents.length > 0) {
                  return (
                    <Box sx={{display: 'flex', flexDirection: 'column', gap: 2}}>
                      {renderComponents.map((component: EmbeddedFlowComponent, index: number) => (
                        <FlowComponentRenderer
                          key={component.id ?? index}
                          component={component}
                          index={index}
                          values={formInputs}
                          touched={touched}
                          fieldErrors={fieldErrors}
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
                            void onSubmit({inputs, action: action.id}).finally(() => {
                              clearInputs();
                            });
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
