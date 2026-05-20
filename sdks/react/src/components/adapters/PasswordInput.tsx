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

import {FieldType} from '@thunderid/browser';
import {FC} from 'react';
import {AdapterProps} from '../../models/adapters';
import {createField} from '../factories/FieldFactory';

/**
 * Password input component for sign-up forms.
 */
const PasswordInput: FC<AdapterProps> = ({
  component,
  formValues,
  touchedFields,
  formErrors,
  onInputChange,
  inputClassName,
}: AdapterProps) => {
  const config: Record<string, unknown> = component.config || {};
  const fieldName: string = (config['identifier'] as string) || (config['name'] as string) || component.id;
  const value: string = formValues[fieldName] || '';
  const error: string | undefined = touchedFields[fieldName] ? formErrors[fieldName] : undefined;

  // Extract validation rules from the component config if available
  const validations: {
    conditions?: {key: string; value: string}[];
    name: string;
  }[] =
    (config['validations'] as {
      conditions?: {key: string; value: string}[];
      name: string;
    }[]) || [];
  const validationHints: string[] = [];

  validations.forEach((validation: any) => {
    if (validation.name === 'LengthValidator') {
      const minLength: string | undefined = validation.conditions?.find((c: any) => c.key === 'min.length')?.value;
      const maxLength: string | undefined = validation.conditions?.find((c: any) => c.key === 'max.length')?.value;
      if (minLength || maxLength) {
        validationHints.push(`Length: ${minLength || '0'}-${maxLength || '∞'} characters`);
      }
    } else if (validation.name === 'UpperCaseValidator') {
      const minLength: string | undefined = validation.conditions?.find((c: any) => c.key === 'min.length')?.value;
      if (minLength && parseInt(minLength, 10) > 0) {
        validationHints.push('Must contain uppercase letter(s)');
      }
    } else if (validation.name === 'LowerCaseValidator') {
      const minLength: string | undefined = validation.conditions?.find((c: any) => c.key === 'min.length')?.value;
      if (minLength && parseInt(minLength, 10) > 0) {
        validationHints.push('Must contain lowercase letter(s)');
      }
    } else if (validation.name === 'NumeralValidator') {
      const minLength: string | undefined = validation.conditions?.find((c: any) => c.key === 'min.length')?.value;
      if (minLength && parseInt(minLength, 10) > 0) {
        validationHints.push('Must contain number(s)');
      }
    } else if (validation.name === 'SpecialCharacterValidator') {
      const minLength: string | undefined = validation.conditions?.find((c: any) => c.key === 'min.length')?.value;
      if (minLength && parseInt(minLength, 10) > 0) {
        validationHints.push('Must contain special character(s)');
      }
    }
  });

  return createField({
    className: inputClassName,
    error,
    label: (config['label'] as string) || 'Password',
    name: fieldName,
    onChange: (newValue: string) => onInputChange(fieldName, newValue),
    placeholder: (config['placeholder'] as string) || 'Enter your password',
    required: (config['required'] as boolean) || false,
    type: FieldType.Password,
    value,
  });
};

export default PasswordInput;
