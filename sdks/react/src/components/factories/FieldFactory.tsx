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
import {ChangeEvent, FC, ReactElement} from 'react';
import Checkbox from '../primitives/Checkbox/Checkbox';
import DatePicker from '../primitives/DatePicker/DatePicker';
import OtpField from '../primitives/OtpField/OtpField';
import PasswordField from '../primitives/PasswordField/PasswordField';
import Select, {SelectOption} from '../primitives/Select/Select';
import TextField from '../primitives/TextField/TextField';

/**
 * Interface for field configuration.
 */
export interface FieldConfig {
  /**
   * Additional CSS class name.
   */
  className?: string;
  /**
   * Whether the field is disabled.
   */
  disabled?: boolean;
  /**
   * Error message to display.
   */
  error?: string;
  /**
   * Display name for the field.
   */
  label: string;
  /**
   * The name of the field.
   */
  name: string;
  /**
   * Callback function when the field loses focus.
   */
  onBlur?: () => void;
  /**
   * Callback function when the field value changes.
   */
  onChange: (value: string) => void;
  /**
   * Additional options for multi-valued fields.
   */
  options?: SelectOption[];
  /**
   * Placeholder text for the field.
   */
  placeholder?: string;
  /**
   * Whether the field is required.
   */
  required: boolean;
  /**
   * Whether the field has been touched/interacted with by the user.
   */
  touched?: boolean;
  /**
   * The field type.
   */
  type: FieldType;
  /**
   * Current value of the field.
   */
  value: string;
}

/**
 * Utility function to validate field values based on type
 */
export const validateFieldValue = (
  value: string,
  type: FieldType,
  required = false,
  touched = false,
): string | null => {
  if (required && touched && (!value || value.trim() === '')) {
    return 'This field is required';
  }

  if (!value || value.trim() === '') {
    return null;
  }

  switch (type) {
    case FieldType.Number: {
      const numValue: number = parseInt(value, 10);
      if (Number.isNaN(numValue)) {
        return 'Please enter a valid number';
      }
      break;
    }
    default:
      break;
  }

  return null;
};

/**
 * Factory function to create form fields based on the EmbeddedSignInFlowAuthenticatorParamType.
 *
 * @param config - The field configuration
 * @returns The appropriate React component for the field type
 *
 * @example
 * ```tsx
 * const field = createField({
 *   param: 'username',
 *   type: EmbeddedSignInFlowAuthenticatorParamType.String,
 *   label: 'Username',
 *   confidential: false,
 *   required: true,
 *   value: '',
 *   onChange: (value) => console.log(value)
 * });
 * ```
 */
export const createField = (config: FieldConfig): ReactElement => {
  const {
    name,
    type,
    label,
    required,
    value,
    onChange,
    onBlur,
    disabled = false,
    error,
    className,
    options = [],
    touched = false,
    placeholder,
  } = config;

  const validationError: string | null = error || validateFieldValue(value, type, required, touched);

  const commonProps: Record<string, any> = {
    className,
    'data-testid': `thunderid-signin-${name}`,
    disabled,
    error: validationError,
    label,
    name,
    onBlur,
    placeholder,
    required,
    value,
  };

  switch (type) {
    case FieldType.Password:
      return <PasswordField {...commonProps} onChange={onChange} />;
    case FieldType.Text:
      return (
        <TextField
          {...commonProps}
          type="text"
          onChange={(e: ChangeEvent<HTMLInputElement>): void => onChange(e.target.value)}
          autoComplete="off"
        />
      );
    case FieldType.Email:
      return (
        <TextField
          {...commonProps}
          type="email"
          onChange={(e: ChangeEvent<HTMLInputElement>): void => onChange(e.target.value)}
          autoComplete="email"
        />
      );
    case FieldType.Tel:
      return (
        <TextField
          {...commonProps}
          type="tel"
          onChange={(e: ChangeEvent<HTMLInputElement>): void => onChange(e.target.value)}
          autoComplete="tel"
        />
      );
    case FieldType.Date:
      return (
        <DatePicker {...commonProps} onChange={(e: ChangeEvent<HTMLInputElement>): void => onChange(e.target.value)} />
      );
    case FieldType.Checkbox: {
      const isChecked: boolean = value === 'true' || (value as any) === true;
      return (
        <Checkbox
          {...commonProps}
          checked={isChecked}
          onChange={(e: ChangeEvent<HTMLInputElement>): void => onChange(e.target.checked.toString())}
        />
      );
    }
    case FieldType.Otp:
      return (
        <OtpField {...commonProps} onChange={(e: ChangeEvent<HTMLInputElement>): void => onChange(e.target.value)} />
      );
    case FieldType.Number:
      return (
        <TextField
          {...commonProps}
          type="number"
          onChange={(e: ChangeEvent<HTMLInputElement>): void => onChange(e.target.value)}
          helperText="Enter a numeric value"
        />
      );
    case FieldType.Select: {
      const fieldOptions: SelectOption[] = options.length > 0 ? options : [];

      if (fieldOptions.length > 0) {
        return (
          <Select
            {...commonProps}
            options={fieldOptions}
            onChange={(e: ChangeEvent<HTMLSelectElement>): void => onChange(e.target.value)}
            helperText="Select from available options"
          />
        );
      }

      return (
        <TextField
          {...commonProps}
          type="text"
          onChange={(e: ChangeEvent<HTMLInputElement>): void => onChange(e.target.value)}
          helperText="Enter multiple values separated by commas (e.g., value1, value2, value3)"
          placeholder="value1, value2, value3"
        />
      );
    }
    default:
      return (
        <TextField
          {...commonProps}
          type="text"
          onChange={(e: ChangeEvent<HTMLInputElement>): void => onChange(e.target.value)}
          helperText="Unknown field type, treating as text"
        />
      );
  }
};

/**
 * React component wrapper for the field factory.
 */
export const FieldFactory: FC<FieldConfig> = (props: FieldConfig): ReactElement => createField(props);

export default FieldFactory;
