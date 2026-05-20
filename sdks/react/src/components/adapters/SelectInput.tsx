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
import {SelectOption} from '../primitives/Select/Select';

/**
 * Select input component for sign-up forms.
 */
const SelectInput: FC<AdapterProps> = ({
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
  const error: string = touchedFields[fieldName] ? formErrors[fieldName] : undefined;

  // Get options from config and convert to SelectOption format
  const rawOptions: string[] = (config['options'] as string[]) || [];
  const options: SelectOption[] = rawOptions.map((option: string) => ({
    label: option,
    value: option,
  }));

  return createField({
    className: inputClassName,
    error,
    label: (config['label'] as string) || '',
    name: fieldName,
    onChange: (newValue: string): void => onInputChange(fieldName, newValue),
    options,
    placeholder: (config['placeholder'] as string) || '',
    required: (config['required'] as boolean) || false,
    type: FieldType.Select,
    value,
  });
};

export default SelectInput;
