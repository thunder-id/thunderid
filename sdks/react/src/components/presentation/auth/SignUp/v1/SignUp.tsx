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
  EmbeddedFlowExecuteRequestPayload,
  EmbeddedFlowExecuteResponse,
  EmbeddedFlowResponseType,
  EmbeddedFlowType,
} from '@thunderid/browser';
import {FC, ReactElement, ReactNode} from 'react';
// eslint-disable-next-line import/no-named-as-default
import BaseSignUp, {BaseSignUpProps, BaseSignUpRenderProps} from './BaseSignUp';
import useThunderID from '../../../../../contexts/ThunderID/useThunderID';

/**
 * Render props function parameters (re-exported from BaseSignUp for convenience)
 */
export type SignUpRenderProps = BaseSignUpRenderProps;

/**
 * Props for the SignUp component.
 */
export type SignUpProps = BaseSignUpProps & {
  /**
   * Render props function for custom UI
   */
  children?: (props: SignUpRenderProps) => ReactNode;
};

/**
 * A styled SignUp component for ThunderID platform that provides embedded sign-up flow with pre-built styling.
 * This component handles the API calls for sign-up and delegates UI logic to BaseSignUp.
 */
const SignUp: FC<SignUpProps> = ({
  className,
  size = 'medium',
  afterSignUpUrl,
  onError,
  onComplete,
  shouldRedirectAfterSignUp = true,
  children,
  ...rest
}: SignUpProps): ReactElement => {
  const {signUp, isInitialized} = useThunderID();

  /**
   * Initialize the sign-up flow.
   */
  const handleInitialize = async (
    payload?: EmbeddedFlowExecuteRequestPayload,
  ): Promise<EmbeddedFlowExecuteResponse> => {
    // Uses simple initialization without applicationId
    const initialPayload: any = payload || {
      flowType: EmbeddedFlowType.Registration,
    };

    return (await signUp(initialPayload)) as EmbeddedFlowExecuteResponse;
  };

  /**
   * Handle sign-up steps.
   */
  const handleOnSubmit = async (payload: EmbeddedFlowExecuteRequestPayload): Promise<EmbeddedFlowExecuteResponse> =>
    (await signUp(payload)) as EmbeddedFlowExecuteResponse;

  /**
   * Handle successful sign-up and redirect.
   */
  const handleComplete = (response: EmbeddedFlowExecuteResponse): any => {
    onComplete?.(response);

    // For non-redirection responses (regular sign-up completion), handle redirect if configured
    if (shouldRedirectAfterSignUp && response?.type !== EmbeddedFlowResponseType.Redirection && afterSignUpUrl) {
      window.location.href = afterSignUpUrl;
    }

    // For redirection responses (social sign-up), handle direct redirect
    if (
      shouldRedirectAfterSignUp &&
      response?.type === EmbeddedFlowResponseType.Redirection &&
      response?.data?.redirectURL &&
      !response.data.redirectURL.includes('oauth') && // Not a social provider redirect
      !response.data.redirectURL.includes('auth') // Not an auth provider redirect
    ) {
      window.location.href = response.data.redirectURL;
    }
  };

  return (
    <BaseSignUp
      afterSignUpUrl={afterSignUpUrl}
      onInitialize={handleInitialize}
      onSubmit={handleOnSubmit}
      onError={onError}
      onComplete={handleComplete}
      className={className}
      size={size}
      isInitialized={isInitialized}
      children={children}
      showLogo={true}
      showTitle={false}
      showSubtitle={false}
      {...rest}
    />
  );
};

export default SignUp;
