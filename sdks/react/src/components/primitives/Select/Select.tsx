/**
 * Copyright (c) 2024, WSO2 LLC. (https://www.wso2.com).
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

import {cx} from '@emotion/css';
import {withVendorCSSClassPrefix, bem} from '@thunderid/browser';
import {FC, SelectHTMLAttributes} from 'react';
import useStyles from './Select.styles';
import useTheme from '../../../contexts/Theme/useTheme';
import FormControl from '../FormControl/FormControl';
import InputLabel from '../InputLabel/InputLabel';

export interface SelectOption {
  /**
   * The text that will be displayed in the select
   */
  label: string;
  /**
   * The value that will be submitted with the form
   */
  value: string;
}

export interface SelectProps extends Omit<SelectHTMLAttributes<HTMLSelectElement>, 'className'> {
  /**
   * Additional CSS class names
   */
  className?: string;
  /**
   * Whether the field is disabled
   */
  disabled?: boolean;
  /**
   * Error message to display below the select
   */
  error?: string;
  /**
   * Helper text to display below the select
   */
  helperText?: string;
  /**
   * Label text to display above the select
   */
  label?: string;
  /**
   * The options to display in the select
   */
  options: SelectOption[];
  /**
   * Placeholder text for the default/empty option
   */
  placeholder?: string;
  /**
   * Whether the field is required
   */
  required?: boolean;
}

const Select: FC<SelectProps> = ({
  label,
  error,
  className,
  required,
  disabled,
  helperText,
  placeholder,
  options,
  style = {},
  ...rest
}: SelectProps) => {
  const {theme, colorScheme}: ReturnType<typeof useTheme> = useTheme();
  const hasError = !!error;
  const styles: Record<string, string> = useStyles(theme, colorScheme, disabled, hasError);

  const selectClassName: string = cx(
    withVendorCSSClassPrefix(bem('select', 'input')),
    styles['select'],
    hasError && styles['selectError'],
    disabled && styles['selectDisabled'],
  );

  return (
    <FormControl
      error={error}
      helperText={helperText}
      className={cx(withVendorCSSClassPrefix(bem('select')), className)}
      style={style}
    >
      {label && (
        <InputLabel required={required} error={hasError}>
          {label}
        </InputLabel>
      )}
      <select
        className={selectClassName}
        disabled={disabled}
        aria-invalid={hasError}
        aria-required={required}
        {...rest}
      >
        {placeholder && (
          <option value="" disabled>
            {placeholder}
          </option>
        )}
        {options.map((option: SelectOption) => (
          <option key={option.value} value={option.value} className={styles['option']}>
            {option.label}
          </option>
        ))}
      </select>
    </FormControl>
  );
};

export default Select;
