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
import {FlowMetadataResponse, WithPreferences, withVendorCSSClassPrefix} from '@thunderid/browser';
import {
  forwardRef,
  ForwardRefExoticComponent,
  ButtonHTMLAttributes,
  ReactElement,
  ReactNode,
  Ref,
  RefAttributes,
} from 'react';
import Button from '../../primitives/Button/Button';

/**
 * Common props shared by all {@link BaseSignUpButton} components.
 */
export interface CommonBaseSignUpButtonProps {
  /**
   * Loading state during sign-up process
   */
  isLoading?: boolean;
  /**
   * Flow metadata returned by the platform (v2 only). `null` while loading or unavailable.
   */
  meta?: FlowMetadataResponse | null;
  /**
   * Function to initiate the sign-up process
   */
  signUp?: () => Promise<void>;
}

/**
 * Props passed to the render function of {@link BaseSignUpButton}
 */
export type BaseSignUpButtonRenderProps = CommonBaseSignUpButtonProps;

/**
 * Props interface of {@link BaseSignUpButton}
 */
export interface BaseSignUpButtonProps
  extends CommonBaseSignUpButtonProps,
    Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'children'>,
    WithPreferences {
  /**
   * Render prop function that receives sign-up props, or traditional ReactNode children
   */
  children?: ((props: BaseSignUpButtonRenderProps) => ReactNode) | ReactNode;
}

/**
 * Base SignUpButton component that supports both render props and traditional props patterns.
 *
 * @example Using render props
 * ```tsx
 * <BaseSignUpButton>
 *   {({ signUp, isLoading }) => (
 *     <button onClick={signUp} disabled={isLoading}>
 *       {isLoading ? 'Creating account...' : 'Create Account'}
 *     </button>
 *   )}
 * </BaseSignUpButton>
 * ```
 *
 * @example Using traditional props
 * ```tsx
 * <BaseSignUpButton className="custom-button">Create Account</BaseSignUpButton>
 * ```
 */
const BaseSignUpButton: ForwardRefExoticComponent<BaseSignUpButtonProps & RefAttributes<HTMLButtonElement>> =
  forwardRef<HTMLButtonElement, BaseSignUpButtonProps>(
    (
      {children, className, style, signUp, isLoading, meta, preferences, ...rest}: BaseSignUpButtonProps,
      ref: Ref<HTMLButtonElement>,
    ): ReactElement => {
      if (typeof children === 'function') {
        return <>{children({isLoading, meta, signUp})}</>;
      }

      return (
        <Button
          ref={ref}
          className={cx(withVendorCSSClassPrefix('sign-up-button'), className)}
          style={style}
          disabled={isLoading}
          loading={isLoading}
          type="button"
          color="primary"
          variant="solid"
          {...rest}
        >
          {children}
        </Button>
      );
    },
  );

BaseSignUpButton.displayName = 'BaseSignUpButton';

export default BaseSignUpButton;
