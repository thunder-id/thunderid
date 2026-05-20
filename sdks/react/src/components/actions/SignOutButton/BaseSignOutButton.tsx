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
 * Common props shared by all {@link BaseSignOutButton} components.
 */
export interface CommonBaseSignOutButtonProps {
  /**
   * Loading state during sign-out process
   */
  isLoading?: boolean;
  /**
   * Flow metadata returned by the platform (v2 only). `null` while loading or unavailable.
   */
  meta?: FlowMetadataResponse | null;
  /**
   * Function to initiate the sign-out process
   */
  signOut: () => Promise<void>;
}

/**
 * Props passed to the render function of {@link BaseSignOutButton}
 */
export type BaseSignOutButtonRenderProps = CommonBaseSignOutButtonProps;

/**
 * Props interface of {@link BaseSignOutButton}
 */
export interface BaseSignOutButtonProps
  extends Partial<CommonBaseSignOutButtonProps>,
    Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'children'>,
    WithPreferences {
  /**
   * Render prop function that receives sign-out props, or traditional ReactNode children
   */
  children?: ((props: BaseSignOutButtonRenderProps) => ReactNode) | ReactNode;
}

/**
 * Base SignOutButton component that supports both render props and traditional props patterns.
 *
 * @example Using render props
 * ```tsx
 * <BaseSignOutButton>
 *   {({signOut, isLoading}) => (
 *     <button onClick={signOut} disabled={isLoading}>
 *       {isLoading ? 'Signing out...' : 'Sign Out'}
 *     </button>
 *   )}
 * </BaseSignOutButton>
 * ```
 *
 * @example Using traditional props
 * ```tsx
 * <BaseSignOutButton className="custom-button">Sign Out</BaseSignOutButton>
 * ```
 */
const BaseSignOutButton: ForwardRefExoticComponent<BaseSignOutButtonProps & RefAttributes<HTMLButtonElement>> =
  forwardRef<HTMLButtonElement, BaseSignOutButtonProps>(
    (
      {children, className, style, signOut, isLoading, meta, preferences, ...rest}: BaseSignOutButtonProps,
      ref: Ref<HTMLButtonElement>,
    ): ReactElement => {
      if (typeof children === 'function') {
        return <>{children({isLoading, meta, signOut})}</>;
      }

      return (
        <Button
          ref={ref}
          className={cx(withVendorCSSClassPrefix('sign-out-button'), className)}
          style={style}
          disabled={isLoading}
          loading={isLoading}
          type="button"
          {...rest}
        >
          {children}
        </Button>
      );
    },
  );

BaseSignOutButton.displayName = 'BaseSignOutButton';

export default BaseSignOutButton;
