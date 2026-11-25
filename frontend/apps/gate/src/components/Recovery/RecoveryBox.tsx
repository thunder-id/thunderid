// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/* eslint-disable @typescript-eslint/no-unsafe-assignment */
/* eslint-disable @typescript-eslint/no-explicit-any */
/* eslint-disable @typescript-eslint/no-unsafe-member-access */
/* eslint-disable @typescript-eslint/no-unsafe-call */

import {FlowComponentRenderer, AuthCardLayout, useDesign} from '@thunderid/design';
import {useTemplateLiteralResolver} from '@thunderid/hooks';
import {useLogger} from '@thunderid/logger/react';
import {EmbeddedFlowEventType, Recovery, useThunderID, type EmbeddedFlowComponent} from '@thunderid/react';
import {TemplateLiteralType} from '@thunderid/utils';
import {Box, Alert, CircularProgress} from '@wso2/oxygen-ui';
import type {JSX, ReactNode} from 'react';
import {useState} from 'react';
import {useTranslation} from 'react-i18next';
import {useSearchParams} from 'react-router';
import RouteConfig from '../../configs/RouteConfig';
import FlowExpired from '../FlowExpired/FlowExpired';

export default function RecoveryBox(): JSX.Element {
  const {resolveAll} = useTemplateLiteralResolver();
  const [searchParams] = useSearchParams();

  const {t} = useTranslation();
  const logger = useLogger('RecoveryBox');
  const {isDesignEnabled} = useDesign();
  const {meta} = useThunderID();
  const [flowError, setFlowError] = useState<string | null>(null);

  const base = import.meta.env.BASE_URL.replace(/\/$/, '');
  const applicationId = searchParams.get('applicationId');
  const signInUrl = applicationId
    ? `${base}${RouteConfig.signIn()}?applicationId=${applicationId}`
    : `${base}${RouteConfig.signIn()}`;

  return (
    <AuthCardLayout
      variant="RecoveryBox"
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
      <Recovery
        afterRecoveryUrl={signInUrl}
        afterFlowExpiredUrl={meta?.application?.url}
        onError={(error: Error) => {
          logger.error('Recovery error:', error);
        }}
        onFlowChange={(response: any) => {
          const messageKey: string | undefined = response?.error?.message?.key;
          if (messageKey) {
            const translated: string = t(messageKey);
            if (translated !== messageKey) {
              setFlowError(translated);

              return;
            }
          }
          const messageDefaultTrimmed: string = response?.error?.message?.defaultValue?.trim() ?? '';
          const messageDefault: string | undefined = messageDefaultTrimmed !== '' ? messageDefaultTrimmed : undefined;
          const fallback: string | undefined = messageDefault ?? response?.error?.description?.defaultValue;
          setFlowError(fallback ?? null);
        }}
      >
        {
          (({
            fieldErrors,
            error,
            touched,
            isLoading,
            components,
            values,
            handleInputChange,
            handleSubmit,
            isFlowExpired,
            isFlowRestarted,
            flowExpiryRedirect,
            meta: flowMeta,
          }: any) =>
            isFlowExpired ? (
              <FlowExpired applicationName={flowMeta?.application?.name} redirect={flowExpiryRedirect} />
            ) : isLoading && !components?.length ? (
              <Box sx={{display: 'flex', justifyContent: 'center', p: 3}}>
                <CircularProgress />
              </Box>
            ) : (
              <>
                {isFlowRestarted && (
                  <Alert severity="info" sx={{mb: 2}}>
                    {t('auth:flowExpired.restarted', 'Your session expired, so we started again.')}
                  </Alert>
                )}
                {(flowError ?? error) && (
                  <Alert severity="error" sx={{mb: 2}}>
                    {flowError ??
                      error?.message ??
                      t('recovery:errors.failed.description', 'Something went wrong. Please try again.')}
                  </Alert>
                )}
                {(components as EmbeddedFlowComponent[])?.length > 0 && (
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
                        onInputChange={handleInputChange}
                        onSubmit={(action, inputs) => {
                          const isTrigger =
                            action.eventType === EmbeddedFlowEventType.Trigger || action.eventType === 'TRIGGER';
                          void handleSubmit(action, inputs, isTrigger);
                        }}
                      />
                    ))}
                  </Box>
                )}
              </>
            )) as unknown as ReactNode
        }
      </Recovery>
    </AuthCardLayout>
  );
}
