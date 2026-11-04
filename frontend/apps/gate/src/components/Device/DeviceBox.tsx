// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useDesign, FlowComponentRenderer, AuthCardLayout} from '@thunderid/design';
import {useTemplateLiteralResolver} from '@thunderid/hooks';
import {
  DeviceGrant,
  CONSENT_INVALID_KEY_PREFIX,
  CONSENT_MODIFIED_KEY_PREFIX,
  getConsentElementDecision,
  getConsentOptionalKey,
  type DeviceGrantRenderProps,
  type EmbeddedFlowComponent,
} from '@thunderid/react';
import {TemplateLiteralType} from '@thunderid/utils';
import {Alert, Box, CircularProgress, Typography} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useFlowForm from '../../hooks/useFlowForm';

interface ConsentElement {
  name: string;
}

interface ConsentPurpose {
  purposeId: string;
  purposeName?: string;
  essential?: ConsentElement[] | null;
  optional?: ConsentElement[] | null;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}

function isConsentElement(value: unknown): value is ConsentElement {
  return isRecord(value) && typeof value.name === 'string';
}

function parseConsentPurposes(value: unknown): ConsentPurpose[] | null {
  let parsed: unknown = value;
  if (typeof parsed === 'string') {
    try {
      parsed = JSON.parse(parsed) as unknown;
    } catch {
      return null;
    }
  }

  if (isRecord(parsed)) parsed = parsed.purposes;
  if (!Array.isArray(parsed)) return null;

  const purposes: ConsentPurpose[] = [];
  for (const purpose of parsed) {
    if (!isRecord(purpose) || typeof purpose.purposeId !== 'string') return null;
    const essential = purpose.essential;
    const optional = purpose.optional;
    if (
      (essential != null && (!Array.isArray(essential) || !essential.every(isConsentElement))) ||
      (optional != null && (!Array.isArray(optional) || !optional.every(isConsentElement)))
    ) {
      return null;
    }
    purposes.push({
      purposeId: purpose.purposeId,
      purposeName: typeof purpose.purposeName === 'string' ? purpose.purposeName : undefined,
      essential,
      optional,
    });
  }

  return purposes;
}

function compileConsentInputs(
  action: EmbeddedFlowComponent,
  inputs: Record<string, string>,
  additionalData: Record<string, unknown>,
): Record<string, string> {
  const consentPrompt = additionalData.consentPrompt;
  if (consentPrompt == null || action.eventType?.toUpperCase() !== 'SUBMIT') return inputs;

  const purposes = parseConsentPurposes(consentPrompt);
  if (!purposes) return inputs;

  const isDeny = action.variant?.toLowerCase() !== 'primary';
  const decisions = {
    approved: !isDeny,
    ...(isDeny ? {reason: 'user_denied'} : {}),
    purposes: purposes.map((purpose) => ({
      approved: !isDeny,
      elements: [
        ...(purpose.essential ?? []).map((element) => ({approved: !isDeny, name: element.name})),
        ...(purpose.optional ?? []).map((element) =>
          getConsentElementDecision(
            purpose.purposeId,
            element,
            !isDeny && inputs[getConsentOptionalKey(purpose.purposeId, element.name)] === 'true',
            inputs,
          ),
        ),
      ],
      purposeName: purpose.purposeName ?? '',
    })),
  };
  const compiledInputs: Record<string, string> = {...inputs, consent_decisions: JSON.stringify(decisions)};
  Object.keys(compiledInputs).forEach((key) => {
    if (
      key.startsWith('__consent_opt__') ||
      key.startsWith(CONSENT_MODIFIED_KEY_PREFIX) ||
      key.startsWith(CONSENT_INVALID_KEY_PREFIX)
    ) {
      delete compiledInputs[key];
    }
  });
  return compiledInputs;
}

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
                        const submittedInputs = compileConsentInputs(action, inputs, additionalData);
                        void onSubmit({inputs: submittedInputs, action: action.id}).finally(() => {
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
