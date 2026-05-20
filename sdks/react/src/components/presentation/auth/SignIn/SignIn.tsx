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
  EmbeddedSignInFlowInitiateResponse,
  EmbeddedSignInFlowHandleResponse,
  EmbeddedSignInFlowHandleRequestPayload,
  Platform,
  Preferences,
} from '@thunderid/browser';
import {FC, ReactElement} from 'react';
import BaseSignIn, {BaseSignInProps} from './BaseSignIn';
import SignInV2, {SignInRenderProps} from './v2/SignIn';
import useThunderID from '../../../../contexts/ThunderID/useThunderID';

/**
 * Props for the SignIn component.
 * Extends BaseSignInProps for full compatibility with the React BaseSignIn component
 */
export type SignInProps = Pick<BaseSignInProps, 'className' | 'onSuccess' | 'onError' | 'variant' | 'size'> & {
  /**
   * Render function for custom UI (render props pattern).
   */
  children?: (props: SignInRenderProps) => ReactElement;
  /**
   * Component-level preferences to override global i18n and theme settings.
   */
  preferences?: Preferences;
};

/**
 * A styled SignIn component that provides native authentication flow with pre-built styling.
 * This component handles the API calls for authentication and delegates UI logic to BaseSignIn.
 *
 * @example
 * ```tsx
 * import { SignIn } from '@thunderid/react';
 *
 * const App = () => {
 *   return (
 *     <SignIn
 *       onSuccess={(authData) => {
 *         console.log('Authentication successful:', authData);
 *         // Handle successful authentication (e.g., redirect, store tokens)
 *       }}
 *       onError={(error) => {
 *         console.error('Authentication failed:', error);
 *       }}
 *       size="medium"
 *       variant="outlined"
 *     />
 *   );
 * };
 * ```
 */
const SignIn: FC<SignInProps> = ({className, size = 'medium', children, preferences, ...rest}: SignInProps) => {
  const {signIn, afterSignInUrl, isInitialized, isLoading, platform} = useThunderID();

  /**
   * Initialize the authentication flow.
   */
  const handleInitialize = async (): Promise<EmbeddedSignInFlowInitiateResponse> =>
    (await signIn({response_mode: 'direct'})) as EmbeddedSignInFlowInitiateResponse;

  /**
   * Handle authentication steps.
   */
  const handleOnSubmit = async (
    payload: EmbeddedSignInFlowHandleRequestPayload,
    request: Request,
  ): Promise<EmbeddedSignInFlowHandleResponse> => (await signIn(payload, request)) as EmbeddedSignInFlowHandleResponse;

  /**
   * Handle successful authentication and redirect with query params.
   */
  const handleSuccess = (authData: Record<string, any>): void => {
    if (authData && afterSignInUrl) {
      const url: URL = new URL(afterSignInUrl, window.location.origin);

      Object.entries(authData).forEach(([key, value]: [string, any]) => {
        if (value !== undefined && value !== null) {
          url.searchParams.append(key, String(value));
        }
      });

      window.location.href = url.toString();
    }
  };

  if (platform === Platform.ThunderID) {
    return (
      <SignInV2
        className={className}
        size={size}
        variant={rest.variant}
        onSuccess={rest.onSuccess}
        onError={rest.onError}
        preferences={preferences}
      >
        {children}
      </SignInV2>
    );
  }

  return (
    <BaseSignIn
      isLoading={isLoading || !isInitialized}
      afterSignInUrl={afterSignInUrl}
      onInitialize={handleInitialize}
      onSubmit={handleOnSubmit}
      onSuccess={handleSuccess}
      className={className}
      size={size}
      showLogo={true}
      showSubtitle={true}
      showTitle={true}
      preferences={preferences}
      {...rest}
    />
  );
};

export default SignIn;
