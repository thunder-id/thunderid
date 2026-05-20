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
import {FC, InputHTMLAttributes} from 'react';
import useStyles from './DatePicker.styles';
import useTheme from '../../../contexts/Theme/useTheme';
import FormControl from '../FormControl/FormControl';
import InputLabel from '../InputLabel/InputLabel';

export interface DatePickerProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'className' | 'type'> {
  /**
   * Additional CSS class names
   */
  className?: string;
  /**
   * Custom date format for the regex pattern
   */
  dateFormat?: string;
  /**
   * Whether the field is disabled
   */
  disabled?: boolean;
  /**
   * Error message to display below the input
   */
  error?: string;
  /**
   * Helper text to display below the input
   */
  helperText?: string;
  /**
   * Label text to display above the input
   */
  label?: string;
  /**
   * Whether the field is required
   */
  required?: boolean;
}

const DatePicker: FC<DatePickerProps> = ({
  label,
  error,
  className,
  required,
  disabled,
  helperText,
  dateFormat = 'yyyy-MM-dd',
  style = {},
  ...rest
}: DatePickerProps) => {
  const {theme, colorScheme}: ReturnType<typeof useTheme> = useTheme();
  const hasError = !!error;
  const styles: Record<string, string> = useStyles(theme, colorScheme, hasError, !!disabled);

  return (
    <FormControl
      error={error}
      helperText={helperText}
      className={cx(withVendorCSSClassPrefix(bem('date-picker')), className)}
      style={style}
    >
      {label && (
        <InputLabel
          required={required}
          error={hasError}
          className={cx(withVendorCSSClassPrefix(bem('date-picker', 'label')), styles['label'])}
        >
          {label}
        </InputLabel>
      )}
      <input
        type="date"
        pattern="\d{4}-\d{2}-\d{2}"
        placeholder={dateFormat}
        className={cx(
          withVendorCSSClassPrefix(bem('date-picker', 'input')),
          styles['input'],
          styles['errorInput'],
          styles['disabledInput'],
          {
            [withVendorCSSClassPrefix(bem('date-picker', 'input', 'error'))]: hasError,
            [withVendorCSSClassPrefix(bem('date-picker', 'input', 'disabled'))]: disabled,
          },
        )}
        disabled={disabled}
        aria-invalid={hasError}
        aria-required={required}
        {...rest}
      />
    </FormControl>
  );
};

export default DatePicker;
