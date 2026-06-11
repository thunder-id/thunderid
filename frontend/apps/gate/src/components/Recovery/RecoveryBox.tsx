/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
import {useTemplateLiteralResolver} from '@thunderid/hooks';
import {useLogger} from '@thunderid/logger/react';
import {EmbeddedFlowEventType, Recovery, type EmbeddedFlowComponent} from '@thunderid/react';
import {TemplateLiteralType} from '@thunderid/utils';
import {Box, Alert, CircularProgress} from '@wso2/oxygen-ui';
import type {JSX, ReactNode} from 'react';
import {useState} from 'react';
import {useTranslation} from 'react-i18next';
import {useSearchParams} from 'react-router';
import ROUTES from '../../constants/routes';

export default function RecoveryBox(): JSX.Element {
  const {resolveAll} = useTemplateLiteralResolver();
  const [searchParams] = useSearchParams();

  const {t} = useTranslation();
  const logger = useLogger('RecoveryBox');
  const {isDesignEnabled} = useDesign();
  const [flowError, setFlowError] = useState<string | null>(null);

  const base = import.meta.env.BASE_URL.replace(/\/$/, '');
  const applicationId = searchParams.get('applicationId');
  const signInUrl = applicationId
    ? `${base}${ROUTES.AUTH.SIGN_IN}?applicationId=${applicationId}`
    : `${base}${ROUTES.AUTH.SIGN_IN}`;

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
            meta: flowMeta,
          }: any) =>
            isLoading && !components?.length ? (
              <Box sx={{display: 'flex', justifyContent: 'center', p: 3}}>
                <CircularProgress />
              </Box>
            ) : (
              <>
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
                        signInFallbackUrl={signInUrl}
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
