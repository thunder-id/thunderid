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

import {useDesign, FlowComponentRenderer, AuthCardLayout} from '@thunderid/design';
import {useTemplateLiteralResolver} from '@thunderid/hooks';
import {EmbeddedFlowComponentType, SignIn, type EmbeddedFlowComponent} from '@thunderid/react';
import {EMAIL_REGEX, TemplateLiteralType} from '@thunderid/utils';
import {Box, Alert, CircularProgress} from '@wso2/oxygen-ui';
import {useEffect, useRef, useState} from 'react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';

export default function SignInBox(): JSX.Element {
  const {resolve, resolveAll} = useTemplateLiteralResolver();
  const {t} = useTranslation();
  const {isDesignEnabled} = useDesign();

  const [formInputs, setFormInputs] = useState<Record<string, string>>({});
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [touched, setTouched] = useState<Record<string, boolean>>({});
  const componentsRef = useRef<EmbeddedFlowComponent[]>([]);
  const debounceTimers = useRef<Record<string, ReturnType<typeof setTimeout>>>({});

  useEffect(() => {
    const timers = debounceTimers.current;
    return () => {
      Object.values(timers).forEach(clearTimeout);
    };
  }, []);

  const collectInputComponents = (components: EmbeddedFlowComponent[]): void => {
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

  const validateFieldFormat = (field: string, value: string): void => {
    const component = componentsRef.current.find((c) => typeof c.ref === 'string' && c.ref === field);
    if (!component) return;

    let error = '';
    if (
      (component.type as EmbeddedFlowComponentType) === EmbeddedFlowComponentType.EmailInput &&
      value.trim() &&
      !EMAIL_REGEX.test(value)
    ) {
      error = `${t('validations:field.email.invalid', 'Please enter a valid email address.')}`;
    }

    setFieldErrors((prev) => ({...prev, [field]: error}));
    if (error) {
      setTouched((prev) => ({...prev, [field]: true}));
    } else {
      setTouched((prev) => ({...prev, [field]: false}));
    }
  };

  const validateForm = (components: EmbeddedFlowComponent[]): boolean => {
    const errors: Record<string, string> = {};
    const touchedFields: Record<string, boolean> = {};
    let isValid = true;

    const check = (comps: EmbeddedFlowComponent[]) => {
      comps.forEach((component: EmbeddedFlowComponent) => {
        if (
          ((component.type as EmbeddedFlowComponentType) === EmbeddedFlowComponentType.TextInput ||
            (component.type as EmbeddedFlowComponentType) === EmbeddedFlowComponentType.PasswordInput ||
            (component.type as EmbeddedFlowComponentType) === EmbeddedFlowComponentType.EmailInput ||
            (component.type as EmbeddedFlowComponentType) === EmbeddedFlowComponentType.PhoneInput ||
            (component.type as EmbeddedFlowComponentType) === EmbeddedFlowComponentType.OtpInput) &&
          component.ref &&
          typeof component.ref === 'string' &&
          typeof component.label === 'string'
        ) {
          const value = formInputs[component.ref] ?? '';
          if (component.required && !value.trim()) {
            const fieldLabel = t(resolve(component.label)!, component.label);
            errors[component.ref] =
              `${t('validations:form.field.required', {defaultValue: '{{field}} is required.', field: fieldLabel})}`;
            touchedFields[component.ref] = true;
            isValid = false;
          } else if (
            (component.type as EmbeddedFlowComponentType) === EmbeddedFlowComponentType.EmailInput &&
            value.trim() &&
            !EMAIL_REGEX.test(value)
          ) {
            errors[component.ref] = `${t('validations:field.email.invalid', 'Please enter a valid email address.')}`;
            touchedFields[component.ref] = true;
            isValid = false;
          }
        }
        if (component.components && Array.isArray(component.components)) check(component.components);
      });
    };
    check(components);

    setFieldErrors(errors);
    setTouched((prev) => ({...prev, ...touchedFields}));
    return isValid;
  };

  const updateInput = (field: string, value: string): void => {
    setFormInputs((prev) => ({...prev, [field]: value}));

    if (debounceTimers.current[field]) {
      clearTimeout(debounceTimers.current[field]);
    }

    debounceTimers.current[field] = setTimeout(() => {
      validateFieldFormat(field, value);
    }, 600);
  };

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
        {({onSubmit, isLoading, components, error, isInitialized, meta: flowMeta, additionalData}) =>
          (isLoading ?? !isInitialized) ? (
            <Box sx={{display: 'flex', justifyContent: 'center', p: 3}}>
              <CircularProgress />
            </Box>
          ) : (
            <>
              {error && (
                <Alert severity="error" sx={{mb: 2}}>
                  {error.message ?? t('signin:errors.signin.failed.description')}
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
                              setFormInputs({});
                              setFieldErrors({});
                            });
                          }}
                        />
                      ))}
                    </Box>
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
