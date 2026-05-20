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

'use client';

import {ThunderIDRuntimeError} from '@thunderid/node';
import {BaseSignUpButton, BaseSignUpButtonProps, useTranslation} from '@thunderid/react';
import {AppRouterInstance} from 'next/dist/shared/lib/app-router-context.shared-runtime';
import {useRouter} from 'next/navigation';
import {forwardRef, ForwardRefExoticComponent, MouseEvent, ReactElement, Ref, RefAttributes, useState} from 'react';
import useThunderID from '../../../contexts/ThunderID/useThunderID';

/**
 * Props interface of {@link SignUpButton}
 */
export type SignUpButtonProps = BaseSignUpButtonProps;

/**
 * SignUpButton component that supports both render props and traditional props patterns.
 * It redirects the user to the ThunderID sign-up page configured for the application.
 *
 * @remarks This component is only supported in browser based React applications (CSR).
 *
 * @example Using render props pattern
 * ```tsx
 * <SignUpButton>
 *   {({ signUp, isLoading }) => (
 *     <button onClick={signUp} disabled={isLoading}>
 *       {isLoading ? 'Creating Account...' : 'Create Account'}
 *     </button>
 *   )}
 * </SignUpButton>
 * ```
 *
 * @example Using traditional props pattern
 * ```tsx
 * <SignUpButton className="custom-button">Create Account</SignUpButton>
 * ```
 *
 * @example Using component-level preferences
 * ```tsx
 * <SignUpButton
 *   preferences={{
 *     i18n: {
 *       bundles: {
 *         'en-US': {
 *           translations: {
 *             'buttons.signUp': 'Custom Sign Up Text'
 *           }
 *         }
 *       }
 *     }
 *   }}
 * >
 *   Custom Sign Up
 * </SignUpButton>
 * ```
 */
const SignUpButton: ForwardRefExoticComponent<SignUpButtonProps & RefAttributes<HTMLButtonElement>> = forwardRef<
  HTMLButtonElement,
  SignUpButtonProps
>(({children, onClick, preferences, ...rest}: SignUpButtonProps, ref: Ref<HTMLButtonElement>): ReactElement => {
  const {signUp, signUpUrl} = useThunderID();
  const router: AppRouterInstance = useRouter();
  const {t} = useTranslation(preferences?.i18n);

  const [isLoading, setIsLoading] = useState(false);

  const handleSignUp = async (e?: MouseEvent<HTMLButtonElement>): Promise<void> => {
    try {
      setIsLoading(true);

      // If a custom `signUpUrl` is provided, use it for navigation.
      if (signUpUrl) {
        router.push(signUpUrl);
      } else if (signUp) {
        await signUp();
      }

      if (onClick) {
        onClick(e!);
      }
    } catch (error) {
      throw new ThunderIDRuntimeError(
        `Sign up failed: ${error instanceof Error ? error.message : String(error)}`,
        'SignUpButton-handleSignUp-RuntimeError-001',
        'nextjs',
        'Something went wrong while trying to sign up. Please try again later.',
      );
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <BaseSignUpButton
      ref={ref}
      onClick={handleSignUp}
      isLoading={isLoading}
      signUp={handleSignUp}
      preferences={preferences}
      {...rest}
    >
      {children ?? t('elements.buttons.signup.text')}
    </BaseSignUpButton>
  );
});

SignUpButton.displayName = 'SignUpButton';

export default SignUpButton;
