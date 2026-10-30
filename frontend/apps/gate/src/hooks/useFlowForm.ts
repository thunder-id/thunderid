// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {EmbeddedFlowComponentType, type EmbeddedFlowComponent} from '@thunderid/react';
import {EMAIL_REGEX} from '@thunderid/utils';
import {useCallback, useEffect, useRef, useState} from 'react';
import {useTranslation} from 'react-i18next';

export default function useFlowForm(resolve: (template: string | undefined) => string | undefined) {
  const {t} = useTranslation();
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

  const collectInputComponents = useCallback((components: EmbeddedFlowComponent[]): void => {
    const fields: EmbeddedFlowComponent[] = [];
    const walk = (items: EmbeddedFlowComponent[]) => {
      items.forEach((component) => {
        if (
          ((component.type as EmbeddedFlowComponentType) === EmbeddedFlowComponentType.TextInput ||
            (component.type as EmbeddedFlowComponentType) === EmbeddedFlowComponentType.PasswordInput ||
            (component.type as EmbeddedFlowComponentType) === EmbeddedFlowComponentType.EmailInput ||
            (component.type as EmbeddedFlowComponentType) === EmbeddedFlowComponentType.PhoneInput ||
            (component.type as EmbeddedFlowComponentType) === EmbeddedFlowComponentType.OtpInput) &&
          component.ref &&
          typeof component.ref === 'string'
        ) {
          fields.push(component);
        }
        if (component.components && Array.isArray(component.components)) walk(component.components);
      });
    };
    walk(components);
    componentsRef.current = fields;
  }, []);

  const validateFieldFormat = useCallback(
    (field: string, value: string): void => {
      const component = componentsRef.current.find((item) => item.ref === field);
      if (!component) return;

      const message =
        (component.type as EmbeddedFlowComponentType) === EmbeddedFlowComponentType.EmailInput &&
        value.trim() &&
        !EMAIL_REGEX.test(value)
          ? t('validations:field.email.invalid', 'Please enter a valid email address.')
          : '';

      setFieldErrors((current) => ({...current, [field]: message}));
      setTouched((current) => ({...current, [field]: Boolean(message)}));
    },
    [t],
  );

  const validateForm = useCallback(
    (components: EmbeddedFlowComponent[], initialValues: Record<string, string> = {}): boolean => {
      const errors: Record<string, string> = {};
      const touchedFields: Record<string, boolean> = {};
      let isValid = true;

      const check = (items: EmbeddedFlowComponent[]) => {
        items.forEach((component) => {
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
            const value = formInputs[component.ref] ?? initialValues[component.ref] ?? '';
            if (component.required && !value.trim()) {
              const fieldLabel = t(resolve(component.label)!, component.label);
              errors[component.ref] = t('validations:form.field.required', {
                defaultValue: '{{field}} is required.',
                field: fieldLabel,
              });
              touchedFields[component.ref] = true;
              isValid = false;
            } else if (
              (component.type as EmbeddedFlowComponentType) === EmbeddedFlowComponentType.EmailInput &&
              value.trim() &&
              !EMAIL_REGEX.test(value)
            ) {
              errors[component.ref] = t('validations:field.email.invalid', 'Please enter a valid email address.');
              touchedFields[component.ref] = true;
              isValid = false;
            }
          }
          if (component.components && Array.isArray(component.components)) check(component.components);
        });
      };
      check(components);

      setFieldErrors(errors);
      setTouched((current) => ({...current, ...touchedFields}));
      return isValid;
    },
    [formInputs, resolve, t],
  );

  const updateInput = useCallback(
    (field: string, value: string): void => {
      setFormInputs((current) => ({...current, [field]: value}));
      if (debounceTimers.current[field]) clearTimeout(debounceTimers.current[field]);
      debounceTimers.current[field] = setTimeout(() => validateFieldFormat(field, value), 600);
    },
    [validateFieldFormat],
  );

  const clearInputs = useCallback((): void => {
    setFormInputs({});
    setFieldErrors({});
  }, []);

  return {
    formInputs,
    fieldErrors,
    touched,
    collectInputComponents,
    validateForm,
    updateInput,
    clearInputs,
  };
}
