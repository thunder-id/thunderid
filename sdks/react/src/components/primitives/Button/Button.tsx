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

import {cx} from '@emotion/css';
import {withVendorCSSClassPrefix, bem} from '@thunderid/browser';
import {ButtonHTMLAttributes, forwardRef, ReactNode, ForwardRefExoticComponent, RefAttributes, Ref} from 'react';
import useStyles from './Button.styles';
import useTheme from '../../../contexts/Theme/useTheme';
import Spinner, {SpinnerSize} from '../Spinner/Spinner';

export type ButtonColor = 'primary' | 'secondary' | 'tertiary' | string;
export type ButtonVariant = 'solid' | 'outline' | 'text' | 'icon';
export type ButtonSize = 'small' | 'medium' | 'large';

export interface ButtonProps extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'color'> {
  /**
   * The button color that determines the color scheme
   */
  color?: ButtonColor;
  /**
   * Icon to display after the button text
   */
  endIcon?: ReactNode;
  /**
   * Whether the button should take the full width of its container
   */
  fullWidth?: boolean;
  /**
   * Whether the button is in a loading state
   */
  loading?: boolean;
  /**
   * The shape of the button: square or round
   */
  shape?: 'square' | 'round';
  /**
   * The size of the button
   */
  size?: ButtonSize;
  /**
   * Icon to display before the button text
   */
  startIcon?: ReactNode;
  /**
   * The button variant that determines the visual style
   */
  variant?: ButtonVariant;
}

const getSpinnerWidth = (sizeVal: ButtonSize, spacingUnit: string): string => {
  if (sizeVal === 'small') {
    return `calc(${spacingUnit} * 1.5)`;
  }
  if (sizeVal === 'medium') {
    return `calc(${spacingUnit} * 2)`;
  }
  return `calc(${spacingUnit} * 2.5)`;
};

/**
 * Button component with multiple variants and types.
 *
 * @example
 * ```tsx
 * // Primary solid button
 * <Button color="primary" variant="solid">
 *   Click me
 * </Button>
 *
 * // Secondary outline button
 * <Button color="secondary" variant="outline" size="large">
 *   Cancel
 * </Button>
 *
 * // Text button with loading state
 * <Button color="tertiary" variant="text" loading>
 *   Loading...
 * </Button>
 *
 * // Button with icons
 * <Button
 *   color="primary"
 *   startIcon={<Icon />}
 *   endIcon={<Arrow />}
 * >
 *   Save and Continue
 * </Button>
 * ```
 */
const Button: ForwardRefExoticComponent<ButtonProps & RefAttributes<HTMLButtonElement>> = forwardRef<
  HTMLButtonElement,
  ButtonProps
>(
  (
    {
      color = 'primary',
      variant = 'solid',
      size = 'medium',
      fullWidth = false,
      loading = false,
      startIcon,
      endIcon,
      children,
      className,
      disabled,
      style,
      shape = 'square',
      ...rest
    }: ButtonProps,
    ref: Ref<HTMLButtonElement>,
  ) => {
    const {theme, colorScheme}: ReturnType<typeof useTheme> = useTheme();
    const styles: Record<string, string> = useStyles(
      theme,
      colorScheme,
      color,
      variant,
      size,
      fullWidth,
      disabled || false,
      loading,
      shape,
    );

    const isIconVariant: boolean = variant === 'icon';

    const spinnerWidth: string = getSpinnerWidth(size, theme.vars.spacing.unit);

    return (
      <button
        ref={ref}
        style={style}
        className={cx(
          withVendorCSSClassPrefix(bem('button')),
          withVendorCSSClassPrefix(bem('button', variant)),
          withVendorCSSClassPrefix(bem('button', color)),
          withVendorCSSClassPrefix(bem('button', size)),
          withVendorCSSClassPrefix(bem('button', shape)),
          fullWidth ? withVendorCSSClassPrefix(bem('button', 'fullWidth')) : undefined,
          loading ? withVendorCSSClassPrefix(bem('button', 'loading')) : undefined,
          disabled || loading ? withVendorCSSClassPrefix(bem('button', 'disabled')) : undefined,
          styles['button'],
          styles['size'],
          styles['variant'],
          styles['fullWidth'],
          styles['loading'],
          styles['shape'],
          className,
        )}
        disabled={disabled || loading}
        {...rest}
      >
        {loading && (
          <span className={cx(withVendorCSSClassPrefix(bem('button', 'spinner')), styles['spinner'])}>
            <Spinner
              size={size as SpinnerSize}
              color="currentColor"
              style={{
                height: spinnerWidth,
                width: spinnerWidth,
              }}
            />
          </span>
        )}
        {!loading && isIconVariant && (
          <span className={cx(withVendorCSSClassPrefix(bem('button', 'icon')), styles['icon'])}>
            {children || startIcon || endIcon}
          </span>
        )}
        {!loading && !isIconVariant && startIcon && (
          <span className={cx(withVendorCSSClassPrefix(bem('button', 'start-icon')), styles['startIcon'])}>
            {startIcon}
          </span>
        )}
        {!isIconVariant && children && (
          <span className={cx(withVendorCSSClassPrefix(bem('button', 'content')), styles['content'])}>{children}</span>
        )}
        {!loading && !isIconVariant && endIcon && (
          <span className={cx(withVendorCSSClassPrefix(bem('button', 'end-icon')), styles['endIcon'])}>{endIcon}</span>
        )}
      </button>
    );
  },
);

Button.displayName = 'Button';

export default Button;
