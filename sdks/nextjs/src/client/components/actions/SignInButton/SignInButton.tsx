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
import {BaseSignInButton, BaseSignInButtonProps, useTranslation} from '@thunderid/react';
import {AppRouterInstance} from 'next/dist/shared/lib/app-router-context.shared-runtime';
import {useRouter} from 'next/navigation';
import {forwardRef, ForwardRefExoticComponent, ReactElement, Ref, RefAttributes, MouseEvent} from 'react';
import useThunderID from '../../../contexts/ThunderID/useThunderID';

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
 * SignInButton component that uses server actions for authentication in Next.js.
 *
 * @example Using render props
 * ```tsx
 * <SignInButton>
 *   {({isLoading}) => (
 *     <button type="submit" disabled={isLoading}>
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
 * @remarks
 * In Next.js with server actions, the sign-in is handled via the server action.
 * When using render props, the custom button should use `type="submit"` instead of `onClick={signIn}`.
 * The `signIn` function in render props is provided for API consistency but should not be used directly.
 */
const SignInButton: ForwardRefExoticComponent<SignInButtonProps & RefAttributes<HTMLButtonElement>> = forwardRef<
  HTMLButtonElement,
  SignInButtonProps
>(
  (
    {className, style, children, preferences, onClick, signInOptions = {}, ...rest}: SignInButtonProps,
    ref: Ref<HTMLButtonElement>,
  ): ReactElement => {
    const {signIn, signInUrl} = useThunderID();
    const router: AppRouterInstance = useRouter();
    const {t} = useTranslation(preferences?.i18n);

    const handleOnClick = async (e: MouseEvent<HTMLButtonElement>): Promise<void> => {
      try {
        // If a custom `signInUrl` is provided, use it for navigation.
        if (signInUrl) {
          router.push(signInUrl);
        } else if (signIn) {
          await signIn(signInOptions);
        }

        if (onClick) {
          onClick(e);
        }
      } catch (error) {
        throw new ThunderIDRuntimeError(
          `Sign in failed: ${error instanceof Error ? error.message : String(error)}`,
          'SignInButton-handleSignIn-RuntimeError-001',
          'nextjs',
          'Something went wrong while trying to sign in. Please try again later.',
        );
      }
    };

    return (
      <BaseSignInButton
        className={className}
        style={style}
        ref={ref}
        preferences={preferences}
        onClick={handleOnClick}
        {...rest}
      >
        {children ?? t('elements.buttons.signin.text')}
      </BaseSignInButton>
    );
  },
);

SignInButton.displayName = 'SignInButton';

export default SignInButton;
