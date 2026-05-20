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

import {ThunderIDRuntimeError, navigate} from '@thunderid/browser';
import {forwardRef, ForwardRefExoticComponent, MouseEvent, ReactElement, Ref, RefAttributes, useState} from 'react';
import BaseSignInButton, {BaseSignInButtonProps} from './BaseSignInButton';
import useThunderID from '../../../contexts/ThunderID/useThunderID';
import useTranslation from '../../../hooks/useTranslation';

/**
 * Props interface of {@link SignInButton}
 */
export type SignInButtonProps = BaseSignInButtonProps & {
  /**
   * Additional parameters to pass to the `authorize` request.
   */
  signInOptions?: Record<string, any>;
};

/**
 * SignInButton component that supports both render props and traditional props patterns.
 *
 * @remarks This component is only supported in browser based React applications (CSR).
 *
 * @example Using render props
 * ```tsx
 * <SignInButton>
 *   {({signIn, isLoading}) => (
 *     <button onClick={signIn} disabled={isLoading}>
 *       {isLoading ? 'Signing in...' : 'Sign In'}
 *     </button>
 *   )}
 * </SignInButton>
 * ```
 *
 * @example Using traditional props
 * ```tsx
 * <SignInButton className="custom-button">Sign In</SignInButton>
 * ```
 *
 * @example Using component-level preferences
 * ```tsx
 * <SignInButton
 *   preferences={{
 *     i18n: {
 *       bundles: {
 *         'en-US': {
 *           translations: {
 *             'buttons.signIn': 'Custom Sign In Text'
 *           }
 *         }
 *       }
 *     }
 *   }}
 * >
 *   Custom Sign In
 * </SignInButton>
 * ```
 */
const SignInButton: ForwardRefExoticComponent<SignInButtonProps & RefAttributes<HTMLButtonElement>> = forwardRef<
  HTMLButtonElement,
  SignInButtonProps
>(
  (
    {children, onClick, preferences, signInOptions: overriddenSignInOptions = {}, ...rest}: SignInButtonProps,
    ref: Ref<HTMLButtonElement>,
  ): ReactElement => {
    const {signIn, signInUrl, signInOptions, meta} = useThunderID();
    const {t} = useTranslation(preferences?.i18n);

    const [isLoading, setIsLoading] = useState(false);

    const handleSignIn = async (e?: MouseEvent<HTMLButtonElement>): Promise<void> => {
      try {
        setIsLoading(true);

        // If a custom `signInUrl` is provided, use it for navigation.
        if (signInUrl) {
          navigate(signInUrl);
        } else {
          await signIn(overriddenSignInOptions ?? signInOptions);
        }

        if (onClick) {
          onClick(e);
        }
      } catch (error) {
        throw new ThunderIDRuntimeError(
          `Sign in failed: ${error instanceof Error ? error.message : String(JSON.stringify(error))}`,
          'SignInButton-handleSignIn-RuntimeError-001',
          'react',
          'Something went wrong while trying to sign in. Please try again later.',
        );
      } finally {
        setIsLoading(false);
      }
    };

    return (
      <BaseSignInButton
        ref={ref}
        onClick={handleSignIn}
        isLoading={isLoading}
        meta={meta}
        signIn={handleSignIn}
        preferences={preferences}
        {...rest}
      >
        {children ?? t('elements.buttons.signin.text')}
      </BaseSignInButton>
    );
  },
);

SignInButton.displayName = 'SignInButton';

export default SignInButton;
