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

import {
  ApplicationNativeAuthenticationConstants,
  EmbeddedSignInFlowAuthenticatorKnownIdPType,
} from '@thunderid/browser';
import {FC, ReactElement} from 'react';
// eslint-disable-next-line import/no-cycle
import {BaseSignInOptionProps} from './SignInOptionFactory';
import useTranslation from '../../../../../../hooks/useTranslation';
import Button from '../../../../../primitives/Button/Button';

/**
 * Multi Option Button Component.
 * Renders authenticators as selectable buttons for multi-option prompts.
 * Used when authenticators don't require immediate user input but need to be selected first.
 */
const MultiOptionButton: FC<BaseSignInOptionProps> = ({
  authenticator,
  isLoading,
  onSubmit,
  buttonClassName = '',
  preferences,
}: BaseSignInOptionProps): ReactElement => {
  const {t} = useTranslation(preferences?.i18n);

  /**
   * Get display name for the authenticator.
   */
  const getDisplayName = (): string => {
    let authenticatorName: any = authenticator.authenticator;

    if (authenticator.idp !== EmbeddedSignInFlowAuthenticatorKnownIdPType.Local) {
      authenticatorName = authenticator.idp;
    }

    switch (authenticatorName) {
      default:
        return t('elements.buttons.multi.option.text', {connection: authenticatorName});
    }
  };

  /**
   * Get appropriate icon for the authenticator type.
   */
  const getIcon = (): ReactElement | null => {
    const {authenticatorId} = authenticator;

    switch (authenticatorId) {
      case ApplicationNativeAuthenticationConstants.SupportedAuthenticators.SmsOtp:
        return (
          <svg width="18" height="18" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
            <path
              fill="currentColor"
              d="M20 15.5c-1.25 0-2.45-.2-3.57-.57a1.02 1.02 0 0 0-1.02.24l-2.2 2.2a15.074 15.074 0 0 1-6.59-6.59l2.2-2.2c.27-.27.35-.67.24-1.02A11.36 11.36 0 0 1 8.5 4c0-.55-.45-1-1-1H4c-.55 0-1 .45-1 1 0 9.39 7.61 17 17 17 .55 0 1-.45 1-1v-3.5c0-.55-.45-1-1-1M12 3v10l3-3h6V3z"
            />
          </svg>
        );
      case ApplicationNativeAuthenticationConstants.SupportedAuthenticators.EmailOtp:
        return (
          <svg width="18" height="18" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
            <path
              fill="currentColor"
              d="M20 4H4c-1.1 0-1.99.9-1.99 2L2 18c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V6c0-1.1-.9-2-2-2m0 4l-8 5l-8-5V6l8 5l8-5z"
            />
          </svg>
        );
      case ApplicationNativeAuthenticationConstants.SupportedAuthenticators.Totp:
        return (
          <svg width="18" height="18" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
            <path fill="currentColor" d="M12 1L3 5v6c0 5.55 3.84 10.74 9 12c5.16-1.26 9-6.45 9-12V5z" />
          </svg>
        );
      case ApplicationNativeAuthenticationConstants.SupportedAuthenticators.PushNotification:
        return (
          <svg width="18" height="18" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
            <path
              fill="currentColor"
              d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10s10-4.48 10-10S17.52 2 12 2m0 18c-4.41 0-8-3.59-8-8s3.59-8 8-8s8 3.59 8 8s-3.59 8-8 8m1-13h-2v6h2zm0 8h-2v2h2z"
            />
          </svg>
        );
      case ApplicationNativeAuthenticationConstants.SupportedAuthenticators.Passkey:
        return (
          <svg fill="currentColor" width="18" height="18" viewBox="0 0 32 32" xmlns="http://www.w3.org/2000/svg">
            <g id="SVGRepo_bgCarrier" strokeWidth="0"></g>
            <g id="SVGRepo_tracerCarrier" strokeLinecap="round" strokeLinejoin="round"></g>
            <g id="SVGRepo_iconCarrier">
              <path d="M7.7 4.7C9.36 3.07 12.68 2 16.17 2S23 3.06 24.6 4.7A1 1 0 0 0 26 3.3C23.6.86 19.34 0 16.16 0S8.72.87 6.3 3.3a1 1 0 0 0 1.4 1.4zM29.2 12.55C26.38 6.88 22 4 16.17 4s-10.22 2.88-13 8.55a1 1 0 0 0 .44 1.34 1 1 0 0 0 1.35-.45C7.4 8.45 11.08 6 16.15 6s8.77 2.44 11.27 7.45a1 1 0 0 0 .9.55.87.87 0 0 0 .44-.1 1 1 0 0 0 .45-1.35zM19.4 28.08c-4.13-1.77-5.8-4.5-6-6.5a2.87 2.87 0 0 1 1.13-2.75c.85-.57 2.1.24 3.87 1.52s4.13 3 6.17 1.45c1.8-1.35 2.34-3.76 1.45-6.44A10.85 10.85 0 0 0 16.16 8C7.2 8 4 15.75 4 23a1 1 0 0 0 2 0c0-3 .73-13 10.16-13 3.9 0 7 3.1 8 6 .3.87.8 3-.75 4.2-.8.6-2-.2-3.8-1.47s-4.07-2.94-6.14-1.56a4.87 4.87 0 0 0-2 4.6c.24 2.56 2.24 6 7.18 8.15A1 1 0 0 0 19 30a1 1 0 0 0 .4-1.92zM10 19.24a7.06 7.06 0 0 1 5.2-4.65c2.24-.43 4.32.6 6 3a1 1 0 1 0 1.62-1.17c-2.9-4.07-6.27-4.12-8-3.8A9.1 9.1 0 0 0 8 18.77c-1 3.94.43 8.27 4.2 12.87a1 1 0 0 0 .8.37.94.94 0 0 0 .63-.23 1 1 0 0 0 .14-1.4c-3.34-4.1-4.62-7.83-3.77-11.13zM25.3 24.3a3 3 0 0 1-3.06.63c-2.4-.57-4.78-2.7-5.3-4.25a1 1 0 1 0-1.9.64c.8 2.33 3.87 4.88 6.74 5.56a6.84 6.84 0 0 0 1.52.18 4.7 4.7 0 0 0 3.4-1.35 1 1 0 0 0-1.4-1.4z"></path>{' '}
            </g>
          </svg>
        );
      case ApplicationNativeAuthenticationConstants.SupportedAuthenticators.MagicLink:
        return (
          <svg width="18" height="18" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
            <path
              fill="currentColor"
              d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10s10-4.48 10-10S17.52 2 12 2m0 18c-4.41 0-8-3.59-8-8s3.59-8 8-8s8 3.59 8 8s-3.59 8-8 8m1-13h-2v6h2zm0 8h-2v2h2z"
            />
          </svg>
        );
      default:
        return (
          <svg width="18" height="18" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
            <path
              fill="currentColor"
              d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10s10-4.48 10-10S17.52 2 12 2m-2 15l-5-5l1.41-1.41L10 14.17l7.59-7.59L19 8z"
            />
          </svg>
        );
    }
  };

  /**
   * Handle button click.
   */
  const handleClick = (): any => {
    // For multi-option buttons, we call onSubmit without form data
    // This will trigger the authenticator selection and likely move to the next step
    onSubmit(authenticator);
  };

  return (
    <Button
      fullWidth
      type="button"
      color="secondary"
      variant="solid"
      disabled={isLoading}
      onClick={handleClick}
      className={buttonClassName}
      startIcon={getIcon()}
    >
      {getDisplayName()}
    </Button>
  );
};

export default MultiOptionButton;
