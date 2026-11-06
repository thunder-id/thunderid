// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useDesign, FlowComponentRenderer, AuthCardLayout} from '@thunderid/design';
import {useTemplateLiteralResolver} from '@thunderid/hooks';
import {
  DeviceGrant,
  type DeviceGrantRenderProps,
  type EmbeddedFlowComponent,
} from '@thunderid/react';
import {TemplateLiteralType} from '@thunderid/utils';
import {Alert, Box, CircularProgress, Typography} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useFlowForm from '../../hooks/useFlowForm';

export default function DeviceBox(): JSX.Element {
  const {resolve, resolveAll} = useTemplateLiteralResolver();
  const {t} = useTranslation();
  const {isDesignEnabled} = useDesign();
  const {
    formInputs,
    fieldErrors: clientFieldErrors,
    touched,
    collectInputComponents,
    validateForm,
    updateInput,
    clearInputs,
  } = useFlowForm(resolve);

  return (
    <AuthCardLayout
      variant="DeviceBox"
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
      <DeviceGrant>
        {({
          onSubmit,
          isLoading,
          components,
          error,
          meta: flowMeta,
          additionalData,
          fieldErrors,
          initialValues,
          result,
        }: DeviceGrantRenderProps) => {
          if (isLoading) {
            return (
              <Box sx={{display: 'flex', justifyContent: 'center', p: 3}}>
                <CircularProgress />
              </Box>
            );
          }

          const renderComponents = components ?? [];

          // On completion, render the terminal prompt returned by the configured flow when present.
          // The generic result copy is only a fallback for flows without a terminal prompt.
          if (result && renderComponents.length === 0) {
            const isSuccess: boolean = result === 'success';
            return (
              <Box sx={{display: 'flex', flexDirection: 'column', gap: 1, py: 1, textAlign: 'center'}}>
                <Typography color={isSuccess ? 'success' : 'error'} component="p" variant="h6">
                  {isSuccess
                    ? t('deviceGrant.messages.complete', 'Device authorized')
                    : t('deviceGrant.messages.failed', 'Device authorization failed')}
                </Typography>
                <Typography color="text.secondary" component="p" variant="body2">
                  {isSuccess
                    ? t('deviceGrant.messages.completeDescription', 'You can return to your device.')
                    : t('deviceGrant.messages.failedDescription', 'Return to your device and start again.')}
                </Typography>
              </Box>
            );
          }

          collectInputComponents(renderComponents);
          const visibleFieldErrors = {...fieldErrors, ...clientFieldErrors};

          if (renderComponents.length > 0) {
            return (
              <>
                {error && !isLoading && (
                  <Alert severity="error" sx={{mb: 2}}>
                    {error.message}
                  </Alert>
                )}
                <Box sx={{display: 'flex', flexDirection: 'column', gap: 2}}>
                  {renderComponents.map((component: EmbeddedFlowComponent, index: number) => (
                    <FlowComponentRenderer
                      key={component.id ?? index}
                      component={component}
                      index={index}
                      values={{...initialValues, ...formInputs}}
                      touched={touched}
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
                      onValidate={(items) => validateForm(items, initialValues)}
                      onSubmit={(action, inputs) => {
                        void onSubmit({inputs, action: action.id}).finally(() => {
                          clearInputs();
                        });
                      }}
                    />
                  ))}
                </Box>
              </>
            );
          }

          if (error) {
            return <Alert severity="error">{error.message}</Alert>;
          }

          return (
            <Box sx={{display: 'flex', justifyContent: 'center', p: 3}}>
              <CircularProgress />
            </Box>
          );
        }}
      </DeviceGrant>
    </AuthCardLayout>
  );
}
