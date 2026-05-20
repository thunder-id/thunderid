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

import {ThunderIDRuntimeError} from '@thunderid/browser';
import {forwardRef, ForwardRefExoticComponent, MouseEvent, ReactElement, Ref, RefAttributes, useState} from 'react';
import BaseSignOutButton, {BaseSignOutButtonProps} from './BaseSignOutButton';
import useThunderID from '../../../contexts/ThunderID/useThunderID';
import useTranslation from '../../../hooks/useTranslation';

/**
 * Props interface of {@link SignOutButton}
 */
export type SignOutButtonProps = BaseSignOutButtonProps;

/**
 * SignOutButton component that supports both render props and traditional props patterns.
 *
 * @remarks This component is only supported in browser based React applications (CSR).
 *
 * @example Using render props pattern
 * ```tsx
 * <SignOutButton>
 *   {({signOut, isLoading}) => (
 *     <button onClick={signOut} disabled={isLoading}>
 *       {isLoading ? 'Signing out...' : 'Sign Out'}
 *     </button>
 *   )}
 * </SignOutButton>
 * ```
 *
 * @example Using traditional props pattern
 * ```tsx
 * <SignOutButton className="custom-button">Sign Out</SignOutButton>
 * ```
 *
 * @example Using component-level preferences
 * ```tsx
 * <SignOutButton
 *   preferences={{
 *     i18n: {
 *       bundles: {
 *         'en-US': {
 *           translations: {
 *             'buttons.signOut': 'Custom Sign Out Text'
 *           }
 *         }
 *       }
 *     }
 *   }}
 * >
 *   Custom Sign Out
 * </SignOutButton>
 * ```
 */
const SignOutButton: ForwardRefExoticComponent<SignOutButtonProps & RefAttributes<HTMLButtonElement>> = forwardRef<
  HTMLButtonElement,
  SignOutButtonProps
>(({children, onClick, preferences, ...rest}: SignOutButtonProps, ref: Ref<HTMLButtonElement>): ReactElement => {
  const {signOut, meta} = useThunderID();
  const {t} = useTranslation(preferences?.i18n);

  const [isLoading, setIsLoading] = useState(false);

  const handleSignOut = async (e?: MouseEvent<HTMLButtonElement>): Promise<void> => {
    try {
      setIsLoading(true);

      await signOut();

      if (onClick) {
        onClick(e);
      }
    } catch (error) {
      throw new ThunderIDRuntimeError(
        `Sign out failed: ${error instanceof Error ? error.message : String(error)}`,
        'SignOutButton-handleSignOut-RuntimeError-001',
        'react',
        'Something went wrong while trying to sign out. Please try again later.',
      );
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <BaseSignOutButton
      ref={ref}
      onClick={handleSignOut}
      isLoading={isLoading}
      meta={meta}
      signOut={handleSignOut}
      preferences={preferences}
      {...rest}
    >
      {children ?? t('elements.buttons.signout.text')}
    </BaseSignOutButton>
  );
});

SignOutButton.displayName = 'SignOutButton';

export default SignOutButton;
