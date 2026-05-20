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
import {type Component, type PropType, type VNode, defineComponent, h} from 'vue';
import Checkbox from '../primitives/Checkbox';
import DatePicker from '../primitives/DatePicker';
import OtpField from '../primitives/OtpField';
import PasswordField from '../primitives/PasswordField';
import Select, {type SelectOption} from '../primitives/Select/Select';
import TextField from '../primitives/TextField';

/**
 * Interface for field configuration.
 */
export interface FieldConfig {
  className?: string;
  disabled?: boolean;
  error?: string;
  label: string;
  name: string;
  onBlur?: () => void;
  onChange: (value: string) => void;
  options?: SelectOption[];
  placeholder?: string;
  required: boolean;
  touched?: boolean;
  type: FieldType;
  value: string;
}

interface FieldFactorySetupProps {
  className?: string;
  disabled: boolean;
  error?: string;
  label: string;
  name: string;
  options: SelectOption[];
  placeholder?: string;
  required: boolean;
  touched: boolean;
  type: FieldType;
  value: string;
}

/**
 * Utility function to validate field values based on type.
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
 * Factory function to create form field VNodes based on FieldType.
 */
export const createField = (config: FieldConfig): VNode => {
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

  const validationError: string | null | undefined = error || validateFieldValue(value, type, required, touched);

  const commonProps: Record<string, unknown> = {
    class: className,
    'data-testid': `thunderid-signin-${name}`,
    disabled,
    error: validationError,
    label,
    modelValue: value,
    name,
    onBlur,
    placeholder,
    required,
  };

  switch (type) {
    case FieldType.Password:
      return h(PasswordField, {
        ...commonProps,
        'onUpdate:modelValue': onChange,
      } as Record<string, unknown>);

    case FieldType.Text:
      return h(TextField, {
        ...commonProps,
        autocomplete: 'off',
        'onUpdate:modelValue': onChange,
        type: 'text',
      } as Record<string, unknown>);

    case FieldType.Email:
      return h(TextField, {
        ...commonProps,
        autocomplete: 'email',
        'onUpdate:modelValue': onChange,
        type: 'email',
      } as Record<string, unknown>);

    case FieldType.Date:
      return h(DatePicker, {
        ...commonProps,
        'onUpdate:modelValue': onChange,
      } as Record<string, unknown>);

    case FieldType.Checkbox: {
      const isChecked: boolean = value === 'true' || (value as unknown) === true;

      return h(Checkbox, {
        ...commonProps,
        modelValue: isChecked,
        'onUpdate:modelValue': (checked: boolean) => onChange(checked.toString()),
      } as Record<string, unknown>);
    }

    case FieldType.Otp:
      return h(OtpField, {
        ...commonProps,
        'onUpdate:modelValue': onChange,
      } as Record<string, unknown>);

    case FieldType.Number:
      return h(TextField, {
        ...commonProps,
        helperText: 'Enter a numeric value',
        'onUpdate:modelValue': onChange,
        type: 'number',
      } as Record<string, unknown>);

    case FieldType.Select: {
      const fieldOptions: SelectOption[] = options.length > 0 ? options : [];

      if (fieldOptions.length > 0) {
        return h(Select, {
          ...commonProps,
          helperText: 'Select from available options',
          'onUpdate:modelValue': onChange,
          options: fieldOptions,
        } as Record<string, unknown>);
      }

      return h(TextField, {
        ...commonProps,
        helperText: 'Enter multiple values separated by commas (e.g., value1, value2, value3)',
        'onUpdate:modelValue': onChange,
        placeholder: 'value1, value2, value3',
        type: 'text',
      } as Record<string, unknown>);
    }

    default:
      return h(TextField, {
        ...commonProps,
        helperText: 'Unknown field type, treating as text',
        'onUpdate:modelValue': onChange,
        type: 'text',
      } as Record<string, unknown>);
  }
};

/**
 * FieldFactory — Vue component wrapper for the field factory.
 */
const FieldFactory: Component = defineComponent({
  name: 'FieldFactory',
  props: {
    className: {default: undefined, type: String},
    disabled: {default: false, type: Boolean},
    error: {default: undefined, type: String},
    label: {required: true, type: String},
    name: {required: true, type: String},
    options: {default: () => [], type: Array as PropType<SelectOption[]>},
    placeholder: {default: undefined, type: String},
    required: {default: false, type: Boolean},
    touched: {default: false, type: Boolean},
    type: {required: true, type: String as PropType<FieldType>},
    value: {default: '', type: String},
  },
  emits: ['change', 'blur'],
  setup(props: FieldFactorySetupProps, {emit}: {emit: any}): any {
    return () =>
      createField({
        className: props.className,
        disabled: props.disabled,
        error: props.error,
        label: props.label,
        name: props.name,
        onBlur: () => emit('blur'),
        onChange: (value: string) => emit('change', value),
        options: props.options,
        placeholder: props.placeholder,
        required: props.required,
        touched: props.touched,
        type: props.type,
        value: props.value,
      });
  },
});

export default FieldFactory;
